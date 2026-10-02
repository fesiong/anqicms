package request

import "kandaoni.com/anqicms/config"

type Archive struct {
	Id           int64                  `json:"id"`            // 文档 ID
	ParentId     int64                  `json:"parent_id"`     // 父级文档 ID
	PlaceId      uint                   `json:"place_id"`      // 城市站 ID
	Title        string                 `json:"title"`         // 文档标题
	SeoTitle     string                 `json:"seo_title"`     // 文档的 SEO 标题
	ModuleId     uint                   `json:"module_id"`     // 模型 ID
	CategoryId   uint                   `json:"category_id"`   // 文档分类 ID
	CategoryIds  []uint                 `json:"category_ids"`  // 文档多分类 ID（启用文档多分类后使用）
	Keywords     string                 `json:"keywords"`      // 文档关键词
	Description  string                 `json:"description"`   // 文档描述
	Content      string                 `json:"content"`       // 文档内容
	Template     string                 `json:"template"`      // 指定文档使用的模板
	Images       []string               `json:"images"`        // 文档图片集
	Extra        map[string]interface{} `json:"extra"`         // 文档扩展字段（模型配置的字段）
	CreatedTime  int64                  `json:"created_time"`  // 文档创建时间
	UrlToken     string                 `json:"url_token"`     // 文档 URL 别名
	Tags         []string               `json:"tags"`          // 文档标签
	CanonicalUrl string                 `json:"canonical_url"` // 文档规范的链接（只有要将文档指向到另外的页面，才需要在这里填写）
	FixedLink    string                 `json:"fixed_link"`    // 文档固定链接（只有你想把文档的链接持久固定，不随伪静态规则改变，才需要填写）
	Flag         string                 `json:"flag"`          // 文档标记，默认无：h=头条、c=推荐、f=幻灯、a=特荐、s=滚动、h=加粗、p=图片、j=跳转
	Flags        []string               `json:"flags"`         // flags 和 flag 二选一
	UserId       uint                   `json:"user_id"`       // 文档作者 ID
	Price        int64                  `json:"price"`         // 文档价格（单位：分）
	Stock        int64                  `json:"stock"`         // 文档库存量（产品可用）
	ReadLevel    int                    `json:"read_level"`    // 阅读关联用户组 level
	Password     string                 `json:"password"`      // 文档密码（设置密码后，文档需要密码访问）
	Sort         uint                   `json:"sort"`          // 排序，数值越大，越靠前（开启文档排序配置后生效）
	Draft        bool                   `json:"draft"`         // 是否是草稿
	RelationIds  []int64                `json:"relation_ids"`  // 相关文档的ID
	Views        uint                   `json:"views"`         // 浏览量

	RemoveTag         bool `json:"-"` // 是否删除标签
	RemoveImage       bool `json:"-"` // 是否删除图片
	RemoveFlag        bool `json:"-"` // 是否删除flag
	RemoveRelationIds bool `json:"-"` // 是否删除关联文档

	ForceSave bool `json:"force_save"` // 在标题冲突时，是否强制保存
	QuickSave bool `json:"quick_save"` // 是否是快速保存，快速保存模式下仅处理标题、关键词、描述、分类、图片、标签、标记、内容
	UpdateAll bool `json:"update_all" ast:"-"`

	KeywordId   uint   `json:"keyword_id"`   // 关键词 ID，采集文档时使用
	OriginUrl   string `json:"origin_url"`   // 文档来源 URL，默认采集文档时使用
	OriginTitle string `json:"origin_title"` // 文档来源标题，默认采集文档时使用
	OriginId    int    `json:"origin_id"`    // 文档来源 ID，默认采集文档时使用
	ContentText string `json:"-"`
}

type ArchiveImageDeleteRequest struct {
	Id         int64 `json:"id"`          // 文档 ID
	ImageIndex int   `json:"image_index"` // 图片索引
}

type ArchiveReplaceRequest struct {
	Replace        bool                    `json:"replace"`                             // 是否立即执行替换任务
	ContentReplace []config.ReplaceKeyword `json:"content_replace" validate:"required"` // 内容替换关键词列表。
}

