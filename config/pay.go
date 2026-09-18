package config

type PluginRetailerConfig struct {
	AllowSelf      int64 `json:"allow_self"`      // 允许自购 0,1
	BecomeRetailer int64 `json:"become_retailer"` // 成为分销员方式， 0 审核，1 自动
}

type PluginOrderConfig struct {
	NoProcess       bool  `json:"no_process"`        // 是否没有交易流程
	AutoFinishDay   int   `json:"auto_finish_day"`   // 自动完成订单时间
	AutoCloseMinute int64 `json:"auto_close_minute"` // 自动关闭订单时间
	SellerPercent   int64 `json:"seller_percent"`    // 商家销售获得收益比例
	NoNeedLogin     bool  `json:"no_need_login"`     // 免登陆下单，免登陆下单会根据收货信息自动创建用户
}
