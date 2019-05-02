package model

type Comment struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	ParentID  uint      `json:"parentID"`
	ArticleID uint      `json:"articleID"`
	UserID    uint      `json:"userID"`
	Message   string    `json:"message"`
	AddTime   int64     `json:"addTime"`
	Parents   []Comment `json:"parents"`
	User      User      `json:"user"`
}
