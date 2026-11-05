package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"kandaoni.com/anqicms/model"
)

// ============================================================
// T7 持久化审批：真实库（站点 1）。
//
// 夹具复用 channelChatTestSite / newChannelTurnTestEnv（随机后缀、按自己创建的 ID
// 精确清理，绝不按 site_id 批量删）。本文件只补三样：
//   - 一条渠道会话行：CreateApproval 必须把 principal.SessionId（内部 chat 会话字符串）
//     解析成 channel_sessions.id 落库，测试必须给它一条真实可查的行；
//   - 构造执行主体的小助手；
//   - 审批行的读取 / 精确清理助手。
// ============================================================

// newApprovalSession 建一条渠道会话行。SessionHash 唯一索引由随机后缀保证不撞车。
func (e *channelTurnTestEnv) newApprovalSession(t *testing.T) *model.ChannelSession {
	t.Helper()
	suffix := channelPermSuffix()
	row := &model.ChannelSession{
		SiteId:         e.w.Id,
		ChannelId:      e.channel.Id,
		PlatformChatId: "appr-chat-" + suffix,
		UserHash:       "appr-user-" + suffix,
		ChatSessionId:  "appr-chat-session-" + suffix,
		Generation:     1,
		SessionHash:    "appr-session-" + suffix,
	}
	if err := e.w.DB.Create(row).Error; err != nil {
		t.Fatalf("创建渠道会话失败：%v", err)
	}
	t.Cleanup(func() {
		if err := e.w.DB.Where("id = ? AND site_id = ?", row.Id, e.w.Id).Delete(&model.ChannelSession{}).Error; err != nil {
			t.Logf("清理渠道会话 %d 失败：%v", row.Id, err)
		}
	})
	return row
}

// approvalTestPrincipal 构造渠道执行主体：AgentId 必须是渠道配置的接待智能体
// （channelForPrincipal 强制两者相等），SessionId 取会话行的内部 chat 会话字符串。
func approvalTestPrincipal(e *channelTurnTestEnv, session *model.ChannelSession) ExecutionPrincipal {
	p := e.request("approval-chat", "创建审批", 1, false).Principal
	p.AgentId = e.channel.AgentId
	p.SessionId = session.ChatSessionId
	return p
}

// newApprovalTestEnv 是审批用例的公共夹具：seo_keyword 全动作 confirm + 白名单 + 全菜单权限。
// 管理员启用状态在用例结束时还原（停用用例会把它改成 0）。
func newApprovalTestEnv(t *testing.T) *channelTurnTestEnv {
	t.Helper()
	w := channelChatTestSite(t)
	withTestAiSetting(t, w, []string{"seo_keyword"})
	env := newChannelTurnTestEnv(t, w, channelTurnTestConfig{
		policy:      `{"intents":{"seo_keyword":{"*":"confirm"}}}`,
		whitelist:   []string{"seo_keyword"},
		permissions: []string{"/"},
	})
	adminId := env.admin.Id
	t.Cleanup(func() {
		if err := w.DB.Model(&model.Admin{}).Where("id = ?", adminId).Update("status", 1).Error; err != nil {
			t.Logf("还原管理员 %d 状态失败：%v", adminId, err)
		}
	})
	return env
}

// createTestApproval 调 CreateApproval 并按 public_id 精确清理。
func createTestApproval(t *testing.T, w *Website, p ExecutionPrincipal, toolName, argsJSON string, ttl time.Duration) *model.ChannelApproval {
	t.Helper()
	row, err := w.CreateApproval(p, toolName, argsJSON, ttl)
	if err != nil {
		t.Fatalf("创建审批失败：%v", err)
	}
	if row == nil || strings.TrimSpace(row.PublicId) == "" {
		t.Fatalf("创建审批必须返回带公开 ID 的行，实际：%+v", row)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("`public_id` = ? AND `site_id` = ?", row.PublicId, w.Id).Delete(&model.ChannelApproval{}).Error; err != nil {
			t.Logf("清理审批 %s 失败：%v", row.PublicId, err)
		}
	})
	return row
}

func loadTestApproval(t *testing.T, w *Website, publicId string) *model.ChannelApproval {
	t.Helper()
	var row model.ChannelApproval
	if err := w.DB.Where("`public_id` = ? AND `site_id` = ?", publicId, w.Id).Take(&row).Error; err != nil {
		t.Fatalf("读取审批 %s 失败：%v", publicId, err)
	}
	return &row
}

