package provider

import (
	"errors"
	"fmt"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/fulltext"
	"kandaoni.com/anqicms/request"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (w *Website) GetTagList(itemId int64, title string, categoryIds []uint, firstLetter string, currentPage, pageSize int, offset int, order string) ([]*model.Tag, int64, error) {
	var tags []*model.Tag
	if currentPage > 1 {
		offset = (currentPage - 1) * pageSize
	}
	var total int64

	builder := w.DB.WithContext(w.Ctx()).Model(&model.Tag{}).Order(order)
	if firstLetter != "" {
		builder = builder.Where("`first_letter` = ?", firstLetter)
	}
	if itemId != 0 {
		var ids []uint
		w.DB.WithContext(w.Ctx()).Model(&model.TagData{}).Where("`item_id` = ?", itemId).Pluck("tag_id", &ids)
		if len(ids) == 0 {
			// 不用再查询了，直接返回结果
			return tags, 0, nil
		}
		builder = builder.Where("`id` IN(?)", ids)
	}
	if title != "" {
		builder = builder.Where("`title` like ?", "%"+title+"%")
	}
	if len(categoryIds) > 0 {
		if len(categoryIds) == 1 {
			builder = builder.Where("`category_id` = ?", categoryIds[0])
		} else {
			builder = builder.Where("`category_id` IN(?)", categoryIds)
		}
	}

	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&tags).Error
	if err != nil {
		return nil, 0, err
	}

	return tags, total, nil
}

func (w *Website) GetTagsByIds(ids []uint) []*model.Tag {
	var tags []*model.Tag
	w.DB.WithContext(w.Ctx()).Model(&model.Tag{}).Where("`id` IN(?)", ids).Find(&tags)

	return tags
}

func (w *Website) GetTagById(id uint) (*model.Tag, error) {
	var tag model.Tag
	if err := w.DB.WithContext(w.Ctx()).Where("id = ?", id).First(&tag).Error; err != nil {
		return nil, err
	}

	return &tag, nil
}

func (w *Website) GetTagContentById(id uint) (*model.TagContent, error) {
	var tagContent model.TagContent
	if err := w.DB.WithContext(w.Ctx()).Where("id = ?", id).First(&tagContent).Error; err != nil {
		return nil, err
	}
	tagContent.Content = w.ReplaceContentUrl(tagContent.Content, true)

	return &tagContent, nil
}

func (w *Website) GetTagByUrlToken(urlToken string) (*model.Tag, error) {
	var tag model.Tag
	if err := w.DB.WithContext(w.Ctx()).Where("url_token = ?", urlToken).First(&tag).Error; err != nil {
		return nil, err
	}
	tag.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
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
	w.DB.Unscoped().Where("`id` = ?", tag.Id).Delete(model.TagContent{})

	//执行删除操作
	err = w.DB.Delete(tag).Error

	if err != nil {
		return err
	}
	w.RemoveFulltextIndex(fulltext.TinyArchive{Id: int64(tag.Id), Type: fulltext.TagType})

	return nil
}

