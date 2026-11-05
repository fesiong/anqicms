package provider

import (
	"crypto/sha256"
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

// wantIdentityDigest 用字面协议独立算身份摘要：站点 \x1f 渠道 \x1f 组织 \x1f 用户，SHA-256 小写 hex。
//
// 刻意不调用被测代码的 bindingIdentityDigest：用它算期望值会让「改分隔符 / 改参数顺序 /
// 漏 TrimSpace」在生产与断言两侧同步漂移，断言永远成立（同义反复）。这里的值只能来自规格。
// 比较前由调用方自行 TrimSpace —— 摘要口径要求归一化，期望值也必须用归一化后的输入。
func wantIdentityDigest(siteId, channelId uint, organizationId, userId string) string {
	joined := fmt.Sprintf("%d\x1f%d\x1f%s\x1f%s", siteId, channelId, organizationId, userId)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(joined)))
}

// wantTargetDigest 同上，第三段是目标类型字符串（private/group）。
func wantTargetDigest(siteId, channelId uint, kind, platformTargetId string) string {
	joined := fmt.Sprintf("%d\x1f%d\x1f%s\x1f%s", siteId, channelId, kind, platformTargetId)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(joined)))
}

// newTestAdmin 在开发库里临时插入一名启用管理员，测试结束按 id 删除。
// 「换绑给另一名管理员」需要第二个管理员，而开发库只有一名启用账号；
// 这里新增而不改动现有管理员，避免把共享账号的状态当成测试耗材。
func newTestAdmin(t *testing.T, w *Website, prefix string) *model.Admin {
	t.Helper()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	admin := &model.Admin{
		UserName: prefix + "-" + suffix,
		Status:   1,
	}
	if err := w.DB.Create(admin).Error; err != nil {
		t.Fatalf("创建临时管理员失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", admin.Id).Delete(&model.Admin{}).Error; err != nil {
			t.Logf("清理临时管理员 %d 失败：%v", admin.Id, err)
		}
	})
	return admin
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
	// 期望值走字面协议（want*Digest），不调用被测摘要函数：改分隔符/参数顺序必须在这里变红。
	if binding.IdentityHash != wantIdentityDigest(w.Id, ch.Id, ident.OrganizationId, ident.UserId) {
		t.Fatalf("绑定身份摘要与约定口径不一致：got %q", binding.IdentityHash)
	}

	var target model.ChannelTarget
	if err := w.DB.Where("binding_id = ?", binding.Id).Take(&target).Error; err != nil {
		t.Fatalf("绑定成功但没有创建私聊目标：%v", err)
	}
	if target.Kind != "private" || target.PlatformTargetId != ident.UserId || target.Approved != 1 {
		t.Fatalf("私聊目标字段不对：kind=%q target=%q approved=%d", target.Kind, target.PlatformTargetId, target.Approved)
	}
	if target.TargetHash != wantTargetDigest(w.Id, ch.Id, "private", ident.UserId) {
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

	// 启动栅栏：先让 10 个 goroutine 全部就位，再同时放行。
	// 否则调度器可能把前几个 goroutine 整串执行完，测试退化成「顺序争抢」，
	// 掩盖不了「先读后写」这类竞态。
	start := make(chan struct{})
	var successes int64
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, consumeErr := w.ConsumeBindingToken(plain, ident); consumeErr == nil {
				atomic.AddInt64(&successes, 1)
			}
		}()
	}
	close(start)
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

	// 跨渠道失败必须整个回滚：换回正确渠道的身份，同一令牌仍然可用。
	binding, err := w.ConsumeBindingToken(plain, testChannelIdentity(chA, "test_user_cross"))
	if err != nil {
		t.Fatalf("跨渠道失败后同一令牌在正确渠道上不可用：%v", err)
	}
	if binding.SiteId != w.Id || binding.ChannelId != chA.Id {
		t.Fatalf("重试成功但绑定归属不对：site=%d channel=%d", binding.SiteId, binding.ChannelId)
	}
	var inB int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ?", chB.Id).Count(&inB).Error; err != nil {
		t.Fatalf("统计渠道 B 绑定失败：%v", err)
	}
	if inB != 0 {
		t.Fatalf("渠道 B 不应产生绑定，实际 %d 条", inB)
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
		Status:    model.InboxStatusPending,
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
	if reloadedInbox.Status != model.InboxStatusCancelled {
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

// TestBindingTokenRebindAfterRevoke 锁住「撤销 → 同一身份再绑」这条唯一可行的换绑路径。
//
// uk_identity_hash / uk_target_hash 覆盖已撤销的历史行，直接插新行必然 1062，
// 所以实现只能复活原行。复活分支一旦写坏（漏重置 revoked_at、漏改 admin_id、
// 漏把私聊目标重新批准），换绑在真实环境里就是彻底不可用，而其余测试全都不会变红。
func TestBindingTokenRebindAfterRevoke(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	adminA := activeTestAdmin(t, w)
	adminB := newTestAdmin(t, w, "rebind")
	ident := testChannelIdentity(ch, "test_user_rebind")

	plainA, _, err := w.CreateBindingToken(ch.Id, adminA.Id)
	if err != nil {
		t.Fatalf("创建首个绑定令牌失败：%v", err)
	}
	first, err := w.ConsumeBindingToken(plainA, ident)
	if err != nil {
		t.Fatalf("首次绑定失败：%v", err)
	}
	var firstTarget model.ChannelTarget
	if err := w.DB.Where("binding_id = ?", first.Id).Take(&firstTarget).Error; err != nil {
		t.Fatalf("读取首个私聊目标失败：%v", err)
	}

	if err = w.RevokeBinding(first.Id, adminA.Id); err != nil {
		t.Fatalf("撤销绑定失败：%v", err)
	}
	var revoked model.ChannelBinding
	if err := w.DB.Where("id = ?", first.Id).Take(&revoked).Error; err != nil {
		t.Fatalf("重读已撤销绑定失败：%v", err)
	}
	if revoked.RevokedAt == 0 {
		t.Fatal("撤销后 revoked_at 仍为 0")
	}
	if revoked.Revision != first.Revision+1 {
		t.Fatalf("撤销应让 revision 递增：前 %d，后 %d", first.Revision, revoked.Revision)
	}

	// 换绑：同一平台身份，由另一名管理员用新令牌重新绑定。
	plainB, _, err := w.CreateBindingToken(ch.Id, adminB.Id)
	if err != nil {
		t.Fatalf("创建换绑令牌失败：%v", err)
	}
	rebound, err := w.ConsumeBindingToken(plainB, ident)
	if err != nil {
		t.Fatalf("撤销后同一身份重新绑定失败：%v", err)
	}
	if rebound == nil {
		t.Fatal("换绑成功但没有返回绑定记录")
	}
	if rebound.Id != first.Id {
		// 唯一索引覆盖已撤销行，复活是唯一可行路径；id 变了说明实现换了方向（例如改写摘要）。
		t.Fatalf("换绑应复活原绑定行 id=%d，实际 %d", first.Id, rebound.Id)
	}
	if rebound.RevokedAt != 0 {
		t.Fatalf("换绑后 revoked_at 应为 0，实际 %d", rebound.RevokedAt)
	}
	if rebound.AdminId != adminB.Id {
		t.Fatalf("换绑后 admin_id 应为 B(%d)，实际 %d", adminB.Id, rebound.AdminId)
	}
	if rebound.PlatformUserId != ident.UserId || rebound.OrgId != ident.OrganizationId {
		t.Fatalf("换绑后平台身份字段不对：user=%q org=%q", rebound.PlatformUserId, rebound.OrgId)
	}
	if rebound.Revision != revoked.Revision+1 {
		t.Fatalf("换绑应让 revision 再递增 1：撤销后 %d，换绑后 %d", revoked.Revision, rebound.Revision)
	}

	var target model.ChannelTarget
	if err := w.DB.Where("target_hash = ?", wantTargetDigest(w.Id, ch.Id, "private", ident.UserId)).Take(&target).Error; err != nil {
		t.Fatalf("换绑后找不到私聊目标：%v", err)
	}
	if target.Id != firstTarget.Id {
		t.Fatalf("换绑应复用原私聊目标 id=%d，实际 %d", firstTarget.Id, target.Id)
	}
	if target.Approved != 1 {
		t.Fatalf("换绑后私聊目标应重新批准，实际 approved=%d", target.Approved)
	}
	if target.BindingId != rebound.Id {
		t.Fatalf("私聊目标指向绑定 %d，期望 %d", target.BindingId, rebound.Id)
	}

	var active int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ? AND revoked_at = 0", ch.Id).Count(&active).Error; err != nil {
		t.Fatalf("统计有效绑定失败：%v", err)
	}
	if active != 1 {
		t.Fatalf("换绑后有效绑定应恰好 1 条，实际 %d 条", active)
	}
	var rows int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ?", ch.Id).Count(&rows).Error; err != nil {
		t.Fatalf("统计绑定总行数失败：%v", err)
	}
	if rows != 1 {
		t.Fatalf("换绑应复用历史行，绑定总行数应为 1，实际 %d 条", rows)
	}
	var targets int64
	if err := w.DB.Model(&model.ChannelTarget{}).Where("channel_id = ?", ch.Id).Count(&targets).Error; err != nil {
		t.Fatalf("统计私聊目标失败：%v", err)
	}
	if targets != 1 {
		t.Fatalf("换绑应复用原私聊目标，目标行数应为 1，实际 %d 条", targets)
	}
}

