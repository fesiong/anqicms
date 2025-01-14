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

	// 是否强制保存
	ForceSave  bool   `json:"force_save"`
	ToLanguage string `json:"to_language"`
	QuickSave  bool   `json:"quick_save"`

	KeywordId   uint   `json:"keyword_id"`
	OriginUrl   string `json:"origin_url"`
	OriginTitle string `json:"origin_title"`
	ContentText string `json:"-"`
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
