package graphql

import (
	"errors"
	"fmt"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"
)

const (
	// maxQueryDepth 查询最大嵌套深度
	maxQueryDepth = 5
	// maxQueryComplexity 根级最大并行字段数（片段展开后）
	maxQueryComplexity = 50
	// maxFieldsPerSelectionSet 单个 selection set 展开片段后的最大字段数
	//
	// graphql-go 的 OverlappingFieldsCanBeMergedRule 会对同一 selection set 内
	// 同名响应字段做两两比较（vendor/github.com/graphql-go/graphql/
	// rules_overlapping_fields_can_be_merged.go 的 collectConflictsWithin，
	// 嵌套 for 循环），复杂度是 O(k²)。只要 k 能被撑大，一个未授权请求就能在
	// 任何解析器真正执行前吃掉几十秒 CPU，因此必须限制单个 set 的字段规模。
	maxFieldsPerSelectionSet = 50
	// maxTotalFields 整个文档允许的最大字段节点数（片段全部展开后统计）
	maxTotalFields = 500
	// maxQueryBytes 查询语句最大字节数
	maxQueryBytes = 64 * 1024
	// countSaturate 计数饱和上限，避免片段互相引用时计数溢出
	countSaturate = 1 << 30
)

var (
	// ErrQueryTooDeep 查询嵌套过深
	ErrQueryTooDeep = errors.New("Query too deep")
	// ErrQueryTooComplex 根级并行字段过多
	ErrQueryTooComplex = errors.New("Query too complex")
	// ErrQueryTooManyFields 查询字段过多
	ErrQueryTooManyFields = errors.New("Query has too many fields")
	// ErrQueryTooLong 查询语句过长
	ErrQueryTooLong = errors.New("Query is too long")
	// ErrFragmentCycle 片段存在循环引用
	ErrFragmentCycle = errors.New("Fragment cycle detected")
	// ErrInvalidQuery 查询语句无法解析
	ErrInvalidQuery = errors.New("invalid GraphQL query")
)

// queryStats 查询规模统计结果（所有指标都基于片段完全展开后的文档）
type queryStats struct {
	// Depth 展开片段后的最大嵌套深度
	Depth int
	// RootFields 根级并行字段数（片段展开后），多个 operation 取最大值
	RootFields int
	// TotalFields 文档中的字段节点总数（按 AST 节点去重）
	TotalFields int
	// MaxSetFields 单个 selection set 展开片段后的最大字段数
	MaxSetFields int
}

// queryCounter 负责把查询文档按片段展开后统计规模
type queryCounter struct {
	// fragments 文档中定义的全部片段（包含未被引用的片段）
	fragments map[string]*ast.FragmentDefinition
	// fragmentFields 记忆化的片段直接字段数，避免片段嵌套导致的重复展开
	fragmentFields map[string]int
	// fragmentDepths 记忆化的片段内部深度，避免片段嵌套导致的重复计算
	fragmentDepths map[string]int
	// visited 已统计过的 selection set，保证字段节点按 AST 节点去重计数
	visited map[*ast.SelectionSet]bool
	stats   queryStats
}

