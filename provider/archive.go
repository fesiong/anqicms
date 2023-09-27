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
	result := w.MemCache.Get(fmt.Sprintf("archive-%d", id))
	if result != nil {
		var ok bool
		archive, ok = result.(*model.Archive)
		if ok {
			return archive
		}
	}

	return nil
}

func (w *Website) AddArchiveCache(archive *model.Archive) {
	w.MemCache.Set(fmt.Sprintf("archive-%d", archive.Id), archive, 300)
}

func (w *Website) DeleteArchiveCache(id uint) {
	w.MemCache.Delete(fmt.Sprintf("archive-%d", id))
}

func (w *Website) GetArchiveById(id uint) (*model.Archive, error) {
	return w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`id` = ?", id)
	})
}

func (w *Website) GetUnscopedArchiveById(id uint) (*model.Archive, error) {
	return w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Unscoped().Where("`id` = ?", id)
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

func (w *Website) GetArchiveDataById(id uint) (*model.ArchiveData, error) {
	var data model.ArchiveData
	err := w.DB.Where("`id` = ?", id).First(&data).Error
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func (w *Website) GetArchiveList(ops func(tx *gorm.DB) *gorm.DB, currentPage, pageSize int, offsets ...int) ([]*model.Archive, int64, error) {
	var archives []*model.Archive

	offset := 0
	if currentPage > 0 {
		offset = (currentPage - 1) * pageSize
	}
	if len(offsets) > 0 {
		offset = offsets[0]
	}
	var total int64
	// 对于没有分页的list，则缓存
	var cacheKey = ""
	if currentPage == 0 {
		sql := w.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
			if ops != nil {
				tx = ops(tx)
			}
			return tx.Limit(pageSize).Offset(offset).Find(&[]*model.Archive{})
		})
		cacheKey = "archive-list-" + library.Md5(sql)[8:24]
		result := w.MemCache.Get(cacheKey)
		if result != nil {
			var ok bool
			archives, ok = result.([]*model.Archive)
			if ok {
				return archives, int64(len(archives)), nil
			}
		}
	}
	builder := w.DB.Model(&model.Archive{})

	if ops != nil {
		builder = ops(builder)
	}

	if currentPage > 0 {
		builder.Count(&total)
	}
	builder = builder.Limit(pageSize).Offset(offset)
	if err := builder.Find(&archives).Error; err != nil {
		return nil, 0, err
	}
	for i := range archives {
		archives[i].GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
		archives[i].Link = w.GetUrl("archive", archives[i], 0)
	}
	// 对于没有分页的list，则缓存
	if currentPage == 0 {
		w.MemCache.Set(cacheKey, archives, 60)
	}
	return archives, total, nil
}

func (w *Website) GetArchiveExtraFromCache(archiveId uint) (archive map[string]*model.CustomField) {
	result := w.MemCache.Get(fmt.Sprintf("archive-extra-%d", archiveId))
	if result != nil {
		var ok bool
		archive, ok = result.(map[string]*model.CustomField)
		if ok {
			return archive
		}
	}

	return nil
}

func (w *Website) AddArchiveExtraCache(archiveId uint, extra map[string]*model.CustomField) {
	w.MemCache.Set(fmt.Sprintf("archive-extra-%d", archiveId), extra, 60)
}

func (w *Website) DeleteArchiveExtraCache(archiveId uint) {
	w.MemCache.Delete(fmt.Sprintf("archive-extra-%d", archiveId))
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
				extraFields[v.FieldName] = &model.CustomField{
					Name:        v.Name,
					Value:       result[v.FieldName],
					Default:     v.Content,
					FollowLevel: v.FollowLevel,
				}
			}
		}
		if loadCache {
			w.AddArchiveExtraCache(id, extraFields)
		}
	}

	return extraFields
}

