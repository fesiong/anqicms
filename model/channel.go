package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"gorm.io/gorm"
)

// 渠道平台标识。数据库里存的就是这两个值，下游适配器按它选择实现。
const (
	ChannelPlatformFeishu   = "feishu"
	ChannelPlatformDingTalk = "dingtalk"
)

// 入站任务状态。状态机（T12a 消费接线后完整）：
//
//	pending    → processing（被某个实例按 lease_until 抢占）
//	processing → done | failed（处理完成 / 处理出错）
//	pending    → cancelled（绑定撤销、渠道删除的级联失效）
//
// 状态值只允许引用这些常量，避免两处各写一份字符串漂移。
// 语义差异不能合并：done 是「处理完了」（含按规则忽略，原因写 last_error），
// failed 是「处理过程出错」（不自动重试，留痕给后台看）。
const (
	InboxStatusPending    = "pending"
	InboxStatusProcessing = "processing"
	InboxStatusDone       = "done"
	InboxStatusFailed     = "failed"
	InboxStatusCancelled  = "cancelled"
)

// appIdentityFieldSeparator 是应用身份拼接用的分隔符（ASCII Unit Separator）。
// 不用 ':' 之类的可打印字符：只要应用ID或组织标识里出现分隔符，
// 相邻字段换一种切分就能拼出同一个字符串，唯一约束会被静默击穿。
const appIdentityFieldSeparator = "\x1f"

// ComputeAppIdentityHash 计算「平台 + 应用ID(+组织)」的 SHA-256 小写十六进制摘要。
// 各段先 TrimSpace 再拼接：后台表单与平台回调常带首尾空格，不归一化会让同一个应用
// 算出两个不同摘要，唯一索引形同虚设。组织标识允许为空（拼成空段）。
func ComputeAppIdentityHash(platform, appId, organizationId string) string {
	joined := strings.Join([]string{
		strings.TrimSpace(platform),
		strings.TrimSpace(appId),
		strings.TrimSpace(organizationId),
	}, appIdentityFieldSeparator)
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])
}

// MessageChannel 站点接入的企业 IM 渠道（飞书长连接 / 钉钉 Stream）。
// 一个企业应用只允许归属一个站点连接，否则平台会把消息随机分流到不同连接；
// 该约束由 AppIdentityHash 的唯一索引在配置层兜底，ChannelLease 再按同一摘要
// 全局判定连接所有权。
type MessageChannel struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	SiteId      uint   `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:站点ID"`
	Platform    string `json:"platform" gorm:"column:platform;type:varchar(16) not null;default:'';index:idx_platform_app_id,priority:1;comment:平台 feishu/dingtalk"`
	Name        string `json:"name" gorm:"column:name;type:varchar(100) not null;default:'';comment:渠道名称"`
	Enabled     int    `json:"enabled" gorm:"column:enabled;type:tinyint(1) not null;default:0;index:idx_enabled;comment:1启用连接 0停用"`
	ChatEnabled int    `json:"chat_enabled" gorm:"column:chat_enabled;type:tinyint(1) not null;default:0;comment:1开启聊天管理 0仅通知"`
	AgentId     uint   `json:"agent_id" gorm:"column:agent_id;type:int(10) unsigned not null;default:0;index:idx_agent_id;comment:接待智能体ID"`
	AppId       string `json:"app_id" gorm:"column:app_id;type:varchar(100) not null;default:'';index:idx_platform_app_id,priority:2;comment:平台应用ID"`
	OrgId       string `json:"org_id" gorm:"column:org_id;type:varchar(100) not null;default:'';comment:组织标识(飞书 tenant_key / 钉钉 corp_id)"`
	// AppIdentityHash 为「平台+应用ID(+组织)」的 SHA-256，唯一：同一企业应用不允许被两个站点连接各写一份配置。
	// 只用唯一索引而不用 (platform, app_id) 复合唯一，避免长复合索引；摘要不序列化。
	AppIdentityHash string `json:"-" gorm:"column:app_identity_hash;type:char(64) not null;default:'';uniqueIndex:uk_app_identity_hash;comment:平台+应用ID(+组织)的SHA256(不序列化)"`
	// AppSecret 为平台应用密钥：不序列化，后台响应不得泄露。
	AppSecret string `json:"-" gorm:"column:app_secret;type:varchar(255) not null;default:'';comment:应用密钥(不序列化)"`
	// policy_json 列保留在库中仅作历史兼容：Go 结构不再读写，渠道授权只认站点曝光与身份。
	// Revision 仅后台保存的乐观锁。
	Revision int `json:"revision" gorm:"column:revision;type:int(10) not null;default:1;comment:配置修订号(仅后台保存的乐观锁)"`
}

