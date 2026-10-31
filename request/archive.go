package request

import "kandaoni.com/anqicms/config"

// Archive 是文档保存/查询的统一请求体，被 archive（文章/产品/自定义模型）共用。
//
// # 指针字段与零值语义（2026-10-29 整改）
//
// 除标题、正文、url_token 这类「空值即非法」的字段外，其余一律用指针：
//
//	nil     —— 调用方没提这个字段，保持库里原值（PATCH 语义，AI/程序化调用默认走这条）
//	非 nil  —— 调用方明确要求写入该值，**包括 0、false、空字符串**
//
// 为什么必须这样：反过来的写法（`if req.X > 0` 才写）会让「库存改 0（售罄）」
// 「价格改 0（免费）」「状态改 0（禁用）」「清空 SEO 标题」全部写不进去，
// 而接口照样回 ok —— 调用方以为改成功，实际库里没变。这是本仓最容易复发的
// 静默失效形态（护栏见 provider/archive_zero_guard_test.go）。
//
// 保持值类型的字段及理由：Title/Content（空值非法）、UrlToken（空=按标题生成）、
// ModuleId（0=由分类推导）、CategoryId（必填）、CreatedTime（0=当前时间）。
type Archive struct {
	Id           int64                  `json:"id"`            // 文档 ID
	ParentId     *int64                 `json:"parent_id"`     // 父级文档 ID；显式传 0 表示移到根级
	PlaceId      *uint                  `json:"place_id"`      // 城市站 ID；显式传 0 表示解除绑定
	Title        string                 `json:"title"`         // 文档标题（必填，空值非法）
	SeoTitle     *string                `json:"seo_title"`     // 文档的 SEO 标题；传空串表示清空（回落到标题）
	ModuleId     uint                   `json:"module_id"`     // 模型 ID（0 表示由分类推导）
	CategoryId   uint                   `json:"category_id"`   // 文档分类 ID
	CategoryIds  *[]uint                `json:"category_ids"`  // 文档多分类 ID；传空数组表示只保留 category_id
	Keywords     *string                `json:"keywords"`      // 文档关键词；传空串表示清空
	Description  *string                `json:"description"`   // 文档描述；传空串表示清空（不再自动截取正文）
	Content      string                 `json:"content"`       // 文档内容（空值非法，不会清空已有正文）
	Template     *string                `json:"template"`      // 指定文档使用的模板；传空串表示用默认模板
	Images       *[]string              `json:"images"`        // 文档图片集；**传空数组才清空**，不传则保持原图
	Extra        map[string]interface{} `json:"extra"`         // 文档扩展字段（模型配置的字段），按 key 存在性生效
	CreatedTime  int64                  `json:"created_time"`  // 文档创建时间（0 表示使用当前时间）
	UrlToken     string                 `json:"url_token"`     // 文档 URL 别名（空表示按标题自动生成）
	Tags         *[]string              `json:"tags"`          // 文档标签；传空数组表示清空标签
	CanonicalUrl *string                `json:"canonical_url"` // 文档规范的链接；传空串表示清空
	FixedLink    *string                `json:"fixed_link"`    // 文档固定链接；传空串表示清空
	Flag         string                 `json:"flag"`          // 文档标记（逗号分隔，兼容旧调用）；与 flags 二选一
	Flags        *[]string              `json:"flags"`         // 文档标记数组；传空数组表示清空标记
	UserId       *uint                  `json:"user_id"`       // 文档作者 ID；传 0 表示匿名
	Price        *int64                 `json:"price"`         // 价格（单位：分）；商品售价与付费阅读单价共用此列，传 0 = 免费
	Stock        *int64                 `json:"stock"`         // 库存量；传 0 = 售罄
	ReadLevel    *int                   `json:"read_level"`    // 阅读关联用户组 level；传 0 表示取消限制
	Password     *string                `json:"password"`      // 文档密码；传空串表示取消密码访问
	Sort         *uint                  `json:"sort"`          // 排序，数值越大越靠前；传 0 表示复位
	Draft        bool                   `json:"draft"`         // 是否是草稿
	RelationIds  *[]int64               `json:"relation_ids"`  // 相关文档的ID；传空数组表示清空关联
	Views        *uint                  `json:"views"`         // 浏览量；传 0 表示清零（导入/修数据用）
	// 产品的字段
	OriginPrice    *int64                `json:"origin_price"`           // 产品原价（划线价），单位：分
	CostPrice      *int64                `json:"cost_price"`             // 成本价，单位：分
	VideoUrl       *string               `json:"video_url"`              // 视频主图地址；传空串表示清空
	NeedLogistics  *bool                 `json:"need_logistics"`         // 是否需要物流配送
	IsFreeShipping *bool                 `json:"is_free_shipping"`       // 是否包邮（免运费）
	SoldCount      *int64                `json:"sold_count"`             // 已售数量，由订单流程累加；只在显式传入时写入
	ReviewCount    *int64                `json:"review_count"`           // 评价数量，系统统计；只在显式传入时写入
	OptionType     *int                  `json:"option_type"`            // SKU 模式：0=无 SKU、1=组合 SKU、2=加价 SKU
	Weight         *float64              `json:"weight"`                 // 如果填写了长宽高，并且设置了体积重系数，则自动计算重量
	WeightUnit     *string               `json:"weight_unit"`            // 重量单位 kg/g/lb；传空串表示清空
	Length         *float64              `json:"length"`                 // 长（用户配置的长度单位）
	Width          *float64              `json:"width"`                  // 宽（用户配置的长度单位）
	Height         *float64              `json:"height"`                 // 高（用户配置的长度单位）
	Volume         *float64              `json:"volume"`                 // 固定是 m³，自动换算
	LengthUnit     *string               `json:"length_unit"`            // 长度单位 m/cm/inch；传空串表示清空
	Skus           []*ArchiveSku         `json:"skus"`                   // SKU 规格列表；只有 update_all=true 或 options 非空时才按整体替换保存
	Options        []*ArchiveOption      `json:"options"`                // SKU 选项（规格维度）定义；不传表示不改动已有规格
	ProductType    *int                  `json:"product_type"`           // 产品类型：1=普通商品、2=批发、3=定制、4=虚拟、5=服务、6=订阅
	AllowOversold  *bool                 `json:"allow_oversold"`         // 是否允许超售（库存为 0 仍可下单）
	WholesaleRules []*WholesaleRule      `json:"wholesale_rules"`        // 批发价规则；只有 update_all=true 或该数组非空时才保存
	OrderFields    *[]config.CustomField `json:"order_fields,omitempty"` // 下单需填写的自定义字段；只有 product_type=3（定制）时写入，其它类型会被清空
	MinQuantity    *int                  `json:"min_quantity"`           // 最小购买数量
	MaxQuantity    *int                  `json:"max_quantity"`           // 最大购买数量
	StepQuantity   *int                  `json:"step_quantity"`          // 购买数量步长

	ForceSave  bool   `json:"force_save"`         // 在标题冲突时，是否强制保存
	ToLanguage string `json:"to_language"`        // 多语言站点的译文语言代码（如 en）；当前保存流程不读取该字段
	QuickSave  bool   `json:"quick_save"`         // 是否是快速保存，快速保存模式下仅处理标题、关键词、描述、分类、图片、标签、标记、内容
	UpdateAll  bool   `json:"update_all" ast:"-"` // 是否整体覆盖：为 true 时才会把空的 skus/options/wholesale_rules 一起保存（即清空）
	// Partial 走 PATCH 语义：只覆盖显式传入的字段，未传的一律保持原值。
	// 与 UpdateAll 互为反向开关，且优先——控制器用 req.UpdateAll = !req.Partial 归一。
	// 自 2026-10-29 起，标量字段一律靠指针是否为 nil 判断，**不再依赖 UpdateAll**；
	// UpdateAll 现在只影响列表类字段（skus/options/wholesale_rules）与 Extra 的补默认值。
	Partial bool `json:"partial" ast:"-"`

	KeywordId   *uint  `json:"keyword_id"`   // 关键词 ID，采集文档时使用
	OriginUrl   string `json:"origin_url"`   // 文档来源 URL，采集文档时使用（空值无意义，不提供清空语义）
	OriginTitle string `json:"origin_title"` // 文档来源标题，采集文档时使用（空值无意义，不提供清空语义）
	// 补充字段
	Brand         *string  `json:"brand"`          // 品牌；传空串表示清空
	CommentCount  *uint    `json:"comment_count"`  // 评论数量，系统统计；只在显式传入时写入
	FavoriteCount *uint    `json:"favorite_count"` // 收藏数量，系统统计；只在显式传入时写入
	AverageScore  *float32 `json:"average_score"`  // 平均评分，系统统计；只在显式传入时写入
	HasPseudo     *int     `json:"has_pseudo"`     // 内容是否经 AI 伪原创改写：0=否、1=是
	OriginId      *int     `json:"origin_id"`      // 来源类型：0=默认、1=采集、2=AI 生成（显式 0 表示清除来源标记）
	ContentText   string   `json:"-"`
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
