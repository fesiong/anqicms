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
	if err := w.DB.AutoMigrate(&model.AgentReceptionPolicy{}, &model.ChannelApproval{}); err != nil {
		t.Fatalf("迁移接待策略/审批表失败：%v", err)
	}
	return w
}

func channelPermSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// newPermTestChannel 建一条「启用 + 开启聊天管理」的渠道。AppId 必带随机后缀：
// uk_app_identity_hash 全局唯一，固定值会撞上库里残留行。AgentId 同样取随机值，
// 避免撞上他人策略。
func newPermTestChannel(t *testing.T, w *Website, policyJSON string, revision int) *model.MessageChannel {
	t.Helper()
	return newPermTestChannelState(t, w, policyJSON, revision, 1, 1)
}

// newPermTestChannelState 允许指定 enabled / chatEnabled，用于验证
// 「渠道停用」「仅通知渠道」必须在权限层被拒绝（R36）。
func newPermTestChannelState(t *testing.T, w *Website, policyJSON string, revision int, enabled, chatEnabled int) *model.MessageChannel {
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

// permTestPrincipal 构造执行主体。AgentId 取渠道配置的接待智能体：
// R35 起门禁要求「执行主体声明的智能体」非 0 且与渠道一致。
func permTestPrincipal(w *Website, ch *model.MessageChannel, adminId, bindingId uint) ExecutionPrincipal {
	return ExecutionPrincipal{
		SiteId:         w.Id,
		AdminId:        adminId,
		BindingId:      bindingId,
		ChannelId:      ch.Id,
		AgentId:        ch.AgentId,
		SessionId:      "perm-sess-" + channelPermSuffix(),
		PolicyRevision: int64(ch.Revision),
	}
}

// newPermTestApproval 建一条审批记录，并在用例结束时按自己的 ID 精确删除。
// mutate 用于构造过期 / 策略 revision 不符 / 他人绑定 / 参数哈希不符等无效变体；
// 若 mutate 把 ArgsHash 改了，这里不会再覆盖（保持调用方意图）。
//
// Action 与 Tool 一样是审批单的**身份字段**（R58）：action 从 argsJson 里取原始值
// （T7 落库口径就是 Pending.Action = actionOfArgsJSON(args)），没有 action 键就是空串。
// 硬编码成某个值会让「审批行 action 与调用 action 是否一致」这条判定在测试里空转。
func newPermTestApproval(t *testing.T, w *Website, bindingId uint, policyRevision int, argsJson string, mutate func(*model.ChannelApproval)) *model.ChannelApproval {
	t.Helper()
	approval := &model.ChannelApproval{
		SiteId:         w.Id,
		BindingId:      bindingId,
		Tool:           "seo_keyword",
		Action:         actionOfArgsJSON(argsJson),
		CanonicalArgs:  argsJson,
		ArgsHash:       ChannelArgsHash(argsJson),
		ExpiresAt:      time.Now().Unix() + 600,
		PolicyRevision: policyRevision,
		Status:         model.ApprovalStatusApproved,
		PublicId:       "perm-approval-" + channelPermSuffix(),
	}
	if mutate != nil {
		mutate(approval)
	}
	if err := w.DB.Create(approval).Error; err != nil {
		t.Fatalf("创建测试审批失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ? AND site_id = ?", approval.Id, w.Id).Delete(&model.ChannelApproval{}).Error; err != nil {
			t.Logf("清理审批 %d 失败：%v", approval.Id, err)
		}
	})
	return approval
}

// ============================================================
// 能力 → 权限映射
// ============================================================

