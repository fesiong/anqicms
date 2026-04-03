package tags

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/provider"
)

type tagSqlNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

// QueryParam 定义安全的查询参数
type QueryParam struct {
	Field string
	Op    string // 只允许: =, !=, >, <, >=, <=, LIKE
	Value interface{}
}

func (node *tagSqlNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}
	var table string
	if args["table"] != nil {
		table = args["table"].String()
	}
	if table == "" {
		// 没有表，不能运行
		return nil
	}

	limit := 0
	offset := 0
	if args["limit"] != nil {
		limitArgs := strings.Split(args["limit"].String(), ",")
		if len(limitArgs) == 2 {
			offset, _ = strconv.Atoi(limitArgs[0])
			limit, _ = strconv.Atoi(limitArgs[1])
		} else if len(limitArgs) == 1 {
			limit, _ = strconv.Atoi(limitArgs[0])
		}
		if limit > currentSite.Content.MaxLimit {
			limit = currentSite.Content.MaxLimit
		}
	}
	if limit < 1 {
		limit = 1
	}
	if args["offset"] != nil {
		offset = args["offset"].Integer()
	}
	single := false
	if args["single"] != nil {
		single = args["single"].Bool()
		if single {
			limit = 1
		}
	}
	order := ""
	if args["order"] != nil {
		tmpOrder := args["order"].String()
		tmpOrder = provider.ParseOrderBy(tmpOrder, "")
		if tmpOrder != "" {
			order = tmpOrder
		}
	}
	// 定义允许的SQL操作符和函数
	where := ""
	var whereParams []interface{}
	if args["where"] != nil {
		rawWhere := args["where"].String()
		parsedWhere, params, ok := parseWhereParams(rawWhere)
		if !ok {
			// 非法SQL，但不返回错误
			return nil
		}
		where = parsedWhere
		whereParams = params
	}
	// 验证 table 是否存在
	if !currentSite.DB.Migrator().HasTable(table) {
		return nil
	}

	if single {
		// 获取一条数据
		var result map[string]any
		dbQuery := currentSite.DB.Table(table)
		if where != "" && len(whereParams) > 0 {
			dbQuery = dbQuery.Where(where, whereParams...)
		}
		if order != "" {
			dbQuery = dbQuery.Order(order)
		}
		err2 := dbQuery.Take(&result).Error
		if err2 != nil {
			return nil
		}
		ctx.Private[node.name] = result
	} else {
		// 获取多条数据
		var resultList []map[string]any
		dbQuery := currentSite.DB.Table(table)
		if where != "" && len(whereParams) > 0 {
			dbQuery = dbQuery.Where(where, whereParams...)
		}
		dbQuery = dbQuery.Limit(limit).Offset(offset)
		if order != "" {
			dbQuery = dbQuery.Order(order)
		}
		dbQuery.Find(&resultList)
		ctx.Private[node.name] = resultList
	}

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagSqlParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagSqlNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("sql-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed sql-tag arguments.", nil)
	}

	wrapper, endtagargs, err := doc.WrapUntilTag("endsql")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endsql' must equal to 'sql'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endsql'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}

