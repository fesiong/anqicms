package response

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

type OrderConfirmResponse struct {
	CurrencyCode   string        `json:"currency_code"`
	CurrencySymbol string        `json:"currency_symbol"`
	OrderId        string        `json:"order_id"`
	Details        []OrderDetail `json:"details"`
	OriginAmount   int64         `json:"origin_amount"`
	Amount         int64         `json:"amount"`
	ServiceFee     int64         `json:"service_fee"`
	DiscountAmount int64         `json:"discount_amount"` // 可能一个订单支持多个优惠
	CouponId       int64         `json:"coupon_id"`
	CouponCodeId   int64         `json:"coupon_code_id"`
	CouponCode     string        `json:"coupon_code"`
	AddressId      uint          `json:"address_id"`
	Remark         string        `json:"remark"`
	ServiceIds     []int64       `json:"service_ids"`
	ActivityId     int64         `json:"activity_id"`
	PayWays        []string      `json:"pay_ways"`
}

type OrderDetail struct {
	GoodsId       int64                      `json:"goods_id"`
	GoodsSkuId    int64                      `json:"goods_sku_id"`
	Price         int64                      `json:"price"`
	OriginPrice   int64                      `json:"origin_price"`
	Amount        int64                      `json:"amount"`
	RealAmount    int64                      `json:"real_amount"` // 实际支付的金额，用于退款的时候进行退款操作
	Quantity      int                        `json:"quantity"`
	Title         string                     `json:"title"`
	Thumb         string                     `json:"thumb"`
	Logo          string                     `json:"logo"`
	SkuTitle      string                     `json:"sku_title"`
	Link          string                     `json:"link"`
	WholesaleRule *model.WholesaleRule       `json:"wholesale_rule,omitempty"`
	OrderExtra    map[string]interface{}     `json:"order_extra"`
	OrderFields   []config.CustomField       `json:"order_fields"`
	SkuValues     []model.ArchiveOptionValue `json:"sku_values"` // SKU 组合字段
}

type OrderStatistic struct {
	Total      int64 `json:"total"`
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Canceled   int64 `json:"canceled"`
	Refunding  int64 `json:"refunding"`
	Refunded   int64 `json:"refunded"`
	Wishlist   int64 `json:"wishlist"`
}
