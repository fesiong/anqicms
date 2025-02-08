package provider

import (
	"encoding/json"
	"fmt"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"regexp"
	"strings"
)

func (w *Website) ReplaceValues(req *request.PluginReplaceRequest) (updateCount int64) {
	// 可以替换的地方： setting|archive|category|tag|anchor|keyword|comment|attachment
	for _, key := range req.Places {
		switch key {
		case "setting":
			total := w.replaceSettingValues(req.Keywords, req.ReplaceTag)
			updateCount += total
			break
		case "archive":
			// 正式表
			total := w.replaceArchiveValues(req.Keywords, req.ReplaceTag)
			updateCount += total
			// 草稿
			total = w.replaceArchiveDraftValues(req.Keywords, req.ReplaceTag)
			updateCount += total
			break
		case "category":
			total := w.replaceCategoryValues(req.Keywords, req.ReplaceTag)
			updateCount += total
			break
		case "tag":
			total := w.replaceTagValues(req.Keywords, req.ReplaceTag)
			updateCount += total
			break
		case "anchor":
			total := w.replaceAnchorValues(req.Keywords, req.ReplaceTag)
			updateCount += total
			break
		case "keyword":
			total := w.replaceKeywordValues(req.Keywords, req.ReplaceTag)
			updateCount += total
			break
		case "comment":
			total := w.replaceCommentValues(req.Keywords, req.ReplaceTag)
			updateCount += total
			break
		case "attachment":
			total := w.replaceAttachmentValues(req.Keywords, req.ReplaceTag)
			updateCount += total
			break
		default:
			break
		}
	}

	return updateCount
}

func (w *Website) replaceSettingValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	// 需要reflect
	var settings []*model.Setting
	w.DB.Find(&settings)
	for _, item := range settings {
		var values map[string]interface{}
		err := json.Unmarshal([]byte(item.Value), &values)
		if err == nil {
			needUpdate := false
			for k, v := range values {
				if k == "extra_fields" {
					val, ok := v.([]config.ExtraField)
					if ok {
						innerUpdate := false
						for i := range val {
							val2 := w.replaceContentText(val[i].Value, replacer)
							if val2 != val[i].Value {
								updateCount++
								innerUpdate = true
								val[i].Value = val2
							}
						}
						if innerUpdate {
							needUpdate = true
							values[k] = val
						}
					}
				} else if val, ok := v.(string); ok {
					val2 := w.replaceContentText(val, replacer)
					if val2 != val {
						updateCount++
						needUpdate = true
						values[k] = val2
					}
				}
			}
			if needUpdate {
				itemValue, err := json.Marshal(values)
				if err == nil {
					w.DB.Model(&model.Setting{}).Where("`key` = ?", item.Key).UpdateColumn("value", string(itemValue))
				}
			}
		}
	}
	if updateCount > 0 {
		w.InitSetting()
	}
	return updateCount
}

func (w *Website) replaceArchiveValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	startId := int64(0)
	var archives []*model.Archive
	for {
		tx := w.DB.Model(&model.Archive{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&archives)
		if len(archives) == 0 {
			break
		}
		startId = archives[len(archives)-1].Id
		for _, archive := range archives {
			needUpdate := false
			title := w.replaceContentText(archive.Title, replacer)
			if archive.Title != title {
				archive.Title = title
				updateCount++
				needUpdate = true
			}
			seoTitle := w.replaceContentText(archive.SeoTitle, replacer)
			if archive.SeoTitle != seoTitle {
				archive.SeoTitle = seoTitle
				updateCount++
				needUpdate = true
			}
			keywords := w.replaceContentText(archive.Keywords, replacer)
			if archive.Keywords != keywords {
				archive.Keywords = keywords
				updateCount++
				needUpdate = true
			}
			description := w.replaceContentText(archive.Description, replacer)
			if archive.Description != description {
				archive.Description = description
				updateCount++
				needUpdate = true
			}
			for i, img := range archive.Images {
				img2 := w.replaceContentText(img, replacer)
				if img != img2 {
					archive.Images[i] = img2
					updateCount++
					needUpdate = true
				}
			}
			//替换完了
			if needUpdate {
				w.DB.Model(archive).Updates(archive)
			}
			var archiveData model.ArchiveData
			w.DB.Where("id = ?", archive.Id).Take(&archiveData)
			var content string
			if replaceTag {
				content = w.replaceContentText(archiveData.Content, replacer)
			} else {
				content = w.ReplaceContentFromConfig(archiveData.Content, replacer)
			}
			if content != archiveData.Content {
				updateCount++
				archiveData.Content = content
				w.DB.Model(&archiveData).UpdateColumns(archiveData)
			}
			// extra
			result := map[string]interface{}{}
			module := w.GetModuleFromCache(archive.ModuleId)
			if module != nil {
				var fields []string
				for _, v := range module.Fields {
					fields = append(fields, "`"+v.FieldName+"`")
				}
				//从数据库中取出来
				if len(fields) > 0 {
					w.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Select(strings.Join(fields, ",")).Scan(&result)
					//extra的CheckBox的值
					innerUpdate := false
					for k, v := range result {
						val, ok := v.(string)
						if ok {
							val2 := w.replaceContentText(val, replacer)
							if val2 != val {
								result[k] = val2
								updateCount++
								innerUpdate = true
							}
						}
					}
					if innerUpdate {
						w.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Updates(result)
					}
				}
			}
		}
	}

	return updateCount
}

