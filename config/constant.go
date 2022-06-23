package config

const Version = "2.1.9"

const (
	StatusOK         = 0
	StatusFailed     = -1
	StatusNoLogin    = 1001
	StatusApiSuccess = 200
)

const (
	CustomFieldTypeText     = "text"
	CustomFieldTypeNumber   = "number"
	CustomFieldTypeTextarea = "textarea"
	CustomFieldTypeRadio    = "radio"
	CustomFieldTypeCheckbox = "checkbox"
	CustomFieldTypeSelect   = "select"
)

const (
	CategoryTypeArchive = 1
	CategoryTypePage    = 3
)

const (
	ContentStatusDraft = 0 // 草稿
	ContentStatusOK    = 1 // 正式内容
	ContentStatusPlan  = 2 // 计划内容，等待发布
)
