package request

type Category struct {
	Id          uint   `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
	ParentId    uint   `json:"parent_id"`
	Sort        uint   `json:"sort"`
	Status      uint   `json:"status"`
	Type        uint   `json:"type"`
}
