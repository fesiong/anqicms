package model

import "gorm.io/gorm"

// ArchiveRelation 相关文档
type ArchiveRelation struct {
	Id         int64 `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	ArchiveId  int64 `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0;index:idx_archive_relation_id,priority:1"`
	RelationId int64 `json:"relation_id" gorm:"column:relation_id;type:bigint(20) not null;default:0;index:idx_archive_relation_id,priority:2"`
}

// ArchiveFavorite 文档收藏表
type ArchiveFavorite struct {
	Id          int64 `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	ArchiveId   int64 `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0;index:idx_archive_collection_id,priority:1"`
	SkuId       int64 `json:"sku_id" gorm:"column:sku_id;type:bigint(20) unsigned not null;default:0;index"`
	UserId      int64 `json:"user_id" gorm:"column:user_id;type:bigint(20) not null;default:0;index;index:idx_archive_collection_id,priority:2"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
}

func (a *ArchiveFavorite) AfterCreate(tx *gorm.DB) (err error) {
	// 更新数量
	var total int64
	tx.Model(&ArchiveFavorite{}).Where("`archive_id` = ?", a.ArchiveId).Count(&total)
	tx.Model(&Archive{}).Where("`id` = ?", a.ArchiveId).UpdateColumn("favorite_count", total)

	return
}
