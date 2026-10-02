package provider

import (
	"context"
	"testing"

	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// 本文件校验「补齐域意图」（catalog_domains.go）声明的路由真实存在。
//
// 为什么必须穷举校验：这些意图不再委托专属 cap，而是直接指向后台端点。
// 路径写错（拼写、路径变更、插件下线）不会在编译期暴露，只会在调用时
// 报"未找到端点"——而那时模型已经把它当成了可用能力。
// 用行为验证（真的跑一次 Compose 并捕获它交给 api_invoke 的参数），
// 比读取声明表更可靠：它同时覆盖 action→路由的映射与参数组装两段逻辑。

// TestDomainIntentRoutesExistInCatalog 穷举每个 action，断言端点在真实目录里存在。
func TestDomainIntentRoutesExistInCatalog(t *testing.T) {
	cat, err := BuildAPICatalog()
	if err != nil {
		t.Fatalf("目录构建失败: %v", err)
	}

	checked, skipped := 0, 0
	for _, s := range intent.IntentCatalog {
		actSpec, ok := s.Params["action"]
		if !ok || len(actSpec.Enum) == 0 {
			continue
		}
		for _, action := range actSpec.Enum {
			var (
				gotCap  string
				gotArgs map[string]any
			)
			inv := func(ctx context.Context, name string, args map[string]any) (string, error) {
				gotCap, gotArgs = name, args
				return "{}", nil
			}
			if _, err := s.Compose(context.Background(), map[string]any{"action": action, "id": 1}, inv); err != nil {
				t.Fatalf("意图 %s 的 action=%s 无法执行（声明与实现不一致）: %v", s.Name, action, err)
			}
			method, _ := gotArgs["method"].(string)
			path, _ := gotArgs["path"].(string)
			if method == "" || path == "" {
				// switchCompose 类意图路由到 cap 名而非端点，不在本测试范围内
				skipped++
				continue
			}
			if gotCap != "api_invoke" {
				t.Errorf("意图 %s 的 action=%s 应经 api_invoke 执行，实际=%s", s.Name, action, gotCap)
			}
			if _, found := cat.FindEndpoint(method, path); !found {
				t.Errorf("意图 %s 的 action=%s 指向不存在的端点: %s %s", s.Name, action, method, path)
			}
			// action 是分派用字段，不能混进端点参数；其余字段必须原样透传
			params, _ := gotArgs["params"].(map[string]any)
			if _, leaked := params["action"]; leaked {
				t.Errorf("意图 %s 把 action 透传进了端点参数", s.Name)
			}
			if params["id"] != 1 {
				t.Errorf("意图 %s 未透传业务参数 id: %#v", s.Name, params)
			}
			checked++
		}
	}
	// 防止判定条件失效导致"一个都没校验却通过"
	if checked < 100 {
		t.Fatalf("校验到的端点路由仅 %d 条，判定条件可能已失效（switchCompose 类 %d 条）", checked, skipped)
	}
	t.Logf("已校验 %d 条端点路由（另有 %d 条走专属 cap，不在此范围）", checked, skipped)
}

// TestDomainIntentsAreDefaultOff 新增的补齐域意图必须默认关闭。
// 它们覆盖面远超精选意图，若因"没配白名单就全开"而默认暴露，等于绕过安全评审。
func TestDomainIntentsAreDefaultOff(t *testing.T) {
	names := []string{}
	for _, s := range intent.IntentCatalog {
		if len(s.Caps) == 1 && s.Caps[0] == "api_invoke" && len(s.Params["action"].Enum) > 0 {
			names = append(names, s.Name)
			if !s.DefaultOff {
				t.Errorf("补齐域意图 %s 未标记 DefaultOff", s.Name)
			}
		}
	}
	if len(names) < 30 {
		t.Fatalf("识别到的补齐域意图仅 %d 个，判定条件可能已失效", len(names))
	}
	t.Logf("补齐域意图 %d 个，全部 DefaultOff", len(names))
}

// TestRecommendedExposedAllExist 推荐白名单里的名字必须真实存在。
// 拼错一个名字不会报错，只会让该能力永远无法被开启——静默失效最难发现。
func TestRecommendedExposedAllExist(t *testing.T) {
	known := map[string]bool{}
	for _, s := range intent.IntentCatalog {
		known[s.Name] = true
	}
	for _, n := range intent.RecommendedExposed() {
		if !known[n] {
			t.Errorf("推荐白名单引用了不存在的意图: %s", n)
		}
	}
}