func (w *Website) replaceArchiveDraftValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	startId := int64(0)
	var archives []*model.ArchiveDraft
	for {
		tx := w.DB.Model(&model.ArchiveDraft{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&archives)
		if len(archives) == 0 {
			break
		}
		startId = archives[len(archives)-1].Id
		for _, archive := range archives {
			needUpdate := false
			title := w.replaceContentText(archive.Title, replacer)
			if archive.Title != title {
				archive.Title = title
				updateCount++
				needUpdate = true
			}
			seoTitle := w.replaceContentText(archive.SeoTitle, replacer)
			if archive.SeoTitle != seoTitle {
				archive.SeoTitle = seoTitle
				updateCount++
				needUpdate = true
			}
			keywords := w.replaceContentText(archive.Keywords, replacer)
			if archive.Keywords != keywords {
				archive.Keywords = keywords
				updateCount++
				needUpdate = true
			}
			description := w.replaceContentText(archive.Description, replacer)
			if archive.Description != description {
				archive.Description = description
				updateCount++
				needUpdate = true
			}
			for i, img := range archive.Images {
				img2 := w.replaceContentText(img, replacer)
				if img != img2 {
					archive.Images[i] = img2
					updateCount++
					needUpdate = true
				}
			}
			//替换完了
			if needUpdate {
				w.DB.Model(archive).Updates(archive)
			}
			var archiveData model.ArchiveData
			w.DB.Where("id = ?", archive.Id).Take(&archiveData)
			var content string
			if replaceTag {
				content = w.replaceContentText(archiveData.Content, replacer)
			} else {
				content = w.ReplaceContentFromConfig(archiveData.Content, replacer)
			}
			if content != archiveData.Content {
				updateCount++
				archiveData.Content = content
				w.DB.Model(&archiveData).UpdateColumns(archiveData)
			}
			// extra
			result := map[string]interface{}{}
			module := w.GetModuleFromCache(archive.ModuleId)
			if module != nil {
				var fields []string
				for _, v := range module.Fields {
					fields = append(fields, "`"+v.FieldName+"`")
				}
				//从数据库中取出来
				if len(fields) > 0 {
					w.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Select(strings.Join(fields, ",")).Scan(&result)
					//extra的CheckBox的值
					innerUpdate := false
					for k, v := range result {
						val, ok := v.(string)
						if ok {
							val2 := w.replaceContentText(val, replacer)
							if val2 != val {
								result[k] = val2
								updateCount++
								innerUpdate = true
							}
						}
					}
					if innerUpdate {
						w.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Updates(result)
					}
				}
			}
		}
	}

	return updateCount
}

