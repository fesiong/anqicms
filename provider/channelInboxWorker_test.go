package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// ============================================================
// T12a：入站消费、审批执行、写门禁注入、会话代次。
//
// 夹具复用 channelChatTestSite / newChannelTurnTestEnv（真实库、随机后缀、
// 按自己创建的 ID 精确清理）。本文件只补三样：
//   - 一条把「平台事件 → ChannelInbox」真实走一遍的入口（wk.handleInbound），
//     这样 PlatformChatId / IdentityHash / BindingId 的补齐是端到端被证明的；
//   - 假适配器的发送记录（复用 channelManager_test.go 的 fakeChannelAdapter）；
//   - 把 runChannelTurnForInbox 换成「svc.runChannelTurn + 脚本模型」，不触达真实 LLM。
// ============================================================

type channelInboxTestConfig struct {
	policy      string
	whitelist   []string
	permissions []string
	exposed     []string
	chatEnabled int
}

type channelInboxTestEnv struct {
	*channelTurnTestEnv
	adapter  *fakeChannelAdapter
	consumer *channelInboxConsumer
	model    *scriptedChannelModel
}

func newChannelInboxTestEnv(t *testing.T, cfg channelInboxTestConfig) *channelInboxTestEnv {
	t.Helper()
	w := channelChatTestSite(t)
	if err := w.DB.AutoMigrate(&model.ChannelInbox{}); err != nil {
		t.Fatalf("迁移入站任务表失败：%v", err)
	}
	withTestAiSetting(t, w, cfg.exposed)

	chatEnabled := cfg.chatEnabled
	if chatEnabled == 0 {
		chatEnabled = 1
	}
	base := newChannelTurnTestEnv(t, w, channelTurnTestConfig{
		policy:      cfg.policy,
		whitelist:   cfg.whitelist,
		permissions: cfg.permissions,
		enabled:     1,
		chatEnabled: chatEnabled,
	})

	// 消费侧从站点上取服务（生产路径就是 w.AiSrv），用完还原，避免污染其它用例。
	prev := w.AiSrv
	w.AiSrv = base.svc
	t.Cleanup(func() { w.AiSrv = prev })

	// 绑定夹具的 OrgId/IdentityHash 是随机值；真实的 ConsumeBindingToken 落库的是
	// bindingIdentityDigest(站点, 渠道, 组织, 平台用户)。消费侧只按该摘要认人，
	// 这里把夹具校正成与生产同口径，避免「夹具摘要与代码口径不一致」的假失败。
	canonicalOrg := base.channel.OrgId
	canonicalHash := bindingIdentityDigest(w.Id, base.channel.Id, canonicalOrg, base.binding.PlatformUserId)
	if err := w.DB.Model(&model.ChannelBinding{}).
		Where("id = ? AND site_id = ?", base.binding.Id, w.Id).
		Updates(map[string]any{"org_id": canonicalOrg, "identity_hash": canonicalHash}).Error; err != nil {
		t.Fatalf("校正测试绑定身份摘要失败：%v", err)
	}
	base.binding.OrgId = canonicalOrg
	base.binding.IdentityHash = canonicalHash

	adapter := newFakeChannelAdapter()
	consumer := newChannelInboxConsumer(w, base.channel.Id, adapter)
	consumer.pollEvery = 10 * time.Millisecond

	// 注册顺序在渠道清理之后 → LIFO 先跑本清理，不会留下悬空的入站任务与会话。
	t.Cleanup(func() {
		var sessions []model.ChannelSession
		if err := w.DB.Where("site_id = ? AND channel_id = ?", w.Id, base.channel.Id).
			Find(&sessions).Error; err == nil {
			for _, session := range sessions {
				if derr := w.DB.Where("session_id = ?", session.ChatSessionId).
					Delete(&model.AiChatMessage{}).Error; derr != nil {
					t.Logf("清理会话消息 %s 失败：%v", session.ChatSessionId, derr)
				}
			}
		}
		if err := w.DB.Where("site_id = ? AND channel_id = ?", w.Id, base.channel.Id).
			Delete(&model.ChannelSession{}).Error; err != nil {
			t.Logf("清理渠道 %d 的会话失败：%v", base.channel.Id, err)
		}
		if err := w.DB.Where("site_id = ? AND channel_id = ?", w.Id, base.channel.Id).
			Delete(&model.ChannelInbox{}).Error; err != nil {
			t.Logf("清理渠道 %d 的入站任务失败：%v", base.channel.Id, err)
		}
	})

	return &channelInboxTestEnv{
		channelTurnTestEnv: base,
		adapter:            adapter,
		consumer:           consumer,
		model:              &scriptedChannelModel{},
	}
}

// useScriptedModel 把执行器接缝换成脚本模型；生产默认实现保持 RunChannelTurn。
func (e *channelInboxTestEnv) useScriptedModel(t *testing.T) {
	t.Helper()
	orig := runChannelTurnForInbox
	runChannelTurnForInbox = func(svc *AiChatService, ctx context.Context, req ChannelTurnRequest) (ChannelTurnResult, error) {
		return svc.runChannelTurn(ctx, req, scriptedChannelFactory(e.model))
	}
	t.Cleanup(func() { runChannelTurnForInbox = orig })
}

