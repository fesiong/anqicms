package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// ============================================================
// 夹具：真实库（站点 1）。
//
// 每个用例自建渠道 / 管理员 / 绑定 / 接待策略，数据都带随机后缀，
// 清理一律按自己创建的 ID 精确删除——绝不按 site_id 批量删，
// 否则会连真实站点数据一起清掉。
// ============================================================

func channelPermTestSite(t *testing.T) *Website {
	t.Helper()
	w := initTestSite(t)
	if w == nil || w.DB == nil {
		t.Fatal("站点 1 未初始化，无法连接开发库")
	}
	// 接待策略表在当前开发库上尚未建立（T1 的迁移清单里没有它），本任务按需补建。
	// 其余表（渠道 / 绑定 / 管理员）由 T1、T2 的迁移保证存在。
	if err := w.DB.AutoMigrate(&model.AgentReceptionPolicy{}); err != nil {
		t.Fatalf("迁移接待策略表失败：%v", err)
	}
	return w
}

func channelPermSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// newPermTestChannel 建一条渠道。AppId 必带随机后缀：uk_app_identity_hash 全局唯一，
// 固定值会撞上库里残留行。AgentId 同样取随机值，避免撞上他人策略。
func newPermTestChannel(t *testing.T, w *Website, policyJSON string, revision int) *model.MessageChannel {
	t.Helper()
	suffix := channelPermSuffix()
	ch := &model.MessageChannel{
		SiteId:      w.Id,
		Platform:    model.ChannelPlatformFeishu,
		Name:        "权限测试渠道-" + suffix,
		Enabled:     1,
		ChatEnabled: 1,
		AgentId:     uint(9000000 + time.Now().UnixNano()%90000),
		AppId:       "perm_app_" + suffix,
		OrgId:       "perm_org_" + suffix,
		AppSecret:   "perm_secret_" + suffix,
		PolicyJson:  policyJSON,
		Revision:    revision,
	}
	if err := w.DB.Create(ch).Error; err != nil {
		t.Fatalf("创建测试渠道失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("channel_id = ? AND site_id = ?", ch.Id, w.Id).Delete(&model.ChannelBinding{}).Error; err != nil {
			t.Logf("清理渠道 %d 的绑定失败：%v", ch.Id, err)
		}
		if err := w.DB.Where("site_id = ? AND agent_id = ?", w.Id, ch.AgentId).Delete(&model.AgentReceptionPolicy{}).Error; err != nil {
			t.Logf("清理渠道 %d 的接待策略失败：%v", ch.Id, err)
		}
		if err := w.DB.Where("id = ? AND site_id = ?", ch.Id, w.Id).Delete(&model.MessageChannel{}).Error; err != nil {
			t.Logf("清理渠道 %d 失败：%v", ch.Id, err)
		}
	})
	return ch
}