// TestBindingTokenConsumeRejectsDisabledAdmin 令牌生成后管理员被停用，消费必须失败。
//
// 只校验生成时的状态是不够的：TTL 有一小时，期间停用的管理员仍能把私聊目标
// 落成 approved=1，之后的投递会一直发给已停用的人。失败还必须完全回滚 ——
// 管理员恢复后同一令牌应当仍然可用，否则一次误停用就废掉了管理员手上的令牌。
func TestBindingTokenConsumeRejectsDisabledAdmin(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)
	originalStatus := admin.Status
	// 共享开发库里的现有管理员：断言失败也必须恢复，否则整套后台登录都会被带坏。
	t.Cleanup(func() {
		if err := w.DB.Model(&model.Admin{}).Where("id = ?", admin.Id).Update("status", originalStatus).Error; err != nil {
			t.Errorf("恢复管理员 %d 的状态失败：%v", admin.Id, err)
		}
	})

	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}
	if err := w.DB.Model(&model.Admin{}).Where("id = ?", admin.Id).Update("status", 0).Error; err != nil {
		t.Fatalf("停用管理员失败：%v", err)
	}

	ident := testChannelIdentity(ch, "test_user_disabled")
	if _, err = w.ConsumeBindingToken(plain, ident); err == nil {
		t.Fatal("管理员已停用时仍然绑定成功")
	} else if !strings.Contains(err.Error(), "管理员") {
		t.Fatalf("停用管理员的错误提示应说明管理员不可用，实际 %q", err.Error())
	}

	var stored model.ChannelBindingToken
	if err := w.DB.Where("token_hash = ?", hashBindingToken(plain)).Take(&stored).Error; err != nil {
		t.Fatalf("读取令牌失败：%v", err)
	}
	if stored.ConsumedAt != 0 {
		t.Fatalf("管理员停用时令牌被消费掉了（事务未回滚）：consumed_at=%d", stored.ConsumedAt)
	}
	var bindings int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ?", ch.Id).Count(&bindings).Error; err != nil {
		t.Fatalf("统计绑定失败：%v", err)
	}
	if bindings != 0 {
		t.Fatalf("管理员停用时不应产生绑定，实际 %d 条", bindings)
	}

	// 恢复管理员后同一令牌仍可用，证明失败路径没有留下半成品。
	if err := w.DB.Model(&model.Admin{}).Where("id = ?", admin.Id).Update("status", originalStatus).Error; err != nil {
		t.Fatalf("恢复管理员状态失败：%v", err)
	}
	if _, err = w.ConsumeBindingToken(plain, ident); err != nil {
		t.Fatalf("管理员恢复后同一令牌不可用：%v", err)
	}
}

