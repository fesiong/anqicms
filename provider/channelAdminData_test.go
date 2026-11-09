package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm/logger"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// 本文件是 T9b（后台渠道数据面）的护栏：绑定、目标、规则、模板、记录、试发、投递重试。
//
// 全部用例走真实开发库（站点 1），随机后缀 + t.Cleanup 按自己创建的 ID 精确清理。
// 平台网络一律不碰：试发用例注入 channelTestAdapterFactory。

// ============================================================
// 夹具
// ============================================================

// channelDataTestSite 在通知夹具之上补齐渠道审计表。
func channelDataTestSite(t *testing.T) *Website {
	t.Helper()
	w := notifyTestSite(t)
	if err := w.DB.AutoMigrate(&model.ChannelAudit{}); err != nil {
		t.Fatalf("迁移渠道审计表失败：%v", err)
	}
	return w
}

// channelDataSeq 与时间戳组合出 12 位随机后缀：
// 管理员 user_name 列只有 30 字符，完整 UnixNano（19 位）加上前缀会超长。
var channelDataSeq int64

func channelDataSuffix() string {
	seq := atomic.AddInt64(&channelDataSeq, 1) % 10000
	return fmt.Sprintf("%08d%04d", time.Now().UnixNano()%100000000, seq)
}

// channelDataTestAdmin 自建一个启用状态的管理员（GroupId 由调用方指定），
// 用完后按 ID 删除。真实库里的管理员不参与创建/删除，避免污染站点配置。
func channelDataTestAdmin(t *testing.T, w *Website, groupId uint) *model.Admin {
	t.Helper()
	suffix := channelDataSuffix()
	admin := &model.Admin{
		UserName: "channel_data_admin_" + suffix,
		Status:   1,
		GroupId:  groupId,
	}
	if err := w.DB.Create(admin).Error; err != nil {
		t.Fatalf("创建测试管理员失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", admin.Id).Delete(&model.Admin{}).Error; err != nil {
			t.Logf("清理测试管理员 %d 失败：%v", admin.Id, err)
		}
	})
	return admin
}

// channelDataBindingRow 直接写一条绑定行：列表/撤销只依赖 admin_id 与站点，
// 不需要走完整的令牌消费流程（那条链路由 channelBinding_test.go 覆盖）。
func channelDataBindingRow(t *testing.T, w *Website, ch *model.MessageChannel, adminId uint) *model.ChannelBinding {
	t.Helper()
	suffix := channelDataSuffix()
	platformUserId := "data_user_" + suffix
	binding := &model.ChannelBinding{
		SiteId:         w.Id,
		ChannelId:      ch.Id,
		OrgId:          ch.OrgId,
		PlatformUserId: platformUserId,
		AdminId:        adminId,
		Revision:       1,
		IdentityHash:   bindingIdentityDigest(w.Id, ch.Id, ch.OrgId, platformUserId),
	}
	if err := w.DB.Create(binding).Error; err != nil {
		t.Fatalf("创建测试绑定失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", binding.Id).Delete(&model.ChannelBinding{}).Error; err != nil {
			t.Logf("清理绑定 %d 失败：%v", binding.Id, err)
		}
	})
	return binding
}

// wantChannelTargetDigest 用字面协议独立算目标摘要（站点 \x1f 渠道 \x1f 类型 \x1f 平台目标）。
// 刻意不复用被测的 bindingTargetDigest：两边一起漂移时断言会永远成立。
func wantChannelTargetDigest(siteId, channelId uint, kind, platformTargetId string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\x1f%d\x1f%s\x1f%s", siteId, channelId, kind, platformTargetId)))
	return hex.EncodeToString(sum[:])
}

func channelDataTargetRow(t *testing.T, w *Website, siteId, channelId uint, kind, platformTargetId string, approved, managementEnabled int) *model.ChannelTarget {
	t.Helper()
	target := &model.ChannelTarget{
		SiteId:            siteId,
		ChannelId:         channelId,
		Kind:              kind,
		PlatformTargetId:  platformTargetId,
		Approved:          approved,
		ManagementEnabled: managementEnabled,
		TargetHash:        wantChannelTargetDigest(siteId, channelId, kind, platformTargetId),
	}
	if err := w.DB.Create(target).Error; err != nil {
		t.Fatalf("创建测试目标失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", target.Id).Delete(&model.ChannelTarget{}).Error; err != nil {
			t.Logf("清理目标 %d 失败：%v", target.Id, err)
		}
	})
	return target
}

func channelDataTemplateRow(t *testing.T, w *Website, siteId uint, key, subject, content string) *model.NotificationTemplate {
	t.Helper()
	tpl := &model.NotificationTemplate{
		SiteId:  siteId,
		Key:     key,
		Name:    "测试模板",
		Subject: subject,
		Content: content,
	}
	if err := w.DB.Create(tpl).Error; err != nil {
		t.Fatalf("创建测试通知模板失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", tpl.Id).Delete(&model.NotificationTemplate{}).Error; err != nil {
			t.Logf("清理通知模板 %d 失败：%v", tpl.Id, err)
		}
	})
	return tpl
}

// channelDataDeliverySeq 保证同一目标上的多条测试投递不会撞 uk_delivery_hash
// （摘要 = event+rule+target，三个都相同会被当成重复扇出）。
var channelDataDeliverySeq int64

func channelDataDeliveryRow(t *testing.T, w *Website, siteId, targetId uint, status string) *model.NotificationDelivery {
	t.Helper()
	suffix := channelDataSuffix()
	channelDataDeliverySeq++
	delivery := &model.NotificationDelivery{
		SiteId:        siteId,
		EventId:       uint(900000000 + channelDataDeliverySeq),
		RuleId:        1,
		TargetId:      targetId,
		Status:        status,
		Attempts:      3,
		NextAttemptAt: time.Now().Add(-time.Minute).Unix(),
		LeaseUntil:    time.Now().Add(time.Minute).Unix(),
		LastError:     "permanent: 历史失败-" + suffix,
		Subject:       "标题-" + suffix,
		Content:       "正文哨兵-" + suffix,
	}
	if err := w.DB.Create(delivery).Error; err != nil {
		t.Fatalf("创建测试投递失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", delivery.Id).Delete(&model.NotificationDelivery{}).Error; err != nil {
			t.Logf("清理投递 %d 失败：%v", delivery.Id, err)
		}
	})
	return delivery
}

