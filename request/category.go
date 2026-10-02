package request

type Category struct {
	Id             uint     `json:"id"`              // 分类 ID
	Title          string   `json:"title"`           // 分类名称
	SeoTitle       string   `json:"seo_title"`       // 分类 SEO 标题
	Keywords       string   `json:"keywords"`        // 分类关键字
	Description    string   `json:"description"`     // 分类描述
	Content        string   `json:"content"`         // 分类内容
	ModuleId       uint     `json:"module_id"`       // 模型 ID
	ParentId       uint     `json:"parent_id"`       // 父级分类 ID
	Sort           uint     `json:"sort"`            // 排序，数值小的在前
	Status         uint     `json:"status"`          // 状态：1 正常，0 隐藏
	Type           uint     `json:"type"`            // 类型：1 文档分类，3 单页
	Template       string   `json:"template"`        // 分类文档列表页模板
	DetailTemplate string   `json:"detail_template"` // 分类文档详情页模板
	UrlToken       string   `json:"url_token"`       // 分类 URL 别名
	Force          bool     `json:"force"`           // URL别名重复时，是否强制更新
	Images         []string `json:"images"`          // 分类图片组
	Logo           string   `json:"logo"`            // 分类图标
	IsInherit      uint     `json:"is_inherit" ast:"-"`
	UpdateAll      bool     `json:"update_all" ast:"-"`

	Extra map[string]interface{} `json:"extra"` // 扩展字段（模型配置的字段）
}

type CategoryDeleteRequest struct {
	Id uint `json:"id"` // 分类 ID
}