// TestApprovalRejectsOtherBinding：审批单只能由它自己的绑定消费。
// 关键在反向对照：同一张单由原绑定点击必须成功，否则「拒绝」可能只是环境坏了。
func TestApprovalRejectsOtherBinding(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	const argsJSON = `{"action":"list","page":1}`
	row := createTestApproval(t, w, p, "seo_keyword", argsJSON, 10*time.Minute)
	if row.SiteId != w.Id || row.BindingId != env.binding.Id {
		t.Fatalf("审批行必须落在本站点本绑定，实际：%+v", row)
	}
	if row.SessionId != session.Id {
		t.Fatalf("SessionId 必须是 channel_sessions.id（%d），实际 %d", session.Id, row.SessionId)
	}
	if row.Status != model.ApprovalStatusPending {
		t.Fatalf("新建审批必须是 pending，实际 %q", row.Status)
	}
	if row.Tool != "seo_keyword" || row.Action != "list" {
		t.Fatalf("工具身份必须是 tool=seo_keyword action=list，实际 tool=%q action=%q", row.Tool, row.Action)
	}

	// 同渠道、同管理员，但另一个绑定行。
	other := newPermTestBinding(t, w, env.channel.Id, env.admin.Id)
	if err := w.DecideApproval(row.PublicId, other.Id, "approve"); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("其他绑定点击必须返回 ErrChannelApprovalInvalid，实际：%v", err)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusPending {
		t.Fatalf("被他人点击后状态必须仍是 pending，实际 %q", got)
	}

	// 反向对照：原绑定点击同一张单必须成功。
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("原绑定点击必须成功（否则上面的拒绝无法归因），实际：%v", err)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusApproved {
		t.Fatalf("批准后状态必须是 approved，实际 %q", got)
	}
}

// TestApprovalExpiresAfter10Minutes：默认 TTL 10 分钟；过期单必须拒绝并惰性标记 expired。
func TestApprovalExpiresAfter10Minutes(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	before := time.Now().Unix()
	row := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)
	if delta := row.ExpiresAt - before; delta < 550 || delta > 650 {
		t.Fatalf("10 分钟 TTL 的 ExpiresAt 必须落在 550–650 秒之间，实际 %d", delta)
	}

	// ttl <= 0 同样按 10 分钟处理（零值不能变成「立刻过期」）。
	fallback := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 0)
	if delta := fallback.ExpiresAt - before; delta < 550 || delta > 650 {
		t.Fatalf("ttl<=0 必须回落成 10 分钟，实际 %d 秒", delta)
	}

	// 手动把有效期推前 1 秒：模拟 10 分钟后点击。
	if err := w.DB.Model(&model.ChannelApproval{}).
		Where("id = ? AND site_id = ?", row.Id, w.Id).
		Update("expires_at", time.Now().Unix()-1).Error; err != nil {
		t.Fatalf("构造过期审批失败：%v", err)
	}
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("过期单点击必须返回 ErrChannelApprovalInvalid，实际：%v", err)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusExpired {
		t.Fatalf("过期单应被惰性标成 expired，实际 %q", got)
	}
	// 过期单也不能被执行消费。
	if err := w.MarkApprovalExecuting(row.PublicId); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("过期单不得进入执行，实际：%v", err)
	}
}

