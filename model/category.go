package model

type Category struct {
	ID           uint   `gorm:"primary_key" json:"id"`
	ParentID     uint   `json:"parentID"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Logo         string `json:"logo"`
	AddTime      int64  `json:"addTime"`
	ArticleCount int    `json:"articleCount"`
}