// lastMessageIsToolResult 报告本次模型调用是否发生在「本轮工具已执行完」之后。
//
// scriptedChannelModel.rounds 跨轮次累计，脚本用 round 判断「本轮第一次调用」会在
// 第二次入站时失效；模型输入里最后一条是 tool 消息才是可靠的「同一轮里已执行过工具」信号
// （每轮开头最后一条一定是 user 消息，见 channelChat.go 的 messages 构造）。
func lastMessageIsToolResult(in []*schema.Message) bool {
	if len(in) == 0 {
		return false
	}
	return in[len(in)-1].Role == schema.Tool
}

// receiveInbound 走真实入站处理（wk.handleInbound），返回落库后的任务行。
func (e *channelInboxTestEnv) receiveInbound(t *testing.T, ev InboundEvent) *model.ChannelInbox {
	t.Helper()
	ev.OrganizationId = e.channel.OrgId
	ev.AppId = e.channel.AppId
	if ev.EventId == "" {
		ev.EventId = fmt.Sprintf("inbox-evt-%d", time.Now().UnixNano())
	}
	if ev.Kind == "" {
		ev.Kind = InboundKindText
	}
	mgr := &channelManager{site: e.w, opts: defaultChannelManagerOptions()}
	wk := &channelWorker{manager: mgr, channel: *e.channel}
	if err := wk.handleInbound(context.Background(), e.adapter, ev); err != nil {
		t.Fatalf("入站处理返回错误：%v", err)
	}
	hash := wantEventDigest(e.w.Id, e.channel.Id, e.channel.Platform, ev.EventId)
	var row model.ChannelInbox
	if err := e.w.DB.Where("event_hash = ?", hash).Take(&row).Error; err != nil {
		t.Fatalf("读取入站任务失败：%v", err)
	}
	if row.Id == 0 {
		t.Fatalf("入站事件没有落库：%+v", ev)
	}
	return &row
}

// privateEvent 构造一条来自绑定管理员（或任意用户）的私聊事件。
func (e *channelInboxTestEnv) privateEvent(userId, text string) InboundEvent {
	return InboundEvent{
		UserId:   userId,
		ChatId:   userId,
		ChatType: ChatTypeP2P,
		Text:     text,
	}
}

func (e *channelInboxTestEnv) reloadInbox(t *testing.T, id uint) *model.ChannelInbox {
	t.Helper()
	var row model.ChannelInbox
	if err := e.w.DB.Where("id = ? AND site_id = ?", id, e.w.Id).Take(&row).Error; err != nil {
		t.Fatalf("重新读取入站任务 %d 失败：%v", id, err)
	}
	return &row
}

func (e *channelInboxTestEnv) approvalsForBinding(t *testing.T) []model.ChannelApproval {
	t.Helper()
	var rows []model.ChannelApproval
	if err := e.w.DB.Where("site_id = ? AND binding_id = ?", e.w.Id, e.binding.Id).
		Order("id asc").Find(&rows).Error; err != nil {
		t.Fatalf("读取审批记录失败：%v", err)
	}
	t.Cleanup(func() {
		for _, row := range rows {
			if err := e.w.DB.Where("`public_id` = ? AND `site_id` = ?", row.PublicId, e.w.Id).
				Delete(&model.ChannelApproval{}).Error; err != nil {
				t.Logf("清理审批 %d 失败：%v", row.Id, err)
			}
		}
	})
	return rows
}

// ============================================================
// 1. 入站写库补齐平台会话、发送人身份摘要与来源绑定
// ============================================================

// TestChannelInboxInboundPersistsPlatformIdentity 证明 handleInbound 不再只写
// site/channel/event_kind/content：PlatformChatId / ChatType / IdentityHash / BindingId
// 是消费侧唯一的身份来源，缺一个就会「不回复」或「回错人」。
func TestChannelInboxInboundPersistsPlatformIdentity(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"api":{"*":"allow"}}}`,
		whitelist:   []string{"api"},
		permissions: []string{"/"},
	})
	row := env.receiveInbound(t, env.privateEvent(env.binding.PlatformUserId, "  你好，测试  "))

	if row.PlatformChatId != env.binding.PlatformUserId {
		t.Fatalf("入站任务必须记录平台会话标识，实际 %q", row.PlatformChatId)
	}
	if row.ChatType != ChatTypeP2P {
		t.Fatalf("入站任务必须记录会话类型，实际 %q", row.ChatType)
	}
	wantHash := bindingIdentityDigest(env.w.Id, env.channel.Id, env.binding.OrgId, env.binding.PlatformUserId)
	if row.IdentityHash != wantHash {
		t.Fatalf("入站任务的身份摘要必须与绑定摘要同口径，实际 %q", row.IdentityHash)
	}
	if row.BindingId != env.binding.Id {
		t.Fatalf("命中的有效绑定必须写入任务行（撤销级联才有对象），实际 %d", row.BindingId)
	}
	if row.Content != "你好，测试" {
		t.Fatalf("正文仍必须走脱敏与去空白，实际 %q", row.Content)
	}
	if row.Status != model.InboxStatusPending {
		t.Fatalf("新入站任务必须是 pending，实际 %q", row.Status)
	}
}

// ============================================================
// 2. 未绑定私聊：不执行、不回复、done 且写明原因
// ============================================================

func TestChannelInboxUnboundMessageIsIgnored(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"api":{"*":"allow"}}}`,
		whitelist:   []string{"api"},
		permissions: []string{"/"},
	})
	env.useScriptedModel(t)

	stranger := "stranger_" + channelPermSuffix()
	row := env.receiveInbound(t, env.privateEvent(stranger, "帮我把所有文章删掉"))
	if row.BindingId != 0 {
		t.Fatalf("未绑定来源的 BindingId 必须是 0，实际 %d", row.BindingId)
	}

	if !env.consumer.processNext(context.Background()) {
		t.Fatal("消费者必须认领这条待处理任务")
	}

	if calls := env.model.calls(); calls != 0 {
		t.Fatalf("未绑定用户的消息不得进入模型，实际调用 %d 次", calls)
	}
	if sends := env.adapter.sendCount(); sends != 0 {
		t.Fatalf("未绑定用户的消息不得回复（不泄露任何信息），实际发送 %d 次", sends)
	}
	after := env.reloadInbox(t, row.Id)
	if after.Status != model.InboxStatusDone {
		t.Fatalf("未绑定消息应标记 done，实际 %q", after.Status)
	}
	if after.LastError != channelInboxReasonUnbound {
		t.Fatalf("忽略原因必须是「%s」，实际 %q", channelInboxReasonUnbound, after.LastError)
	}
}

