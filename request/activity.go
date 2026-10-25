package request

import "github.com/lib/pq"

// ActivityRequest 营销活动保存请求。
// ⚠️ 金额字段（price）单位是**分**。保存是逐字段覆盖：未传的字段会被清成零值。
type ActivityRequest struct {
	Id           int64                  `json:"id"`              // 活动 ID，0 表示新建，>0 表示按 id 更新
	GoodsType    string                 `json:"goods_type"`      // 商品类型标记（≤16 字符），用于前台区分活动来源；后台表单不改它，通常留空
	StartTime    int64                  `json:"start_time"`      // 活动开始时间（unix 秒）
	EndTime      int64                  `json:"end_time"`        // 活动结束时间（unix 秒）
	UserLimit    int64                  `json:"user_limit"`      // 每人限购数量，0 表示不限
	Title        string                 `json:"title"`           // 活动名称
	Description  string                 `json:"description"`     // 活动介绍
	Images       pq.StringArray         `json:"images"`          // 活动宣传图 URL 列表
	GoodsId      int64                  `json:"goods_id"`        // 关联商品（文档）ID
	Price        int64                  `json:"price"`           // 活动价（分）
	Stock        int64                  `json:"stock"`           // 活动库存；传了 skus 时以 skus 的库存合计为准
	Extra        map[string]interface{} `json:"extra,omitempty"` // 附加设置对象（自定义扩展数据，原样存库）
	CanUseCoupon bool                   `json:"can_use_coupon"`  // 是否允许与优惠券等其他优惠叠加
	Status       uint                   `json:"status"`          // 活动状态：1 生效，0 失效

	// Skus 规格价格与库存。⚠️ 传空数组或不传会**删除该活动的全部规格**，
	// 改标题这类局部更新前必须先 GET 详情把 skus 一起回传。
	Skus []ActivitySku `json:"skus"`
}

// ActivitySku 活动规格（每个商品 SKU 在活动中的价格与库存）。
type ActivitySku struct {
	Id         int64 `json:"id"`          // 规格记录 ID，0 表示新增
	ActivityId int64 `json:"activity_id"` // 所属活动 ID（由服务端回填，客户端不用传）
	SkuId      int64 `json:"sku_id"`      // 商品规格 ID
	GoodsId    int64 `json:"goods_id"`    // 商品 ID
	Price      int64 `json:"price"`       // 该规格的活动价（分）
	Stock      int64 `json:"stock"`       // 该规格的活动库存
}