// newPermTestAdmin 建一个普通管理员及其分组。status 用于验证「停用管理员必须拒绝」。
func newPermTestAdmin(t *testing.T, w *Website, permissions []string, status uint) *model.Admin {
	t.Helper()
	suffix := channelPermSuffix()
	group := &model.AdminGroup{
		Title:   "权限测试组-" + suffix,
		Status:  1,
		Setting: model.GroupSetting{Permissions: permissions},
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

// setPermTestAllowedIntents 写入指定智能体的接待白名单（清理由渠道夹具统一按
// (site_id, agent_id) 删除）。names 为 nil 时写入空数组，用于「空数组必须拒绝」。
func setPermTestAllowedIntents(t *testing.T, w *Website, agentId uint, names []string) {
	t.Helper()
	if names == nil {
		names = []string{}
	}
	buf, err := json.Marshal(names)
	if err != nil {
		t.Fatalf("序列化接待白名单失败：%v", err)
	}
	res := w.DB.Model(&model.AgentReceptionPolicy{}).
		Where("site_id = ? AND agent_id = ?", w.Id, agentId).
		Update("allowed_intents", string(buf))
	if res.Error != nil {
		t.Fatalf("更新接待白名单失败：%v", res.Error)
	}
	if res.RowsAffected == 0 {
		row := &model.AgentReceptionPolicy{SiteId: w.Id, AgentId: agentId, AllowedIntents: string(buf)}
		if err := w.DB.Create(row).Error; err != nil {
			t.Fatalf("创建接待白名单失败：%v", err)
		}
	}
}

func permTestPrincipal(w *Website, ch *model.MessageChannel, adminId, bindingId uint) ExecutionPrincipal {
	return ExecutionPrincipal{
		SiteId:         w.Id,
		AdminId:        adminId,
		BindingId:      bindingId,
		ChannelId:      ch.Id,
		SessionId:      "perm-sess-" + channelPermSuffix(),
		PolicyRevision: int64(ch.Revision),
	}
}

// ============================================================
// 能力 → 权限映射
// ============================================================

// TestEveryCapHasPermissionDecision 遍历真实的 capHandlers：每个 cap 都必须
// 要么映射到后台菜单权限路径，要么显式 blocked。任何「未解析」都判失败。
func TestEveryCapHasPermissionDecision(t *testing.T) {
	w := channelPermTestSite(t)
	svc := &AiChatService{db: w.DB, site: w}
	caps := svc.assembleCapHandlers()
	if len(caps) == 0 {
		t.Fatal("capHandlers 为空，无法校验权限映射")
	}

	var menuMapped, dynamic, builtinBlocked, unmappedBlocked int
	var undecided []string
	names := make([]string, 0, len(caps))
	for name := range caps {
		names = append(names, name)
		perm, ok := ChannelCapPermission(name)
		if !ok {
			undecided = append(undecided, name)
			continue
		}
		switch {
		case perm.Blocked && perm.MenuPath != "":
			t.Errorf("cap %s 同时 blocked 且有菜单路径，判定矛盾", name)
		case perm.Blocked:
			if strings.TrimSpace(perm.Reason) == "" {
				t.Errorf("cap %s 被 blocked 但没写原因", name)
			}
			if perm.Builtin {
				builtinBlocked++
			} else {
				unmappedBlocked++
			}
		case perm.MenuPath == "":
			t.Errorf("cap %s 既未 blocked，也没有菜单权限路径", name)
		case perm.Dynamic:
			dynamic++
		default:
			menuMapped++
		}
	}
	sort.Strings(names)
	if len(undecided) > 0 {
		sort.Strings(undecided)
		t.Fatalf("以下 cap 没有权限判定（未知 cap 必须显式 blocked 或映射菜单）：%s", strings.Join(undecided, ", "))
	}
	t.Logf("cap 总数=%d 菜单映射=%d 动态解析=%d 内置关闭=%d 未映射关闭=%d\n%s",
		len(caps), menuMapped, dynamic, builtinBlocked, unmappedBlocked, strings.Join(names, ", "))

	// 规格 §7：fs/shell/联网首版固定关闭。少写一个就说明映射表被削弱了。
	for _, name := range []string{
		"read_file", "write_file", "edit_file", "search_replace",
		"bash", "grep", "glob", "list_directory", "web_fetch", "web_search",
	} {
		perm, ok := ChannelCapPermission(name)
		if !ok || !perm.Blocked {
			t.Errorf("cap %s 必须显式 blocked（规格 §7：fs/shell/联网首版关闭），实际 ok=%v blocked=%v", name, ok, perm.Blocked)
		}
	}
	// api_* 是通用入口：必须有显式菜单入口权限，不能默认放行。
	for _, name := range []string{"api_list", "api_schema", "api_invoke"} {
		perm, ok := ChannelCapPermission(name)
		if !ok || perm.Blocked || perm.MenuPath == "" {
			t.Errorf("cap %s 必须映射到显式菜单入口权限，实际 ok=%v perm=%+v", name, ok, perm)
		}
	}
	// 反向对照：任何未登记的 cap 名都必须拿不到判定（默认拒绝）。
	if _, ok := ChannelCapPermission("no_such_cap_" + channelPermSuffix()); ok {
		t.Error("未登记的 cap 必须返回 ok=false（默认拒绝）")
	}
}

// ============================================================
// 意图级门禁
// ============================================================

func TestUnknownIntentIsDenied(t *testing.T) {
	w := channelPermTestSite(t)
	p := ExecutionPrincipal{
		SiteId: w.Id, AdminId: 1, BindingId: 1, ChannelId: 1,
		SessionId: "perm-unknown", PolicyRevision: 1,
	}
	err := w.CheckIntentPermission(context.Background(), p, "no_such_intent_"+channelPermSuffix(), "list")
	if !errors.Is(err, ErrChannelIntentUnknown) {
		t.Fatalf("未知意图必须返回 ErrChannelIntentUnknown，实际：%v", err)
	}
}

func TestAdminWithoutMenuPermissionIsDenied(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w, `{"intents":{"seo_keyword":{"list":"allow"}}}`, 1)
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})

	// 普通管理员：有账号权限，没有关键词菜单权限。
	denied := newPermTestAdmin(t, w, []string{"/account/list"}, 1)
	deniedBinding := newPermTestBinding(t, w, ch.Id, denied.Id)
	err := w.CheckIntentPermission(ctx, permTestPrincipal(w, ch, denied.Id, deniedBinding.Id), "seo_keyword", "list")
	if !errors.Is(err, ErrChannelPermissionDenied) {
		t.Fatalf("缺少菜单权限的管理员必须被拒，实际：%v", err)
	}

	// 反向对照：同一个意图、同一个渠道，只把菜单权限补上就必须放行。
	allowed := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
	allowedBinding := newPermTestBinding(t, w, ch.Id, allowed.Id)
	if err := w.CheckIntentPermission(ctx, permTestPrincipal(w, ch, allowed.Id, allowedBinding.Id), "seo_keyword", "list"); err != nil {
		t.Fatalf("有菜单权限的管理员应放行，实际：%v", err)
	}

	// 停用管理员（Status != 1）必须被拒，且不能因为分组权限齐全而放行。
	disabled := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 0)
	disabledBinding := newPermTestBinding(t, w, ch.Id, disabled.Id)
	err = w.CheckIntentPermission(ctx, permTestPrincipal(w, ch, disabled.Id, disabledBinding.Id), "seo_keyword", "list")
	if !errors.Is(err, ErrChannelAdminInvalid) {
		t.Fatalf("停用管理员必须返回 ErrChannelAdminInvalid，实际：%v", err)
	}
}

