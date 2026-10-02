package model

import (
	"database/sql/driver"
	"encoding/json"
)

type PaymentAccount struct {
	Id                 int64     `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`                       // 账户 ID
	CreatedTime        int64     `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time" ast:"-"` // 创建时间
	UpdatedTime        int64     `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time" ast:"-"` // 更新时间
	PayWay             string    `json:"pay_way" gorm:"column:pay_way;type:varchar(32) not null;default:'';index"`                           // 支付方式：wechat=微信网页支付，weapp=微信小程序支付，alipay=支付宝支付，offline=线下支付，balance=余额支付，paypal=paypal
	AccountName        string    `json:"account_name" gorm:"column:name;type:varchar(100) not null"`                                         // 账户名称
	Status             uint      `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"`                                     // 状态 (1启用, 0停用, 2风控暂停)
	HealthScore        int       `json:"health_score" gorm:"column:health_score;type:int(10) not null;default:0"`                            // 健康度评分 (0-100)
	PayConfig          PayConfig `json:"pay_config" gorm:"pay_config;type:text default null"`                                                // 支付配置
	MinAmount          int64     `json:"min_amount" gorm:"column:min_amount;type:bigint(20) not null;default:0"`                             // 最小收款金额
	MaxAmount          int64     `json:"max_amount" gorm:"column:max_amount;type:bigint(20) not null;default:0"`                             // 最大收款金额
	DailyAmountLimit   int64     `json:"daily_amount_limit" gorm:"column:daily_amount_limit;type:bigint(20) not null;default:0"`             // 单日收款上限
	DailyCountLimit    int64     `json:"daily_count_limit" gorm:"column:daily_count_limit;type:bigint(20) not null;default:0"`               // 单日收款次数上限
	MonthlyAmountLimit int64     `json:"monthly_amount_limit" gorm:"column:monthly_amount_limit;type:bigint(20) not null;default:0"`         // 单月收款上限
	LastUsedTime       int64     `json:"last_used_time" gorm:"column:last_used_time;type:int(11) not null;default:0"`                        // 最后使用时间
	UsedCount          int64     `json:"used_count" gorm:"column:used_count;type:bigint(20) not null;default:0"`                             // 累计使用次数
	UsedAmount         int64     `json:"used_amount" gorm:"column:used_amount;type:bigint(20) not null;default:0"`                           // 累计使用金额
	Weight             int64     `json:"weight" gorm:"column:weight;type:bigint(20) not null;default:0"`                                     // 权重
	IsDefault          bool      `json:"is_default" gorm:"column:is_default;type:tinyint(1) not null;default:0"`                             // 是否为默认收款账户
}

// PaymentStatistic 该表每日统计，month_amount 则是当月累计
type PaymentStatistic struct {
	Id            int64 `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	AccountId     int64 `json:"account_id" gorm:"column:account_id;type:int(10) unsigned not null;default:0;index"` // 支付账号ID
	StatTime      int64 `json:"stat_time" gorm:"column:stat_time;type:int(11) not null;default:0;index"`            // 统计日期
	DailyAmount   int64 `json:"daily_amount" gorm:"column:daily_amount;type:bigint(20) not null;default:0"`         // 当日收款金额
	MonthlyAmount int64 `json:"monthly_amount" gorm:"column:monthly_amount;type:bigint(20) not null;default:0"`     // 当月收款金额
	DailyCount    int64 `json:"daily_count" gorm:"column:daily_count;type:int(10) unsigned not null;default:0"`     // 当日交易笔数
}

type PayConfig struct {
	AppId          string `json:"app_id"`           // 支付宝的appId，微信的appId，PayPal的clientId, Stripe的apiKey
	AppSecret      string `json:"app_secret"`       // 支付宝的privateKey， 微信的appSecret，PayPal的clientSecret, Stripe的publicKey
	ApiKey         string `json:"api_key"`          // 微信的apiKey
	Account        string `json:"account"`          // PayPal的account，Stripe的account，微信的商户号，支付宝的商户号
	CertPath       string `json:"cert_path"`        // 支付宝的应用公钥，微信的证书路径
	PublicCertPath string `json:"public_cert_path"` // 支付宝的公钥证书文件路径，微信的keyPath
	RootCertPath   string `json:"root_cert_path"`   // 支付宝的根证书文件路径
	WebhookId      string `json:"webhook_id"`       // PayPal的webhookId，Stripe的webhookSecret
	Sandbox        bool   `json:"sandbox"`          // 沙盒环境
}

func (e PayConfig) Value() (driver.Value, error) {
	return json.Marshal(e)
}

// Scan 这里不报错
func (e *PayConfig) Scan(data interface{}) error {
	_ = json.Unmarshal(data.([]byte), &e)
	return nil
}
