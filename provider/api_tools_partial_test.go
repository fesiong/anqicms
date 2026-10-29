package provider

import "testing"

// injectPartialUpdate 是「局部更新不被清空」的最后一道闸门。
//
// 背景（2026-10-03）：8 处控制器曾无条件 `req.UpdateAll = true`，
// 于是「只传一个字段」会把其余字段按零值写回（正文清空、标签清空、
// 会员 status 归 0 即账号被禁用），而回执仍是 ok=true。
//
// 注入点选在 provider 而非 pkg/mcp/intent，因为 `api` 意图（通用端点调用通道）
// 能调到本清单里的任何端点，而它不走 capEndpoints / invokeRoutes。
// 实测那条通道确实漏了 partial：调 POST /archive/detail 传 {id,title}，
// 端点回「未定义模型」（module_id 被清零导致 SaveArchive 走错分支）。

// TestInjectPartialUpdateCoversKnownEndpoints 清单里的端点必须都注入到 partial。
//
// 漏一个，那个端点的局部更新就会静默清空字段——症状是 ok=true 的假成功。
func TestInjectPartialUpdateCoversKnownEndpoints(t *testing.T) {
	mustCover := []string{
		"/archive/detail",
		"/category/detail",
		"/module/detail",
		"/setting/nav",
		"/plugin/tag/detail",
		"/plugin/user/detail",
		// 2026-10-29 起 UserGroupRequest 也有 UpdateAll/Partial 开关：
		// 原来是「拼新对象 Save 全列」，改个名字就会把 Level/Price/Setting 清成零值、
		// 把 Status（停用）刷回 1。端点是 save-or-create，既有记录必须走 PATCH。
		"/plugin/user/group/detail",
		// 2026-10-29 起 CouponRequest 也指针化 + 加了 UpdateAll/Partial：
		// 原来是逐字段无条件赋值，改个标题就把面额/门槛/有效期清成 0，
		// 而面额清零的券还挂着已发出的券码（核销按 0 元抵扣，无法从券码侧补回）。
		"/plugin/coupon/detail",
		// 活动同理，而且更狠：不传 skus 会走到 `len(skus)==0` 分支删光全部规格。
		"/plugin/activity/detail",
		// 2026-10-29 第三批：素材 / 素材分类 / 城市站全部指针化。
		// 素材原本硬编码 Status=1（永远无法禁用）且 md5 取的是旧内容；
		// 城市站改个标题就把经纬度/时区/logo/正文清空，还会用空标题重新生成
		// url_token —— 旧链接 404 且不可回滚。
		"/plugin/material/detail",
		"/plugin/material/category/detail",
		"/plugin/place/detail",
	}
	for _, p := range mustCover {
		got := injectPartialUpdate("POST", p, map[string]any{"id": 1, "title": "x"})
		if got["partial"] != true {
			t.Errorf("POST %s 应注入 partial=true（UpdateAll 型端点，漏了会清空未传字段）", p)
		}
	}
}

// TestInjectPartialUpdateSkipsNonPatch POST 之外的请求、以及不支持 PATCH 的端点，
// 都不该被注入——给了也无效（端点没有 Partial 字段），只会误导后来者。
func TestInjectPartialUpdateSkipsNonPatch(t *testing.T) {
	// GET 不该注入
	if got := injectPartialUpdate("GET", "/archive/detail", map[string]any{"id": 1}); got["partial"] != nil {
		t.Error("GET 不该注入 partial")
	}
	// 不在清单里的端点（逐字段赋值、没有 Partial 字段）不该注入
	for _, p := range []string{"/plugin/redirect/detail", "/plugin/material/import"} {
		if got := injectPartialUpdate("POST", p, map[string]any{"id": 1}); got["partial"] != nil {
			t.Errorf("POST %s 是逐字段无条件赋值型，注入 partial 无效，不该加进清单", p)
		}
	}
	// 参数为空时原样返回
	if got := injectPartialUpdate("POST", "/archive/detail", nil); got != nil {
		t.Error("params 为 nil 时应原样返回 nil")
	}
}

// TestInjectPartialUpdateNotOverridable 调用方传 partial:false 也关不掉 PATCH。
//
// 关掉就退回全量覆盖、静默清空数据，而回执是 ok=true——AI 无从察觉。
// 想要全量覆盖应走显式动作，不能靠这个开关。
func TestInjectPartialUpdateNotOverridable(t *testing.T) {
	got := injectPartialUpdate("POST", "/archive/detail", map[string]any{
		"id": 1, "title": "x", "partial": false,
	})
	if got["partial"] != true {
		t.Errorf("调用方传 partial:false 不该能关掉 PATCH，实际=%#v", got["partial"])
	}
}

// TestInjectPartialUpdateDoesNotMutateInput 不能就地改调用方的 map。
//
// 调用方可能复用同一个 params 调多个端点；就地写入会串味。
func TestInjectPartialUpdateDoesNotMutateInput(t *testing.T) {
	in := map[string]any{"id": 1, "title": "x"}
	_ = injectPartialUpdate("POST", "/archive/detail", in)
	if _, leaked := in["partial"]; leaked {
		t.Error("injectPartialUpdate 不该修改入参 map")
	}
}
