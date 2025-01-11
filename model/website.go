package model

import (
	"kandaoni.com/anqicms/config"
)

type Website struct {
	Model
	RootPath     string             `json:"root_path" gorm:"column:root_path;type:varchar(190) not null;default:''"`
	Name         string             `json:"name" gorm:"column:name;type:varchar(128) not null;default:''"`
	Mysql        config.MysqlConfig `json:"mysql" gorm:"column:mysql;type:text;default null"`
	TokenSecret  string             `json:"token_secret" gorm:"column:token_secret;type:varchar(128) not null;default:''"`
	Status       uint               `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	ParentId     uint               `json:"parent_id" gorm:"column:parent_id;type:int(10) not null;default:0"`
	SyncTime     int64              `json:"sync_time" gorm:"column:sync_time;type:int(11);default:0"`
	LanguageIcon string             `json:"language_icon" gorm:"column:language_icon;type:varchar(255) not null;default:''"` // 图标
	Language     string             `json:"language" gorm:"-"`
	BaseUrl      string             `json:"base_url" gorm:"-"`
}
