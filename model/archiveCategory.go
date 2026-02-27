package model

// ArchiveCategory 文档关联的多分类ID
type ArchiveCategory struct {
	Id         int64 `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	CategoryId uint  `json:"category_id" gorm:"column:category_id;type:int(10) unsigned not null;default:0;index:idx_category_id;index:idx_archive_category_id,priority:2"`
	ArchiveId  int64 `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0;index:idx_archive_category_id,priority:1"`
}
