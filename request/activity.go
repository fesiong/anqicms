package request

import "github.com/lib/pq"

// ActivityRequest 营销活动保存请求。
// ⚠️ 金额字段（price）单位是**分**。
//
// 零值语义（2026-10-29 指针化）：除 Id/Title 外全部是**指针**。
// nil = 调用方没提这个字段（PATCH，保持库里原值）；非 nil = 显式写入，含 0 ——
// 这里每个 0 都有业务含义：status=0 活动失效、price=0 免费、stock=0 售罄、
// user_limit=0 不限购、goods_id=0 不关联商品、can_use_coupon=false 不许叠加优惠。
//
// Skus 单独说明（这是最容易出事的一个）：
//
//	nil       → 不动规格（PATCH）
//	空数组    → **删除**该活动的全部规格（显式清空）
//	非空      → 按列表同步，未出现在列表里的规格会被删掉
//
// 改造前「不传 skus」与「传空数组」是同一个零值，于是「只改个标题」就把
// 规格整批删了，活动价与库存随之失效，而回执是 ok。
type ActivityRequest struct {
	Id           int64                  `json:"id"`              // 活动 ID，0 表示新建，>0 表示按 id 更新
	GoodsType    *string                `json:"goods_type"`      // 商品类型标记（≤16 字符），用于前台区分活动来源；后台表单不改它，通常留空
	StartTime    *int64                 `json:"start_time"`      // 活动开始时间（unix 秒）
	EndTime      *int64                 `json:"end_time"`        // 活动结束时间（unix 秒）
	UserLimit    *int64                 `json:"user_limit"`      // 每人限购数量，0 表示不限
	Title        string                 `json:"title"`           // 活动名称（空串视为「不改」：清空标题没有业务意义）
	Description  *string                `json:"description"`     // 活动介绍
	Images       *pq.StringArray        `json:"images"`          // 活动宣传图 URL 列表
	GoodsId      *int64                 `json:"goods_id"`        // 关联商品（文档）ID
	Price        *int64                 `json:"price"`           // 活动价（分）
	Stock        *int64                 `json:"stock"`           // 活动库存；传了 skus 时以 skus 的库存合计为准
	Extra        map[string]interface{} `json:"extra,omitempty"` // 附加设置对象（自定义扩展数据，原样存库）
	CanUseCoupon *bool                  `json:"can_use_coupon"`  // 是否允许与优惠券等其他优惠叠加
	Status       *uint                  `json:"status"`          // 活动状态：1 生效，0 失效

	// Skus 规格价格与库存。nil=不动、[]=清空全部、非空=按列表同步。
	Skus *[]ActivitySku `json:"skus"`

	// UpdateAll 走全量覆盖语义（后台表单提交）；Partial 是它的反向开关且优先，
	// 控制器用 `if !req.Partial { req.UpdateAll = true }` 归一，AI/MCP 通道注入 partial=true。
	UpdateAll bool `json:"update_all" ast:"-"`
	Partial   bool `json:"partial" ast:"-"`
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
