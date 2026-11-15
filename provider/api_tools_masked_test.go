package provider

import (
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// TestFillMaskedFromCurrent 锁住「掩码回写」的三种现值语义。
//
// 背景：短信 / 发信这类配置端点的 GET 自己就会脱敏，回查到的现值也是
// ********。旧实现把它当成「拿不到现值」直接报错，导致标准的
// 「读全量 → 改一个 → 整包写回」流程必然失败。正确语义是把该键从参数里
// 删掉（= 未改动），并递归删掉因此变空的父对象。
func TestFillMaskedFromCurrent(t *testing.T) {
	t.Run("现值是真值→替换成真值", func(t *testing.T) {
		out, leftover := fillMaskedFromCurrent(
			map[string]any{"password": intent.MaskedSecret},
			map[string]any{"password": "real-secret"},
		)
		if len(leftover) != 0 {
			t.Fatalf("有真值不应报 leftover，实得 %v", leftover)
		}
		if out["password"] != "real-secret" {
			t.Fatalf("掩码应被替换成真值，实得 %#v", out["password"])
		}
	})

	t.Run("现值是掩码→删键且无 leftover", func(t *testing.T) {
		out, leftover := fillMaskedFromCurrent(
			map[string]any{"password": intent.MaskedSecret, "provider": "aliyun"},
			map[string]any{"password": intent.MaskedSecret},
		)
		if len(leftover) != 0 {
			t.Fatalf("现值是掩码表示「库里确有值」，不该报 leftover，实得 %v", leftover)
		}
		if _, ok := out["password"]; ok {
			t.Fatalf("现值是掩码时必须删键（= 未改动），实得 %#v", out["password"])
		}
		if out["provider"] != "aliyun" {
			t.Fatalf("非掩码字段不应被改动，实得 %#v", out)
		}
	})

	t.Run("现值缺失→仍报 leftover", func(t *testing.T) {
		out, leftover := fillMaskedFromCurrent(
			map[string]any{"password": intent.MaskedSecret},
			map[string]any{},
		)
		if len(leftover) != 1 || leftover[0] != "password" {
			t.Fatalf("现值缺失时应报 leftover=[password]，实得 %v", leftover)
		}
		if out["password"] != intent.MaskedSecret {
			t.Fatalf("拿不到真值时应原样保留掩码（由上层报错），实得 %#v", out["password"])
		}
	})

	t.Run("现值是空串→仍报 leftover", func(t *testing.T) {
		_, leftover := fillMaskedFromCurrent(
			map[string]any{"password": intent.MaskedSecret},
			map[string]any{"password": ""},
		)
		if len(leftover) != 1 || leftover[0] != "password" {
			t.Fatalf("空串等于没有可沿用的真值，应报 leftover，实得 %v", leftover)
		}
	})

	t.Run("嵌套对象内两字段都被删空→父对象被删除", func(t *testing.T) {
		out, leftover := fillMaskedFromCurrent(
			map[string]any{
				"aliyun": map[string]any{
					"domestic": map[string]any{
						"access_key_id":     intent.MaskedSecret,
						"access_key_secret": intent.MaskedSecret,
					},
				},
				"provider": "aliyun",
			},
			map[string]any{
				"aliyun": map[string]any{
					"domestic": map[string]any{
						"access_key_id":     intent.MaskedSecret,
						"access_key_secret": intent.MaskedSecret,
						"sign_name":         "旧签名",
					},
				},
			},
		)
		if len(leftover) != 0 {
			t.Fatalf("现值都是掩码，不该报 leftover，实得 %v", leftover)
		}
		if _, ok := out["aliyun"]; ok {
			t.Fatalf("嵌套对象的键全被删空时父对象必须一并删除，实得 %#v", out["aliyun"])
		}
		if out["provider"] != "aliyun" {
			t.Fatalf("顶层非掩码字段不应被改动，实得 %#v", out)
		}
	})

	t.Run("嵌套对象仍有保留键→父对象保留", func(t *testing.T) {
		out, leftover := fillMaskedFromCurrent(
			map[string]any{
				"aliyun": map[string]any{
					"domestic": map[string]any{
						"access_key_id": intent.MaskedSecret,
						"sign_name":     "新签名",
					},
				},
			},
			map[string]any{
				"aliyun": map[string]any{
					"domestic": map[string]any{"access_key_id": intent.MaskedSecret},
				},
			},
		)
		if len(leftover) != 0 {
			t.Fatalf("不应报 leftover，实得 %v", leftover)
		}
		dm, _ := out["aliyun"].(map[string]any)["domestic"].(map[string]any)
		if dm["sign_name"] != "新签名" {
			t.Fatalf("改动字段丢失：%#v", out)
		}
		if _, ok := dm["access_key_id"]; ok {
			t.Fatalf("掩码键应被删除，实得 %#v", dm)
		}
	})
}

// TestRestoreMaskedParamsAcceptsMaskedCurrent 用真实的 restoreMaskedParams +
// 桩出来的 GET 端点跑一遍「读全量 → 改一个 → 写回」：GET 返回的现值本身是掩码
// （与短信 / 发信端点一致），参数里原样回传掩码不应再报错。
func TestRestoreMaskedParamsAcceptsMaskedCurrent(t *testing.T) {
	app := iris.New()
	manage := app.Party("/system/api")
	manage.Get("/masked-setting", func(ctx iris.Context) {
		ctx.JSON(iris.Map{
			"code": 0,
			"msg":  "",
			"data": iris.Map{
				"provider": "aliyun",
				"aliyun": iris.Map{
					"domestic": iris.Map{
						"access_key_id":     intent.MaskedSecret,
						"access_key_secret": intent.MaskedSecret,
						"sign_name":         "旧签名",
					},
				},
			},
		})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	SetInvokerApplicationForTest(app)
	t.Cleanup(func() { SetInvokerApplicationForTest(nil) })

	site := &Website{TokenSecret: "unit-test-secret"}
	params := map[string]any{
		"provider": "twilio",
		"aliyun": map[string]any{
			"domestic": map[string]any{
				"access_key_id":     intent.MaskedSecret,
				"access_key_secret": intent.MaskedSecret,
			},
		},
	}

	out, err := restoreMaskedParams(site, 2, "POST", "/system/api/masked-setting", params)
	if err != nil {
		t.Fatalf("现值是掩码时不应报错（标准写回流程必须可用）：%v", err)
	}
	if out["provider"] != "twilio" {
		t.Fatalf("独立字段的改动丢失：%#v", out)
	}
	if _, ok := out["aliyun"]; ok {
		t.Fatalf("全掩码的嵌套对象应被删除，实得 %#v", out["aliyun"])
	}
}

// TestRestoreMaskedParamsStillRejectsMissingCurrent 反向锁：现值缺失或为空时
// 仍必须报 leftover——那才是真的拿不到值，放行会把 ******** 写进库。
func TestRestoreMaskedParamsStillRejectsMissingCurrent(t *testing.T) {
	app := iris.New()
	manage := app.Party("/system/api")
	manage.Get("/empty-setting", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"code": 0, "msg": "", "data": iris.Map{"password": ""}})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("Build 失败: %v", err)
	}
	SetInvokerApplicationForTest(app)
	t.Cleanup(func() { SetInvokerApplicationForTest(nil) })

	site := &Website{TokenSecret: "unit-test-secret"}
	_, err := restoreMaskedParams(site, 2, "POST", "/system/api/empty-setting",
		map[string]any{"password": intent.MaskedSecret})
	if err == nil {
		t.Fatal("现值为空串时必须拒绝，否则会把掩码写进配置")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Fatalf("错误信息应点名残留字段，实得：%v", err)
	}
}
