package request

type Article struct {
	Id           uint   `json:"id"`
	Title        string `json:"title"`
	CategoryName string `json:"category_name"`
	CategoryId   uint   `json:"category_id"`
	Keywords     string `json:"keywords"`
	Description  string `json:"description"`
	Content      string `json:"content"`
}
