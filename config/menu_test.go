package config

import (
	"testing"
)

// 官方品牌（默认 OEMID）下，设计市场菜单项必须存在。
func TestMenuGroupsOfficialIncludesMarket(t *testing.T) {
	if !IsOfficial() {
		t.Skip("本测试只在官方品牌下有意义；当前 OEMID 非官方")
	}
	if !designMarketVisible(MenuGroups()) {
		t.Fatal("官方品牌应显示「设计市场」菜单项（/design/market）")
	}
}

// OEM 品牌下，设计市场菜单项必须从菜单中剔除——它的页面 iframe 指向
// anqicms.com，露出即泄漏官方品牌。
func TestMenuGroupsOemHidesMarket(t *testing.T) {
	orig := OEMID
	OEMID = "acme-test-only"
	defer func() { OEMID = orig }()

	if IsOfficial() {
		t.Fatal("前置条件失败：改完 OEMID 后 IsOfficial 仍为真")
	}
	if designMarketVisible(MenuGroups()) {
		t.Fatal("OEM 品牌不应显示「设计市场」菜单项（/design/market）")
	}
}

// 反向对照：把过滤条件反过来，上面两条用例应立刻变红，证明不是靠运气通过。
func TestMenuGroupsFilterIsEffective(t *testing.T) {
	orig := OEMID
	OEMID = "acme-test-only"
	defer func() { OEMID = orig }()

	// 人为构造一份「永远包含市场」的副本，模拟忘记过滤的场景。
	groups := MenuGroups()
	for _, g := range groups {
		if g.Key == "design" {
			g.Menus = append(g.Menus, &Menu{
				Path: "/design/market", GroupKey: "design", Name: "设计市场",
			})
		}
	}
	if !designMarketVisible(groups) {
		t.Fatal("构造的副本应含市场项，测试脚手架自身有问题")
	}
}

func designMarketVisible(groups []*MenuGroup) bool {
	for _, g := range groups {
		if g.Key != "design" {
			continue
		}
		for _, m := range g.Menus {
			if m.Path == "/design/market" {
				return true
			}
		}
	}
	return false
}