// BeforeSave 在写入前补算应用身份摘要，避免每个写入口都要记得调 ComputeAppIdentityHash。
// 漏算的代价不对称：第一条空摘要能插进去，第二条才撞 uk_app_identity_hash，
// 报出来的还是「应用已被别的站点占用」这种误导性错误。
//
// AppId 为空即返回 nil：部分更新/只带主键的结构体更新时 app_id 是零值，
// 若照算会把空应用摘要写进已有行，直接击穿唯一约束并毁掉原摘要。
func (m *MessageChannel) BeforeSave(tx *gorm.DB) error {
	if strings.TrimSpace(m.AppId) == "" {
		return nil
	}
	m.AppIdentityHash = ComputeAppIdentityHash(m.Platform, m.AppId, m.OrgId)
	return nil
}

// ChannelBindingToken 后台管理员生成的一次性绑定 token。
// 明文只在创建响应里返回一次；库里只存 SHA-256 摘要。
type ChannelBindingToken struct {
	Id          uint  `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	SiteId      uint  `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:站点ID"`
	ChannelId   uint  `json:"channel_id" gorm:"column:channel_id;type:int(10) unsigned not null;default:0;index:idx_channel_id;comment:渠道ID"`
	AdminId     uint  `json:"admin_id" gorm:"column:admin_id;type:int(10) unsigned not null;default:0;index:idx_admin_id;comment:生成该token的管理员ID"`
	// TokenHash 为 token 明文的 SHA-256：唯一去重，且不序列化。
	TokenHash  string `json:"-" gorm:"column:token_hash;type:char(64) not null;default:'';uniqueIndex:uk_token_hash;comment:令牌SHA256(不序列化)"`
	ExpiresAt  int64  `json:"expires_at" gorm:"column:expires_at;type:bigint(20) not null;default:0;index:idx_expires_at;comment:过期时间戳(创建后3600秒)"`
	ConsumedAt int64  `json:"consumed_at" gorm:"column:consumed_at;type:bigint(20) not null;default:0;comment:消费时间戳(0=未消费)"`
}

// ChannelBinding 平台身份与后台管理员的一次性绑定结果。
// 唯一性用完整平台身份摘要，避免长复合索引。
type ChannelBinding struct {
	Id             uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime    int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime    int64  `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	SiteId         uint   `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:站点ID"`
	ChannelId      uint   `json:"channel_id" gorm:"column:channel_id;type:int(10) unsigned not null;default:0;index:idx_channel_id;comment:渠道ID"`
	OrgId          string `json:"org_id" gorm:"column:org_id;type:varchar(100) not null;default:'';comment:组织标识(飞书 tenant_key / 钉钉 corp_id)"`
	PlatformUserId string `json:"platform_user_id" gorm:"column:platform_user_id;type:varchar(128) not null;default:'';comment:平台用户标识"`
	AdminId        uint   `json:"admin_id" gorm:"column:admin_id;type:int(10) unsigned not null;default:0;index:idx_admin_id;comment:绑定的管理员ID"`
	RevokedAt      int64  `json:"revoked_at" gorm:"column:revoked_at;type:bigint(20) not null;default:0;comment:撤销时间戳(0=有效)"`
	Revision       int    `json:"revision" gorm:"column:revision;type:int(10) not null;default:1;comment:修订号(乐观锁)"`
	// IdentityHash 为「站点+渠道+组织+用户」规范化身份的 SHA-256，唯一。
	IdentityHash string `json:"identity_hash" gorm:"column:identity_hash;type:char(64) not null;default:'';uniqueIndex:uk_identity_hash;comment:完整平台身份SHA256"`
}

