package request

type OrderRequest struct {
	Id                uint                 `json:"id"`
	OrderId           string               `json:"order_id"`
	PaymentId         string               `json:"payment_id"`
	UserId            uint                 `json:"user_id"`
	AddressId         uint                 `json:"address_id"`
	Remark            string               `json:"remark"`
	Type              string               `json:"type"`
	Status            int                  `json:"status"`
	RefundStatus      int                  `json:"refund_status"`
	OriginAmount      int64                `json:"origin_amount"`
	Amount            int64                `json:"amount"`
	PaidTime          int64                `json:"paid_time"`
	EndTime           int64                `json:"end_time"`
	DeliverTime       int64                `json:"deliver_time"`
	FinishedTime      int64                `json:"finished_time"`
	DiscountAmount    int64                `json:"discount_amount"` // 可能一个订单支持多个优惠
	CouponCodeId      string               `json:"-"`
	ShareUserId       uint                 `json:"share_user_id"`       // 分享者
	ShareAmount       int64                `json:"share_amount"`        // 分销可得金额
	ShareParentAmount int64                `json:"share_parent_amount"` // 分销可得金额
	ShareSelfAmount   int64                `json:"share_self_amount"`   // 本级
	ExpressCompany    string               `json:"express_company"`     // 快递公司
	TrackingNumber    string               `json:"tracking_number"`     // 快递单号
	Address           *OrderAddressRequest `json:"address"`
	Details           []OrderDetail        `json:"details"`

	// 接受单个，不需要detail
	GoodsId  uint `json:"goods_id"`
	Quantity int  `json:"quantity"`
}

type PaymentRequest struct {
	OrderId string `json:"order_id"`
	UserId  uint   `json:"user_id"`
	PayWay  string `json:"pay_way"`
}

type OrderDetail struct {
	Id          uint   `json:"id"`
	OrderId     string `json:"order_id"`
	UserId      uint   `json:"user_id"`
	GoodsId     uint   `json:"goods_id"`
	GoodsItemId uint   `json:"goods_item_id"`
	Price       int64  `json:"price"`
	OriginPrice int64  `json:"origin_price"`
	Amount      int64  `json:"amount"`
	RealAmount  int64  `json:"real_amount"` // 实际支付的金额，用于退款的时候进行退款操作
	Quantity    int    `json:"quantity"`
	Status      int    `json:"status"`
}

type OrderRefundRequest struct {
	OrderId string `json:"order_id"`
	Status  int    `json:"status"`
}

type PluginPayConfig struct {
	AlipayAppId          string `json:"alipay_app_id"`
	AlipayPrivateKey     string `json:"alipay_private_key"`
	AlipayCertPath       string `json:"alipay_cert_path"`        // 应用公钥证书路径
	AlipayRootCertPath   string `json:"alipay_root_cert_path"`   // 支付宝根证书文件路径
	AlipayPublicCertPath string `json:"alipay_public_cert_path"` // 支付宝公钥证书文件路径

	WechatAppId     string `json:"wechat_app_id"`
	WechatAppSecret string `json:"wechat_app_secret"`
	WeappAppId      string `json:"weapp_app_id"`
	WeappAppSecret  string `json:"weapp_app_secret"`
	WechatMchId     string `json:"wechat_mch_id"`
	WechatApiKey    string `json:"wechat_api_key"`
	WechatCertPath  string `json:"wechat_cert_path"`
	WechatKeyPath   string `json:"wechat_key_path"`
}

type PluginOrderConfig struct {
	NoProcess       bool  `json:"no_process"`        // 是否没有交易流程
	AutoFinishDay   int   `json:"auto_finish_day"`   // 自动完成订单时间
	AutoCloseMinute int64 `json:"auto_close_minute"` // 自动关闭订单时间
}

type OrderAddressRequest struct {
	Id          uint   `json:"id"`
	UserId      uint   `json:"user_id"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	Province    string `json:"province"`
	City        string `json:"city"`
	Country     string `json:"country"`
	AddressInfo string `json:"address_info"`
	Postcode    string `json:"postcode"`
	Status      int    `json:"status"`
}

type OrderExportRequest struct {
	Status    string `json:"status"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
}