// parseWhereParams 解析where参数为安全的查询条件
func parseWhereParams(whereStr string) (string, []interface{}, bool) {
	if whereStr == "" {
		return "", nil, true
	}

	// 严格的验证：只允许 field=value 或 field op value 格式，用 AND 或 OR 连接
	// 示例: id=1 AND status='active' OR category LIKE 'news%'

	// 1. 基础验证
	if len(whereStr) > 200 {
		return "", nil, false
	}

	// 2. 检查危险关键词（包括各种变体）
	dangerousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bselect\b`),
		regexp.MustCompile(`(?i)\binsert\b`),
		regexp.MustCompile(`(?i)\bdelete\b`),
		regexp.MustCompile(`(?i)\bupdate\b`),
		regexp.MustCompile(`(?i)\bdrop\b`),
		regexp.MustCompile(`(?i)\bcreate\b`),
		regexp.MustCompile(`(?i)\balter\b`),
		regexp.MustCompile(`(?i)\bexec(ute)?\b`),
		regexp.MustCompile(`(?i)\bunion\b`),
		regexp.MustCompile(`(?i)\bjoin\b`),
		regexp.MustCompile(`(?i)\bhaving\b`),
		regexp.MustCompile(`(?i)\bgroup\s+by\b`),
		regexp.MustCompile(`(?i)\border\s+by\b`),
		regexp.MustCompile(`(?i)\blimit\b`),
		regexp.MustCompile(`(?i)\boffset\b`),
		regexp.MustCompile(`(?i)\bin\s*\(`), // 防止 IN 子查询
		regexp.MustCompile(`(?i)\bbetween\b`),
		regexp.MustCompile(`(?i)\bexists\s*\(`),
		regexp.MustCompile(`(?i)\bcase\b`),
		regexp.MustCompile(`(?i)\bwhen\b`),
		regexp.MustCompile(`(?i)\bthen\b`),
		regexp.MustCompile(`(?i)\belse\b`),
		regexp.MustCompile(`(?i)\bend\b`),
		regexp.MustCompile(`(?i)\bprocedure\b`),
		regexp.MustCompile(`(?i)\bfunction\b`),
		regexp.MustCompile(`(?i)\btrigger\b`),
		regexp.MustCompile(`(?i)\bview\b`),
		regexp.MustCompile(`(?i)\bindex\b`),
		regexp.MustCompile(`--`),                 // SQL注释
		regexp.MustCompile(`/\*`),                // 多行注释开始
		regexp.MustCompile(`\*/`),                // 多行注释结束
		regexp.MustCompile(`#`),                  // MySQL注释
		regexp.MustCompile(`;`),                  // 查询分隔符
		regexp.MustCompile(`\x00`),               // 空字符
		regexp.MustCompile(`\x1a`),               // Ctrl-Z
		regexp.MustCompile(`\x0d\x0a|\x0a|\x0d`), // 换行符
	}

	// 移除多余空格（保留单个空格）
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(whereStr, " ")

	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(normalized) {
			return "", nil, false
		}
	}

	// 3. 使用安全的模式匹配
	// 允许的格式: field op value [AND|OR field op value]*
	// op 只能是: =, !=, >, <, >=, <=, LIKE
	// value 可以是: 数字 或 '字符串' (字符串内不能有单引号)

	// 分割条件
	conditions := regexp.MustCompile(`\s+(AND|OR)\s+`).Split(normalized, -1)
	if len(conditions) == 0 {
		return "", nil, false
	}

	// 验证每个条件
	conditionPattern := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\s*(=|!=|>|<|>=|<=|LIKE)\s*('[^']*'|\d+(?:\.\d+)?)$`)

	var whereClauses []string
	var params []interface{}

	for _, cond := range conditions {
		cond = strings.TrimSpace(cond)
		if cond == "" {
			continue
		}

		matches := conditionPattern.FindStringSubmatch(cond)
		if matches == nil || len(matches) != 4 {
			return "", nil, false
		}

		field := matches[1]
		op := matches[2]
		valueStr := matches[3]

		// 验证字段名
		if !isValidFieldName(field) {
			return "", nil, false
		}

		// 解析值
		var value interface{}
		if strings.HasPrefix(valueStr, "'") && strings.HasSuffix(valueStr, "'") {
			// 字符串值
			value = strings.Trim(valueStr, "'")
			// 额外的字符串安全检查
			if strings.Contains(value.(string), "'") {
				return "", nil, false
			}
		} else {
			// 数字值
			if strings.Contains(valueStr, ".") {
				// 浮点数
				if f, err := strconv.ParseFloat(valueStr, 64); err != nil {
					return "", nil, false
				} else {
					value = f
				}
			} else {
				// 整数
				if i, err := strconv.ParseInt(valueStr, 10, 64); err != nil {
					return "", nil, false
				} else {
					value = i
				}
			}
		}

		// 添加到查询条件
		whereClauses = append(whereClauses, fmt.Sprintf("%s %s ?", field, op))
		params = append(params, value)
	}

	if len(whereClauses) == 0 {
		return "", nil, false
	}

	whereClause := strings.Join(whereClauses, " %s ") // 临时占位符
	// 重新用AND/OR连接条件
	whereClause = strings.Replace(normalized, normalized, whereClause, 1)

	// 手动构建正确的WHERE语句
	logicOperators := regexp.MustCompile(`\s+(AND|OR)\s+`).FindAllString(normalized, -1)

	whereClause = whereClauses[0]
	for i, logicOp := range logicOperators {
		if i+1 < len(whereClauses) {
			whereClause += logicOp + whereClauses[i+1]
		}
	}

	return whereClause, params, true
}

// isValidFieldName 验证字段名是否合法
func isValidFieldName(fieldName string) bool {
	if fieldName == "" || len(fieldName) > 64 {
		return false
	}

	// 字段名只能包含字母、数字、下划线，且以字母或下划线开头
	fieldNamePattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	return fieldNamePattern.MatchString(fieldName)
}
