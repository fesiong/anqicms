package config

const Version = "2.2.8"

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
	CustomFieldTypeImage    = "image"
	CustomFieldTypeFile     = "file"
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

const (
	UrlTokenTypeFull = 0
	UrlTokenTypeSort = 1
)

const (
	StorageTypeLocal   = "local" // or empty
	StorageTypeAliyun  = "aliyun"
	StorageTypeTencent = "tencent"
	StorageTypeQiniu   = "qiniu"
	StorageTypeUpyun   = "upyun"
)