// TestEveryCapHasPermissionDecision 遍历真实的 capHandlers：每个 cap 都必须
// 要么映射到后台菜单权限路径，要么显式 blocked。任何「未解析」都判失败。
//
// 「端点查不到」单独统计并断言为 0（R37）：路由改名 / api_catalog 过期会让整类
// 能力静默变成 blocked，行为上仍是拒绝（安全），但功能会悄悄消失而不变红——
// 这类漂移必须在测试里点名。
func TestEveryCapHasPermissionDecision(t *testing.T) {
	w := channelPermTestSite(t)
	svc := &AiChatService{db: w.DB, site: w}
	caps := svc.assembleCapHandlers()
	if len(caps) == 0 {
		t.Fatal("capHandlers 为空，无法校验权限映射")
	}

	var menuMapped, dynamic, builtinBlocked, unmappedBlocked int
	var undecided, missingEndpoints []string
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
			switch {
			case perm.MissingEndpoint:
				missingEndpoints = append(missingEndpoints, name)
			case perm.Builtin:
				builtinBlocked++
			default:
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
	sort.Strings(missingEndpoints)
	t.Logf("cap 总数=%d 菜单映射=%d 动态解析=%d 内置关闭=%d 未映射关闭=%d 端点缺失=%d\n%s",
		len(caps), menuMapped, dynamic, builtinBlocked, unmappedBlocked, len(missingEndpoints), strings.Join(names, ", "))
	if len(missingEndpoints) > 0 {
		t.Errorf("以下 cap 声明的端点不在端点目录中（route/ 改名或目录过期，跑 go run ./cmd/apicatalog 重新生成）：%s",
			strings.Join(missingEndpoints, ", "))
	}

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
		SiteId: w.Id, AdminId: 1, BindingId: 1, ChannelId: 1, AgentId: 1,
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
	// 审批通过后的复核：必须带审批凭据自证（R34），除 confirm 外其余门禁照旧生效。
	argsJson := `{"action":"list","title":"confirm-sentinel"}`
	approval := newPermTestApproval(t, w, binding.Id, ch.Revision, argsJson, nil)
	if err := w.CheckIntentPermissionApproved(ctx, p, approval.PublicId, argsJson, "seo_keyword", "list"); err != nil {
		t.Fatalf("持有效审批的复核应放行，实际：%v", err)
	}
	// allow 路径不应被哨兵误伤，也不该要求审批凭据。
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
// 绑定有效性 / 渠道状态门禁（R36、规格 §6）
// ============================================================

// TestRevokedBindingIsDenied 绑定是执行主体的身份凭据：撤销、借他人绑定、跨渠道绑定
// 都必须立即失效（规格 §6）。三个子用例都真实改库 / 真实建库行，不是只改内存结构——
// 删掉 checkChannelBinding 里的 revoked_at = 0、admin_id 或 channel_id 条件都会变红。
func TestRevokedBindingIsDenied(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	policy := `{"intents":{"seo_keyword":{"*":"allow"}}}`

	// 公共夹具：放行 seo_keyword 的渠道 + 有菜单权限的管理员 + 有效绑定。
	setup := func(t *testing.T) (*model.MessageChannel, *model.Admin, *model.ChannelBinding) {
		t.Helper()
		ch := newPermTestChannel(t, w, policy, 1)
		setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})
		admin := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
		binding := newPermTestBinding(t, w, ch.Id, admin.Id)
		return ch, admin, binding
	}

	t.Run("撤销后立即失效", func(t *testing.T) {
		ch, admin, binding := setup(t)
		p := permTestPrincipal(w, ch, admin.Id, binding.Id)
		if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); err != nil {
			t.Fatalf("撤销前应放行（基线），实际：%v", err)
		}
		// 真实改库：revoked_at 非 0。
		if err := w.DB.Model(&model.ChannelBinding{}).
			Where("id = ? AND site_id = ?", binding.Id, w.Id).
			Update("revoked_at", time.Now().Unix()).Error; err != nil {
			t.Fatalf("撤销绑定失败：%v", err)
		}
		if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); !errors.Is(err, ErrChannelPrincipalInvalid) {
			t.Fatalf("已撤销绑定必须返回 ErrChannelPrincipalInvalid，实际：%v", err)
		}
	})

	t.Run("绑定属于他管理员", func(t *testing.T) {
		ch, admin, _ := setup(t)
		// 库里真实存在一条属于另一个管理员的绑定，principal 的 AdminId 指向本人。
		other := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
		otherBinding := newPermTestBinding(t, w, ch.Id, other.Id)
		p := permTestPrincipal(w, ch, admin.Id, otherBinding.Id)
		if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); !errors.Is(err, ErrChannelPrincipalInvalid) {
			t.Fatalf("借用他人绑定必须返回 ErrChannelPrincipalInvalid，实际：%v", err)
		}
	})

	t.Run("绑定属于他渠道", func(t *testing.T) {
		chA, admin, bindingA := setup(t)
		chB := newPermTestChannel(t, w, policy, 1)
		setPermTestAllowedIntents(t, w, chB.AgentId, []string{"seo_keyword"})
		// principal 声明渠道 B，却拿渠道 A 的绑定 ID：渠道与绑定必须一一对应。
		p := permTestPrincipal(w, chB, admin.Id, bindingA.Id)
		if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); !errors.Is(err, ErrChannelPrincipalInvalid) {
			t.Fatalf("跨渠道绑定必须返回 ErrChannelPrincipalInvalid，实际：%v", err)
		}
		// 反向对照：换成渠道 A 自己的绑定必须放行（否决「一律拒绝」的实现）。
		if err := w.CheckIntentPermission(ctx, permTestPrincipal(w, chA, admin.Id, bindingA.Id), "seo_keyword", "list"); err != nil {
			t.Fatalf("本渠道绑定应放行，实际：%v", err)
		}
	})
}

