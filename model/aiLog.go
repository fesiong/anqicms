package model

type AiLog struct {
	Id          int    `json:"id" gorm:"column:id;not null;PRIMARY_KEY;AUTO_INCREMENT"`
	UserId      uint   `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index:idx_user_id"`
	TextLength  int64  `json:"text_length" gorm:"column:text_length;type:int(11)"`
	Prompt      string `json:"prompt" gorm:"column:prompt;type:text;default null"`
	Content     string `json:"content" gorm:"column:content;type:text;default null"`
	AiRemain    int64  `json:"ai_remain" gorm:"column:ai_remain;type:int(10) not null;default:0"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(10);default 0;autoCreateTime"`
}
