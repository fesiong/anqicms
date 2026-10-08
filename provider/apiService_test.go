package provider

import "testing"

// 排序子句只允许列名（可带反引号、可带表前缀）+ asc/desc，以及 RAND()。
// 引号、括号、注释符、|| 必须整条丢弃，而不是「看起来像函数就放行」。
func TestParseOrderBy(t *testing.T) {
	cases := []struct {
		name   string
		order  string
		prefix string
		want   string
	}{
		// --- 正常用法（模板与内部默认排序依赖这些形式） ---
		{"默认带前缀列", "created_time desc", "archives", "archives.created_time desc"},
		{"反引号列", "archives.`created_time` desc", "archives", "archives.`created_time` desc"},
		{"反引号无表名", "`sort` desc", "archives", "archives.`sort` desc"},
		{"多列", "sort desc, created_time desc", "archives", "archives.sort desc, archives.created_time desc"},
		{"大小写方向", "id ASC", "", "id ASC"},
		{"无前缀", "id desc", "", "id desc"},
		{"点号前缀去重", "id desc", "archives.", "archives.id desc"},
		{"随机", "Rand()", "archives", "RAND()"},
		{"随机无括号", "rand", "archives", "RAND()"},

		// --- 注入面 ---
		{"空", "", "archives", ""},
		{"仅空白", "   ", "archives", ""},
		// 2026-09-29 攻击流量里的真实 payload（用 /**/ 代替空格、SUBSTR..FROM..FOR 规避逗号）
		{"事件payload",
			"length(''||(case/**/when/**/(((select/**/ascii(substr(user_name/**/from/**/4/**/for/**/1))/**/from/**/admins/**/limit/**/1/**/offset/**/0)>78))/**/then/**/sleep(0.3)/**/else/**/0/**/end)||'x')",
			"archives", ""},
		{"旧白名单函数拼接", "length(''||(case when 1=1 then sleep(3) else 0 end)||'x')", "archives", ""},
		{"单引号包裹的表达式", "'a'='a'", "archives", ""},
		{"sleep 直传", "sleep(3)", "", ""},
		{"if 函数", "if(1=1,sleep(3),0)", "", ""},
		{"updatexml 报错注入", "updatexml(1,concat(0x7e,(select user())),1)", "", ""},
		{"分号堆叠", "id; drop table admins", "", ""},
		{"注释符", "id-- desc", "", ""},
		{"行尾注释", "id /* x */ desc", "", ""},
		{"union", "id union select password from admins", "", ""},
		{"随机加方向", "rand() desc", "archives", ""},
		{"聚合函数不再放行", "Max(id)", "archives", ""},
		{"反引号逃逸", "`id`=`x`", "", ""},
		{"多列混注入", "sort desc,(select 1)", "archives", "archives.sort desc"},
		{"表名伪装", "from admins", "", ""},
		{"三词以上", "id desc nulls first", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseOrderBy(c.order, c.prefix); got != c.want {
				t.Errorf("ParseOrderBy(%q, %q) = %q, want %q", c.order, c.prefix, got, c.want)
			}
		})
	}
}