// ChannelTarget 通知/聊天目标：群聊或已绑定管理员的私聊。
type ChannelTarget struct {
	Id                uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime       int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime       int64  `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	SiteId            uint   `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:站点ID"`
	ChannelId         uint   `json:"channel_id" gorm:"column:channel_id;type:int(10) unsigned not null;default:0;index:idx_channel_id;comment:渠道ID"`
	Kind              string `json:"kind" gorm:"column:kind;type:varchar(16) not null;default:'';comment:目标类型 group/private"`
	PlatformTargetId  string `json:"platform_target_id" gorm:"column:platform_target_id;type:varchar(128) not null;default:'';comment:平台目标标识(群 chat_id / 私聊用户标识)"`
	BindingId         uint   `json:"binding_id" gorm:"column:binding_id;type:int(10) unsigned not null;default:0;index:idx_binding_id;comment:私聊目标来源绑定ID(群聊为0)"`
	Approved          int    `json:"approved" gorm:"column:approved;type:tinyint(1) not null;default:0;comment:1已批准 0未批准"`
	ManagementEnabled int    `json:"management_enabled" gorm:"column:management_enabled;type:tinyint(1) not null;default:0;comment:1该群允许管理 0仅通知"`
	// TargetHash 为「站点+渠道+类型+平台目标」规范化身份的 SHA-256，唯一。
	TargetHash string `json:"target_hash" gorm:"column:target_hash;type:char(64) not null;default:'';uniqueIndex:uk_target_hash;comment:目标身份SHA256"`
}

// ChannelInbox 平台入站事件（去重后持久化）。绑定消息不留 token 正文。
type ChannelInbox struct {
	Id          uint  `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64 `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	SiteId      uint  `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:站点ID"`
	ChannelId   uint  `json:"channel_id" gorm:"column:channel_id;type:int(10) unsigned not null;default:0;index:idx_channel_id;comment:渠道ID"`
	// BindingId 指向产生这条排队任务的绑定。撤销绑定时凭它失效排队任务（规格 §6）。
	// 入站去重发生在绑定之前，所以这里允许为 0（未绑定来源）。
	BindingId  uint   `json:"binding_id" gorm:"column:binding_id;type:int(10) unsigned not null;default:0;index:idx_binding_id;comment:来源绑定ID(0=未绑定)"`
	EventKind  string `json:"event_kind" gorm:"column:event_kind;type:varchar(32) not null;default:'';index:idx_event_kind;comment:事件类型 message/card_action/..."`
	Content    string `json:"content" gorm:"column:content;type:text;comment:脱敏正文(不存token明文)"`
	Status     string `json:"status" gorm:"column:status;type:varchar(16) not null;default:'';index:idx_status;index:idx_status_lease_until,priority:1;comment:处理状态"`
	LeaseUntil int64  `json:"lease_until" gorm:"column:lease_until;type:bigint(20) not null;default:0;index:idx_lease_until;index:idx_status_lease_until,priority:2;comment:处理租约到期时间戳"`
	// EventHash 为平台事件标识的 SHA-256，唯一，用于入站去重。
	// 摘要不序列化：后台响应不得输出事件摘要（全局约束）。
	EventHash string `json:"-" gorm:"column:event_hash;type:char(64) not null;default:'';uniqueIndex:uk_event_hash;comment:平台事件SHA256(不序列化)"`
	// PlatformChatId 是回复目标：私聊为用户 open_id/staffId，群聊为 chat_id。
	// 消费侧据此派生会话摘要与出站目标，不再回读平台 payload。
	PlatformChatId string `json:"platform_chat_id" gorm:"column:platform_chat_id;type:varchar(128) not null;default:'';comment:平台会话标识(回复目标)"`
	// IdentityHash 是发送人身份的 SHA-256（站点+渠道+组织+平台用户），
	// 用于查 channel_bindings.identity_hash。**不存平台用户明文**，也不序列化。
	IdentityHash string `json:"-" gorm:"column:identity_hash;type:char(64) not null;default:'';index:idx_identity_hash;comment:发送人身份SHA256(不序列化)"`
	// ChatType 取 group / p2p：群聊与私聊的权限边界、回复目标都不同，必须随任务落库。
	ChatType string `json:"chat_type" gorm:"column:chat_type;type:varchar(16) not null;default:'';comment:会话类型 group/p2p"`
	// LastError 记录忽略原因或处理失败原因（已脱敏，不含正文与平台标识原文）。
	LastError string `json:"last_error" gorm:"column:last_error;type:varchar(255) not null;default:'';comment:忽略/失败原因(已脱敏)"`
}

