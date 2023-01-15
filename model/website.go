package model

import "kandaoni.com/anqicms/config"

type Website struct {
	Model
	RootPath string             `json:"root_path" gorm:"column:root_path;type:varchar(190) not null;default:''"`
	Name     string             `json:"name" gorm:"column:name;type:varchar(128) not null;default:''"`
	Mysql    config.MysqlConfig `json:"mysql" gorm:"column:mysql;type:text;default null"`
	Status   uint               `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	BaseUrl  string             `json:"base_url" gorm:"-"`
	ErrorMsg string             `json:"error_msg" gorm:"-"`
}