// TestApprovalDoubleClickExecutesOnce：并发/重复点击只有一个赢家，消费也只能成功一次。
func TestApprovalDoubleClickExecutesOnce(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	row := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)

	// 并发点击：8 个 goroutine 用栅栏同时起跑（R67）。只有 2 个 goroutine 的版本
	// 经常在第一个请求已经提交之后才发出第二个，check-then-act 的回归会长期假绿。
	const clickers = 8
	var (
		start     sync.WaitGroup
		done      sync.WaitGroup
		mu        sync.Mutex
		successes int
	)
	start.Add(clickers)
	done.Add(clickers)
	for i := 0; i < clickers; i++ {
		go func() {
			defer done.Done()
			start.Done()
			start.Wait()
			err := w.DecideApproval(row.PublicId, env.binding.Id, "approve")
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
				return
			}
			if !errors.Is(err, ErrChannelApprovalInvalid) {
				t.Errorf("并发点击失败方必须返回 ErrChannelApprovalInvalid，实际：%v", err)
			}
		}()
	}
	done.Wait()
	if successes != 1 {
		t.Fatalf("并发点击必须只有一个赢家，实际 %d", successes)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusApproved {
		t.Fatalf("并发点击后状态必须是 approved，实际 %q", got)
	}

	// 顺序重放（同一管理员再点一次）同样拒绝。
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("重复点击必须返回 ErrChannelApprovalInvalid，实际：%v", err)
	}

	// 消费只能成功一次。
	if err := w.MarkApprovalExecuting(row.PublicId); err != nil {
		t.Fatalf("第一次消费必须成功，实际：%v", err)
	}
	if err := w.MarkApprovalExecuting(row.PublicId); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("第二次消费必须失败（不重复执行），实际：%v", err)
	}
	if err := w.CompleteApproval(row.PublicId, "完成"); err != nil {
		t.Fatalf("收尾必须成功，实际：%v", err)
	}
	// 收尾之后再消费一次仍然失败：否则调用方重试会造成真实副作用。
	if err := w.MarkApprovalExecuting(row.PublicId); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("已完成审批不得再次执行，实际：%v", err)
	}
	if err := w.CompleteApproval(row.PublicId, "再次完成"); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("已完成审批不得重复收尾，实际：%v", err)
	}

	// 第二次执行也必须失败：走真实执行器路径（runApprovedCall 先消费再执行）。
	// 这条断言是 R60 接线的唯一覆盖——去掉 runApprovedCall 里的 MarkApprovalExecuting，
	// 同一条审批重放会真的执行第二次。
	execRow := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)
	if err := w.DecideApproval(execRow.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("批准失败：%v", err)
	}
	executed := stubEndpointCap(t, env.svc)
	chatId := "approval-replay-" + channelPermSuffix()
	env.cleanupSession(t, env.binding, chatId)
	req := env.request(chatId, "确认执行", 1, false)
	req.Approval = &ChannelApprovalGrant{
		ApprovalId: execRow.PublicId,
		ToolName:   "seo_keyword",
		ArgsJSON:   execRow.CanonicalArgs,
	}
	if _, err := env.svc.runChannelTurn(context.Background(), req, scriptedChannelFactory(&scriptedChannelModel{})); err != nil {
		t.Fatalf("首次批准执行必须成功，实际：%v", err)
	}
	if *executed != 1 {
		t.Fatalf("首次批准执行必须执行 1 次，实际 %d 次", *executed)
	}
	if _, err := env.svc.runChannelTurn(context.Background(), req, scriptedChannelFactory(&scriptedChannelModel{})); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("同一条审批第二次执行必须返回 ErrChannelApprovalInvalid，实际：%v", err)
	}
	if *executed != 1 {
		t.Fatalf("同一条审批不得执行第二次，实际累计 %d 次", *executed)
	}
}

// TestApprovalRejectedWhenArgsChange：参数被改过的脏行不得批准、也不得执行。
func TestApprovalRejectedWhenArgsChange(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	row := createTestApproval(t, w, p, "seo_keyword", `{"action":"list","page":1}`, 10*time.Minute)

	// 脏行：参数正文被改成另一份，摘要保持原样（等价于绕过审批改参数）。
	if err := w.DB.Model(&model.ChannelApproval{}).
		Where("id = ? AND site_id = ?", row.Id, w.Id).
		Update("canonical_args", `{"action":"list","page":2}`).Error; err != nil {
		t.Fatalf("构造参数不一致的审批失败：%v", err)
	}
	if err := w.MarkApprovalExecuting(row.PublicId); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("参数被改过的记录不得进入执行，实际：%v", err)
	}
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("参数被改过的记录不得批准，实际：%v", err)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusPending {
		t.Fatalf("参数不一致时状态必须保持 pending，实际 %q", got)
	}

	// 反向对照：参数恢复一致后同一张单必须可以批准（证明上面的拒绝来自参数比对）。
	if err := w.DB.Model(&model.ChannelApproval{}).
		Where("id = ? AND site_id = ?", row.Id, w.Id).
		Update("canonical_args", `{"action":"list","page":1}`).Error; err != nil {
		t.Fatalf("还原审批参数失败：%v", err)
	}
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("参数一致时必须可以批准，实际：%v", err)
	}
	if err := w.MarkApprovalExecuting(row.PublicId); err != nil {
		t.Fatalf("参数一致且已批准时必须可以执行，实际：%v", err)
	}
}