// TestChannelDisabledIsDenied 渠道停用（enabled = 0）时，意图级与每次 cap 复核都必须拒绝。
func TestChannelDisabledIsDenied(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannelState(t, w, `{"intents":{"seo_keyword":{"*":"allow"}}}`, 1, 0, 1)
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})
	admin := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	p := permTestPrincipal(w, ch, admin.Id, binding.Id)

	if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); !errors.Is(err, ErrChannelDisabled) {
		t.Fatalf("停用渠道的意图检查必须返回 ErrChannelDisabled，实际：%v", err)
	}
	if err := w.CheckCapPermission(ctx, p, "api_list", `{}`); !errors.Is(err, ErrChannelDisabled) {
		t.Fatalf("停用渠道的 cap 复核必须返回 ErrChannelDisabled，实际：%v", err)
	}
}

// TestChatDisabledIsDenied 「仅通知」渠道（chat_enabled = 0）不得执行任何管理动作。
func TestChatDisabledIsDenied(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannelState(t, w, `{"intents":{"seo_keyword":{"*":"allow"}}}`, 1, 1, 0)
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})
	admin := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	p := permTestPrincipal(w, ch, admin.Id, binding.Id)

	if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); !errors.Is(err, ErrChannelDisabled) {
		t.Fatalf("仅通知渠道的意图检查必须返回 ErrChannelDisabled，实际：%v", err)
	}
	if err := w.CheckCapPermission(ctx, p, "api_list", `{}`); !errors.Is(err, ErrChannelDisabled) {
		t.Fatalf("仅通知渠道的 cap 复核必须返回 ErrChannelDisabled，实际：%v", err)
	}
}

