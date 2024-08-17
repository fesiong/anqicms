package provider

import (
	"errors"
	"fmt"
	"github.com/jinzhu/now"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func (w *Website) GetArchiveByIdFromCache(id uint) (archive *model.Archive) {
	err := w.Cache.Get(fmt.Sprintf("archive-%d", id), archive)
	if err != nil {
		return nil
	}

	return archive
}

func (w *Website) AddArchiveCache(archive *model.Archive) {
	_ = w.Cache.Set(fmt.Sprintf("archive-%d", archive.Id), archive, 300)
}

func (w *Website) DeleteArchiveCache(id uint) {
	w.Cache.Delete(fmt.Sprintf("archive-%d", id))
}

func (w *Website) GetArchiveById(id uint) (*model.Archive, error) {
	return w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`id` = ?", id)
	})
}

func (w *Website) GetArchiveByTitle(title string) (*model.Archive, error) {
	return w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`title` = ?", title)
	})
}

func (w *Website) GetArchiveByFixedLink(link string) (*model.Archive, error) {
	return w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`fixed_link` = ?", link)
	})
}

func (w *Website) GetArchiveByUrlToken(urlToken string) (*model.Archive, error) {
	if urlToken == "" {
		return nil, errors.New("empty token")
	}
	return w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`url_token` = ?", urlToken)
	})
}

func (w *Website) GetArchiveByOriginUrl(keyword string) (*model.Archive, error) {
	return w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`origin_url` = ?", keyword).Order("id desc")
	})
}

func (w *Website) GetArchiveByFunc(ops func(tx *gorm.DB) *gorm.DB) (*model.Archive, error) {
	var archive model.Archive
	err := ops(w.DB).Take(&archive).Error
	if err != nil {
		return nil, err
	}
	archive.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
	archive.Link = w.GetUrl("archive", &archive, 0)
	return &archive, nil
}

func (w *Website) GetArchiveDraftById(id uint) (*model.ArchiveDraft, error) {
	return w.GetArchiveDraftByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`id` = ?", id)
	})
}

func (w *Website) GetArchiveDraftByTitle(title string) (*model.ArchiveDraft, error) {
	return w.GetArchiveDraftByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`title` = ?", title)
	})
}

func (w *Website) GetArchiveDraftByFixedLink(link string) (*model.ArchiveDraft, error) {
	return w.GetArchiveDraftByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`fixed_link` = ?", link)
	})
}

func (w *Website) GetArchiveDraftByUrlToken(urlToken string) (*model.ArchiveDraft, error) {
	if urlToken == "" {
		return nil, errors.New("empty token")
	}
	return w.GetArchiveDraftByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`url_token` = ?", urlToken)
	})
}

func (w *Website) GetArchiveDraftByOriginUrl(keyword string) (*model.ArchiveDraft, error) {
	return w.GetArchiveDraftByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`origin_url` = ?", keyword).Order("id desc")
	})
}

func (w *Website) GetArchiveDraftByFunc(ops func(tx *gorm.DB) *gorm.DB) (*model.ArchiveDraft, error) {
	var archive model.ArchiveDraft
	err := ops(w.DB).Take(&archive).Error
	if err != nil {
		return nil, err
	}
	archive.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
	archive.Link = w.GetUrl("archive", &archive, 0)
	return &archive, nil
}

func (w *Website) GetArchiveDataById(id uint) (*model.ArchiveData, error) {
	var data model.ArchiveData
	err := w.DB.Where("`id` = ?", id).First(&data).Error
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func (w *Website) GetArchiveList(ops func(tx *gorm.DB) *gorm.DB, order string, currentPage, pageSize int, offsets ...int) ([]*model.Archive, int64, error) {
	var archives []*model.Archive

	offset := 0
	if currentPage > 0 {
		offset = (currentPage - 1) * pageSize
	}
	var draft = false
	if len(offsets) > 0 {
		offset = offsets[0]
		if len(offsets) > 1 {
			draft = offsets[1] == 1
		}
	}
	var total int64
	// 对于没有分页的list，则缓存
	var cacheKey = ""
	if currentPage == 0 && !draft {
		sql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
			if ops != nil {
				tx = ops(tx)
			}
			return tx.Order(order).Limit(pageSize).Offset(offset).Find(&[]*model.Archive{})
		})
		cacheKey = "archive-list-" + library.Md5(sql)[8:24]
		err := w.Cache.Get(cacheKey, &archives)
		if err == nil {
			return archives, int64(len(archives)), nil
		}
	}
	var builder *gorm.DB
	if draft {
		builder = w.DB.Table("`archive_drafts` as archives").Order(order)
	} else {
		builder = w.DB.Model(&model.Archive{}).Order(order)
	}

	if ops != nil {
		builder = ops(builder)
	}

	if currentPage > 0 {
		// 缓存count
		sqlCount := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
			if ops != nil {
				tx = ops(tx)
			}
			return tx.Order(order).Find(&[]*model.Archive{})
		})
		cacheKeyCount := "archive-list-count" + library.Md5(sqlCount)[8:24]
		err := w.Cache.Get(cacheKeyCount, &total)
		if err != nil {
			builder.Count(&total)
			_ = w.Cache.Set(cacheKeyCount, total, 300)
		}
		// 分页提速，先查出ID，再查询结果
		// 先查询ID
		var archiveIds []uint
		builder.Limit(pageSize).Offset(offset).Select("archives.id").Pluck("id", &archiveIds)
		if len(archiveIds) > 0 {
			if draft {
				w.DB.Table("`archive_drafts` as archives").Where("id IN (?)", archiveIds).Order(order).Scan(&archives)
			} else {
				w.DB.Model(&model.Archive{}).Where("id IN (?)", archiveIds).Order(order).Scan(&archives)
			}
		}
		for i := range archives {
			archives[i].GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
			archives[i].Link = w.GetUrl("archive", archives[i], 0)
		}
	} else {
		builder = builder.Limit(pageSize).Offset(offset)
		if err := builder.Find(&archives).Error; err != nil {
			return nil, 0, err
		}
		for i := range archives {
			archives[i].GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
			archives[i].Link = w.GetUrl("archive", archives[i], 0)
		}
		// 对于没有分页的list，则缓存
		_ = w.Cache.Set(cacheKey, archives, 300)
	}

	return archives, total, nil
}