// TestApprovalRejectedAfterAdminDisabled：管理员失效后点击必须拿到 ErrChannelAdminInvalid。
func TestApprovalRejectedAfterAdminDisabled(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	row := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)

	if err := w.DB.Model(&model.Admin{}).Where("id = ?", env.admin.Id).Update("status", 0).Error; err != nil {
		t.Fatalf("停用管理员失败：%v", err)
	}
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); !errors.Is(err, ErrChannelAdminInvalid) {
		t.Fatalf("停用管理员点击必须返回 ErrChannelAdminInvalid，实际：%v", err)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusPending {
		t.Fatalf("管理员失效时状态必须保持 pending，实际 %q", got)
	}
	// 停用后连创建都不行：审批必须在入口就把失效主体挡掉。
	if _, err := w.CreateApproval(p, "seo_keyword", `{"action":"list"}`, 10*time.Minute); !errors.Is(err, ErrChannelAdminInvalid) {
		t.Fatalf("停用管理员不得创建审批，实际：%v", err)
	}

	// 反向对照：恢复启用后同一张单必须可以批准。
	if err := w.DB.Model(&model.Admin{}).Where("id = ?", env.admin.Id).Update("status", 1).Error; err != nil {
		t.Fatalf("恢复管理员失败：%v", err)
	}
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("管理员恢复后必须可以批准，实际：%v", err)
	}
}

// TestRestartMarksExecutingAsUnknown：重启恢复只归档、不重放。
func TestRestartMarksExecutingAsUnknown(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	pending := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)
	approved := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)
	if err := w.DecideApproval(approved.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("批准失败：%v", err)
	}
	executing := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)
	if err := w.DecideApproval(executing.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("批准失败：%v", err)
	}
	if err := w.MarkApprovalExecuting(executing.PublicId); err != nil {
		t.Fatalf("消费失败：%v", err)
	}

	if err := w.recoverChannelApprovals(); err != nil {
		t.Fatalf("启动恢复必须成功，实际：%v", err)
	}
	if got := loadTestApproval(t, w, pending.PublicId).Status; got != model.ApprovalStatusCancelled {
		t.Fatalf("重启后 pending 必须置 cancelled，实际 %q", got)
	}
	if got := loadTestApproval(t, w, approved.PublicId).Status; got != model.ApprovalStatusCancelled {
		t.Fatalf("重启后 approved 必须置 cancelled，实际 %q", got)
	}
	if got := loadTestApproval(t, w, executing.PublicId).Status; got != model.ApprovalStatusUnknown {
		t.Fatalf("重启后 executing 必须置 unknown，实际 %q", got)
	}

	// 不自动重放：恢复后两条都不能再进入执行。
	if err := w.MarkApprovalExecuting(executing.PublicId); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("重启恢复后不得重放 executing，实际：%v", err)
	}
	if err := w.MarkApprovalExecuting(approved.PublicId); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("重启恢复后不得重放 approved，实际：%v", err)
	}

	// 幂等：重复调用结果不变。
	if err := w.recoverChannelApprovals(); err != nil {
		t.Fatalf("重复恢复必须成功，实际：%v", err)
	}
	if got := loadTestApproval(t, w, executing.PublicId).Status; got != model.ApprovalStatusUnknown {
		t.Fatalf("重复恢复不得改动 unknown 行，实际 %q", got)
	}
	if got := loadTestApproval(t, w, pending.PublicId).Status; got != model.ApprovalStatusCancelled {
		t.Fatalf("重复恢复不得改动 cancelled 行，实际 %q", got)
	}
}

