package provider

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/mcp/intent"
	"kandaoni.com/anqicms/request"
)

// channelAdmin.go 是后台「渠道管理」的业务层（T9a：渠道本身的读写、状态与能力展示）。
//
// 分层约定（R81）：controller/manageController/channel.go 只做取参、调用本文件、
// 输出统一 {code,msg,data}；掩码、站点隔离、乐观锁、删除级联全在这里。
// 所有查询强制带 site_id：跨站一律按「不存在」处理，不区分「无权」与「没有」。
//
// 全部错误文案都是静态串，且不含密钥、摘要或驱动原始错误——这些端点会把 msg 直接
// 回给后台页面，任何原文透出都等于把配置细节暴露给非授权管理员。

// ChannelSecretMask 是应用密钥的掩码哨兵。
//
// 与 SendmailPasswordMask 同值但**独立声明**：复用同一个常量会让「改发信密码掩码」
// 静默改变渠道接口的协议；两处掩码将来是否需要一致是产品问题，不是实现细节。
// 保存时收到这个值或空串，表示「沿用库中真值」（R84）。
const ChannelSecretMask = "********"

// channelSecretSafeLoggerConfig 是渠道写入专用的 GORM 日志配置：
// 与 dao.go 的 defaultLogger 同写入端、同级别，只把 ParameterizedQueries 打开。
//
// 默认 logger 在 Warn 级别会把整条 SQL（含 INSERT 的参数值）打出来；渠道表唯一的
// 写入错误路径恰恰是 uk_app_identity_hash 冲突，那一行 INSERT 里带着 app_secret。
// 参数化后日志只剩占位符，密钥不再进入日志。
var channelSecretSafeLoggerConfig = logger.Config{
	SlowThreshold:             500 * time.Millisecond,
	LogLevel:                  logger.Warn,
	IgnoreRecordNotFoundError: true,
	Colorful:                  false,
	ParameterizedQueries:      true,
}

// channelSecretSafeLogger 是携带 app_secret 的写操作唯一允许使用的 GORM logger。
var channelSecretSafeLogger = logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), channelSecretSafeLoggerConfig)

// SaveMessageChannel 是 message_channels 的唯一写入入口。
//
// 用 Session 把参数化 logger 绑到本次写入（gorm.Session.Logger 会覆盖 tx.Config.Logger），
// 而不是全局改 defaultLogger：其它表的排障仍需要完整参数。
func (w *Website) SaveMessageChannel(write func(tx *gorm.DB) error) error {
	return write(w.DB.Session(&gorm.Session{Logger: channelSecretSafeLogger}))
}

