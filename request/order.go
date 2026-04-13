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
	ActivityId        int64                `json:"activity_id"`     // 活动ID
	CouponId          int64                `json:"coupon_id"`
	CouponCodeId      int64                `json:"coupon_code_id"`
	CouponCode        string               `json:"coupon_code"`
	ShareUserId       uint                 `json:"share_user_id"`       // 分享者
	ShareAmount       int64                `json:"share_amount"`        // 分销可得金额
	ShareParentAmount int64                `json:"share_parent_amount"` // 分销可得金额
	ShareSelfAmount   int64                `json:"share_self_amount"`   // 本级
	ExpressCompany    string               `json:"express_company"`     // 快递公司
	TrackingNumber    string               `json:"tracking_number"`     // 快递单号
	Address           *OrderAddressRequest `json:"address"`
	Details           []OrderDetail        `json:"details"`
	ServiceIds        []int64              `json:"service_ids"`

	// 接受单个，不需要detail
	GoodsId    int64 `json:"goods_id"`
	GoodsSkuId int64 `json:"goods_sku_id"`
	Quantity   int   `json:"quantity"`
}

type PaymentRequest struct {
	OrderId string `json:"order_id"`
	UserId  uint   `json:"user_id"`
	PayWay  string `json:"pay_way"`
}

type OrderDetail struct {
	Id          uint                       `json:"id"`
	OrderId     string                     `json:"order_id"`
	UserId      uint                       `json:"user_id"`
	GoodsId     int64                      `json:"goods_id"`
	GoodsSkuId  int64                      `json:"goods_sku_id"`
	Price       int64                      `json:"price"`
	OriginPrice int64                      `json:"origin_price"`
	Amount      int64                      `json:"amount"`
	RealAmount  int64                      `json:"real_amount"` // 实际支付的金额，用于退款的时候进行退款操作
	Quantity    int                        `json:"quantity"`
	Status      int                        `json:"status"`
	SkuValues   []model.ArchiveOptionValue `json:"sku_values"` // SKU 组合字段
	OrderExtra  map[string]interface{}     `json:"order_extra"`
}

type OrderRefundRequest struct {
	OrderId string `json:"order_id"`
	Status  int    `json:"status"`
}

type OrderAddressRequest struct {
	Id          uint   `json:"id"`
	UserId      uint   `json:"user_id"`
	Name        string `json:"name"`
	LastName    string `json:"last_name"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Province    string `json:"province"`
	City        string `json:"city"`
	Town        string `json:"town"`
	Country     string `json:"country"`
	AddressInfo string `json:"address_info"`
	Company     string `json:"company"`
	Postcode    string `json:"postcode"`
	Status      int    `json:"status"`
}

type OrderExportRequest struct {
	Status    string `json:"status"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
}

type OrderReviewRequest struct {
	Id           int64    `json:"id"`
	ArchiveId    int64    `json:"archive_id"`
	ArchiveSkuId int64    `json:"archive_sku_id"`
	OrderId      string   `json:"order_id"`
	UserId       uint     `json:"user_id"`
	UserName     string   `json:"user_name"`
	Email        string   `json:"email"`
	Ip           string   `json:"ip"`
	Score        int      `json:"score"`
	Content      string   `json:"content"`
	Status       uint     `json:"status"`
	Images       []string `json:"images"`
}

type CartRequest struct {
	Id          int64                      `json:"id"`
	UserId      int64                      `json:"user_id"`
	Token       string                     `json:"token"`
	UserName    string                     `json:"user_name"`
	ArchiveId   int64                      `json:"archive_id"`
	SkuId       int64                      `json:"sku_id"`
	SkuValues   []model.ArchiveOptionValue `json:"sku_values"` // SKU 组合字段
	Quantity    int                        `json:"quantity"`
	Force       bool                       `json:"force"` // 强制覆盖
	Title       string                     `json:"title"`
	SkuTitle    string                     `json:"sku_title"`
	Description string                     `json:"description"`
	Remark      string                     `json:"remark"`
	Thumb       string                     `json:"thumb"`
	Price       int64                      `json:"price"`
	OriginPrice int64                      `json:"origin_price"`
	Status      int                        `json:"status"`
	CreatedTime int64                      `json:"created_time"`
	UpdatedTime int64                      `json:"updated_time"`
	OrderExtra  map[string]interface{}     `json:"order_extra"`
}

type OrderServiceRequest struct {
	Id          int64                  `json:"id"`
	ServiceCode string                 `json:"service_code"`
	ServiceName string                 `json:"service_name"`
	ServiceType string                 `json:"service_type"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Sort        uint                   `json:"sort"`
	Fee         int64                  `json:"fee"`
	FeeType     int                    `json:"fee_type"`
	MaxFee      int64                  `json:"max_fee"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}
