package model

// NotificationRule 通知规则：首版按事件类型匹配，不引入任意表达式条件语言。
type NotificationRule struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	SiteId      uint   `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:站点ID"`
	Name        string `json:"name" gorm:"column:name;type:varchar(100) not null;default:'';comment:规则名称"`
	EventType   string `json:"event_type" gorm:"column:event_type;type:varchar(64) not null;default:'';index:idx_event_type;comment:事件类型 guestbook.created/order.created/order.paid/agent.failed"`
	Enabled     int    `json:"enabled" gorm:"column:enabled;type:tinyint(1) not null;default:0;index:idx_enabled;comment:1启用 0停用"`
	TargetIds   string `json:"target_ids" gorm:"column:target_ids;type:text;comment:接收目标ID JSON数组"`
	TemplateId  uint   `json:"template_id" gorm:"column:template_id;type:int(10) unsigned not null;default:0;index:idx_template_id;comment:通知模板ID"`
}

// NotificationTemplate 通知模板：通用聊天文案 + 平台覆盖，不存平台 JSON。
type NotificationTemplate struct {
	Id                    uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime           int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime           int64  `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	SiteId                uint   `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_key,priority:1;comment:站点ID"`
	Key                   string `json:"key" gorm:"column:key;type:varchar(64) not null;default:'';index:idx_site_key,priority:2;comment:模板键"`
	Name                  string `json:"name" gorm:"column:name;type:varchar(100) not null;default:'';comment:模板名称"`
	Subject               string `json:"subject" gorm:"column:subject;type:varchar(255) not null;default:'';comment:标题"`
	Content               string `json:"content" gorm:"column:content;type:text;comment:正文(Pongo2 受限上下文)"`
	PlatformOverridesJson string `json:"platform_overrides_json" gorm:"column:platform_overrides_json;type:text;comment:按平台覆盖的标题与正文JSON"`
}

// NotificationEvent 通知事件：白名单 payload，同一业务事件只创建一次。
type NotificationEvent struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	SiteId      uint   `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:站点ID"`
	EventType   string `json:"event_type" gorm:"column:event_type;type:varchar(64) not null;default:'';index:idx_event_type;comment:事件类型"`
	EventKey    string `json:"event_key" gorm:"column:event_key;type:varchar(191) not null;default:'';comment:业务事件键(事件类型+业务标识)"`
	OccurredAt  int64  `json:"occurred_at" gorm:"column:occurred_at;type:bigint(20) not null;default:0;index:idx_occurred_at;comment:业务发生时间戳"`
	PayloadJson string `json:"payload_json" gorm:"column:payload_json;type:text;comment:白名单字段JSON"`
	Status      string `json:"status" gorm:"column:status;type:varchar(16) not null;default:'';index:idx_status;comment:状态(含 waiting_classification)"`
	// EventKeyHash 为 EventKey 的 SHA-256，唯一，用于「同一业务事件只创建一次」。
	EventKeyHash string `json:"event_key_hash" gorm:"column:event_key_hash;type:char(64) not null;default:'';uniqueIndex:uk_event_key_hash;comment:业务事件键SHA256"`
}

// NotificationDelivery 逐目标投递任务。
type NotificationDelivery struct {
	Id                uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime       int64  `json:"created_time" gorm:"column:created_time;type:bigint(20);autoCreateTime;index:idx_created_time"`
	UpdatedTime       int64  `json:"updated_time" gorm:"column:updated_time;type:bigint(20);autoUpdateTime;index:idx_updated_time"`
	SiteId            uint   `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0;index:idx_site_id;comment:站点ID"`
	EventId           uint   `json:"event_id" gorm:"column:event_id;type:int(10) unsigned not null;default:0;index:idx_event_rule_target,priority:1;comment:事件ID"`
	RuleId            uint   `json:"rule_id" gorm:"column:rule_id;type:int(10) unsigned not null;default:0;index:idx_event_rule_target,priority:2;comment:规则ID"`
	TargetId          uint   `json:"target_id" gorm:"column:target_id;type:int(10) unsigned not null;default:0;index:idx_event_rule_target,priority:3;comment:目标ID"`
	Status            string `json:"status" gorm:"column:status;type:varchar(16) not null;default:'pending';index:idx_status;comment:状态 pending/sending/retry/sent/failed/cancelled/unknown"`
	Attempts          int    `json:"attempts" gorm:"column:attempts;type:int(10) not null;default:0;comment:已尝试次数(最多6次)"`
	NextAttemptAt     int64  `json:"next_attempt_at" gorm:"column:next_attempt_at;type:bigint(20) not null;default:0;index:idx_next_attempt_at;comment:下次尝试时间戳"`
	LeaseUntil        int64  `json:"lease_until" gorm:"column:lease_until;type:bigint(20) not null;default:0;index:idx_lease_until;comment:发送租约到期时间戳"`
	LastError         string `json:"last_error" gorm:"column:last_error;type:text;comment:最近一次错误摘要"`
	PlatformMessageId string `json:"platform_message_id" gorm:"column:platform_message_id;type:varchar(128) not null;default:'';comment:平台消息ID"`
	// DeliveryHash 为 (EventId, RuleId, TargetId) 的 SHA-256，唯一，避免长复合唯一索引。
	DeliveryHash string `json:"delivery_hash" gorm:"column:delivery_hash;type:char(64) not null;default:'';uniqueIndex:uk_delivery_hash;comment:投递三元组SHA256"`
}
