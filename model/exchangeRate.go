package model

type ExchangeRate struct {
	Id             int64   `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	BaseCurrency   string  `json:"base_currency" gorm:"column:base_currency;size:3;not null"`     // 基准货币(ISO 4217代码)
	TargetCurrency string  `json:"target_currency" gorm:"column:target_currency;size:3;not null"` // 目标货币(ISO 4217代码)
	Rate           float64 `json:"rate" gorm:"column:rate;type:decimal(18,8);not null"`           // 汇率值(1单位基准货币可兑换的目标货币量)
	Source         string  `json:"source" gorm:"column:source;size:32;default:central_bank"`      // '数据来源(如央行/第三方API)
	CreatedTime    int64   `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime    int64   `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"` // 更新时间
}

// ExchangeRateHistory 汇率历史数据
type ExchangeRateHistory struct {
	ExchangeRate
}
