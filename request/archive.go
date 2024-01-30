package request

import "kandaoni.com/anqicms/config"

type Archive struct {
	Id           uint                   `json:"id"`
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
	Sort         uint                   `json:"sort"`  // 数值越大，越靠前
	Draft        bool                   `json:"draft"` // 是否是存草稿

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
	Id         uint `json:"id"`
	ImageIndex int  `json:"image_index"`
}

type ArchiveReplaceRequest struct {
	Replace        bool                    `json:"replace"`
	ContentReplace []config.ReplaceKeyword `json:"content_replace"`
}

type ArchivesUpdateRequest struct {
	Ids []uint `json:"ids"`

	CategoryId  uint   `json:"category_id"`
	CategoryIds []uint `json:"category_ids"`
	Status      uint   `json:"status"`
	Flag        string `json:"flag"`
	Time        uint   `json:"time"`
	DailyLimit  int    `json:"daily_limit"` //每日限额
	StartHour   int    `json:"start_hour"`  //每天开始时间
	EndHour     int    `json:"end_hour"`    //每天结束时间
}

type ArchivePasswordRequest struct {
	Id       uint   `json:"id"`
	Password string `json:"password"`
}
