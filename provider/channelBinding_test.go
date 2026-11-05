package provider

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
)

// setupChannelBindingTest 连接本机开发库（站点 1）并确保绑定相关的表存在。
//
// 仓库约定（AGENTS.md §2）：provider 包整包跑会被沙箱 SIGTERM，必须按 -run 分批。
// 这里只做幂等 AutoMigrate，不 DROP 任何表 —— 库里是站点 1 的真实数据。
func setupChannelBindingTest(t *testing.T) *Website {
	t.Helper()
	w := initTestSite(t)
	if w == nil || w.DB == nil {
		t.Fatal("站点 1 未初始化，无法连接开发库")
	}
	err := w.DB.AutoMigrate(
		&model.MessageChannel{},
		&model.ChannelBindingToken{},
		&model.ChannelBinding{},
		&model.ChannelTarget{},
		&model.ChannelApproval{},
		&model.ChannelInbox{},
	)
	if err != nil {
		t.Fatalf("迁移渠道绑定相关表失败：%v", err)
	}
	return w
}

// activeTestAdmin 取一条现有且启用的管理员；取不到直接 Fatal（不伪造管理员）。
func activeTestAdmin(t *testing.T, w *Website) *model.Admin {
	t.Helper()
	admins, total := w.GetAdminList(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("status = ?", 1).Order("id asc")
	}, 1, 1)
	if total == 0 || len(admins) == 0 {
		t.Fatal("开发库没有启用状态的管理员，无法测试绑定")
	}
	return admins[0]
}

// newTestChannel 自建一条渠道，并在测试结束时按自己的 ID 精确清理。
// AppId 带随机后缀：uk_app_identity_hash 是全局唯一索引，固定值会撞上残留行。
func newTestChannel(t *testing.T, w *Website) *model.MessageChannel {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ch := &model.MessageChannel{
		SiteId:   w.Id,
		Platform: model.ChannelPlatformFeishu,
		Name:     "绑定测试渠道-" + suffix,
		Enabled:  0,
		AppId:    "test_app_" + suffix,
		OrgId:    "test_org",
	}
	if err := w.DB.Create(ch).Error; err != nil {
		t.Fatalf("创建测试渠道失败：%v", err)
	}
	t.Cleanup(func() { cleanupTestChannel(t, w, ch.Id) })
	return ch
}

// cleanupTestChannel 以本测试自己的渠道 ID 为轴删除，绝不按 site_id 批量删，
// 否则会连带删掉站点 1 的真实渠道与绑定。
func cleanupTestChannel(t *testing.T, w *Website, channelId uint) {
	t.Helper()
	var bindingIds []uint
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ?", channelId).Pluck("id", &bindingIds).Error; err != nil {
		t.Logf("清理测试渠道 %d 时读取绑定失败：%v", channelId, err)
	}
	if len(bindingIds) > 0 {
		if err := w.DB.Where("binding_id IN ?", bindingIds).Delete(&model.ChannelApproval{}).Error; err != nil {
			t.Logf("清理测试渠道 %d 的审批失败：%v", channelId, err)
		}
		if err := w.DB.Where("binding_id IN ?", bindingIds).Delete(&model.ChannelInbox{}).Error; err != nil {
			t.Logf("清理测试渠道 %d 的入站任务失败：%v", channelId, err)
		}
	}
	for _, spec := range []struct {
		name  string
		model interface{}
	}{
		{"目标", &model.ChannelTarget{}},
		{"绑定", &model.ChannelBinding{}},
		{"令牌", &model.ChannelBindingToken{}},
	} {
		if err := w.DB.Where("channel_id = ?", channelId).Delete(spec.model).Error; err != nil {
			t.Logf("清理测试渠道 %d 的%s失败：%v", channelId, spec.name, err)
		}
	}
	if err := w.DB.Where("id = ? AND site_id = ?", channelId, w.Id).Delete(&model.MessageChannel{}).Error; err != nil {
		t.Logf("清理测试渠道 %d 失败：%v", channelId, err)
	}
}

func testChannelIdentity(ch *model.MessageChannel, userId string) ChannelPlatformIdentity {
	return ChannelPlatformIdentity{
		ChannelId:      ch.Id,
		OrganizationId: ch.OrgId,
		AppId:          ch.AppId,
		UserId:         userId,
	}
}