func (w *Website) GetArchiveExtraFromCache(archiveId uint) (extra map[string]*model.CustomField) {
	err := w.Cache.Get(fmt.Sprintf("archive-extra-%d", archiveId), &extra)
	if err != nil {
		return nil
	}

	return extra
}

func (w *Website) AddArchiveExtraCache(archiveId uint, extra map[string]*model.CustomField) {
	_ = w.Cache.Set(fmt.Sprintf("archive-extra-%d", archiveId), extra, 60)
}

func (w *Website) DeleteArchiveExtraCache(archiveId uint) {
	w.Cache.Delete(fmt.Sprintf("archive-extra-%d", archiveId))
}

func (w *Website) GetArchiveExtra(moduleId, id uint, loadCache bool) map[string]*model.CustomField {
	if loadCache {
		cached := w.GetArchiveExtraFromCache(id)
		if cached != nil {
			return cached
		}
	}
	//读取extra
	result := map[string]interface{}{}
	extraFields := map[string]*model.CustomField{}
	module := w.GetModuleFromCache(moduleId)
	if module != nil {
		var fields []string
		for _, v := range module.Fields {
			fields = append(fields, "`"+v.FieldName+"`")
		}
		//从数据库中取出来
		if len(fields) > 0 {
			w.DB.Table(module.TableName).Where("`id` = ?", id).Select(strings.Join(fields, ",")).Scan(&result)
			//extra的CheckBox的值
			for _, v := range module.Fields {
				if v.Type == config.CustomFieldTypeImage || v.Type == config.CustomFieldTypeFile {
					value, ok := result[v.FieldName].(string)
					if ok && value != "" && !strings.HasPrefix(value, "http") && !strings.HasPrefix(value, "//") {
						result[v.FieldName] = w.PluginStorage.StorageUrl + value
					}
				}
				// render
				if v.Type == config.CustomFieldTypeEditor && w.Content.Editor == "markdown" {
					value, ok := result[v.FieldName].(string)
					if ok {
						result[v.FieldName] = library.MarkdownToHTML(value)
					}
				}
				extraFields[v.FieldName] = &model.CustomField{
					Name:        v.Name,
					Value:       result[v.FieldName],
					Default:     v.Content,
					FollowLevel: v.FollowLevel,
					Type:        v.Type,
					FieldName:   v.FieldName,
				}
			}
		}
		if loadCache {
			w.AddArchiveExtraCache(id, extraFields)
		}
	}

	return extraFields
}

