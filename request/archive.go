package request

import "kandaoni.com/anqicms/config"

type Archive struct {
	Id           int64                  `json:"id"`
	ParentId     int64                  `json:"parent_id"`
	PlaceId      uint                   `json:"place_id"`
	Title        string                 `json:"title"`
	SeoTitle     string                 `json:"seo_title"`
	ModuleId     uint                   `json:"module_id"`
	CategoryId   uint                   `json:"category_id"`
	CategoryIds  []uint                 `json:"category_ids"`
	Keywords     string                 `json:"keywords"`
	Description  string                 `json:"description"`
	Content      string                 `json:"content"`
	Template     string                 `json:"template"`
	Images       []string               `json:"images"`
	Extra        map[string]interface{} `json:"extra"`
	CreatedTime  int64                  `json:"created_time"`
	UrlToken     string                 `json:"url_token"`
	Tags         []string               `json:"tags"`
	CanonicalUrl string                 `json:"canonical_url"`
	FixedLink    string                 `json:"fixed_link"`
	Flag         string                 `json:"flag"`
	Flags        []string               `json:"flags"`
	UserId       uint                   `json:"user_id"`
	Price        int64                  `json:"price"`
	Stock        int64                  `json:"stock"`
	ReadLevel    int                    `json:"read_level"` // 阅读关联 group level
	Password     string                 `json:"password"`
	Sort         uint                   `json:"sort"`         // 数值越大，越靠前
	Draft        bool                   `json:"draft"`        // 是否是存草稿
	RelationIds  []int64                `json:"relation_ids"` // 相关文档的ID
	Views        uint                   `json:"views"`        // 浏览量
	// 产品的字段
	OriginPrice    int64                `json:"origin_price"`
	CostPrice      int64                `json:"cost_price"`
	VideoUrl       string               `json:"video_url"`
	NeedLogistics  bool                 `json:"need_logistics"`
	IsFreeShipping bool                 `json:"is_free_shipping"`
	SoldCount      int64                `json:"sold_count"`
	ReviewCount    int64                `json:"review_count"`
	OptionType     int                  `json:"option_type"`
	Weight         float64              `json:"weight"`      // 如果填写了长宽高，并且设置了体积重系数，则自动计算重量
	WeightUnit     string               `json:"weight_unit"` // 如果填写了长宽高，并且设置了体积重系数，则自动使用重量单位kg
	Length         float64              `json:"length"`
	Width          float64              `json:"width"`
	Height         float64              `json:"height"`
	Volume         float64              `json:"volume"` // 固定是 m³，自动换算
	LengthUnit     string               `json:"length_unit"`
	Skus           []*ArchiveSku        `json:"skus"`
	Options        []*ArchiveOption     `json:"options"`
	ProductType    int                  `json:"product_type"`
	AllowOversold  bool                 `json:"allow_oversold"`
	WholesaleRules []*WholesaleRule     `json:"wholesale_rules"`
	OrderFields    []config.CustomField `json:"order_fields,omitempty"`
	MinQuantity    int                  `json:"min_quantity"`  // 最小购买数量
	MaxQuantity    int                  `json:"max_quantity"`  // 最大购买数量
	StepQuantity   int                  `json:"step_quantity"` // 购买数量步长

	RemoveTag         bool `json:"-"` // 是否删除标签
	RemoveImage       bool `json:"-"` // 是否删除图片
	RemoveFlag        bool `json:"-"` // 是否删除flag
	RemoveRelationIds bool `json:"-"` // 是否删除关联文档

	// 是否强制保存
	ForceSave  bool   `json:"force_save"`
	ToLanguage string `json:"to_language"`
	QuickSave  bool   `json:"quick_save"`
	UpdateAll  bool   `json:"update_all" ast:"-"`
	// Partial 走 PATCH 语义：只覆盖显式传入的字段，未传的一律保持原值。
	// 与 UpdateAll 互为反向开关，且优先——控制器用 req.UpdateAll = !req.Partial 归一。
	Partial bool `json:"partial" ast:"-"`

	KeywordId   uint   `json:"keyword_id"`
	OriginUrl   string `json:"origin_url"`
	OriginTitle string `json:"origin_title"`
	// 补充字段
	Brand         string  `json:"brand"`
	CommentCount  uint    `json:"comment_count"`
	FavoriteCount uint    `json:"favorite_count"`
	AverageScore  float32 `json:"average_score"`
	HasPseudo     int     `json:"has_pseudo"`
	OriginId      int     `json:"origin_id"`
	ContentText   string  `json:"-"`
}

type ArchiveSku struct {
	Id          int64                `json:"id"`
	ArchiveId   int64                `json:"archive_id"`
	Title       string               `json:"title"`     // sku 组合名字
	UrlToken    string               `json:"url_token"` // sku的token
	Sku         string               `json:"sku"`       // sku 货号
	Barcode     string               `json:"barcode"`   //  条形码
	HsCode      string               `json:"hs_code"`   //  海关编码
	Images      []string             `json:"images"`
	Stock       int64                `json:"stock"`
	Price       int64                `json:"price"`
	OriginPrice int64                `json:"origin_price"` // 原始价格
	CostPrice   int64                `json:"cost_price"`   // 成本价
	Weight      float64              `json:"weight"`       // 如果填写了长宽高，并且设置了体积重系数，则自动计算重量
	WeightUnit  string               `json:"weight_unit"`  // 如果填写了长宽高，并且设置了体积重系数，则自动使用重量单位kg
	Length      float64              `json:"length"`
	Width       float64              `json:"width"`
	Height      float64              `json:"height"`
	Volume      float64              `json:"volume"` // 固定是 m³，自动换算
	LengthUnit  string               `json:"length_unit"`
	Values      []ArchiveOptionValue `json:"values"`
	Status      int                  `json:"status"` // 状态,0:下架,1:上架
}

type ArchiveOptionValue struct {
	Id          int64    `json:"id"`
	ArchiveId   int64    `json:"archive_id"`
	OptionId    int64    `json:"option_id"`
	Title       string   `json:"title"`
	Color       string   `json:"color"`
	Images      []string `json:"images"`
	Price       int64    `json:"price"` // 加价，sku加价模式使用
	Stock       int64    `json:"stock"` // sku加价模式使用
	IsDefault   bool     `json:"is_default"`
	OptionTitle string   `json:"option_title"`
}

type ArchiveOption struct {
	Id          int64                 `json:"id"`
	ArchiveId   int64                 `json:"archive_id"`
	Title       string                `json:"title"`
	IsMain      uint                  `json:"is_main"`      // 是否主规格
	IsAttribute bool                  `json:"is_attribute"` // 是否是属性，属性不参与sku计算
	Values      []*ArchiveOptionValue `json:"values"`
}

type WholesaleRule struct {
	Id           int64 `json:"id"`
	ArchiveId    int64 `json:"archive_id"`
	MinQuantity  int   `json:"min_quantity"`
	MaxQuantity  int   `json:"max_quantity"`
	DiscountRate int   `json:"discount_rate"`
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
