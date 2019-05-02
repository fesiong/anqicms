package model

type Relation struct {
	ID         uint  `gorm:"primary_key" json:"id"`
	CategoryID uint  `json:"categoryID"`
	AddTime    int64 `json:"addTime"`
	ArticleID  uint  `json:"articleID"`
}