// TestBindingTokenExpiresAfter3600Seconds 口令有效期必须精确落在创建后 3600 秒，
// 且过期令牌不得再被消费。
func TestBindingTokenExpiresAfter3600Seconds(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)

	plain, expiresAt, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}
	if strings.TrimSpace(plain) == "" {
		t.Fatal("创建的令牌明文为空")
	}
	// now 在创建之后取：create 内部取整秒，差值只可能是 3600 或 3599（跨秒）。
	now := time.Now().Unix()
	if d := expiresAt - now; d < 3599 || d > 3600 {
		t.Fatalf("有效期应为 3600 秒，实际 %d 秒", d)
	}

	var stored model.ChannelBindingToken
	if err := w.DB.Where("channel_id = ? AND admin_id = ?", ch.Id, admin.Id).Order("id desc").Take(&stored).Error; err != nil {
		t.Fatalf("库里没有写入绑定令牌：%v", err)
	}
	if stored.ExpiresAt != expiresAt {
		t.Fatalf("库内过期时间 %d 与返回值 %d 不一致", stored.ExpiresAt, expiresAt)
	}
	if stored.ConsumedAt != 0 {
		t.Fatalf("新令牌的 consumed_at 应为 0，实际 %d", stored.ConsumedAt)
	}
	if stored.TokenHash == plain || strings.Contains(stored.TokenHash, plain) {
		t.Fatalf("库里存了明文令牌：hash=%q", stored.TokenHash)
	}
	if stored.TokenHash != hashBindingToken(plain) {
		t.Fatalf("库内摘要不是明文 token 的 SHA-256：got %q", stored.TokenHash)
	}

	ident := testChannelIdentity(ch, "test_user_expired")
	if err := w.DB.Model(&model.ChannelBindingToken{}).Where("id = ?", stored.Id).Update("expires_at", now-1).Error; err != nil {
		t.Fatalf("改写过期时间失败：%v", err)
	}
	if _, err = w.ConsumeBindingToken(plain, ident); err == nil {
		t.Fatal("过期令牌仍可消费")
	}

	var active int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ? AND revoked_at = 0", ch.Id).Count(&active).Error; err != nil {
		t.Fatalf("统计绑定失败：%v", err)
	}
	if active != 0 {
		t.Fatalf("过期令牌不应产生绑定，实际 %d 条", active)
	}
	if err := w.DB.Where("id = ?", stored.Id).Take(&stored).Error; err != nil {
		t.Fatalf("重读令牌失败：%v", err)
	}
	if stored.ConsumedAt != 0 {
		t.Fatalf("过期令牌被标记为已消费：consumed_at=%d", stored.ConsumedAt)
	}
}

// TestBindingTokenConsumeIsSingleUse 同一明文只能成功一次，且成功后落库的绑定、
// 私聊目标与摘要字段完整。
func TestBindingTokenConsumeIsSingleUse(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)

	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}
	ident := testChannelIdentity(ch, "test_user_single")
	binding, err := w.ConsumeBindingToken(plain, ident)
	if err != nil {
		t.Fatalf("首次消费失败：%v", err)
	}
	if binding == nil || binding.Id == 0 {
		t.Fatal("消费成功但没有返回绑定记录")
	}
	if binding.SiteId != w.Id || binding.ChannelId != ch.Id || binding.AdminId != admin.Id {
		t.Fatalf("绑定归属字段不对：site=%d channel=%d admin=%d", binding.SiteId, binding.ChannelId, binding.AdminId)
	}
	if binding.PlatformUserId != ident.UserId || binding.OrgId != ident.OrganizationId {
		t.Fatalf("绑定平台身份字段不对：user=%q org=%q", binding.PlatformUserId, binding.OrgId)
	}
	if binding.RevokedAt != 0 {
		t.Fatalf("新绑定的 revoked_at 应为 0，实际 %d", binding.RevokedAt)
	}
	if binding.IdentityHash != bindingIdentityDigest(w.Id, ch.Id, ident.OrganizationId, ident.UserId) {
		t.Fatalf("绑定身份摘要与约定口径不一致：got %q", binding.IdentityHash)
	}

	var target model.ChannelTarget
	if err := w.DB.Where("binding_id = ?", binding.Id).Take(&target).Error; err != nil {
		t.Fatalf("绑定成功但没有创建私聊目标：%v", err)
	}
	if target.Kind != "private" || target.PlatformTargetId != ident.UserId || target.Approved != 1 {
		t.Fatalf("私聊目标字段不对：kind=%q target=%q approved=%d", target.Kind, target.PlatformTargetId, target.Approved)
	}
	if target.TargetHash != bindingTargetDigest(w.Id, ch.Id, "private", ident.UserId) {
		t.Fatalf("目标摘要与约定口径不一致：got %q", target.TargetHash)
	}

	if _, err = w.ConsumeBindingToken(plain, ident); err == nil {
		t.Fatal("同一明文第二次消费仍然成功")
	}

	var active int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("identity_hash = ? AND revoked_at = 0", binding.IdentityHash).Count(&active).Error; err != nil {
		t.Fatalf("统计有效绑定失败：%v", err)
	}
	if active != 1 {
		t.Fatalf("重复消费后有效绑定应仍为 1 条，实际 %d 条", active)
	}
}

