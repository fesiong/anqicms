package provider

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"log"
	"regexp"
	"strings"
	"unicode"
)

func (w *Website) DeleteCacheIndex() {
	w.RemoveHtmlCache("/")
}

var (
	fieldNameRegex  = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	tableFieldRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+\.[a-zA-Z0-9_]+$`)
	sqlFuncRegex    = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*\(.*\)$`)
)

func OrderByFilter(order string, prefix string) string {
	if order == "" {
		return ""
	}
	if prefix != "" && strings.HasSuffix(prefix, ".") {
		prefix = prefix[:len(prefix)-1]
	}

	orders := strings.Split(order, ",")
	validOrders := make([]string, 0, len(orders))

	for _, o := range orders {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}

		// 分割字段和排序方向
		fields := strings.Fields(o)
		if len(fields) == 0 {
			continue
		}

		fieldName := fields[0]

		processedField, isValid := processFieldName(fieldName, prefix)
		if !isValid {
			continue // 跳过无效字段
		}
		orderStr := processedField

		// 检查排序方向
		if len(fields) > 1 {
			direction := strings.ToUpper(fields[1])
			if direction == "ASC" || direction == "DESC" {
				orderStr += " " + fields[1]
			}
			// 如果有额外参数，忽略
		}

		validOrders = append(validOrders, orderStr)
	}

	if len(validOrders) == 0 {
		return ""
	}

	return strings.Join(validOrders, ", ")
}

// processFieldName 处理字段名，应用表名前缀并验证安全性
func processFieldName(fieldName string, prefix string) (string, bool) {
	// 特殊处理 RAND() 函数（大小写不敏感）
	if strings.EqualFold(fieldName, "rand()") || strings.EqualFold(fieldName, "rand") {
		return "RAND()", true
	}
	// 检查是否已经是完整的表名.字段名格式
	if tableFieldRegex.MatchString(fieldName) {
		// 验证表名和字段名都安全
		parts := strings.Split(fieldName, ".")
		if len(parts) == 2 {
			if fieldNameRegex.MatchString(parts[0]) && fieldNameRegex.MatchString(parts[1]) {
				return fieldName, true
			}
		}
		return "", false
	}

	// 检查是否为 SQL 函数调用
	if sqlFuncRegex.MatchString(fieldName) {
		// 验证函数调用安全性
		if isValidSQLFunction(fieldName) {
			return fieldName, true
		}
		return "", false
	}

	// 检查是否为普通字段名
	if fieldNameRegex.MatchString(fieldName) {
		// 如果提供了前缀，则添加前缀
		if prefix != "" {
			return prefix + "." + fieldName, true
		}
		return fieldName, true
	}

	return "", false
}

// isValidSQLFunction 验证 SQL 函数调用的安全性
func isValidSQLFunction(funcCall string) bool {
	// 提取函数名和参数
	openParen := strings.Index(funcCall, "(")
	if openParen == -1 {
		return false
	}

	funcName := funcCall[:openParen]
	params := funcCall[openParen+1 : len(funcCall)-1] // 去掉末尾的 ")"

	// 验证函数名
	if !fieldNameRegex.MatchString(funcName) {
		return false
	}

	// 常见的允许的 SQL 函数白名单
	allowedFunctions := map[string]bool{
		"rand": true, "random": true, "length": true, "char_length": true,
		"upper": true, "lower": true, "substr": true, "substring": true,
		"concat": true, "coalesce": true, "nullif": true,
	}

	funcNameLower := strings.ToLower(funcName)
	if _, ok := allowedFunctions[funcNameLower]; !ok {
		return false
	}

	// 验证参数（可以包含逗号、空格和字段名）
	if params == "" {
		return true // 允许无参数函数
	}

	// 分割多个参数
	paramList := strings.Split(params, ",")
	for _, param := range paramList {
		param = strings.TrimSpace(param)

		// 允许数字字面量
		if isNumeric(param) {
			continue
		}

		// 允许字符串字面量（单引号包围）
		if len(param) >= 2 && param[0] == '\'' && param[len(param)-1] == '\'' {
			// 简单的字符串字面量验证
			continue
		}

		// 允许字段名
		if !fieldNameRegex.MatchString(param) {
			return false
		}
	}

	return true
}

// isNumeric 检查字符串是否为数字
func isNumeric(s string) bool {
	if s == "" {
		return false
	}

	// 检查是否全是数字
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func init() {
	// check what if this server can visit google
	go func() {
		resp, err := library.GetURLData("https://www.google.com", "", 5)
		if err != nil {
			config.GoogleValid = false
		} else {
			config.GoogleValid = true
			log.Println("google-status", resp.StatusCode)
		}
	}()
}
