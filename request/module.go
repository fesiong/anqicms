package request

import "kandaoni.com/anqicms/config"

type ModuleRequest struct {
	Id             uint                 `json:"id"`
	TableName      string               `json:"table_name"`
	UrlToken       string               `json:"url_token"`
	Title          string               `json:"title"`
	Name           string               `json:"name"`
	Keywords       string               `json:"keywords"`
	Description    string               `json:"description"`
	Fields         []config.CustomField `json:"fields"`
	CategoryFields []config.CustomField `json:"category_fields"`
	IsSystem       int                  `json:"is_system"`
	TitleName      string               `json:"title_name"`
	Status         uint                 `json:"status"`
}

type ModuleFieldRequest struct {
	Id        uint   `json:"id"`
	FieldName string `json:"field_name"`
}
