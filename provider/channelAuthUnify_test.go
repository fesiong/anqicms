package provider

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
)

// channelKeywordAPIStub 起一张最小 iris 路由表，只注册 seo_keyword 的 list 端点。
//
// 单测进程里没有常驻服务，InvokeAdminAPI 会以「当前进程未注册 iris 应用」失败——
// 那是环境缺失，不是权限判定。要让「工具调用确实成功」这条断言有意义，
// 必须把终点端点补上，才能区分「授权放行后执行成功」与「授权拒绝」。
func channelKeywordAPIStub(t *testing.T) {
	t.Helper()
	app := iris.New()
	app.Get("/system/api/plugin/keyword/list", func(ctx iris.Context) {
		ctx.JSON(iris.Map{"code": config.StatusOK, "msg": "", "data": []any{}, "total": 0, "exact": true})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("构建测试用 iris 应用失败：%v", err)
	}
	SetInvokerApplicationForTest(app)
	t.Cleanup(func() { SetInvokerApplicationForTest(nil) })
}

// 后台分组没有任何菜单权限，也不能再阻止渠道工具调用（用户报告的实际故障）。
func TestChannelToolCallIgnoresAdminMenuPermissions(t *testing.T) {
	w := channelChatTestSite(t)
	withTestAiSetting(t, w, []string{"seo_keyword"})
	channelKeywordAPIStub(t)
	env := newChannelTurnTestEnv(t, w, channelTurnTestConfig{})
	ctx := context.Background()
	chatId := "auth-unify-" + channelPermSuffix()
	env.cleanupSession(t, env.binding, chatId)

	st, err := env.svc.prepareChannelTurn(ctx, env.request(chatId, "列出关键词", 1, false))
	if err != nil {
		t.Fatalf("准备渠道轮次失败：%v", err)
	}
	defer st.release()

	res, err := st.executeToolCall(ctx, &schema.ToolCall{
		ID:   "auth-unify-1",
		Type: "function",
		Function: schema.FunctionCall{
			Name:      "seo_keyword",
			Arguments: `{"action":"list","page":1,"page_size":1}`,
		},
	})
	if err != nil {
		t.Fatalf("后台菜单权限不应再约束渠道工具：%v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("工具调用结果必须是成功：%+v", res)
	}
}

// 能力视图只展示与执行同源的曝光/群聊口径，且不泄露菜单映射、策略或智能体授权。
func TestChannelCapabilitiesShowsExposureAndGroup(t *testing.T) {
	w := channelAdminTestSite(t)
	withTestAiSetting(t, w, []string{"seo_keyword"})

	view, err := w.ChannelCapabilities()
	if err != nil {
		t.Fatalf("读取能力清单失败：%v", err)
	}
	byName := map[string]ChannelCapabilityIntent{}
	for _, item := range view.Intents {
		byName[item.Name] = item
	}
	for _, name := range []string{"seo_keyword", "content_article", "api"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("能力视图缺少意图 %s", name)
		}
	}
	if !byName["seo_keyword"].Exposed {
		t.Error("站点曝光内的 seo_keyword 必须标记 exposed=true")
	}
	if byName["content_article"].Exposed {
		t.Error("未曝光的 content_article 必须标记 exposed=false")
	}
	if !byName["api"].GroupAllowed {
		t.Error("api_schema 所在意图 api 必须标记 group_allowed=true")
	}
	if byName["content_article"].GroupAllowed {
		t.Error("content_article 在群聊不可用，group_allowed 必须为 false")
	}

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("序列化能力清单失败：%v", err)
	}
	for _, key := range []string{"menu_paths", "blocked_builtins", "agent_allowed", "policy", "reason", "blocked"} {
		if strings.Contains(string(raw), `"`+key+`"`) {
			t.Errorf("能力清单仍包含已删除字段 %q", key)
		}
	}
}

// 详情与保存接口不再携带 policy_json。
func TestChannelDetailOmitsPolicyJSON(t *testing.T) {
	w := channelAdminTestSite(t)
	ch := newManagerTestChannel(t, w, false)

	detail, err := w.ChannelAdminDetail(ch.Id)
	if err != nil {
		t.Fatalf("读取渠道详情失败：%v", err)
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("序列化渠道详情失败：%v", err)
	}
	if strings.Contains(string(raw), "policy_json") {
		t.Fatal("渠道详情仍返回 policy_json")
	}
}

// 权限层不得再出现策略、白名单、菜单映射或路径黑名单。
// 反向对照：在 channelPermissions.go 里加回 ChannelCapPermission 或 ParseChannelPolicy，本用例变红。
func TestChannelPermissionLayerHasNoCapMapping(t *testing.T) {
	raw, err := os.ReadFile("channelPermissions.go")
	if err != nil {
		t.Fatalf("读取 channelPermissions.go 失败：%v", err)
	}
	for _, forbidden := range []string{
		"ChannelCapPermission", "channelCapHardBlocked", "channelIntentMenuTargets",
		"channelMenuPathForURI", "channelNormalizeInvokePath", "channelAPIPathBlacklist",
		"ParseChannelPolicy", "checkChannelAgentWhitelist", "checkChannelPolicy",
		"checkChannelAdmin", "CapPermission",
	} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("权限层仍包含已删除实现：%s", forbidden)
		}
	}

	principal, err := os.ReadFile("channelPrincipal.go")
	if err != nil {
		t.Fatalf("读取 channelPrincipal.go 失败：%v", err)
	}
	for _, forbidden := range []string{"PolicyRevision", "ChannelPolicy", "PolicyDecision", "ChannelArgsHash"} {
		if strings.Contains(string(principal), forbidden) {
			t.Errorf("执行主体仍包含策略实现：%s", forbidden)
		}
	}
}
