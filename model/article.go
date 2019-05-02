package model

type Article struct {
	ID           uint       `gorm:"primary_key" json:"id"`
	Title        string     `json:"title"`
	SeoTitle     string     `json:"seoTitle"`
	Keywords     string     `json:"keywords"`
	Description  string     `json:"description"`
	Message      string     `json:"message"`
	Logo         string     `json:"logo"`
	Views        uint       `json:"views"`
	AddTime      int64      `json:"addTime"`
	CommentCount uint       `json:"commentCount"`
	Status       int        `json:"status"`
	IsRecommend  int        `json:"isRecommend"`
	Categories   []Category `gorm:"many2many:relation;ForeignKey:ID;AssociationForeignKey:ID" json:"categories"`
	Comments     []Comment  `gorm:"ForeignKey:ID" json:"comments"`
}
