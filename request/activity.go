package request

import "github.com/lib/pq"

type ActivityRequest struct {
	Id           int64                  `json:"id"`
	GoodsType    string                 `json:"goods_type"`
	StartTime    int64                  `json:"start_time"`
	EndTime      int64                  `json:"end_time"`
	UserLimit    int64                  `json:"user_limit"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Images       pq.StringArray         `json:"images"`
	GoodsId      int64                  `json:"goods_id"`
	Price        int64                  `json:"price"`
	Stock        int64                  `json:"stock"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
	CanUseCoupon bool                   `json:"can_use_coupon" `
	Status       uint                   `json:"status"`

	Skus []ActivitySku `json:"skus"`
}

type ActivitySku struct {
	Id         int64 `json:"id"`
	ActivityId int64 `json:"activity_id"`
	SkuId      int64 `json:"sku_id"`
	GoodsId    int64 `json:"goods_id"`
	Price      int64 `json:"price"`
	Stock      int64 `json:"stock"`
}