func (w *Website) replaceCategoryValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	var categories []*model.Category
	w.DB.Find(&categories)
	for _, category := range categories {
		needUpdate := false
		title := w.replaceContentText(category.Title, replacer)
		if category.Title != title {
			category.Title = title
			updateCount++
			needUpdate = true
		}
		seoTitle := w.replaceContentText(category.SeoTitle, replacer)
		if category.SeoTitle != seoTitle {
			category.SeoTitle = seoTitle
			updateCount++
			needUpdate = true
		}
		keywords := w.replaceContentText(category.Keywords, replacer)
		if category.Keywords != keywords {
			category.Keywords = keywords
			updateCount++
			needUpdate = true
		}
		description := w.replaceContentText(category.Description, replacer)
		if category.Description != description {
			category.Description = description
			updateCount++
			needUpdate = true
		}
		for i, img := range category.Images {
			img2 := w.replaceContentText(img, replacer)
			if img != img2 {
				category.Images[i] = img2
				updateCount++
				needUpdate = true
			}
		}
		logo := w.replaceContentText(category.Logo, replacer)
		if category.Logo != logo {
			category.Logo = logo
			updateCount++
			needUpdate = true
		}
		var content string
		if replaceTag {
			content = w.replaceContentText(category.Content, replacer)
		} else {
			content = w.ReplaceContentFromConfig(category.Content, replacer)
		}
		if content != category.Content {
			updateCount++
			category.Content = content
			needUpdate = true
		}
		//替换完了
		if needUpdate {
			w.DB.Model(category).Updates(category)
		}
	}
	return updateCount
}

func (w *Website) replaceTagValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	startId := uint(0)
	var tags []*model.Tag
	for {
		tx := w.DB.Model(&model.Tag{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&tags)
		if len(tags) == 0 {
			break
		}
		startId = tags[len(tags)-1].Id
		for _, tag := range tags {
			needUpdate := false
			title := w.replaceContentText(tag.Title, replacer)
			if tag.Title != title {
				tag.Title = title
				updateCount++
				needUpdate = true
			}
			seoTitle := w.replaceContentText(tag.SeoTitle, replacer)
			if tag.SeoTitle != seoTitle {
				tag.SeoTitle = seoTitle
				updateCount++
				needUpdate = true
			}
			keywords := w.replaceContentText(tag.Keywords, replacer)
			if tag.Keywords != keywords {
				tag.Keywords = keywords
				updateCount++
				needUpdate = true
			}
			description := w.replaceContentText(tag.Description, replacer)
			if tag.Description != description {
				tag.Description = description
				updateCount++
				needUpdate = true
			}
			if needUpdate {
				w.DB.Model(tag).Updates(tag)
			}
		}
	}

	return updateCount
}

func (w *Website) replaceAnchorValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	startId := uint(0)
	var anchors []*model.Anchor
	for {
		tx := w.DB.Model(&model.Anchor{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&anchors)
		if len(anchors) == 0 {
			break
		}
		startId = anchors[len(anchors)-1].Id
		for _, item := range anchors {
			needUpdate := false
			title := w.replaceContentText(item.Title, replacer)
			if item.Title != title {
				item.Title = title
				updateCount++
				needUpdate = true
			}
			link := w.replaceContentText(item.Link, replacer)
			if item.Link != link {
				item.Link = link
				updateCount++
				needUpdate = true
			}
			if needUpdate {
				w.DB.Model(item).Updates(item)
			}
		}
	}

	return updateCount
}

func (w *Website) replaceKeywordValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	startId := uint(0)
	var keywords []*model.Keyword
	for {
		tx := w.DB.Model(&model.Keyword{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&keywords)
		if len(keywords) == 0 {
			break
		}
		startId = keywords[len(keywords)-1].Id
		for _, item := range keywords {
			needUpdate := false
			title := w.replaceContentText(item.Title, replacer)
			if item.Title != title {
				item.Title = title
				updateCount++
				needUpdate = true
			}
			if needUpdate {
				w.DB.Model(item).Updates(item)
			}
		}
	}

	return updateCount
}

func (w *Website) replaceCommentValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	startId := uint(0)
	var comments []*model.Comment
	for {
		tx := w.DB.Model(&model.Comment{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&comments)
		if len(comments) == 0 {
			break
		}
		startId = comments[len(comments)-1].Id
		for _, item := range comments {
			needUpdate := false
			userName := w.replaceContentText(item.UserName, replacer)
			if item.UserName != userName {
				item.UserName = userName
				updateCount++
				needUpdate = true
			}
			var content string
			if replaceTag {
				content = w.replaceContentText(item.Content, replacer)
			} else {
				content = w.ReplaceContentFromConfig(item.Content, replacer)
			}
			if item.Content != content {
				item.Content = content
				updateCount++
				needUpdate = true
			}
			if needUpdate {
				w.DB.Model(item).Updates(item)
			}
		}
	}

	return updateCount
}