// TestApprovalCreatedRowMatchesPermissionProof 是 R58 在 T7 侧的护栏：
// CreateApproval 落库的 tool/action/args_hash 必须与 T5 的
// CheckIntentPermissionApproved 同源，否则「管理员确认真实发生、执行时永远对不上」。
func TestApprovalCreatedRowMatchesPermissionProof(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	// 键序与执行侧不同：摘要必须按规范化后比对，而不是按原文。
	row := createTestApproval(t, w, p, "seo_keyword", `{"page":1,"action":"list"}`, 10*time.Minute)
	if row.CanonicalArgs != `{"action":"list","page":1}` {
		t.Fatalf("CanonicalArgs 必须是规范化后的参数，实际 %q", row.CanonicalArgs)
	}
	if row.ArgsHash == "" || row.ArgsHash != ChannelArgsHash(row.CanonicalArgs) {
		t.Fatalf("ArgsHash 必须是规范化参数的 ChannelArgsHash，实际 %q", row.ArgsHash)
	}
	if row.ArgsHash != ChannelArgsHash(`{"action":"list","page":1}`) {
		t.Fatalf("ArgsHash 必须与同源规范化算法一致，实际 %q", row.ArgsHash)
	}

	// ArgsHash 绝不序列化到任何响应（与 ChannelLease.AppIdentityHash 同口径）。
	buf, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("序列化审批行失败：%v", err)
	}
	if strings.Contains(string(buf), "args_hash") || strings.Contains(string(buf), row.ArgsHash) {
		t.Fatalf("审批行的 ArgsHash 不得出现在序列化结果里，实际：%s", string(buf))
	}

	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("批准失败：%v", err)
	}
	if err := w.CheckIntentPermissionApproved(context.Background(), p, row.PublicId, row.CanonicalArgs, "seo_keyword", row.Action); err != nil {
		t.Fatalf("T7 落库的 tool/action/args_hash 必须能被 T5 的自证接受，实际：%v", err)
	}

	// 零参意图：空串 / 全空白 / 字面 null 一律按 {} 落库，action 必须是空串（不得写 "*"）。
	for _, zero := range []string{"", "   ", "null", "  null  "} {
		empty := createTestApproval(t, w, p, "seo_keyword", zero, 10*time.Minute)
		if empty.CanonicalArgs != "{}" {
			t.Fatalf("零参审批的 CanonicalArgs 必须是 {}，输入 %q 实际 %q", zero, empty.CanonicalArgs)
		}
		if empty.Action != "" {
			t.Fatalf("零参审批的 Action 必须为空串（不得回落 \"*\"），输入 %q 实际 %q", zero, empty.Action)
		}
	}

	// 入口还必须挡住「无法反查会话」与「工具名为空」：这两种行落库后永远点不动。
	orphan := p
	orphan.SessionId = "missing-chat-session-" + channelPermSuffix()
	if _, err := w.CreateApproval(orphan, "seo_keyword", `{"action":"list"}`, 10*time.Minute); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("会话查不到时必须返回 ErrChannelApprovalInvalid，实际：%v", err)
	}
	if _, err := w.CreateApproval(p, "  ", `{"action":"list"}`, 10*time.Minute); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("工具名为空时必须返回 ErrChannelApprovalInvalid，实际：%v", err)
	}
}

// TestApprovalRejectedWhenPolicyRevisionChanges：策略改版后旧审批必须重新确认（R61 第 1 步）。
func TestApprovalRejectedWhenPolicyRevisionChanges(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	row := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)

	// 渠道策略改版：revision 递增。
	if err := w.DB.Model(&model.MessageChannel{}).
		Where("id = ? AND site_id = ?", env.channel.Id, w.Id).
		Update("revision", env.channel.Revision+1).Error; err != nil {
		t.Fatalf("递增渠道修订号失败：%v", err)
	}
	// 创建侧：主体声明的修订号已过期。
	if _, err := w.CreateApproval(p, "seo_keyword", `{"action":"list"}`, 10*time.Minute); !errors.Is(err, ErrChannelPolicyStale) {
		t.Fatalf("策略已改版时创建审批必须返回 ErrChannelPolicyStale，实际：%v", err)
	}
	// 点击侧：旧单不得沿用旧策略批准。
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); !errors.Is(err, ErrChannelPolicyStale) {
		t.Fatalf("策略已改版时点击必须返回 ErrChannelPolicyStale，实际：%v", err)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusPending {
		t.Fatalf("策略不一致时状态必须保持 pending，实际 %q", got)
	}

	// 反向对照：修订号还原后同一张单必须可以批准。
	if err := w.DB.Model(&model.MessageChannel{}).
		Where("id = ? AND site_id = ?", env.channel.Id, w.Id).
		Update("revision", env.channel.Revision).Error; err != nil {
		t.Fatalf("还原渠道修订号失败：%v", err)
	}
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("修订号一致时必须可以批准，实际：%v", err)
	}
}

// ============================================================
// 修复轮 1（R64–R67）：自证先于消费、并发消费、参数归一、决策与收尾
// ============================================================

