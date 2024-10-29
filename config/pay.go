package config

type PluginPayConfig struct {
	AlipayAppId          string `json:"alipay_app_id"`
	AlipayPrivateKey     string `json:"alipay_private_key"`
	AlipayCertPath       string `json:"alipay_cert_path"`        // 应用公钥证书路径
	AlipayRootCertPath   string `json:"alipay_root_cert_path"`   // 支付宝根证书文件路径
	AlipayPublicCertPath string `json:"alipay_public_cert_path"` // 支付宝公钥证书文件路径

	WechatAppId     string `json:"wechat_app_id"`     // 公众号
	WechatAppSecret string `json:"wechat_app_secret"` // 公众号
	WeappAppId      string `json:"weapp_app_id"`      // 小程序
	WeappAppSecret  string `json:"weapp_app_secret"`  // 小程序
	WechatMchId     string `json:"wechat_mch_id"`     // 公众号、小程序共用商户ID
	WechatApiKey    string `json:"wechat_api_key"`    // 公众号、小程序共用支付密钥
	WechatCertPath  string `json:"wechat_cert_path"`  // 证书路径
	WechatKeyPath   string `json:"wechat_key_path"`   // 证书路径

	PaypalClientId     string `json:"paypal_client_id"`     // paypal
	PaypalClientSecret string `json:"paypal_client_secret"` // paypal
}

type PluginRetailerConfig struct {
	AllowSelf      int64 `json:"allow_self"`      // 允许自购 0,1
	BecomeRetailer int64 `json:"become_retailer"` // 成为分销员方式， 0 审核，1 自动
}

type PluginOrderConfig struct {
	NoProcess       bool  `json:"no_process"`        // 是否没有交易流程
	AutoFinishDay   int   `json:"auto_finish_day"`   // 自动完成订单时间
	AutoCloseMinute int64 `json:"auto_close_minute"` // 自动关闭订单时间
	SellerPercent   int64 `json:"seller_percent"`    // 商家销售获得收益比例
}
