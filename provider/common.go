package provider

import (
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
)

type statistics struct {
	ArchiveCount  int64 `json:"archive_count"`
	CategoryCount int64 `json:"category_count"`
	LinkCount     int64 `json:"link_count"`
	CommentCount  int64 `json:"comment_count"`
}

func Statistics() *statistics {
	var archiveCount int64
	var categoryCount int64
	var linkCount int64
	var commentCount int64
	dao.DB.Model(&model.Archive{}).Where("status != 99").Count(&archiveCount)
	dao.DB.Model(&model.Category{}).Where("status != 99").Count(&categoryCount)
	dao.DB.Model(&model.Link{}).Where("status != 99").Count(&linkCount)
	dao.DB.Model(&model.Comment{}).Where("status != 99").Count(&commentCount)

	return &statistics{
		ArchiveCount:  archiveCount,
		CategoryCount: categoryCount,
		LinkCount:     linkCount,
		CommentCount:  categoryCount,
	}
}
