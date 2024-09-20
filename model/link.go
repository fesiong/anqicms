package model

import (
	"gorm.io/gorm"
)

const LinkStatusWait = uint(0)
const LinkStatusOk = uint(1)
const LinkStatusNofollow = uint(2)
const LinkStatusNotTitle = uint(3)
const LinkStatusNotMatch = uint(4)

type Link struct {
	Model
	Title       string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	Link        string `json:"link" gorm:"column:link;type:varchar(250) not null;default:''"`
	BackLink    string `json:"back_link" gorm:"column:back_link;type:varchar(250) not null;default:''"`
	MyTitle     string `json:"my_title" gorm:"column:my_title;type:varchar(250) not null;default:''"`
	MyLink      string `json:"my_link" gorm:"column:my_link;type:varchar(250) not null;default:''"`
	Contact     string `json:"contact" gorm:"column:contact;type:varchar(250) not null;default:''"`
	Remark      string `json:"remark" gorm:"column:remark;type:varchar(250) not null;default:''"`
	Nofollow    uint   `json:"nofollow" gorm:"column:nofollow;type:tinyint(1) unsigned not null;default:0"`
	Sort        uint   `json:"sort" gorm:"column:sort;type:int(10) unsigned not null;default:99;index:idx_sort"`
	Status      uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
	CheckedTime int64  `json:"checked_time" gorm:"column:checked_time;type:int(11) not null;default:0"`
}

func (link *Link) Save(db *gorm.DB) error {
	if err := db.Save(link).Error; err != nil {
		return err
	}

	return nil
}

func (link *Link) Delete(db *gorm.DB) error {
	if err := db.Delete(link).Error; err != nil {
		return err
	}
	return nil
}