// TestBindingTokenConcurrentConsume 10 个 goroutine 抢同一明文，只能有 1 个成功。
// 依赖的是「条件更新 + RowsAffected == 1」的原子消费，不是内存锁。
func TestBindingTokenConcurrentConsume(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)

	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}
	ident := testChannelIdentity(ch, "test_user_concurrent")

	var successes int64
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, consumeErr := w.ConsumeBindingToken(plain, ident); consumeErr == nil {
				atomic.AddInt64(&successes, 1)
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Fatalf("并发消费成功次数应恰好为 1，实际 %d", successes)
	}
	var active int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ? AND revoked_at = 0", ch.Id).Count(&active).Error; err != nil {
		t.Fatalf("统计有效绑定失败：%v", err)
	}
	if active != 1 {
		t.Fatalf("并发消费后有效绑定应仍为 1 条，实际 %d 条", active)
	}
}

// TestBindingTokenRejectsCrossChannel 跨渠道消费必须失败，且事务回滚不能让令牌被吃掉。
func TestBindingTokenRejectsCrossChannel(t *testing.T) {
	w := setupChannelBindingTest(t)
	chA := newTestChannel(t, w)
	chB := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)

	plain, _, err := w.CreateBindingToken(chA.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}
	// 用一个属于渠道 B 的身份去消费渠道 A 的令牌。
	ident := testChannelIdentity(chB, "test_user_cross")
	if _, err = w.ConsumeBindingToken(plain, ident); err == nil {
		t.Fatal("跨渠道消费仍然成功")
	}

	var stored model.ChannelBindingToken
	if err := w.DB.Where("token_hash = ?", hashBindingToken(plain)).Take(&stored).Error; err != nil {
		t.Fatalf("读取令牌失败：%v", err)
	}
	if stored.ConsumedAt != 0 {
		t.Fatalf("跨渠道失败后令牌被消费掉了（事务未回滚）：consumed_at=%d", stored.ConsumedAt)
	}
	var bindings int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id IN ?", []uint{chA.Id, chB.Id}).Count(&bindings).Error; err != nil {
		t.Fatalf("统计绑定失败：%v", err)
	}
	if bindings != 0 {
		t.Fatalf("跨渠道消费不应产生绑定，实际 %d 条", bindings)
	}
}

// TestBindingTokenRejectsExistingBinding 已有绑定时必须拒绝，且绝不覆盖：
// 同一平台身份重复绑定、同一管理员同渠道换身份绑定都要报错。
func TestBindingTokenRejectsExistingBinding(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)

	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}
	first, err := w.ConsumeBindingToken(plain, testChannelIdentity(ch, "test_user_a"))
	if err != nil {
		t.Fatalf("首次绑定失败：%v", err)
	}

	// 1) 同一平台身份再次绑定：必须拒绝。
	plainSame, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建第二个绑定令牌失败：%v", err)
	}
	if _, err = w.ConsumeBindingToken(plainSame, testChannelIdentity(ch, "test_user_a")); err == nil {
		t.Fatal("同一平台身份重复绑定仍然成功")
	} else if !strings.Contains(err.Error(), "已绑定") {
		t.Fatalf("重复绑定错误提示应说明身份已绑定，实际 %q", err.Error())
	}

	// 2) 同一管理员同渠道换一个身份绑定：必须拒绝并提示先解绑。
	plainOther, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建第三个绑定令牌失败：%v", err)
	}
	if _, err = w.ConsumeBindingToken(plainOther, testChannelIdentity(ch, "test_user_b")); err == nil {
		t.Fatal("同一管理员同渠道的第二个身份绑定仍然成功")
	} else if !strings.Contains(err.Error(), "解绑") {
		t.Fatalf("管理员重复绑定错误提示应提示先解绑，实际 %q", err.Error())
	}

	// 原绑定不能被覆盖，也不能多出有效绑定。
	var bindings []model.ChannelBinding
	if err := w.DB.Where("channel_id = ?", ch.Id).Find(&bindings).Error; err != nil {
		t.Fatalf("读取绑定失败：%v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("拒绝后应只有原绑定 1 条，实际 %d 条", len(bindings))
	}
	if bindings[0].Id != first.Id || bindings[0].AdminId != first.AdminId || bindings[0].PlatformUserId != "test_user_a" {
		t.Fatalf("原绑定被覆盖：%+v", bindings[0])
	}
}