// TestApprovalProofFailureDoesNotConsume 是 C1（R64）的反向用例：
// 自证必须在消费**之前**。错配参数 / 错工具 / 别人的单号重放一张 approved 单，
// 必须报错、不执行，而且审批行必须仍是 approved。
//
// 为什么状态断言是这条用例的核心：MarkApprovalExecuting 的 WHERE 只有
// site_id + public_id + status + expires_at，同站点任意绑定只要知道 public_id
// 就能把一张合法 approved 单推进 executing。若先消费后自证，这次失败会把它
// 落成 failed（结果摘要改写、状态不可回退），真正的主人再也执行不了这张单——
// 不是提权，但审批台账被烧毁。当前（先消费）实现下本用例必红。
func TestApprovalProofFailureDoesNotConsume(t *testing.T) {
	w := channelChatTestSite(t)
	withTestAiSetting(t, w, []string{"api", "seo_keyword"})
	env := newChannelTurnTestEnv(t, w, channelTurnTestConfig{
		policy:      `{"intents":{"api":{"schema":"confirm"},"seo_keyword":{"*":"confirm"}}}`,
		whitelist:   []string{"api", "seo_keyword"},
		permissions: []string{"/"},
	})
	ctx := context.Background()
	chatId := "approval-proof-" + channelPermSuffix()
	env.cleanupSession(t, env.binding, chatId)

	const argsJSON = `{"action":"schema","path":"/system/api/archive/list"}`
	row := env.newApprovalRow(t, "api", argsJSON, model.ApprovalStatusApproved, time.Now().Add(10*time.Minute).Unix())

	// 所有 cap 都换成计数器：任何一次委托都算「执行了」。
	executed := 0
	for name := range env.svc.capHandlers {
		env.svc.capHandlers[name] = func(ctx context.Context, args string) (string, error) {
			executed++
			return "已执行", nil
		}
	}

	deniedReplay := func(grant *ChannelApprovalGrant) {
		t.Helper()
		req := env.request(chatId, "确认执行", 1, false)
		req.Approval = grant
		if _, err := env.svc.runChannelTurn(ctx, req, scriptedChannelFactory(&scriptedChannelModel{})); !errors.Is(err, ErrChannelApprovalInvalid) {
			t.Fatalf("凭据与审批不符必须返回 ErrChannelApprovalInvalid，实际：%v", err)
		}
		if executed != 0 {
			t.Fatalf("自证失败时不得执行任何 cap，实际执行 %d 次", executed)
		}
		if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusApproved {
			t.Fatalf("自证失败不得消费审批：状态必须仍是 approved，实际 %q", got)
		}
	}

	// 1) 错配参数：参数被换掉（摘要对不上）。
	deniedReplay(&ChannelApprovalGrant{
		ApprovalId: row.PublicId,
		ToolName:   "api",
		ArgsJSON:   `{"action":"schema","path":"/system/api/archive/detail"}`,
	})
	// 2) 错工具：工具名被换掉，参数逐字相同——摘要一致，只能靠 tool 身份拦住。
	deniedReplay(&ChannelApprovalGrant{ApprovalId: row.PublicId, ToolName: "seo_keyword", ArgsJSON: argsJSON})
	// 3) 别人的单：单号根本不存在（同站点其它绑定知道 public_id 的攻击面）。
	deniedReplay(&ChannelApprovalGrant{
		ApprovalId: "appr-not-exists-" + channelPermSuffix(),
		ToolName:   "api",
		ArgsJSON:   argsJSON,
	})

	// 反向对照：正确的凭据必须仍然能执行——证明上面的拒绝不是环境坏了。
	req := env.request(chatId, "确认执行", 1, false)
	req.Approval = &ChannelApprovalGrant{ApprovalId: row.PublicId, ToolName: "api", ArgsJSON: argsJSON}
	if _, err := env.svc.runChannelTurn(ctx, req, scriptedChannelFactory(&scriptedChannelModel{})); err != nil {
		t.Fatalf("正确凭据必须执行成功，实际：%v", err)
	}
	if executed != 1 {
		t.Fatalf("正确凭据必须执行 1 次，实际 %d 次", executed)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusSucceeded {
		t.Fatalf("执行后状态必须是 succeeded，实际 %q", got)
	}
}

// TestApprovalMarkExecutingConcurrentSingleWinner 锁住消费侧的 check-then-act（R67）。
//
// 顺序调用两次 MarkApprovalExecuting 抓不到「先查后写」：第一次已经把状态改走了，
// 第二次无论怎么实现都会失败。这里让 8 个 goroutine 在栅栏后同时消费同一张 approved 单：
// 只有条件更新（status='approved' AND expires_at > now）能让赢家恰好一个。
// 变异验证：去掉条件更新里的 status 谓词，8 个都会成功，本用例立刻变红。
func TestApprovalMarkExecutingConcurrentSingleWinner(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	row := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("批准失败：%v", err)
	}

	const consumers = 8
	var (
		start   sync.WaitGroup
		done    sync.WaitGroup
		mu      sync.Mutex
		winners int
		losers  int
	)
	start.Add(consumers)
	done.Add(consumers)
	for i := 0; i < consumers; i++ {
		go func() {
			defer done.Done()
			start.Done()
			start.Wait()
			err := w.MarkApprovalExecuting(row.PublicId)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				winners++
			case errors.Is(err, ErrChannelApprovalInvalid):
				losers++
			default:
				t.Errorf("并发消费失败方必须返回 ErrChannelApprovalInvalid，实际：%v", err)
			}
		}()
	}
	done.Wait()
	if winners != 1 || losers != consumers-1 {
		t.Fatalf("并发消费必须恰好一个赢家：赢家 %d / 失败 %d（期望 1 / %d）", winners, losers, consumers-1)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusExecuting {
		t.Fatalf("并发消费后状态必须是 executing，实际 %q", got)
	}
}

