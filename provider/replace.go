package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) ReplaceValues(req *request.PluginReplaceRequest) (updateCount int64) {
	// 可以替换的地方： setting|archive|category|tag|anchor|keyword|comment|attachment|nav|link|redirect|place|guestbook|template
	for _, key := range req.Places {
		switch key {
		case "setting":
			total := w.replaceSettingValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "archive":
			// 正式表
			total := w.replaceArchiveValues(req.Keywords, req.ReplaceTag)
			updateCount += total
			// 草稿
			total = w.replaceArchiveDraftValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "category":
			total := w.replaceCategoryValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "tag":
			total := w.replaceTagValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "anchor":
			total := w.replaceAnchorValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "keyword":
			total := w.replaceKeywordValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "comment":
			total := w.replaceCommentValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "attachment":
			total := w.replaceAttachmentValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "nav":
			total := w.replaceNavValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "link":
			total := w.replaceLinkValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "redirect":
			total := w.replaceRedirectValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "place":
			total := w.replacePlaceValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "guestbook":
			total := w.replaceGuestbookValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		case "template":
			total := w.replaceTemplateValues(req.Keywords, req.ReplaceTag)
			updateCount += total
		}
	}

	return updateCount
}

func (w *Website) replaceSettingValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	var settings []*model.Setting
	w.DB.Find(&settings)
	for _, item := range settings {
		var values map[string]interface{}
		err := json.Unmarshal([]byte(item.Value), &values)
		if err == nil {
			needUpdate := false
			for k, v := range values {
				changed, newV := w.replaceSettingValue(v, replacer)
				if changed > 0 {
					updateCount += changed
					needUpdate = true
					values[k] = newV
				}
			}
			if needUpdate {
				itemValue, err := json.Marshal(values)
				if err == nil {
					w.DB.Model(&model.Setting{}).Where("`key` = ?", item.Key).UpdateColumn("value", string(itemValue))
				}
			}
		} else {
			// 非 JSON 对象（纯字符串或数组），尝试直接替换
			var val interface{}
			if err := json.Unmarshal([]byte(item.Value), &val); err == nil {
				changed, newVal := w.replaceSettingValue(val, replacer)
				if changed > 0 {
					updateCount += changed
					itemValue, err := json.Marshal(newVal)
					if err == nil {
						w.DB.Model(&model.Setting{}).Where("`key` = ?", item.Key).UpdateColumn("value", string(itemValue))
					}
				}
			}
		}
	}
	if updateCount > 0 {
		w.InitSetting()
	}
	return updateCount
}

// replaceSettingValue 递归替换任意层级的值（map / slice / string），
// 返回被替换的字符串数量以及替换后的新值。
func (w *Website) replaceSettingValue(v interface{}, replacer []config.ReplaceKeyword) (int64, interface{}) {
	switch val := v.(type) {
	case string:
		newStr := w.replaceContentText(val, replacer)
		if newStr != val {
			return 1, newStr
		}
		return 0, val
	case map[string]interface{}:
		changed := int64(0)
		needUpdate := false
		for k, sub := range val {
			c, newSub := w.replaceSettingValue(sub, replacer)
			if c > 0 {
				changed += c
				val[k] = newSub
				needUpdate = true
			}
		}
		if !needUpdate {
			return 0, val
		}
		return changed, val
	case []interface{}:
		changed := int64(0)
		needUpdate := false
		for i, sub := range val {
			c, newSub := w.replaceSettingValue(sub, replacer)
			if c > 0 {
				changed += c
				val[i] = newSub
				needUpdate = true
			}
		}
		if !needUpdate {
			return 0, val
		}
		return changed, val
	default:
		return 0, val
	}
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
		}
	}
	// 批量处理所有模型的 extra 字段（多模型）
	updateCount += w.replaceArchiveModuleValues(replacer)

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
		}
	}
	// 批量处理所有模型的 extra 字段（多模型）
	updateCount += w.replaceArchiveModuleValues(replacer)

	return updateCount
}