// TestBindingTokenRegenerateInvalidatesOld 生成新令牌必须让同管理员同渠道的旧令牌失效。
func TestBindingTokenRegenerateInvalidatesOld(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)

	oldPlain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建首个绑定令牌失败：%v", err)
	}
	newPlain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建第二个绑定令牌失败：%v", err)
	}
	if oldPlain == newPlain {
		t.Fatal("两次生成的明文相同，随机源有问题")
	}

	var oldToken model.ChannelBindingToken
	if err := w.DB.Where("token_hash = ?", hashBindingToken(oldPlain)).Take(&oldToken).Error; err != nil {
		t.Fatalf("读取旧令牌失败：%v", err)
	}
	// 失效手段是把 expires_at 压到当前时间（不借用 consumed_at：它的语义是「被消费过」）。
	if oldToken.ExpiresAt > time.Now().Unix() {
		t.Fatalf("重新生成后旧令牌未失效：expires_at=%d", oldToken.ExpiresAt)
	}
	if oldToken.ConsumedAt != 0 {
		t.Fatalf("旧令牌不应被标记为已消费：consumed_at=%d", oldToken.ConsumedAt)
	}

	ident := testChannelIdentity(ch, "test_user_regen")
	if _, err = w.ConsumeBindingToken(oldPlain, ident); err == nil {
		t.Fatal("重新生成后旧明文仍然可用")
	}
	binding, err := w.ConsumeBindingToken(newPlain, ident)
	if err != nil {
		t.Fatalf("新明文不可用：%v", err)
	}
	if binding.ChannelId != ch.Id || binding.AdminId != admin.Id {
		t.Fatalf("新明文产生的绑定归属不对：channel=%d admin=%d", binding.ChannelId, binding.AdminId)
	}
}

