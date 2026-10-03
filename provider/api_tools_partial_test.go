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
	// 不在清单里的端点（如逐字段赋值的 material/place/group）不该注入
	for _, p := range []string{"/plugin/material/detail", "/plugin/place/detail", "/plugin/user/group/detail"} {
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
