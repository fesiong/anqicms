package request

// CouponRequest 优惠券保存请求。
// ⚠️ 金额字段（money/max_money/need_money）单位一律是**分**，后台表单显示的是元。
//
// 零值语义（2026-10-29 指针化）：除 Id/Title 外全部是**指针**。
// nil = 调用方没提这个字段（PATCH 语义，保持库里原值）；
// 非 nil = 显式写入，包含 0 —— 而 0 在这里几乎都是有意义的：
// status=0 停用、expire=0 跟随固定起止时间、need_money=0 无门槛、
// need_integral=0 免费兑换、range_type=0 全站可用、max_money=0 不封顶。
//
// 为什么不保留值类型：用 `req.Money > 0` 判断「要不要写」会让
// 「把满减券改成无门槛（need_money=0）」写不进去，而逐字段无条件赋值
// 又会让「只改标题」把面额清零 —— 而面额清零的券还挂着已发出的券码。
type CouponRequest struct {
	Id           int64   `json:"id"`            // 优惠券 ID，0 表示新建，>0 表示按 id 更新
	Money        *int64  `json:"money"`         // 面额（分）；money_type=2 时填百分比数值
	StartTime    *int64  `json:"start_time"`    // 生效开始时间（unix 秒），0 表示不限制
	EndTime      *int64  `json:"end_time"`      // 生效结束时间（unix 秒），0 表示不限制
	Title        *string `json:"title"`         // 优惠券标题
	Description  *string `json:"description"`   // 使用说明
	Expire       *int64  `json:"expire"`        // 领取后有效天数，0 表示跟随 start_time/end_time
	Status       *int    `json:"status"`        // 1 启用，0 停用
	Type         *int    `json:"type"`          // 券类型：1 注册送 2 限量领取 3 下单赠送 4 促销满减(无需领取) 5 自由规则 6 积分兑换 7 活动奖品
	MaxMoney     *int64  `json:"max_money"`     // 随机/百分比券的最高抵扣金额（分）
	MoneyType    *int    `json:"money_type"`    // 金额类型：0 固定金额 1 随机金额 2 百分比
	NeedIntegral *int64  `json:"need_integral"` // 兑换所需积分（type=6 时使用）
	NeedMoney    *int64  `json:"need_money"`    // 使用门槛金额（分），0 表示无门槛
	RangeType    *int    `json:"range_type"`    // 适用范围：0 全部商品 1 仅指定商品可用 2 指定商品不可用

	// UpdateAll 走全量覆盖语义（后台表单提交：字段没出现 = 用户清空了它）。
	// Partial 是它的反向开关，且优先——控制器用 `if !req.Partial { req.UpdateAll = true }` 归一。
	// AI/MCP 通道会注入 partial=true，于是「没传 = 别碰」。
	UpdateAll bool `json:"update_all" ast:"-"`
	Partial   bool `json:"partial" ast:"-"`
}

// CouponCodeRequest 券码保存/删除请求。
//
// 零值语义：UserId/StartTime/EndTime/Status/Money 为指针（0 都是有意义的值：
// 未绑定会员、跟随优惠券有效期、标记失效、0 元券码）。
// Code 保留值类型 —— 空串的语义是「服务端自动生成」，不是「清空券码」。
type CouponCodeRequest struct {
	Id        int64   `json:"id"`         // 券码 ID，0 表示新建
	UserId    *int64  `json:"user_id"`    // 归属会员 ID，0 表示未绑定会员
	CouponId  int64   `json:"coupon_id"`  // 所属优惠券 ID
	Code      string  `json:"code"`       // 券码串，留空则服务端自动生成
	StartTime *int64  `json:"start_time"` // 券码级生效开始时间（unix 秒），0 表示跟随优惠券
	EndTime   *int64  `json:"end_time"`   // 券码级生效结束时间（unix 秒），0 表示跟随优惠券
	Status    *int    `json:"status"`     // 1 可用，0 已使用或已失效
	Money     *int64  `json:"money"`      // 该券码面额（分），覆盖优惠券默认面额
	Ids       []int64 `json:"ids"`        // 批量删除的券码 ID 列表（code/delete 用）
}

// CouponArchiveRequest 优惠券适用商品请求。
type CouponArchiveRequest struct {
	CouponId   int64   `json:"coupon_id"`   // 优惠券 ID
	ArchiveId  int64   `json:"archive_id"`  // 单个商品（文档）ID
	ArchiveIds []int64 `json:"archive_ids"` // 商品 ID 列表（批量添加或移除）
}

// MakeCouponCodeRequest 批量生成券码请求。
type MakeCouponCodeRequest struct {
	CouponId int64 `json:"coupon_id"` // 为哪张优惠券生成券码
	Count    int   `json:"count"`     // 生成张数
	UserId   int64 `json:"user_id"`   // 绑定到指定会员 ID，0 表示不绑定
}
