package provider

import (
	"testing"
)

func TestOrderByFilter(t *testing.T) {
	tests := []struct {
		name     string
		order    string
		prefix   string
		expected string
	}{
		// 基础测试
		{
			name:     "空输入",
			order:    "",
			prefix:   "",
			expected: "",
		},
		{
			name:     "空输入带前缀",
			order:    "",
			prefix:   "users",
			expected: "",
		},

		// 单字段排序
		{
			name:     "单字段升序",
			order:    "id asc",
			prefix:   "",
			expected: "id asc",
		},
		{
			name:     "单字段降序",
			order:    "created_at desc",
			prefix:   "",
			expected: "created_at desc",
		},
		{
			name:     "单字段无方向",
			order:    "name",
			prefix:   "",
			expected: "name",
		},

		// 多字段排序
		{
			name:     "多字段排序",
			order:    "id asc, name desc, created_at",
			prefix:   "",
			expected: "id asc, name desc, created_at",
		},
		{
			name:     "多字段带空格",
			order:    "  id   asc  ,  name   desc  ",
			prefix:   "",
			expected: "id asc, name desc",
		},

		// 带表名前缀的测试
		{
			name:     "单字段带前缀",
			order:    "id asc",
			prefix:   "users",
			expected: "users.id asc",
		},
		{
			name:     "多字段带前缀",
			order:    "id asc, name desc",
			prefix:   "products",
			expected: "products.id asc, products.name desc",
		},
		{
			name:     "已包含表名前缀",
			order:    "users.id asc, orders.created_at desc",
			prefix:   "",
			expected: "users.id asc, orders.created_at desc",
		},
		{
			name:     "已包含表名前缀且传入前缀",
			order:    "users.id asc",
			prefix:   "orders",
			expected: "users.id asc", // 不重复添加前缀
		},

		// 函数调用测试
		{
			name:     "RAND函数",
			order:    "rand()",
			prefix:   "",
			expected: "RAND()",
		},
		{
			name:     "RAND函数带方向",
			order:    "rand() desc",
			prefix:   "",
			expected: "RAND() desc",
		},
		{
			name:     "RAND函数大小写不敏感",
			order:    "RAND(), Rand(), ranD()",
			prefix:   "",
			expected: "RAND(), RAND(), RAND()",
		},
		{
			name:     "其他SQL函数",
			order:    "UPPER(name) desc, LENGTH(description) asc",
			prefix:   "",
			expected: "UPPER(name) desc, LENGTH(description) asc",
		},
		{
			name:     "带参数的SQL函数",
			order:    "SUBSTR(name, 1, 3), CONCAT(first_name, ' ', last_name)",
			prefix:   "",
			expected: "SUBSTR(name, 1, 3), CONCAT(first_name, ' ', last_name)",
		},
		{
			name:     "带前缀的SQL函数",
			order:    "UPPER(name) desc",
			prefix:   "customers",
			expected: "UPPER(customers.name) desc",
		},

		// 混合测试
		{
			name:     "混合字段类型",
			order:    "id asc, RAND(), UPPER(name) desc, created_at",
			prefix:   "products",
			expected: "products.id asc, RAND(), UPPER(products.name) desc, products.created_at",
		},
		{
			name:     "复杂混合",
			order:    "products.id asc, orders.total desc, RAND(), UPPER(customers.name)",
			prefix:   "",
			expected: "products.id asc, orders.total desc, RAND(), UPPER(customers.name)",
		},

		// 边界和错误情况
		{
			name:     "无效字段名",
			order:    "id, name-with-dash, valid_field",
			prefix:   "",
			expected: "id, valid_field",
		},
		{
			name:     "SQL注入尝试-注释",
			order:    "id; DROP TABLE users --",
			prefix:   "",
			expected: "",
		},
		{
			name:     "SQL注入尝试-union",
			order:    "id UNION SELECT * FROM users",
			prefix:   "",
			expected: "",
		},
		{
			name:     "SQL注入尝试-括号",
			order:    "id) OR (1=1",
			prefix:   "",
			expected: "",
		},
		{
			name:     "无效方向",
			order:    "id random_direction, name asc",
			prefix:   "",
			expected: "id, name asc",
		},
		{
			name:     "多个逗号",
			order:    "id,,,name",
			prefix:   "",
			expected: "id, name",
		},
		{
			name:     "额外参数",
			order:    "id asc extra_param, name desc",
			prefix:   "",
			expected: "id asc, name desc",
		},

		// 数字和字符串参数
		{
			name:     "数字参数函数",
			order:    "SUBSTRING(name FROM 2 FOR 3)",
			prefix:   "",
			expected: "", // 不支持FROM...FOR语法
		},
		{
			name:     "字符串参数函数",
			order:    "CONCAT('Mr. ', name)",
			prefix:   "",
			expected: "CONCAT('Mr. ', name)",
		},

		// 特殊字符处理
		{
			name:     "带下划线前缀",
			order:    "_id asc, __name desc",
			prefix:   "",
			expected: "_id asc, __name desc",
		},
		{
			name:     "数字开头字段",
			order:    "1column, column2",
			prefix:   "",
			expected: "column2", // 数字开头不是有效标识符
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OrderByFilter(tt.order, tt.prefix)
			if result != tt.expected {
				t.Errorf("OrderByFilter(%q, %q) = %q, want %q",
					tt.order, tt.prefix, result, tt.expected)
			}
		})
	}
}