func (w *Website) SaveArchive(req *request.Archive) (*model.Archive, error) {
	if len(req.CategoryIds) > 0 {
		for i := 0; i < len(req.CategoryIds); i++ {
			// 防止 0
			if req.CategoryIds[i] == 0 {
				req.CategoryIds = append(req.CategoryIds[:i], req.CategoryIds[i+1:]...)
			}
		}
	}
	if len(req.CategoryIds) == 0 && req.CategoryId > 0 {
		req.CategoryIds = append(req.CategoryIds, req.CategoryId)
	}
	if len(req.CategoryIds) == 0 {
		return nil, errors.New(w.Tr("PleaseSelectAColumn"))
	}
	for _, catId := range req.CategoryIds {
		category := w.GetCategoryFromCache(catId)
		if category == nil || category.Type != config.CategoryTypeArchive {
			return nil, errors.New(w.Tr("PleaseSelectAColumn"))
		}
	}
	req.CategoryId = req.CategoryIds[0]
	category, err := w.GetCategoryById(req.CategoryId)
	if err != nil {
		return nil, errors.New(w.Tr("PleaseSelectAColumn"))
	}
	module := w.GetModuleFromCache(category.ModuleId)
	if module == nil {
		return nil, errors.New(w.Tr("UndefinedModel"))
	}
	if len(req.Title) == 0 {
		return nil, errors.New(w.Tr("PleaseFillInTheArticleTitle"))
	}
	var draft *model.ArchiveDraft
	newPost := false
	isReleased := false
	if req.Id > 0 {
		// 先读草稿
		draft, err = w.GetArchiveDraftById(req.Id)
		if err != nil {
			archive, err := w.GetArchiveById(req.Id)
			if err != nil {
				return nil, err
			}
			draft = &model.ArchiveDraft{
				Archive: *archive,
				Status:  config.ContentStatusOK,
			}
			isReleased = true
		}
	} else {
		newPost = true
		draft = &model.ArchiveDraft{}
	}
	// createdTime
	if req.CreatedTime > 0 {
		draft.CreatedTime = req.CreatedTime
	}
	if !req.QuickSave {
		draft.Status = config.ContentStatusOK
	}
	if draft.CreatedTime > time.Now().Unix() {
		// 未来时间，设置为待发布
		draft.Status = config.ContentStatusPlan
	}
	if req.Draft {
		draft.Status = config.ContentStatusDraft
	}
	// 判断重复
	req.UrlToken = library.ParseUrlToken(req.UrlToken)
	if req.UrlToken == "" {
		req.UrlToken = library.GetPinyin(req.Title, w.Content.UrlTokenType == config.UrlTokenTypeSort)
	}
	if req.UrlToken == "" {
		req.UrlToken = time.Now().Format("a-20060102150405")
	}
	draft.UrlToken = w.VerifyArchiveUrlToken(req.UrlToken, draft.Id)
	if utf8.RuneCountInString(req.Title) > 190 {
		req.Title = string([]rune(req.Title)[:190])
		if strings.Count(req.Title, " ") > 1 {
			req.Title = req.Title[:strings.LastIndexAny(req.Title, " ")]
		}
	}
	//提取描述
	if req.Description == "" {
		tmpContent := req.Content
		if w.Content.Editor == "markdown" {
			tmpContent = library.MarkdownToHTML(tmpContent)
		}
		req.Description = library.ParseDescription(strings.ReplaceAll(CleanTagsAndSpaces(tmpContent), "\n", " "))
	}
	// 限制数量
	descRunes := []rune(req.Description)
	if len(descRunes) > 1000 {
		req.Description = string(descRunes[:1000])
	}
	if len(req.Flag) > 0 {
		req.Flags = strings.Split(req.Flag, ",")
	}

	if req.QuickSave {
		// quick save 只支持6个字段
		draft.ModuleId = category.ModuleId
		draft.Title = req.Title
		draft.Keywords = req.Keywords
		draft.Description = req.Description
		draft.CategoryId = req.CategoryId
		// 保存主表
		if isReleased {
			// 已发布的，quickSave 就保存到正式表
			if err = w.DB.Save(&draft.Archive).Error; err != nil {
				return nil, err
			}
		} else {
			// 否则保存到草稿表
			if err = w.DB.Save(&draft).Error; err != nil {
				return nil, err
			}
		}
		// 保存Flags
		_ = w.SaveArchiveFlags(draft.Id, req.Flags)
		// 保存分类ID
		_ = w.SaveArchiveCategories(draft.Id, req.CategoryIds)
		// tags
		_ = w.SaveTagData(draft.Id, req.Tags)

		// 清除缓存
		w.DeleteArchiveCache(draft.Id)
		w.DeleteArchiveExtraCache(draft.Id)
		if isReleased {
			err = w.SuccessReleaseArchive(&draft.Archive, newPost)
		}
		// 返回结果
		return &draft.Archive, nil
	}
	// 正常的保存行为
	draft.ModuleId = category.ModuleId
	draft.Title = req.Title
	draft.SeoTitle = req.SeoTitle
	draft.Keywords = req.Keywords
	draft.Description = req.Description
	draft.CategoryId = req.CategoryId
	draft.Images = req.Images
	draft.Template = req.Template
	draft.CanonicalUrl = req.CanonicalUrl
	oldFixedLink := draft.FixedLink
	draft.FixedLink = req.FixedLink
	draft.Price = req.Price
	draft.Stock = req.Stock
	draft.ReadLevel = req.ReadLevel
	draft.Password = req.Password
	draft.Sort = req.Sort
	if req.UserId > 0 {
		draft.UserId = req.UserId
	}

	if req.KeywordId > 0 {
		draft.KeywordId = req.KeywordId
	}
	if req.OriginUrl != "" {
		if utf8.RuneCountInString(req.OriginUrl) > 190 {
			req.OriginUrl = string([]rune(req.OriginUrl)[:190])
		}
		draft.OriginUrl = req.OriginUrl
	}
	if req.OriginTitle != "" {
		if utf8.RuneCountInString(req.OriginTitle) > 190 {
			req.OriginTitle = string([]rune(req.OriginTitle)[:190])
		}
		draft.OriginTitle = req.OriginTitle
	}

	//extra
	extraFields := map[string]interface{}{}
	if len(module.Fields) > 0 {
		for _, v := range module.Fields {
			//先检查是否有必填而没有填写的
			if v.Required && req.Extra[v.FieldName] == nil {
				return nil, errors.New(w.Tr("ItIsRequired", v.Name))
			}
			if req.Extra[v.FieldName] != nil {
				extraValue, ok := req.Extra[v.FieldName].(map[string]interface{})
				if ok {
					if v.Required && extraValue["value"] == nil && extraValue["value"] == "" {
						return nil, errors.New(w.Tr("ItIsRequired", v.Name))
					}
					if v.Type == config.CustomFieldTypeCheckbox {
						//只有这个类型的数据是数组,数组转成,分隔字符串
						if val, ok := extraValue["value"].([]interface{}); ok {
							var val2 []string
							for _, v2 := range val {
								val2 = append(val2, v2.(string))
							}
							extraFields[v.FieldName] = strings.Join(val2, ",")
						}
					} else if v.Type == config.CustomFieldTypeNumber {
						//只有这个类型的数据是数字，转成数字
						extraFields[v.FieldName], _ = strconv.Atoi(fmt.Sprintf("%v", extraValue["value"]))
					} else {
						value, ok := extraValue["value"].(string)
						if ok {
							extraFields[v.FieldName] = strings.TrimPrefix(value, w.PluginStorage.StorageUrl)
						} else {
							extraFields[v.FieldName] = extraValue["value"]
						}
					}
				}
			} else {
				if v.Type == config.CustomFieldTypeNumber {
					//只有这个类型的数据是数字，转成数字
					extraFields[v.FieldName] = 0
				} else {
					extraFields[v.FieldName] = ""
				}
			}
		}
	}

	if len(req.Flags) == 0 {
		imgRe, _ := regexp.Compile(`(?i)<img.*?src=["'](.+?)["'].*?>`)
		hasImage := imgRe.MatchString(req.Content)
		if !hasImage {
			// 匹配Markdown ![新的图片](http://xxx/xxx.webp)
			imgRe, _ = regexp.Compile(`!\[([^]]*)\]\(([^)]+)\)`)
			hasImage = imgRe.MatchString(req.Content)
		}
		if hasImage {
			req.Flags = append(req.Flags, "p")
		}
	}
	// 将单个&nbsp;替换为空格
	req.Content = library.ReplaceSingleSpace(req.Content)
	// todo 应该只替换 src,href 中的 baseUrl
	req.Content = strings.ReplaceAll(req.Content, w.System.BaseUrl, "")
	baseHost := ""
	urls, err := url.Parse(w.System.BaseUrl)
	if err == nil {
		baseHost = urls.Host
	}
	autoAddImage := false
	//提取缩略图
	if len(draft.Images) == 0 {
		re, _ := regexp.Compile(`(?i)<img.*?src=["'](.+?)["'].*?>`)
		match := re.FindStringSubmatch(req.Content)
		if len(match) > 1 {
			draft.Images = append(draft.Images, match[1])
			autoAddImage = true
		} else {
			// 匹配Markdown ![新的图片](http://xxx/xxx.webp)
			re, _ = regexp.Compile(`!\[([^]]*)\]\(([^)]+)\)`)
			match = re.FindStringSubmatch(req.Content)
			if len(match) > 2 {
				draft.Images = append(draft.Images, match[2])
				autoAddImage = true
			}
		}
	}
	// 过滤外链
	if w.Content.FilterOutlink == 1 {
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
					return match[2]
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
					return match[1]
				}
			}
			return s
		})
	}

	for i, v := range draft.Images {
		draft.Images[i] = strings.TrimPrefix(v, w.PluginStorage.StorageUrl)
	}
	// 如果是 已经发布了的，则保存到正式表
	if isReleased {
		err = w.DB.Debug().Save(&draft.Archive).Error
		if err != nil {
			return nil, err
		}
		// 如果status 不是 1，则删除正式表内容，保存到草稿
		if draft.Status != config.ContentStatusOK {
			isReleased = false
			// 从 archives 表删除
			if err = w.DB.Delete(&draft.Archive).Error; err != nil {
				return nil, err
			}
			// 数据移到 archiveDraft 表
			w.DB.Save(draft)
		}
	} else {
		// 保存到草稿
		err = w.DB.Debug().Save(draft).Error
		if err != nil {
			return nil, err
		}
		// 如果是正式发布，则删除草稿，并保存到正式表
		if draft.Status == config.ContentStatusOK {
			isReleased = true
			// 保存到正式表
			w.DB.Save(&draft.Archive)
			// 并删除草稿
			w.DB.Delete(draft)
		}
	}

	// 保存内容表
	archiveData := model.ArchiveData{
		Content: req.Content,
	}
	archiveData.Id = draft.Id
	err = w.DB.Save(&archiveData).Error
	if err != nil {
		return nil, err
	}
	// 保存Flags
	_ = w.SaveArchiveFlags(draft.Id, req.Flags)
	// 保存分类ID
	_ = w.SaveArchiveCategories(draft.Id, req.CategoryIds)
	// 保存 Relations
	_ = w.SaveArchiveRelations(draft.Id, req.RelationIds)
	//检查有多少个material
	var materialIds []uint
	re, _ := regexp.Compile(`(?i)<div.*?data-material="(\d+)".*?>`)
	matches := re.FindAllStringSubmatch(req.Content, -1)
	if len(matches) > 0 {
		for _, match := range matches {
			//记录material
			materialId, _ := strconv.Atoi(match[1])
			if materialId > 0 {
				materialIds = append(materialIds, uint(materialId))
			}
		}
	}
	go w.LogMaterialData(materialIds, "archive", draft.Id)
	// 自动提取远程图片改成保存后处理
	if w.Content.RemoteDownload == 1 {
		hasChangeImg := false
		re, _ = regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
		archiveData.Content = re.ReplaceAllStringFunc(archiveData.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 2 {
				return s
			}
			imgUrl, err2 := url.Parse(match[1])
			if err2 == nil {
				if imgUrl.Host != "" && imgUrl.Host != baseHost && !strings.HasPrefix(match[1], w.PluginStorage.StorageUrl) {
					//外链
					attachment, err2 := w.DownloadRemoteImage(match[1], "")
					if err2 == nil {
						// 下载完成
						hasChangeImg = true
						s = strings.Replace(s, match[1], attachment.Logo, 1)
					}
				}
			}
			return s
		})
		// 匹配Markdown ![新的图片](http://xxx/xxx.webp)
		re, _ = regexp.Compile(`!\[([^]]*)\]\(([^)]+)\)`)
		archiveData.Content = re.ReplaceAllStringFunc(archiveData.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 3 {
				return s
			}
			imgUrl, err2 := url.Parse(match[2])
			if err2 == nil {
				if imgUrl.Host != "" && imgUrl.Host != baseHost && !strings.HasPrefix(match[2], w.PluginStorage.StorageUrl) {
					//外链
					attachment, err2 := w.DownloadRemoteImage(match[2], "")
					if err2 == nil {
						// 下载完成
						hasChangeImg = true
						s = strings.Replace(s, match[2], attachment.Logo, 1)
					}
				}
			}
			return s
		})
		if hasChangeImg {
			w.DB.Model(&archiveData).UpdateColumn("content", archiveData.Content)
			// 更新data
			if autoAddImage {
				//提取缩略图
				draft.Images = draft.Images[:0]
				re, _ = regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
				match := re.FindStringSubmatch(archiveData.Content)
				if len(match) > 1 {
					draft.Images = append(draft.Images, match[1])
				} else {
					// 匹配Markdown ![新的图片](http://xxx/xxx.webp)
					re, _ = regexp.Compile(`!\[([^]]*)\]\(([^)]+)\)`)
					match = re.FindStringSubmatch(archiveData.Content)
					if len(match) > 2 {
						draft.Images = append(draft.Images, match[2])
					}
				}
				if isReleased {
					w.DB.Model(&draft.Archive).UpdateColumn("images", draft.Images)
				} else {
					w.DB.Model(draft).UpdateColumn("images", draft.Images)
				}
			}
		}
	}
	//extra
	if len(extraFields) > 0 {
		//入库
		// 先检查是否存在
		var existsId uint
		w.DB.Table(module.TableName).Where("`id` = ?", draft.Id).Pluck("id", &existsId)
		if existsId > 0 {
			// 已存在
			w.DB.Table(module.TableName).Where("`id` = ?", draft.Id).Updates(extraFields)
		} else {
			// 新建
			extraFields["id"] = draft.Id
			w.DB.Table(module.TableName).Where("`id` = ?", draft.Id).Create(extraFields)
		}
	}
	// 如果没有图片
	if len(draft.Images) == 0 && w.PluginTitleImage.Open {
		draft.ArchiveData = &archiveData
		// 自动生成一个
		go func() {
			logo, content, err := w.NewTitleImage().DrawTitles(draft.Title, archiveData.Content)
			if err == nil {
				if content != archiveData.Content {
					w.DB.Model(&archiveData).UpdateColumn("content", content)
				}
				if len(logo) > 0 {
					draft.Images = append(draft.Images, strings.TrimPrefix(logo, w.PluginStorage.StorageUrl))
				}
				if isReleased {
					w.DB.Model(&draft.Archive).UpdateColumn("images", draft.Images)
				} else {
					w.DB.Model(draft).UpdateColumn("images", draft.Images)
				}
			}
		}()
	}

	// tags
	_ = w.SaveTagData(draft.Id, req.Tags)

	// 缓存清理
	if oldFixedLink != "" || draft.FixedLink != "" {
		w.DeleteCacheFixedLinks()
	}
	// 清除缓存
	w.DeleteArchiveCache(draft.Id)
	w.DeleteArchiveExtraCache(draft.Id)

	if isReleased {
		// 尝试添加全文索引
		w.AddFulltextIndex(&TinyArchive{
			Id:          uint64(draft.Id),
			ModuleId:    draft.ModuleId,
			Title:       draft.Title,
			Keywords:    draft.Keywords,
			Description: draft.Description,
			Content:     archiveData.Content,
		})
		w.FlushIndex()

		err = w.SuccessReleaseArchive(&draft.Archive, newPost)
	}

	return &draft.Archive, nil
}