// TestAgentPrincipalMustMatchChannel 白名单校验的主体必须与执行主体是同一个智能体（R35）：
// 会话切到另一个智能体后，不能拿渠道里配置的接待智能体的白名单去放行。
//
// 反向构造：给「另一个智能体」也写一份允许该意图的白名单，这样测试失败只可能来自
// 「执行智能体 ≠ 渠道接待智能体」这条判定本身，而不是白名单缺记录。
func TestAgentPrincipalMustMatchChannel(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w, `{"intents":{"seo_keyword":{"*":"allow"}}}`, 1)
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})
	admin := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	p := permTestPrincipal(w, ch, admin.Id, binding.Id)

	if err := w.CheckIntentPermission(ctx, p, "seo_keyword", "list"); err != nil {
		t.Fatalf("主体智能体与渠道一致时应放行（基线），实际：%v", err)
	}

	// 另一个智能体也有同样的白名单，但 principal 声明的智能体不是渠道的接待智能体。
	otherAgentId := ch.AgentId + 1
	setPermTestAllowedIntents(t, w, otherAgentId, []string{"seo_keyword"})
	t.Cleanup(func() {
		if err := w.DB.Where("site_id = ? AND agent_id = ?", w.Id, otherAgentId).Delete(&model.AgentReceptionPolicy{}).Error; err != nil {
			t.Logf("清理对照智能体 %d 的接待策略失败：%v", otherAgentId, err)
		}
	})
	switched := p
	switched.AgentId = otherAgentId
	if err := w.CheckIntentPermission(ctx, switched, "seo_keyword", "list"); !errors.Is(err, ErrChannelAgentNotAllowed) {
		t.Fatalf("执行智能体与渠道接待智能体不一致必须返回 ErrChannelAgentNotAllowed，实际：%v", err)
	}

	// AgentId 为 0 同样拒绝（缺省拒绝，不猜渠道配置）。
	missing := p
	missing.AgentId = 0
	if err := w.CheckIntentPermission(ctx, missing, "seo_keyword", "list"); !errors.Is(err, ErrChannelAgentNotAllowed) {
		t.Fatalf("缺少执行智能体必须返回 ErrChannelAgentNotAllowed，实际：%v", err)
	}
}

// ============================================================
// 审批自证（R34）
// ============================================================

