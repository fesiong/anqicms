package provider

import (
	"irisweb/config"
	"irisweb/model"
	"irisweb/request"
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
			ItemType: req.ItemType,
			ItemId: req.ItemId,
			UserId: req.UserId,
			Ip: req.Ip,
			ParentId: req.ParentId,
			ToUid: req.ToUid,
		}
	}

	comment.UserName = req.UserName
	comment.Content = req.Content

	err = comment.Save(config.DB)
	return
}

func GetCommentList(itemType string, itemId uint, order string, currentPage int, pageSize int) ([]*model.Comment, int64, error) {
	var comments []*model.Comment
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := config.DB.Model(&model.Comment{}).Where("`status` != 99")
	if itemType != "" {
		builder = builder.Where("`item_type` = ? and item_id = ?", itemType, itemId)
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
			if err := config.DB.Where("id = ?", v.ParentId).First(&parent).Error; err == nil {
				comments[i].Parent = &parent
			}
		}
	}

	return comments, total, nil
}

func GetCommentById(id uint) (*model.Comment, error) {
	var comment model.Comment
	if err := config.DB.Where("id = ?", id).First(&comment).Error; err != nil {
		return nil, err
	}
	//获取itemItile
	if comment.ItemType == model.ItemTypeArticle {
		article, err := GetArticleById(comment.ItemId)
		if err == nil {
			comment.ItemTitle = article.Title
		}
	} else if comment.ItemType == model.ItemTypeProduct {
		//todo
	}

	//获取parent
	if comment.ParentId > 0 {
		var parent model.Comment
		if err := config.DB.Where("id = ?", comment.ParentId).First(&parent).Error; err == nil {
			comment.Parent = &parent
		}
	}

	return &comment, nil
}
