package request

import "kandaoni.com/anqicms/model"

type OrderRequest struct {
	Id                int64                `json:"id"`
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
	CouponId          int64                `json:"coupon_id"`
	CouponCodeId      int64                `json:"coupon_code_id"`
	CouponCode        string               `json:"coupon_code"`
	ShareUserId       uint                 `json:"share_user_id"`       // 分享者
	ShareAmount       int64                `json:"share_amount"`        // 分销可得金额
	ShareParentAmount int64                `json:"share_parent_amount"` // 分销可得金额
	ShareSelfAmount   int64                `json:"share_self_amount"`   // 本级
	Address           *OrderAddressRequest `json:"address"`
	Details           []OrderDetail        `json:"details"`

	// 接受单个，不需要detail
	GoodsId  int64 `json:"goods_id"`
	Quantity int   `json:"quantity"`
	// 重置密码
	Password string `json:"password"`
	Code     string `json:"code"`
}

type OrderDeliveryRequest struct {
	OrderId        string `json:"order_id"`        // 订单号
	ExpressCompany string `json:"express_company"` // 快递公司
	TrackingNumber string `json:"tracking_number"` // 快递单号
}

type OrderFinishedRequest struct {
	OrderId string `json:"order_id"` // 订单号
}

type PaymentRequest struct {
	OrderId string `json:"order_id"` // 订单号
	UserId  uint   `json:"user_id"`  // 订购用户
	PayWay  string `json:"pay_way"`  // 支付方式，为空时为 offline：wechat=微信网页支付，weapp=微信小程序支付，alipay=支付宝支付，offline=线下支付，balance=余额支付，paypal=paypal
}

type PaymentAccountRequest struct {
	Id                 int64           `json:"id"`                   // 账户 ID
	PayWay             string          `json:"pay_way"`              // 支付方式：wechat=微信网页支付，weapp=微信小程序支付，alipay=支付宝支付，offline=线下支付，balance=余额支付，paypal=paypal
	AccountName        string          `json:"account_name"`         // 账户名称
	Status             uint            `json:"status"`               // 状态 (1启用, 0停用, 2风控暂停)
	HealthScore        int             `json:"health_score"`         // 健康度评分 (0-100)
	PayConfig          model.PayConfig `json:"pay_config"`           // 支付配置
	MinAmount          int64           `json:"min_amount"`           // 最小收款金额
	MaxAmount          int64           `json:"max_amount"`           // 最大收款金额
	DailyAmountLimit   int64           `json:"daily_amount_limit"`   // 单日收款上限
	DailyCountLimit    int64           `json:"daily_count_limit"`    // 单日收款次数上限
	MonthlyAmountLimit int64           `json:"monthly_amount_limit"` // 单月收款上限
	Weight             int64           `json:"weight"`               // 权重
	IsDefault          bool            `json:"is_default"`           // 是否为默认收款账户
}

type PaymentAccountDeleteRequest struct {
	Id int64 `json:"id"` // 账户 ID
}

type OrderDetail struct {
	Id          uint   `json:"id"`
	OrderId     string `json:"order_id"`
	UserId      uint   `json:"user_id"`
	GoodsId     int64  `json:"goods_id"`
	GoodsSkuId  int64  `json:"goods_sku_id"`
	Price       int64  `json:"price"`
	OriginPrice int64  `json:"origin_price"`
	Amount      int64  `json:"amount"`
	RealAmount  int64  `json:"real_amount"` // 实际支付的金额，用于退款的时候进行退款操作
	Quantity    int    `json:"quantity"`
	Status      int    `json:"status"`
}

type OrderRefundRequest struct {
	OrderId string `json:"order_id"` // 订单号
	Status  int    `json:"status"`   // 退款处理状态：0=待处理，1=已退款，-1=退款失败
}

type OrderAddressRequest struct {
	Id           uint   `json:"id"`
	UserId       uint   `json:"user_id"`
	Name         string `json:"name"`
	LastName     string `json:"last_name"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	Subscribed   string `json:"subscribed"`
	Province     string `json:"province"`
	ProvinceCode string `json:"province_code"`
	City         string `json:"city"`
	Town         string `json:"town"`
	Country      string `json:"country"`
	CountryCode  string `json:"country_code"`
	AddressInfo  string `json:"address_info"`
	Company      string `json:"company"`
	Postcode     string `json:"postcode"`
	Status       int    `json:"status"`
}

type OrderExportRequest struct {
	Status    string `json:"status"`     // 订单状态：waiting,paid,delivery,finished,refunding,closed
	StartTime int64  `json:"start_time"` // 开始时间戳
	EndTime   int64  `json:"end_time"`   // 结束时间戳
}