// TestApprovalProof 验证 CheckIntentPermissionApproved 是「跳过 confirm」的唯一入口，
// 且必须自证审批：审批标识、绑定、过期、参数哈希、策略修订号任一不符即拒绝。
// 同时验证「审批通过」不是总开关：曝光、菜单权限、绑定有效性仍然逐项生效。
func TestApprovalProof(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w, `{"intents":{"seo_keyword":{"list":"confirm","create":"allow"}}}`, 1)
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword"})
	admin := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	p := permTestPrincipal(w, ch, admin.Id, binding.Id)
	// R58：参数里带 action，审批行的 action 由它派生，与调用方传入的 action 逐字比对。
	argsJson := `{"action":"list","title":"审批自证"}`

	validApproval := func(t *testing.T) *model.ChannelApproval {
		t.Helper()
		return newPermTestApproval(t, w, binding.Id, ch.Revision, argsJson, nil)
	}

	rejectCases := []struct {
		name  string
		setup func(t *testing.T) (publicId string, args string)
	}{
		{
			name:  "无审批标识",
			setup: func(t *testing.T) (string, string) { return "   ", argsJson },
		},
		{
			name: "审批记录不存在",
			setup: func(t *testing.T) (string, string) {
				return "no-such-approval-" + channelPermSuffix(), argsJson
			},
		},
		{
			name: "参数不是合法 JSON",
			setup: func(t *testing.T) (string, string) {
				return validApproval(t).PublicId, `{"title":`
			},
		},
		{
			name: "参数哈希不符",
			setup: func(t *testing.T) (string, string) {
				return validApproval(t).PublicId, `{"title":"被篡改的参数"}`
			},
		},
		{
			name: "审批属于他人绑定",
			setup: func(t *testing.T) (string, string) {
				other := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
				otherBinding := newPermTestBinding(t, w, ch.Id, other.Id)
				return newPermTestApproval(t, w, otherBinding.Id, ch.Revision, argsJson, nil).PublicId, argsJson
			},
		},
		{
			name: "审批已过期",
			setup: func(t *testing.T) (string, string) {
				return newPermTestApproval(t, w, binding.Id, ch.Revision, argsJson, func(a *model.ChannelApproval) {
					a.ExpiresAt = time.Now().Unix() - 1
				}).PublicId, argsJson
			},
		},
		{
			name: "策略修订号不符",
			setup: func(t *testing.T) (string, string) {
				return newPermTestApproval(t, w, binding.Id, ch.Revision+1, argsJson, nil).PublicId, argsJson
			},
		},
		{
			name: "未批准（pending）",
			setup: func(t *testing.T) (string, string) {
				return newPermTestApproval(t, w, binding.Id, ch.Revision, argsJson, func(a *model.ChannelApproval) {
					a.Status = model.ApprovalStatusPending
				}).PublicId, argsJson
			},
		},
	}
	for _, tc := range rejectCases {
		t.Run(tc.name, func(t *testing.T) {
			publicId, args := tc.setup(t)
			err := w.CheckIntentPermissionApproved(ctx, p, publicId, args, "seo_keyword", "list")
			if !errors.Is(err, ErrChannelApprovalInvalid) {
				t.Fatalf("必须返回 ErrChannelApprovalInvalid，实际：%v", err)
			}
		})
	}

	t.Run("有效审批放行", func(t *testing.T) {
		if err := w.CheckIntentPermissionApproved(ctx, p, validApproval(t).PublicId, argsJson, "seo_keyword", "list"); err != nil {
			t.Fatalf("有效审批应放行，实际：%v", err)
		}
	})

	t.Run("executing 状态同样有效", func(t *testing.T) {
		approval := newPermTestApproval(t, w, binding.Id, ch.Revision, argsJson, func(a *model.ChannelApproval) {
			a.Status = model.ApprovalStatusExecuting
		})
		if err := w.CheckIntentPermissionApproved(ctx, p, approval.PublicId, argsJson, "seo_keyword", "list"); err != nil {
			t.Fatalf("executing 状态的审批应放行，实际：%v", err)
		}
	})

	// 以下三个子用例证明「审批通过」不是总开关：其余门禁仍逐项生效。
	t.Run("有效审批不能越过站点曝光", func(t *testing.T) {
		withTestAiSetting(t, w, []string{"content_article"})
		err := w.CheckIntentPermissionApproved(ctx, p, validApproval(t).PublicId, argsJson, "seo_keyword", "list")
		if !errors.Is(err, ErrChannelIntentNotExposed) {
			t.Fatalf("有效审批仍必须受站点曝光门禁约束，实际：%v", err)
		}
	})

	t.Run("有效审批不能越过菜单权限", func(t *testing.T) {
		noMenu := newPermTestAdmin(t, w, []string{"/account/list"}, 1)
		noMenuBinding := newPermTestBinding(t, w, ch.Id, noMenu.Id)
		noMenuP := permTestPrincipal(w, ch, noMenu.Id, noMenuBinding.Id)
		approval := newPermTestApproval(t, w, noMenuBinding.Id, ch.Revision, argsJson, nil)
		err := w.CheckIntentPermissionApproved(ctx, noMenuP, approval.PublicId, argsJson, "seo_keyword", "list")
		if !errors.Is(err, ErrChannelPermissionDenied) {
			t.Fatalf("有效审批仍必须受菜单权限约束，实际：%v", err)
		}
	})

	t.Run("有效审批不能越过绑定撤销", func(t *testing.T) {
		revoked := newPermTestAdmin(t, w, []string{"/plugin/contentops/keyword"}, 1)
		revokedBinding := newPermTestBinding(t, w, ch.Id, revoked.Id)
		approval := newPermTestApproval(t, w, revokedBinding.Id, ch.Revision, argsJson, nil)
		revokedP := permTestPrincipal(w, ch, revoked.Id, revokedBinding.Id)
		if err := w.DB.Model(&model.ChannelBinding{}).
			Where("id = ? AND site_id = ?", revokedBinding.Id, w.Id).
			Update("revoked_at", time.Now().Unix()).Error; err != nil {
			t.Fatalf("撤销绑定失败：%v", err)
		}
		err := w.CheckIntentPermissionApproved(ctx, revokedP, approval.PublicId, argsJson, "seo_keyword", "list")
		if !errors.Is(err, ErrChannelPrincipalInvalid) {
			t.Fatalf("有效审批仍必须受绑定有效性约束，实际：%v", err)
		}
	})
}

// ============================================================
// 审批自证必须绑定工具身份（R58）
// ============================================================