// SuccessReleaseArchive
// 文章发布成功后的一些处理
func (w *Website) SuccessReleaseArchive(archive *model.Archive, newPost bool) error {
	archive.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
	archive.Link = w.GetUrl("archive", archive, 0)
	//添加锚文本
	if w.PluginAnchor.ReplaceWay == 1 {
		go w.ReplaceContent(nil, "archive", archive.Id, archive.Link)
	}
	//提取锚文本
	if w.PluginAnchor.KeywordWay == 1 {

		go w.AutoInsertAnchor(archive.Id, archive.Keywords, archive.Link)
	}

	w.DeleteCacheIndex()
	// 删除列表缓存
	w.Cache.CleanAll("archive-list")

	//新发布的文章，执行推送
	if newPost {
		go func() {
			w.PushArchive(archive.Link)
			if w.PluginSitemap.AutoBuild == 1 {
				_ = w.AddonSitemap("archive", archive.Link, time.Unix(archive.UpdatedTime, 0).Format("2006-01-02"))
			}
		}()
	}
	// 更新缓存
	go func() {
		// 生成文章页，生成栏目页，生成首页，生成tag
		// 上传到静态服务器
		cachePath := w.CachePath + "pc"
		// 生成文章页
		link := w.GetUrl("archive", archive, 0)
		link = strings.TrimPrefix(link, w.System.BaseUrl)
		_ = w.GetAndCacheHtmlData(link, false)
		if w.System.TemplateType != config.TemplateTypeAuto {
			_ = w.GetAndCacheHtmlData(link, true)
		}
		archivePath := cachePath + transToLocalPath(link, "")
		_ = w.SyncHtmlCacheToStorage(archivePath, link)
		// 生成栏目页，只生成第一页
		category := w.GetCategoryFromCache(archive.CategoryId)
		if category != nil {
			link = w.GetUrl("category", category, 0)
			link = strings.TrimPrefix(link, w.System.BaseUrl)
			_ = w.GetAndCacheHtmlData(link, false)
			if w.System.TemplateType != config.TemplateTypeAuto {
				_ = w.GetAndCacheHtmlData(link, true)
			}
			categoryPath := cachePath + transToLocalPath(link, "")
			_ = w.SyncHtmlCacheToStorage(categoryPath, link)
		}
		// 生成Tag，只生成第一页
		tags := w.GetTagsByItemId(archive.Id)
		if len(tags) > 0 {
			link = w.GetUrl("tagIndex", nil, 0)
			link = strings.TrimPrefix(link, w.System.BaseUrl)
			// 先生成首页
			_ = w.GetAndCacheHtmlData(link, false)
			if w.System.TemplateType != config.TemplateTypeAuto {
				_ = w.GetAndCacheHtmlData(link, true)
			}
			tagPath := cachePath + transToLocalPath(link, "")
			_ = w.SyncHtmlCacheToStorage(tagPath, link)
			for _, tag := range tags {
				link = w.GetUrl("tag", tag, 0)
				link = strings.TrimPrefix(link, w.System.BaseUrl)
				_ = w.GetAndCacheHtmlData(link, false)
				if w.System.TemplateType != config.TemplateTypeAuto {
					_ = w.GetAndCacheHtmlData(link, true)
				}
				tagPath = cachePath + transToLocalPath(link, "")
				_ = w.SyncHtmlCacheToStorage(tagPath, link)
			}
		}
	}()

	return nil
}

