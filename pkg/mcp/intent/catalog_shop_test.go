package intent

import (
	"context"
	"strings"
	"testing"
)

// catalog_shop_test.go —— 商城（AnqiShop 合并）新增端点的意图覆盖护栏。
//
// 背景：商城分支带来 45 个新端点（优惠券/营销活动/物流/询盘客服/评价/增值服务/
// Google Merchant 同步/愿望清单），合并时只补了 domain.go 的域映射，
// 没有任何意图声明它们——结果是「端点目录里有、模型面里没有」，
// 覆盖率审计把它们统统算作 generic（只能靠通用 api 意图手填路径，
// 而 api 是 DefaultOff，等于完全不可达）。
//
// 这两条测试守的是两个方向：
//  1. 端点 → 意图：45 个端点每个都必须有语义化入口，少一个就红；
//  2. 参数 → 说明：新增意图的每个参数必须有描述，写操作的字段清单
//     必须写进 values 的说明里（免得模型「猜」字段名，猜错会被端点静默忽略）。
func TestShopMallEndpointsAllDeclared(t *testing.T) {
	expected := map[string][]string{
		"commerce_marketing": {
			"GET /plugin/coupon/list",
			"GET /plugin/coupon/detail",
			"POST /plugin/coupon/detail",
			"POST /plugin/coupon/delete",
			"GET /plugin/coupon/archives",
			"POST /plugin/coupon/archives/add",
			"POST /plugin/coupon/archives/delete",
			"GET /plugin/coupon/code/list",
			"POST /plugin/coupon/code/detail",
			"POST /plugin/coupon/code/delete",
			"POST /plugin/coupon/code/make",
			"GET /plugin/activity/list",
			"GET /plugin/activity/detail",
			"POST /plugin/activity/detail",
			"POST /plugin/activity/delete",
		},
		"commerce_fulfillment": {
			"GET /plugin/shipping/setting",
			"POST /plugin/shipping/setting",
			"GET /plugin/communication/order/list",
			"GET /plugin/communication/order/detail",
			"POST /plugin/communication/order/close",
			"POST /plugin/communication/order/images/save",
			"GET /plugin/communication/message/list",
			"POST /plugin/communication/message/send",
			"POST /plugin/communication/message/delete",
			"POST /plugin/communication/quote/send",
			"GET /plugin/communication/setting",
			"POST /plugin/communication/setting",
		},
		"commerce_review": {
			"GET /plugin/order/reviews",
			"GET /plugin/order/review",
			"POST /plugin/order/review/save",
			"POST /plugin/order/review/delete",
			"GET /plugin/order/services",
			"GET /plugin/order/service",
			"POST /plugin/order/service/save",
			"POST /plugin/order/service/delete",
		},
		"seo_merchant": {
			"GET /plugin/googlemerchant/config",
			"POST /plugin/googlemerchant/config",
			"GET /plugin/googlemerchant/auth/url",
			"POST /plugin/googlemerchant/callback",
			"POST /plugin/googlemerchant/account/select",
			"POST /plugin/googlemerchant/sync",
			"POST /plugin/googlemerchant/sync/delete",
			"GET /plugin/googlemerchant/sync/logs",
			"GET /plugin/googlemerchant/sync/statuses",
		},
		// 愿望清单只有一个只读端点，挂在既有 commerce（会员）意图下，不单开意图。
		"commerce": {
			"GET /plugin/wishlist/list",
		},
	}

	declare := map[string]bool{}
	for name, routes := range intentRouteRegistry {
		for _, target := range routes {
			declare[name+"|"+normalizeTarget(target)] = true
		}
	}

	// 先独立核对清单规模，再逐条断言——否则"意图整体缺失"时计数会被 continue 吃掉，
	// 报出"实际 1 个"这种误导性的数字（第一版就踩了）。
	total := 0
	for _, list := range expected {
		total += len(list)
	}
	if total != 45 {
		t.Fatalf("期望的商城端点清单应为 45 个（合并新增的端点数），实际 %d 个——"+
			"清单本身被改动过，请先核对商城合并的端点增量", total)
	}

	for intentName, list := range expected {
		if _, ok := intentRouteRegistry[intentName]; !ok {
			t.Errorf("意图 %s 未声明任何路由（invokeRoutes 未登记）：商城端点因此没有语义化入口", intentName)
			continue
		}
		for _, target := range list {
			if !declare[intentName+"|"+normalizeTarget(target)] {
				t.Errorf("商城端点 %s 未被意图 %s 声明", target, intentName)
			}
		}
	}
	t.Logf("商城端点覆盖：%d 个端点分属 %d 个意图", total, len(expected))
}

// normalizeTarget 把 "GET /plugin/x" 规范化成 "GET /system/api/plugin/x"，
// 与 invokeRoutes 实际调用的路径口径一致。
func normalizeTarget(target string) string {
	parts := strings.Fields(strings.TrimSpace(target))
	if len(parts) != 2 {
		return target
	}
	return strings.ToUpper(parts[0]) + " " + normalizeEndpointPath(parts[1])
}