// newChannelDataForeignChannel 建一条「异站点」渠道（SiteId 指向不存在的站点）。
func newChannelDataForeignChannel(t *testing.T, w *Website) *model.MessageChannel {
	t.Helper()
	suffix := channelDataSuffix()
	foreign := &model.MessageChannel{
		SiteId:    w.Id + 90001,
		Platform:  model.ChannelPlatformFeishu,
		Name:      "异站点渠道-" + suffix,
		AppId:     "foreign_app_" + suffix,
		OrgId:     "foreign_org_" + suffix,
		AppSecret: "foreign_secret_" + suffix,
		Revision:  1,
	}
	if err := w.DB.Create(foreign).Error; err != nil {
		t.Fatalf("创建异站点渠道失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", foreign.Id).Delete(&model.MessageChannel{}).Error; err != nil {
			t.Logf("清理异站点渠道 %d 失败：%v", foreign.Id, err)
		}
	})
	return foreign
}

func channelBindingViewContains(list []ChannelBindingView, id uint) bool {
	for _, item := range list {
		if item.Id == id {
			return true
		}
	}
	return false
}

// ============================================================
// R89 绑定
// ============================================================

// TestChannelBindingListIsSelfOnly 锁住「默认只看本人」与「跨人操作需要超级管理员」。
//
// 反向对照：非超级管理员（自建 group_id=2）用 all=1 必须被拒绝，撤销他人绑定也必须被拒绝；
// 只测「本人能看自己」证明不了越权被挡住。
func TestChannelBindingListIsSelfOnly(t *testing.T) {
	w := channelDataTestSite(t)
	ch := newManagerTestChannel(t, w, false)
	self := channelDataTestAdmin(t, w, 2)
	other := channelDataTestAdmin(t, w, 2)

	mine := channelDataBindingRow(t, w, ch, self.Id)
	theirs := channelDataBindingRow(t, w, ch, other.Id)

	list, err := w.ChannelBindings(self.Id, false)
	if err != nil {
		t.Fatalf("读取本人绑定失败：%v", err)
	}
	if !channelBindingViewContains(list, mine.Id) {
		t.Fatalf("默认列表必须包含本人的绑定 %d", mine.Id)
	}
	if channelBindingViewContains(list, theirs.Id) {
		t.Fatalf("默认列表不得包含他人的绑定 %d", theirs.Id)
	}
	for _, item := range list {
		if item.AdminId != self.Id {
			t.Fatalf("默认列表只能返回本人绑定，出现管理员 %d", item.AdminId)
		}
	}
	rawList, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("序列化绑定列表失败：%v", err)
	}
	if strings.Contains(string(rawList), "identity_hash") || strings.Contains(string(rawList), mine.IdentityHash) {
		t.Fatal("绑定列表不得输出身份摘要")
	}

	if _, err := w.ChannelBindings(self.Id, true); !errors.Is(err, ErrChannelForbidden) {
		t.Fatalf("非超级管理员的 all=1 必须被拒绝，实得 %v", err)
	}

	all, err := w.ChannelBindings(1, true)
	if err != nil {
		t.Fatalf("超级管理员查看全部绑定失败：%v", err)
	}
	if !channelBindingViewContains(all, mine.Id) || !channelBindingViewContains(all, theirs.Id) {
		t.Fatalf("超级管理员 all=1 必须同时看到两条绑定（本人 %d / 他人 %d）", mine.Id, theirs.Id)
	}

	// 反向对照：普通管理员不能撤销别人的绑定。
	if err := w.ChannelBindingRevoke(theirs.Id, self.Id); !errors.Is(err, ErrChannelForbidden) {
		t.Fatalf("普通管理员撤销他人绑定必须被拒绝，实得 %v", err)
	}
	var untouched model.ChannelBinding
	if err := w.DB.Where("id = ?", theirs.Id).Take(&untouched).Error; err != nil {
		t.Fatalf("回读他人绑定失败：%v", err)
	}
	if untouched.RevokedAt != 0 {
		t.Fatalf("被拒绝的撤销不得改动绑定，revoked_at=%d", untouched.RevokedAt)
	}

	if err := w.ChannelBindingRevoke(mine.Id, self.Id); err != nil {
		t.Fatalf("本人撤销自己的绑定必须成功：%v", err)
	}
	var revoked model.ChannelBinding
	if err := w.DB.Where("id = ?", mine.Id).Take(&revoked).Error; err != nil {
		t.Fatalf("回读本人绑定失败：%v", err)
	}
	if revoked.RevokedAt == 0 {
		t.Fatal("撤销后 revoked_at 必须被写入")
	}

	if err := w.ChannelBindingRevoke(theirs.Id, 1); err != nil {
		t.Fatalf("超级管理员撤销他人绑定必须成功：%v", err)
	}
	var revokedBySuper model.ChannelBinding
	if err := w.DB.Where("id = ?", theirs.Id).Take(&revokedBySuper).Error; err != nil {
		t.Fatalf("回读他人绑定失败：%v", err)
	}
	if revokedBySuper.RevokedAt == 0 {
		t.Fatal("超级管理员撤销后 revoked_at 必须被写入")
	}
}

// ============================================================
// R90 目标
// ============================================================

