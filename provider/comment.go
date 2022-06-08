package provider

import (
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func SaveComment(req *request.PluginComment) (comment *model.Comment, err error) {
	if req.Id > 0 {
		comment, err = GetCommentById(req.Id)
		if err != nil {
			return nil, err
		}
	} else {
		comment = &model.Comment{
			Status:      0,
			ArchiveId: req.ArchiveId,
			UserId: req.UserId,
			Ip: req.Ip,
			ParentId: req.ParentId,
			ToUid: req.ToUid,
		}
	}

	comment.UserName = req.UserName
	comment.Content = req.Content

	err = comment.Save(dao.DB)
	return
}

func GetCommentList(archiveId uint, order string, currentPage int, pageSize int, offset int) ([]*model.Comment, int64, error) {
	var comments []*model.Comment
	if currentPage > 1 {
		offset = (currentPage - 1) * pageSize
	}
	var total int64

	builder := dao.DB.Model(&model.Comment{})
	if archiveId > 0 {
		builder = builder.Where("archive_id = ?", archiveId)
	}
	if order != "" {
		builder = builder.Order(order)
	}
	if err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&comments).Error; err != nil {
		return nil, 0, err
	}
	for i, v := range comments {
		if v.ParentId > 0 {
			var parent model.Comment
			if err := dao.DB.Where("id = ?", v.ParentId).First(&parent).Error; err == nil {
				comments[i].Parent = &parent
			}
		}
	}

	return comments, total, nil
}

func GetCommentById(id uint) (*model.Comment, error) {
	var comment model.Comment
	if err := dao.DB.Where("id = ?", id).First(&comment).Error; err != nil {
		return nil, err
	}
	//获取itemItile
	archive, err := GetArchiveById(comment.ArchiveId)
	if err == nil {
		comment.ItemTitle = archive.Title
	}

	//获取parent
	if comment.ParentId > 0 {
		var parent model.Comment
		if err := dao.DB.Where("id = ?", comment.ParentId).First(&parent).Error; err == nil {
			comment.Parent = &parent
		}
	}

	return &comment, nil
}
