package response

type MultiLangSite struct {
	Id            uint   `json:"id"`
	RootPath      string `json:"root_path"`
	Name          string `json:"name"`
	Status        bool   `json:"status"`
	ParentId      uint   `json:"parent_id"`
	SyncTime      int64  `json:"sync_time"`
	LanguageIcon  string `json:"language_icon"` // 图标
	LanguageEmoji string `json:"language_emoji"`
	LanguageName  string `json:"language_name"`
	Language      string `json:"language"`
	IsCurrent     bool   `json:"is_current"`
	Link          string `json:"link"`
	BaseUrl       string `json:"base_url"`
	ErrorMsg      string `json:"error_msg"`
}
