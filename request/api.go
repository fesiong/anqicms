package request

import "kandaoni.com/anqicms/model"

type ApiArchiveRequest struct {
	Id       int64  `json:"id"`
	UrlToken string `json:"url_token"`
	Render   bool   `json:"render"`
	Password string `json:"password"`
	UserId   uint   `json:"user_id"`

	UserGroup *model.UserGroup `json:"-"`
	UserInfo  *model.User      `json:"-"`
}

type ApiArchiveListRequest struct {
	Id                 int64    `json:"id"`
	Ids                []int64  `json:"ids"`
	Render             bool     `json:"render"`
	ParentId           int64    `json:"parent_id"`
	CategoryIds        []int    `json:"category_ids"`
	ExcludeCategoryIds []int    `json:"exclude_category_ids"`
	ExcludeFlags       []string `json:"exclude_flags"`
	ModuleId           int64    `json:"module_id"`
	AuthorId           int64    `json:"author_id"`
	ShowFlag           bool     `json:"show_flag"`
	ShowContent        bool     `json:"show_content"`
	ShowExtra          bool     `json:"show_extra"`
	ShowCategory       bool     `json:"show_category"`
	ShowTag            bool     `json:"show_tag"`
	Draft              bool     `json:"draft"`
	Child              bool     `json:"child"`
	Order              string   `json:"order"`
	Tag                string   `json:"tag"`
	TagId              int64    `json:"tag_id"`
	TagIds             []int64  `json:"tag_ids"`
	Flag               string   `json:"flag"`
	Q                  string   `json:"q"`
	Like               string   `json:"like"`
	Keywords           string   `json:"keywords"`
	Type               string   `json:"type"`
	Page               int      `json:"page"`
	Limit              int      `json:"limit"`
	Offset             int      `json:"offset"`
	UserId             uint     `json:"user_id"`
	CombineMode        string   `json:"combine_mode"`
	CombineId          int64    `json:"combine_id"`
	// 更多的参数筛选
	ExtraFields map[string]interface{} `json:"extra_fields"`
}

type ApiCategoryRequest struct {
	Id       int64  `json:"id"`
	UrlToken string `json:"url_token"`
	Render   bool   `json:"render"`
}

type ApiCategoryListRequest struct {
	ModuleId int64 `json:"module_id"`
	ParentId int64 `json:"parent_id"`
	All      bool  `json:"all"`
	Limit    int   `json:"limit"`
	Offset   int   `json:"offset"`
}

type ApiTagRequest struct {
	Id       int64  `json:"id"`
	UrlToken string `json:"url_token"`
	Render   bool   `json:"render"`
}

type ApiTagListRequest struct {
	ItemId      int64  `json:"item_id"`
	CategoryIds []int  `json:"category_ids"`
	Type        string `json:"type"`
	Letter      string `json:"letter"`
	Order       string `json:"order"`
	Page        int    `json:"page"`
	Limit       int    `json:"limit"`
	Offset      int    `json:"offset"`
}

type ApiFilterRequest struct {
	ModuleId     int64             `json:"module_id"`
	ShowAll      bool              `json:"show_all"`
	AllText      string            `json:"all_text"`
	ShowPrice    bool              `json:"show_price"`
	ShowCategory bool              `json:"show_category"`
	ParentId     int64             `json:"parent_id"`
	CategoryId   int64             `json:"category_id"`
	UrlParams    map[string]string `json:"url_params"`
}