// ChannelSession 渠道会话与内部 chat 会话的映射。
// 唯一摘要包含站点/渠道/平台会话/用户全部隔离维度。
type ChannelSession struct {
	Id             uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime    int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime    int64  `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	SiteId         uint   `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:站点ID"`
	ChannelId      uint   `json:"channel_id" gorm:"column:channel_id;type:int(10) unsigned not null;default:0;index:idx_channel_id;comment:渠道ID"`
	PlatformChatId string `json:"platform_chat_id" gorm:"column:platform_chat_id;type:varchar(128) not null;default:'';comment:平台会话标识(群/私聊)"`
	UserHash       string `json:"user_hash" gorm:"column:user_hash;type:char(64) not null;default:'';comment:平台用户身份SHA256"`
	ChatSessionId  string `json:"chat_session_id" gorm:"column:chat_session_id;type:varchar(64) not null;default:'';index:idx_chat_session_id;comment:内部chat会话ID"`
	Generation     int64  `json:"generation" gorm:"column:generation;type:bigint(20) not null;default:1;comment:会话代次(切换智能体/新建会话时递增，旧待执行工作失效)"`
	// SessionHash 为「站点+渠道+平台会话+用户」规范化摘要的 SHA-256，唯一。
	SessionHash string `json:"session_hash" gorm:"column:session_hash;type:char(64) not null;default:'';uniqueIndex:uk_session_hash;comment:会话隔离维度SHA256"`
}

// ChannelLease 平台连接所有权租约。
// 唯一性按应用身份摘要全局判定：同一企业应用不允许被两个站点连接同时持有。
// 所有持久化领取都以 FencingVersion 校验，防止过期持有者继续写入。
type ChannelLease struct {
	Id          uint  `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime"`
	UpdatedTime int64 `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime"`
	SiteId      uint  `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:持有租约的站点ID"`
	// AppIdentityHash 为「平台+应用ID(+组织)」的 SHA-256，全局唯一，每应用一个活动所有者。
	// 摘要不序列化：后台响应不得输出应用身份摘要（全局约束）。
	AppIdentityHash string `json:"-" gorm:"column:app_identity_hash;type:char(64) not null;default:'';uniqueIndex:uk_app_identity_hash;comment:应用身份SHA256(不序列化)"`
	Owner           string `json:"owner" gorm:"column:owner;type:varchar(128) not null;default:'';comment:持有者实例标识"`
	ExpiresAt       int64  `json:"expires_at" gorm:"column:expires_at;type:bigint(20) not null;default:0;index:idx_expires_at;comment:租约到期时间戳(30秒，10秒续租)"`
	FencingVersion  int64  `json:"fencing_version" gorm:"column:fencing_version;type:bigint(20) not null;default:0;comment:栅栏版本(每次换主递增)"`
}

// NewChannelLease 按渠道配置构造一条连接租约，并同步派生应用身份摘要。
// ChannelLease 上不存 platform/app_id（规格 §4 只定义摘要列），因此摘要只能在这里算：
// 让调用方各自调 ComputeAppIdentityHash 迟早会漏，而漏掉的租约会带空摘要入库，
// 「同一企业应用只允许一个活动所有者」的判据就被绕过去了。channel 约定非 nil。
func NewChannelLease(siteId uint, channel *MessageChannel, owner string, expiresAt int64) *ChannelLease {
	return &ChannelLease{
		SiteId:          siteId,
		AppIdentityHash: ComputeAppIdentityHash(channel.Platform, channel.AppId, channel.OrgId),
		Owner:           owner,
		ExpiresAt:       expiresAt,
	}
}

// ChannelAudit 渠道管理行为审计。不保存秘密与完整敏感正文。
// idx_site_created 的列序必须是 (site_id, created_time)：审计分页恒为
// WHERE site_id=? ORDER BY created_time DESC，反序的 (created_time, site_id)
// 用不上前缀过滤，站点一多就退化成全表扫描。
type ChannelAudit struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_site_created,priority:2;comment:发生时间"`
	SiteId      uint   `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_created,priority:1;comment:站点ID"`
	ChannelId   uint   `json:"channel_id" gorm:"column:channel_id;type:int(10) unsigned not null;default:0;index:idx_channel_id;comment:渠道ID"`
	BindingId   uint   `json:"binding_id" gorm:"column:binding_id;type:int(10) unsigned not null;default:0;index:idx_binding_id;comment:操作者绑定ID(0=系统)"`
	TaskId      string `json:"task_id" gorm:"column:task_id;type:varchar(64) not null;default:'';index:idx_task_id;comment:关联任务/运行ID"`
	Action      string `json:"action" gorm:"column:action;type:varchar(64) not null;default:'';index:idx_action;comment:动作名"`
	TargetHash  string `json:"target_hash" gorm:"column:target_hash;type:char(64) not null;default:'';index:idx_target_hash;comment:目标身份摘要"`
	Result      string `json:"result" gorm:"column:result;type:varchar(32) not null;default:'';comment:结果 ok/denied/failed/unknown"`
}
