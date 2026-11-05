package provider

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// channelAdminData.go 是后台「渠道数据面」的业务层（T9b）：
// 绑定、通知目标、通知规则、通知模板、运行记录、试发与投递重试。
//
// 与 channelAdmin.go 同一分层约定（R81）：控制器只取参和输出，校验/隔离/投影都在这里。
// 所有查询强制 site_id；跨站一律按「不存在」处理。所有对外文案静态化，
// 不含密钥、token、摘要或驱动原文。

// ErrChannelForbidden 表示当前管理员没有执行该操作的权限（需要超级管理员）。
// 控制器把它映射为 1002，与后台其它「权限不足」响应一致。
var ErrChannelForbidden = errors.New("权限不足。Permission denied.")

// 绑定/目标/规则/模板/记录的静态失败文案。
var (
	errChannelBindingNotFound       = errors.New("绑定不存在")
	errChannelPrivateTargetManual   = errors.New("私聊目标只能由绑定流程创建")
	errChannelTargetKindInvalid     = errors.New("目标类型只支持群聊")
	errChannelTargetPlatformMissing = errors.New("平台目标标识不能为空")
	errChannelTargetFlagInvalid     = errors.New("批准状态与管理开关只能是 0 或 1")
	errChannelTargetConflict        = errors.New("该目标已存在")
	errChannelTargetNotFound        = errors.New("目标不存在")
	errChannelTargetNotApproved     = errors.New("目标未批准，不能发送消息")
	errChannelEventTypeInvalid      = errors.New("事件类型不支持")
	errChannelRuleEnabledInvalid    = errors.New("启用状态只能是 0 或 1")
	errChannelRuleTargetsRequired   = errors.New("接收目标不能为空")
	errChannelRuleTargetInvalid     = errors.New("接收目标必须属于本站点")
	errChannelRuleTemplateInvalid   = errors.New("通知模板必须属于本站点")
	errChannelRuleNotFound          = errors.New("通知规则不存在")
	errChannelTemplateKeyRequired   = errors.New("模板键不能为空")
	errChannelTemplateKeyConflict   = errors.New("模板键已存在")
	errChannelTemplateNotFound      = errors.New("通知模板不存在")
	errChannelRecordTypeInvalid     = errors.New("记录类型不支持")
	errChannelRecordsFailed         = errors.New("读取记录失败")
	errChannelDeliveryNotFound      = errors.New("投递不存在")
	errChannelDeliveryNotRetryable  = errors.New("仅失败或结果未知的投递可重试")
	errChannelDeliveryRetryFailed   = errors.New("重试投递失败")
	errChannelTestConfigInvalid     = errors.New("渠道配置不完整")
	errChannelTestSendFailed        = errors.New("试发失败")
	errChannelAuditFailed           = errors.New("写入审计失败")
	errChannelAdminInvalid          = errors.New("管理员不存在或已被禁用")
	errChannelTargetReadFailed      = errors.New("读取目标失败")
)

// channelTestMessageText 是试发的固定示例文本（R94）。不含站点数据，也不可配置。
const channelTestMessageText = "AnQiCMS 测试消息"

// channelTestSendTimeout 是单次试发的超时：不能让后台请求被平台网络拖住。
const channelTestSendTimeout = 15 * time.Second

// channelTestAdapterFactory 是试发时构造平台适配器的入口。
// 测试注入假适配器，生产使用 NewPlatformAdapter；绝不走真实网络之外的第二条路径。
var channelTestAdapterFactory = NewPlatformAdapter

// ChannelBindingView 是绑定的白名单投影：不含 identity_hash、token 摘要。
type ChannelBindingView struct {
	Id             uint   `json:"id"`
	ChannelId      uint   `json:"channel_id"`
	OrgId          string `json:"org_id"`
	PlatformUserId string `json:"platform_user_id"`
	AdminId        uint   `json:"admin_id"`
	RevokedAt      int64  `json:"revoked_at"`
	CreatedTime    int64  `json:"created_time"`
	UpdatedTime    int64  `json:"updated_time"`
}

// ChannelTargetView 是通知目标的白名单投影：不含 target_hash。
type ChannelTargetView struct {
	Id                uint   `json:"id"`
	ChannelId         uint   `json:"channel_id"`
	Kind              string `json:"kind"`
	PlatformTargetId  string `json:"platform_target_id"`
	BindingId         uint   `json:"binding_id"`
	Approved          int    `json:"approved"`
	ManagementEnabled int    `json:"management_enabled"`
	CreatedTime       int64  `json:"created_time"`
	UpdatedTime       int64  `json:"updated_time"`
}

// ChannelRuleView 是通知规则视图。TargetIds 在这里还原成数组，
// 仓储里仍是 JSON 文本（与 T8 扇出读取的口径一致）。
type ChannelRuleView struct {
	Id          uint   `json:"id"`
	Name        string `json:"name"`
	EventType   string `json:"event_type"`
	Enabled     int    `json:"enabled"`
	TargetIds   []uint `json:"target_ids"`
	TemplateId  uint   `json:"template_id"`
	CreatedTime int64  `json:"created_time"`
	UpdatedTime int64  `json:"updated_time"`
}

