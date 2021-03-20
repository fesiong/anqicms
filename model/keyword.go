package model

import (
	"gorm.io/gorm"
)

type Keyword struct {
	Model
	Title    string `json:"title" gorm:"column:title;type:varchar(250) unique not null;default:'';index"`
	Status   uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
}

func (keyword *Keyword) Save(db *gorm.DB) error {
	if err := db.Save(keyword).Error; err != nil {
		return err
	}

	return nil
}

func (keyword *Keyword) Delete(db *gorm.DB) error {
	if err := db.Delete(keyword).Error; err != nil {
		return err
	}

	return nil
}
