package request

type Product struct {
	Id           uint     `json:"id"`
	Title        string   `json:"title"`
	CategoryName string   `json:"category_name"`
	CategoryId   uint     `json:"category_id"`
	Keywords     string   `json:"keywords"`
	Description  string   `json:"description"`
	Content      string   `json:"content"`
	Price        float64  `json:"price"`
	Stock        uint     `json:"stock"`
	Images       []string `json:"images"`
}

type ProductThumb struct {
	Id  uint   `json:"id"`
	Src string `json:"src"`
}
