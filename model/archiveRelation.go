package model

// ArchiveRelation 相关文档
type ArchiveRelation struct {
	Id         uint `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	ArchiveId  uint `json:"archive_id" gorm:"column:archive_id;type:int(10) unsigned not null;default:0;index:idx_archive_relation_id,priority:1"`
	RelationId uint `json:"relation_id" gorm:"column:relation_id;type:int(10) unsigned not null;default:0;index:idx_archive_relation_id,priority:2"`
}