// TestApprovalRejectsInvalidDecision：决策只接受 approve/reject（R61/M5）。
func TestApprovalRejectsInvalidDecision(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	row := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)

	// once_allow / full_control 是「把一次点击放大成长期绕过」的取值，必须拒绝；
	// 空串、无关值同理。它们都不能改动审批状态。
	for _, decision := range []string{"once_allow", "full_control", "", "   ", "cancel", "approved"} {
		if err := w.DecideApproval(row.PublicId, env.binding.Id, decision); !errors.Is(err, ErrChannelApprovalInvalid) {
			t.Fatalf("非法决策 %q 必须返回 ErrChannelApprovalInvalid，实际：%v", decision, err)
		}
		if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusPending {
			t.Fatalf("非法决策 %q 不得改动审批状态，实际 %q", decision, got)
		}
	}

	// 反向对照：合法决策（去空白 + 小写）必须生效，否则上面的拒绝无法归因。
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "  APPROVE  "); err != nil {
		t.Fatalf("合法决策必须生效，实际：%v", err)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusApproved {
		t.Fatalf("批准后状态必须是 approved，实际 %q", got)
	}
}

// TestFailApprovalRecordsFailureSummary：executing → failed，摘要单行且最多 200 个 rune（M5）。
func TestFailApprovalRecordsFailureSummary(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	session := env.newApprovalSession(t)
	p := approvalTestPrincipal(env, session)

	row := createTestApproval(t, w, p, "seo_keyword", `{"action":"list"}`, 10*time.Minute)
	if err := w.DecideApproval(row.PublicId, env.binding.Id, "approve"); err != nil {
		t.Fatalf("批准失败：%v", err)
	}
	// 状态机只有 executing → failed：还没消费的单不得直接落 failed。
	if err := w.FailApproval(row.PublicId, "还没开始就失败"); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("未消费的审批不得置 failed，实际：%v", err)
	}
	if got := loadTestApproval(t, w, row.PublicId).Status; got != model.ApprovalStatusApproved {
		t.Fatalf("失败收尾被拒后状态必须仍是 approved，实际 %q", got)
	}
	if err := w.MarkApprovalExecuting(row.PublicId); err != nil {
		t.Fatalf("消费失败：%v", err)
	}

	// 多行 + 超长中文：摘要必须是单行、最多 200 个 rune（按字节截会劈开汉字）。
	line := strings.Repeat("执行失败：端点返回错误。", 40)
	long := line + "\n" + line + "\n\t" + line
	if err := w.FailApproval(row.PublicId, long); err != nil {
		t.Fatalf("置 failed 失败：%v", err)
	}
	stored := loadTestApproval(t, w, row.PublicId)
	if stored.Status != model.ApprovalStatusFailed {
		t.Fatalf("FailApproval 后状态必须是 failed，实际 %q", stored.Status)
	}
	if stored.ResultSummary == "" || strings.ContainsAny(stored.ResultSummary, "\r\n\t") {
		t.Fatalf("结果摘要必须是非空单行，实际 %q", stored.ResultSummary)
	}
	if runes := []rune(stored.ResultSummary); len(runes) != channelApprovalResultSummaryLimit {
		t.Fatalf("超长摘要必须截到 %d 个 rune，实际 %d", channelApprovalResultSummaryLimit, len(runes))
	} else if strings.ContainsRune(string(runes), '\uFFFD') {
		t.Fatalf("按 rune 截断不得出现半个汉字，实际 %q", stored.ResultSummary)
	}
	// 收尾只对 executing 生效：重复收尾必须拒绝，否则失败原因会被后来的结果覆盖。
	if err := w.FailApproval(row.PublicId, "再次失败"); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("已收尾的审批不得重复置 failed，实际：%v", err)
	}
	if err := w.CompleteApproval(row.PublicId, "又成功了"); !errors.Is(err, ErrChannelApprovalInvalid) {
		t.Fatalf("失败收尾后不得再写成 succeeded，实际：%v", err)
	}
}

