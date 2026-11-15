package provider

import (
	"errors"
	"testing"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
)

// TestSmsBusinessMessage 钉住前后台共用的「字典即白名单」判据：
//
//	① 真实语言键 → 命中且译文正确；
//	② JSON 解析器原文 → 不命中；
//	③ 含 % 的原文 → 不命中且不被 Sprintf 改写；
//	④ OEM 污染：原文里塞官方品牌名，Tr 会因为 Brandize 改写而误判成键，
//	   统一判据必须仍然判「不命中」——这条是本次缺陷的靶心。
func TestSmsBusinessMessage(t *testing.T) {
	smsLoadTestI18n(t)
	site := &Website{System: &config.SystemConfig{Language: "zh-CN"}}
	translate := site.TplTr

	t.Run("① 真实语言键命中且译文正确", func(t *testing.T) {
		err := errors.New("SmsCodeExpired")
		got, ok := SmsBusinessMessage(site, err)
		if !ok {
			t.Fatal("真实语言键应命中")
		}
		want := translate("SmsCodeExpired")
		if want == "SmsCodeExpired" {
			t.Fatal("locales 缺 SmsCodeExpired 译文，断言会失去意义")
		}
		if got != want {
			t.Fatalf("译文 = %q，期望 %q", got, want)
		}
	})

	t.Run("② JSON解析器原文不命中", func(t *testing.T) {
		err := errors.New("json: cannot unmarshal number into Go struct field x of type string")
		if _, ok := SmsBusinessMessage(site, err); ok {
			t.Fatal("解析器原文不得被判定成语言键")
		}
	})

	t.Run("③ 含百分号的原文不命中且不被改写", func(t *testing.T) {
		raw := "invalid character '%' looking for beginning of value"
		got, ok := SmsBusinessMessage(site, errors.New(raw))
		if ok {
			t.Fatalf("含 %% 的原文不得被判定成语言键（got=%q）", got)
		}
		if got != "" {
			t.Fatalf("不命中时不应返回被 Sprintf 改写过的文本：%q", got)
		}
	})

	t.Run("④ OEM品牌名污染时不得误判成键", func(t *testing.T) {
		oldOEM := config.OEMID
		config.OEMID = "acme-test"
		t.Cleanup(func() { config.OEMID = oldOEM })

		// 这句是典型的内部错误原文（非语言键），但含官方品牌名 anqicms。
		raw := "open /srv/anqicms/config.json: permission denied"
		// 先证明旧判据（Tr = Brandize(tr(x))）确实会被品牌替换污染：
		if site.Tr(raw) == raw {
			t.Fatalf("反向对照失效：OEM=%q 下 Tr 未改写 %q，用例④无法区分两种判据", config.OEMID, raw)
		}
		if _, ok := SmsBusinessMessage(site, errors.New(raw)); ok {
			t.Fatal("OEM 构建下含品牌名的普通错误被误判成语言键（原文会直接送出门）")
		}
	})
}

// smsLoadTestI18n 为 provider 包测试注入真实语言包（cwd 是 provider/）。
func smsLoadTestI18n(t *testing.T) {
	t.Helper()
	app := iris.New()
	if err := app.I18n.Load("../locales/*/*.yml", config.LoadLocales()...); err != nil {
		t.Fatalf("加载 locales 失败：%v", err)
	}
	app.I18n.SetDefault("zh-CN")
	old := I18n
	I18n = app.I18n
	t.Cleanup(func() { I18n = old })
}
