package request

import "irisweb/config"

type Product struct {
	Id           uint                   `json:"id"`
	Title        string                 `json:"title"`
	CategoryName string                 `json:"category_name"`
	CategoryId   uint                   `json:"category_id"`
	Keywords     string                 `json:"keywords"`
	Description  string                 `json:"description"`
	Content      string                 `json:"content"`
	Price        float64                `json:"price"`
	Stock        uint                   `json:"stock"`
	Images       []string               `json:"images"`
	Extra        map[string]interface{} `json:"extra"`
}

type ProductThumb struct {
	Id  uint   `json:"id"`
	Src string `json:"src"`
}

type ProductExtraFieldsSetting struct {
	Fields []*config.CustomField `json:"fields"`
}
