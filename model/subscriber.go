package model

type Subscriber struct {
	Id          int64  `json:"id" gorm:"column:id;type:int(11) not null;primary_key;AUTO_INCREMENT"`
	UserId      int64  `json:"user_id" gorm:"column:user_id;type:int(11);index"`
	Email       string `json:"email" gorm:"column:email;type:varchar(100);not null;index"`
	CategoryId  int64  `json:"category_id" gorm:"column:category_id;type:int(11);index"`
	Remark      string `json:"remark" gorm:"column:remark;type:varchar(255)"`
	Status      int    `json:"status" gorm:"column:status;type:tinyint(2)"`        // 1=有效，0=无效
	SendTimes   int    `json:"send_times" gorm:"column:send_times;type:int(11)"`   // 发送次数
	ErrorTimes  int    `json:"error_times" gorm:"column:error_times;type:int(11)"` // 错误次数
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
}

type SubscriberCategory struct {
	Id              int64  `json:"id" gorm:"column:id;type:int(11) not null;primary_key;AUTO_INCREMENT"`
	Title           string `json:"title" gorm:"column:title;type:varchar(100);not null"`
	SubscriberCount int64  `json:"subscriber_count" gorm:"column:subscriber_count;type:int(11)"`
	CreatedTime     int64  `json:"created_time" gorm:"column:created_time;type:int(11)"`
}
