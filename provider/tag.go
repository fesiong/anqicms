package provider

import (
	"fmt"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"strings"
	"time"
)

func (w *Website) GetTagList(itemId uint, title string, firstLetter string, currentPage, pageSize int, offset int) ([]*model.Tag, int64, error) {
	var tags []*model.Tag
	if currentPage > 1 {
		offset = (currentPage - 1) * pageSize
	}
	var total int64

	builder := w.DB.Model(&model.Tag{}).Order("id desc")
	if firstLetter != "" {
		builder = builder.Where("`first_letter` = ?", firstLetter)
	}
	if itemId != 0 {
		var ids []uint
		w.DB.Model(&model.TagData{}).Where("`item_id` = ?", itemId).Pluck("tag_id", &ids)
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

func (w *Website) GetTagById(id uint) (*model.Tag, error) {
	var tag model.Tag
	if err := w.DB.Where("id = ?", id).First(&tag).Error; err != nil {
		return nil, err
	}

	return &tag, nil
}

func (w *Website) GetTagByUrlToken(urlToken string) (*model.Tag, error) {
	var tag model.Tag
	if err := w.DB.Where("url_token = ?", urlToken).First(&tag).Error; err != nil {
		return nil, err
	}

	return &tag, nil
}

func (w *Website) GetTagByTitle(title string) (*model.Tag, error) {
	var tag model.Tag
	if err := w.DB.Where("`title` = ?", title).First(&tag).Error; err != nil {
		return nil, err
	}

	return &tag, nil
}

func (w *Website) DeleteTag(id uint) error {
	tag, err := w.GetTagById(id)
	if err != nil {
		return err
	}

	//删除记录
	w.DB.Unscoped().Where("`tag_id` = ?", tag.Id).Delete(model.TagData{})

	//执行删除操作
	err = w.DB.Delete(tag).Error

	if err != nil {
		return err
	}

	return nil
}

func (w *Website) SaveTag(req *request.PluginTag) (tag *model.Tag, err error) {
	newPost := false
	if req.Id > 0 {
		tag, err = w.GetTagById(req.Id)
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
	req.UrlToken = library.ParseUrlToken(req.UrlToken)
	if req.UrlToken == "" {
		req.UrlToken = library.GetPinyin(req.Title, w.Content.UrlTokenType == config.UrlTokenTypeSort)
	}
	tag.UrlToken = w.VerifyTagUrlToken(req.UrlToken, tag.Id)

	if tag.FirstLetter == "" {
		letter := "A"
		if tag.UrlToken != "-" {
			letter = string(tag.UrlToken[0])
		}
		tag.FirstLetter = strings.ToUpper(letter)
	}

	err = w.DB.Save(tag).Error

	if err != nil {
		return
	}

	if newPost && tag.Status == config.ContentStatusOK {
		link := w.GetUrl("tag", tag, 0)
		go w.PushArchive(link)
		if w.PluginSitemap.AutoBuild == 1 {
			_ = w.AddonSitemap("tag", link, time.Unix(tag.CreatedTime, 0).Format("2006-01-02"))
		}
	}

	return
}

func (w *Website) SaveTagData(itemId uint, tagNames []string) error {
	if len(tagNames) == 0 {
		w.DB.Where("`item_id` = ?", itemId).Delete(&model.TagData{})
		return nil
	}
	var tagIds = make([]uint, 0, len(tagNames))
	for _, tagName := range tagNames {
		if tagName == "" {
			continue
		}
		tag, err := w.GetTagByTitle(tagName)
		if err != nil {
			newToken := library.GetPinyin(tagName, w.Content.UrlTokenType == config.UrlTokenTypeSort)
			newToken = w.VerifyTagUrlToken(newToken, 0)
			letter := "A"
			if len(newToken) > 0 && newToken != "-" {
				letter = string(newToken[0])
			}
			tag = &model.Tag{
				Title:       tagName,
				UrlToken:    newToken,
				FirstLetter: strings.ToUpper(letter),
				Status:      1,
			}
			w.DB.Where("`title` = ?", tag.Title).FirstOrCreate(tag)

			link := w.GetUrl("tag", tag, 0)
			go w.PushArchive(link)
			if w.PluginSitemap.AutoBuild == 1 {
				_ = w.AddonSitemap("tag", link, time.Unix(tag.CreatedTime, 0).Format("2006-01-02"))
			}
		}
		tagIds = append(tagIds, tag.Id)
		tagData := model.TagData{
			TagId:  tag.Id,
			ItemId: itemId,
		}
		w.DB.Where("`item_id` = ? and `tag_id` = ?", itemId, tagData.TagId).FirstOrCreate(&tagData)
	}
	w.DB.Where("`item_id` = ? and `tag_id` not in(?)", itemId, tagIds).Delete(&model.TagData{})

	return nil
}

func (w *Website) GetTagsByItemId(itemId uint) []*model.Tag {
	var tags []*model.Tag
	var tagIds []uint
	err := w.DB.Model(&model.TagData{}).Where("`item_id` = ?", itemId).Pluck("tag_id", &tagIds).Error
	if err != nil {
		return nil
	}
	if len(tagIds) > 0 {
		w.DB.Where("id IN(?)", tagIds).Find(&tags)
	}

	return tags
}

func (w *Website) VerifyTagUrlToken(urlToken string, id uint) string {
	index := 0
	for {
		tmpToken := urlToken
		if index > 0 {
			tmpToken = fmt.Sprintf("%s-%d", urlToken, index)
		}
		// 判断分类
		_, err := w.GetCategoryByUrlToken(tmpToken)
		if err == nil {
			index++
			continue
		}
		// 判断archive
		tmpTag, err := w.GetTagByUrlToken(tmpToken)
		if err == nil && tmpTag.Id != id {
			index++
			continue
		}
		urlToken = tmpToken
		break
	}

	return urlToken
}