// TestChannelTargetRejectsPrivateAndForeignSite 锁住「私聊目标只能由绑定流程创建」与站点隔离。
func TestChannelTargetRejectsPrivateAndForeignSite(t *testing.T) {
	w := channelDataTestSite(t)
	ch := newManagerTestChannel(t, w, false)
	foreign := newChannelDataForeignChannel(t, w)
	suffix := channelDataSuffix()

	if _, err := w.ChannelTargetSave(&request.ChannelTargetRequest{
		ChannelId:        ch.Id,
		Kind:             TargetKindPrivate,
		PlatformTargetId: "private_" + suffix,
		Approved:         1,
	}); err == nil {
		t.Fatal("手工创建私聊目标必须被拒绝（私聊只能由绑定流程产生）")
	}
	var count int64
	if err := w.DB.Model(&model.ChannelTarget{}).Where("channel_id = ?", ch.Id).Count(&count).Error; err != nil {
		t.Fatalf("统计目标失败：%v", err)
	}
	if count != 0 {
		t.Fatalf("被拒绝的私聊目标不得落库，实际 %d 行", count)
	}

	if _, err := w.ChannelTargetSave(&request.ChannelTargetRequest{
		ChannelId:        foreign.Id,
		Kind:             TargetKindGroup,
		PlatformTargetId: "foreign_group_" + suffix,
	}); !errors.Is(err, errChannelAdminNotFound) {
		t.Fatalf("异站点渠道的目标必须按「渠道不存在」拒绝，实得 %v", err)
	}

	if _, err := w.ChannelTargetSave(&request.ChannelTargetRequest{
		ChannelId: ch.Id,
		Kind:      TargetKindGroup,
	}); err == nil {
		t.Fatal("平台目标标识为空必须被拒绝")
	}
	if _, err := w.ChannelTargetSave(&request.ChannelTargetRequest{
		ChannelId:        ch.Id,
		Kind:             TargetKindGroup,
		PlatformTargetId: "grp_" + suffix,
		Approved:         2,
	}); err == nil {
		t.Fatal("approved 只接受 0/1")
	}
	if _, err := w.ChannelTargetSave(&request.ChannelTargetRequest{
		ChannelId:         ch.Id,
		Kind:              TargetKindGroup,
		PlatformTargetId:  "grp_" + suffix,
		ManagementEnabled: 5,
	}); err == nil {
		t.Fatal("management_enabled 只接受 0/1")
	}

	platformTargetId := "grp_" + suffix
	created, err := w.ChannelTargetSave(&request.ChannelTargetRequest{
		ChannelId:         ch.Id,
		Kind:              TargetKindGroup,
		PlatformTargetId:  platformTargetId,
		Approved:          1,
		ManagementEnabled: 1,
	})
	if err != nil {
		t.Fatalf("创建群目标失败：%v", err)
	}
	if created.Id == 0 || created.Approved != 1 || created.ManagementEnabled != 1 {
		t.Fatalf("创建结果不对：%+v", created)
	}
	var row model.ChannelTarget
	if err := w.DB.Where("id = ?", created.Id).Take(&row).Error; err != nil {
		t.Fatalf("回读目标失败：%v", err)
	}
	if row.TargetHash != wantChannelTargetDigest(w.Id, ch.Id, TargetKindGroup, platformTargetId) {
		t.Fatalf("目标摘要口径不对：%q", row.TargetHash)
	}
	if row.BindingId != 0 {
		t.Fatalf("手工群目标不得关联绑定，实际 binding_id=%d", row.BindingId)
	}

	// 反向对照：已有群目标不能通过保存接口改成私聊（绕过绑定流程）。
	if _, err := w.ChannelTargetSave(&request.ChannelTargetRequest{
		Id:               created.Id,
		ChannelId:        ch.Id,
		Kind:             TargetKindPrivate,
		PlatformTargetId: platformTargetId,
	}); err == nil {
		t.Fatal("把群目标改成私聊必须被拒绝")
	}
	var reloaded model.ChannelTarget
	if err := w.DB.Where("id = ?", created.Id).Take(&reloaded).Error; err != nil {
		t.Fatalf("回读目标失败：%v", err)
	}
	if reloaded.Kind != TargetKindGroup || reloaded.Approved != 1 {
		t.Fatalf("被拒绝的更新不得改动目标：kind=%q approved=%d", reloaded.Kind, reloaded.Approved)
	}

	// 取消批准用 approved=0（不提供删除端点）。
	updated, err := w.ChannelTargetSave(&request.ChannelTargetRequest{
		Id:               created.Id,
		ChannelId:        ch.Id,
		Kind:             TargetKindGroup,
		PlatformTargetId: platformTargetId,
		Approved:         0,
	})
	if err != nil {
		t.Fatalf("取消批准失败：%v", err)
	}
	if updated.Approved != 0 {
		t.Fatalf("取消批准后 approved 应为 0，实际 %d", updated.Approved)
	}

	list, err := w.ChannelTargets(ch.Id)
	if err != nil {
		t.Fatalf("读取目标列表失败：%v", err)
	}
	found := false
	for _, item := range list {
		if item.Id == created.Id {
			found = true
		}
	}
	if !found {
		t.Fatalf("目标列表必须包含 %d", created.Id)
	}
	if _, err := w.ChannelTargets(foreign.Id); !errors.Is(err, errChannelAdminNotFound) {
		t.Fatalf("异站点渠道的目标列表必须按「渠道不存在」拒绝，实得 %v", err)
	}

	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", created.Id).Delete(&model.ChannelTarget{}).Error; err != nil {
			t.Logf("清理目标 %d 失败：%v", created.Id, err)
		}
	})
}

// ============================================================
// R91 规则
// ============================================================