type ArchiveFlagsRequest struct {
	Ids  []int64 `json:"ids"`  // 文档 ID
	Flag string  `json:"flag"` // 标记，支持多个，多个用逗号隔开
}

type ArchiveStatusRequest struct {
	Ids    []int64 `json:"ids"`    // 文档 ID
	Status uint    `json:"status"` // 状态, 0 草稿，1 正式文档
}

type ArchivesTimeRequest struct {
	Ids  []int64 `json:"ids"`                      // 文档 ID
	Time uint    `json:"time" validate:"required"` // 时间类型：1 更新选中文档的 created_time 2 更新选中文档 的 updated_time，3 更新所有文档的 created_time，4 更新所有文档的updated_time
}

type ArchivesPlanRequest struct {
	Ids        []int64 `json:"ids" validate:"required"` // 文档 ID
	DailyLimit int     `json:"daily_limit"`             //每日发布数量
	StartHour  int     `json:"start_hour"`              //每天开始时间(时)，0-23
	EndHour    int     `json:"end_hour"`                //每天结束时间(时)，0-23

}

type ArchiveSortRequest struct {
	Id   int64 `json:"id" validate:"required"` // 文档 ID
	Sort uint  `json:"sort"`                   // 排序值，越大越靠前
}

type ArchivesParentRequest struct {
	Ids      []int64 `json:"ids"`       // 文档 ID
	ParentId int64   `json:"parent_id"` // 父级文档 ID
}

type ArchiveCategoryRequest struct {
	Ids         []int64 `json:"ids"`          // 文档 ID
	CategoryId  uint    `json:"category_id"`  // 文档分类 ID
	CategoryIds []uint  `json:"category_ids"` // 文档多分类 ID（启用文档多分类后使用）
}

type ArchivesTagRequest struct {
	Ids  []int64  `json:"ids"`  // 文档 ID
	Tags []string `json:"tags"` // 文档标签
}

type ArchiveRecoverRequest struct {
	Id int64 `json:"id"` // 文档 ID
}

type ArchivePasswordRequest struct {
	Id       int64  `json:"id"`
	Password string `json:"password"`
}

type QuickImportArchiveRequest struct {
	FileName        string   `form:"file_name"`         // 上传分片对应的原始文件名
	Md5             string   `form:"md5"`               // 文件 MD5（用于校验分片完整性）
	Chunk           int      `form:"chunk"`             // 当前分片序号（从 0 开始）
	Chunks          int      `form:"chunks"`            // 总分片数
	CategoryId      uint     `form:"category_id"`       // 导入后归属的分类 ID
	TitleType       int      `form:"title_type"`        // 1来自内容，0来自文件标题
	Size            int64    `form:"size"`              // 文件总大小（字节）
	PlanType        int      `form:"plan_type"`         // 发布计划类型（0 立即，1 计划发布）
	PlanStart       int      `form:"plan_start"`        // 计划开始时间 0 立即 1 跟随最后一篇 2 半小时 3 1小时 4 2小时 5 4小时 6 8小时 7 12小时 8 24小时
	Days            int      `form:"days"`              // 分成多少天发布
	CheckDuplicate  bool     `form:"check_duplicate"`   // 是否检查重复标题
	InsertImage     int      `form:"insert_image"`      // 0 不插入，1不插入，2 自定义插入图片 3 从图片分类里插入
	Images          []string `form:"images[]"`          // 自定义插入的图片地址列表
	ImageCategoryId int      `form:"image_category_id"` // 从图片分类插入图片时使用的图片分类 ID
}

type ArchiveFavoriteRequest struct {
	ArchiveId  int64   `json:"archive_id"`
	SkuId      int64   `json:"sku_id"`
	ArchiveIds []int64 `json:"archive_ids"`
}

type ImportExcelTemplateRequest struct {
	CategoryId uint `json:"category_id" validate:"required"` // 文档分类ID
}