// TestRevokeBindingCancelsPendingWork 撤销绑定必须同时失效未执行审批与排队任务，
// 且重复撤销幂等；不存在的绑定、非法的操作者必须报错。
func TestRevokeBindingCancelsPendingWork(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}
	binding, err := w.ConsumeBindingToken(plain, testChannelIdentity(ch, "test_user_revoke"))
	if err != nil {
		t.Fatalf("绑定失败：%v", err)
	}

	approval := &model.ChannelApproval{
		SiteId:         w.Id,
		BindingId:      binding.Id,
		Tool:           "archive",
		Action:         "create",
		Status:         model.ApprovalStatusPending,
		ExpiresAt:      time.Now().Unix() + 600,
		PublicId:       "test-public-" + suffix,
		PolicyRevision: 1,
	}
	if err := w.DB.Create(approval).Error; err != nil {
		t.Fatalf("插入待确认审批失败：%v", err)
	}
	inbox := &model.ChannelInbox{
		SiteId:    w.Id,
		ChannelId: ch.Id,
		BindingId: binding.Id,
		EventKind: "message",
		Status:    "pending",
		EventHash: hashBindingToken("inbox-" + suffix),
	}
	if err := w.DB.Create(inbox).Error; err != nil {
		t.Fatalf("插入排队任务失败：%v", err)
	}

	if err = w.RevokeBinding(binding.Id, admin.Id); err != nil {
		t.Fatalf("撤销绑定失败：%v", err)
	}

	var reloaded model.ChannelBinding
	if err := w.DB.Where("id = ?", binding.Id).Take(&reloaded).Error; err != nil {
		t.Fatalf("重读绑定失败：%v", err)
	}
	if reloaded.RevokedAt == 0 {
		t.Fatal("撤销后 revoked_at 仍为 0")
	}
	var reloadedApproval model.ChannelApproval
	if err := w.DB.Where("id = ?", approval.Id).Take(&reloadedApproval).Error; err != nil {
		t.Fatalf("重读审批失败：%v", err)
	}
	if reloadedApproval.Status != model.ApprovalStatusCancelled {
		t.Fatalf("未执行审批状态应为 cancelled，实际 %q", reloadedApproval.Status)
	}
	var reloadedInbox model.ChannelInbox
	if err := w.DB.Where("id = ?", inbox.Id).Take(&reloadedInbox).Error; err != nil {
		t.Fatalf("重读入站任务失败：%v", err)
	}
	if reloadedInbox.Status != "cancelled" {
		t.Fatalf("排队任务状态应为 cancelled，实际 %q", reloadedInbox.Status)
	}
	// 私聊目标来自有效绑定（规格 §4），撤销后不得再被当成已批准的投递目标。
	var target model.ChannelTarget
	if err := w.DB.Where("binding_id = ?", binding.Id).Take(&target).Error; err != nil {
		t.Fatalf("读取私聊目标失败：%v", err)
	}
	if target.Approved != 0 {
		t.Fatalf("撤销后私聊目标仍为已批准：approved=%d", target.Approved)
	}

	// 幂等：再撤一次不报错。
	if err = w.RevokeBinding(binding.Id, admin.Id); err != nil {
		t.Fatalf("重复撤销应幂等返回 nil，实际 %v", err)
	}
	// 不存在的绑定要报错。
	if err = w.RevokeBinding(0, admin.Id); err == nil {
		t.Fatal("撤销不存在的绑定没有报错")
	}
	// 操作者必须存在且启用。
	if err = w.RevokeBinding(binding.Id, 0); err == nil {
		t.Fatal("操作者不存在时撤销没有报错")
	}
	// 不属于本站点的绑定要报错。
	foreign := &model.ChannelBinding{
		SiteId:       w.Id + 1,
		ChannelId:    ch.Id,
		AdminId:      admin.Id,
		IdentityHash: hashBindingToken("foreign-" + suffix),
	}
	if err = w.DB.Create(foreign).Error; err != nil {
		t.Fatalf("插入跨站点绑定失败：%v", err)
	}
	if err = w.RevokeBinding(foreign.Id, admin.Id); err == nil {
		t.Fatal("撤销其它站点的绑定没有报错")
	}
}
