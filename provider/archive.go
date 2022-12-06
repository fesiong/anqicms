package provider

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"
)

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
	return w.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`url_token` = ?", urlToken)
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

	log.Printf("%#v, %#v, %#v", currentPage, pageSize, offsets)
	builder := w.DB.Model(&model.Archive{}).Debug()

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
	return archives, total, nil
}

func (w *Website) GetArchiveExtra(moduleId, id uint) map[string]*model.CustomField {
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
	}

	return extraFields
}

func (w *Website) SaveArchive(req *request.Archive) (archive *model.Archive, err error) {
	category, err := w.GetCategoryById(req.CategoryId)
	if err != nil {
		return nil, errors.New(w.Lang("请选择一个栏目"))
	}
	module := w.GetModuleFromCache(category.ModuleId)
	if module == nil {
		return nil, errors.New(w.Lang("未定义模型"))
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
	archive.UrlToken = w.VerifyArchiveUrlToken(req.UrlToken, archive.Id)

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
	if req.UserId > 0 {
		archive.UserId = req.UserId
	}

	if req.KeywordId > 0 {
		archive.KeywordId = req.KeywordId
	}
	if req.OriginUrl != "" {
		archive.OriginUrl = req.OriginUrl
	}
	if req.OriginTitle != "" {
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
	//goquery
	htmlR := strings.NewReader(req.Content)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err == nil {
		baseHost := ""
		urls, err := url.Parse(w.System.BaseUrl)
		if err == nil {
			baseHost = urls.Host
		}

		//提取描述
		if req.Description == "" {
			textRune := []rune(strings.ReplaceAll(CleanTagsAndSpaces(doc.Text()), "\n", " "))
			if len(textRune) > 150 {
				archive.Description = string(textRune[:150])
			} else {
				archive.Description = string(textRune)
			}
		}
		//下载远程图片
		if w.Content.RemoteDownload == 1 {
			doc.Find("img").Each(func(i int, s *goquery.Selection) {
				src, exists := s.Attr("src")
				if exists && src != "" {
					alt := s.AttrOr("alt", "")
					imgUrl, err := url.Parse(src)
					if err == nil {
						if imgUrl.Host != "" && imgUrl.Host != baseHost && !strings.HasPrefix(src, w.PluginStorage.StorageUrl) {
							//外链
							attachment, err := w.DownloadRemoteImage(src, alt)
							if err == nil {
								s.SetAttr("src", attachment.Logo)
							}
						}
					}
				} else {
					s.Remove()
				}
			})
		}
		//提取缩略图
		if len(archive.Images) == 0 {
			imgSections := doc.Find("img")
			if imgSections.Length() > 0 {
				//获取第一条
				src := imgSections.Eq(0).AttrOr("src", "")
				if src != "" {
					archive.Images = append(archive.Images, src)
				}
			}
		}

		//过滤外链
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists {
				aUrl, err := url.Parse(href)
				if err == nil {
					if aUrl.Host != "" && aUrl.Host != baseHost {
						//外链
						if w.Content.FilterOutlink == 1 {
							//过滤外链
							s.Contents().Unwrap()
						} else {
							//增加nofollow
							s.SetAttr("rel", "nofollow")
						}
					}
				}
			}
		})
		//检查有多少个material
		var materialIds []uint
		doc.Find("div[data-material]").Each(func(i int, s *goquery.Selection) {
			tmpId, exists := s.Attr("data-material")
			if exists {
				//记录material
				materialId, err := strconv.Atoi(tmpId)
				if err == nil {
					materialIds = append(materialIds, uint(materialId))
				}
			}
		})
		go w.LogMaterialData(materialIds, "archive", archive.Id)

		//返回最终可用的内容
		req.Content, _ = doc.Find("body").Html()
	}
	// 限制数量
	descRunes := []rune(archive.Description)
	if len(descRunes) > 250 {
		archive.Description = string(descRunes[:250])
	}
	for i, v := range archive.Images {
		archive.Images[i] = strings.TrimPrefix(v, w.PluginStorage.StorageUrl)
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
	} else {
		if err := w.DB.Delete(archive).Error; err != nil {
			return err
		}
	}

	if archive.FixedLink != "" {
		w.DeleteCacheFixedLinks()
	}
	w.DeleteCacheIndex()
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

func (w *Website) UpdateArchiveCategory(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(w.Lang("无可操作的文档"))
	}
	err := w.DB.Model(&model.Archive{}).Where("id IN (?)", req.Ids).UpdateColumn("category_id", req.CategoryId).Error

	return err
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
	w.DB.Debug().Model(&model.Archive{}).Unscoped().Where("`deleted_at` is not null AND `deleted_at` < ?", time.Now().AddDate(0, 0, -30)).Find(&archives)
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

func (w *Website) CheckArchiveHasOrder(userId uint, archiveId uint) bool {
	if userId == 0 || archiveId == 0 {
		return false
	}
	var exist int64
	w.DB.Table("`orders` as o").Joins("INNER JOIN `order_details` as d ON o.order_id = d.order_id AND d.`goods_id` = ?", archiveId).Where("o.user_id = ? AND o.`status` IN(?)", userId, []int{
		config.OrderStatusPaid,
		config.OrderStatusDelivering,
		config.OrderStatusCompleted}).Count(&exist)
	if exist > 0 {
		return true
	}

	//var orderIds []string
	//w.DB.Model(&model.OrderDetail{}).Where("`user_id` = ? and `goods_id` = ?", userId, archiveId).Pluck("order_id", &orderIds)
	//if len(orderIds) == 0 {
	//	return false
	//}
	//
	//var exist int64
	//w.DB.Model(&model.Order{}).Where("`order_id` IN(?) and `status` > ?", orderIds, 0).Count(&exist)
	//if exist > 0 {
	//	return true
	//}

	return false
}
