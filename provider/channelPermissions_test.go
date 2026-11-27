package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// ============================================================
// 夹具：真实库（站点 1）。
//
// 每个用例自建渠道 / 管理员 / 绑定，数据都带随机后缀，
// 清理一律按自己创建的 ID 精确删除——绝不按 site_id 批量删，
// 否则会连真实站点数据一起清掉。
// ============================================================

func channelPermTestSite(t *testing.T) *Website {
	t.Helper()
	w := initTestSite(t)
	if w == nil || w.DB == nil {
		t.Fatal("站点 1 未初始化，无法连接开发库")
	}
	return w
}

func channelPermSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// newPermTestChannel 建一条「启用 + 开启聊天管理」的渠道。AppId 必带随机后缀：
// uk_app_identity_hash 全局唯一，固定值会撞上库里残留行。AgentId 同样取随机值。
func newPermTestChannel(t *testing.T, w *Website, revision int) *model.MessageChannel {
	t.Helper()
	return newPermTestChannelState(t, w, revision, 1, 1)
}

// newPermTestChannelState 允许指定 enabled / chatEnabled，用于验证
// 「渠道停用」「仅通知渠道」必须在权限层被拒绝（R36）。
func newPermTestChannelState(t *testing.T, w *Website, revision, enabled, chatEnabled int) *model.MessageChannel {
	t.Helper()
	suffix := channelPermSuffix()
	ch := &model.MessageChannel{
		SiteId:      w.Id,
		Platform:    model.ChannelPlatformFeishu,
		Name:        "权限测试渠道-" + suffix,
		Enabled:     enabled,
		ChatEnabled: chatEnabled,
		AgentId:     uint(9000000 + time.Now().UnixNano()%90000),
		AppId:       "perm_app_" + suffix,
		OrgId:       "perm_org_" + suffix,
		AppSecret:   "perm_secret_" + suffix,
		Revision:    revision,
	}
	if err := w.DB.Create(ch).Error; err != nil {
		t.Fatalf("创建测试渠道失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("channel_id = ? AND site_id = ?", ch.Id, w.Id).Delete(&model.ChannelBinding{}).Error; err != nil {
			t.Logf("清理渠道 %d 的绑定失败：%v", ch.Id, err)
		}
		if err := w.DB.Where("id = ? AND site_id = ?", ch.Id, w.Id).Delete(&model.MessageChannel{}).Error; err != nil {
			t.Logf("清理渠道 %d 失败：%v", ch.Id, err)
		}
	})
	return ch
}

// newPermTestAdmin 建一个普通管理员及其分组。status 用于验证「停用管理员必须拒绝」。
func newPermTestAdmin(t *testing.T, w *Website, status uint) *model.Admin {
	t.Helper()
	suffix := channelPermSuffix()
	group := &model.AdminGroup{
		Title:  "权限测试组-" + suffix,
		Status: 1,
	}
	if err := w.DB.Create(group).Error; err != nil {
		t.Fatalf("创建测试管理员分组失败：%v", err)
	}
	admin := &model.Admin{
		UserName: "perm_admin_" + suffix,
		Password: "perm_test_password",
		Status:   status,
		GroupId:  group.Id,
	}
	if err := w.DB.Create(admin).Error; err != nil {
		w.DB.Where("id = ?", group.Id).Delete(&model.AdminGroup{})
		t.Fatalf("创建测试管理员失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", admin.Id).Delete(&model.Admin{}).Error; err != nil {
			t.Logf("清理管理员 %d 失败：%v", admin.Id, err)
		}
		if err := w.DB.Where("id = ?", group.Id).Delete(&model.AdminGroup{}).Error; err != nil {
			t.Logf("清理管理员分组 %d 失败：%v", group.Id, err)
		}
	})
	return admin
}

// newPermTestBinding 建一条有效绑定（revoked_at = 0）。
func newPermTestBinding(t *testing.T, w *Website, channelId, adminId uint) *model.ChannelBinding {
	t.Helper()
	suffix := channelPermSuffix()
	binding := &model.ChannelBinding{
		SiteId:         w.Id,
		ChannelId:      channelId,
		OrgId:          "perm_bind_org_" + suffix,
		PlatformUserId: "perm_bind_user_" + suffix,
		AdminId:        adminId,
		Revision:       1,
		IdentityHash:   model.ComputeAppIdentityHash("perm_binding", suffix, fmt.Sprint(channelId)),
	}
	if err := w.DB.Create(binding).Error; err != nil {
		t.Fatalf("创建测试绑定失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ? AND site_id = ?", binding.Id, w.Id).Delete(&model.ChannelBinding{}).Error; err != nil {
			t.Logf("清理绑定 %d 失败：%v", binding.Id, err)
		}
	})
	return binding
}