func (w *Website) replaceAttachmentValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	startId := uint(0)
	var attachments []*model.Attachment
	for {
		tx := w.DB.Model(&model.Attachment{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&attachments)
		if len(attachments) == 0 {
			break
		}
		startId = attachments[len(attachments)-1].Id
		for _, item := range attachments {
			needUpdate := false
			filename := w.replaceContentText(item.FileName, replacer)
			if item.FileName != filename {
				item.FileName = filename
				updateCount++
				needUpdate = true
			}
			if needUpdate {
				w.DB.Model(item).Updates(item)
			}
		}
	}

	return updateCount
}

func (w *Website) replaceTemplateValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	startId := uint(0)
	var attachments []*model.Attachment
	for {
		tx := w.DB.Model(&model.Attachment{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&attachments)
		if len(attachments) == 0 {
			break
		}
		startId = attachments[len(attachments)-1].Id
		for _, item := range attachments {
			needUpdate := false
			filename := w.replaceContentText(item.FileName, replacer)
			if item.FileName != filename {
				item.FileName = filename
				updateCount++
				needUpdate = true
			}
			if needUpdate {
				w.DB.Model(item).Updates(item)
			}
		}
	}

	return updateCount
}

// ReplaceContentFromConfig 替换文章内容
func (w *Website) ReplaceContentFromConfig(content string, replacer []config.ReplaceKeyword) string {
	if content == "" || len(replacer) <= 0 {
		return content
	}

	// 替换功能，只替换内容，不替换标签， 因此需要将标签存起来，并在最后还原
	var replaced = map[string]string{}
	if strings.Contains(content, "<") {
		re, _ := regexp.Compile(`<[^>]+>`)
		results := re.FindAllString(content, -1)
		for i, v := range results {
			key := fmt.Sprintf("{$%d}", i)
			replaced[key] = v
			content = strings.ReplaceAll(content, v, key)
		}
	}
	content = w.replaceContentText(content, replacer)
	for key, val := range replaced {
		content = strings.ReplaceAll(content, key, val)
	}

	return content
}

func (w *Website) replaceContentText(content string, replacer []config.ReplaceKeyword) string {
	if content == "" || len(replacer) <= 0 {
		return content
	}

	var re *regexp.Regexp
	var err error
	for _, v := range replacer {
		// 增加支持正则表达式替换
		if strings.HasPrefix(v.From, "{") && strings.HasSuffix(v.From, "}") && len(v.From) > 2 {
			newWord := v.From[1 : len(v.From)-1]
			// 支持特定规则：邮箱地址，手机号，电话号码，网址、微信号，QQ号，
			if newWord == "邮箱地址" {
				re, err = regexp.Compile(`\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*`)
			} else if newWord == "日期" {
				re, err = regexp.Compile(`\d{2,4}[\-/年月日]\d{1,2}[\-/年月日]?(\d{1,2}[\-/年月日]?)?`)
			} else if newWord == "时间" {
				re, err = regexp.Compile(`\d{2}[:时分秒]\d{2}[:时分秒]?(\d{2}[:时分秒]?)?`)
			} else if newWord == "电话号码" {
				re, err = regexp.Compile(`[+\d]{2}[\d\-+\s]{5,16}`)
			} else if newWord == "QQ号" {
				re, err = regexp.Compile(`[1-9]\d{4,10}`)
			} else if newWord == "微信号" {
				re, err = regexp.Compile(`[a-zA-Z][a-zA-Z\d_-]{5,19}`)
			} else if newWord == "网址" {
				re, err = regexp.Compile(`(?i)((http|ftp|https)://)?[\w\-_]+(\.[\w\-_]+)+([\w\-.,@?^=%&:/~+#]*[\w\-@?^=%&/~+#])?`)
			} else {
				re, err = regexp.Compile(newWord)
			}

			if err == nil {
				content = re.ReplaceAllString(content, v.To)
			}
			continue
		}
		content = strings.ReplaceAll(content, v.From, v.To)
	}

	return content
}