func (w *Website) SaveTag(req *request.PluginTag) (tag *model.Tag, err error) {
	newPost := false
	req.Title = strings.TrimSpace(req.Title)
	if len(req.Title) == 0 {
		return nil, errors.New(w.Tr("TagNameCannotBeEmpty"))
	}
	if req.Id > 0 {
		tag, err = w.GetTagById(req.Id)
		if err != nil {
			// 表示不存在，则新建一个
			tag = &model.Tag{
				Status: 1,
			}
			tag.Id = req.Id
			newPost = true
		}
	} else {
		tag, err = w.GetTagByTitle(req.Title)
		if err != nil {
			tag = &model.Tag{
				Status: 1,
			}
			newPost = true
		}
	}
	tag.Title = req.Title
	tag.SeoTitle = req.SeoTitle
	tag.Keywords = req.Keywords
	tag.Status = 1
	tag.Description = req.Description
	tag.FirstLetter = req.FirstLetter
	tag.CategoryId = req.CategoryId
	tag.Template = req.Template
	tag.Logo = req.Logo
	if tag.Logo != "" {
		tag.Logo = strings.TrimPrefix(tag.Logo, w.PluginStorage.StorageUrl)
	}
	// 判断重复
	tag.UrlToken = w.VerifyTagUrlToken(req.UrlToken, tag.Title, tag.Id)

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
	// 保存 content
	if len(req.Content) > 0 {
		// 将单个&nbsp;替换为空格
		req.Content = library.ReplaceSingleSpace(req.Content)
		// todo 应该只替换 src,href 中的 baseUrl
		req.Content = w.ReplaceContentUrl(req.Content, false)
		// 过滤外链
		if w.Content.FilterOutlink == 1 || w.Content.FilterOutlink == 2 {
			baseHost := ""
			urls, err := url.Parse(w.System.BaseUrl)
			if err == nil {
				baseHost = urls.Host
			}

			re, _ := regexp.Compile(`(?i)<a.*?href="(.+?)".*?>(.*?)</a>`)
			req.Content = re.ReplaceAllStringFunc(req.Content, func(s string) string {
				match := re.FindStringSubmatch(s)
				if len(match) < 3 {
					return s
				}
				aUrl, err2 := url.Parse(match[1])
				if err2 == nil {
					if aUrl.Host != "" && aUrl.Host != baseHost {
						//过滤外链
						if w.Content.FilterOutlink == 1 {
							return match[2]
						} else if !strings.Contains(match[0], "nofollow") {
							newUrl := match[1] + `" rel="nofollow`
							s = strings.Replace(s, match[1], newUrl, 1)
						}
					}
				}
				return s
			})
			// 匹配Markdown [link](url)
			// 由于不支持零宽断言，因此匹配所有
			re, _ = regexp.Compile(`!?\[([^]]*)\]\(([^)]+)\)`)
			req.Content = re.ReplaceAllStringFunc(req.Content, func(s string) string {
				// 过滤掉 ! 开头的
				if strings.HasPrefix(s, "!") {
					return s
				}
				match := re.FindStringSubmatch(s)
				if len(match) < 3 {
					return s
				}
				aUrl, err2 := url.Parse(match[2])
				if err2 == nil {
					if aUrl.Host != "" && aUrl.Host != baseHost {
						//过滤外链
						if w.Content.FilterOutlink == 1 {
							return match[1]
						}
						// 添加 nofollow 不在这里处理，因为md不支持
					}
				}
				return s
			})
		}
		tagContent := &model.TagContent{
			Id:      tag.Id,
			Content: req.Content,
		}
		err = w.DB.Save(&tagContent).Error
		if err != nil {
			return nil, err
		}
	}

	if newPost && tag.Status == config.ContentStatusOK {
		link := w.GetUrl("tag", tag, 0)
		go w.PushArchive(link)
		if w.PluginSitemap.AutoBuild == 1 {
			_ = w.AddonSitemap("tag", link, time.Unix(tag.CreatedTime, 0).Format("2006-01-02"), tag)
		}
	}
	if w.PluginFulltext.UseTag {
		w.AddFulltextIndex(fulltext.TinyArchive{
			Id:          int64(tag.Id),
			Type:        fulltext.TagType,
			Title:       tag.Title,
			Keywords:    tag.Keywords,
			Description: tag.Description,
		})
		w.FlushIndex()
	}

	return
}

func (w *Website) SaveTagData(itemId int64, tagNames []string) error {
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
			newToken := w.VerifyTagUrlToken("", tagName, 0)
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
				go w.AddonSitemap("tag", link, time.Unix(tag.CreatedTime, 0).Format("2006-01-02"), tag)
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

func (w *Website) GetTagsByItemId(itemId int64) []*model.Tag {
	var tags []*model.Tag
	var tagIds []uint
	err := w.DB.WithContext(w.Ctx()).Model(&model.TagData{}).Where("`item_id` = ?", itemId).Pluck("tag_id", &tagIds).Error
	if err != nil {
		return nil
	}
	if len(tagIds) > 0 {
		w.DB.Where("id IN(?)", tagIds).Find(&tags)
	}

	return tags
}

func (w *Website) VerifyTagUrlToken(urlToken string, title string, id uint) string {
	newToken := false
	if urlToken == "" {
		urlToken = library.GetPinyin(title, w.Content.UrlTokenType == config.UrlTokenTypeSort)
		if len(urlToken) > 100 {
			urlToken = urlToken[:100]
		}
		lastId := id
		if id == 0 {
			lastId = model.GetNextTagId(w.DB)
		}
		urlToken += "-t" + strconv.Itoa(int(lastId))
		newToken = true
	}
	if newToken == false {
		urlToken = strings.ToLower(library.ParseUrlToken(urlToken))
		// 防止超出长度
		if len(urlToken) > 150 {
			urlToken = urlToken[:150]
		}
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
	}

	return urlToken
}
