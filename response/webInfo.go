package response

type WebInfo struct {
	Title       string `json:"title"`
	Keywords    string `json:"keywords"`
	Description string `json:"description"`
	NavBar      uint   `json:"nav_bar"`
	PageName    string `json:"page_name"`
}
