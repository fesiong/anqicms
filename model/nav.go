package model

import (
	"fmt"
	"gorm.io/gorm"
	"time"
)

const (
	NavTypeSystem   = 0
	NavTypeCategory = 1
	NavTypeOutlink  = 2
)

type Nav struct {
	Model
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primary_key"`
	Title       string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	SubTitle    string `json:"sub_title" gorm:"column:sub_title;type:varchar(250) not null;default:''"`
	Description string `json:"description" gorm:"column:description;type:varchar(250) not null;default:''"`
	ParentId    uint   `json:"parent_id" gorm:"column:parent_id;type:int(10) unsigned not null;default:0;index:idx_parent_id"`
	NavType     uint   `json:"nav_type" gorm:"column:nav_type;type:int(10) unsigned not null;default:0;index:idx_nav_type"`
	PageId      uint   `json:"page_id" gorm:"column:page_id;type:int(10) unsigned not null;default:0;index:idx_page_id"`
	Link        string `json:"link" gorm:"column:link;type:varchar(250) not null;default:''"`
	Sort        uint   `json:"sort" gorm:"column:sort;type:int(10) unsigned not null;default:99;index:idx_sort"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11) not null;default:0;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:int(11) not null;default:0;index:idx_updated_time"`
	DeletedTime int64  `json:"-" gorm:"column:deleted_time;type:int(11) not null;default:0"`
	NavList     []*Nav `json:"nav_list" gorm:"-"`

}

func (nav *Nav) Save(db *gorm.DB) error {
	if nav.Id == 0 {
		nav.CreatedTime = time.Now().Unix()
	}
	nav.UpdatedTime = time.Now().Unix()

	if err := db.Save(nav).Error; err != nil {
		return err
	}

	return nil
}

func (nav *Nav) Delete(db *gorm.DB) error {
	if err := db.Model(nav).Updates(Nav{Status: 99, DeletedTime: time.Now().Unix()}).Error; err != nil {
		return err
	}

	return nil
}

func (nav *Nav) GetLink() string {
	link := ""
	if nav.NavType == NavTypeSystem {
		if nav.PageId == 0 {
			//首页
			link = "/"
		}
	} else if nav.NavType == NavTypeCategory {
		link = fmt.Sprintf("/category/%d", nav.PageId)
	} else if nav.NavType == NavTypeOutlink {
		//外链
		link = nav.Link
	}

	return link
}
