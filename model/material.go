package model

import "gorm.io/gorm"

type MaterialCategory struct {
	Id            uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime   int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime   int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	Title         string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	MaterialCount uint   `json:"material_count" gorm:"column:material_count;type:int(10) unsigned not null;default:0"`
	Status        uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
}

type Material struct {
	Id            uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime   int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime   int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	Title         string `json:"title" gorm:"column:title;type:varchar(250) not null;default:''"`
	CategoryId    uint   `json:"category_id" gorm:"column:category_id;type:int(10) unsigned not null;default:0;index:idx_category_id"`
	Content       string `json:"content" gorm:"column:content;type:longtext default null"`
	OriginUrl     string `json:"origin_url" gorm:"column:origin_url;type:varchar(190) not null;default:'';index"`
	Keyword       string `json:"keyword" gorm:"column:keyword;type:varchar(250) not null;default:''"`
	Status        uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	AutoUpdate    uint   `json:"auto_update" gorm:"column:auto_update;type:tinyint(1) unsigned not null;default:0"`
	UseCount      uint   `json:"use_count" gorm:"column:use_count;type:int(10) unsigned not null;default:0"`
	Md5           string `json:"md5" gorm:"column:md5;type:varchar(32) default null;index:idx_md5"`
	CategoryTitle string `json:"category_title" gorm:"-"`
}

type MaterialData struct {
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	MaterialId  uint   `json:"material_id" gorm:"column:material_id;type:int(10) not null;default:0;index"`
	ItemType    string `json:"item_type" gorm:"column:item_type;type:varchar(32) not null;default:'';index:idx_item_type"`
	ItemId      int64  `json:"item_id" gorm:"column:item_id;type:bigint(20) not null;default:0;index:idx_item_type"`
}

func (category *MaterialCategory) Delete(db *gorm.DB) error {
	if err := db.Delete(category).Error; err != nil {
		return err
	}

	return nil
}