// permTestPrincipal 构造执行主体。AgentId 取渠道配置的接待智能体：
// R35 起门禁要求「执行主体声明的智能体」非 0 且与渠道一致。
func permTestPrincipal(w *Website, ch *model.MessageChannel, adminId, bindingId uint) ExecutionPrincipal {
	return ExecutionPrincipal{
		SiteId:    w.Id,
		AdminId:   adminId,
		BindingId: bindingId,
		ChannelId: ch.Id,
		AgentId:   ch.AgentId,
		SessionId: "perm-sess-" + channelPermSuffix(),
	}
}

// ============================================================
// 绑定有效性 / 渠道状态门禁（R36、规格 §6）
// ============================================================

// TestRevokedBindingIsDenied 绑定是执行主体的身份凭据：撤销、借他人绑定、跨渠道绑定
// 都必须立即失效（规格 §6）。三个子用例都真实改库 / 真实建库行，不是只改内存结构——
// 删掉 checkChannelBinding 里的 revoked_at = 0、admin_id 或 channel_id 条件都会变红。
func TestRevokedBindingIsDenied(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()

	// 公共夹具：启用的渠道 + 正常管理员 + 有效绑定。
	setup := func(t *testing.T) (*model.MessageChannel, *model.Admin, *model.ChannelBinding) {
		t.Helper()
		ch := newPermTestChannel(t, w, 1)
		admin := newPermTestAdmin(t, w, 1)
		binding := newPermTestBinding(t, w, ch.Id, admin.Id)
		return ch, admin, binding
	}

	t.Run("撤销后立即失效", func(t *testing.T) {
		ch, admin, binding := setup(t)
		p := permTestPrincipal(w, ch, admin.Id, binding.Id)
		if err := w.checkChannelCall(ctx, p); err != nil {
			t.Fatalf("撤销前应放行（基线），实际：%v", err)
		}
		// 真实改库：revoked_at 非 0。
		if err := w.DB.Model(&model.ChannelBinding{}).
			Where("id = ? AND site_id = ?", binding.Id, w.Id).
			Update("revoked_at", time.Now().Unix()).Error; err != nil {
			t.Fatalf("撤销绑定失败：%v", err)
		}
		if err := w.checkChannelCall(ctx, p); !errors.Is(err, ErrChannelPrincipalInvalid) {
			t.Fatalf("已撤销绑定必须返回 ErrChannelPrincipalInvalid，实际：%v", err)
		}
	})

	t.Run("绑定属于他管理员", func(t *testing.T) {
		ch, admin, _ := setup(t)
		// 库里真实存在一条属于另一个管理员的绑定，principal 的 AdminId 指向本人。
		other := newPermTestAdmin(t, w, 1)
		otherBinding := newPermTestBinding(t, w, ch.Id, other.Id)
		p := permTestPrincipal(w, ch, admin.Id, otherBinding.Id)
		if err := w.checkChannelCall(ctx, p); !errors.Is(err, ErrChannelPrincipalInvalid) {
			t.Fatalf("借用他人绑定必须返回 ErrChannelPrincipalInvalid，实际：%v", err)
		}
	})

	t.Run("绑定属于他渠道", func(t *testing.T) {
		chA, admin, bindingA := setup(t)
		chB := newPermTestChannel(t, w, 1)
		// principal 声明渠道 B，却拿渠道 A 的绑定 ID：渠道与绑定必须一一对应。
		p := permTestPrincipal(w, chB, admin.Id, bindingA.Id)
		if err := w.checkChannelCall(ctx, p); !errors.Is(err, ErrChannelPrincipalInvalid) {
			t.Fatalf("跨渠道绑定必须返回 ErrChannelPrincipalInvalid，实际：%v", err)
		}
		// 反向对照：换成渠道 A 自己的绑定必须放行（否决「一律拒绝」的实现）。
		if err := w.checkChannelCall(ctx, permTestPrincipal(w, chA, admin.Id, bindingA.Id)); err != nil {
			t.Fatalf("本渠道绑定应放行，实际：%v", err)
		}
	})
}