// ============================================================
// 3. 已绑定私聊：执行一轮并回复到绑定的私聊目标
// ============================================================

func TestChannelInboxBoundPrivateReplies(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"api":{"*":"allow"}}}`,
		whitelist:   []string{"api"},
		permissions: []string{"/"},
	})
	env.useScriptedModel(t)

	row := env.receiveInbound(t, env.privateEvent(env.binding.PlatformUserId, "你好"))
	if !env.consumer.processNext(context.Background()) {
		t.Fatal("消费者必须认领这条待处理任务")
	}

	sends := env.adapter.sends()
	if len(sends) != 1 {
		t.Fatalf("已绑定私聊必须回复一次，实际 %d 次", len(sends))
	}
	if sends[0].target.Kind != TargetKindPrivate {
		t.Fatalf("私聊回复目标类型必须是 %s，实际 %q", TargetKindPrivate, sends[0].target.Kind)
	}
	if sends[0].target.PlatformTargetId != env.binding.PlatformUserId {
		t.Fatalf("私聊回复必须发到绑定管理员的平台用户标识，实际 %q", sends[0].target.PlatformTargetId)
	}
	if strings.TrimSpace(sends[0].msg.Text) == "" {
		t.Fatal("回复正文不能为空")
	}
	if calls := env.model.calls(); calls == 0 {
		t.Fatal("已绑定私聊必须真正执行一轮（模型调用 0 次说明链路空转）")
	}
	after := env.reloadInbox(t, row.Id)
	if after.Status != model.InboxStatusDone || after.LastError != "" {
		t.Fatalf("成功处理应标记 done 且无原因，实际 status=%q last_error=%q", after.Status, after.LastError)
	}
}

// ============================================================
// 4. 群聊：已批准的群目标才执行并回复到 chat_id
// ============================================================

func TestChannelInboxGroupRepliesToChat(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"api":{"*":"allow"}}}`,
		whitelist:   []string{"api"},
		permissions: []string{"/"},
	})
	env.useScriptedModel(t)

	chatId := "group-chat-" + channelPermSuffix()
	env.newGroupTarget(t, chatId, 1, 1)

	row := env.receiveInbound(t, InboundEvent{
		UserId:       env.binding.PlatformUserId,
		ChatId:       chatId,
		ChatType:     ChatTypeGroup,
		MentionedBot: true,
		Text:         "帮我看一下接口清单",
	})
	if !env.consumer.processNext(context.Background()) {
		t.Fatal("消费者必须认领这条待处理任务")
	}

	sends := env.adapter.sends()
	if len(sends) != 1 {
		t.Fatalf("群聊必须回复一次，实际 %d 次", len(sends))
	}
	if sends[0].target.Kind != TargetKindGroup {
		t.Fatalf("群聊回复目标类型必须是 %s，实际 %q", TargetKindGroup, sends[0].target.Kind)
	}
	if sends[0].target.PlatformTargetId != chatId {
		t.Fatalf("群聊回复必须发到 chat_id，实际 %q", sends[0].target.PlatformTargetId)
	}
	after := env.reloadInbox(t, row.Id)
	if after.Status != model.InboxStatusDone {
		t.Fatalf("群聊成功处理应标记 done，实际 %q", after.Status)
	}
}

