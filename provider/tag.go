package provider

import (
	"errors"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"strings"
)

func GetTagList(itemId uint, title string, firstLetter string, currentPage, pageSize int, offset int) ([]*model.Tag, int64, error) {
	var tags []*model.Tag
	if currentPage > 1 {
		offset = (currentPage - 1) * pageSize
	}
	var total int64

	builder := dao.DB.Model(&model.Tag{}).Order("id desc")
	if firstLetter != "" {
		builder = builder.Where("`first_letter` = ?", firstLetter)
	}
	if itemId != 0 {
		var ids []uint
		dao.DB.Model(&model.TagData{}).Where("`item_id` = ?", itemId).Pluck("tag_id", &ids)
		if len(ids) == 0 {
			// 否则只有0
			ids = append(ids, 0)
		}
		builder = builder.Where("`id` IN(?)", ids)
	}
	if title != "" {
		builder = builder.Where("`title` like ?", "%"+title+"%")
	}

	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&tags).Error
	if err != nil {
		return nil, 0, err
	}

	return tags, total, nil
}

func GetTagById(id uint) (*model.Tag, error) {
	var tag model.Tag
	if err := dao.DB.Where("id = ?", id).First(&tag).Error; err != nil {
		return nil, err
	}

	return &tag, nil
}

func GetTagByUrlToken(urlToken string) (*model.Tag, error) {
	var tag model.Tag
	if err := dao.DB.Where("url_token = ?", urlToken).First(&tag).Error; err != nil {
		return nil, err
	}

	return &tag, nil
}

func GetTagByTitle(title string) (*model.Tag, error) {
	var tag model.Tag
	if err := dao.DB.Where("`title` = ?", title).First(&tag).Error; err != nil {
		return nil, err
	}

	return &tag, nil
}

func DeleteTag(id uint) error {
	tag, err := GetTagById(id)
	if err != nil {
		return err
	}

	//删除记录
	dao.DB.Unscoped().Where("`tag_id` = ?", tag.Id).Delete(model.TagData{})

	//执行删除操作
	err = dao.DB.Delete(tag).Error

	if err != nil {
		return err
	}

	return nil
}

func SaveTag(req *request.PluginTag) (tag *model.Tag, err error) {
	newPost := false
	if req.Id > 0 {
		tag, err = GetTagById(req.Id)
		if err != nil {
			return nil, err
		}
	} else {
		tag = &model.Tag{
			Status: 1,
		}
		newPost = true
	}
	tag.Title = req.Title
	tag.SeoTitle = req.SeoTitle
	tag.Keywords = req.Keywords
	tag.Status = 1
	tag.UrlToken = req.UrlToken
	tag.Description = req.Description
	tag.FirstLetter = req.FirstLetter
	// 判断重复
	if req.UrlToken != "" {
		req.UrlToken = library.ParseUrlToken(req.UrlToken)
		exists, err := GetTagByUrlToken(req.UrlToken)
		if err == nil {
			if tag.Id == 0 || (tag.Id > 0 && exists.Id != tag.Id) {
				return nil, errors.New(config.Lang("自定义URL重复"))
			}
		}
		tag.UrlToken = req.UrlToken
	}
	if tag.UrlToken == "" {
		newToken := library.GetPinyin(req.Title)
		_, err := GetTagByUrlToken(newToken)
		if err == nil {
			//增加随机
			newToken += library.GenerateRandString(3)
		}
		tag.UrlToken = newToken
	}
	if tag.FirstLetter == "" {
		letter := "A"
		if tag.UrlToken != "-" {
			letter = string(tag.UrlToken[0])
		}
		tag.FirstLetter = strings.ToUpper(letter)
	}

	err = dao.DB.Save(tag).Error

	if err != nil {
		return
	}

	if newPost && tag.Status == config.ContentStatusOK {
		link := GetUrl("tag", tag, 0)
		go PushArchive(link)
		if config.JsonData.PluginSitemap.AutoBuild == 1 {
			_ = AddonSitemap("tag", link)
		}
	}

	return
}

func SaveTagData(itemId uint, tagNames []string) error {
	if len(tagNames) == 0 {
		dao.DB.Where("`item_id` = ?", itemId).Delete(&model.TagData{})
		return nil
	}
	var tagIds = make([]uint, 0, len(tagNames))
	for _, tagName := range tagNames {
		tag, err := GetTagByTitle(tagName)
		if err != nil {
			newToken := library.GetPinyin(tagName)
			_, err = GetTagByUrlToken(newToken)
			if err == nil {
				//增加随机
				newToken += library.GenerateRandString(3)
			}
			letter := "A"
			if newToken != "-" {
				letter = string(newToken[0])
			}
			tag = &model.Tag{
				Title:       tagName,
				UrlToken:    newToken,
				FirstLetter: strings.ToUpper(letter),
				Status:      1,
			}
			dao.DB.Save(tag)

			link := GetUrl("tag", tag, 0)
			go PushArchive(link)
			if config.JsonData.PluginSitemap.AutoBuild == 1 {
				_ = AddonSitemap("tag", link)
			}
		}
		tagIds = append(tagIds, tag.Id)
		tagData := model.TagData{
			TagId:  tag.Id,
			ItemId: itemId,
		}
		dao.DB.Where("`item_id` = ? and `tag_id` = ?", itemId, tagData.TagId).FirstOrCreate(&tagData)
	}
	dao.DB.Where("`item_id` = ? and `tag_id` not in(?)", itemId, tagIds).Delete(&model.TagData{})

	return nil
}

func GetTagsByItemId(itemId uint) []*model.Tag {
	var tags []*model.Tag
	var tagIds []uint
	err := dao.DB.Model(&model.TagData{}).Where("`item_id` = ?", itemId).Pluck("tag_id", &tagIds).Error
	if err != nil {
		return nil
	}
	if len(tagIds) > 0 {
		dao.DB.Where("id IN(?)", tagIds).Find(&tags)
	}

	return tags
}