// replaceArchiveModuleValues 批量替换所有模型表（module.TableName）中的自定义字段值。
// 由于文档分属不同的 module（多模型），这里按 module 分组批量处理，
// 避免在 replaceArchiveValues / replaceArchiveDraftValues 中逐条查询模型表。
// archive 表和 archive_draft 表共用同一批模型表，因此一次调用即可覆盖两者。
func (w *Website) replaceArchiveModuleValues(replacer []config.ReplaceKeyword) (updateCount int64) {
	modules := w.GetCacheModules()
	for i := range modules {
		module := &modules[i]
		if module.TableName == "" || len(module.Fields) == 0 {
			continue
		}
		// 只读取自定义字段，避免读到 id 等系统字段时被错误覆盖
		var fieldNames []string
		for _, v := range module.Fields {
			fieldNames = append(fieldNames, "`"+v.FieldName+"`")
		}
		startId := int64(0)
		for {
			var rows []map[string]interface{}
			err := w.DB.Table(module.TableName).
				Select("id, " + strings.Join(fieldNames, ",")).
				Where("`id` > ?", startId).
				Order("id asc").
				Limit(1000).
				Scan(&rows).Error
			if err != nil || len(rows) == 0 {
				break
			}
			// 记录本批次最大 id 用于翻页
			var lastId int64
			for _, row := range rows {
				id := row["id"]
				if id != nil {
					switch v := id.(type) {
					case int64:
						lastId = v
					case int:
						lastId = int64(v)
					case int32:
						lastId = int64(v)
					}
				}
				innerUpdate := false
				for k, v := range row {
					if k == "id" {
						continue
					}
					changed, newV := w.replaceModuleExtraValue(v, replacer)
					if changed > 0 {
						row[k] = newV
						updateCount += changed
						innerUpdate = true
					}
				}
				if innerUpdate {
					// 去掉 id 字段，只更新自定义字段
					delete(row, "id")
					w.DB.Table(module.TableName).Where("`id` = ?", id).Updates(row)
				}
			}
			if lastId == 0 {
				break
			}
			startId = lastId
		}
	}

	return updateCount
}
// 自定义字段可能存储为纯字符串、[]byte（JSON 列）、JSON 字符串（texts/images/timeline 等）、
// map / slice / number / bool，这里统一处理，对任意层级的字符串叶子执行替换。
// 返回被替换的字符串数量以及替换后的新值。
func (w *Website) replaceModuleExtraValue(v interface{}, replacer []config.ReplaceKeyword) (int64, interface{}) {
	switch val := v.(type) {
	case string:
		newStr := w.replaceContentText(val, replacer)
		if newStr != val {
			return 1, newStr
		}
		return 0, val
	case []byte:
		// JSON 列可能返回 []byte，先尝试当作 JSON 解析递归处理；
		// 若不是 JSON（纯文本），则直接替换字符串。
		str := string(val)
		var parsed interface{}
		if json.Unmarshal(val, &parsed) == nil {
			changed, newParsed := w.replaceModuleExtraValue(parsed, replacer)
			if changed > 0 {
				buf, err := json.Marshal(newParsed)
				if err == nil {
					return changed, buf
				}
			}
			return 0, val
		}
		newStr := w.replaceContentText(str, replacer)
		if newStr != str {
			return 1, newStr
		}
		return 0, val
	case map[string]interface{}:
		changed := int64(0)
		needUpdate := false
		for k, sub := range val {
			c, newSub := w.replaceModuleExtraValue(sub, replacer)
			if c > 0 {
				changed += c
				val[k] = newSub
				needUpdate = true
			}
		}
		if !needUpdate {
			return 0, val
		}
		return changed, val
	case []interface{}:
		changed := int64(0)
		needUpdate := false
		for i, sub := range val {
			c, newSub := w.replaceModuleExtraValue(sub, replacer)
			if c > 0 {
				changed += c
				val[i] = newSub
				needUpdate = true
			}
		}
		if !needUpdate {
			return 0, val
		}
		return changed, val
	default:
		return 0, val
	}
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
		// 替换分类自定义字段（multi-model CategoryFields 存放在 Extra 中）
		if len(category.Extra) > 0 {
			extraUpdate := false
			for k, v := range category.Extra {
				changed, newV := w.replaceModuleExtraValue(v, replacer)
				if changed > 0 {
					updateCount += changed
					category.Extra[k] = newV
					extraUpdate = true
				}
			}
			if extraUpdate {
				needUpdate = true
			}
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
			if replaceTag {
				fileLocation := w.replaceContentText(item.FileLocation, replacer)
				if item.FileLocation != fileLocation {
					item.FileLocation = fileLocation
					updateCount++
					needUpdate = true
				}
				fileLogo := w.replaceContentText(item.Logo, replacer)
				if item.Logo != fileLogo {
					item.Logo = fileLogo
					updateCount++
					needUpdate = true
				}
			}
			if needUpdate {
				w.DB.Model(item).Updates(item)
			}
		}
	}

	return updateCount
}