// TestChannelInboxGroupTargetNotApprovedIsIgnored 是群聊的负面用例：
// 群目标不存在/未批准/未开启管理时不得执行、不得回复，且必须留下去敏原因。
func TestChannelInboxGroupTargetNotApprovedIsIgnored(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"api":{"*":"allow"}}}`,
		whitelist:   []string{"api"},
		permissions: []string{"/"},
	})
	env.useScriptedModel(t)

	chatId := "group-unapproved-" + channelPermSuffix()
	row := env.receiveInbound(t, InboundEvent{
		UserId:       env.binding.PlatformUserId,
		ChatId:       chatId,
		ChatType:     ChatTypeGroup,
		MentionedBot: true,
		Text:         "帮我看一下接口清单",
	})
	if !env.consumer.processNext(context.Background()) {
		t.Fatal("消费者必须认领这条待处理任务")
	}

	if calls := env.model.calls(); calls != 0 {
		t.Fatalf("未获授权的群不得进入模型，实际调用 %d 次", calls)
	}
	if sends := env.adapter.sendCount(); sends != 0 {
		t.Fatalf("未获授权的群不得回复，实际发送 %d 次", sends)
	}
	after := env.reloadInbox(t, row.Id)
	if after.Status != model.InboxStatusDone {
		t.Fatalf("群目标拒绝应标记 done（忽略），实际 %q", after.Status)
	}
	if after.LastError != ErrChannelTargetNotApproved.Error() {
		t.Fatalf("忽略原因必须是群目标哨兵文案，实际 %q", after.LastError)
	}
}

// TestChannelInboxGroupSenderNeedsOwnBinding 锁住「群聊不继承权限」：
// 群目标已批准、机器人也已被 @，但发送人自己没有绑定 → 不得借用群主或渠道管理员的身份执行，
// 也不得回复（回复等于向陌生人确认「机器人在这里」）。
func TestChannelInboxGroupSenderNeedsOwnBinding(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"api":{"*":"allow"}}}`,
		whitelist:   []string{"api"},
		permissions: []string{"/"},
	})
	env.useScriptedModel(t)
	executed := stubEndpointCap(t, env.svc)

	chatId := "group-unbound-sender-" + channelPermSuffix()
	env.newGroupTarget(t, chatId, 1, 1)

	row := env.receiveInbound(t, InboundEvent{
		UserId:       "group_stranger_" + channelPermSuffix(),
		ChatId:       chatId,
		ChatType:     ChatTypeGroup,
		MentionedBot: true,
		Text:         "帮我看一下接口清单",
	})
	if row.BindingId != 0 {
		t.Fatalf("未绑定发送人的任务行不得挂上别人的绑定，实际 %d", row.BindingId)
	}
	if !env.consumer.processNext(context.Background()) {
		t.Fatal("消费者必须认领这条待处理任务")
	}

	if calls := env.model.calls(); calls != 0 {
		t.Fatalf("未绑定发送人在已批准群里也不得进入模型，实际调用 %d 次", calls)
	}
	if *executed != 0 {
		t.Fatalf("未绑定发送人不得借用任何身份执行工具，实际执行 %d 次", *executed)
	}
	if sends := env.adapter.sendCount(); sends != 0 {
		t.Fatalf("未绑定发送人的群消息不得回复，实际发送 %d 次", sends)
	}
	after := env.reloadInbox(t, row.Id)
	if after.Status != model.InboxStatusDone {
		t.Fatalf("未绑定发送人的群消息应标记 done（忽略），实际 %q", after.Status)
	}
	if after.LastError != channelInboxReasonUnbound {
		t.Fatalf("忽略原因必须是「%s」，实际 %q", channelInboxReasonUnbound, after.LastError)
	}
}

// ============================================================
// 5. 策略 confirm → 建审批 → 批准执行一次 → 重复批准不重复执行
// ============================================================

