package model

import (
	"gorm.io/gorm"
	"time"
)

type Category struct {
	Model
	Id          uint    `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primary_key"`
	Title       string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	Description string `json:"description" gorm:"column:description;type:varchar(250) not null;default:''"`
	Content     string `json:"content" gorm:"column:content;type:longtext default null"`
	ParentId    uint    `json:"parent_id" gorm:"column:parent_id;type:int(10) unsigned not null;default:0;index:idx_parent_id"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
	CreatedTime int64   `json:"created_time" gorm:"column:created_time;type:int(11) not null;default:0;index:idx_created_time"`
	UpdatedTime int64   `json:"updated_time" gorm:"column:updated_time;type:int(11) not null;default:0;index:idx_updated_time"`
	DeletedTime int64   `json:"-" gorm:"column:deleted_time;type:int(11) not null;default:0"`
}

func (category *Category) Save(db *gorm.DB) error {
	if category.Id == 0 {
		category.CreatedTime = time.Now().Unix()
	}

	if err := db.Save(category).Error; err != nil {
		return err
	}

	return nil
}