// replaceTemplateValues 遍历当前站点模板目录下的所有模板文件，对文件内容进行替换。
// 模板是磁盘文件而非数据库记录，因此这里直接读写文件。
func (w *Website) replaceTemplateValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	tplDir := w.GetTemplateDir()
	if tplDir == "" {
		return 0
	}
	// filepath.Walk 会递归遍历模板目录
	_ = filepath.Walk(tplDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// 仅处理模板文件，跳过非文本文件
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".html", ".htm", ".tpl", ".txt", ".js", ".css", ".json", ".yml", ".yaml", ".xml", ".svg":
		default:
			return nil
		}
		buf, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(buf)
		var newContent string
		if replaceTag {
			newContent = w.replaceContentText(content, replacer)
		} else {
			newContent = w.ReplaceContentFromConfig(content, replacer)
		}
		if newContent == content {
			return nil
		}
		updateCount++
		_ = os.WriteFile(path, []byte(newContent), info.Mode())
		return nil
	})

	return updateCount
}

// replaceNavValues 替换导航菜单内容
func (w *Website) replaceNavValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	var navs []*model.Nav
	w.DB.Find(&navs)
	for _, nav := range navs {
		needUpdate := false
		title := w.replaceContentText(nav.Title, replacer)
		if nav.Title != title {
			nav.Title = title
			updateCount++
			needUpdate = true
		}
		subTitle := w.replaceContentText(nav.SubTitle, replacer)
		if nav.SubTitle != subTitle {
			nav.SubTitle = subTitle
			updateCount++
			needUpdate = true
		}
		description := w.replaceContentText(nav.Description, replacer)
		if nav.Description != description {
			nav.Description = description
			updateCount++
			needUpdate = true
		}
		link := w.replaceContentText(nav.Link, replacer)
		if nav.Link != link {
			nav.Link = link
			updateCount++
			needUpdate = true
		}
		logo := w.replaceContentText(nav.Logo, replacer)
		if nav.Logo != logo {
			nav.Logo = logo
			updateCount++
			needUpdate = true
		}
		if needUpdate {
			w.DB.Model(nav).Updates(nav)
		}
	}

	return updateCount
}

