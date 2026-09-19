package provider

// ACAutomaton 是一个轻量级的 AC 自动机（Aho-Corasick）实现，
// 用于在一段文本中同时匹配多个模式串（tag title）。
// 相比逐个 strings.Contains，AC 自动机只需扫描文本一次即可找到所有匹配。

// acNode 是 AC 自动机的一个节点
type acNode struct {
	children map[rune]*acNode
	fail     *acNode
	// output 存储以该节点为结尾的模式串索引
	output []int
}

// ACAutomaton AC 自动机
type ACAutomaton struct {
	root    *acNode
	pattern []string
	// patternIds 与 pattern 一一对应，存储每个模式串关联的业务 id
	patternIds []uint
}

// NewACAutomaton 创建一个空的 AC 自动机
func NewACAutomaton() *ACAutomaton {
	return &ACAutomaton{
		root: &acNode{
			children: make(map[rune]*acNode),
		},
	}
}

// AddPattern 添加一个模式串及其关联的 id
func (ac *ACAutomaton) AddPattern(pattern string, id uint) {
	if pattern == "" {
		return
	}
	idx := len(ac.pattern)
	ac.pattern = append(ac.pattern, pattern)
	ac.patternIds = append(ac.patternIds, id)

	node := ac.root
	for _, ch := range pattern {
		child, ok := node.children[ch]
		if !ok {
			child = &acNode{children: make(map[rune]*acNode)}
			node.children[ch] = child
		}
		node = child
	}
	node.output = append(node.output, idx)
}

// Build 在添加完所有模式串后调用，构建 fail 指针
func (ac *ACAutomaton) Build() {
	// 使用 BFS 构建 fail 指针
	queue := make([]*acNode, 0, 1024)

	// 第一层节点的 fail 指向 root
	for _, child := range ac.root.children {
		child.fail = ac.root
		queue = append(queue, child)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for ch, child := range current.children {
			// 计算 child 的 fail 指针
			fail := current.fail
			for fail != nil {
				if failNode, ok := fail.children[ch]; ok {
					child.fail = failNode
					break
				}
				fail = fail.fail
			}
			if child.fail == nil {
				child.fail = ac.root
			}

			// 合并 fail 节点的 output（后缀匹配）
			if child.fail != ac.root && len(child.fail.output) > 0 {
				child.output = append(child.output, child.fail.output...)
			}

			queue = append(queue, child)
		}
	}
}

// MatchResult 存储一个匹配结果
type MatchResult struct {
	TagId uint
}

// Search 在文本中搜索所有匹配的模式串，最多返回 maxResults 个不重复的结果。
// 当达到 maxResults 时立即停止扫描。
func (ac *ACAutomaton) Search(text string, maxResults int) []MatchResult {
	if maxResults <= 0 {
		return nil
	}

	results := make([]MatchResult, 0, maxResults)
	seen := make(map[uint]bool, maxResults)

	node := ac.root
	for _, ch := range text {
		// 沿着 fail 链找到能转移的节点
		for node != ac.root {
			if _, ok := node.children[ch]; ok {
				break
			}
			node = node.fail
		}
		if child, ok := node.children[ch]; ok {
			node = child
		}
		// node == ac.root 表示没有匹配，继续扫描

		// 收集当前节点的所有 output
		if len(node.output) > 0 {
			for _, patternIdx := range node.output {
				tagId := ac.patternIds[patternIdx]
				if !seen[tagId] {
					seen[tagId] = true
					results = append(results, MatchResult{TagId: tagId})
					if len(results) >= maxResults {
						return results
					}
				}
			}
		}
	}

	return results
}
