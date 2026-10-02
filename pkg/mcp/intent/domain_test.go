package intent

import "testing"

// TestDomainOfPath 锁定有代表性的归域判定。
// 重点是 setting/nav——它与 setting 共享一级 ns，只能靠二级路径区分，
// 若误并回 system 域，structure 域就只剩友链 4 个端点，失去裁剪意义。
func TestDomainOfPath(t *testing.T) {
	cases := []struct {
		path string
		want Domain
	}{
		// 一级 ns 判定
		{"/system/api/setting/system", DomainSystem},
		{"/system/api/archive/list", DomainContent},
		{"/system/api/attachment/list", DomainMedia},
		{"/system/api/admin/list", DomainAccount},
		{"/system/api/statistic/spider", DomainTraffic},
		{"/system/api/design/index", DomainDesign},
		// 二级例外
		{"/system/api/setting/nav/list", DomainStructure},
		// plugin 必须看二级模块，不能按一级 plugin 兜底
		{"/system/api/plugin/push/baidu", DomainSeo},
		{"/system/api/plugin/backup/restore", DomainSiteOps},
		{"/system/api/plugin/user/list", DomainCommerce},
		{"/system/api/plugin/material/list", DomainContentOps},
		{"/system/api/plugin/wechat/menu", DomainChannel},
		{"/system/api/plugin/guestbook/list", DomainInteraction},
		{"/system/api/plugin/link/list", DomainStructure},
		// 相对路径（去掉 /system/api 前缀）同样可判定
		{"plugin/push/baidu", DomainSeo},
		{"setting/nav/list", DomainStructure},
	}
	for _, c := range cases {
		if got := DomainOfPath(c.path); got != c.want {
			t.Errorf("%s 归域错误：期望 %s，实得 %s", c.path, c.want, got)
		}
	}
}

// TestDomainOfPathUnknown 未登记的 ns 必须暴露为 unknown，
// 而不是被兜底成某个域——兜底会让"新模块忘记登记"这个错误静默消失。
func TestDomainOfPathUnknown(t *testing.T) {
	cases := []string{
		"",
		"/",
		"/system/api",
		"/system/api/no_such_ns/foo",
		"/system/api/plugin",                  // plugin 缺二级，无语义
		"/system/api/plugin/no_such_mod/list", // 未登记的二级模块
	}
	for _, p := range cases {
		if got := DomainOfPath(p); got != DomainUnknown {
			t.Errorf("%q 应返回 unknown，实得 %s", p, got)
		}
	}
}

// TestDomainMapValuesAreKnown 映射表里的每个值都必须是已声明的域。
// 防止拼写错误（如 "contnetops"）悄悄新增一个不存在的域。
func TestDomainMapValuesAreKnown(t *testing.T) {
	known := map[Domain]bool{}
	for _, d := range AllDomains() {
		known[d] = true
	}
	check := func(name string, m map[string]Domain) {
		for k, v := range m {
			if !known[v] {
				t.Errorf("%s[%q] = %q 不是已声明的域", name, k, v)
			}
			if v == DomainUnknown {
				t.Errorf("%s[%q] 显式映射到了 unknown，应直接不登记", name, k)
			}
		}
	}
	check("domainByNS", domainByNS)
	check("domainByPlugin", domainByPlugin)
	for ns, sub := range nsSubdomain {
		check("nsSubdomain["+ns+"]", sub)
	}
}

// TestSplitAdminPath 只剥 /system/api 这一层固定前缀，不得吃掉真实 ns。
func TestSplitAdminPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/system/api/plugin/push/list", "plugin/push/list"},
		{"plugin/push/list", "plugin/push/list"},
		{"/system/api/archive/list", "archive/list"},
		{"/system/api", ""},
		{"", ""},
		{"//plugin//push//", "plugin/push"},
	}
	for _, c := range cases {
		segs := splitAdminPath(c.in)
		got := ""
		for i, s := range segs {
			if i > 0 {
				got += "/"
			}
			got += s
		}
		if got != c.want {
			t.Errorf("splitAdminPath(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

// TestDomainLabelCoversAll 每个正式域都要有中文标签，否则对外展示会退化成英文 key。
func TestDomainLabelCoversAll(t *testing.T) {
	for _, d := range AllDomains() {
		if d == DomainUnknown {
			t.Fatal("AllDomains 不应包含 unknown——它不是域，而是映射缺口的哨兵值")
		}
		if DomainLabel(d) == "" {
			t.Errorf("域 %s 缺少中文标签", d)
		}
	}
}