// channelAppIdentityTaken 在写入前检查应用身份是否已被别的渠道占用。
//
// 预检查的意义不只是文案：命中时直接返回静态错误，避免走到唯一键冲突那条
// 「数据库报错 → 日志打印 SQL」的路径。唯一键仍然保留，用于兜住并发插入。
func (w *Website) channelAppIdentityTaken(channelId uint, appIdentityHash string) (bool, error) {
	query := w.DB.Model(&model.MessageChannel{}).Where("app_identity_hash = ?", appIdentityHash)
	if channelId > 0 {
		query = query.Where("id <> ?", channelId)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

var (
	// errChannelAdminNotFound 表示渠道不存在或不属于当前站点。
	// 两种情形必须共用一条文案：否则跨站探测能从「记录不存在」与「无权访问」的差异里
	// 判断出别的站点存了哪些渠道 ID。
	errChannelAdminNotFound = errors.New("渠道不存在")
	// errChannelRevisionStale 表示乐观锁冲突（配置在别处被改过）。
	errChannelRevisionStale = errors.New("配置已被修改，请刷新后重试")
	// errChannelAppIdentConflict 表示同一个企业应用已归属别的渠道。
	// 不回显摘要、不回显对方站点的任何信息（R85）。
	errChannelAppIdentConflict = errors.New("该企业应用已被其它渠道使用")
)

// 渠道保存与删除的静态失败文案。
var (
	errChannelReadFailed     = errors.New("读取渠道失败")
	errChannelSaveFailed     = errors.New("保存渠道失败")
	errChannelDeleteFailed   = errors.New("删除渠道失败")
	errChannelCancelWork     = errors.New("取消未完成工作失败")
	errChannelRevokeFailed   = errors.New("撤销绑定失败")
	errChannelTargetsFailed  = errors.New("删除渠道目标失败")
	errChannelDeliveryFailed = errors.New("读取投递统计失败")
)

// 保存入参的静态校验文案（R85）。
var (
	errChannelPlatformInvalid = errors.New("平台只支持飞书或钉钉")
	errChannelNameRequired    = errors.New("渠道名称不能为空")
	errChannelAppIdRequired   = errors.New("应用ID不能为空")
	errChannelSecretRequired  = errors.New("应用密钥不能为空")
	errChannelEnabledInvalid  = errors.New("启用状态只能是 0 或 1")
	errChannelChatInvalid     = errors.New("聊天管理开关只能是 0 或 1")
	errChannelAgentRequired   = errors.New("开启聊天管理时必须选择接待智能体")
	errChannelPolicyInvalid   = errors.New("策略配置必须是 JSON 对象")
)

// ChannelListItem 是渠道列表项。字段是白名单投影，不是模型序列化：
// AppSecret 只回掩码，AppIdentityHash 完全不出现（R84）。
type ChannelListItem struct {
	Id          uint   `json:"id"`
	CreatedTime int64  `json:"created_time"`
	UpdatedTime int64  `json:"updated_time"`
	Platform    string `json:"platform"`
	Name        string `json:"name"`
	Enabled     int    `json:"enabled"`
	ChatEnabled int    `json:"chat_enabled"`
	AgentId     uint   `json:"agent_id"`
	AppId       string `json:"app_id"`
	OrgId       string `json:"org_id"`
	AppSecret   string `json:"app_secret"`
	Revision    int    `json:"revision"`
}

// ChannelDetailView 在列表项基础上补策略 JSON。
// 策略里只有「意图+动作 → allow/confirm/deny」，不含任何密钥，可以原样返回。
type ChannelDetailView struct {
	ChannelListItem
	PolicyJson string `json:"policy_json"`
}

// ChannelSaveResult 是保存回执。ConnectionPending 表示保存已成功，
// 但连接没能在本次请求内重建（下次扫描会继续尝试），前端据此提示「连接待生效」。
type ChannelSaveResult struct {
	Id                uint `json:"id"`
	Revision          int  `json:"revision"`
	ConnectionPending bool `json:"connection_pending"`
}

// ChannelStatusView 是连接与工作者状态（R87）。
// Connected 只代表**本进程**是否持有该渠道的运行中 worker：多实例部署时拿到的是
// 「当前响应这个请求的实例」的视角，不是集群共识。
type ChannelStatusView struct {
	Id                uint   `json:"id"`
	Platform          string `json:"platform"`
	Name              string `json:"name"`
	Enabled           int    `json:"enabled"`
	ChatEnabled       int    `json:"chat_enabled"`
	AgentId           uint   `json:"agent_id"`
	Revision          int    `json:"revision"`
	Connected         bool   `json:"connected"`
	LeaseExpiresAt    int64  `json:"lease_expires_at"`
	PendingDeliveries int64  `json:"pending_deliveries"`
	FailedDeliveries  int64  `json:"failed_deliveries"`
}

// ChannelCapabilityIntent 是单个意图在渠道侧的可配置状态（R88）。
type ChannelCapabilityIntent struct {
	Name       string   `json:"name"`
	Title      string   `json:"title"`
	Risk       string   `json:"risk"`
	Domain     string   `json:"domain"`
	DefaultOff bool     `json:"default_off"`
	Actions    []string `json:"actions"`
	MenuPaths  []string `json:"menu_paths"`
	Blocked    bool     `json:"blocked"`
	Reason     string   `json:"reason"`
	// AgentAllowed 只在请求带 agent_id 时出现（nil 会被 omitempty 丢掉）。
	AgentAllowed *bool `json:"agent_allowed,omitempty"`
	// Policy 是该意图的缺省裁决（策略里的 "*"，未配置即 deny）。
	// 动作级差异由渠道策略本身表达，这里不做有损合并。
	Policy string `json:"policy"`
}

// ChannelCapabilitiesView 是 /capabilities 的载荷。
// 不含任何参数 schema（模型不该在后台页面里看到提示词级细节），也不含密钥。
type ChannelCapabilitiesView struct {
	Intents         []ChannelCapabilityIntent `json:"intents"`
	BlockedBuiltins []string                  `json:"blocked_builtins"`
}

// ChannelAdminList 返回当前站点的渠道列表（密钥掩码）。
func (w *Website) ChannelAdminList() ([]ChannelListItem, error) {
	if w == nil || w.DB == nil {
		return nil, errChannelReadFailed
	}
	var channels []model.MessageChannel
	if err := w.DB.Where("`site_id` = ?", w.Id).Order("id desc").Find(&channels).Error; err != nil {
		log.Printf("[channel] 站点 %d 读取渠道列表失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return nil, errChannelReadFailed
	}
	items := make([]ChannelListItem, 0, len(channels))
	for i := range channels {
		items = append(items, channelListItemOf(&channels[i]))
	}
	return items, nil
}

// ChannelAdminDetail 返回渠道详情（含策略 JSON，密钥仍是掩码）。
func (w *Website) ChannelAdminDetail(channelId uint) (*ChannelDetailView, error) {
	channel, err := w.channelForAdmin(channelId)
	if err != nil {
		return nil, err
	}
	return &ChannelDetailView{
		ChannelListItem: channelListItemOf(channel),
		PolicyJson:      channel.PolicyJson,
	}, nil
}

// ChannelAdminSave 新建或更新渠道（R85）。
//
// 新建（id==0）要求密钥必填；更新（id>0）必须带与库中一致的 revision，
// 用条件更新 `WHERE id AND site_id AND revision` 加自增 revision 实现乐观锁。
// 保存成功后请求一次单渠道重建：管理者未启动属正常（测试/CLI），忽略；
// 其它失败只记日志并把 connection_pending 置真，不让「连接重启失败」把保存判成失败——
// 配置已经落库，报失败只会诱导管理员重复提交。
func (w *Website) ChannelAdminSave(req *request.Channel) (*ChannelSaveResult, error) {
	if w == nil || w.DB == nil || req == nil {
		return nil, errChannelSaveFailed
	}
	platform := strings.TrimSpace(req.Platform)
	switch platform {
	case model.ChannelPlatformFeishu, model.ChannelPlatformDingTalk:
	default:
		return nil, errChannelPlatformInvalid
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errChannelNameRequired
	}
	appId := strings.TrimSpace(req.AppId)
	if appId == "" {
		return nil, errChannelAppIdRequired
	}
	orgId := strings.TrimSpace(req.OrgId)
	if req.Enabled != 0 && req.Enabled != 1 {
		return nil, errChannelEnabledInvalid
	}
	if req.ChatEnabled != 0 && req.ChatEnabled != 1 {
		return nil, errChannelChatInvalid
	}
	if req.ChatEnabled == 1 && req.AgentId == 0 {
		return nil, errChannelAgentRequired
	}
	policyJson := strings.TrimSpace(req.PolicyJson)
	if policyJson != "" {
		// 只接受 JSON **对象**：数组/标量能落库，但读取侧 ParseChannelPolicy
		// 会解析失败并按「策略损坏」整条拒绝，表现为保存成功、聊天立刻不可用。
		var policy map[string]any
		if err := json.Unmarshal([]byte(policyJson), &policy); err != nil || policy == nil {
			return nil, errChannelPolicyInvalid
		}
	}

	if req.Id == 0 {
		return w.createAdminChannel(req, platform, name, appId, orgId, policyJson)
	}
	return w.updateAdminChannel(req, platform, name, appId, orgId, policyJson)
}

func (w *Website) createAdminChannel(req *request.Channel, platform, name, appId, orgId, policyJson string) (*ChannelSaveResult, error) {
	secret := req.AppSecret
	if strings.TrimSpace(secret) == "" || secret == ChannelSecretMask {
		return nil, errChannelSecretRequired
	}
	channel := &model.MessageChannel{
		SiteId:      w.Id,
		Platform:    platform,
		Name:        name,
		Enabled:     req.Enabled,
		ChatEnabled: req.ChatEnabled,
		AgentId:     req.AgentId,
		AppId:       appId,
		OrgId:       orgId,
		AppSecret:   secret,
		PolicyJson:  policyJson,
		Revision:    1,
	}
	taken, err := w.channelAppIdentityTaken(0, model.ComputeAppIdentityHash(platform, appId, orgId))
	if err != nil {
		log.Printf("[channel] 站点 %d 检查应用身份失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return nil, errChannelSaveFailed
	}
	if taken {
		return nil, errChannelAppIdentConflict
	}
	if err := w.SaveMessageChannel(func(tx *gorm.DB) error {
		return tx.Create(channel).Error
	}); err != nil {
		if isDuplicateKeyError(err) {
			return nil, errChannelAppIdentConflict
		}
		log.Printf("[channel] 站点 %d 新建渠道失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return nil, errChannelSaveFailed
	}
	return w.scheduleChannelRebuild(channel.Id, channel.Revision), nil
}

func (w *Website) updateAdminChannel(req *request.Channel, platform, name, appId, orgId, policyJson string) (*ChannelSaveResult, error) {
	existing, err := w.channelForAdmin(req.Id)
	if err != nil {
		return nil, err
	}
	// 先比一次再进条件更新：条件更新的 0 行既可能是 revision 冲突、
	// 也可能是行在读取后被删，两者给同一句「请刷新重试」才不会互相泄底。
	if req.Revision != existing.Revision {
		return nil, errChannelRevisionStale
	}
	secret := req.AppSecret
	if strings.TrimSpace(secret) == "" || secret == ChannelSecretMask {
		secret = existing.AppSecret
	}
	// 用 map 更新：BeforeSave 钩子在 map 更新里拿不到 AppId，摘要必须在应用层补算，
	// 否则保存后摘要会停在旧值，「同一企业应用全局唯一」的判据直接被击穿。
	appIdentityHash := model.ComputeAppIdentityHash(platform, appId, orgId)
	taken, err := w.channelAppIdentityTaken(req.Id, appIdentityHash)
	if err != nil {
		log.Printf("[channel] 站点 %d 检查应用身份失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return nil, errChannelSaveFailed
	}
	if taken {
		return nil, errChannelAppIdentConflict
	}
	var res *gorm.DB
	if err := w.SaveMessageChannel(func(tx *gorm.DB) error {
		res = tx.Model(&model.MessageChannel{}).
			Where("`id` = ? AND `site_id` = ? AND `revision` = ?", req.Id, w.Id, req.Revision).
			Updates(map[string]any{
				"platform":          platform,
				"name":              name,
				"app_id":            appId,
				"org_id":            orgId,
				"app_identity_hash": appIdentityHash,
				"app_secret":        secret,
				"enabled":           req.Enabled,
				"chat_enabled":      req.ChatEnabled,
				"agent_id":          req.AgentId,
				"policy_json":       policyJson,
				"revision":          gorm.Expr("revision + 1"),
				"updated_time":      time.Now().Unix(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 行在读取后被删/被改：不在这里往下走（避免对不存在的渠道改会话代次）。
			return nil
		}
		// 切换接待智能体等于换执行主体：该渠道全部会话代次递增，
		// 旧审批与旧上下文立即失效（规格 §8「权限仍允许」）。
		if existing.AgentId != req.AgentId {
			if txErr := tx.Model(&model.ChannelSession{}).
				Where("`site_id` = ? AND `channel_id` = ?", w.Id, req.Id).
				Update("generation", gorm.Expr("generation + 1")).Error; txErr != nil {
				return txErr
			}
		}
		return nil
	}); err != nil {
		if isDuplicateKeyError(err) {
			return nil, errChannelAppIdentConflict
		}
		log.Printf("[channel] 站点 %d 更新渠道 %d 失败：%s", w.Id, req.Id, sanitizePlatformMessage(err.Error()))
		return nil, errChannelSaveFailed
	}
	if res.RowsAffected == 0 {
		return nil, errChannelRevisionStale
	}
	return w.scheduleChannelRebuild(req.Id, existing.Revision+1), nil
}

// scheduleChannelRebuild 让配置立刻生效，并把「没生效」变成可观测的回执。
func (w *Website) scheduleChannelRebuild(channelId uint, revision int) *ChannelSaveResult {
	result := &ChannelSaveResult{Id: channelId, Revision: revision}
	if err := w.RebuildChannel(channelId); err != nil && !errors.Is(err, errChannelManagerStopped) {
		log.Printf("[channel] 站点 %d 渠道 %d 保存后重建连接失败：%s", w.Id, channelId, sanitizePlatformMessage(err.Error()))
		result.ConnectionPending = true
	}
	return result
}

// ChannelAdminDelete 关闭并移除渠道（R86）。
//
// 事务内按依赖顺序清理：待处理入站任务 → 未执行审批 → 撤销绑定 → 删目标 → 删渠道行。
// 审计与投递历史**保留**：它们是「发生过什么」的记录，渠道没了也要能追溯；
// 投递行在发送前会复核目标/渠道有效性，悬空引用不会导致误发。
func (w *Website) ChannelAdminDelete(channelId uint) error {
	if w == nil || w.DB == nil || channelId == 0 {
		return errChannelAdminNotFound
	}
	if _, err := w.channelForAdmin(channelId); err != nil {
		return err
	}
	err := w.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()

		if err := tx.Model(&model.ChannelInbox{}).
			Where("`site_id` = ? AND `channel_id` = ? AND `status` = ?", w.Id, channelId, model.InboxStatusPending).
			Update("status", model.InboxStatusCancelled).Error; err != nil {
			return errChannelCancelWork
		}

		var bindingIds []uint
		if err := tx.Model(&model.ChannelBinding{}).
			Where("`site_id` = ? AND `channel_id` = ?", w.Id, channelId).
			Pluck("id", &bindingIds).Error; err != nil {
			return errChannelCancelWork
		}
		if len(bindingIds) > 0 {
			if err := tx.Model(&model.ChannelApproval{}).
				Where("`site_id` = ? AND `binding_id` IN ? AND `status` IN ?", w.Id, bindingIds,
					[]string{model.ApprovalStatusPending, model.ApprovalStatusApproved}).
				Update("status", model.ApprovalStatusCancelled).Error; err != nil {
				return errChannelCancelWork
			}
		}

		if err := tx.Model(&model.ChannelBinding{}).
			Where("`site_id` = ? AND `channel_id` = ? AND `revoked_at` = 0", w.Id, channelId).
			Updates(map[string]any{
				"revoked_at": now,
				"revision":   gorm.Expr("revision + 1"),
			}).Error; err != nil {
			return errChannelRevokeFailed
		}

		if err := tx.Where("`site_id` = ? AND `channel_id` = ?", w.Id, channelId).
			Delete(&model.ChannelTarget{}).Error; err != nil {
			return errChannelTargetsFailed
		}

		if err := tx.Where("`id` = ? AND `site_id` = ?", channelId, w.Id).
			Delete(&model.MessageChannel{}).Error; err != nil {
			return errChannelDeleteFailed
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errChannelCancelWork) || errors.Is(err, errChannelRevokeFailed) ||
			errors.Is(err, errChannelTargetsFailed) || errors.Is(err, errChannelDeleteFailed) {
			return err
		}
		log.Printf("[channel] 站点 %d 删除渠道 %d 失败：%s", w.Id, channelId, sanitizePlatformMessage(err.Error()))
		return errChannelDeleteFailed
	}
	// 事务提交后再让 worker 退出：rebuildOne 读不到行就会停连接并释放租约。
	if err := w.RebuildChannel(channelId); err != nil && !errors.Is(err, errChannelManagerStopped) {
		log.Printf("[channel] 站点 %d 渠道 %d 删除后停止连接失败：%s", w.Id, channelId, sanitizePlatformMessage(err.Error()))
	}
	return nil
}

// ChannelAdminStatus 返回连接与工作者状态（R87）。
func (w *Website) ChannelAdminStatus(channelId uint) (*ChannelStatusView, error) {
	channel, err := w.channelForAdmin(channelId)
	if err != nil {
		return nil, err
	}
	running, leaseExpiresAt := currentChannelRuntime(w.Id, channelId)
	pending, failed, err := w.countChannelDeliveries(channelId)
	if err != nil {
		return nil, err
	}
	return &ChannelStatusView{
		Id:                channel.Id,
		Platform:          channel.Platform,
		Name:              channel.Name,
		Enabled:           channel.Enabled,
		ChatEnabled:       channel.ChatEnabled,
		AgentId:           channel.AgentId,
		Revision:          channel.Revision,
		Connected:         running,
		LeaseExpiresAt:    leaseExpiresAt,
		PendingDeliveries: pending,
		FailedDeliveries:  failed,
	}, nil
}

// countChannelDeliveries 统计该渠道目标上的待发与失败投递。
// 投递行只有 target_id，没有 channel_id，所以必须经 channel_targets 关联——
// 直接按 site_id 统计会把整个站点的投递都算到每个渠道头上。
func (w *Website) countChannelDeliveries(channelId uint) (pending, failed int64, err error) {
	count := func(statuses []string) int64 {
		var total int64
		res := w.DB.Model(&model.NotificationDelivery{}).
			Joins("JOIN `channel_targets` ON `channel_targets`.`id` = `notification_deliveries`.`target_id`").
			Where("`notification_deliveries`.`site_id` = ? AND `channel_targets`.`site_id` = ? AND `channel_targets`.`channel_id` = ? AND `notification_deliveries`.`status` IN ?",
				w.Id, w.Id, channelId, statuses).
			Count(&total)
		if res.Error != nil {
			log.Printf("[channel] 站点 %d 统计渠道 %d 投递失败：%s", w.Id, channelId, sanitizePlatformMessage(res.Error.Error()))
			err = res.Error
			return 0
		}
		return total
	}
	pending = count([]string{NotifyDeliveryStatusPending, NotifyDeliveryStatusSending, NotifyDeliveryStatusRetry})
	if err != nil {
		return 0, 0, errChannelDeliveryFailed
	}
	failed = count([]string{NotifyDeliveryStatusFailed, NotifyDeliveryStatusUnknown})
	if err != nil {
		return 0, 0, errChannelDeliveryFailed
	}
	return pending, failed, nil
}

// ChannelCapabilities 返回能力、动作、权限映射、风险与当前可配置状态（R88）。
//
// 与执行门禁共用同一份声明：菜单映射来自 channelIntentMenuTargets、内置黑名单来自
// channelBuiltinBlockedCaps —— 另写一份展示用名单，迟早会出现「页面显示可用、执行被拒」
// 或者更糟的「页面显示关闭、执行其实放行」。
//
// agentId>0 时补 agent_allowed，并据此解析当前渠道策略：渠道与智能体是一对一
// （channel.agent_id），因此按 agent_id 反查本站点渠道；查不到就是空策略（全部 deny）。
func (w *Website) ChannelCapabilities(agentId uint) (*ChannelCapabilitiesView, error) {
	if w == nil || w.DB == nil {
		return nil, errChannelReadFailed
	}
	policy := ChannelPolicy{}
	if agentId > 0 {
		var channel model.MessageChannel
		err := w.DB.Where("`site_id` = ? AND `agent_id` = ?", w.Id, agentId).Order("id desc").Take(&channel).Error
		if err == nil {
			// 策略解析失败按空策略处理（Decision 全部 deny），与执行侧一致。
			policy, _ = ParseChannelPolicy(channel.PolicyJson)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[channel] 站点 %d 按智能体 %d 读取渠道失败：%s", w.Id, agentId, sanitizePlatformMessage(err.Error()))
			return nil, errChannelReadFailed
		}
	}

	intents := make([]ChannelCapabilityIntent, 0, len(intent.IntentCatalog))
	for _, spec := range intent.IntentCatalog {
		if spec == nil {
			continue
		}
		menuPaths, hardBlocked := channelIntentMenuTargets(spec)
		item := ChannelCapabilityIntent{
			Name:       spec.Name,
			Title:      spec.Title,
			Risk:       string(spec.Risk),
			Domain:     string(spec.Domain),
			DefaultOff: spec.DefaultOff,
			Actions:    channelIntentActions(spec),
			MenuPaths:  menuPaths,
			Blocked:    hardBlocked != "",
			Reason:     hardBlocked,
			Policy:     string(policy.Decision(spec.Name, "")),
		}
		if menuPaths == nil {
			item.MenuPaths = []string{}
		}
		if agentId > 0 {
			// 复用执行门禁的白名单判定，不另写一份：展示与执行必须同源。
			allowed := w.checkChannelAgentWhitelist(context.Background(), agentId, spec.Name) == nil
			item.AgentAllowed = &allowed
		}
		intents = append(intents, item)
	}

	blocked := make([]string, 0, len(channelBuiltinBlockedCaps)+len(channelUnmappedBlockedCaps))
	for name := range channelBuiltinBlockedCaps {
		blocked = append(blocked, name)
	}
	for name := range channelUnmappedBlockedCaps {
		blocked = append(blocked, name)
	}
	sort.Strings(blocked)

	return &ChannelCapabilitiesView{Intents: intents, BlockedBuiltins: blocked}, nil
}

// channelIntentActions 取意图的动作枚举（spec.Params["action"].Enum）；没有 action 参数时给空数组。
func channelIntentActions(spec *intent.IntentSpec) []string {
	if spec == nil {
		return []string{}
	}
	param, ok := spec.Params["action"]
	if !ok || len(param.Enum) == 0 {
		return []string{}
	}
	actions := make([]string, len(param.Enum))
	copy(actions, param.Enum)
	return actions
}

// channelForAdmin 取当前站点的渠道行；跨站一律按「不存在」处理。
func (w *Website) channelForAdmin(channelId uint) (*model.MessageChannel, error) {
	if w == nil || w.DB == nil || channelId == 0 {
		return nil, errChannelAdminNotFound
	}
	var channel model.MessageChannel
	err := w.DB.Where("`id` = ? AND `site_id` = ?", channelId, w.Id).Take(&channel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errChannelAdminNotFound
		}
		log.Printf("[channel] 站点 %d 读取渠道 %d 失败：%s", w.Id, channelId, sanitizePlatformMessage(err.Error()))
		return nil, errChannelReadFailed
	}
	return &channel, nil
}

// channelListItemOf 是渠道行的白名单投影：密钥只回掩码，摘要不出现在任何字段里。
func channelListItemOf(channel *model.MessageChannel) ChannelListItem {
	masked := ""
	if strings.TrimSpace(channel.AppSecret) != "" {
		masked = ChannelSecretMask
	}
	return ChannelListItem{
		Id:          channel.Id,
		CreatedTime: channel.CreatedTime,
		UpdatedTime: channel.UpdatedTime,
		Platform:    channel.Platform,
		Name:        channel.Name,
		Enabled:     channel.Enabled,
		ChatEnabled: channel.ChatEnabled,
		AgentId:     channel.AgentId,
		AppId:       channel.AppId,
		OrgId:       channel.OrgId,
		AppSecret:   masked,
		Revision:    channel.Revision,
	}
}
