package request

type Article struct {
	Id           uint   `form:"id"`
	Title        string `form:"title" validate:"required"`
	CategoryName string `form:"category_name" validate:"required"`
	Keywords     string `form:"keywords"`
	Description  string `form:"description"`
	Content      string `form:"content" validate:"required"`
	File         string `form:"file"`
}
