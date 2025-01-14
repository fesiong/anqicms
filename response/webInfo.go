package response

type WebInfo struct {
	Title        string `json:"title"`
	Keywords     string `json:"keywords"`
	Description  string `json:"description"`
	NavBar       int64  `json:"nav_bar"`
	PageId       int64  `json:"-"`
	PageName     string `json:"page_name"`
	CanonicalUrl string `json:"canonical_url"` // 当前页面的规范URL
	TotalPages   int    `json:"total_pages"`
	CurrentPage  int    `json:"current_page"`
}
