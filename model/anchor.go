package model

import (
	"gorm.io/gorm"
)

type Anchor struct {
	//默认字段
	Id           uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime  int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime  int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	Title        string `json:"title" gorm:"column:title;type:varchar(190) not null;default:'';unique"`
	ArchiveId    int64  `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0"`
	Link         string `json:"link" gorm:"column:link;type:varchar(190) not null;default:'';index"`
	Weight       int    `json:"weight" gorm:"column:weight;type:int(10) not null;default:0;index:idx_weight"`
	ReplaceCount int64  `json:"replace_count" gorm:"column:replace_count;type:int(10) not null;default:0"`
	Status       uint   `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
}

type AnchorData struct {
	//默认字段
	Id          uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	UpdatedTime int64  `json:"updated_time" gorm:"column:updated_time;type:int(11);autoUpdateTime;index:idx_updated_time"`
	AnchorId    uint   `json:"anchor_id" gorm:"column:anchor_id;type:int(10) not null;default:0;index"`
	ItemType    string `json:"item_type" gorm:"column:item_type;type:varchar(32) not null;default:'';index:idx_item_type"`
	ItemId      int64  `json:"item_id" gorm:"column:item_id;type:bigint(20) not null;default:0;index:idx_item_type"`
}

func (anchor *Anchor) Save(db *gorm.DB) error {
	if err := db.Save(anchor).Error; err != nil {
		return err
	}

	return nil
}

func (anchor *Anchor) Delete(db *gorm.DB) error {
	if err := db.Delete(anchor).Error; err != nil {
		return err
	}

	return nil
}