// replaceLinkValues 替换友情链接内容
func (w *Website) replaceLinkValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	startId := uint(0)
	var links []*model.Link
	for {
		tx := w.DB.Model(&model.Link{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&links)
		if len(links) == 0 {
			break
		}
		startId = links[len(links)-1].Id
		for _, item := range links {
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
			backLink := w.replaceContentText(item.BackLink, replacer)
			if item.BackLink != backLink {
				item.BackLink = backLink
				updateCount++
				needUpdate = true
			}
			myTitle := w.replaceContentText(item.MyTitle, replacer)
			if item.MyTitle != myTitle {
				item.MyTitle = myTitle
				updateCount++
				needUpdate = true
			}
			myLink := w.replaceContentText(item.MyLink, replacer)
			if item.MyLink != myLink {
				item.MyLink = myLink
				updateCount++
				needUpdate = true
			}
			contact := w.replaceContentText(item.Contact, replacer)
			if item.Contact != contact {
				item.Contact = contact
				updateCount++
				needUpdate = true
			}
			remark := w.replaceContentText(item.Remark, replacer)
			if item.Remark != remark {
				item.Remark = remark
				updateCount++
				needUpdate = true
			}
			logo := w.replaceContentText(item.Logo, replacer)
			if item.Logo != logo {
				item.Logo = logo
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

// replaceRedirectValues 替换跳转链接内容
func (w *Website) replaceRedirectValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	var redirects []*model.Redirect
	w.DB.Find(&redirects)
	for _, item := range redirects {
		needUpdate := false
		fromUrl := w.replaceContentText(item.FromUrl, replacer)
		if item.FromUrl != fromUrl {
			item.FromUrl = fromUrl
			updateCount++
			needUpdate = true
		}
		toUrl := w.replaceContentText(item.ToUrl, replacer)
		if item.ToUrl != toUrl {
			item.ToUrl = toUrl
			updateCount++
			needUpdate = true
		}
		if needUpdate {
			w.DB.Model(item).Updates(item)
		}
	}

	return updateCount
}

// replacePlaceValues 替换地区/位置内容
func (w *Website) replacePlaceValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	var places []*model.Place
	w.DB.Find(&places)
	for _, item := range places {
		needUpdate := false
		title := w.replaceContentText(item.Title, replacer)
		if item.Title != title {
			item.Title = title
			updateCount++
			needUpdate = true
		}
		seoTitle := w.replaceContentText(item.SeoTitle, replacer)
		if item.SeoTitle != seoTitle {
			item.SeoTitle = seoTitle
			updateCount++
			needUpdate = true
		}
		keywords := w.replaceContentText(item.Keywords, replacer)
		if item.Keywords != keywords {
			item.Keywords = keywords
			updateCount++
			needUpdate = true
		}
		description := w.replaceContentText(item.Description, replacer)
		if item.Description != description {
			item.Description = description
			updateCount++
			needUpdate = true
		}
		var content string
		if replaceTag {
			content = w.replaceContentText(item.Content, replacer)
		} else {
			content = w.ReplaceContentFromConfig(item.Content, replacer)
		}
		if content != item.Content {
			updateCount++
			item.Content = content
			needUpdate = true
		}
		for i, img := range item.Images {
			img2 := w.replaceContentText(img, replacer)
			if img != img2 {
				item.Images[i] = img2
				updateCount++
				needUpdate = true
			}
		}
		logo := w.replaceContentText(item.Logo, replacer)
		if item.Logo != logo {
			item.Logo = logo
			updateCount++
			needUpdate = true
		}
		if needUpdate {
			w.DB.Model(item).Updates(item)
		}
	}

	return updateCount
}

// replaceGuestbookValues 替换留言板内容
func (w *Website) replaceGuestbookValues(replacer []config.ReplaceKeyword, replaceTag bool) (updateCount int64) {
	startId := uint(0)
	var guestbooks []*model.Guestbook
	for {
		tx := w.DB.Model(&model.Guestbook{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&guestbooks)
		if len(guestbooks) == 0 {
			break
		}
		startId = guestbooks[len(guestbooks)-1].Id
		for _, item := range guestbooks {
			needUpdate := false
			userName := w.replaceContentText(item.UserName, replacer)
			if item.UserName != userName {
				item.UserName = userName
				updateCount++
				needUpdate = true
			}
			contact := w.replaceContentText(item.Contact, replacer)
			if item.Contact != contact {
				item.Contact = contact
				updateCount++
				needUpdate = true
			}
			var content string
			if replaceTag {
				content = w.replaceContentText(item.Content, replacer)
			} else {
				content = w.ReplaceContentFromConfig(item.Content, replacer)
			}
			if content != item.Content {
				updateCount++
				item.Content = content
				needUpdate = true
			}
			refer := w.replaceContentText(item.Refer, replacer)
			if item.Refer != refer {
				item.Refer = refer
				updateCount++
				needUpdate = true
			}
			// 替换 ExtraData 中的字符串值
			if len(item.ExtraData) > 0 {
				innerUpdate := false
				for k, v := range item.ExtraData {
					val, ok := v.(string)
					if !ok {
						continue
					}
					val2 := w.replaceContentText(val, replacer)
					if val2 != val {
						item.ExtraData[k] = val2
						updateCount++
						innerUpdate = true
					}
				}
				if innerUpdate {
					needUpdate = true
				}
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
