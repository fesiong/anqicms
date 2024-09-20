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
	Refer     string    `json:"refer" gorm:"column:refer;type:varchar(250) not null;default:''"`
	ExtraData extraData `json:"extra_data" gorm:"column:extra_data;type:longtext default null"`
}

type extraData map[string]interface{}

func (e extraData) Value() (driver.Value, error) {
	return json.Marshal(e)
}

func (e *extraData) Scan(data interface{}) error {
	return json.Unmarshal(data.([]byte), &e)
}
