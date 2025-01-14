package model

import (
	"gorm.io/gorm"
)

const (
	NavTypeSystem   = 0
	NavTypeCategory = 1
	NavTypeOutlink  = 2
	NavTypeArchive  = 3
)

type Nav struct {
	Model
	Title       string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	SubTitle    string `json:"sub_title" gorm:"column:sub_title;type:varchar(250) not null;default:''"`
	Description string `json:"description" gorm:"column:description;type:varchar(1000) not null;default:''"`
	ParentId    uint   `json:"parent_id" gorm:"column:parent_id;type:int(10) unsigned not null;default:0;index:idx_parent_id"`
	NavType     uint   `json:"nav_type" gorm:"column:nav_type;type:int(10) unsigned not null;default:0;index:idx_nav_type"`
	PageId      int64  `json:"page_id" gorm:"column:page_id;type:bigint(20) not null;default:0;index:idx_page_id"`
	TypeId      uint   `json:"type_id" gorm:"column:type_id;type:int(10) unsigned not null;default:1;index:idx_type_id"`
	Link        string `json:"link" gorm:"column:link;type:varchar(250) not null;default:''"`
	Sort        uint   `json:"sort" gorm:"column:sort;type:int(10) unsigned not null;default:99;index:idx_sort"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	NavList     []*Nav `json:"nav_list" gorm:"-"`
	IsCurrent   bool   `json:"is_current" gorm:"-"`
	Spacer      string `json:"spacer" gorm:"-"`
}

type NavType struct {
	Model
	Title string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
}

func (nav *Nav) Save(db *gorm.DB) error {
	if err := db.Save(nav).Error; err != nil {
		return err
	}

	return nil
}

func (nav *Nav) Delete(db *gorm.DB) error {
	if err := db.Delete(nav).Error; err != nil {
		return err
	}
	return nil
}
