package main

import (
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
)

// 这些用例是 -verify 的反向对照。
//
// 为什么必须要它们：真机上跑一遍 -verify 通过，很可能只是因为当时只登记了
// 一个品牌、禁词集合为空——那种「通过」什么也不能证明。只有构造出第二个
// 品牌，确认泄漏真的被抓到，这套检查才算验证过。

func officialBrand() config.OEMBrand {
	return config.OEMBrand{
		ID:           "anqicms",
		Name:         "AnQiCMS",
		NameCN:       "安企CMS",
		Vendor:       "AnQiCMS",
		Website:      "https://www.anqicms.com",
		AuthBase:     "https://auth.anqicms.com/auth",
		DownloadBase: "https://www.anqicms.com/downloads",
		Binary:       "anqishop",
	}
}

func acmeBrand() config.OEMBrand {
	return config.OEMBrand{
		ID:           "acme",
		Name:         "AcmeCMS",
		NameCN:       "艾克米CMS",
		Vendor:       "Acme Inc",
		Website:      "https://cms.example.com",
		AuthBase:     "https://auth.example.com/auth",
		DownloadBase: "https://cms.example.com/downloads",
		Binary:       "acmecms",
	}
}

// OEM 站点上出现官方品牌 → 必须报出来。
func TestVerifyCatchesOfficialBrandOnOEMSite(t *testing.T) {
	acme := acmeBrand()
	forbidden, allowed := forbiddenTokens(acme, []config.OEMBrand{officialBrand(), acme})

	leaks := scanBody(`<title>安企CMS - 管理后台</title>`, forbidden, allowed)
	if len(leaks) == 0 {
		t.Fatalf("页面里的官方品牌没被抓到，禁词表=%v", forbidden)
	}

	// 同一份检查不能把本品牌自己当成泄漏。
	if got := scanBody(`<title>AcmeCMS - 管理后台</title>`, forbidden, allowed); len(got) != 0 {
		t.Fatalf("本品牌被误判为泄漏：%v", got)
	}
}

// 官方站点上出现 OEM 品牌 → 同样必须报出来（反方向也要成立）。
func TestVerifyCatchesOEMBrandOnOfficialSite(t *testing.T) {
	official := officialBrand()
	forbidden, allowed := forbiddenTokens(official, []config.OEMBrand{official, acmeBrand()})

	leaks := scanBody(`<a href="https://cms.example.com">AcmeCMS</a>`, forbidden, allowed)
	if len(leaks) == 0 {
		t.Fatalf("页面里的 OEM 品牌没被抓到，禁词表=%v", forbidden)
	}
	if got := scanBody(`<a href="https://www.anqicms.com">AnQiCMS</a>`, forbidden, allowed); len(got) != 0 {
		t.Fatalf("官方品牌被误判为泄漏：%v", got)
	}
}

// 本品牌名是别的品牌名的子串时（官方 AnQiCMS，OEM 叫 CMS），不能互相误伤。
func TestForbiddenTokensKeepsSubstringOfOwnBrand(t *testing.T) {
	own := config.OEMBrand{ID: "official", Name: "AnQiCMS", Website: "https://www.anqicms.com"}
	other := config.OEMBrand{ID: "cms", Name: "CMS", Website: "https://cms.example.com"}
	forbidden, _ := forbiddenTokens(own, []config.OEMBrand{own, other})
	for _, token := range forbidden {
		if token == "cms" {
			t.Fatalf("CMS 是 AnQiCMS 的子串，若当成禁词，官方站点每页都会误报：%v", forbidden)
		}
	}
}

// 命中要带上下文，否则运维人员没法判断那是正文还是模板漏出。
func TestScanBodyExcerptHasContext(t *testing.T) {
	forbidden := []string{"anqicms"}
	body := strings.Repeat("填充内容", 40) + "由 AnQiCMS 驱动" + strings.Repeat("后续", 40)
	leaks := scanBody(body, forbidden, nil)
	if len(leaks) != 1 {
		t.Fatalf("期望 1 处泄漏，实际 %d", len(leaks))
	}
	if !strings.Contains(leaks[0].where, "由") || !strings.Contains(leaks[0].where, "驱动") {
		t.Fatalf("上下文不完整：%q", leaks[0].where)
	}
}

// -oem 传错时要能识别出站点真实品牌，否则用户只会看到一堆「缺少本品牌标识」。
func TestDetectBrandFindsOtherBrand(t *testing.T) {
	if got := detectBrand(`<title>AnQiCMS</title>`, []config.OEMBrand{officialBrand()}); got != "anqicms" {
		t.Fatalf("期望识别出 anqicms，实际 %q", got)
	}
	if got := detectBrand(`<title>AcmeCMS</title>`, []config.OEMBrand{acmeBrand()}); got != "acme" {
		t.Fatalf("期望识别出 acme，实际 %q", got)
	}
	// 页面里没有品牌字样时不能瞎猜。
	if got := detectBrand(`<title>管理后台</title>`, []config.OEMBrand{officialBrand()}); got != "" {
		t.Fatalf("不该识别出品牌，实际 %q", got)
	}
}