// TestChannelRuleValidatesTargetsAndEventType 锁住事件类型白名单与目标/模板的站点归属。
func TestChannelRuleValidatesTargetsAndEventType(t *testing.T) {
	w := channelDataTestSite(t)
	ch := newManagerTestChannel(t, w, false)
	target := channelDataTargetRow(t, w, w.Id, ch.Id, TargetKindGroup, "rule_grp_"+channelDataSuffix(), 1, 0)
	foreignTarget := channelDataTargetRow(t, w, w.Id+90001, ch.Id, TargetKindGroup, "rule_foreign_"+channelDataSuffix(), 1, 0)
	tpl := channelDataTemplateRow(t, w, w.Id, "rule_tpl_"+channelDataSuffix(), "标题", "正文")
	foreignTpl := channelDataTemplateRow(t, w, w.Id+90001, "rule_foreign_tpl_"+channelDataSuffix(), "标题", "正文")

	invalid := []struct {
		name string
		req  *request.ChannelRuleRequest
	}{
		{"未知事件类型", &request.ChannelRuleRequest{EventType: "channel.unknown", Enabled: 1, TargetIds: []uint{target.Id}}},
		{"非法启用值", &request.ChannelRuleRequest{EventType: NotifyEventGuestbookCreated, Enabled: 2, TargetIds: []uint{target.Id}}},
		{"目标不存在", &request.ChannelRuleRequest{EventType: NotifyEventGuestbookCreated, Enabled: 1, TargetIds: []uint{target.Id, 999999999}}},
		{"目标属于异站点", &request.ChannelRuleRequest{EventType: NotifyEventGuestbookCreated, Enabled: 1, TargetIds: []uint{foreignTarget.Id}}},
		{"模板属于异站点", &request.ChannelRuleRequest{EventType: NotifyEventGuestbookCreated, Enabled: 1, TargetIds: []uint{target.Id}, TemplateId: foreignTpl.Id}},
		{"模板不存在", &request.ChannelRuleRequest{EventType: NotifyEventGuestbookCreated, Enabled: 1, TargetIds: []uint{target.Id}, TemplateId: 999999999}},
	}
	for _, tc := range invalid {
		if _, err := w.ChannelRuleSave(tc.req); err == nil {
			t.Errorf("%s 应被拒绝", tc.name)
		}
	}

	created, err := w.ChannelRuleSave(&request.ChannelRuleRequest{
		Name:       "规则-" + channelDataSuffix(),
		EventType:  NotifyEventGuestbookCreated,
		Enabled:    1,
		TargetIds:  []uint{target.Id},
		TemplateId: tpl.Id,
	})
	if err != nil {
		t.Fatalf("创建规则失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", created.Id).Delete(&model.NotificationRule{}).Error; err != nil {
			t.Logf("清理规则 %d 失败：%v", created.Id, err)
		}
	})
	var row model.NotificationRule
	if err := w.DB.Where("id = ?", created.Id).Take(&row).Error; err != nil {
		t.Fatalf("回读规则失败：%v", err)
	}
	var targetIds []uint
	if err := json.Unmarshal([]byte(row.TargetIds), &targetIds); err != nil {
		t.Fatalf("规则目标必须是 JSON 数组：%v", err)
	}
	if len(targetIds) != 1 || targetIds[0] != target.Id {
		t.Fatalf("规则目标落库不对：%+v", targetIds)
	}

	list, err := w.ChannelRules()
	if err != nil {
		t.Fatalf("读取规则列表失败：%v", err)
	}
	found := false
	for _, item := range list {
		if item.Id == created.Id {
			found = true
			if len(item.TargetIds) != 1 || item.TargetIds[0] != target.Id {
				t.Fatalf("规则视图目标不对：%+v", item.TargetIds)
			}
		}
	}
	if !found {
		t.Fatalf("规则列表必须包含 %d", created.Id)
	}

	if err := w.ChannelRuleDelete(created.Id); err != nil {
		t.Fatalf("删除规则失败：%v", err)
	}
	if err := w.ChannelRuleDelete(created.Id); !errors.Is(err, errChannelRuleNotFound) {
		t.Fatalf("重复删除必须报「规则不存在」，实得 %v", err)
	}
	var remaining int64
	if err := w.DB.Model(&model.NotificationRule{}).Where("id = ?", created.Id).Count(&remaining).Error; err != nil {
		t.Fatalf("统计规则失败：%v", err)
	}
	if remaining != 0 {
		t.Fatal("删除后规则行仍存在")
	}
}

// ============================================================
// R92 模板
// ============================================================

// TestChannelTemplatePreviewHidesPrivateFieldsForGroup 锁住「群预览看不到私聊字段」，
// 并顺带锁住模板保存前的词法白名单与唯一键。
func TestChannelTemplatePreviewHidesPrivateFieldsForGroup(t *testing.T) {
	w := channelDataTestSite(t)
	suffix := channelDataSuffix()

	payload, err := json.Marshal(map[string]any{
		"guestbook_id": 1,
		"user_name":    "姓名哨兵TD1",
		"contact":      "联系哨兵TD2",
		"content":      "正文哨兵TD3",
		"buyer_name":   "买家哨兵TD4",
		"buyer_phone":  "手机哨兵TD5",
		"buyer_email":  "邮箱哨兵TD6",
	})
	if err != nil {
		t.Fatalf("序列化预览载荷失败：%v", err)
	}
	content := "留言人：{{ guestbook.user_name }} 联系方式：{{ guestbook.contact }} " +
		"正文：{{ guestbook.content }} 手机：{{ order.buyer_phone }}"
	sentinels := []string{"TD1", "TD2", "TD3", "TD5"}

	group, err := w.ChannelTemplatePreview(&request.ChannelTemplatePreviewRequest{
		Subject:     "[{{ site_name }}] {{ event_label }}",
		Content:     content,
		TargetKind:  TargetKindGroup,
		PayloadJson: string(payload),
	})
	if err != nil {
		t.Fatalf("群预览失败：%v", err)
	}
	for _, sentinel := range sentinels {
		if strings.Contains(group.Subject, sentinel) || strings.Contains(group.Text, sentinel) {
			t.Fatalf("群预览出现了私聊字段 %s：%q", sentinel, group.Subject+group.Text)
		}
	}

	private, err := w.ChannelTemplatePreview(&request.ChannelTemplatePreviewRequest{
		Subject:     "[{{ site_name }}] {{ event_label }}",
		Content:     content,
		TargetKind:  TargetKindPrivate,
		PayloadJson: string(payload),
	})
	if err != nil {
		t.Fatalf("私聊预览失败：%v", err)
	}
	for _, sentinel := range sentinels {
		if !strings.Contains(private.Text, sentinel) {
			t.Fatalf("私聊预览必须渲染出 %s，实际 %q", sentinel, private.Text)
		}
	}

	if _, err := w.ChannelTemplatePreview(&request.ChannelTemplatePreviewRequest{
		Content:    "x",
		TargetKind: "broadcast",
	}); err == nil {
		t.Fatal("target_kind 只接受 group/private")
	}
	if _, err := w.ChannelTemplatePreview(&request.ChannelTemplatePreviewRequest{
		Content:     "{{ site_name }}",
		TargetKind:  TargetKindGroup,
		PayloadJson: "{不是JSON",
	}); err == nil {
		t.Fatal("非法 payload_json 必须被拒绝")
	}

	// 已保存模板走 id 预览：模板正文引用私聊字段，群预览同样不得出现哨兵。
	saved := channelDataTemplateRow(t, w, w.Id, "preview_"+suffix, "标题", content)
	byId, err := w.ChannelTemplatePreview(&request.ChannelTemplatePreviewRequest{
		Id:          saved.Id,
		TargetKind:  TargetKindGroup,
		PayloadJson: string(payload),
	})
	if err != nil {
		t.Fatalf("按模板ID预览失败：%v", err)
	}
	for _, sentinel := range sentinels {
		if strings.Contains(byId.Text, sentinel) {
			t.Fatalf("按ID群预览出现了私聊字段 %s：%q", sentinel, byId.Text)
		}
	}

	// 模板保存：词法白名单与 (site_id,key) 唯一。
	if err := ValidateNotificationTemplate("标题", `{% system with name="SiteName" %}`); err == nil {
		t.Fatal("ValidateNotificationTemplate 必须拒绝 system 标签")
	}
	if err := ValidateNotificationTemplate("标题", `{{ x|safe }}`); err == nil {
		t.Fatal("ValidateNotificationTemplate 必须拒绝 safe 过滤器")
	}
	if err := ValidateNotificationTemplate("标题", "正文 {{ site_name }}"); err != nil {
		t.Fatalf("合法模板不得被拒绝：%v", err)
	}

	key := "saved_" + suffix
	created, err := w.ChannelTemplateSave(&request.ChannelTemplateRequest{
		Key:     key,
		Name:    "模板",
		Subject: "标题",
		Content: "正文",
	})
	if err != nil {
		t.Fatalf("保存模板失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", created.Id).Delete(&model.NotificationTemplate{}).Error; err != nil {
			t.Logf("清理模板 %d 失败：%v", created.Id, err)
		}
	})
	if _, err := w.ChannelTemplateSave(&request.ChannelTemplateRequest{
		Key:     key,
		Name:    "重复",
		Subject: "标题",
		Content: "正文",
	}); !errors.Is(err, errChannelTemplateKeyConflict) {
		t.Fatalf("重复模板键必须报静态冲突文案，实得 %v", err)
	}
	if _, err := w.ChannelTemplateSave(&request.ChannelTemplateRequest{
		Key:     "bad_" + suffix,
		Subject: "标题",
		Content: `{% include "x" %}`,
	}); err == nil {
		t.Fatal("含非法标签的模板必须被拒绝")
	}
	list, err := w.ChannelTemplates()
	if err != nil {
		t.Fatalf("读取模板列表失败：%v", err)
	}
	found := false
	for _, item := range list {
		if item.Id == created.Id {
			found = true
		}
	}
	if !found {
		t.Fatalf("模板列表必须包含 %d", created.Id)
	}
}

