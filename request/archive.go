package request

import "kandaoni.com/anqicms/config"

type Archive struct {
	Id           int64                  `json:"id"`
	ParentId     int64                  `json:"parent_id"`
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

	// 是否强制保存
	ForceSave  bool   `json:"force_save"`
	ToLanguage string `json:"to_language"`
	QuickSave  bool   `json:"quick_save"`

	KeywordId   uint   `json:"keyword_id"`
	OriginUrl   string `json:"origin_url"`
	OriginTitle string `json:"origin_title"`
	ContentText string `json:"-"`
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
	Id         int64 `json:"id"`
	ImageIndex int   `json:"image_index"`
}

type ArchiveReplaceRequest struct {
	Replace        bool                    `json:"replace"`
	ContentReplace []config.ReplaceKeyword `json:"content_replace"`
}

type ArchivesUpdateRequest struct {
	Ids         []int64 `json:"ids"`
	ParentId    int64   `json:"parent_id"`
	CategoryId  uint    `json:"category_id"`
	CategoryIds []uint  `json:"category_ids"`
	Status      uint    `json:"status"`
	Flag        string  `json:"flag"`
	Time        uint    `json:"time"`
	DailyLimit  int     `json:"daily_limit"` //每日限额
	StartHour   int     `json:"start_hour"`  //每天开始时间
	EndHour     int     `json:"end_hour"`    //每天结束时间
}

type ArchivePasswordRequest struct {
	Id       int64  `json:"id"`
	Password string `json:"password"`
}

type QuickImportArchiveRequest struct {
	FileName        string   `form:"file_name"`
	Md5             string   `form:"md5"`
	Chunk           int      `form:"chunk"`
	Chunks          int      `form:"chunks"`
	CategoryId      uint     `form:"category_id"`
	TitleType       int      `form:"title_type"` // 1来自内容，0来自文件标题
	Size            int64    `form:"size"`
	PlanType        int      `form:"plan_type"`
	PlanStart       int      `form:"plan_start"`      // 计划开始时间 0 立即 1 跟随最后一篇 2 半小时 3 1小时 4 2小时 5 4小时 6 8小时 7 12小时 8 24小时
	Days            int      `form:"days"`            // 分成多少天发布
	CheckDuplicate  bool     `form:"check_duplicate"` // 是否检查重复标题
	InsertImage     int      `form:"insert_image"`    // 0 不插入，1不插入，2 自定义插入图片 3 从图片分类里插入
	Images          []string `form:"images[]"`
	ImageCategoryId int      `form:"image_category_id"`
}

type ArchiveFavoriteRequest struct {
	ArchiveId  int64   `json:"archive_id"`
	SkuId      int64   `json:"sku_id"`
	ArchiveIds []int64 `json:"archive_ids"`
}
