package request

// NavConfig 导航保存请求。指针字段语义见 request/archive.go 顶部注释：
// nil = 没提这个字段（保持原值），非 nil = 明确写入（含 0 / 空串）。
type NavConfig struct {
	Id          uint    `json:"id"`          // 导航 ID
	Title       string  `json:"title"`       // 导航标题（新建时必填，空值非法）
	SubTitle    *string `json:"sub_title"`   // 子标题；传空串表示清空
	Description *string `json:"description"` // 描述；传空串表示清空
	ParentId    *uint   `json:"parent_id"`   // 父级导航 ID；显式传 0 表示移到顶级
	NavType     *uint   `json:"nav_type"`    // 导航类型：0 系统导航，1 分类导航，2 外链导航，3 文档导航，4 城市站导航
	PageId      *int64  `json:"page_id"`     // 关联类型的 ID，如文档ID、分类ID、城市ID；显式传 0 表示解除关联
	TypeId      *uint   `json:"type_id"`     // 导航分组 ID；显式传 0 表示未分组
	Link        *string `json:"link"`        // 导航链接；传空串表示清空
	Sort        *uint   `json:"sort"`        // 排序；显式传 0 表示复位
	Style       *string `json:"style"`       // 样式（class）；传空串表示清空
	Status      *uint   `json:"status" ast:"-"`
	Logo        *string `json:"logo"` // 图标 URL；传空串表示清空
	UpdateAll   bool    `json:"update_all" ast:"-"`
	// Partial 走 PATCH 语义：只覆盖显式传入的字段，未传的一律保持原值。
	// 与 UpdateAll 互为反向开关，且优先——控制器用 req.UpdateAll = !req.Partial 归一。
	Partial bool `json:"partial" ast:"-"`
}

type NavTypeRequest struct {
	Id    uint   `json:"id"`    // 导航分组 ID
	Title string `json:"title"` // 导航分组名称
}

type DeleteNavRequest struct {
	Id uint `json:"id"` // 导航 ID
}