func (w *Website) UpdateArchiveUrlToken(archive *model.Archive) error {
	if archive.UrlToken == "" {
		newToken := library.GetPinyin(archive.Title, w.Content.UrlTokenType == config.UrlTokenTypeSort)
		archive.UrlToken = w.VerifyArchiveUrlToken(newToken, archive.Id)

		w.DB.Model(&model.Archive{}).Where("`id` = ?", archive.Id).UpdateColumn("url_token", archive.UrlToken)
	}

	return nil
}

func (w *Website) RecoverArchive(draft *model.ArchiveDraft) error {
	w.PublishPlanArchive(draft)

	go func() {
		var doc TinyArchive
		w.DB.Table("`archives` as archives").Joins("left join `archive_data` as d on archives.id=d.id").Select("archives.id,archives.title,archives.keywords,archives.description,archives.module_id,d.content").Where("archives.`id` > ?", draft.Id).Take(&doc)
		// 尝试添加全文索引
		w.AddFulltextIndex(&doc)
	}()

	return nil
}

func (w *Website) DeleteArchive(archive *model.Archive) error {
	// 数据移到 archiveDraft 表
	draft := &model.ArchiveDraft{
		Archive: *archive,
	}
	draft.Status = config.ContentStatusDelete
	err := w.DB.Model(&model.ArchiveDraft{}).Where("`id` = ?", draft.Id).Save(draft).Error
	if err != nil {
		return err
	}
	// 从 archives 表删除
	if err = w.DB.Unscoped().Delete(archive).Error; err != nil {
		return err
	}

	if archive.FixedLink != "" {
		w.DeleteCacheFixedLinks()
	}
	w.DeleteCacheIndex()
	// 删除列表缓存
	w.Cache.CleanAll("archive-list")
	w.RemoveHtmlCache(w.GetUrl("archive", archive, 0))
	w.RemoveFulltextIndex(uint64(archive.Id))
	// 每次删除文档，都清理一次Sitemap
	if w.PluginSitemap.AutoBuild == 1 {
		w.DeleteSitemap(w.PluginSitemap.Type)
	}

	return nil
}

