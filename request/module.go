package request

import "kandaoni.com/anqicms/config"

type ModuleRequest struct {
	Id        uint                 `json:"id"`
	TableName string               `json:"table_name"`
	UrlToken  string               `json:"url_token"`
	Title     string               `json:"title"`
	Fields    []config.CustomField `json:"fields"`
	IsSystem  int                  `json:"is_system"`
	TitleName string               `json:"title_name"`
	Status    uint                 `json:"status"`
}

type ModuleFieldRequest struct {
	Id        uint   `json:"id"`
	FieldName string `json:"field_name"`
}
