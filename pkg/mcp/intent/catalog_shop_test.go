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

// TestCouponSavePreservesFieldsViaAliasedGetKey 校验优惠券保存前的「回查补齐」真的生效。
//
// 两处都容易静默失效：
//  1. coupon 的 GET 详情读的是 **coupon_id**，POST 写的是 **id**。补齐逻辑默认拿 id 去查，
//     参数名对不上时端点只返回 0 条（不报错），保护表就成了摆设；
//  2. SaveCoupon 是逐字段无条件赋值，漏补的字段会被清成 0——面额清零的券还挂着已发出去的券码。
func TestCouponSavePreservesFieldsViaAliasedGetKey(t *testing.T) {
	spec := specByNameForTest(t, "commerce_marketing")

	var calls []map[string]any
	inv := func(_ context.Context, name string, args map[string]any) (string, error) {
		if name != "api_invoke" {
			t.Fatalf("应统一经 api_invoke 执行，实际调用了 %s", name)
		}
		calls = append(calls, args)
		if args["method"] == "GET" {
			return `{"code":0,"data":{"id":7,"title":"旧标题","money":5000,"type":2,"status":1,"range_type":1}}`, nil
		}
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
	if len(calls) != 2 {
		t.Fatalf("应发生 GET+POST 两次调用，实际 %d 次", len(calls))
	}

	getParams, _ := calls[0]["params"].(map[string]any)
	if calls[0]["method"] != "GET" {
		t.Fatalf("第一次调用应为 GET 回查，实际 %v", calls[0]["method"])
	}
	if toInt64(getParams["coupon_id"]) != 7 {
		t.Errorf("回查必须用端点认的参数名 coupon_id，实际 params=%v（用 id 会查不到，保护表静默失效）", getParams)
	}

	postParams, _ := calls[1]["params"].(map[string]any)
	if postParams["title"] != "新标题" {
		t.Errorf("显式传入的 title 应被尊重，实际 %v", postParams["title"])
	}
	for field, want := range map[string]float64{"money": 5000, "type": 2, "status": 1, "range_type": 1} {
		if got := toFloat(postParams[field]); got != want {
			t.Errorf("未传的 %s 应从回查结果补齐为 %v，实际 %v（会被清零）", field, want, postParams[field])
		}
	}
}

// TestActivitySavePreservesSkus 校验活动保存不会顺手删光规格。
//
// SaveActivity 在 len(req.Skus)==0 时会 Delete 该活动的全部 ActivitySku。
// 也就是说「只改个活动标题」如果不补齐 skus，规格会整批消失，活动价与库存随之失效。
func TestActivitySavePreservesSkus(t *testing.T) {
	spec := specByNameForTest(t, "commerce_marketing")

	var calls []map[string]any
	inv := func(_ context.Context, _ string, args map[string]any) (string, error) {
		calls = append(calls, args)
		if args["method"] == "GET" {
			return `{"code":0,"data":{"id":9,"title":"活动","price":1999,"skus":[{"id":3,"sku_id":11,"price":1999,"stock":5}]}}`, nil
		}
		return `{"code":0,"data":null}`, nil
	}

	if _, err := spec.Compose(context.Background(), map[string]any{
		"action": "activity_save",
		"id":     9,
		"values": map[string]any{"title": "改名"},
	}, inv); err != nil {
		t.Fatalf("activity_save 执行失败: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("应发生 GET+POST 两次调用，实际 %d 次", len(calls))
	}
	postParams, _ := calls[1]["params"].(map[string]any)
	skus, _ := postParams["skus"].([]any)
	if len(skus) != 1 {
		t.Fatalf("未传 skus 时应从回查结果补齐，否则会删除全部规格；实际 skus=%v", postParams["skus"])
	}
	if toFloat(postParams["price"]) != 1999 {
		t.Errorf("未传的 price 应被补齐，实际 %v", postParams["price"])
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
