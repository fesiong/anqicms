package model

import (
	"gorm.io/gorm"
)

type Comment struct {
	Model
	ArchiveId int64    `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0;index:idx_archive_id"`
	UserId    uint     `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index:idx_user_id"`
	UserName  string   `json:"user_name" gorm:"column:user_name;type:varchar(32) not null;default:''"`
	Ip        string   `json:"ip" gorm:"column:ip;type:varchar(32) not null;default:''"`
	VoteCount int      `json:"vote_count" gorm:"column:vote_count;type:int(10) not null;default:0;index:idx_vote_count"`
	Content   string   `json:"content" gorm:"column:content;type:longtext default null"`
	ParentId  uint     `json:"parent_id" gorm:"column:parent_id;type:int(10) unsigned not null;default:0;index:idx_parent_id"`
	ToUid     uint     `json:"to_uid" gorm:"column:to_uid;type:int(10) unsigned not null;default:0;index:idx_to_uid"`
	Status    uint     `json:"status" gorm:"column:status;type:tinyint(1) unsigned not null;default:0"`
	ItemTitle string   `json:"item_title" gorm:"-"`
	Parent    *Comment `json:"parent" gorm:"-"`
	Active    bool     `json:"active" gorm:"-"`
}

type CommentPraise struct {
	Id          int64 `json:"id" gorm:"column:id;type:bigint(20) not null AUTO_INCREMENT;primaryKey"`
	CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:int(11);autoCreateTime;index:idx_created_time"`
	ArchiveId   int64 `json:"archive_id" gorm:"column:archive_id;type:bigint(20) not null;default:0;index"`
	CommentId   int64 `json:"comment_id" gorm:"column:comment_id;type:bigint(20) not null;default:0;index:idx_comment_id;index:idx_user_comment_id,priority:2"`
	UserId      uint  `json:"user_id" gorm:"column:user_id;type:int(10) unsigned not null;default:0;index:idx_user_comment_id,priority:1"`
	Rate        int   `json:"rate" gorm:"column:rate;type:tinyint(1) not null;default:1"` // 1=赞，-1=踩，目前只支持 赞
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

func (cp *CommentPraise) AfterCreate(tx *gorm.DB) (err error) {
	// 更新数量
	var total int64
	tx.Model(&CommentPraise{}).Where("`comment_id` = ?", cp.CommentId).Count(&total)
	tx.Model(&Comment{}).Where("`id` = ?", cp.CommentId).UpdateColumn("vote_count", total)

	return
}