// TestApprovalProofBindsToolIdentity 证明审批凭据不是「一张参数哈希票」：
// 审批行的 tool / action 是「管理员批准了哪一次操作」的身份字段，必须逐字比对。
//
// 越权路径（本轮实测）：审批行 tool=seo_keyword、args={"action":"list","page":1}，
// 复核时传 intentName=content_article + 同样参数 → 只比摘要时摘要一致，于是
// 「确认一个工具，可以执行另一个工具」。本用例的 exposure / 白名单 / 策略 / 菜单权限
// 都为 content_article 放行，因此去掉 tool/action 比对后它必然会通过——
// 失败只可能来自工具身份这一条判定（反向对照有效）。
func TestApprovalProofBindsToolIdentity(t *testing.T) {
	w := channelPermTestSite(t)
	ctx := context.Background()
	ch := newPermTestChannel(t, w,
		`{"intents":{"seo_keyword":{"*":"allow"},"content_article":{"*":"allow"}}}`, 1)
	setPermTestAllowedIntents(t, w, ch.AgentId, []string{"seo_keyword", "content_article"})
	admin := newPermTestAdmin(t, w, []string{"/"}, 1)
	binding := newPermTestBinding(t, w, ch.Id, admin.Id)
	p := permTestPrincipal(w, ch, admin.Id, binding.Id)
	withTestAiSetting(t, w, []string{"seo_keyword", "content_article"})

	argsJson := `{"action":"list","page":1}`
	row := newPermTestApproval(t, w, binding.Id, ch.Revision, argsJson, nil)

	// 1) 参数完全一致、工具不同：必须拒绝。
	if err := w.CheckIntentPermissionApproved(ctx, p, row.PublicId, argsJson, "content_article", "list"); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("审批行 tool=seo_keyword 时用 content_article 复核必须返回 ErrChannelApprovalInvalid，实际：%v", err)
	}
	// 2) 工具一致、action 不同（含空 action）：同样必须拒绝。
	for _, action := range []string{"create", ""} {
		if err := w.CheckIntentPermissionApproved(ctx, p, row.PublicId, argsJson, "seo_keyword", action); !errors.Is(err, ErrChannelApprovalInvalid) {
			t.Fatalf("审批行 action=list 时用 action=%q 复核必须返回 ErrChannelApprovalInvalid，实际：%v", action, err)
		}
	}
	// 3) 完全一致：必须放行（反向对照，否决「一律拒绝」的实现）。
	if err := w.CheckIntentPermissionApproved(ctx, p, row.PublicId, argsJson, "seo_keyword", "list"); err != nil {
		t.Fatalf("工具与参数都与审批行一致时必须放行，实际：%v", err)
	}

	// 4) 空 action 就用空 action 比，**不回落 "*"**：审批行 action 取自 ArgsJSON 原始值，
	// 无 action 键即空串；拿 "*" 去比必须对不上（否则「批准一个无动作调用」=「批准全部动作」）。
	noActionArgs := `{"title":"无动作参数"}`
	noActionRow := newPermTestApproval(t, w, binding.Id, ch.Revision, noActionArgs, nil)
	if noActionRow.Action != "" {
		t.Fatalf("ArgsJSON 没有 action 时审批行 action 必须是空串，实际：%q", noActionRow.Action)
	}
	if err := w.CheckIntentPermissionApproved(ctx, p, noActionRow.PublicId, noActionArgs, "seo_keyword", ""); err != nil {
		t.Fatalf("空 action 与审批行空 action 一致时必须放行（策略仍按 * 兜底），实际：%v", err)
	}
	if err := w.CheckIntentPermissionApproved(ctx, p, noActionRow.PublicId, noActionArgs, "seo_keyword", "*"); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("用 \"*\" 复核空 action 的审批行必须被拒（不得回落通配符），实际：%v", err)
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
	p := ExecutionPrincipal{SiteId: w.Id, AdminId: 1, BindingId: 1, ChannelId: 1, AgentId: 1, SessionId: "perm-exposure", PolicyRevision: 1}
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
