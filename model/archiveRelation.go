package model

// ArchiveRelation 相关文档
type ArchiveRelation struct {
	Id         int64 `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	ArchiveId  int64 `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0;index:idx_archive_relation_id,priority:1"`
	RelationId int64 `json:"relation_id" gorm:"column:relation_id;type:bigint(20) not null;default:0;index:idx_archive_relation_id,priority:2"`
}