// ChannelTemplateView 是通知模板视图。
type ChannelTemplateView struct {
	Id                    uint   `json:"id"`
	Key                   string `json:"key"`
	Name                  string `json:"name"`
	Subject               string `json:"subject"`
	Content               string `json:"content"`
	PlatformOverridesJson string `json:"platform_overrides_json"`
	CreatedTime           int64  `json:"created_time"`
	UpdatedTime           int64  `json:"updated_time"`
}

// ChannelTemplatePreviewView 是预览结果：不落库，也不回显载荷原文。
type ChannelTemplatePreviewView struct {
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

// ChannelDeliveryRecord 是投递记录的白名单投影（不含 subject/content/delivery_hash）。
type ChannelDeliveryRecord struct {
	Id                uint   `json:"id"`
	EventId           uint   `json:"event_id"`
	RuleId            uint   `json:"rule_id"`
	TargetId          uint   `json:"target_id"`
	Status            string `json:"status"`
	Attempts          int    `json:"attempts"`
	NextAttemptAt     int64  `json:"next_attempt_at"`
	LeaseUntil        int64  `json:"lease_until"`
	LastError         string `json:"last_error"`
	PlatformMessageId string `json:"platform_message_id"`
	CreatedTime       int64  `json:"created_time"`
}

// ChannelInboxRecord 是入站任务的白名单投影（不含 content/event_hash）。
type ChannelInboxRecord struct {
	Id          uint   `json:"id"`
	ChannelId   uint   `json:"channel_id"`
	BindingId   uint   `json:"binding_id"`
	EventKind   string `json:"event_kind"`
	Status      string `json:"status"`
	LeaseUntil  int64  `json:"lease_until"`
	CreatedTime int64  `json:"created_time"`
}

// ChannelApprovalRecord 是审批记录的白名单投影
// （不含 canonical_args/args_hash/public_id/result_summary）。
type ChannelApprovalRecord struct {
	Id             uint   `json:"id"`
	BindingId      uint   `json:"binding_id"`
	SessionId      uint   `json:"session_id"`
	Tool           string `json:"tool"`
	Action         string `json:"action"`
	Status         string `json:"status"`
	ExpiresAt      int64  `json:"expires_at"`
	PolicyRevision int    `json:"policy_revision"`
	CreatedTime    int64  `json:"created_time"`
}

// ChannelAuditRecord 是审计记录的白名单投影（不含 target_hash）。
type ChannelAuditRecord struct {
	Id          uint   `json:"id"`
	CreatedTime int64  `json:"created_time"`
	ChannelId   uint   `json:"channel_id"`
	BindingId   uint   `json:"binding_id"`
	TaskId      string `json:"task_id"`
	Action      string `json:"action"`
	Result      string `json:"result"`
}

// ChannelRecordsPage 是分页结果。List 的元素类型由 type 决定。
type ChannelRecordsPage struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// ChannelTestSendResult 是试发回执（R94）。
type ChannelTestSendResult struct {
	Valid   bool   `json:"valid"`
	Sent    bool   `json:"sent"`
	Class   string `json:"class,omitempty"`
	Message string `json:"message,omitempty"`
}

// ChannelBindings 返回当前站点的绑定。
//
// all=false 只返回当前管理员自己的绑定；all=true 需要超级管理员
// （adminId==1 或 GroupId==1），否则 1002（R89）。
func (w *Website) ChannelBindings(adminId uint, all bool) ([]ChannelBindingView, error) {
	if w == nil || w.DB == nil || adminId == 0 {
		return nil, ErrChannelForbidden
	}
	if all {
		super, err := w.channelAdminIsSuper(adminId)
		if err != nil {
			return nil, err
		}
		if !super {
			return nil, ErrChannelForbidden
		}
	}
	query := w.DB.Where("`site_id` = ?", w.Id)
	if !all {
		query = query.Where("`admin_id` = ?", adminId)
	}
	var bindings []model.ChannelBinding
	if err := query.Order("id desc").Find(&bindings).Error; err != nil {
		log.Printf("[channel] 站点 %d 读取绑定列表失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return nil, errors.New("读取绑定失败")
	}
	views := make([]ChannelBindingView, 0, len(bindings))
	for i := range bindings {
		views = append(views, channelBindingViewOf(&bindings[i]))
	}
	return views, nil
}

// ChannelBindingRevoke 解除绑定：本人可解自己的，他人绑定需要超级管理员（R89）。
func (w *Website) ChannelBindingRevoke(bindingId, operatorAdminId uint) error {
	if w == nil || w.DB == nil || bindingId == 0 || operatorAdminId == 0 {
		return errChannelBindingNotFound
	}
	var binding model.ChannelBinding
	if err := w.DB.Where("`id` = ? AND `site_id` = ?", bindingId, w.Id).Take(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errChannelBindingNotFound
		}
		log.Printf("[channel] 站点 %d 读取绑定 %d 失败：%s", w.Id, bindingId, sanitizePlatformMessage(err.Error()))
		return errors.New("读取绑定失败")
	}
	if binding.AdminId != operatorAdminId {
		super, err := w.channelAdminIsSuper(operatorAdminId)
		if err != nil {
			return err
		}
		if !super {
			return ErrChannelForbidden
		}
	}
	return w.RevokeBinding(binding.Id, operatorAdminId)
}

// ChannelTargets 返回某渠道在本站点的目标列表。
func (w *Website) ChannelTargets(channelId uint) ([]ChannelTargetView, error) {
	channel, err := w.channelForAdmin(channelId)
	if err != nil {
		return nil, err
	}
	var targets []model.ChannelTarget
	if err := w.DB.Where("`site_id` = ? AND `channel_id` = ?", w.Id, channel.Id).
		Order("id desc").Find(&targets).Error; err != nil {
		log.Printf("[channel] 站点 %d 读取渠道 %d 的目标失败：%s", w.Id, channel.Id, sanitizePlatformMessage(err.Error()))
		return nil, errChannelTargetReadFailed
	}
	views := make([]ChannelTargetView, 0, len(targets))
	for i := range targets {
		views = append(views, channelTargetViewOf(&targets[i]))
	}
	return views, nil
}

// ChannelTargetSave 新建/更新群目标（R90）。
//
// 私聊目标只能由绑定流程创建：手工新建、或把已有目标改成 private 一律拒绝。
// 目标属于本站点渠道；批准与群管理开关在同一个记录上。
func (w *Website) ChannelTargetSave(req *request.ChannelTargetRequest) (*ChannelTargetView, error) {
	if w == nil || w.DB == nil || req == nil || req.ChannelId == 0 {
		return nil, errChannelAdminNotFound
	}
	channel, err := w.channelForAdmin(req.ChannelId)
	if err != nil {
		return nil, err
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == channelTargetKindPrivate {
		return nil, errChannelPrivateTargetManual
	}
	if kind != TargetKindGroup {
		return nil, errChannelTargetKindInvalid
	}
	platformTargetId := strings.TrimSpace(req.PlatformTargetId)
	if platformTargetId == "" {
		return nil, errChannelTargetPlatformMissing
	}
	if !channelAdminFlagValid(req.Approved) || !channelAdminFlagValid(req.ManagementEnabled) {
		return nil, errChannelTargetFlagInvalid
	}
	targetHash := bindingTargetDigest(w.Id, channel.Id, kind, platformTargetId)

	if req.Id == 0 {
		taken, err := w.channelTargetHashTaken(0, targetHash)
		if err != nil {
			log.Printf("[channel] 站点 %d 检查目标重复失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
			return nil, errChannelTargetReadFailed
		}
		if taken {
			return nil, errChannelTargetConflict
		}
		target := &model.ChannelTarget{
			SiteId:            w.Id,
			ChannelId:         channel.Id,
			Kind:              kind,
			PlatformTargetId:  platformTargetId,
			Approved:          req.Approved,
			ManagementEnabled: req.ManagementEnabled,
			TargetHash:        targetHash,
		}
		if err := w.DB.Create(target).Error; err != nil {
			if isDuplicateKeyError(err) {
				return nil, errChannelTargetConflict
			}
			log.Printf("[channel] 站点 %d 新建目标失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
			return nil, errChannelTargetReadFailed
		}
		view := channelTargetViewOf(target)
		return &view, nil
	}

	var existing model.ChannelTarget
	if err := w.DB.Where("`id` = ? AND `site_id` = ? AND `channel_id` = ?", req.Id, w.Id, channel.Id).
		Take(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errChannelTargetNotFound
		}
		log.Printf("[channel] 站点 %d 读取目标 %d 失败：%s", w.Id, req.Id, sanitizePlatformMessage(err.Error()))
		return nil, errChannelTargetReadFailed
	}
	if existing.Kind != TargetKindGroup {
		return nil, errChannelPrivateTargetManual
	}
	taken, err := w.channelTargetHashTaken(existing.Id, targetHash)
	if err != nil {
		log.Printf("[channel] 站点 %d 检查目标重复失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return nil, errChannelTargetReadFailed
	}
	if taken {
		return nil, errChannelTargetConflict
	}
	res := w.DB.Model(&model.ChannelTarget{}).
		Where("`id` = ? AND `site_id` = ? AND `channel_id` = ?", existing.Id, w.Id, channel.Id).
		Updates(map[string]any{
			"kind":               kind,
			"platform_target_id": platformTargetId,
			"target_hash":        targetHash,
			"approved":           req.Approved,
			"management_enabled": req.ManagementEnabled,
			"updated_time":       time.Now().Unix(),
		})
	if res.Error != nil {
		if isDuplicateKeyError(res.Error) {
			return nil, errChannelTargetConflict
		}
		log.Printf("[channel] 站点 %d 更新目标 %d 失败：%s", w.Id, existing.Id, sanitizePlatformMessage(res.Error.Error()))
		return nil, errChannelTargetReadFailed
	}
	existing.Kind = kind
	existing.PlatformTargetId = platformTargetId
	existing.TargetHash = targetHash
	existing.Approved = req.Approved
	existing.ManagementEnabled = req.ManagementEnabled
	view := channelTargetViewOf(&existing)
	return &view, nil
}

// channelTargetHashTaken 检查目标摘要是否已被别的行占用（排除自身）。
func (w *Website) channelTargetHashTaken(targetId uint, targetHash string) (bool, error) {
	query := w.DB.Model(&model.ChannelTarget{}).Where("`target_hash` = ?", targetHash)
	if targetId > 0 {
		query = query.Where("`id` <> ?", targetId)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ChannelRules 返回本站点的通知规则。
func (w *Website) ChannelRules() ([]ChannelRuleView, error) {
	if w == nil || w.DB == nil {
		return nil, errChannelReadFailed
	}
	var rules []model.NotificationRule
	if err := w.DB.Where("`site_id` = ?", w.Id).Order("id desc").Find(&rules).Error; err != nil {
		log.Printf("[channel] 站点 %d 读取通知规则失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return nil, errors.New("读取通知规则失败")
	}
	views := make([]ChannelRuleView, 0, len(rules))
	for i := range rules {
		views = append(views, channelRuleViewOf(&rules[i]))
	}
	return views, nil
}

// ChannelRuleSave 新建/更新通知规则（R91）。
func (w *Website) ChannelRuleSave(req *request.ChannelRuleRequest) (*ChannelRuleView, error) {
	if w == nil || w.DB == nil || req == nil {
		return nil, errors.New("保存通知规则失败")
	}
	eventType := strings.TrimSpace(req.EventType)
	switch eventType {
	case NotifyEventGuestbookCreated, NotifyEventOrderCreated, NotifyEventOrderPaid, NotifyEventAgentFailed:
	default:
		return nil, errChannelEventTypeInvalid
	}
	if !channelAdminFlagValid(req.Enabled) {
		return nil, errChannelRuleEnabledInvalid
	}
	targetIds, err := w.channelValidateRuleTargets(req.TargetIds)
	if err != nil {
		return nil, err
	}
	if req.TemplateId > 0 {
		var count int64
		if err := w.DB.Model(&model.NotificationTemplate{}).
			Where("`id` = ? AND `site_id` = ?", req.TemplateId, w.Id).Count(&count).Error; err != nil {
			log.Printf("[channel] 站点 %d 校验通知模板失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
			return nil, errors.New("保存通知规则失败")
		}
		if count == 0 {
			return nil, errChannelRuleTemplateInvalid
		}
	}
	targetJson, err := json.Marshal(targetIds)
	if err != nil {
		return nil, errors.New("保存通知规则失败")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = notificationEventLabel(eventType)
	}
	values := map[string]any{
		"name":        name,
		"event_type":  eventType,
		"enabled":     req.Enabled,
		"target_ids":  string(targetJson),
		"template_id": req.TemplateId,
	}
	if req.Id == 0 {
		rule := &model.NotificationRule{
			SiteId:     w.Id,
			Name:       name,
			EventType:  eventType,
			Enabled:    req.Enabled,
			TargetIds:  string(targetJson),
			TemplateId: req.TemplateId,
		}
		if err := w.DB.Create(rule).Error; err != nil {
			log.Printf("[channel] 站点 %d 新建通知规则失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
			return nil, errors.New("保存通知规则失败")
		}
		view := channelRuleViewOf(rule)
		return &view, nil
	}
	values["updated_time"] = time.Now().Unix()
	res := w.DB.Model(&model.NotificationRule{}).
		Where("`id` = ? AND `site_id` = ?", req.Id, w.Id).
		Updates(values)
	if res.Error != nil {
		log.Printf("[channel] 站点 %d 更新通知规则 %d 失败：%s", w.Id, req.Id, sanitizePlatformMessage(res.Error.Error()))
		return nil, errors.New("保存通知规则失败")
	}
	if res.RowsAffected == 0 {
		return nil, errChannelRuleNotFound
	}
	var rule model.NotificationRule
	if err := w.DB.Where("`id` = ? AND `site_id` = ?", req.Id, w.Id).Take(&rule).Error; err != nil {
		log.Printf("[channel] 站点 %d 回读通知规则 %d 失败：%s", w.Id, req.Id, sanitizePlatformMessage(err.Error()))
		return nil, errors.New("保存通知规则失败")
	}
	view := channelRuleViewOf(&rule)
	return &view, nil
}

// channelValidateRuleTargets 去重并校验目标全部属于本站点。
func (w *Website) channelValidateRuleTargets(targetIds []uint) ([]uint, error) {
	if len(targetIds) == 0 {
		return nil, errChannelRuleTargetsRequired
	}
	unique := make([]uint, 0, len(targetIds))
	seen := make(map[uint]bool, len(targetIds))
	for _, id := range targetIds {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return nil, errChannelRuleTargetsRequired
	}
	var count int64
	if err := w.DB.Model(&model.ChannelTarget{}).
		Where("`site_id` = ? AND `id` IN ?", w.Id, unique).Count(&count).Error; err != nil {
		log.Printf("[channel] 站点 %d 校验规则目标失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return nil, errChannelRuleTargetInvalid
	}
	if count != int64(len(unique)) {
		return nil, errChannelRuleTargetInvalid
	}
	return unique, nil
}

// ChannelRuleDelete 删除本站点的通知规则；不存在时返回静态文案。
func (w *Website) ChannelRuleDelete(ruleId uint) error {
	if w == nil || w.DB == nil || ruleId == 0 {
		return errChannelRuleNotFound
	}
	res := w.DB.Where("`id` = ? AND `site_id` = ?", ruleId, w.Id).Delete(&model.NotificationRule{})
	if res.Error != nil {
		log.Printf("[channel] 站点 %d 删除通知规则 %d 失败：%s", w.Id, ruleId, sanitizePlatformMessage(res.Error.Error()))
		return errors.New("删除通知规则失败")
	}
	if res.RowsAffected == 0 {
		return errChannelRuleNotFound
	}
	return nil
}

// ChannelTemplates 返回本站点的通知模板。
func (w *Website) ChannelTemplates() ([]ChannelTemplateView, error) {
	if w == nil || w.DB == nil {
		return nil, errChannelReadFailed
	}
	var templates []model.NotificationTemplate
	if err := w.DB.Where("`site_id` = ?", w.Id).Order("id desc").Find(&templates).Error; err != nil {
		log.Printf("[channel] 站点 %d 读取通知模板失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return nil, errors.New("读取通知模板失败")
	}
	views := make([]ChannelTemplateView, 0, len(templates))
	for i := range templates {
		views = append(views, channelTemplateViewOf(&templates[i]))
	}
	return views, nil
}

// ValidateNotificationTemplate 校验通知模板的标题与正文（R92）。
//
// 只做词法白名单：标签与过滤器必须在允许清单内。渲染期还有独立 TemplateSet
// 与 BanTag 兜底，两层都不可省略。
func ValidateNotificationTemplate(subject, content string) error {
	if len([]rune(subject)) > notifyTemplateSubjectMaxRunes {
		return errNotifyTemplateInvalid
	}
	if err := validateNotificationTemplateSyntax(subject); err != nil {
		return err
	}
	return validateNotificationTemplateSyntax(content)
}

// ChannelTemplateSave 新建/更新通知模板（R92）。(site_id,key) 唯一。
func (w *Website) ChannelTemplateSave(req *request.ChannelTemplateRequest) (*ChannelTemplateView, error) {
	if w == nil || w.DB == nil || req == nil {
		return nil, errors.New("保存通知模板失败")
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, errChannelTemplateKeyRequired
	}
	if err := ValidateNotificationTemplate(req.Subject, req.Content); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = key
	}
	query := w.DB.Model(&model.NotificationTemplate{}).Where("`site_id` = ? AND `key` = ?", w.Id, key)
	if req.Id > 0 {
		query = query.Where("`id` <> ?", req.Id)
	}
	var conflict int64
	if err := query.Count(&conflict).Error; err != nil {
		log.Printf("[channel] 站点 %d 检查模板键冲突失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return nil, errors.New("保存通知模板失败")
	}
	if conflict > 0 {
		return nil, errChannelTemplateKeyConflict
	}
	if req.Id == 0 {
		tpl := &model.NotificationTemplate{
			SiteId:                w.Id,
			Key:                   key,
			Name:                  name,
			Subject:               req.Subject,
			Content:               req.Content,
			PlatformOverridesJson: req.PlatformOverridesJson,
		}
		if err := w.DB.Create(tpl).Error; err != nil {
			if isDuplicateKeyError(err) {
				return nil, errChannelTemplateKeyConflict
			}
			log.Printf("[channel] 站点 %d 新建通知模板失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
			return nil, errors.New("保存通知模板失败")
		}
		view := channelTemplateViewOf(tpl)
		return &view, nil
	}
	res := w.DB.Model(&model.NotificationTemplate{}).
		Where("`id` = ? AND `site_id` = ?", req.Id, w.Id).
		Updates(map[string]any{
			"key":                     key,
			"name":                    name,
			"subject":                 req.Subject,
			"content":                 req.Content,
			"platform_overrides_json": req.PlatformOverridesJson,
			"updated_time":            time.Now().Unix(),
		})
	if res.Error != nil {
		if isDuplicateKeyError(res.Error) {
			return nil, errChannelTemplateKeyConflict
		}
		log.Printf("[channel] 站点 %d 更新通知模板 %d 失败：%s", w.Id, req.Id, sanitizePlatformMessage(res.Error.Error()))
		return nil, errors.New("保存通知模板失败")
	}
	if res.RowsAffected == 0 {
		return nil, errChannelTemplateNotFound
	}
	var tpl model.NotificationTemplate
	if err := w.DB.Where("`id` = ? AND `site_id` = ?", req.Id, w.Id).Take(&tpl).Error; err != nil {
		log.Printf("[channel] 站点 %d 回读通知模板 %d 失败：%s", w.Id, req.Id, sanitizePlatformMessage(err.Error()))
		return nil, errors.New("保存通知模板失败")
	}
	view := channelTemplateViewOf(&tpl)
	return &view, nil
}

// ChannelTemplatePreview 按目标安全级别渲染模板（R92）：不落库、不记录正文。
func (w *Website) ChannelTemplatePreview(req *request.ChannelTemplatePreviewRequest) (*ChannelTemplatePreviewView, error) {
	if w == nil || w.DB == nil || req == nil {
		return nil, errChannelTemplateNotFound
	}
	targetKind := strings.TrimSpace(req.TargetKind)
	if targetKind != TargetKindGroup && targetKind != TargetKindPrivate {
		return nil, errChannelTargetKindInvalid
	}
	var tpl *model.NotificationTemplate
	if req.Id > 0 {
		var saved model.NotificationTemplate
		if err := w.DB.Where("`id` = ? AND `site_id` = ?", req.Id, w.Id).Take(&saved).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errChannelTemplateNotFound
			}
			log.Printf("[channel] 站点 %d 读取通知模板 %d 失败：%s", w.Id, req.Id, sanitizePlatformMessage(err.Error()))
			return nil, errors.New("读取通知模板失败")
		}
		tpl = &saved
	} else {
		if err := ValidateNotificationTemplate(req.Subject, req.Content); err != nil {
			return nil, err
		}
		tpl = &model.NotificationTemplate{Subject: req.Subject, Content: req.Content}
	}
	siteName, siteURL := "", ""
	if w.System != nil {
		siteName = w.System.SiteName
		siteURL = w.System.BaseUrl
	}
	subject, text, err := RenderNotificationText(tpl, "", "", req.PayloadJson, targetKind, siteName, siteURL)
	if err != nil {
		return nil, err
	}
	return &ChannelTemplatePreviewView{Subject: subject, Text: text}, nil
}

// ChannelRecords 分页返回运行记录（R93）。
//
// 投影是白名单：每个字段单独赋值，禁止直接序列化模型。模型里带着
// content/canonical_args/result_summary/public_id/各种摘要，一旦直接返回，
// 就等于把这些数据整体放到了后台页面上。
func (w *Website) ChannelRecords(recordType, status string, page, pageSize int) (*ChannelRecordsPage, error) {
	if w == nil || w.DB == nil {
		return nil, errChannelRecordsFailed
	}
	recordType = strings.TrimSpace(recordType)
	status = strings.TrimSpace(status)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	result := &ChannelRecordsPage{Page: page, PageSize: pageSize}

	switch recordType {
	case "delivery":
		query := w.DB.Model(&model.NotificationDelivery{}).Where("`site_id` = ?", w.Id)
		if status != "" {
			query = query.Where("`status` = ?", status)
		}
		if err := query.Count(&result.Total).Error; err != nil {
			return nil, errChannelRecordsFailed
		}
		var rows []model.NotificationDelivery
		if err := query.Order("created_time desc, id desc").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
			log.Printf("[channel] 站点 %d 读取投递记录失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
			return nil, errChannelRecordsFailed
		}
		list := make([]ChannelDeliveryRecord, 0, len(rows))
		for _, row := range rows {
			list = append(list, ChannelDeliveryRecord{
				Id:                row.Id,
				EventId:           row.EventId,
				RuleId:            row.RuleId,
				TargetId:          row.TargetId,
				Status:            row.Status,
				Attempts:          row.Attempts,
				NextAttemptAt:     row.NextAttemptAt,
				LeaseUntil:        row.LeaseUntil,
				LastError:         row.LastError,
				PlatformMessageId: row.PlatformMessageId,
				CreatedTime:       row.CreatedTime,
			})
		}
		result.List = list
	case "inbox":
		query := w.DB.Model(&model.ChannelInbox{}).Where("`site_id` = ?", w.Id)
		if status != "" {
			query = query.Where("`status` = ?", status)
		}
		if err := query.Count(&result.Total).Error; err != nil {
			return nil, errChannelRecordsFailed
		}
		var rows []model.ChannelInbox
		if err := query.Order("created_time desc, id desc").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
			log.Printf("[channel] 站点 %d 读取入站任务失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
			return nil, errChannelRecordsFailed
		}
		list := make([]ChannelInboxRecord, 0, len(rows))
		for _, row := range rows {
			list = append(list, ChannelInboxRecord{
				Id:          row.Id,
				ChannelId:   row.ChannelId,
				BindingId:   row.BindingId,
				EventKind:   row.EventKind,
				Status:      row.Status,
				LeaseUntil:  row.LeaseUntil,
				CreatedTime: row.CreatedTime,
			})
		}
		result.List = list
	case "approval":
		query := w.DB.Model(&model.ChannelApproval{}).Where("`site_id` = ?", w.Id)
		if status != "" {
			query = query.Where("`status` = ?", status)
		}
		if err := query.Count(&result.Total).Error; err != nil {
			return nil, errChannelRecordsFailed
		}
		var rows []model.ChannelApproval
		if err := query.Order("created_time desc, id desc").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
			log.Printf("[channel] 站点 %d 读取审批记录失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
			return nil, errChannelRecordsFailed
		}
		list := make([]ChannelApprovalRecord, 0, len(rows))
		for _, row := range rows {
			list = append(list, ChannelApprovalRecord{
				Id:             row.Id,
				BindingId:      row.BindingId,
				SessionId:      row.SessionId,
				Tool:           row.Tool,
				Action:         row.Action,
				Status:         row.Status,
				ExpiresAt:      row.ExpiresAt,
				PolicyRevision: row.PolicyRevision,
				CreatedTime:    row.CreatedTime,
			})
		}
		result.List = list
	case "audit":
		query := w.DB.Model(&model.ChannelAudit{}).Where("`site_id` = ?", w.Id)
		if status != "" {
			query = query.Where("`result` = ?", status)
		}
		if err := query.Count(&result.Total).Error; err != nil {
			return nil, errChannelRecordsFailed
		}
		var rows []model.ChannelAudit
		if err := query.Order("created_time desc, id desc").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
			log.Printf("[channel] 站点 %d 读取审计记录失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
			return nil, errChannelRecordsFailed
		}
		list := make([]ChannelAuditRecord, 0, len(rows))
		for _, row := range rows {
			list = append(list, ChannelAuditRecord{
				Id:          row.Id,
				CreatedTime: row.CreatedTime,
				ChannelId:   row.ChannelId,
				BindingId:   row.BindingId,
				TaskId:      row.TaskId,
				Action:      row.Action,
				Result:      row.Result,
			})
		}
		result.List = list
	default:
		return nil, errChannelRecordTypeInvalid
	}
	return result, nil
}

// ChannelDeliveryRetry 对失败/结果未知的投递做人工重试（R95）。
func (w *Website) ChannelDeliveryRetry(deliveryId uint) error {
	if w == nil || w.DB == nil || deliveryId == 0 {
		return errChannelDeliveryNotFound
	}
	var delivery model.NotificationDelivery
	if err := w.DB.Where("`id` = ? AND `site_id` = ?", deliveryId, w.Id).Take(&delivery).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errChannelDeliveryNotFound
		}
		log.Printf("[channel] 站点 %d 读取投递 %d 失败：%s", w.Id, deliveryId, sanitizePlatformMessage(err.Error()))
		return errors.New("读取投递失败")
	}
	if delivery.Status != NotifyDeliveryStatusFailed && delivery.Status != NotifyDeliveryStatusUnknown {
		return errChannelDeliveryNotRetryable
	}
	now := time.Now().Unix()
	res := w.DB.Model(&model.NotificationDelivery{}).
		Where("`id` = ? AND `site_id` = ? AND `status` IN ?", delivery.Id, w.Id,
			[]string{NotifyDeliveryStatusFailed, NotifyDeliveryStatusUnknown}).
		Updates(map[string]any{
			"status":          NotifyDeliveryStatusRetry,
			"next_attempt_at": now,
			"lease_until":     0,
			"attempts":        0,
			"last_error":      "",
			"updated_time":    now,
		})
	if res.Error != nil {
		log.Printf("[channel] 站点 %d 重试投递 %d 失败：%s", w.Id, deliveryId, sanitizePlatformMessage(res.Error.Error()))
		return errChannelDeliveryRetryFailed
	}
	if res.RowsAffected == 0 {
		// 并发下可能已经被别人改成 retry/sent：仍是「不可重试」。
		return errChannelDeliveryNotRetryable
	}
	channelId := uint(0)
	var target model.ChannelTarget
	if err := w.DB.Where("`id` = ? AND `site_id` = ?", delivery.TargetId, w.Id).Take(&target).Error; err == nil {
		channelId = target.ChannelId
	}
	w.writeChannelAudit("delivery.retry", channelId, 0, "", "ok")
	return nil
}

// ChannelTestSend 验证渠道配置，或向已批准目标发一条固定示例消息（R94）。
func (w *Website) ChannelTestSend(channelId, targetId uint) (*ChannelTestSendResult, error) {
	channel, err := w.channelForAdmin(channelId)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(channel.Platform) == "" ||
		strings.TrimSpace(channel.AppId) == "" ||
		strings.TrimSpace(channel.AppSecret) == "" {
		return nil, errChannelTestConfigInvalid
	}
	creds := PlatformCredentials{
		AppId:          channel.AppId,
		AppSecret:      channel.AppSecret,
		OrganizationId: channel.OrgId,
	}
	adapter, ok := channelTestAdapterFactory(channel.Platform, creds)
	if !ok || adapter == nil {
		return nil, errChannelTestConfigInvalid
	}
	if targetId == 0 {
		return &ChannelTestSendResult{Valid: true}, nil
	}

	var target model.ChannelTarget
	if err := w.DB.Where("`id` = ? AND `site_id` = ? AND `channel_id` = ?", targetId, w.Id, channel.Id).
		Take(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errChannelTargetNotFound
		}
		log.Printf("[channel] 站点 %d 读取试发目标 %d 失败：%s", w.Id, targetId, sanitizePlatformMessage(err.Error()))
		return nil, errChannelTargetReadFailed
	}
	if target.Approved != 1 {
		return nil, errChannelTargetNotApproved
	}

	ctx, cancel := context.WithTimeout(context.Background(), channelTestSendTimeout)
	defer cancel()
	result, sendErr := adapter.Send(ctx, TargetRef{
		Kind:             target.Kind,
		PlatformTargetId: target.PlatformTargetId,
	}, OutboundMessage{Text: channelTestMessageText})

	targetHash := bindingTargetDigest(w.Id, channel.Id, target.Kind, target.PlatformTargetId)
	if sendErr != nil {
		// 发送失败不是接口失败：后台需要看到分类与脱敏后的平台文案。
		message := errChannelTestSendFailed.Error()
		class := ""
		if classified := ClassifyPlatformSendError(sendErr); classified != nil {
			class = string(classified.Class)
			message = sanitizePlatformMessage(classified.Message)
		}
		w.writeChannelAudit("channel.test", channel.Id, 0, targetHash, "failed")
		return &ChannelTestSendResult{Valid: true, Class: class, Message: message}, nil
	}
	_ = result
	w.writeChannelAudit("channel.test", channel.Id, 0, targetHash, "ok")
	return &ChannelTestSendResult{Valid: true, Sent: true}, nil
}

// channelAdminIsSuper 判断管理员是否具备跨管理员操作权限（adminId==1 或 GroupId==1）。
func (w *Website) channelAdminIsSuper(adminId uint) (bool, error) {
	if w == nil || w.DB == nil || adminId == 0 {
		return false, ErrChannelForbidden
	}
	var admin model.Admin
	if err := w.DB.Where("`id` = ?", adminId).Take(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrChannelForbidden
		}
		log.Printf("[channel] 站点 %d 读取管理员 %d 失败：%s", w.Id, adminId, sanitizePlatformMessage(err.Error()))
		return false, errChannelReadFailed
	}
	if admin.Status != 1 {
		return false, ErrChannelForbidden
	}
	return admin.Id == 1 || admin.GroupId == 1, nil
}

// writeChannelAudit 写一条渠道审计。审计失败不改变业务结果，只记日志。
func (w *Website) writeChannelAudit(action string, channelId, bindingId uint, targetHash, result string) {
	row := &model.ChannelAudit{
		SiteId:     w.Id,
		ChannelId:  channelId,
		BindingId:  bindingId,
		Action:     action,
		TargetHash: targetHash,
		Result:     result,
	}
	if err := w.DB.Create(row).Error; err != nil {
		log.Printf("[channel] 站点 %d 写入渠道审计失败（action=%s result=%s）：%s",
			w.Id, action, result, sanitizePlatformMessage(err.Error()))
	}
}

// channelAdminFlagValid 判断 0/1 开关值是否合法。
func channelAdminFlagValid(value int) bool {
	return value == 0 || value == 1
}

func channelBindingViewOf(binding *model.ChannelBinding) ChannelBindingView {
	return ChannelBindingView{
		Id:             binding.Id,
		ChannelId:      binding.ChannelId,
		OrgId:          binding.OrgId,
		PlatformUserId: binding.PlatformUserId,
		AdminId:        binding.AdminId,
		RevokedAt:      binding.RevokedAt,
		CreatedTime:    binding.CreatedTime,
		UpdatedTime:    binding.UpdatedTime,
	}
}

func channelTargetViewOf(target *model.ChannelTarget) ChannelTargetView {
	return ChannelTargetView{
		Id:                target.Id,
		ChannelId:         target.ChannelId,
		Kind:              target.Kind,
		PlatformTargetId:  target.PlatformTargetId,
		BindingId:         target.BindingId,
		Approved:          target.Approved,
		ManagementEnabled: target.ManagementEnabled,
		CreatedTime:       target.CreatedTime,
		UpdatedTime:       target.UpdatedTime,
	}
}

func channelRuleViewOf(rule *model.NotificationRule) ChannelRuleView {
	targetIds := []uint{}
	if strings.TrimSpace(rule.TargetIds) != "" {
		if err := json.Unmarshal([]byte(rule.TargetIds), &targetIds); err != nil {
			targetIds = []uint{}
		}
	}
	return ChannelRuleView{
		Id:          rule.Id,
		Name:        rule.Name,
		EventType:   rule.EventType,
		Enabled:     rule.Enabled,
		TargetIds:   targetIds,
		TemplateId:  rule.TemplateId,
		CreatedTime: rule.CreatedTime,
		UpdatedTime: rule.UpdatedTime,
	}
}

func channelTemplateViewOf(tpl *model.NotificationTemplate) ChannelTemplateView {
	return ChannelTemplateView{
		Id:                    tpl.Id,
		Key:                   tpl.Key,
		Name:                  tpl.Name,
		Subject:               tpl.Subject,
		Content:               tpl.Content,
		PlatformOverridesJson: tpl.PlatformOverridesJson,
		CreatedTime:           tpl.CreatedTime,
		UpdatedTime:           tpl.UpdatedTime,
	}
}