func TestChannelInboxConfirmCreatesApprovalAndApproveExecutesOnce(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"seo_keyword":{"create":"confirm"}}}`,
		whitelist:   []string{"seo_keyword"},
		permissions: []string{"/"},
		exposed:     []string{"seo_keyword"},
	})
	env.useScriptedModel(t)
	executed := stubEndpointCap(t, env.svc)
	env.model.script = func(round int, in []*schema.Message) *schema.Message {
		if round > 0 {
			return schema.AssistantMessage("已暂停，等待确认。", nil)
		}
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "inbox-confirm-1",
			Type:     "function",
			Function: schema.FunctionCall{Name: "seo_keyword", Arguments: `{"action":"create","title":"渠道测试"}`},
		}})
	}

	inbox := env.receiveInbound(t, env.privateEvent(env.binding.PlatformUserId, "建一个关键词"))
	if !env.consumer.processNext(context.Background()) {
		t.Fatal("消费者必须认领这条待处理任务")
	}
	if *executed != 0 {
		t.Fatalf("confirm 的写操作在本轮不得执行，实际 %d 次", *executed)
	}

	rows := env.approvalsForBinding(t)
	if len(rows) != 1 {
		t.Fatalf("confirm 必须落一条审批，实际 %d 条", len(rows))
	}
	approval := rows[0]
	if approval.Status != model.ApprovalStatusPending {
		t.Fatalf("新建审批必须是 pending，实际 %q", approval.Status)
	}
	if approval.Tool != "seo_keyword" || approval.Action != "create" {
		t.Fatalf("审批必须记录工具与动作，实际 %q/%q", approval.Tool, approval.Action)
	}
	if approval.Generation != 1 {
		t.Fatalf("审批必须记录创建时的会话代次，实际 %d", approval.Generation)
	}
	if approval.SessionId == 0 {
		t.Fatal("审批必须挂在真实会话行上（SessionId 不得为 0）")
	}

	sends := env.adapter.sends()
	if len(sends) != 1 {
		t.Fatalf("建单后必须回复确认提示，实际 %d 次", len(sends))
	}
	prompt := sends[0].msg.Text
	if !strings.Contains(prompt, approval.PublicId) || !strings.Contains(prompt, "批准") {
		t.Fatalf("确认提示必须给出公开 ID 与批准指令，实际 %q", prompt)
	}
	if strings.Contains(prompt, `"title"`) || strings.Contains(prompt, "渠道测试") {
		t.Fatalf("确认提示不得包含参数原文，实际 %q", prompt)
	}
	if after := env.reloadInbox(t, inbox.Id); after.Status != model.InboxStatusDone {
		t.Fatalf("建单成功应标记 done，实际 %q", after.Status)
	}

	// 管理员在同一条私聊里回「批准 <公开ID>」。
	env.receiveInbound(t, env.privateEvent(env.binding.PlatformUserId, "批准 "+approval.PublicId))
	if !env.consumer.processNext(context.Background()) {
		t.Fatal("审批指令必须被消费者认领")
	}
	if *executed != 1 {
		t.Fatalf("批准后必须执行且只执行一次，实际 %d 次", *executed)
	}
	if calls := env.model.calls(); calls != 1 {
		// 第一轮模型 1 次（工具调用即进入确认，不再追一轮）；批准执行不调用模型。
		t.Fatalf("批准执行不得调用模型，实际累计 %d 次", calls)
	}
	var afterApprove model.ChannelApproval
	if err := env.w.DB.Where("`public_id` = ? AND `site_id` = ?", approval.PublicId, env.w.Id).
		Take(&afterApprove).Error; err != nil {
		t.Fatalf("读取审批状态失败：%v", err)
	}
	if afterApprove.Status != model.ApprovalStatusSucceeded {
		t.Fatalf("批准执行成功后审批必须是 succeeded，实际 %q", afterApprove.Status)
	}
	if sends := env.adapter.sends(); len(sends) != 2 {
		t.Fatalf("批准后必须把执行结果回复到私聊，实际累计 %d 次", len(sends))
	}

	// 再发一次同样的批准指令：只能被拒绝，绝不能执行第二次。
	env.receiveInbound(t, env.privateEvent(env.binding.PlatformUserId, "批准 "+approval.PublicId))
	if !env.consumer.processNext(context.Background()) {
		t.Fatal("重复批准指令必须被消费者认领（然后拒绝）")
	}
	if *executed != 1 {
		t.Fatalf("重复批准不得重复执行，实际执行 %d 次", *executed)
	}
	sends = env.adapter.sends()
	if len(sends) != 3 {
		t.Fatalf("重复批准必须给出一条说明，实际累计 %d 次", len(sends))
	}
	if !strings.Contains(sends[2].msg.Text, "已处理") {
		t.Fatalf("重复批准应回复「已处理」，实际 %q", sends[2].msg.Text)
	}
}

// TestChannelInboxConcurrentApproveExecutesOnce：两个并发批准指令只能有一个赢家。
//
// 单实例内消费者是串行的，并发只可能来自「两台实例同时收到点击/指令」，
// 所以这里直接并发调用指令处理函数，验证条件更新确实互斥。
func TestChannelInboxConcurrentApproveExecutesOnce(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"seo_keyword":{"create":"confirm"}}}`,
		whitelist:   []string{"seo_keyword"},
		permissions: []string{"/"},
		exposed:     []string{"seo_keyword"},
	})
	env.useScriptedModel(t)
	executed := stubEndpointCap(t, env.svc)
	env.model.script = func(round int, in []*schema.Message) *schema.Message {
		if round > 0 {
			return schema.AssistantMessage("已暂停，等待确认。", nil)
		}
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "inbox-concurrent-1",
			Type:     "function",
			Function: schema.FunctionCall{Name: "seo_keyword", Arguments: `{"action":"create","title":"并发测试"}`},
		}})
	}

	env.receiveInbound(t, env.privateEvent(env.binding.PlatformUserId, "建一个关键词"))
	if !env.consumer.processNext(context.Background()) {
		t.Fatal("消费者必须认领这条待处理任务")
	}
	rows := env.approvalsForBinding(t)
	if len(rows) != 1 {
		t.Fatalf("confirm 必须落一条审批，实际 %d 条", len(rows))
	}
	publicId := rows[0].PublicId

	var wg sync.WaitGroup
	replies := make([]string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			replies[idx] = env.consumer.handleApprovalCommand(
				context.Background(), env.binding, channelApprovalDecisionApprove, publicId)
		}(i)
	}
	wg.Wait()

	if *executed != 1 {
		t.Fatalf("并发批准只能执行一次，实际 %d 次", *executed)
	}
	var row model.ChannelApproval
	if err := env.w.DB.Where("`public_id` = ? AND `site_id` = ?", publicId, env.w.Id).Take(&row).Error; err != nil {
		t.Fatalf("读取审批状态失败：%v", err)
	}
	if row.Status != model.ApprovalStatusSucceeded {
		t.Fatalf("赢家执行成功后审批必须是 succeeded，实际 %q", row.Status)
	}
	nonEmpty := 0
	for _, reply := range replies {
		if strings.TrimSpace(reply) != "" {
			nonEmpty++
		}
	}
	if nonEmpty != 2 {
		t.Fatalf("两个并发指令都必须有回执（一个成功、一个已处理），实际 %d 条", nonEmpty)
	}
}

// ============================================================
// 6. 绑定在消费前被撤销：不执行、不回复、done 有原因
// ============================================================

