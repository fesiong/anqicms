package model

// ArchiveFlag 文档关联的Flag
type ArchiveFlag struct {
	Id        int64  `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	ArchiveId int64  `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0;uniqueIndex:idx_archive_id_flag,priority:1"`
	Flag      string `json:"flag" gorm:"column:flag;type:char(1) default null;uniqueIndex:idx_archive_id_flag,priority:2;index"` // 'c','h','p','f','s','j','a','b'
}

type ArchiveFlags struct {
	ArchiveId int64  `json:"archive_id"`
	Flags     string `json:"flags"`
}
