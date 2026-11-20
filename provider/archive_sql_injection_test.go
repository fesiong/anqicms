package provider

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"kandaoni.com/anqicms/library"
)

// TestGetArchiveListKeywordInjection 回归 GitHub issue #93：
// search q / module q / GraphQL archives.q 最终都汇入 GetArchiveList 的 ops。
// 反斜杠 + 单引号 + OR 1=1 曾经因为 ToSQL 内联字符串 + Raw 执行而逃逸出
// LIKE 字面量，返回全站文章。参数化后它必须被当成普通字符串、匹配 0 条。
func TestGetArchiveListKeywordInjection(t *testing.T) {
	dbSite, err := GetDBWebsiteInfo(1)
	if err != nil {
		t.Skipf("本地无可用站点库，跳过：%v", err)
	}
	InitWebsite(dbSite)
	w := GetWebsite(1)
	if w == nil || w.DB == nil {
		t.Skip("站点未初始化，跳过")
	}
	w.Cache = library.InitMemoryCache()

	cases := []struct {
		name     string
		keyword  string
		wantZero bool
	}{
		{"永真注入", `666\' OR 1=1#`, true},
		{"永假注入", `x\' AND 1=2#`, true},
		{"普通词", `zzzznotexist9999`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kw := tc.keyword
			archives, _, err := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				return tx.Where("`title` like ?", "%"+kw+"%")
			}, "archives.`created_time` desc", 1, 100)
			if err != nil {
				t.Fatalf("GetArchiveList 出错：%v", err)
			}
			if tc.wantZero && len(archives) != 0 {
				t.Fatalf("注入词 %q 返回了 %d 条，疑似仍可注入（issue #93 回归）", kw, len(archives))
			}
		})
	}
}

// TestDryRunSQLKeepsPlaceholders 是 dryRunSQL 的正向不变量：
// 它返回的 SQL 必须保留 ? 占位符，且 Vars 承载原始输入（含反斜杠/引号），
// 绝不能出现内联的字面量。任何把 dryRunSQL 改回 ToSQL 的行为都会让此测试变红。
func TestDryRunSQLKeepsPlaceholders(t *testing.T) {
	dbSite, err := GetDBWebsiteInfo(1)
	if err != nil {
		t.Skipf("本地无可用站点库，跳过：%v", err)
	}
	InitWebsite(dbSite)
	w := GetWebsite(1)
	if w == nil || w.DB == nil {
		t.Skip("站点未初始化，跳过")
	}

	payload := `666\' OR 1=1#`
	sql, vars := w.dryRunSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Model(&struct{}{}).Table("archives").Where("`title` like ?", "%"+payload+"%").Find(&[]struct{}{})
	})
	if !strings.Contains(sql, "?") {
		t.Fatalf("SQL 未保留占位符（疑似被内联）：%s", sql)
	}
	if strings.Contains(sql, "OR 1=1") {
		t.Fatalf("SQL 里出现了注入载荷字面量，说明被内联：%s", sql)
	}
	if len(vars) != 1 || !strings.Contains(strings.ReplaceAll(vars[0].(string), "\\", ""), payload[:3]) {
		t.Fatalf("Vars 未承载原始输入：%#v", vars)
	}
}