// TestChannelInboxRevokedBindingMidFlightIsIgnored 锁住「已认领后绑定被撤销」：
// 任务已脱离 pending，撤销级联不会再动它，消费侧必须自己重新查绑定。
func TestChannelInboxRevokedBindingMidFlightIsIgnored(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"api":{"*":"allow"}}}`,
		whitelist:   []string{"api"},
		permissions: []string{"/"},
	})
	env.useScriptedModel(t)

	env.receiveInbound(t, env.privateEvent(env.binding.PlatformUserId, "你好"))
	row, ok, err := env.consumer.claimNext(context.Background())
	if err != nil {
		t.Fatalf("认领任务失败：%v", err)
	}
	if !ok || row == nil {
		t.Fatal("必须认领到刚写入的任务")
	}
	if row.Status != model.InboxStatusProcessing {
		t.Fatalf("认领后任务必须是 processing，实际 %q", row.Status)
	}

	if err := env.w.RevokeBinding(env.binding.Id, env.admin.Id); err != nil {
		t.Fatalf("撤销绑定失败：%v", err)
	}

	env.consumer.processInbox(context.Background(), row)
	if calls := env.model.calls(); calls != 0 {
		t.Fatalf("绑定已撤销的消息不得进入模型，实际调用 %d 次", calls)
	}
	if sends := env.adapter.sendCount(); sends != 0 {
		t.Fatalf("绑定已撤销的消息不得回复，实际发送 %d 次", sends)
	}
	after := env.reloadInbox(t, row.Id)
	if after.Status != model.InboxStatusDone {
		t.Fatalf("撤销后的任务应标记 done，实际 %q", after.Status)
	}
	if after.LastError != channelInboxReasonUnbound {
		t.Fatalf("忽略原因必须说明未绑定，实际 %q", after.LastError)
	}
}

// TestChannelInboxRevokeCascadesPendingRows 是上一条的反向对照：
// 仍在 pending 的任务由撤销级联直接置 cancelled —— 正因如此，
// 「消费前撤销」只能靠已认领（processing）的任务来表达。
func TestChannelInboxRevokeCascadesPendingRows(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"api":{"*":"allow"}}}`,
		whitelist:   []string{"api"},
		permissions: []string{"/"},
	})
	row := env.receiveInbound(t, env.privateEvent(env.binding.PlatformUserId, "你好"))
	if err := env.w.RevokeBinding(env.binding.Id, env.admin.Id); err != nil {
		t.Fatalf("撤销绑定失败：%v", err)
	}
	after := env.reloadInbox(t, row.Id)
	if after.Status != model.InboxStatusCancelled {
		t.Fatalf("撤销绑定必须级联取消 pending 任务，实际 %q", after.Status)
	}
	if env.consumer.processNext(context.Background()) {
		t.Fatal("已取消的任务不得再被认领")
	}
}

// ============================================================
// 7. 会话代次递增
// ============================================================

// generationTestEnv 只建渠道 + 管理员 + 绑定 + 会话行，不建执行器夹具。
type generationTestEnv struct {
	w       *Website
	channel *model.MessageChannel
	admin   *model.Admin
	binding *model.ChannelBinding
	session *model.ChannelSession
}

func newGenerationTestEnv(t *testing.T) *generationTestEnv {
	t.Helper()
	w := channelChatTestSite(t)
	ch := newPermTestChannel(t, w, `{"intents":{}}`, 1)
	admin := newPermTestAdmin(t, w, []string{"/"}, 1)

	suffix := channelPermSuffix()
	userId := "gen_user_" + suffix
	binding := &model.ChannelBinding{
		SiteId:         w.Id,
		ChannelId:      ch.Id,
		OrgId:          ch.OrgId,
		PlatformUserId: userId,
		AdminId:        admin.Id,
		IdentityHash:   bindingIdentityDigest(w.Id, ch.Id, ch.OrgId, userId),
		Revision:       1,
	}
	if err := w.DB.Create(binding).Error; err != nil {
		t.Fatalf("创建绑定失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ? AND site_id = ?", binding.Id, w.Id).Delete(&model.ChannelBinding{}).Error; err != nil {
			t.Logf("清理绑定 %d 失败：%v", binding.Id, err)
		}
		if err := w.DB.Where("site_id = ? AND channel_id = ?", w.Id, ch.Id).
			Delete(&model.ChannelSession{}).Error; err != nil {
			t.Logf("清理渠道 %d 的会话失败：%v", ch.Id, err)
		}
	})

	userHash := PlatformIdentityHash(w.Id, ch.Id, ch.Platform, ch.OrgId, userId)
	session := &model.ChannelSession{
		SiteId:         w.Id,
		ChannelId:      ch.Id,
		PlatformChatId: userId,
		UserHash:       userHash,
		ChatSessionId:  "gen_chat_" + suffix,
		Generation:     1,
		SessionHash:    "gen_session_" + suffix,
	}
	if err := w.DB.Create(session).Error; err != nil {
		t.Fatalf("创建会话行失败：%v", err)
	}
	return &generationTestEnv{w: w, channel: ch, admin: admin, binding: binding, session: session}
}

func (e *generationTestEnv) generation(t *testing.T) int64 {
	t.Helper()
	var row model.ChannelSession
	if err := e.w.DB.Where("id = ? AND site_id = ?", e.session.Id, e.w.Id).Take(&row).Error; err != nil {
		t.Fatalf("读取会话行失败：%v", err)
	}
	return row.Generation
}

