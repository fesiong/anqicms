package library

// 指针取值辅助函数。
//
// 为什么需要它们：后台的保存类请求结构体统一用指针区分「调用方没传这个字段」
// （nil，保持库里原值）与「调用方显式传了零值」（非 nil，写 0 / 写空串）。
// 没有指针时二者无法区分，只能退化成 `if req.X > 0` 这种守卫，于是
// 「库存改成 0（售罄）」「状态改成 0（禁用）」「把 SEO 标题清空」全都写不进去，
// 而回执依然是成功——这是本仓反复出现的静默失效形态。
//
// 注意：指针只用于**请求结构体**，model 层保持值类型（GORM 列有默认值语义）。

// Ptr 返回 v 的地址，用于把常量/字面量填进指针字段。
func Ptr[T any](v T) *T {
	return &v
}

// Deref 解引用，nil 时返回类型零值。
//
// 只在「nil 与零值等价」的场景使用；若 nil 与零值语义不同（例如
// stock 的 nil 表示"不修改"、0 表示"售罄"），必须显式判 nil，不要用 Deref。
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// DerefOr 解引用，nil 时返回 def。
func DerefOr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// Assign 把请求里的指针值写进目标字段，统一「全量覆盖」与「局部更新」两种语义：
//
//	非 nil       → 写入（含 0 / false / 空串，这是「售罄」「禁用」「清空」的唯一入口）
//	nil + force  → 写零值（表单全量提交：没出现在表单里 = 用户清空了它）
//	nil + !force → 不动（PATCH：调用方没提这个字段）
//
// force 传 req.UpdateAll。原来这段逻辑在每个保存函数里手写一遍
// （`if req.X != nil { … } else if req.UpdateAll { … }`），
// 抄漏一个分支就是静默清空或静默不写，所以收敛成一个函数。
func Assign[T any](dst *T, src *T, force bool) {
	if src != nil {
		*dst = *src
		return
	}
	if force {
		var zero T
		*dst = zero
	}
}