// TestChannelDisabledIsDenied 渠道停用（enabled = 0）时每次调用都必须拒绝。
func TestChannelDisabledIsDenied(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannelState(t, w, 1, 0, 1)
	admin := newPermTestAdmin(t, w, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	p := permTestPrincipal(w, ch, admin.Id, binding.Id)

	if err := w.checkChannelCall(ctx, p); !errors.Is(err, ErrChannelDisabled) {
		t.Fatalf("停用渠道必须返回 ErrChannelDisabled，实际：%v", err)
	}
}

// TestChatDisabledIsDenied 「仅通知」渠道（chat_enabled = 0）不得执行任何管理动作。
func TestChatDisabledIsDenied(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannelState(t, w, 1, 1, 0)
	admin := newPermTestAdmin(t, w, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	p := permTestPrincipal(w, ch, admin.Id, binding.Id)

	if err := w.checkChannelCall(ctx, p); !errors.Is(err, ErrChannelDisabled) {
		t.Fatalf("仅通知渠道必须返回 ErrChannelDisabled，实际：%v", err)
	}
}

// TestAgentPrincipalMustMatchChannel 执行主体声明的智能体必须与渠道接待智能体一致（R35）。
func TestAgentPrincipalMustMatchChannel(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w, 1)
	admin := newPermTestAdmin(t, w, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	p := permTestPrincipal(w, ch, admin.Id, binding.Id)

	if err := w.checkChannelCall(ctx, p); err != nil {
		t.Fatalf("主体智能体与渠道一致时应放行（基线），实际：%v", err)
	}

	// principal 声明的智能体不是渠道的接待智能体。
	switched := p
	switched.AgentId = ch.AgentId + 1
	if err := w.checkChannelCall(ctx, switched); !errors.Is(err, ErrChannelAgentNotAllowed) {
		t.Fatalf("执行智能体与渠道接待智能体不一致必须返回 ErrChannelAgentNotAllowed，实际：%v", err)
	}

	// AgentId 为 0 同样拒绝（缺省拒绝，不猜渠道配置）。
	missing := p
	missing.AgentId = 0
	if err := w.checkChannelCall(ctx, missing); !errors.Is(err, ErrChannelAgentNotAllowed) {
		t.Fatalf("缺少执行智能体必须返回 ErrChannelAgentNotAllowed，实际：%v", err)
	}
}

// ============================================================
// 站点曝光
// ============================================================

// TestChannelExposureConfigUsesOwnSite 曝光配置必须读「该站点自己的」配置，
// 不能拿主站点/进程级配置顶替（provider/mcp.go 的 GetMcpConfig 只返回主站点配置）。
func TestChannelExposureConfigUsesOwnSite(t *testing.T) {
	w := channelPermTestSite(t)

	// 场景 1：站点配置了曝光清单后，只有被点名的意图放行；DefaultOff 未被点名必须拒绝。
	withTestAiSetting(t, w, []string{"content_article"})
	if !intent.IntentAllowed(w.channelExposureConfig(), "content_article") {
		t.Fatal("被点名的意图应放行")
	}
	if intent.IntentAllowed(w.channelExposureConfig(), "seo_keyword") {
		t.Fatal("曝光清单非空但未点名的意图必须拒绝")
	}
	if intent.IntentAllowed(w.channelExposureConfig(), "system_plugin") {
		t.Fatal("DefaultOff 意图未被点名时必须拒绝")
	}
	// 反向对照：空配置下同一意图是放行的，说明上面拒绝来自站点自己的曝光清单，
	// 而不是「所有意图默认拒绝」的实现。
	if !intent.IntentAllowed(intent.Config{}, "seo_keyword") {
		t.Fatal("空曝光配置下 seo_keyword 本应放行，对照失效")
	}

	// 场景 2：换一个站点（Id=2）不该继承站点 1 的收窄曝光清单。
	// 这里直接构造只带 Id/DB 的站点实例：非主站点的 LoadAiSetting 会立刻返回空配置，
	// 不会去碰站点 1 的缓存，正好用来证明读的是「站点自己的」配置而不是主站点配置。
	w2 := &Website{Id: 2, DB: w.DB}
	if !intent.IntentAllowed(w2.channelExposureConfig(), "seo_keyword") {
		t.Fatal("站点 2 不应继承站点 1 的曝光清单")
	}
}

// withTestAiSetting 临时写入站点 MCP 曝光白名单，并在用例结束时精确还原原值
// （原值不存在则删除该行）。Cache 为 nil 的 Website 会直接读库，不受进程缓存影响。
func withTestAiSetting(t *testing.T, w *Website, exposedIntents []string) {
	t.Helper()
	raw := w.GetSettingValue(AiSettingKey)
	var root map[string]any
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &root); err != nil {
			t.Fatalf("解析现有 ai_setting 失败：%v", err)
		}
	}
	if root == nil {
		root = map[string]any{}
	}
	mcp, _ := root["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}
	mcp["exposed_intents"] = exposedIntents
	root["mcp"] = mcp
	buf, err := json.Marshal(root)
	if err != nil {
		t.Fatalf("序列化 ai_setting 失败：%v", err)
	}
	if err := w.DB.Save(&model.Setting{Key: AiSettingKey, Value: string(buf)}).Error; err != nil {
		t.Fatalf("写入 ai_setting 失败：%v", err)
	}
	// LoadAiSetting 会写 86400 秒的进程缓存，不清掉的话读到的还是旧值，
	// 用例也会把临时配置泄漏给同一进程里的后续用例。
	if w.Cache != nil {
		w.Cache.Delete(AiSettingKey)
	}
	t.Cleanup(func() {
		defer func() {
			if w.Cache != nil {
				w.Cache.Delete(AiSettingKey)
			}
		}()
		if strings.TrimSpace(raw) == "" {
			if err := w.DB.Where("`key` = ?", AiSettingKey).Delete(&model.Setting{}).Error; err != nil {
				t.Logf("清理临时 ai_setting 失败：%v", err)
			}
			return
		}
		if err := w.DB.Model(&model.Setting{}).Where("`key` = ?", AiSettingKey).Update("value", raw).Error; err != nil {
			t.Logf("还原 ai_setting 失败：%v", err)
		}
	})
}