// TestChannelGenerationBumpedOnRevokeAndRevive：撤销与换绑复活都必须让旧代次失效。
func TestChannelGenerationBumpedOnRevokeAndRevive(t *testing.T) {
	env := newGenerationTestEnv(t)
	if got := env.generation(t); got != 1 {
		t.Fatalf("基线代次应为 1，实际 %d", got)
	}

	// 反向对照：撤销另一条绑定不得影响本会话代次。
	otherAdmin := newPermTestAdmin(t, env.w, []string{"/"}, 1)
	other := newPermTestBinding(t, env.w, env.channel.Id, otherAdmin.Id)
	if err := env.w.RevokeBinding(other.Id, otherAdmin.Id); err != nil {
		t.Fatalf("撤销无关绑定失败：%v", err)
	}
	if got := env.generation(t); got != 1 {
		t.Fatalf("撤销无关绑定不得改动本会话代次，实际 %d", got)
	}

	if err := env.w.RevokeBinding(env.binding.Id, env.admin.Id); err != nil {
		t.Fatalf("撤销绑定失败：%v", err)
	}
	if got := env.generation(t); got != 2 {
		t.Fatalf("撤销绑定必须让本会话代次 +1，实际 %d", got)
	}

	plain, _, err := env.w.CreateBindingToken(env.channel.Id, env.admin.Id)
	if err != nil {
		t.Fatalf("生成绑定令牌失败：%v", err)
	}
	if _, err := env.w.ConsumeBindingToken(plain, ChannelPlatformIdentity{
		ChannelId:      env.channel.Id,
		OrganizationId: env.channel.OrgId,
		AppId:          env.channel.AppId,
		UserId:         env.binding.PlatformUserId,
	}); err != nil {
		t.Fatalf("换绑复活失败：%v", err)
	}
	if got := env.generation(t); got != 3 {
		t.Fatalf("换绑复活必须让本会话代次再 +1，实际 %d", got)
	}
}

// TestChannelGenerationBumpedOnAgentChange：切换接待智能体必须让该渠道全部会话换代。
func TestChannelGenerationBumpedOnAgentChange(t *testing.T) {
	env := newGenerationTestEnv(t)
	w := env.w
	ch := env.channel

	save := func(agentId uint, name string) {
		t.Helper()
		var current model.MessageChannel
		if err := w.DB.Where("id = ? AND site_id = ?", ch.Id, w.Id).Take(&current).Error; err != nil {
			t.Fatalf("读取渠道失败：%v", err)
		}
		_, err := w.updateAdminChannel(&request.Channel{
			Id:          current.Id,
			Platform:    current.Platform,
			Name:        name,
			AppId:       current.AppId,
			OrgId:       current.OrgId,
			Enabled:     current.Enabled,
			ChatEnabled: current.ChatEnabled,
			AgentId:     agentId,
			PolicyJson:  current.PolicyJson,
			Revision:    current.Revision,
		}, current.Platform, name, current.AppId, current.OrgId, current.PolicyJson)
		if err != nil {
			t.Fatalf("保存渠道失败：%v", err)
		}
	}

	// 反向对照：智能体没变（只改名字）时代次必须不动。
	save(ch.AgentId, "改名不换代-"+channelPermSuffix())
	if got := env.generation(t); got != 1 {
		t.Fatalf("智能体未变不得改代次，实际 %d", got)
	}

	save(ch.AgentId+1, "切换智能体-"+channelPermSuffix())
	if got := env.generation(t); got != 2 {
		t.Fatalf("切换接待智能体必须让渠道会话代次 +1，实际 %d", got)
	}
}

// TestChannelApprovalGenerationRecordedAndEnforced：审批落库必须带创建时代次，
// 代次变化后 DecideApproval 必须拒绝（ErrChannelPolicyStale）。
func TestChannelApprovalGenerationRecordedAndEnforced(t *testing.T) {
	env := newApprovalTestEnv(t)
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	argsJSON := `{"action":"list","page":1}`
	row := createTestApproval(t, env.w, p, "seo_keyword", argsJSON, 10*time.Minute)
	if row.Generation != session.Generation {
		t.Fatalf("审批必须记录创建时的会话代次 %d，实际 %d", session.Generation, row.Generation)
	}

	// 会话代次变化（换绑/切换智能体）后，旧审批必须失效。
	if err := env.w.DB.Model(&model.ChannelSession{}).
		Where("id = ? AND site_id = ?", session.Id, env.w.Id).
		Update("generation", session.Generation+1).Error; err != nil {
		t.Fatalf("提升会话代次失败：%v", err)
	}
	if err := env.w.DecideApproval(row.PublicId, env.binding.Id, "approve"); !errors.Is(err, ErrChannelPolicyStale) {
		t.Fatalf("代次不符时批准必须返回 ErrChannelPolicyStale，实际：%v", err)
	}

	// 反向对照：代次恢复一致后同一张单必须能批准，证明上面的拒绝来自代次而非环境。
	if err := env.w.DB.Model(&model.ChannelSession{}).
		Where("id = ? AND site_id = ?", session.Id, env.w.Id).
		Update("generation", session.Generation).Error; err != nil {
		t.Fatalf("恢复会话代次失败：%v", err)
	}
	if err := env.w.DecideApproval(row.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("代次一致时批准必须成功，实际：%v", err)
	}
}

// ============================================================
// 8. 写门禁：生产后端让策略 allow 的写操作真正执行
// ============================================================

// TestChannelPolicyApprovalBackendInjected 锁住接线：站点服务必须自带渠道写门后端，
// 否则「策略 allow 的写操作」在生产里全部被拒（Critical 2 的同源问题）。
func TestChannelPolicyApprovalBackendInjected(t *testing.T) {
	w := channelChatTestSite(t)
	svc := w.NewAiChatService()
	if svc == nil {
		t.Fatal("NewAiChatService 返回 nil")
	}
	backend := svc.channelApprovalBackend()
	if backend == nil {
		t.Fatal("站点服务必须注入渠道写门审批后端（nil 会让所有写操作恒拒绝）")
	}
	decision, reason := backend(context.Background(), ChannelApprovalRequest{})
	if decision != "allow" {
		t.Fatalf("生产后端必须返回 allow，实际 %q", decision)
	}
	if strings.TrimSpace(reason) == "" {
		t.Fatal("生产后端必须给出可审计的放行理由")
	}
}

