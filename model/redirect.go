package model

type Redirect struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	FromUrl     string `json:"from_url" gorm:"column:from_url;type:varchar(190) not null;default:'';unique"`
	ToUrl       string `json:"to_url" gorm:"column:to_url;type:varchar(250) not null;default:''"`
	SiteId      uint   `json:"-" gorm:"-"`
}