func TestOrderByFilter_EdgeCases(t *testing.T) {
	// 表名前缀边界测试
	edgeTests := []struct {
		name     string
		order    string
		prefix   string
		expected string
	}{
		{
			name:     "前缀带点号",
			order:    "id asc",
			prefix:   "users.",
			expected: "users.id asc",
		},
		{
			name:     "前缀带多个点号",
			order:    "id asc",
			prefix:   "schema.users.",
			expected: "schema.users.id asc",
		},
		{
			name:     "复杂表名前缀",
			order:    "id asc, name desc",
			prefix:   "my_schema.my_table",
			expected: "my_schema.my_table.id asc, my_schema.my_table.name desc",
		},
		{
			name:     "字段名带点号但不是表名",
			order:    "user.id asc", // 但user.id是完整表名.字段名格式
			prefix:   "users",
			expected: "user.id asc", // 保持原样
		},
		{
			name:     "函数带表名前缀参数",
			order:    "CONCAT(users.first_name, users.last_name)",
			prefix:   "",
			expected: "CONCAT(users.first_name, users.last_name)",
		},
		{
			name:     "嵌套函数",
			order:    "UPPER(CONCAT(first_name, last_name))",
			prefix:   "",
			expected: "", // 不支持嵌套函数
		},
		{
			name:     "字段名包含SQL关键字但不匹配",
			order:    "brand asc, random_column desc",
			prefix:   "",
			expected: "brand asc, random_column desc",
		},
	}

	for _, tt := range edgeTests {
		t.Run(tt.name, func(t *testing.T) {
			result := OrderByFilter(tt.order, tt.prefix)
			if result != tt.expected {
				t.Errorf("OrderByFilter(%q, %q) = %q, want %q",
					tt.order, tt.prefix, result, tt.expected)
			}
		})
	}
}

func TestOrderByFilter_Performance(t *testing.T) {
	// 性能测试：多次调用确保没有panic
	orders := []string{
		"id asc",
		"name desc",
		"created_at, updated_at desc",
		"RAND()",
		"UPPER(name) asc",
	}

	prefixes := []string{"", "users", "products", "orders"}

	for i := 0; i < 1000; i++ {
		for _, order := range orders {
			for _, prefix := range prefixes {
				result := OrderByFilter(order, prefix)
				_ = result // 确保函数可以正常处理
			}
		}
	}
}

func TestIsValidSQLFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"RAND()", true},
		{"rand()", true},
		{"UPPER(name)", true},
		{"LENGTH(description)", true},
		{"CONCAT(first, last)", true},
		{"SUBSTR(name, 1, 3)", true},
		{"COALESCE(name, 'Unknown')", true},
		{"INVALID_FUNC()", false},       // 不在白名单中
		{"UPPER(CONCAT(a,b))", false},   // 嵌套函数
		{"RAND(", false},                // 括号不匹配
		{"UPPER(name", false},           // 缺少右括号
		{"UPPER()); DROP TABLE", false}, // 注入尝试
		{"UPPER('test')", true},
		{"SUBSTR(name, 1)", true},
		{"SUBSTR(name, 'pattern')", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isValidSQLFunction(tt.input)
			if result != tt.expected {
				t.Errorf("isValidSQLFunction(%q) = %v, want %v",
					tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"00123", true},
		{"", false},
		{"12.3", false},
		{"12a", false},
		{" 123", false},
		{"123 ", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumeric(tt.input)
			if result != tt.expected {
				t.Errorf("isNumeric(%q) = %v, want %v",
					tt.input, result, tt.expected)
			}
		})
	}
}

// 基准测试
func BenchmarkOrderByFilter(b *testing.B) {
	testCases := []struct {
		order  string
		prefix string
	}{
		{"id asc", "users"},
		{"name desc, created_at asc", "products"},
		{"RAND()", ""},
		{"UPPER(name) desc, LENGTH(description)", "customers"},
		{"id, name, email, created_at, updated_at", "orders"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			OrderByFilter(tc.order, tc.prefix)
		}
	}
}
