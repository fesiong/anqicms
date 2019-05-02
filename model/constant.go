package model

// 通用常量
const (
	// NoParent 无父结点时的parent_id
	NoParent = 0

	// MaxOrder 最大的排序号
	MaxOrder = 10000

	// MinOrder 最小的排序号
	MinOrder = 0

	// PageSize 默认每页的条数
	PageSize = 20

	// MaxPageSize 每页最大的条数
	MaxPageSize = 100

	// MinPageSize 每页最小的条数
	MinPageSize = 5

	// MaxNameLen 最大的名称长度
	MaxNameLen = 100

	// MaxContentLen 最大的内容长度
	MaxContentLen = 50000

	// MaxCategoryCount 最多可以属于几个分类
	MaxCategoryCount = 6
)

const (
	// ContentTypeMarkdown markdown
	ContentTypeMarkdown = 1

	// ContentTypeHTML html
	ContentTypeHTML = 2
)
