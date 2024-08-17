package model

// ArchiveFlag 文档关联的Flag
type ArchiveFlag struct {
	Id        uint   `json:"id" gorm:"column:id;type:int(10) unsigned not null AUTO_INCREMENT;primaryKey"`
	ArchiveId uint   `json:"archive_id" gorm:"column:archive_id;type:int(10) unsigned not null;default:0;uniqueIndex:idx_archive_id_flag,priority:1"`
	Flag      string `json:"flag" gorm:"column:flag;type:char(1) default null;uniqueIndex:idx_archive_id_flag,priority:2;index"` // 'c','h','p','f','s','j','a','b'
}

type ArchiveFlags struct {
	ArchiveId uint   `json:"archive_id"`
	Flags     string `json:"flags"`
}
