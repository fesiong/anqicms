package request

type NavConfig struct {
	Id          uint   `json:"id"`          // 导航 ID
	Title       string `json:"title"`       // 导航标题
	SubTitle    string `json:"sub_title"`   // 子标题
	Description string `json:"description"` // 描述
	ParentId    uint   `json:"parent_id"`   // 父级导航 ID
	NavType     uint   `json:"nav_type"`    // 导航类型：0 系统导航，1 分类导航，2 外链导航，3 文档导航，4 城市站导航
	PageId      int64  `json:"page_id"`     // 关联类型的 ID，如文档ID、分类ID、城市ID
	TypeId      uint   `json:"type_id"`     // 导航分组 ID
	Link        string `json:"link"`        // 导航链接
	Sort        uint   `json:"sort"`        // 排序
	Style       string `json:"style"`       // 样式（class）
	Status      uint   `json:"status" ast:"-"`
	Logo        string `json:"logo"` // 图标 URL
	UpdateAll   bool   `json:"update_all" ast:"-"`
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