// TestApprovalArgsNormalizationIsSingleSource 是 I2（R65）的护栏：
// T6 的 Pending.ArgsJSON 与 T7 落库的 CanonicalArgs 必须逐字相等（零参场景也要），
// 且 CreateApproval 必须拒绝非对象参数。
//
// 两处各写一份归一时，空串在 T6 是 ""、在 T7 变成 {}：零参意图「批得下、执行不了」，
// 而且每试一次都会先消费审批（C1 未修时直接烧单）。[]、"x"、1 是同一种病的另一面。
func TestApprovalArgsNormalizationIsSingleSource(t *testing.T) {
	env := newApprovalTestEnv(t)
	w := env.w
	ctx := context.Background()

	cases := []struct {
		name      string
		arguments string
		want      string
		wantAct   string
	}{
		{"零参（空 arguments）", "", "{}", ""},
		{"零参（字面 null）", "null", "{}", ""},
		{"普通参数（键序需归一）", `{"page":1,"action":"list"}`, `{"action":"list","page":1}`, "list"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chatId := "approval-args-" + channelPermSuffix()
			env.cleanupSession(t, env.binding, chatId)
			arguments := tc.arguments
			asking := &scriptedChannelModel{script: func(round int, in []*schema.Message) *schema.Message {
				return schema.AssistantMessage("", []schema.ToolCall{{
					ID:       "approval-args-call",
					Type:     "function",
					Function: schema.FunctionCall{Name: "seo_keyword", Arguments: arguments},
				}})
			}}
			first, err := env.svc.runChannelTurn(ctx, env.request(chatId, "列一下关键词", 1, false), scriptedChannelFactory(asking))
			if err != nil {
				t.Fatalf("confirm 应返回待确认而非失败，实际：%v", err)
			}
			if first.Pending == nil {
				t.Fatalf("必须返回待确认字段，实际：%+v", first)
			}
			if first.Pending.ArgsJSON != tc.want {
				t.Fatalf("Pending.ArgsJSON 必须是 %q，实际 %q", tc.want, first.Pending.ArgsJSON)
			}
			if first.Pending.Action != tc.wantAct {
				t.Fatalf("Pending.Action 必须是 %q，实际 %q", tc.wantAct, first.Pending.Action)
			}

			// T7 照抄 Pending：落库的参数必须与 Pending 逐字相等。
			p := env.request(chatId, "创建审批", 1, false).Principal
			p.AgentId = env.channel.AgentId
			p.SessionId = first.ChatSessionId
			row := createTestApproval(t, w, p, first.Pending.ToolName, first.Pending.ArgsJSON, 10*time.Minute)
			if row.CanonicalArgs != first.Pending.ArgsJSON {
				t.Fatalf("审批行 CanonicalArgs 必须与 Pending.ArgsJSON 逐字相等，Pending=%q 行=%q",
					first.Pending.ArgsJSON, row.CanonicalArgs)
			}
			if row.Action != first.Pending.Action {
				t.Fatalf("审批行 Action 必须与 Pending.Action 逐字相等，Pending=%q 行=%q", first.Pending.Action, row.Action)
			}
			if row.ArgsHash != ChannelArgsHash(row.CanonicalArgs) {
				t.Fatalf("ArgsHash 必须与落库参数同源，实际 %q", row.ArgsHash)
			}

			// 非对象参数：工具参数只能是对象，必须在创建时就拒绝。
			for _, bad := range []string{"[]", `"x"`, "1", "true", "{} trailing"} {
				if _, err := w.CreateApproval(p, "seo_keyword", bad, 10*time.Minute); !errors.Is(err, ErrChannelApprovalInvalid) {
					t.Fatalf("非对象参数 %q 必须返回 ErrChannelApprovalInvalid，实际：%v", bad, err)
				}
			}
		})
	}
}