func (w *Website) DeleteArchiveDraft(draft *model.ArchiveDraft) error {
	w.DB.Unscoped().Delete(draft)
	// 删除 文档内容
	w.DB.Unscoped().Where("`id` = ?", draft.Id).Delete(&model.ArchiveData{})
	// 删除 文档分类
	w.DB.Unscoped().Where("`archive_id` = ?", draft.Id).Delete(&model.ArchiveCategory{})
	// 删除 文档Flag
	w.DB.Unscoped().Where("`archive_id` = ?", draft.Id).Delete(&model.ArchiveFlag{})
	// 删除 文档TagData
	w.DB.Unscoped().Where("`item_id` = ?", draft.Id).Delete(&model.TagData{})

	return nil
}

func (w *Website) UpdateArchiveRecommend(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(w.Tr("NoDocumentToOperate"))
	}
	for _, id := range req.Ids {
		_ = w.SaveArchiveFlags(id, strings.Split(req.Flag, ","))
	}
	// 删除列表缓存
	w.Cache.CleanAll("archive-list")

	return nil
}

func (w *Website) UpdateArchiveStatus(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(w.Tr("NoDocumentToOperate"))
	}
	if req.Status == config.ContentStatusOK {
		// 改成正式发布
		var drafts []*model.ArchiveDraft
		w.DB.Model(&model.ArchiveDraft{}).Where("`id` IN (?)", req.Ids).Find(&drafts)
		for _, draft := range drafts {
			draft.CreatedTime = time.Now().Unix()
			draft.UpdatedTime = time.Now().Unix()
			w.PublishPlanArchive(draft)
		}
	} else {
		// 从正式表移到草稿表
		hasFixedLink := false
		var archives []*model.Archive
		w.DB.Model(&model.Archive{}).Where("`id` IN (?)", req.Ids).Find(&archives)
		for _, archive := range archives {
			// 转为草稿
			// 数据移到 archiveDraft 表
			draft := &model.ArchiveDraft{
				Archive: *archive,
			}
			draft.Status = config.ContentStatusDraft
			err := w.DB.Model(&model.ArchiveDraft{}).Where("`id` = ?", draft.Id).Save(draft).Error
			if err != nil {
				return err
			}
			// 从 archives 表删除
			if err := w.DB.Unscoped().Delete(archive).Error; err != nil {
				return err
			}
			if archive.FixedLink != "" {
				hasFixedLink = true
			}
			w.RemoveHtmlCache(w.GetUrl("archive", archive, 0))
			w.RemoveFulltextIndex(uint64(archive.Id))
		}
		if hasFixedLink {
			w.DeleteCacheFixedLinks()
		}
		w.DeleteCacheIndex()
		// 删除列表缓存
		w.Cache.CleanAll("archive-list")
	}

	return nil
}

func (w *Website) UpdateArchiveTime(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(w.Tr("NoDocumentToOperate"))
	}
	var err error
	if req.Time == 4 {
		// updated_time 所有文档
		err = w.DB.Model(&model.Archive{}).Where("`id` > 0").UpdateColumn("updated_time", time.Now().Unix()).Error
	} else if req.Time == 3 {
		// created_time 所有文档
		err = w.DB.Model(&model.Archive{}).Where("`id` > 0").UpdateColumn("created_time", time.Now().Unix()).Error
	} else if req.Time == 2 {
		// updated_time
		err = w.DB.Model(&model.Archive{}).Where("`id` IN (?)", req.Ids).UpdateColumn("updated_time", time.Now().Unix()).Error
		err = w.DB.Model(&model.ArchiveDraft{}).Where("`id` IN (?)", req.Ids).UpdateColumn("updated_time", time.Now().Unix()).Error
	} else {
		err = w.DB.Model(&model.Archive{}).Where("`id` IN (?)", req.Ids).UpdateColumn("created_time", time.Now().Unix()).Error
		err = w.DB.Model(&model.ArchiveDraft{}).Where("`id` IN (?)", req.Ids).UpdateColumn("created_time", time.Now().Unix()).Error
	}
	// 删除列表缓存
	w.Cache.CleanAll("archive-list")
	return err
}

