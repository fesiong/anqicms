package model

import (
	"gorm.io/gorm"
)

type Comment struct {
	Model
	ArchiveId uint     `json:"archive_id" gorm:"column:archive_id;type:int(10) unsigned not null;default:0;index:idx_archive_id"`
	UserId    uint     `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index:idx_user_id"`
	UserName  string   `json:"user_name" gorm:"column:user_name;type:varchar(32) not null;default:''"`
	Ip        string   `json:"ip" gorm:"column:ip;type:varchar(32) not null;default:''"`
	VoteCount int      `json:"vote_count" gorm:"column:vote_count;type:int(10) not null;default:0;index:idx_vote_count"`
	Content   string   `json:"content" gorm:"column:content;type:longtext default null"`
	ParentId  uint     `json:"parent_id" gorm:"column:parent_id;type:int(10) unsigned not null;default:0;index:idx_parent_id"`
	ToUid     uint     `json:"to_uid" gorm:"column:to_uid;type:int(10) unsigned not null;default:0;index:idx_to_uid"`
	Status    uint     `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0;index:idx_status"`
	ItemTitle string   `json:"item_title" gorm:"-"`
	Parent    *Comment `json:"parent" gorm:"-"`
	Active    bool     `json:"active" gorm:"-"`
}

func (comment *Comment) Save(db *gorm.DB) error {
	if err := db.Save(comment).Error; err != nil {
		return err
	}
	comment.UpdateCommentCount(db)

	return nil
}

func (comment *Comment) Delete(db *gorm.DB) error {
	if err := db.Delete(comment).Error; err != nil {
		return err
	}
	comment.UpdateCommentCount(db)

	return nil
}

func (comment *Comment) UpdateCommentCount(db *gorm.DB) {
	// 更新数量
	var total int64
	db.Model(&Comment{}).Where("`archive_id` = ?", comment.ArchiveId).Count(&total)
	db.Model(&Archive{}).Where("`id` = ?", comment.ArchiveId).UpdateColumn("comment_count", total)
}
