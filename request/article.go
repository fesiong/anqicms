package request

import "irisweb/config"

type Article struct {
	Id           uint                   `json:"id"`
	Title        string                 `json:"title"`
	CategoryName string                 `json:"category_name"`
	CategoryId   uint                   `json:"category_id"`
	Keywords     string                 `json:"keywords"`
	Description  string                 `json:"description"`
	Content      string                 `json:"content"`
	Template     string                 `json:"template"`
	Images       []string               `json:"images"`
	Extra        map[string]interface{} `json:"extra"`

	KeywordId   uint   `json:"keyword_id"`
	OriginUrl   string `json:"origin_url"`
	OriginTitle string `json:"origin_title"`
	ContentText string `json:"-" gorm:"-"`
}

type ArticleExtraFieldsSetting struct {
	Fields []*config.CustomField `json:"fields"`
}

type ArticleReplaceRequest struct {
	ContentReplace []string `json:"content_replace"`
}