// ============================================================
// R93 记录
// ============================================================

// TestChannelRecordsProjectionHidesSensitiveFields 用真实 JSON 断言禁止字段的**缺席**。
//
// 只断言允许字段存在是空转的：模型序列化会把 content/canonical_args/result_summary/public_id
// 一起带出来，测试照样通过。所以这里逐个比对 "字段名": 键。
func TestChannelRecordsProjectionHidesSensitiveFields(t *testing.T) {
	w := channelDataTestSite(t)
	ch := newManagerTestChannel(t, w, false)
	suffix := channelDataSuffix()

	target := channelDataTargetRow(t, w, w.Id, ch.Id, TargetKindGroup, "rec_grp_"+suffix, 1, 0)

	event := &model.NotificationEvent{
		SiteId:       w.Id,
		EventType:    NotifyEventGuestbookCreated,
		EventKey:     "records:" + suffix,
		OccurredAt:   time.Now().Unix(),
		PayloadJson:  `{"content":"事件正文哨兵"}`,
		Status:       NotifyEventStatusReady,
		EventKeyHash: notifyEventKeyHash("records:" + suffix),
	}
	if err := w.DB.Create(event).Error; err != nil {
		t.Fatalf("创建事件失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", event.Id).Delete(&model.NotificationEvent{}).Error; err != nil {
			t.Logf("清理事件 %d 失败：%v", event.Id, err)
		}
	})

	rule := &model.NotificationRule{
		SiteId:    w.Id,
		Name:      "记录规则-" + suffix,
		EventType: NotifyEventGuestbookCreated,
		Enabled:   1,
		TargetIds: fmt.Sprintf("[%d]", target.Id),
	}
	if err := w.DB.Create(rule).Error; err != nil {
		t.Fatalf("创建规则失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", rule.Id).Delete(&model.NotificationRule{}).Error; err != nil {
			t.Logf("清理规则 %d 失败：%v", rule.Id, err)
		}
	})

	delivery := channelDataDeliveryRow(t, w, w.Id, target.Id, NotifyDeliveryStatusFailed)

	inbox := &model.ChannelInbox{
		SiteId:    w.Id,
		ChannelId: ch.Id,
		EventKind: "message",
		Content:   "入站正文哨兵-" + suffix,
		Status:    model.InboxStatusPending,
		EventHash: channelDigest(channelDataSuffix()),
	}
	if err := w.DB.Create(inbox).Error; err != nil {
		t.Fatalf("创建入站任务失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", inbox.Id).Delete(&model.ChannelInbox{}).Error; err != nil {
			t.Logf("清理入站任务 %d 失败：%v", inbox.Id, err)
		}
	})

	audit := &model.ChannelAudit{
		SiteId:     w.Id,
		ChannelId:  ch.Id,
		TaskId:     "task-" + suffix,
		Action:     "record.audit",
		TargetHash: channelDigest("audit-" + suffix),
		Result:     "ok",
	}
	if err := w.DB.Create(audit).Error; err != nil {
		t.Fatalf("创建审计失败：%v", err)
	}
	t.Cleanup(func() {
		if err := w.DB.Where("id = ?", audit.Id).Delete(&model.ChannelAudit{}).Error; err != nil {
			t.Logf("清理审计 %d 失败：%v", audit.Id, err)
		}
	})

	forbiddenKeys := []string{"\"content\":", "\"canonical_args\":", "\"result_summary\":", "\"public_id\":", "\"identity_hash\":", "\"target_hash\":", "\"event_hash\":"}
	cases := []struct {
		name       string
		recordType string
		wantID     uint
		wantKeys   []string
	}{
		{"delivery", "delivery", delivery.Id, []string{"\"platform_message_id\":", "\"attempts\":", "\"next_attempt_at\":"}},
		{"inbox", "inbox", inbox.Id, []string{"\"event_kind\":", "\"lease_until\":"}},
		{"audit", "audit", audit.Id, []string{"\"action\":", "\"task_id\":"}},
	}
	for _, tc := range cases {
		page, err := w.ChannelRecords(tc.recordType, "", 1, 1000)
		if err != nil {
			t.Fatalf("%s 记录读取失败：%v", tc.name, err)
		}
		if page.Page != 1 || page.PageSize != 100 {
			t.Fatalf("%s 分页参数不对：page=%d page_size=%d（page_size 上限 100）", tc.name, page.Page, page.PageSize)
		}
		if page.Total < 1 {
			t.Fatalf("%s 记录总数必须大于 0", tc.name)
		}
		raw, err := json.Marshal(page.List)
		if err != nil {
			t.Fatalf("%s 记录序列化失败：%v", tc.name, err)
		}
		body := string(raw)
		for _, key := range forbiddenKeys {
			if strings.Contains(body, key) {
				t.Fatalf("%s 记录出现了禁止字段 %s：%s", tc.name, key, body)
			}
		}
		for _, key := range tc.wantKeys {
			if !strings.Contains(body, key) {
				t.Fatalf("%s 记录缺少允许字段 %s：%s", tc.name, key, body)
			}
		}
		if !strings.Contains(body, fmt.Sprintf("\"id\":%d", tc.wantID)) {
			t.Fatalf("%s 记录必须包含本次创建的行 %d", tc.name, tc.wantID)
		}
		if !strings.Contains(body, suffix) && tc.recordType == "audit" {
			// 审计的 task_id 带后缀：证明投影的是本用例的行，而不是别站点的同 ID 行。
			t.Fatalf("审计记录缺少本用例的 task_id")
		}
	}

	page, err := w.ChannelRecords("delivery", NotifyDeliveryStatusFailed, 1, 0)
	if err != nil {
		t.Fatalf("按状态筛选失败：%v", err)
	}
	if page.PageSize != 20 {
		t.Fatalf("page_size 缺省应为 20，实际 %d", page.PageSize)
	}
	raw, err := json.Marshal(page.List)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	if strings.Contains(string(raw), fmt.Sprintf("\"id\":%d", delivery.Id)) == false {
		t.Fatalf("失败状态筛选必须包含本次投递 %d", delivery.Id)
	}

	if _, err := w.ChannelRecords("", "", 1, 20); err == nil {
		t.Fatal("缺少 type 必须被拒绝")
	}
	if _, err := w.ChannelRecords("message", "", 1, 20); err == nil {
		t.Fatal("未知 type 必须被拒绝")
	}
}