func (w *Website) UpdateArchiveReleasePlan(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(w.Tr("NoDocumentToOperate"))
	}
	num := 0
	if req.EndHour <= req.StartHour {
		// 大一小时
		req.EndHour = req.StartHour + 1
	}
	if req.DailyLimit < 1 {
		req.DailyLimit = len(req.Ids)
	}
	// 间隔用秒
	gap := (req.EndHour - req.StartHour) * 3600 / req.DailyLimit
	// 从第0天开始
	dayNum := 0
	h := time.Now().Hour()
	if req.EndHour < h {
		// 当天不发布
		dayNum++
	}
	startTime := now.BeginningOfDay().AddDate(0, 0, dayNum).Add(time.Duration(req.StartHour) * time.Hour)
	if startTime.Before(time.Now()) {
		startTime = time.Now()
	}
	for _, id := range req.Ids {
		draft, err := w.GetArchiveDraftById(id)
		if err != nil {
			// 文档不存在，跳过
			continue
		}
		num++
		w.DB.Model(&model.ArchiveDraft{}).Where("`id` = ?", draft.Id).UpdateColumns(map[string]interface{}{
			"created_time": startTime.Unix(),
			"updated_time": startTime.Unix(),
			"status":       config.ContentStatusPlan,
		})
		startTime = startTime.Add(time.Duration(gap) * time.Second)
		if startTime.Hour() >= req.EndHour {
			// 达到数量加一天
			dayNum++
			// 重置时间
			startTime = now.BeginningOfDay().AddDate(0, 0, dayNum).Add(time.Duration(req.StartHour) * time.Hour)
		}

	}

	return nil
}

func (w *Website) UpdateArchiveCategory(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(w.Tr("NoDocumentToOperate"))
	}
	// 保存分类ID
	if len(req.CategoryIds) == 0 && req.CategoryId > 0 {
		req.CategoryIds = append(req.CategoryIds, req.CategoryId)
	}
	var defaultCategory *model.Category
	for _, catId := range req.CategoryIds {
		category, err := w.GetCategoryById(catId)
		if err != nil {
			return errors.New(w.Tr("CategoryDoesNotExist"))
		}
		if defaultCategory == nil {
			defaultCategory = category
		}
	}
	if len(req.CategoryIds) == 0 || defaultCategory == nil {
		return errors.New(w.Tr("PleaseSelectACategory"))
	}
	for _, arcId := range req.Ids {
		_ = w.SaveArchiveCategories(arcId, req.CategoryIds)
	}
	// 更新主分类ID
	w.DB.Model(&model.Archive{}).Where("`id` IN(?)", req.Ids).UpdateColumns(map[string]interface{}{
		"category_id": defaultCategory.Id,
		"module_id":   defaultCategory.ModuleId,
	})
	// 更新草稿表分类ID
	w.DB.Model(&model.ArchiveDraft{}).Where("`id` IN(?)", req.Ids).UpdateColumns(map[string]interface{}{
		"category_id": defaultCategory.Id,
		"module_id":   defaultCategory.ModuleId,
	})
	// 删除列表缓存
	w.Cache.CleanAll("archive-list")
	// end

	return nil
}

// DeleteCacheFixedLinks 固定链接
func (w *Website) DeleteCacheFixedLinks() {
	w.Cache.Delete("fixedLinks")
}

func (w *Website) GetCacheFixedLinks() map[string]uint {
	if w.DB == nil {
		return nil
	}
	var fixedLinks = map[string]uint{}

	err := w.Cache.Get("fixedLinks", &fixedLinks)
	if err == nil {
		return fixedLinks
	}

	var archives []model.Archive
	w.DB.Model(model.Archive{}).Where("`fixed_link` != ''").Select("fixed_link", "id").Scan(&archives)
	for i := range archives {
		fixedLinks[archives[i].FixedLink] = archives[i].Id
	}

	_ = w.Cache.Set("fixedLinks", fixedLinks, 0)

	return fixedLinks
}

func (w *Website) GetFixedLinkFromCache(fixedLink string) uint {
	links := w.GetCacheFixedLinks()

	archiveId, ok := links[fixedLink]
	if ok {
		return archiveId
	}

	return 0
}

// PublishPlanArchives 发布计划文章，单次最多处理100篇
func (w *Website) PublishPlanArchives() {
	timeStamp := time.Now().Unix()

	var drafts []*model.ArchiveDraft
	w.DB.Model(&model.ArchiveDraft{}).Where("`status` = ? and created_time < ?", config.ContentStatusPlan, timeStamp).Limit(100).Find(&drafts)
	if len(drafts) > 0 {
		for _, draft := range drafts {
			w.PublishPlanArchive(draft)
		}
	}
}

func (w *Website) PublishPlanArchive(archiveDraft *model.ArchiveDraft) {
	// 发布的步骤：将草稿转移到正式表，删除草稿
	w.DB.Save(&archiveDraft.Archive)

	w.DB.Delete(archiveDraft)

	_ = w.SuccessReleaseArchive(&archiveDraft.Archive, true)
}

// CleanArchives 计划任务删除存档，30天前被删除的
func (w *Website) CleanArchives() {
	if w.DB == nil {
		return
	}
	var drafts []model.ArchiveDraft
	w.DB.Model(&model.ArchiveDraft{}).Unscoped().Where("`status` = ? AND `updated_time` < ?", config.ContentStatusDelete, time.Now().AddDate(0, 0, -30)).Find(&drafts)
	if len(drafts) > 0 {
		modules := w.GetCacheModules()
		var mapModules = map[uint]model.Module{}
		for _, v := range modules {
			mapModules[v.Id] = v
		}
		for _, draft := range drafts {
			w.DB.Unscoped().Where("id = ?", draft.Id).Delete(model.ArchiveData{})
			if module, ok := mapModules[draft.ModuleId]; ok {
				w.DB.Unscoped().Where("id = ?", draft.Id).Delete(module.TableName)
			}
			w.DB.Unscoped().Where("id = ?", draft.Id).Delete(model.ArchiveDraft{})
		}
	}
}