// TestBindingTokenRejectsGroupIdentity 绑定只允许在应用私聊里完成，群聊身份必须拒绝。
func TestBindingTokenRejectsGroupIdentity(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)

	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}
	ident := testChannelIdentity(ch, "test_user_group")
	ident.IsGroup = true
	if _, err = w.ConsumeBindingToken(plain, ident); err == nil {
		t.Fatal("群聊身份仍然可以完成绑定")
	} else if !strings.Contains(err.Error(), "私聊") {
		t.Fatalf("群聊绑定的错误提示应说明只能在私聊完成，实际 %q", err.Error())
	}

	var stored model.ChannelBindingToken
	if err := w.DB.Where("token_hash = ?", hashBindingToken(plain)).Take(&stored).Error; err != nil {
		t.Fatalf("读取令牌失败：%v", err)
	}
	if stored.ConsumedAt != 0 {
		t.Fatalf("群聊被拒后令牌被消费掉了（事务未回滚）：consumed_at=%d", stored.ConsumedAt)
	}
	var bindings int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ?", ch.Id).Count(&bindings).Error; err != nil {
		t.Fatalf("统计绑定失败：%v", err)
	}
	if bindings != 0 {
		t.Fatalf("群聊身份不应产生绑定，实际 %d 条", bindings)
	}
}

