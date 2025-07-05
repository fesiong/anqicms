package model

import (
	"database/sql/driver"
	"encoding/json"
)

type Guestbook struct {
	Model
	UserName  string    `json:"user_name" gorm:"column:user_name;type:varchar(250) not null;default:''"`
	Contact   string    `json:"contact" gorm:"column:contact;type:varchar(250) not null;default:''"`
	Content   string    `json:"content" gorm:"column:content;type:text default null"`
	Ip        string    `json:"ip" gorm:"column:ip;type:varchar(32) not null;default:''"`
	Status    int       `json:"status" gorm:"column:status;type:tinyint(1) not null;default:0"` // 状态，0 未处理，1 正常，2 垃圾
	Refer     string    `json:"refer" gorm:"column:refer;type:varchar(250) not null;default:''"`
	SiteId    uint      `json:"site_id" gorm:"column:site_id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"` // 默认为0 表示本站，如果有ID，表示子站ID
	ExtraData extraData `json:"extra_data" gorm:"column:extra_data;type:longtext default null"`
}

type extraData map[string]interface{}

func (e extraData) Value() (driver.Value, error) {
	return json.Marshal(e)
}

// Scan 这里不报错
func (e *extraData) Scan(data interface{}) error {
	_ = json.Unmarshal(data.([]byte), &e)
	return nil
}