func (w *Website) SaveArchive(req *request.Archive) (archive *model.Archive, err error) {
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
		return nil, errors.New(w.Lang("请选择一个栏目"))
	}
	for _, catId := range req.CategoryIds {
		category := w.GetCategoryFromCache(catId)
		if category == nil || category.Type != config.CategoryTypeArchive {
			return nil, errors.New(w.Lang("请选择一个栏目"))
		}
	}
	req.CategoryId = req.CategoryIds[0]
	category, err := w.GetCategoryById(req.CategoryId)
	if err != nil {
		return nil, errors.New(w.Lang("请选择一个栏目"))
	}
	module := w.GetModuleFromCache(category.ModuleId)
	if module == nil {
		return nil, errors.New(w.Lang("未定义模型"))
	}
	if len(req.Title) == 0 {
		return nil, errors.New(w.Lang("请填写文章标题"))
	}
	newPost := false
	if req.Id > 0 {
		archive, err = w.GetArchiveById(req.Id)
		if err != nil {
			return nil, err
		}
	} else {
		newPost = true
		archive = &model.Archive{
			Status: 1,
		}
	}
	// createdTime
	if req.CreatedTime > 0 {
		archive.CreatedTime = req.CreatedTime
	}
	archive.Status = config.ContentStatusOK
	if req.Draft {
		archive.Status = config.ContentStatusDraft
	}
	if archive.CreatedTime > time.Now().Unix() {
		// 未来时间，设置为待发布
		archive.Status = config.ContentStatusPlan
	}
	// 判断重复
	req.UrlToken = library.ParseUrlToken(req.UrlToken)
	if req.UrlToken == "" {
		req.UrlToken = library.GetPinyin(req.Title, w.Content.UrlTokenType == config.UrlTokenTypeSort)
	}
	if req.UrlToken == "" {
		req.UrlToken = time.Now().Format("a-20060102150405")
	}
	archive.UrlToken = w.VerifyArchiveUrlToken(req.UrlToken, archive.Id)
	if utf8.RuneCountInString(req.Title) > 250 {
		req.Title = string([]rune(req.Title)[:250])
		if strings.Count(req.Title, " ") > 1 {
			req.Title = req.Title[:strings.LastIndexAny(req.Title, " ")]
		}
	}
	archive.ModuleId = category.ModuleId
	archive.Title = req.Title
	archive.SeoTitle = req.SeoTitle
	archive.Keywords = req.Keywords
	archive.Description = req.Description
	archive.CategoryId = req.CategoryId
	archive.Images = req.Images
	archive.Template = req.Template
	archive.CanonicalUrl = req.CanonicalUrl
	archive.Flag = req.Flag
	oldFixedLink := archive.FixedLink
	archive.FixedLink = req.FixedLink
	archive.Price = req.Price
	archive.Stock = req.Stock
	archive.ReadLevel = req.ReadLevel
	archive.Password = req.Password
	archive.Sort = req.Sort
	if req.UserId > 0 {
		archive.UserId = req.UserId
	}

	if req.KeywordId > 0 {
		archive.KeywordId = req.KeywordId
	}
	if req.OriginUrl != "" {
		if utf8.RuneCountInString(req.OriginUrl) > 190 {
			req.OriginUrl = string([]rune(req.OriginUrl)[:190])
		}
		archive.OriginUrl = req.OriginUrl
	}
	if req.OriginTitle != "" {
		if utf8.RuneCountInString(req.OriginTitle) > 190 {
			req.OriginTitle = string([]rune(req.OriginTitle)[:190])
		}
		archive.OriginTitle = req.OriginTitle
	}

	//extra
	extraFields := map[string]interface{}{}
	if len(module.Fields) > 0 {
		for _, v := range module.Fields {
			//先检查是否有必填而没有填写的
			if v.Required && req.Extra[v.FieldName] == nil {
				return nil, fmt.Errorf(w.Lang("%s必填"), v.Name)
			}
			if req.Extra[v.FieldName] != nil {
				extraValue, ok := req.Extra[v.FieldName].(map[string]interface{})
				if ok {
					if v.Required && extraValue["value"] == nil && extraValue["value"] == "" {
						return nil, fmt.Errorf(w.Lang("%s必填"), v.Name)
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

	// 将单个&nbsp;替换为空格
	req.Content = library.ReplaceSingleSpace(req.Content)
	req.Content = strings.ReplaceAll(req.Content, w.System.BaseUrl, "")
	baseHost := ""
	urls, err := url.Parse(w.System.BaseUrl)
	if err == nil {
		baseHost = urls.Host
	}
	autoAddImage := false
	//提取描述
	if req.Description == "" {
		tmpContent := req.Content
		if w.Content.Editor == "markdown" {
			tmpContent = library.MarkdownToHTML(tmpContent)
		}
		archive.Description = library.ParseDescription(strings.ReplaceAll(CleanTagsAndSpaces(tmpContent), "\n", " "))
	}
	//提取缩略图
	if len(archive.Images) == 0 {
		re, _ := regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
		match := re.FindStringSubmatch(req.Content)
		if len(match) > 1 {
			archive.Images = append(archive.Images, match[1])
			autoAddImage = true
		} else {
			// 匹配Markdown ![新的图片](http://xxx/xxx.webp)
			re, _ = regexp.Compile(`!\[([^]]*)\]\(([^)]+)\)`)
			match = re.FindStringSubmatch(req.Content)
			if len(match) > 2 {
				archive.Images = append(archive.Images, match[2])
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

	// 限制数量
	descRunes := []rune(archive.Description)
	if len(descRunes) > 1000 {
		archive.Description = string(descRunes[:1000])
	}
	for i, v := range archive.Images {
		archive.Images[i] = strings.TrimPrefix(v, w.PluginStorage.StorageUrl)
	}
	// 如果没有图片
	if len(archive.Images) == 0 && w.PluginTitleImage.Open {
		// 自动生成一个
		logo, err := w.NewTitleImage(archive.Title).Save(w)
		if err == nil {
			archive.Images = append(archive.Images, logo)
		}
	}

	// 保存主表
	err = w.DB.Save(archive).Error
	if err != nil {
		return nil, err
	}
	// 保存内容表
	archiveData := model.ArchiveData{
		Content: req.Content,
	}
	archiveData.Id = archive.Id
	err = w.DB.Save(&archiveData).Error
	if err != nil {
		w.DB.Delete(archive)
		return nil, err
	}
	// 保存分类ID
	for _, catId := range req.CategoryIds {
		arcCategory := model.ArchiveCategory{
			CategoryId: catId,
			ArchiveId:  archive.Id,
		}
		w.DB.Model(&model.ArchiveCategory{}).Where("`category_id` = ? and `archive_id` = ?", catId, archive.Id).FirstOrCreate(&arcCategory)
	}
	// 删除额外的
	w.DB.Unscoped().Where("`archive_id` = ? and `category_id` NOT IN (?)", archive.Id, req.CategoryIds).Delete(&model.ArchiveCategory{})
	// end
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
	go w.LogMaterialData(materialIds, "archive", archive.Id)
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
				archive.Images = archive.Images[:0]
				re, _ = regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
				match := re.FindStringSubmatch(req.Content)
				if len(match) > 1 {
					archive.Images = append(archive.Images, match[1])
				} else {
					// 匹配Markdown ![新的图片](http://xxx/xxx.webp)
					re, _ = regexp.Compile(`!\[([^]]*)\]\(([^)]+)\)`)
					match = re.FindStringSubmatch(req.Content)
					if len(match) > 2 {
						archive.Images = append(archive.Images, match[2])
					}
				}
				w.DB.Model(archive).UpdateColumn("images", archive.Images)
			}
		}
	}
	//extra
	if len(extraFields) > 0 {
		//入库
		// 先检查是否存在
		var existsId uint
		w.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Pluck("id", &existsId)
		if existsId > 0 {
			// 已存在
			w.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Updates(extraFields)
		} else {
			// 新建
			extraFields["id"] = archive.Id
			w.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Create(extraFields)
		}
	}

	// tags
	_ = w.SaveTagData(archive.Id, req.Tags)

	// 缓存清理
	if oldFixedLink != "" || archive.FixedLink != "" {
		w.DeleteCacheFixedLinks()
	}
	// 清除缓存
	w.DeleteArchiveCache(archive.Id)
	w.DeleteArchiveExtraCache(archive.Id)

	// 尝试添加全文索引
	w.AddFulltextIndex(&TinyArchive{
		Id:       archive.Id,
		ModuleId: archive.ModuleId,
		Title:    archive.Title,
		Keywords: archive.Keywords,
		Content:  archiveData.Content,
	})

	err = w.SuccessReleaseArchive(archive, newPost)
	return
}

func (w *Website) SuccessReleaseArchive(archive *model.Archive, newPost bool) error {
	archive.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
	archive.Link = w.GetUrl("archive", archive, 0)
	//添加锚文本
	if w.PluginAnchor.ReplaceWay == 1 {
		go w.ReplaceContent(nil, "archive", archive.Id, archive.Link)
	}
	//提取锚文本
	if w.PluginAnchor.KeywordWay == 1 && archive.Status == config.ContentStatusOK {

		go w.AutoInsertAnchor(archive.Id, archive.Keywords, archive.Link)
	}

	w.DeleteCacheIndex()

	//新发布的文章，执行推送
	if newPost && archive.Status == config.ContentStatusOK {
		go w.PushArchive(archive.Link)
		if w.PluginSitemap.AutoBuild == 1 {
			_ = w.AddonSitemap("archive", archive.Link, time.Unix(archive.UpdatedTime, 0).Format("2006-01-02"))
		}
	}

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

func (w *Website) RecoverArchive(archive *model.Archive) error {
	err := w.DB.Unscoped().Model(&model.Archive{}).Where("id", archive.Id).UpdateColumn("deleted_at", nil).Error
	if err != nil {
		return err
	}
	// 恢复 文档分类
	w.DB.Unscoped().Model(&model.ArchiveCategory{}).Where("`archive_id` = ?", archive.Id).UpdateColumn("deleted_at", nil)

	if archive.FixedLink != "" {
		w.DeleteCacheFixedLinks()
	}
	w.DeleteCacheIndex()
	var doc TinyArchive
	w.DB.Table("`archives` as a").Joins("left join `archive_data` as d on a.id=d.id").Select("a.id,a.title,a.keywords,a.module_id,d.content").Where("a.`id` > ?", archive.Id).Take(&doc)
	// 尝试添加全文索引
	w.AddFulltextIndex(&doc)

	return nil
}

func (w *Website) DeleteArchive(archive *model.Archive) error {
	if archive.DeletedAt.Valid {
		if err := w.DB.Unscoped().Delete(archive).Error; err != nil {
			return err
		}
		w.DB.Unscoped().Where("`id` = ?", archive.Id).Delete(&model.ArchiveData{})
		// 删除 文档分类
		w.DB.Unscoped().Where("`archive_id` = ?", archive.Id).Delete(&model.ArchiveCategory{})
	} else {
		if err := w.DB.Delete(archive).Error; err != nil {
			return err
		}
		// 删除 文档分类
		w.DB.Where("`archive_id` = ?", archive.Id).Delete(&model.ArchiveCategory{})
	}

	if archive.FixedLink != "" {
		w.DeleteCacheFixedLinks()
	}
	w.DeleteCacheIndex()
	w.RemoveHtmlCache(w.GetUrl("archive", archive, 0))
	w.RemoveFulltextIndex(archive.Id)

	return nil
}

func (w *Website) UpdateArchiveRecommend(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New("无可操作的文档")
	}
	err := w.DB.Model(&model.Archive{}).Where("id IN (?)", req.Ids).UpdateColumn("flag", req.Flag).Error

	return err
}

func (w *Website) UpdateArchiveStatus(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(w.Lang("无可操作的文档"))
	}
	err := w.DB.Model(&model.Archive{}).Where("`id` IN (?)", req.Ids).UpdateColumn("status", req.Status).Error
	// 如果选择的有待发布的内容，则将时间更新为当前时间
	if req.Status == config.ContentStatusOK {
		w.DB.Model(&model.Archive{}).Where("`id` IN (?) and `created_time` > ?", req.Ids, time.Now().Unix()).UpdateColumn("created_time", time.Now().Unix())
	}
	return err
}

func (w *Website) UpdateArchiveTime(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(w.Lang("无可操作的文档"))
	}
	var err error
	if req.Time == 4 {
		// updated_time 所有文档
		err = w.DB.Model(&model.Archive{}).UpdateColumn("updated_time", time.Now().Unix()).Error
	} else if req.Time == 3 {
		// created_time 所有文档
		err = w.DB.Model(&model.Archive{}).UpdateColumn("created_time", time.Now().Unix()).Error
	} else if req.Time == 2 {
		// updated_time
		err = w.DB.Model(&model.Archive{}).Where("`id` IN (?)", req.Ids).UpdateColumn("updated_time", time.Now().Unix()).Error
	} else {
		err = w.DB.Model(&model.Archive{}).Where("`id` IN (?)", req.Ids).UpdateColumn("created_time", time.Now().Unix()).Error
	}
	return err
}

func (w *Website) UpdateArchiveReleasePlan(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(w.Lang("无可操作的文档"))
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
		archive, err := w.GetArchiveById(id)
		if err != nil {
			// 文档不存在，跳过
			continue
		}
		if archive.Status == config.ContentStatusOK {
			// 正常的文档跳过
			continue
		}
		num++
		w.DB.Model(&model.Archive{}).Where("`id` = ?", archive.Id).UpdateColumns(map[string]interface{}{
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
		return errors.New(w.Lang("无可操作的文档"))
	}
	// 保存分类ID
	if len(req.CategoryIds) == 0 && req.CategoryId > 0 {
		req.CategoryIds = append(req.CategoryIds, req.CategoryId)
	}
	for _, catId := range req.CategoryIds {
		_, err := w.GetCategoryById(catId)
		if err != nil {
			return errors.New("分类不存在")
		}
	}
	if len(req.CategoryIds) == 0 {
		return errors.New("请选择分类")
	}
	for _, arcId := range req.Ids {
		for _, catId := range req.CategoryIds {
			arcCategory := model.ArchiveCategory{
				CategoryId: catId,
				ArchiveId:  arcId,
			}
			w.DB.Model(&model.ArchiveCategory{}).Where("`category_id` = ? and `archive_id` = ?", catId, arcId).FirstOrCreate(&arcCategory)
		}
		// 删除额外的
		w.DB.Unscoped().Where("`archive_id` = ? and `category_id` NOT IN (?)", arcId, req.CategoryIds).Delete(&model.ArchiveCategory{})
		// 更新主分类ID
		w.DB.Model(&model.Archive{}).Where("`id` = ?", arcId).UpdateColumn("category_id", req.CategoryIds[0])
	}
	// end

	return nil
}

// DeleteCacheFixedLinks 固定链接
func (w *Website) DeleteCacheFixedLinks() {
	w.MemCache.Delete("fixedLinks")
}

func (w *Website) GetCacheFixedLinks() map[string]uint {
	if w.DB == nil {
		return nil
	}
	var fixedLinks = map[string]uint{}

	result := w.MemCache.Get("fixedLinks")
	if result != nil {
		var ok bool
		fixedLinks, ok = result.(map[string]uint)
		if ok {
			return fixedLinks
		}
	}

	var archives []model.Archive
	w.DB.Model(model.Archive{}).Where("`fixed_link` != ''").Select("fixed_link", "id").Scan(&archives)
	for i := range archives {
		fixedLinks[archives[i].FixedLink] = archives[i].Id
	}

	w.MemCache.Set("fixedLinks", fixedLinks, 0)

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

	var archives []*model.Archive
	w.DB.Model(&model.Archive{}).Where("`status` = ? and created_time < ?", config.ContentStatusPlan, timeStamp).Limit(100).Find(&archives)
	if len(archives) > 0 {
		for _, archive := range archives {
			archive.Status = config.ContentStatusOK
			w.DB.Save(archive)

			link := w.GetUrl("archive", archive, 0)

			//提取锚文本
			if w.PluginAnchor.KeywordWay == 1 {
				go w.AutoInsertAnchor(archive.Id, archive.Keywords, link)
			}
			go w.PushArchive(link)
			if w.PluginSitemap.AutoBuild == 1 {
				_ = w.AddonSitemap("archive", link, time.Unix(archive.UpdatedTime, 0).Format("2006-01-02"))
			}
		}
	}
}

// CleanArchives 计划任务删除存档，30天前被删除的
func (w *Website) CleanArchives() {
	if w.DB == nil {
		return
	}
	var archives []model.Archive
	w.DB.Model(&model.Archive{}).Unscoped().Where("`deleted_at` is not null AND `deleted_at` < ?", time.Now().AddDate(0, 0, -30)).Find(&archives)
	if len(archives) > 0 {
		modules := w.GetCacheModules()
		var mapModules = map[uint]model.Module{}
		for _, v := range modules {
			mapModules[v.Id] = v
		}
		for _, archive := range archives {
			w.DB.Unscoped().Where("id = ?", archive.Id).Delete(model.ArchiveData{})
			if module, ok := mapModules[archive.ModuleId]; ok {
				w.DB.Unscoped().Where("id = ?", archive.Id).Delete(module.TableName)
			}
			w.DB.Unscoped().Where("id = ?", archive.Id).Delete(model.Archive{})
		}
	}
}

func (w *Website) VerifyArchiveUrlToken(urlToken string, id uint) string {
	index := 0
	// 防止超出长度
	if len(urlToken) > 150 {
		urlToken = urlToken[:150]
	}
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
			w.DB.Table("`orders` as o").Joins("INNER JOIN `order_details` as d ON o.order_id = d.order_id AND d.`goods_id` = ?", archive.Id).Where("o.user_id = ? AND o.`status` IN(?)", userId, []int{
				config.OrderStatusPaid,
				config.OrderStatusDelivering,
				config.OrderStatusCompleted}).Count(&exist)
			if exist > 0 {
				archive.HasOrdered = true
			}

			archive.HasOrdered = false
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
			w.DB.Model(&model.ArchiveCategory{}).Where("`category_id` = ? and `archive_id` = ?", arc.CategoryId, arc.Id).FirstOrCreate(&arcCategory)
		}
	}
}