// TestSuperAdminStillBoundByCapMapping 超管只豁免「菜单前缀匹配」，
// 不豁免 blocked/未映射的 cap，也不豁免 api_invoke 的路径黑名单。
func TestSuperAdminStillBoundByCapMapping(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w, `{"intents":{"content_article":{"delete":"allow"}}}`, 1)
	binding := newPermTestBinding(t, w, ch.Id, 1)
	p := permTestPrincipal(w, ch, 1, binding.Id)

	if err := w.CheckCapPermission(ctx, p, "read_file", `{"file_path":"/etc/passwd"}`); !errors.Is(err, ErrChannelCapBlocked) {
		t.Fatalf("超管调用被关闭的内置 cap 必须返回 ErrChannelCapBlocked，实际：%v", err)
	}
	err := w.CheckCapPermission(ctx, p, "api_invoke", `{"method":"POST","path":"/system/api/admin/list"}`)
	if !errors.Is(err, ErrChannelAPIPathBlocked) {
		t.Fatalf("超管经 api_invoke 访问 /admin 必须返回 ErrChannelAPIPathBlocked，实际：%v", err)
	}
	if err := w.CheckCapPermission(ctx, p, "no_such_cap_"+channelPermSuffix(), `{}`); !errors.Is(err, ErrChannelCapBlocked) {
		t.Fatalf("未登记的 cap 必须返回 ErrChannelCapBlocked，实际：%v", err)
	}
	// 反向对照：黑名单之外的只读端点，超管应通过（否决「一律拒绝」的实现）。
	if err := w.CheckCapPermission(ctx, p, "api_invoke", `{"method":"GET","path":"/system/api/archive/list"}`); err != nil {
		t.Fatalf("超管调用非黑名单端点应通过，实际：%v", err)
	}
}

// ============================================================
// 渠道策略（intent + action）
// ============================================================

