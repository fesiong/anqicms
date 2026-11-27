package model

// SmsCode 短信验证码状态机：只存 HMAC 摘要，不存验证码明文。
// 字段与列定义逐字来自规格 §4.2。
type SmsCode struct {
	Id           uint   `json:"id" gorm:"column:id;type:int unsigned;primaryKey;autoIncrement"`
	SiteId       uint   `json:"site_id" gorm:"column:site_id;type:int unsigned;not null;default:0;index:idx_site_phone_scene,priority:1"`
	PhoneCountry string `json:"phone_country" gorm:"column:phone_country;type:varchar(8);not null;default:'86';index:idx_site_phone_scene,priority:2"`
	Phone        string `json:"phone" gorm:"column:phone;type:varchar(32);not null;default:'';index:idx_site_phone_scene,priority:3"`
	Scene        string `json:"scene" gorm:"column:scene;type:varchar(32);not null;default:'';index:idx_site_phone_scene,priority:4"`

	CodeHash    string `json:"-" gorm:"column:code_hash;type:char(64);not null;default:''"`
	Attempts    int    `json:"attempts" gorm:"column:attempts;type:int;not null;default:0"`
	MaxAttempts int    `json:"max_attempts" gorm:"column:max_attempts;type:int;not null;default:5"`
	ExpiresAt   int64  `json:"expires_at" gorm:"column:expires_at;type:bigint;not null;default:0;index:idx_expires_at"`
	ConsumedAt  int64  `json:"consumed_at" gorm:"column:consumed_at;type:bigint;not null;default:0"`

	Provider   string `json:"provider" gorm:"column:provider;type:varchar(16);not null;default:''"`
	Region     string `json:"region" gorm:"column:region;type:varchar(16);not null;default:''"`
	BizId      string `json:"biz_id" gorm:"column:biz_id;type:varchar(64);not null;default:''"`
	SendStatus int    `json:"send_status" gorm:"column:send_status;type:tinyint;not null;default:0"` // 0 待发 1 成功 -1 失败
	LogId      uint   `json:"log_id" gorm:"column:log_id;type:int unsigned;not null;default:0"`

	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:bigint;not null;default:0"`
}

// SmsLog 短信发送流水：不存验证码明文，也不存渲染后的正文。
// 字段与列定义逐字来自规格 §4.3；TemplateId 列宽 2000（R153）：Twilio 的该槽
// 承载整段未渲染文案（provider/sms/resolve.go），其 Body 上限是 1600 字符，
// varchar(500) 仍会让接近上限的模板触发 1406。老库由 provider/dao.go 的
// ensureSmsLogTemplateColumn 幂等加宽。
type SmsLog struct {
	Id           uint   `json:"id" gorm:"column:id;type:int unsigned;primaryKey;autoIncrement"`
	SiteId       uint   `json:"site_id" gorm:"column:site_id;type:int unsigned;not null;default:0;index:idx_site_created,priority:1"`
	PhoneCountry string `json:"phone_country" gorm:"column:phone_country;type:varchar(8);not null;default:'86'"`
	Phone        string `json:"phone" gorm:"column:phone;type:varchar(32);not null;default:''"`
	Scene        string `json:"scene" gorm:"column:scene;type:varchar(32);not null;default:''"`
	Provider     string `json:"provider" gorm:"column:provider;type:varchar(16);not null;default:''"`
	Region       string `json:"region" gorm:"column:region;type:varchar(16);not null;default:''"`
	TemplateId   string `json:"template_id" gorm:"column:template_id;type:varchar(2000);not null;default:''"`
	SignName     string `json:"sign_name" gorm:"column:sign_name;type:varchar(64);not null;default:''"`
	BizId        string `json:"biz_id" gorm:"column:biz_id;type:varchar(64);not null;default:''"`

	Status    int    `json:"status" gorm:"column:status;type:tinyint;not null;default:0"` // 0 pending 1 success -1 failed
	ErrCode   string `json:"err_code" gorm:"column:err_code;type:varchar(64);not null;default:''"`
	ErrMsg    string `json:"err_msg" gorm:"column:err_msg;type:varchar(500);not null;default:''"`
	Retryable int    `json:"retryable" gorm:"column:retryable;type:tinyint(1);not null;default:0"`
	CostMs    int    `json:"cost_ms" gorm:"column:cost_ms;type:int;not null;default:0"`

	Ip          string `json:"ip" gorm:"column:ip;type:varchar(64);not null;default:''"`
	UserId      uint   `json:"user_id" gorm:"column:user_id;type:int unsigned;not null;default:0"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint;not null;default:0;index:idx_site_created,priority:2"`
}
