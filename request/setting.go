package request

type NavConfig struct {
	Id          uint   `json:"id"`
	Title       string `json:"title"`
	SubTitle    string `json:"sub_title"`
	Description string `json:"description"`
	ParentId    uint   `json:"parent_id"`
	NavType     uint   `json:"nav_type"`
	PageId      int64  `json:"page_id"`
	TypeId      uint   `json:"type_id"`
	Link        string `json:"link"`
	Sort        uint   `json:"sort"`
	Status      uint   `json:"status"`
}

type NavTypeRequest struct {
	Id    uint   `json:"id"`
	Title string `json:"title"`
}