// TestBindingTokenRejectsAppOrOrgMismatch 平台应用或组织与渠道配置不一致必须拒绝。
// 这条是「令牌能不能跨应用用」的唯一门槛：不校验就会被别的企业应用消费掉。
func TestBindingTokenRejectsAppOrOrgMismatch(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)

	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}

	badApp := testChannelIdentity(ch, "test_user_mismatch")
	badApp.AppId = ch.AppId + "-other"
	if _, err = w.ConsumeBindingToken(plain, badApp); err == nil {
		t.Fatal("平台应用不匹配时仍然绑定成功")
	} else if !strings.Contains(err.Error(), "应用") {
		t.Fatalf("应用不匹配的错误提示应说明应用问题，实际 %q", err.Error())
	}

	badOrg := testChannelIdentity(ch, "test_user_mismatch")
	badOrg.OrganizationId = "other_org"
	if _, err = w.ConsumeBindingToken(plain, badOrg); err == nil {
		t.Fatal("组织不匹配时仍然绑定成功")
	} else if !strings.Contains(err.Error(), "组织") {
		t.Fatalf("组织不匹配的错误提示应说明组织问题，实际 %q", err.Error())
	}

	var stored model.ChannelBindingToken
	if err := w.DB.Where("token_hash = ?", hashBindingToken(plain)).Take(&stored).Error; err != nil {
		t.Fatalf("读取令牌失败：%v", err)
	}
	if stored.ConsumedAt != 0 {
		t.Fatalf("校验失败后令牌被消费掉了（事务未回滚）：consumed_at=%d", stored.ConsumedAt)
	}
	var bindings int64
	if err := w.DB.Model(&model.ChannelBinding{}).Where("channel_id = ?", ch.Id).Count(&bindings).Error; err != nil {
		t.Fatalf("统计绑定失败：%v", err)
	}
	if bindings != 0 {
		t.Fatalf("校验失败不应产生绑定，实际 %d 条", bindings)
	}

	// 正确身份仍然可用：两次拒绝都必须完整回滚。
	if _, err = w.ConsumeBindingToken(plain, testChannelIdentity(ch, "test_user_mismatch")); err != nil {
		t.Fatalf("校验失败后同一令牌在正确身份上不可用：%v", err)
	}
}

// TestBindingTokenDigestIgnoresSurroundingSpace 摘要口径按 TrimSpace 归一化：
// 平台回调和后台表单常带首尾空白，不归一化会让同一个人算出两个摘要、唯一索引形同虚设。
//
// 期望值用字面协议（want*Digest）算，输入取归一化后的值：仓库里的明文身份带空格、
// 期望摘要没有空格，两边只有真的做了 TrimSpace 才会相等。
func TestBindingTokenDigestIgnoresSurroundingSpace(t *testing.T) {
	w := setupChannelBindingTest(t)
	ch := newTestChannel(t, w)
	admin := activeTestAdmin(t, w)

	plain, _, err := w.CreateBindingToken(ch.Id, admin.Id)
	if err != nil {
		t.Fatalf("创建绑定令牌失败：%v", err)
	}
	ident := ChannelPlatformIdentity{
		ChannelId:      ch.Id,
		OrganizationId: "  " + ch.OrgId + "  ",
		AppId:          " " + ch.AppId + "\t",
		UserId:         "  test_user_padded\n",
	}
	const wantUser = "test_user_padded"
	binding, err := w.ConsumeBindingToken(plain, ident)
	if err != nil {
		t.Fatalf("带首尾空白的身份绑定失败：%v", err)
	}
	if binding.PlatformUserId != wantUser {
		t.Fatalf("落库的平台用户标识未归一化：got %q want %q", binding.PlatformUserId, wantUser)
	}
	if binding.OrgId != ch.OrgId {
		t.Fatalf("落库的组织标识未归一化：got %q want %q", binding.OrgId, ch.OrgId)
	}
	if got, want := binding.IdentityHash, wantIdentityDigest(w.Id, ch.Id, ch.OrgId, wantUser); got != want {
		t.Fatalf("带空白的身份摘要与去空白口径不一致：got %q want %q", got, want)
	}
	var target model.ChannelTarget
	if err := w.DB.Where("binding_id = ?", binding.Id).Take(&target).Error; err != nil {
		t.Fatalf("读取私聊目标失败：%v", err)
	}
	if got, want := target.TargetHash, wantTargetDigest(w.Id, ch.Id, "private", wantUser); got != want {
		t.Fatalf("带空白的目标摘要与去空白口径不一致：got %q want %q", got, want)
	}
}
