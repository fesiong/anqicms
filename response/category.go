package response

type CategoryTemplate struct {
	Template       string `json:"template"`
	DetailTemplate string `json:"detail_template"`
}

type ApiCategory struct {
	Id       uint   `json:"id"`
	ParentId uint   `json:"parent_id"`
	Title    string `json:"title"`
}