// TestChannelAllowedWriteExecutesWithProductionBackend：策略 allow 的写意图
// 在注入生产后端后必须真正执行；不注入（nil）时仍必须被拒（保留既有语义）。
func TestChannelAllowedWriteExecutesWithProductionBackend(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"seo_keyword":{"create":"allow"}}}`,
		whitelist:   []string{"seo_keyword"},
		permissions: []string{"/"},
		exposed:     []string{"seo_keyword"},
	})
	env.useScriptedModel(t)
	executed := stubEndpointCap(t, env.svc)
	// scriptedChannelModel.rounds 是**跨轮次累计**的，不能按 round 判断「本轮第一次调用」：
	// 这里按「最后一条输入是不是工具结果」区分「本轮已执行过工具」与「本轮刚开始」。
	// 否则第二次入站的首个模型调用会直接回文本，写操作永远得不到执行（假失败）。
	env.model.script = func(_ int, in []*schema.Message) *schema.Message {
		if lastMessageIsToolResult(in) {
			return schema.AssistantMessage("已处理。", nil)
		}
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "inbox-write-1",
			Type:     "function",
			Function: schema.FunctionCall{Name: "seo_keyword", Arguments: `{"action":"create","title":"写门禁测试"}`},
		}})
	}

	// 1) 没有后端（既有夹具语义）→ 写操作被拒，cap 一次都不执行。
	env.receiveInbound(t, env.privateEvent(env.binding.PlatformUserId, "建一个关键词"))
	if !env.consumer.processNext(context.Background()) {
		t.Fatal("消费者必须认领这条待处理任务")
	}
	if *executed != 0 {
		t.Fatalf("没有审批后端时写操作必须被拒，实际执行 %d 次", *executed)
	}

	// 2) 注入生产后端 → 同一个写操作必须真正执行一次。
	env.svc.SetChannelApprovalBackend(channelPolicyApprovalBackend)
	t.Cleanup(func() { env.svc.SetChannelApprovalBackend(nil) })
	env.receiveInbound(t, env.privateEvent(env.binding.PlatformUserId, "再建一个关键词"))
	if !env.consumer.processNext(context.Background()) {
		t.Fatal("消费者必须认领第二条待处理任务")
	}
	if *executed != 1 {
		t.Fatalf("注入生产后端后写操作必须执行一次，实际 %d 次", *executed)
	}
}

// ============================================================
// 9. worker 生命周期：拿到租约后必须启动消费者，Run 返回后不得留下孤儿 goroutine
// ============================================================

func TestChannelWorkerStartsInboxConsumer(t *testing.T) {
	w := channelManagerTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	suffix := channelPermSuffix()
	row := &model.ChannelInbox{
		SiteId:         w.Id,
		ChannelId:      ch.Id,
		EventKind:      InboundKindText,
		Content:        "ping",
		Status:         model.InboxStatusPending,
		EventHash:      channelDigest("t12a-wiring", suffix),
		PlatformChatId: "wiring-chat-" + suffix,
		IdentityHash:   channelDigest("t12a-identity", suffix),
		ChatType:       ChatTypeP2P,
	}
	if err := w.DB.Create(row).Error; err != nil {
		t.Fatalf("写入待处理入站任务失败：%v", err)
	}

	registry := newFakeChannelFactory(nil)
	opts := testChannelManagerOptions(registry)
	opts.inboxPollEvery = 10 * time.Millisecond
	startTestChannelManager(t, w, opts)
	registry.waitForRun(t, ch.AppId, 1)

	deadline := time.Now().Add(3 * time.Second)
	var after model.ChannelInbox
	for time.Now().Before(deadline) {
		if err := w.DB.Where("id = ? AND site_id = ?", row.Id, w.Id).Take(&after).Error; err != nil {
			t.Fatalf("读取入站任务失败：%v", err)
		}
		if after.Status == model.InboxStatusDone {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if after.Status != model.InboxStatusDone {
		t.Fatalf("worker 必须启动入站消费者处理 pending 任务，实际状态仍是 %q", after.Status)
	}
	if after.LastError != channelInboxReasonChatDisabled {
		t.Fatalf("仅通知渠道（chat_enabled=0）应写明原因，实际 %q", after.LastError)
	}

	// 停管理器后消费者必须退出：先停再写一条 pending 任务，状态不得再变化
	//（若消费者没退出，10ms 轮询会在下面的等待窗口内把它处理掉）。
	w.StopChannelManager()
	late := &model.ChannelInbox{
		SiteId:         w.Id,
		ChannelId:      ch.Id,
		EventKind:      InboundKindText,
		Content:        "late",
		Status:         model.InboxStatusPending,
		EventHash:      channelDigest("t12a-wiring-late", suffix),
		PlatformChatId: "wiring-late-" + suffix,
		IdentityHash:   channelDigest("t12a-identity-late", suffix),
		ChatType:       ChatTypeP2P,
	}
	if err := w.DB.Create(late).Error; err != nil {
		t.Fatalf("写入第二条任务失败：%v", err)
	}
	time.Sleep(50 * time.Millisecond)
	var lateAfter model.ChannelInbox
	if err := w.DB.Where("id = ? AND site_id = ?", late.Id, w.Id).Take(&lateAfter).Error; err != nil {
		t.Fatalf("读取第二条任务失败：%v", err)
	}
	if lateAfter.Status != model.InboxStatusPending {
		t.Fatalf("停止管理器后消费者必须退出，任务状态被改成了 %q", lateAfter.Status)
	}
}