func (w *Website) VerifyArchiveUrlToken(urlToken string, id uint) string {
	index := 0
	// 防止超出长度
	if len(urlToken) > 150 {
		urlToken = urlToken[:150]
	}
	urlToken = strings.ToLower(urlToken)
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
		tmpArc, err := w.GetArchiveByUrlToken(tmpToken)
		if err == nil && tmpArc.Id != id {
			index++
			continue
		}
		// 判断archiveDraft
		tmpDraft, err := w.GetArchiveDraftByUrlToken(tmpToken)
		if err == nil && tmpDraft.Id != id {
			index++
			continue
		}
		urlToken = tmpToken
		break
	}

	return urlToken
}

func (w *Website) CheckArchiveHasOrder(userId uint, archive *model.Archive, userGroup *model.UserGroup) *model.Archive {
	if archive.Price == 0 && archive.ReadLevel == 0 {
		archive.HasOrdered = true
	}
	if userId > 0 {
		if archive.UserId == userId {
			archive.HasOrdered = true
		} else if archive.Price > 0 {
			var exist int64
			w.DB.Debug().Table("`orders` as o").Joins("INNER JOIN `order_details` as d ON o.order_id = d.order_id AND d.`goods_id` = ?", archive.Id).Where("o.user_id = ? AND o.`status` IN(?)", userId, []int{
				config.OrderStatusPaid,
				config.OrderStatusDelivering,
				config.OrderStatusCompleted}).Count(&exist)
			if exist > 0 {
				archive.HasOrdered = true
			} else {
				archive.HasOrdered = false
			}
		}
		if archive.ReadLevel > 0 && !archive.HasOrdered {
			if userGroup != nil && userGroup.Level >= archive.ReadLevel {
				archive.HasOrdered = true
			}
		}
	}

	return archive
}

func (w *Website) UpgradeMultiCategory() {
	type tinyArchive struct {
		Id         uint `json:"id"`
		CategoryId uint `json:"category_id"`
	}
	var lastId uint = 0
	for {
		var archives []*tinyArchive
		w.DB.Model(&model.Archive{}).Where("`id` > ?", lastId).Order("id asc").Limit(1000).Scan(&archives)
		if len(archives) == 0 {
			break
		}
		lastId = archives[len(archives)-1].Id
		for _, arc := range archives {
			arcCategory := model.ArchiveCategory{
				CategoryId: arc.CategoryId,
				ArchiveId:  arc.Id,
			}
			w.DB.Model(&model.ArchiveCategory{}).Where("`archive_id` = ? and `category_id` = ?", arc.Id, arc.CategoryId).FirstOrCreate(&arcCategory)
		}
	}
}

func (w *Website) GetArchiveFlags(archiveId uint) string {
	var flags []string
	w.DB.Model(&model.ArchiveFlag{}).Where("`archive_id` = ?", archiveId).Pluck("flag", &flags)

	return strings.Join(flags, ",")
}

func (w *Website) SaveArchiveFlags(archiveId uint, flags []string) error {
	if len(flags) == 0 {
		w.DB.Where("`archive_id` = ?", archiveId).Delete(&model.ArchiveFlag{})
		return nil
	}
	for _, flag := range flags {
		arcFlag := model.ArchiveFlag{
			Flag:      flag,
			ArchiveId: archiveId,
		}
		w.DB.Model(&model.ArchiveFlag{}).Where("`archive_id` = ? and `flag` = ?", arcFlag.ArchiveId, arcFlag.Flag).FirstOrCreate(&arcFlag)
	}
	// 删除额外的
	w.DB.Unscoped().Where("`archive_id` = ? and `flag` NOT IN (?)", archiveId, flags).Delete(&model.ArchiveFlag{})

	return nil
}

func (w *Website) SaveArchiveCategories(archiveId uint, categoryIds []uint) error {
	if len(categoryIds) == 0 {
		w.DB.Where("`archive_id` = ?", archiveId).Delete(&model.ArchiveCategory{})
		return nil
	}
	for _, catId := range categoryIds {
		arcCategory := model.ArchiveCategory{
			CategoryId: catId,
			ArchiveId:  archiveId,
		}
		w.DB.Model(&model.ArchiveCategory{}).Where("`archive_id` = ? and `category_id` = ?", archiveId, catId).FirstOrCreate(&arcCategory)
	}
	// 删除额外的
	w.DB.Unscoped().Where("`archive_id` = ? and `category_id` NOT IN (?)", archiveId, categoryIds).Delete(&model.ArchiveCategory{})

	return nil
}

// GetArchiveRelations 仅返回正式的文档
func (w *Website) GetArchiveRelations(archiveId uint) []*model.Archive {
	var relations []*model.Archive
	var relationIds []uint
	w.DB.Model(&model.ArchiveRelation{}).Where("`archive_id` = ?", archiveId).Pluck("relation_id", &relationIds)
	if len(relationIds) > 0 {
		w.DB.Model(&model.Archive{}).Where("`id` IN (?)", relationIds).Find(&relations)
		for i := range relations {
			relations[i].GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
			relations[i].Link = w.GetUrl("archive", relations[i], 0)
		}
		return relations
	}

	return nil
}

func (w *Website) SaveArchiveRelations(archiveId uint, relationIds []uint) error {
	if len(relationIds) == 0 {
		w.DB.Where("`archive_id` = ?", archiveId).Delete(&model.ArchiveRelation{})
		return nil
	}
	for _, rid := range relationIds {
		arcRelation := model.ArchiveRelation{
			ArchiveId:  archiveId,
			RelationId: rid,
		}
		w.DB.Model(&model.ArchiveRelation{}).Where("`archive_id` = ? and `relation_id` = ?", archiveId, rid).FirstOrCreate(&arcRelation)
	}
	// 删除额外的
	w.DB.Unscoped().Where("`archive_id` = ? and `relation_id` NOT IN (?)", archiveId, relationIds).Delete(&model.ArchiveRelation{})

	return nil
}