// ============================================================
// R95 投递重试
// ============================================================

// TestDeliveryManualRetryOnlyFailedOrUnknown 锁住「只有 failed/unknown 可人工重试」。
func TestDeliveryManualRetryOnlyFailedOrUnknown(t *testing.T) {
	w := channelDataTestSite(t)
	ch := newManagerTestChannel(t, w, false)
	target := channelDataTargetRow(t, w, w.Id, ch.Id, TargetKindGroup, "retry_grp_"+channelDataSuffix(), 1, 0)

	failed := channelDataDeliveryRow(t, w, w.Id, target.Id, NotifyDeliveryStatusFailed)
	unknown := channelDataDeliveryRow(t, w, w.Id, target.Id, NotifyDeliveryStatusUnknown)
	sent := channelDataDeliveryRow(t, w, w.Id, target.Id, NotifyDeliveryStatusSent)
	pending := channelDataDeliveryRow(t, w, w.Id, target.Id, NotifyDeliveryStatusPending)
	foreign := channelDataDeliveryRow(t, w, w.Id+90001, target.Id, NotifyDeliveryStatusFailed)

	if err := w.ChannelDeliveryRetry(failed.Id); err != nil {
		t.Fatalf("failed 投递必须可重试：%v", err)
	}
	var retried model.NotificationDelivery
	if err := w.DB.Where("id = ?", failed.Id).Take(&retried).Error; err != nil {
		t.Fatalf("回读投递失败：%v", err)
	}
	if retried.Status != NotifyDeliveryStatusRetry {
		t.Fatalf("重试后状态必须是 retry，实际 %q", retried.Status)
	}
	if retried.Attempts != 0 || retried.LeaseUntil != 0 || retried.LastError != "" {
		t.Fatalf("重试必须重置尝试次数/租约/错误：%+v", retried)
	}
	if retried.NextAttemptAt <= 0 || retried.NextAttemptAt > time.Now().Add(time.Minute).Unix() {
		t.Fatalf("重试必须把下次尝试时间设为当前时间，实际 %d", retried.NextAttemptAt)
	}

	if err := w.ChannelDeliveryRetry(unknown.Id); err != nil {
		t.Fatalf("unknown 投递必须可重试：%v", err)
	}

	for _, tc := range []struct {
		name string
		id   uint
	}{{"已发送", sent.Id}, {"待发送", pending.Id}} {
		err := w.ChannelDeliveryRetry(tc.id)
		if err == nil {
			t.Fatalf("%s 的投递不得人工重试", tc.name)
		}
		if !strings.Contains(err.Error(), "仅失败或结果未知") {
			t.Fatalf("%s 必须返回静态文案，实得 %v", tc.name, err)
		}
		var untouched model.NotificationDelivery
		if err := w.DB.Where("id = ?", tc.id).Take(&untouched).Error; err != nil {
			t.Fatalf("回读投递失败：%v", err)
		}
		if untouched.Status == NotifyDeliveryStatusRetry {
			t.Fatalf("%s 的投递被错误地改成了 retry", tc.name)
		}
	}

	if err := w.ChannelDeliveryRetry(foreign.Id); err == nil {
		t.Fatal("异站点投递必须按「不存在」拒绝")
	}
	if err := w.ChannelDeliveryRetry(0); err == nil {
		t.Fatal("缺少投递ID必须被拒绝")
	}

	var audits []model.ChannelAudit
	if err := w.DB.Where("site_id = ? AND action = ?", w.Id, "delivery.retry").Order("id desc").Limit(5).Find(&audits).Error; err != nil {
		t.Fatalf("读取审计失败：%v", err)
	}
	if len(audits) == 0 {
		t.Fatal("人工重试必须写审计")
	}
	if audits[0].Result != "ok" || audits[0].ChannelId != ch.Id {
		t.Fatalf("重试审计内容不对：%+v", audits[0])
	}
}

// ============================================================
// R94 试发
// ============================================================