// TestShopIntentParamsHaveDocs 守住「参数说明补齐」这条要求。
//
// 两个都是静默失败的类型：
//   - 参数没写描述：模型只能从名字猜，猜错不会报错，端点 ReadJSON 直接忽略该字段，
//     调用方看到 code=0 却什么都没改（这正是之前 id→order_id 改名要单列 renames 的原因）；
//   - values 没写字段清单：模型会照着别的意图的习惯编字段名（比如把优惠券的
//     money_type 写成 type），同样静默失效。
func TestShopIntentParamsHaveDocs(t *testing.T) {
	shopIntents := []string{"commerce_marketing", "commerce_fulfillment", "commerce_review", "seo_merchant"}

	byName := map[string]*IntentSpec{}
	for _, s := range IntentCatalog {
		byName[s.Name] = s
	}

	for _, name := range shopIntents {
		spec, ok := byName[name]
		if !ok {
			t.Errorf("意图 %s 不存在于 IntentCatalog", name)
			continue
		}
		for pname, p := range spec.Params {
			if strings.TrimSpace(p.Desc) == "" {
				t.Errorf("意图 %s 的参数 %s 缺少说明", name, pname)
			}
		}
		values, ok := spec.Params["values"]
		if !ok {
			t.Errorf("意图 %s 缺少 values 参数：写操作的字段清单无处声明", name)
			continue
		}
		// 每个会写数据的 action 都必须在 values 的说明里被点名，
		// 否则等于让模型自己猜字段名。
		missing := []string{}
		for action, target := range intentRouteRegistry[name] {
			if !strings.HasPrefix(strings.ToUpper(target), "POST") {
				continue
			}
			if !strings.Contains(values.Desc, action) {
				missing = append(missing, action)
			}
		}
		if len(missing) > 0 {
			t.Errorf("意图 %s 的 values 说明未覆盖这些写操作：%v", name, missing)
		}
	}
}

// TestCouponSaveUsesPartialNotGetFill 校验优惠券保存走 PATCH 而不是「回查补齐」。
//
// 历史：SaveCoupon 原本是逐字段无条件赋值，意图层只能靠 GET 回查把旧值补齐，
// 而 coupon 的详情读的是 **coupon_id**、写的是 **id**，参数名对不上时
// 回查返回 0 条（不报错），保护表就成了摆设 —— 面额被清成 0 的券还挂着
// 已发出去的券码，事后无法从券码侧补回。
//
// 2026-10-29 起 provider.SaveCoupon 改成 PATCH（指针 + UpdateAll/Partial），
// 于是这里应当**只发一次 POST**：不再回查（回查要读旧值，读端点返回
// 派生值时等于用假数据覆盖真值），未传字段由端点自己保持原值。
func TestCouponSaveUsesPartialNotGetFill(t *testing.T) {
	spec := specByNameForTest(t, "commerce_marketing")

	var calls []map[string]any
	inv := func(_ context.Context, name string, args map[string]any) (string, error) {
		if name != "api_invoke" {
			t.Fatalf("应统一经 api_invoke 执行，实际调用了 %s", name)
		}
		calls = append(calls, args)
		return `{"code":0,"data":null}`, nil
	}

	_, err := spec.Compose(context.Background(), map[string]any{
		"action": "coupon_save",
		"id":     7,
		"values": map[string]any{"title": "新标题"},
	}, inv)
	if err != nil {
		t.Fatalf("coupon_save 执行失败: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("端点已是 PATCH 语义，应只有一次 POST，实际 %d 次（多出来的那次是 GET 回查）", len(calls))
	}
	if calls[0]["method"] != "POST" {
		t.Errorf("应为 POST，实际 %v", calls[0]["method"])
	}

	postParams, _ := calls[0]["params"].(map[string]any)
	if postParams["title"] != "新标题" {
		t.Errorf("显式传入的 title 应被尊重，实际 %v", postParams["title"])
	}
	// 关键：除 id/title 外不应凭空出现别的字段。多出来的字段意味着
	// 意图层又去回查并覆盖了调用方没提的值。
	for field := range postParams {
		switch field {
		case "id", "title", "partial":
		default:
			t.Errorf("不该出现未传的字段 %s=%v（端点已是 PATCH，回查补齐会覆盖真值）",
				field, postParams[field])
		}
	}
}

// TestActivitySavePreservesSkus 校验活动保存不会顺手删光规格。
//
// 历史：SaveActivity 在 len(req.Skus)==0 时会 Delete 该活动的全部 ActivitySku，
// 而「没传 skus」与「传空数组」在值类型下是同一个零值 —— 只改个活动标题就会
// 把规格整批删掉，活动价与库存随之失效。当时靠意图层 GET 回查补齐 skus 兜底。
//
// 2026-10-29 起 request.ActivityRequest.Skus 改成 *[]ActivitySku 三态
// （nil=不动 / []=清空 / 非空=同步），端点自己就能区分，于是这里应当
// **只发一次 POST 且不带 skus**：不带 = 不动，不再需要回查。
func TestActivitySavePreservesSkus(t *testing.T) {
	spec := specByNameForTest(t, "commerce_marketing")

	var calls []map[string]any
	inv := func(_ context.Context, _ string, args map[string]any) (string, error) {
		calls = append(calls, args)
		return `{"code":0,"data":null}`, nil
	}

	if _, err := spec.Compose(context.Background(), map[string]any{
		"action": "activity_save",
		"id":     9,
		"values": map[string]any{"title": "改名"},
	}, inv); err != nil {
		t.Fatalf("activity_save 执行失败: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("端点已是 PATCH 语义，应只有一次 POST，实际 %d 次（多出来的那次是 GET 回查）", len(calls))
	}
	postParams, _ := calls[0]["params"].(map[string]any)
	if _, ok := postParams["skus"]; ok {
		t.Errorf("未传 skus 时不该凭空带上 skus（端点的 nil=不动，补了等于替调用方做主）；实际 %v", postParams["skus"])
	}
	if postParams["title"] != "改名" {
		t.Errorf("显式传入的 title 应被尊重，实际 %v", postParams["title"])
	}
}

func specByNameForTest(t *testing.T, name string) *IntentSpec {
	t.Helper()
	for _, s := range IntentCatalog {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("意图 %s 不存在", name)
	return nil
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return -1
}
