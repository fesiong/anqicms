package request

type Category struct {
	Id             uint     `json:"id"`
	Title          string   `json:"title"`
	SeoTitle       string   `json:"seo_title"`
	Keywords       string   `json:"keywords"`
	Description    string   `json:"description"`
	Content        string   `json:"content"`
	ModuleId       uint     `json:"module_id"`
	ParentId       uint     `json:"parent_id"`
	Sort           uint     `json:"sort"`
	Status         uint     `json:"status"`
	Type           uint     `json:"type"`
	Template       string   `json:"template"`
	DetailTemplate string   `json:"detail_template"`
	UrlToken       string   `json:"url_token"`
	Force          bool     `json:"force"`
	Images         []string `json:"images"`
	Logo           string   `json:"logo"`
	IsInherit      uint     `json:"is_inherit"`

	Extra map[string]interface{} `json:"extra"`
}