// analyzeQuery 解析查询并统计规模。
// 超过限制时返回对应的哨兵错误（ErrQueryTooComplex 等），由调用方直接拒绝请求。
func analyzeQuery(query string) (*queryStats, error) {
	if len(query) > maxQueryBytes {
		return nil, ErrQueryTooLong
	}

	doc, err := parser.Parse(parser.ParseParams{
		Source: source.NewSource(&source.Source{
			Body: []byte(query),
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidQuery, err.Error())
	}

	c := &queryCounter{
		fragments:      map[string]*ast.FragmentDefinition{},
		fragmentFields: map[string]int{},
		fragmentDepths: map[string]int{},
		visited:        map[*ast.SelectionSet]bool{},
	}

	// 收集全部片段定义：graphql-go 的校验会遍历文档里的每一个 selection set，
	// 未被引用的片段同样会进入两两比较，所以不能只统计被引用的片段。
	for _, def := range doc.Definitions {
		if fd, ok := def.(*ast.FragmentDefinition); ok && fd.Name != nil {
			c.fragments[fd.Name.Value] = fd
		}
	}

	if err := c.detectFragmentCycles(); err != nil {
		return nil, err
	}

	for _, def := range doc.Definitions {
		switch d := def.(type) {
		case *ast.OperationDefinition:
			if d.SelectionSet == nil {
				continue
			}
			c.walk(d.SelectionSet)
			// 根级并行字段数：片段在引用处展开后计入根级
			if root := c.countSelectionSet(d.SelectionSet); root > c.stats.RootFields {
				c.stats.RootFields = root
			}
			if depth := c.selectionSetDepth(d.SelectionSet, 1); depth > c.stats.Depth {
				c.stats.Depth = depth
			}
		case *ast.FragmentDefinition:
			if d.SelectionSet == nil {
				continue
			}
			c.walk(d.SelectionSet)
		}
	}

	switch {
	case c.stats.Depth > maxQueryDepth:
		return &c.stats, ErrQueryTooDeep
	case c.stats.RootFields > maxQueryComplexity:
		return &c.stats, ErrQueryTooComplex
	case c.stats.MaxSetFields > maxFieldsPerSelectionSet:
		return &c.stats, ErrQueryTooManyFields
	case c.stats.TotalFields > maxTotalFields:
		return &c.stats, ErrQueryTooManyFields
	}
	return &c.stats, nil
}

// detectFragmentCycles 检测片段之间的循环引用。
// 循环片段既是无效 GraphQL，也会在展开统计时造成无限递归，必须先拦掉。
func (c *queryCounter) detectFragmentCycles() error {
	const (
		visiting = 1
		visited  = 2
	)
	state := map[string]int{}

	var visit func(name string) error
	visit = func(name string) error {
		switch state[name] {
		case visiting:
			return ErrFragmentCycle
		case visited:
			return nil
		}
		state[name] = visiting
		frag := c.fragments[name]
		if frag != nil && frag.SelectionSet != nil {
			for _, sel := range frag.SelectionSet.Selections {
				spread, ok := sel.(*ast.FragmentSpread)
				if !ok || spread.Name == nil {
					continue
				}
				if err := visit(spread.Name.Value); err != nil {
					return err
				}
			}
		}
		state[name] = visited
		return nil
	}

	for name := range c.fragments {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

// walk 遍历 selection set，统计字段节点总数与单个 set 的最大字段数。
// 片段引用不在这里展开：片段自身的 selection set 会在片段定义处单独统计。
func (c *queryCounter) walk(set *ast.SelectionSet) {
	if set == nil || c.visited[set] {
		return
	}
	c.visited[set] = true

	if direct := c.countSelectionSet(set); direct > c.stats.MaxSetFields {
		c.stats.MaxSetFields = direct
	}

	for _, sel := range set.Selections {
		switch s := sel.(type) {
		case *ast.Field:
			c.stats.TotalFields++
			c.walk(s.SelectionSet)
		case *ast.InlineFragment:
			c.walk(s.SelectionSet)
		case *ast.FragmentSpread:
			// 片段定义的 selection set 由 walk 在片段定义处处理
		}
	}
}

// countSelectionSet 统计 selection set 在当前层（片段在引用处展开）的字段数。
// 只统计本层字段，不下钻字段的子选择集。
func (c *queryCounter) countSelectionSet(set *ast.SelectionSet) int {
	if set == nil {
		return 0
	}
	total := 0
	for _, sel := range set.Selections {
		switch s := sel.(type) {
		case *ast.Field:
			total = addCapped(total, 1)
		case *ast.InlineFragment:
			total = addCapped(total, c.countSelectionSet(s.SelectionSet))
		case *ast.FragmentSpread:
			total = addCapped(total, c.fragmentFieldCount(s.Name))
		}
	}
	return total
}

// fragmentFieldCount 返回片段被引用时在引用处贡献的字段数，结果记忆化。
func (c *queryCounter) fragmentFieldCount(name *ast.Name) int {
	if name == nil {
		return 0
	}
	key := name.Value
	if cached, ok := c.fragmentFields[key]; ok {
		return cached
	}
	frag := c.fragments[key]
	if frag == nil {
		// 未定义的片段交给 graphql 自身报错，这里不计数
		c.fragmentFields[key] = 0
		return 0
	}
	// 先占位，防止异常结构导致递归展开
	c.fragmentFields[key] = countSaturate
	count := c.countSelectionSet(frag.SelectionSet)
	c.fragmentFields[key] = count
	return count
}

// selectionSetDepth 计算 selection set 的最大层级（片段在引用处展开）。
// base 是本层的层级号（根级为 1）。
func (c *queryCounter) selectionSetDepth(set *ast.SelectionSet, base int) int {
	if set == nil {
		return base - 1
	}
	maxDepth := base - 1
	for _, sel := range set.Selections {
		switch s := sel.(type) {
		case *ast.Field:
			depth := base
			if s.SelectionSet != nil && len(s.SelectionSet.Selections) > 0 {
				depth = c.selectionSetDepth(s.SelectionSet, base+1)
			}
			if depth > maxDepth {
				maxDepth = depth
			}
		case *ast.InlineFragment:
			if depth := c.selectionSetDepth(s.SelectionSet, base); depth > maxDepth {
				maxDepth = depth
			}
		case *ast.FragmentSpread:
			// 片段内字段与引用处在同一层，片段内部深度额外叠加
			if fragDepth := c.fragmentDepth(s.Name); fragDepth > 0 {
				if depth := base + fragDepth - 1; depth > maxDepth {
					maxDepth = depth
				}
			}
		}
	}
	return maxDepth
}

// fragmentDepth 返回片段内部的最大层级（片段根为第 1 层），结果记忆化。
func (c *queryCounter) fragmentDepth(name *ast.Name) int {
	if name == nil {
		return 0
	}
	key := name.Value
	if cached, ok := c.fragmentDepths[key]; ok {
		return cached
	}
	frag := c.fragments[key]
	if frag == nil {
		c.fragmentDepths[key] = 0
		return 0
	}
	// 先占位，防止异常结构导致递归展开
	c.fragmentDepths[key] = countSaturate
	depth := c.selectionSetDepth(frag.SelectionSet, 1)
	c.fragmentDepths[key] = depth
	return depth
}

// addCapped 饱和加法，防止计数溢出
func addCapped(a, b int) int {
	if a >= countSaturate || b >= countSaturate {
		return countSaturate
	}
	if sum := a + b; sum > countSaturate {
		return countSaturate
	} else {
		return sum
	}
}

// queryErrorMessage 把守卫错误转换成可展示给调用方的提示
func queryErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrQueryTooDeep):
		return ErrQueryTooDeep.Error()
	case errors.Is(err, ErrQueryTooComplex):
		return ErrQueryTooComplex.Error()
	case errors.Is(err, ErrQueryTooManyFields):
		return ErrQueryTooManyFields.Error()
	case errors.Is(err, ErrQueryTooLong):
		return ErrQueryTooLong.Error()
	case errors.Is(err, ErrFragmentCycle):
		return ErrFragmentCycle.Error()
	default:
		return ErrInvalidQuery.Error()
	}
}