// TestChannelTestSendUsesApprovedTarget 锁住「只向已批准目标发示例消息」且不碰真实网络。
func TestChannelTestSendUsesApprovedTarget(t *testing.T) {
	w := channelDataTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	fake := newNotifyFakeAdapter()
	original := channelTestAdapterFactory
	channelTestAdapterFactory = func(string, PlatformCredentials) (PlatformAdapter, bool) {
		return fake, true
	}
	t.Cleanup(func() { channelTestAdapterFactory = original })

	// 1) 无目标：只校验配置，不发消息，且必须如实说自己没验证过（T12b R1）。
	result, err := w.ChannelTestSend(ch.Id, 0)
	if err != nil {
		t.Fatalf("配置校验失败：%v", err)
	}
	if result.Valid || result.Verified || result.Sent {
		t.Fatalf("无目标只做配置校验，不得声称有效或已验证：%+v", result)
	}
	if strings.TrimSpace(result.Message) == "" {
		t.Fatal("未验证的回执必须说明原因")
	}
	if fake.callCount() != 0 {
		t.Fatal("无目标时不得发送任何消息")
	}

	// 2) 未批准目标：拒绝，且不调用适配器。
	unapproved := channelDataTargetRow(t, w, w.Id, ch.Id, TargetKindGroup, "test_unapproved_"+channelDataSuffix(), 0, 0)
	if _, err := w.ChannelTestSend(ch.Id, unapproved.Id); err == nil {
		t.Fatal("未批准目标不得试发")
	}
	if fake.callCount() != 0 {
		t.Fatal("未批准目标不得调用平台适配器")
	}

	// 3) 已批准目标：发送固定文本，目标引用必须一致。
	platformTargetId := "test_grp_" + channelDataSuffix()
	approved := channelDataTargetRow(t, w, w.Id, ch.Id, TargetKindGroup, platformTargetId, 1, 0)
	result, err = w.ChannelTestSend(ch.Id, approved.Id)
	if err != nil {
		t.Fatalf("试发失败：%v", err)
	}
	if !result.Valid || !result.Sent {
		t.Fatalf("试发结果不对：%+v", result)
	}
	if fake.callCount() != 1 {
		t.Fatalf("已批准目标必须发送一次，实际 %d", fake.callCount())
	}
	call := fake.calls[0]
	if call.target.PlatformTargetId != platformTargetId || call.target.Kind != TargetKindGroup {
		t.Fatalf("试发目标不对：%+v", call.target)
	}
	if call.text != channelTestMessageText {
		t.Fatalf("试发文本必须是固定示例文本，实际 %q", call.text)
	}

	// 4) 异站点渠道：按不存在处理。
	foreign := newChannelDataForeignChannel(t, w)
	if _, err := w.ChannelTestSend(foreign.Id, 0); !errors.Is(err, errChannelAdminNotFound) {
		t.Fatalf("异站点渠道试发必须按「渠道不存在」拒绝，实得 %v", err)
	}

	// 5) 平台返回失败：回执里带分类与脱敏文案，并落 failed 审计。
	errorFake := newNotifyFakeAdapter(notifyFakeResponse{
		err: NewSendError(ErrorClassPermanent, "目标不可达\n第二行"),
	})
	channelTestAdapterFactory = func(string, PlatformCredentials) (PlatformAdapter, bool) {
		return errorFake, true
	}
	result, err = w.ChannelTestSend(ch.Id, approved.Id)
	if err != nil {
		t.Fatalf("发送失败不得变成接口失败：%v", err)
	}
	if result.Sent || result.Class != string(ErrorClassPermanent) {
		t.Fatalf("失败回执不对：%+v", result)
	}
	if strings.Contains(result.Message, "\n") {
		t.Fatalf("平台文案必须压成单行：%q", result.Message)
	}

	// 6) 配置不全的渠道：无目标校验也必须失败。
	if err := w.DB.Model(&model.MessageChannel{}).Where("id = ?", ch.Id).Update("app_secret", "").Error; err != nil {
		t.Fatalf("清空密钥失败：%v", err)
	}
	if _, err := w.ChannelTestSend(ch.Id, 0); err == nil {
		t.Fatal("缺少应用密钥的渠道不能通过配置校验")
	}

	var audits []model.ChannelAudit
	if err := w.DB.Where("site_id = ? AND action = ? AND channel_id = ?", w.Id, "channel.test", ch.Id).Order("id desc").Find(&audits).Error; err != nil {
		t.Fatalf("读取审计失败：%v", err)
	}
	if len(audits) < 2 {
		t.Fatalf("试发必须写 ok 与 failed 两条审计，实际 %d", len(audits))
	}
	results := map[string]bool{}
	for _, item := range audits {
		results[item.Result] = true
	}
	if !results["ok"] || !results["failed"] {
		t.Fatalf("试发审计结果不对：%+v", results)
	}
}

// TestChannelTestSendTreatsOrgIdAsOptional 锁住「组织标识不再是配置门槛」：
// 它由建连前的平台接口自动获取、取不到则由首条入站事件锚定，因此配置校验只在
// 应用 ID / 密钥缺失时拒绝。负向对照：清空 app_id 或 app_secret 必须变红，
// 证明校验仍在真正检查凭据，而不是整段空转。
func TestChannelTestSendTreatsOrgIdAsOptional(t *testing.T) {
	w := channelDataTestSite(t)
	ch := newManagerTestChannel(t, w, true)

	fake := newNotifyFakeAdapter()
	original := channelTestAdapterFactory
	channelTestAdapterFactory = func(string, PlatformCredentials) (PlatformAdapter, bool) {
		return fake, true
	}
	t.Cleanup(func() { channelTestAdapterFactory = original })

	// 正向对照：凭据完整时配置校验通过，但不得声称已验证（没有连平台）。
	result, err := w.ChannelTestSend(ch.Id, 0)
	if err != nil {
		t.Fatalf("凭据完整的渠道应通过配置校验：%v", err)
	}
	if result.Valid || result.Verified || result.Sent {
		t.Fatalf("无目标校验不得声称有效/已验证/已发送：%+v", result)
	}

	// 只清组织标识：不再拒绝——自动获取/事件锚定会补上。
	if err := w.DB.Model(&model.MessageChannel{}).Where("id = ?", ch.Id).Update("org_id", "").Error; err != nil {
		t.Fatalf("清空组织标识失败：%v", err)
	}
	if _, err := w.ChannelTestSend(ch.Id, 0); err != nil {
		t.Fatalf("组织标识留空必须能通过配置校验（由自动获取/事件锚定补齐），实得：%v", err)
	}

	// 纯空白同样按「留空」处理（与建连校验的 TrimSpace 语义一致）。
	if err := w.DB.Model(&model.MessageChannel{}).Where("id = ?", ch.Id).Update("org_id", "   ").Error; err != nil {
		t.Fatalf("写入空白组织标识失败：%v", err)
	}
	if _, err := w.ChannelTestSend(ch.Id, 0); err != nil {
		t.Fatalf("空白组织标识必须等同留空而不是报错，实得：%v", err)
	}

	// 反向对照：真正的凭据缺失仍必须拒绝。
	if err := w.DB.Model(&model.MessageChannel{}).Where("id = ?", ch.Id).Update("app_id", "").Error; err != nil {
		t.Fatalf("清空应用 ID 失败：%v", err)
	}
	if _, err := w.ChannelTestSend(ch.Id, 0); err == nil {
		t.Fatal("缺少应用 ID 的渠道不能通过配置校验")
	}
	if err := w.DB.Model(&model.MessageChannel{}).Where("id = ?", ch.Id).Updates(map[string]any{
		"app_id": ch.AppId, "app_secret": "",
	}).Error; err != nil {
		t.Fatalf("清空应用密钥失败：%v", err)
	}
	if _, err := w.ChannelTestSend(ch.Id, 0); err == nil {
		t.Fatal("缺少应用密钥的渠道不能通过配置校验")
	}
	if fake.callCount() != 0 {
		t.Fatal("配置校验不得触发平台发送")
	}
}

