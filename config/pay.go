package config

type pluginPayConfig struct {
	AlipayAppId      string `json:"alipay_app_id"`
	AlipayPrivateKey string `json:"alipay_private_key"`
	AlipayCertPath   string `json:"alipay_cert_path"`

	WeixinAppId     string `json:"weixin_app_id"`
	WeixinAppSecret string `json:"weixin_app_secret"`
	WeixinMchId     string `json:"weixin_mch_id"`
	WeixinApiKey    string `json:"weixin_api_key"`
	WeixinCertPath  string `json:"weixin_cert_path"`
	WeixinKeyPath   string `json:"weixin_key_path"`
}

type pluginWeappConfig struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
	//公众号存在的部分
	Token          string `json:"token"`
	EncodingAESKey string `json:"encoding_aes_key"`
}

type pluginRetailerConfig struct {
	GoodsPrice     int64 `json:"goods_price"`
	ShareReward    int64 `json:"share_reward"`    // 分销佣金比例
	ParentReward   int64 `json:"parent_reward"`   // 邀请奖励比例
	AllowSelf      int64 `json:"allow_self"`      // 允许自购 0,1
	BecomeRetailer int64 `json:"become_retailer"` // 成为分销员方式， 0 审核，1 自动
}
