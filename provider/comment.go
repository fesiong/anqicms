package provider

import (
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) SaveComment(req *request.PluginComment) (comment *model.Comment, err error) {
	if req.Id > 0 {
		comment, err = w.GetCommentById(req.Id)
		if err != nil {
			return nil, err
		}
	} else {
		comment = &model.Comment{
			Status:    req.Status,
			ArchiveId: req.ArchiveId,
			UserId:    req.UserId,
			Ip:        req.Ip,
			ParentId:  req.ParentId,
			ToUid:     req.ToUid,
		}
	}
	comment.Status = req.Status
	comment.UserName = req.UserName
	comment.Content = req.Content

	err = comment.Save(w.DB)
	return
}

func (w *Website) GetCommentList(archiveId int64, userId uint, order string, currentPage int, pageSize int, offset int) ([]*model.Comment, int64, error) {
	var comments []*model.Comment
	if currentPage > 1 {
		offset = (currentPage - 1) * pageSize
	}
	var total int64

	builder := w.DB.Model(&model.Comment{}).WithContext(w.Ctx())
	if archiveId > 0 {
		builder = builder.Where("archive_id = ?", archiveId)
	}
	if userId > 0 {
		builder = builder.Where("user_id = ?", userId)
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
			if err := w.DB.Where("id = ?", v.ParentId).First(&parent).Error; err == nil {
				comments[i].Parent = &parent
			}
		}
	}

	return comments, total, nil
}

func (w *Website) GetCommentById(id uint) (*model.Comment, error) {
	var comment model.Comment
	if err := w.DB.WithContext(w.Ctx()).Where("id = ?", id).First(&comment).Error; err != nil {
		return nil, err
	}
	//获取itemItile
	archive, err := w.GetArchiveById(comment.ArchiveId)
	if err == nil {
		comment.ItemTitle = archive.Title
	}

	//获取parent
	if comment.ParentId > 0 {
		var parent model.Comment
		if err := w.DB.WithContext(w.Ctx()).Where("id = ?", comment.ParentId).First(&parent).Error; err == nil {
			comment.Parent = &parent
		}
	}

	return &comment, nil
}

func (w *Website) GetCommentPraise(userId uint, commentId int64) (*model.CommentPraise, error) {
	var praise model.CommentPraise
	if err := w.DB.WithContext(w.Ctx()).Where("user_id = ? and comment_id = ?", userId, commentId).Take(&praise).Error; err != nil {
		return nil, err
	}

	return &praise, nil
}

func (w *Website) AddCommentPraise(userId uint, commentId int64, archiveId int64) (praise *model.CommentPraise, err error) {
	praise, err = w.GetCommentPraise(userId, commentId)
	if err != nil {
		praise = &model.CommentPraise{
			UserId:    userId,
			CommentId: commentId,
			ArchiveId: archiveId,
			Rate:      1,
		}
		if err = w.DB.Create(praise).Error; err != nil {
			return nil, err
		}
	}

	return praise, nil
}