func TestChannelPolicyDefaultsToDeny(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()

	cases := []struct {
		name   string
		policy string
	}{
		{"未配置策略", ""},
		{"策略未列该意图", `{"intents":{"other_intent":{"list":"allow"}}}`},
		{"策略未列该动作", `{"intents":{"seo_keyword":{"create":"allow"}}}`},
		{"策略取值非法", `{"intents":{"seo_keyword":{"list":"yes"}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := newPermTestChannel(t, w, tc.policy, 1)
			setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})
			admin := newPermTestAdmin(t, w, []string{"/plugin/contentops"}, 1)
			binding := newPermTestBinding(t, w, ch.Id, admin.Id)
			err := w.CheckIntentPermission(ctx, permTestPrincipal(w, ch, admin.Id, binding.Id), "seo_keyword", "list")
			if !errors.Is(err, ErrChannelPolicyDenied) {
				t.Fatalf("缺省必须拒绝，实际：%v", err)
			}
		})
	}

	// 反向对照：显式 allow 时必须放行，否则「全部拒绝」的实现也能通过上面的用例。
	ch := newPermTestChannel(t, w, `{"intents":{"seo_keyword":{"list":"allow"}}}`, 1)
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})
	admin := newPermTestAdmin(t, w, []string{"/plugin/contentops"}, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	if err := w.CheckIntentPermission(ctx, permTestPrincipal(w, ch, admin.Id, binding.Id), "seo_keyword", "list"); err != nil {
		t.Fatalf("显式 allow 应放行，实际：%v", err)
	}
}

func TestChannelPolicyDecisionTable(t *testing.T) {
	// 空串 / 全空白 = 未配置，判定必须一律 deny，且解析错误不算致命。
	for _, raw := range []string{"", "   ", "{}"} {
		policy, err := ParseChannelPolicy(raw)
		if err != nil {
			t.Fatalf("raw=%q 不应返回解析错误：%v", raw, err)
		}
		if got := policy.Decision("seo_keyword", "list"); got != PolicyDecisionDeny {
			t.Errorf("raw=%q 必须判定 deny，实际 %q", raw, got)
		}
	}

	// 解析失败：返回错误，但判定必须仍是 deny（不能因为解析错就放行）。
	policy, err := ParseChannelPolicy(`{"intents":`)
	if err == nil {
		t.Error("非法 JSON 必须返回解析错误")
	}
	if got := policy.Decision("seo_keyword", "list"); got != PolicyDecisionDeny {
		t.Errorf("解析失败时必须判定 deny，实际 %q", got)
	}

	raw := `{
		"intents": {
			"seo_keyword": {"list": "allow", "create": "confirm", "*": "deny", "delete": "yes"},
			"content_article": {"*": "allow"}
		},
		"unknown_top_level": {"x": 1}
	}`
	policy, err = ParseChannelPolicy(raw)
	if err != nil {
		t.Fatalf("合法策略不应报错：%v", err)
	}
	cases := []struct {
		intent, action string
		want           PolicyDecision
	}{
		{"seo_keyword", "list", PolicyDecisionAllow},     // 精确命中
		{"seo_keyword", "create", PolicyDecisionConfirm}, // 精确命中
		{"seo_keyword", "delete", PolicyDecisionDeny},    // 非法取值按 deny
		{"seo_keyword", "update", PolicyDecisionDeny},    // 落到 "*"
		{"content_article", "save", PolicyDecisionAllow}, // 意图级 "*"
		{"content_article", "", PolicyDecisionAllow},     // action 为空也走 "*"
		{"seo_anchor", "list", PolicyDecisionDeny},       // 未配置的意图
	}
	for _, tc := range cases {
		if got := policy.Decision(tc.intent, tc.action); got != tc.want {
			t.Errorf("Decision(%q,%q) = %q, want %q", tc.intent, tc.action, got, tc.want)
		}
	}
}

func TestChannelSwitchNeedsConfirmSentinel(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w, `{"intents":{"seo_keyword":{"list":"confirm","create":"allow"}}}`, 1)
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})
	admin := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	p := permTestPrincipal(w, ch, admin.Id, binding.Id)

	err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list")
	if !errors.Is(err, ErrChannelPolicyNeedsConfirm) {
		t.Fatalf("confirm 必须返回可 errors.Is 的 ErrChannelPolicyNeedsConfirm，实际：%v", err)
	}
	if errors.Is(err, ErrChannelPolicyDenied) {
		t.Fatalf("confirm 不能被当成 deny：%v", err)
	}
	// 审批通过后的复核：除 confirm 外其余门禁照旧生效，不能再次要求确认。
	if err := w.CheckIntentPermissionApproved(ctx, p, "seo_keyword", "list"); err != nil {
		t.Fatalf("审批后复核应放行，实际：%v", err)
	}
	// allow 路径不应被哨兵误伤。
	if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "create"); err != nil {
		t.Fatalf("allow 应放行，实际：%v", err)
	}
}

func TestAgentWhitelistDeniesUnlistedIntent(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w, `{"intents":{"seo_keyword":{"*":"allow"}}}`, 1)
	admin := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	p := permTestPrincipal(w, ch, admin.Id, binding.Id)

	// 无记录
	err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list")
	if !errors.Is(err, ErrChannelAgentNotAllowed) {
		t.Fatalf("无接待策略必须拒绝，实际：%v", err)
	}
	// 空数组
	setPermTestAllowedIntents(t, w, ch.AgentId, nil)
	if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); !errors.Is(err, ErrChannelAgentNotAllowed) {
		t.Fatalf("空白名单必须拒绝，实际：%v", err)
	}
	// 未列名
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"content_article"})
	if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); !errors.Is(err, ErrChannelAgentNotAllowed) {
		t.Fatalf("白名单未列该意图必须拒绝，实际：%v", err)
	}
	// 反向对照：列名后必须放行（白名单不支持通配符，只认精确名）
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})
	if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); err != nil {
		t.Fatalf("白名单含该意图应放行，实际：%v", err)
	}
	// 通配符不被支持：写了 "*" 也不能放行
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"*"})
	if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); !errors.Is(err, ErrChannelAgentNotAllowed) {
		t.Fatalf("智能体白名单不支持通配符，实际：%v", err)
	}
}

func TestPolicyRevisionMismatchDenied(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w, `{"intents":{"seo_keyword":{"*":"allow"}}}`, 3)
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})
	admin := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)

	stale := permTestPrincipal(w, ch, admin.Id, binding.Id)
	stale.PolicyRevision = 2
	err := w.CheckIntentPermission(ctx, stale, "seo_keyword", "list")
	if !errors.Is(err, ErrChannelPolicyStale) {
		t.Fatalf("策略修订号不一致必须返回 ErrChannelPolicyStale，实际：%v", err)
	}

	current := permTestPrincipal(w, ch, admin.Id, binding.Id)
	if current.PolicyRevision != 3 {
		t.Fatalf("夹具修订号应为 3，实际 %d", current.PolicyRevision)
	}
	if err := w.CheckIntentPermission(ctx, current, "seo_keyword", "list"); err != nil {
		t.Fatalf("修订号一致应放行，实际：%v", err)
	}
}

// ============================================================
// api_invoke 最终路径黑名单
// ============================================================

func TestAPIInvokeBlocksChannelAdminPaths(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w, `{"intents":{"api":{"*":"allow"}}}`, 1)
	binding := newPermTestBinding(t, w, ch.Id, 1)
	p := permTestPrincipal(w, ch, 1, binding.Id)

	blocked := []string{
		"/channels/list",
		"/system/api/channels/detail",
		"/system/api/channels/binding/revoke",
		"/system/api/admin/list",
		"/system/api/website/list",
		"/system/api/setting/system",
		"/system/api/anqi/skill/list",
	}
	for _, path := range blocked {
		args := fmt.Sprintf(`{"method":"POST","path":%q}`, path)
		err := w.CheckCapPermission(ctx, p, "api_invoke", args)
		if !errors.Is(err, ErrChannelAPIPathBlocked) {
			t.Errorf("path=%s 必须返回 ErrChannelAPIPathBlocked，实际：%v", path, err)
		}
	}

	// 非黑名单路径（真实端点）应通过。
	if err := w.CheckCapPermission(ctx, p, "api_invoke", `{"method":"GET","path":"/system/api/archive/list"}`); err != nil {
		t.Fatalf("非黑名单端点应通过，实际：%v", err)
	}
	// 目录里不存在的路径不能靠「不在黑名单」混过去（无法映射权限 → 拒绝）。
	err := w.CheckCapPermission(ctx, p, "api_invoke", `{"method":"GET","path":"/system/api/no-such-endpoint-`+channelPermSuffix()+`"}`)
	if !errors.Is(err, ErrChannelCapBlocked) {
		t.Fatalf("目录中不存在的路径必须拒绝，实际：%v", err)
	}
}

func TestAPIInvokeBlocksEncodedOrPrefixedChannelPaths(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w, `{"intents":{"api":{"*":"allow"}}}`, 1)
	binding := newPermTestBinding(t, w, ch.Id, 1)
	p := permTestPrincipal(w, ch, 1, binding.Id)

	variants := []string{
		"channels/detail",                             // 无前缀
		"/channels/detail",                            // 单斜杠前缀
		"//channels/detail",                           // 双斜杠前缀
		"/system/api/channels/detail",                 // 完整前缀
		"/SYSTEM/API/Channels/Detail",                 // 大小写变体
		"/system/api/./channels/detail",               // 冗余当前目录
		"/system/api//channels/detail",                // 重复分隔符
		"/system/api/channels/detail/../binding/list", // 祖先回溯
		"/system/api/channels/",                       // 尾斜杠
		"/system/api/ADMIN/group/list",                // 管理员路径大小写变体
	}
	for _, raw := range variants {
		args := fmt.Sprintf(`{"method":"POST","path":%q}`, raw)
		err := w.CheckCapPermission(ctx, p, "api_invoke", args)
		if !errors.Is(err, ErrChannelAPIPathBlocked) {
			t.Errorf("path=%q 必须命中黑名单，实际：%v", raw, err)
		}
	}

	// 边界对照：前缀只按段匹配，channelsfoo 不是 /channels。
	args := fmt.Sprintf(`{"method":"GET","path":%q}`, "/system/api/channelsfoo/list")
	err := w.CheckCapPermission(ctx, p, "api_invoke", args)
	if errors.Is(err, ErrChannelAPIPathBlocked) {
		t.Fatalf("channelsfoo 不应被当成 /channels 前缀命中：%v", err)
	}
	if !errors.Is(err, ErrChannelCapBlocked) {
		t.Fatalf("不存在的端点应被映射门禁拒绝，实际：%v", err)
	}
}

// ============================================================
// 站点曝光门禁
// ============================================================

// TestSiteExposureGateUsesOwnSiteConfig 曝光门禁必须读「该站点自己的」配置，
// 不能拿主站点/进程级配置顶替（provider/mcp.go 的 GetMcpConfig 只返回主站点配置）。
func TestSiteExposureGateUsesOwnSiteConfig(t *testing.T) {
	w := channelPermTestSite(t)

	// 场景 1：站点配置了白名单后，只有被点名的意图放行；DefaultOff 未被点名必须拒绝。
	withTestAiSetting(t, w, []string{"content_article"})
	if err := w.checkChannelIntentExposure("content_article"); err != nil {
		t.Fatalf("被点名的意图应放行，实际：%v", err)
	}
	if err := w.checkChannelIntentExposure("seo_keyword"); !errors.Is(err, ErrChannelIntentNotExposed) {
		t.Fatalf("白名单非空但未点名的意图必须拒绝，实际：%v", err)
	}
	if err := w.checkChannelIntentExposure("system_plugin"); !errors.Is(err, ErrChannelIntentNotExposed) {
		t.Fatalf("DefaultOff 意图未被点名时必须拒绝，实际：%v", err)
	}
	// 反向对照：空配置下同一意图是放行的，说明上面拒绝来自站点自己的白名单，
	// 而不是「所有意图默认拒绝」的实现。
	if !intent.IntentAllowed(intent.Config{}, "seo_keyword") {
		t.Fatal("空曝光配置下 seo_keyword 本应放行，对照失效")
	}

	// 场景 2：换一个站点（Id=2）不该继承站点 1 的收窄白名单。
	// 这里直接构造只带 Id/DB 的站点实例：非主站点的 LoadAiSetting 会立刻返回空配置，
	// 不会去碰站点 1 的缓存，正好用来证明门禁读的是「站点自己的」配置而不是主站点配置。
	w2 := &Website{Id: 2, DB: w.DB}
	if err := w2.checkChannelIntentExposure("seo_keyword"); err != nil {
		t.Fatalf("站点 2 不应继承站点 1 的曝光白名单，实际：%v", err)
	}

	// 场景 3：端到端——渠道意图检查在曝光门禁处拒绝 DefaultOff 意图。
	p := ExecutionPrincipal{SiteId: w.Id, AdminId: 1, BindingId: 1, ChannelId: 1, SessionId: "perm-exposure", PolicyRevision: 1}
	err := w.CheckIntentPermission(context.Background(), p, "system_plugin", "backup_dump")
	if !errors.Is(err, ErrChannelIntentNotExposed) {
		t.Fatalf("DefaultOff 意图必须被站点曝光门禁拒绝，实际：%v", err)
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