// ============================================================
// F0 密钥安全日志
// ============================================================

// TestChannelWriteUsesParameterizedLogger 锁住两件事：
//  1. 运行时：携带 app_secret 的写操作必须用参数化日志（SQL 里不展开参数值）；
//  2. 源码：写入统一走 SaveMessageChannel，且里面有 Session(...Logger: channelSecretSafeLogger)。
//
// 负向对照：把 SaveMessageChannel 里那一行删掉后重新解析，同一匹配器必须返回 false，
// 否则说明断言本身没抓住任何东西。
func TestChannelWriteUsesParameterizedLogger(t *testing.T) {
	if !channelSecretSafeLoggerConfig.ParameterizedQueries {
		t.Fatal("channelSecretSafeLoggerConfig 必须开启 ParameterizedQueries")
	}
	value := reflect.ValueOf(channelSecretSafeLogger)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		t.Fatalf("channelSecretSafeLogger 必须是可读的指针类型，实际 %T", channelSecretSafeLogger)
	}
	configField := value.Elem().FieldByName("Config")
	if !configField.IsValid() || !configField.CanInterface() {
		t.Fatal("无法读取 channelSecretSafeLogger 的 Config 字段（gorm logger 结构变更？）")
	}
	cfg, ok := configField.Interface().(logger.Config)
	if !ok {
		t.Fatalf("channelSecretSafeLogger.Config 不是 logger.Config：%T", configField.Interface())
	}
	if !cfg.ParameterizedQueries {
		t.Fatal("channelSecretSafeLogger 本身必须带 ParameterizedQueries=true（不能只改配置变量）")
	}

	managePath, err := FindManageRoute()
	if err != nil {
		t.Fatalf("定位 route/manage.go 失败：%v", err)
	}
	path := filepath.Join(filepath.Dir(filepath.Dir(managePath)), "provider", "channelAdmin.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", path, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, raw, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", path, err)
	}

	save := channelFindFunc(t, file, "SaveMessageChannel")
	if !channelUsesSecretSafeSession(fset, save) {
		t.Fatal("SaveMessageChannel 必须用 Session(&gorm.Session{Logger: channelSecretSafeLogger}) 写入")
	}

	// 负向对照：删掉那一行后同一匹配器必须报 false。
	var printed strings.Builder
	if err := printer.Fprint(&printed, fset, save); err != nil {
		t.Fatalf("打印函数失败：%v", err)
	}
	stripped := channelStripLine(printed.String(), "channelSecretSafeLogger")
	if stripped == printed.String() {
		t.Fatal("负向对照构造失败：没找到要删除的行")
	}
	snippet := "package p\n\n" + stripped + "\n"
	snippetFile, err := parser.ParseFile(token.NewFileSet(), "snippet.go", snippet, 0)
	if err != nil {
		t.Fatalf("解析负向对照片段失败：%v\n%s", err, snippet)
	}
	if channelUsesSecretSafeSession(token.NewFileSet(), channelFindFunc(t, snippetFile, "SaveMessageChannel")) {
		t.Fatal("负向对照失败：删掉 Session(...) 后匹配器仍然命中，说明断言是空转的")
	}

	// 新建与更新都必须走同一入口，并先做应用身份预检查。
	for _, name := range []string{"createAdminChannel", "updateAdminChannel"} {
		body := channelFindFunc(t, file, name)
		if !channelBodyCalls(body, "SaveMessageChannel") {
			t.Errorf("%s 必须走 w.SaveMessageChannel 写入（否则日志会展开 app_secret）", name)
		}
		if !channelBodyCalls(body, "channelAppIdentityTaken") {
			t.Errorf("%s 必须先预检查 app_identity_hash，普通冲突不该触发数据库错误", name)
		}
	}
}

func channelFindFunc(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name && fn.Body != nil {
			return fn
		}
	}
	t.Fatalf("源码里找不到函数 %s", name)
	return nil
}

// channelUsesSecretSafeSession 匹配 Session(&gorm.Session{Logger: channelSecretSafeLogger})。
// 地址运算符要单独拆：&gorm.Session{...} 的 AST 是 UnaryExpr(&) 包着 CompositeLit。
func channelUsesSecretSafeSession(fset *token.FileSet, fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Session" || len(call.Args) != 1 {
			return true
		}
		lit := channelCompositeLit(call.Args[0])
		if lit == nil || channelExprString(fset, lit.Type) != "gorm.Session" {
			return true
		}
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Logger" {
				continue
			}
			value, ok := kv.Value.(*ast.Ident)
			if ok && value.Name == "channelSecretSafeLogger" {
				found = true
			}
		}
		return true
	})
	return found
}

// channelCompositeLit 拆掉取地址运算符后返回复合字面量。
func channelCompositeLit(expr ast.Expr) *ast.CompositeLit {
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}
	lit, _ := expr.(*ast.CompositeLit)
	return lit
}

func channelBodyCalls(fn *ast.FuncDecl, name string) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if fun.Name == name {
				found = true
			}
		case *ast.SelectorExpr:
			if fun.Sel.Name == name {
				found = true
			}
		}
		return true
	})
	return found
}

func channelStripLine(text, needle string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, needle) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
