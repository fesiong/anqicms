package request

import "kandaoni.com/anqicms/config"

type ModuleRequest struct {
	Id             uint                 `json:"id"`              // 模型 ID
	TableName      string               `json:"table_name"`      // 模型扩展表名
	UrlToken       string               `json:"url_token"`       // URL 别名
	Title          string               `json:"title"`           // 模型名称
	Name           string               `json:"name"`            // 模型标识
	Keywords       string               `json:"keywords"`        // 模型关键词
	Description    string               `json:"description"`     // 模型描述
	Fields         []config.CustomField `json:"fields"`          // 文档扩展字段
	CategoryFields []config.CustomField `json:"category_fields"` // 分类扩展字段
	IsSystem       int                  `json:"is_system"`       // 是否内置模型
	TitleName      string               `json:"-" ast:"-"`       // 已废弃
	Status         uint                 `json:"status"`          // 模型的启用状态：0 禁用，1 启用
	UpdateAll      bool                 `json:"update_all" ast:"-"`
	// Partial 走 PATCH 语义：只覆盖显式传入的字段，未传的一律保持原值。
	// 与 UpdateAll 互为反向开关，且优先——控制器用 req.UpdateAll = !req.Partial 归一。
	Partial bool `json:"partial" ast:"-"`
}

type ModuleFieldRequest struct {
	Id        uint   `json:"id"`         // 模型 ID
	FieldName string `json:"field_name"` // 字段标识
}

type DeleteModuleRequest struct {
	Id uint `json:"id"` // 模型 ID
}
