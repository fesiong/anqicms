package request

// Category 的分类保存请求。
//
// 指针字段语义与 request.Archive 一致：nil = 没提这个字段（保持原值），
// 非 nil = 明确要求写入（含 0 / 空串）。理由见 request/archive.go 顶部注释。
type Category struct {
	Id             uint      `json:"id"`              // 分类 ID
	Title          string    `json:"title"`           // 分类名称（必填，空值非法）
	SeoTitle       *string   `json:"seo_title"`       // 分类 SEO 标题；传空串表示清空
	Keywords       *string   `json:"keywords"`        // 分类关键字；传空串表示清空
	Description    *string   `json:"description"`     // 分类描述；传空串表示清空
	Content        *string   `json:"content"`         // 分类内容（单页型分类的正文）；传空串表示清空
	ModuleId       uint      `json:"module_id"`       // 模型 ID（0 表示由分类类型推导）
	ParentId       *uint     `json:"parent_id"`       // 父级分类 ID；显式传 0 表示移到顶级
	Sort           *uint     `json:"sort"`            // 排序，数值小的在前；显式传 0 表示复位
	Status         *uint     `json:"status"`          // 状态：1 正常，0 隐藏
	Type           uint      `json:"type"`            // 类型：1 文档分类，3 单页（0 非法）
	Template       *string   `json:"template"`        // 分类文档列表页模板；传空串表示用默认模板
	DetailTemplate *string   `json:"detail_template"` // 分类文档详情页模板；传空串表示用默认模板
	UrlToken       string    `json:"url_token"`       // 分类 URL 别名（空表示自动生成）
	Force          bool      `json:"force"`           // URL别名重复时，是否强制更新
	Images         *[]string `json:"images"`          // 分类图片组；传空数组表示清空
	Logo           *string   `json:"logo"`            // 分类图标；传空串表示清空
	IsInherit      *uint     `json:"is_inherit" ast:"-"`
	UpdateAll      bool      `json:"update_all" ast:"-"`
	// Partial 走 PATCH 语义：只覆盖显式传入的字段，未传的一律保持原值。
	// 与 UpdateAll 互为反向开关，且优先——控制器用 req.UpdateAll = !req.Partial 归一。
	Partial bool `json:"partial" ast:"-"`

	Extra map[string]interface{} `json:"extra"` // 扩展字段（模型配置的字段）
}

type CategoryDeleteRequest struct {
	Id uint `json:"id"` // 分类 ID
}
