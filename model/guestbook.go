package model

import (
	"database/sql/driver"
	"encoding/json"
)

type Guestbook struct {
	Id          uint      `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64     `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64     `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	UserName    string    `json:"user_name" gorm:"column:user_name;type:varchar(250) not null;default:''"`
	Contact     string    `json:"contact" gorm:"column:contact;type:varchar(250) not null;default:''"`
	Content     string    `json:"content" gorm:"column:content;type:text default null"`
	Ip          string    `json:"ip" gorm:"column:ip;type:varchar(32) not null;default:''"`
	Status      int       `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"` // 状态，0 未处理，1 正常，2 垃圾
	Refer       string    `json:"refer" gorm:"column:refer;type:varchar(250) not null;default:''"`
	SiteId      uint      `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null;default:0"` // 默认为0 表示本站，如果有ID，表示子站ID
	ExtraData   ExtraData `json:"extra_data" gorm:"column:extra_data;type:longtext default null"`
}

type ExtraData map[string]interface{}

func (e ExtraData) Value() (driver.Value, error) {
	return json.Marshal(e)
}

// Scan 这里不报错
func (e *ExtraData) Scan(data interface{}) error {
	_ = json.Unmarshal(data.([]byte), &e)
	return nil
}
