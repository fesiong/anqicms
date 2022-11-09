package provider

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func GetArchiveById(id uint) (*model.Archive, error) {
	var archive model.Archive
	db := dao.DB
	err := db.Where("`id` = ?", id).First(&archive).Error
	if err != nil {
		return nil, err
	}

	return &archive, nil
}

func GetUnscopedArchiveById(id uint) (*model.Archive, error) {
	var archive model.Archive
	db := dao.DB
	err := db.Unscoped().Where("`id` = ?", id).First(&archive).Error
	if err != nil {
		return nil, err
	}

	return &archive, nil
}

func GetArchiveByTitle(title string) (*model.Archive, error) {
	var archive model.Archive
	db := dao.DB
	err := db.Where("`title` = ?", title).First(&archive).Error
	if err != nil {
		return nil, err
	}

	return &archive, nil
}

func GetArchiveByFixedLink(link string) (*model.Archive, error) {
	var archive model.Archive
	db := dao.DB
	err := db.Where("`fixed_link` = ?", link).First(&archive).Error
	if err != nil {
		return nil, err
	}

	return &archive, nil
}

func GetArchiveByUrlToken(urlToken string) (*model.Archive, error) {
	var archive model.Archive
	db := dao.DB
	err := db.Where("`url_token` = ?", urlToken).First(&archive).Error
	if err != nil {
		return nil, err
	}

	return &archive, nil
}

func GetArchiveDataById(id uint) (*model.ArchiveData, error) {
	var data model.ArchiveData
	err := dao.DB.Where("`id` = ?", id).First(&data).Error
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func GetArchiveList(ops func(tx *gorm.DB) *gorm.DB, currentPage int, pageSize int) ([]*model.Archive, int64, error) {
	var archives []*model.Archive
	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := dao.DB.Model(&model.Archive{})

	if ops != nil {
		builder = ops(builder)
	}

	builder = builder.Count(&total).Limit(pageSize).Offset(offset)
	if err := builder.Find(&archives).Error; err != nil {
		return nil, 0, err
	}

	return archives, total, nil
}

func GetArchiveExtra(moduleId, id uint) map[string]*model.CustomField {
	//读取extra
	result := map[string]interface{}{}
	extraFields := map[string]*model.CustomField{}
	module := GetModuleFromCache(moduleId)
	if module != nil {
		var fields []string
		for _, v := range module.Fields {
			fields = append(fields, "`"+v.FieldName+"`")
		}
		//从数据库中取出来
		if len(fields) > 0 {
			dao.DB.Table(module.TableName).Where("`id` = ?", id).Select(strings.Join(fields, ",")).Scan(&result)
			//extra的CheckBox的值
			for _, v := range module.Fields {
				if v.Type == config.CustomFieldTypeImage || v.Type == config.CustomFieldTypeFile {
					value, ok := result[v.FieldName].(string)
					if ok && value != "" && !strings.HasPrefix(value, "http") && !strings.HasPrefix(value, "//") {
						result[v.FieldName] = config.JsonData.PluginStorage.StorageUrl + value
					}
				}
				extraFields[v.FieldName] = &model.CustomField{
					Name:    v.Name,
					Value:   result[v.FieldName],
					Default: v.Content,
				}
			}
		}
	}

	return extraFields
}

func SaveArchive(req *request.Archive) (archive *model.Archive, err error) {
	category, err := GetCategoryById(req.CategoryId)
	if err != nil {
		return nil, errors.New(config.Lang("请选择一个栏目"))
	}
	module := GetModuleFromCache(category.ModuleId)
	if module == nil {
		return nil, errors.New(config.Lang("未定义模型"))
	}

	newPost := false
	if req.Id > 0 {
		archive, err = GetArchiveById(req.Id)
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
		req.UrlToken = library.GetPinyin(req.Title)
	}
	archive.UrlToken = VerifyArchiveUrlToken(req.UrlToken, archive.Id)

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
				return nil, fmt.Errorf(config.Lang("%s必填"), v.Name)
			}
			if req.Extra[v.FieldName] != nil {
				extraValue, ok := req.Extra[v.FieldName].(map[string]interface{})
				if ok {
					if v.Required && extraValue["value"] == nil && extraValue["value"] == "" {
						return nil, fmt.Errorf(config.Lang("%s必填"), v.Name)
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
							extraFields[v.FieldName] = strings.TrimPrefix(value, config.JsonData.PluginStorage.StorageUrl)
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
	req.Content = strings.ReplaceAll(req.Content, config.JsonData.System.BaseUrl, "")
	//goquery
	htmlR := strings.NewReader(req.Content)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err == nil {
		baseHost := ""
		urls, err := url.Parse(config.JsonData.System.BaseUrl)
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
		if config.JsonData.Content.RemoteDownload == 1 {
			doc.Find("img").Each(func(i int, s *goquery.Selection) {
				src, exists := s.Attr("src")
				if exists && src != "" {
					alt := s.AttrOr("alt", "")
					imgUrl, err := url.Parse(src)
					if err == nil {
						if imgUrl.Host != "" && imgUrl.Host != baseHost && !strings.HasPrefix(src, config.JsonData.PluginStorage.StorageUrl) {
							//外链
							attachment, err := DownloadRemoteImage(src, alt)
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
		for i, v := range archive.Images {
			archive.Images[i] = strings.TrimPrefix(v, config.JsonData.PluginStorage.StorageUrl)
		}

		//过滤外链
		doc.Find("a").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists {
				aUrl, err := url.Parse(href)
				if err == nil {
					if aUrl.Host != "" && aUrl.Host != baseHost {
						//外链
						if config.JsonData.Content.FilterOutlink == 1 {
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
		go LogMaterialData(materialIds, "archive", archive.Id)

		//返回最终可用的内容
		req.Content, _ = doc.Find("body").Html()
	}
	// 限制数量
	descRunes := []rune(archive.Description)
	if len(descRunes) > 250 {
		archive.Description = string(descRunes[:250])
	}

	// 保存主表
	err = dao.DB.Save(archive).Error
	if err != nil {
		return nil, err
	}
	// 保存内容表
	archiveData := model.ArchiveData{
		Content: req.Content,
	}
	archiveData.Id = archive.Id
	err = dao.DB.Save(&archiveData).Error
	if err != nil {
		dao.DB.Delete(archive)
		return nil, err
	}

	//extra
	if len(extraFields) > 0 {
		//入库
		// 先检查是否存在
		var existsId uint
		dao.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Pluck("id", &existsId)
		if existsId > 0 {
			// 已存在
			dao.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Updates(extraFields)
		} else {
			// 新建
			extraFields["id"] = archive.Id
			dao.DB.Table(module.TableName).Where("`id` = ?", archive.Id).Create(extraFields)
		}
	}

	// tags
	_ = SaveTagData(archive.Id, req.Tags)

	// 缓存清理
	if oldFixedLink != "" || archive.FixedLink != "" {
		DeleteCacheFixedLinks()
	}

	// 尝试添加全文索引
	AddFulltextIndex(&TinyArchive{
		Id:       archive.Id,
		ModuleId: archive.ModuleId,
		Title:    archive.Title,
		Keywords: archive.Keywords,
		Content:  archiveData.Content,
	})

	err = SuccessReleaseArchive(archive, newPost)
	return
}

func SuccessReleaseArchive(archive *model.Archive, newPost bool) error {
	link := GetUrl("archive", archive, 0)
	//添加锚文本
	if config.JsonData.PluginAnchor.ReplaceWay == 1 {
		go ReplaceContent(nil, "archive", archive.Id, link)
	}
	//提取锚文本
	if config.JsonData.PluginAnchor.KeywordWay == 1 && archive.Status == config.ContentStatusOK {

		go AutoInsertAnchor(archive.Id, archive.Keywords, link)
	}

	DeleteCacheIndex()

	//新发布的文章，执行推送
	if newPost && archive.Status == config.ContentStatusOK {
		go PushArchive(link)
		if config.JsonData.PluginSitemap.AutoBuild == 1 {
			_ = AddonSitemap("archive", link, time.Unix(archive.UpdatedTime, 0).Format("2006-01-02"))
		}
	}

	return nil
}

func UpdateArchiveUrlToken(archive *model.Archive) error {
	if archive.UrlToken == "" {
		newToken := library.GetPinyin(archive.Title)
		archive.UrlToken = VerifyArchiveUrlToken(newToken, archive.Id)

		dao.DB.Model(&model.Archive{}).Where("`id` = ?", archive.Id).UpdateColumn("url_token", archive.UrlToken)
	}

	return nil
}

func RecoverArchive(archive *model.Archive) error {
	err := dao.DB.Unscoped().Model(&model.Archive{}).Where("id", archive.Id).UpdateColumn("deleted_at", nil).Error
	if err != nil {
		return err
	}

	if archive.FixedLink != "" {
		DeleteCacheFixedLinks()
	}
	DeleteCacheIndex()
	var doc TinyArchive
	dao.DB.Table("`archives` as a").Joins("left join `archive_data` as d on a.id=d.id").Select("a.id,a.title,a.keywords,a.module_id,d.content").Where("a.`id` > ?", archive.Id).Take(&doc)
	// 尝试添加全文索引
	AddFulltextIndex(&doc)

	return nil
}

func DeleteArchive(archive *model.Archive) error {
	if archive.DeletedAt.Valid {
		if err := dao.DB.Unscoped().Delete(archive).Error; err != nil {
			return err
		}
	} else {
		if err := dao.DB.Delete(archive).Error; err != nil {
			return err
		}
	}

	if archive.FixedLink != "" {
		DeleteCacheFixedLinks()
	}
	DeleteCacheIndex()
	RemoveFulltextIndex(archive.Id)

	return nil
}

func UpdateArchiveRecommend(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New("无可操作的文档")
	}
	err := dao.DB.Model(&model.Archive{}).Where("id IN (?)", req.Ids).UpdateColumn("flag", req.Flag).Error

	return err
}

func UpdateArchiveStatus(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(config.Lang("无可操作的文档"))
	}
	err := dao.DB.Model(&model.Archive{}).Where("`id` IN (?)", req.Ids).UpdateColumn("status", req.Status).Error
	// 如果选择的有待发布的内容，则将时间更新为当前时间
	if req.Status == config.ContentStatusOK {
		dao.DB.Model(&model.Archive{}).Where("`id` IN (?) and `created_time` > ?", req.Ids, time.Now().Unix()).UpdateColumn("created_time", time.Now().Unix())
	}
	return err
}

func UpdateArchiveCategory(req *request.ArchivesUpdateRequest) error {
	if len(req.Ids) == 0 {
		return errors.New(config.Lang("无可操作的文档"))
	}
	err := dao.DB.Model(&model.Archive{}).Where("id IN (?)", req.Ids).UpdateColumn("category_id", req.CategoryId).Error

	return err
}

// DeleteCacheFixedLinks 固定链接
func DeleteCacheFixedLinks() {
	library.MemCache.Delete("fixedLinks")
}

func GetCacheFixedLinks() map[string]uint {
	if dao.DB == nil {
		return nil
	}
	var fixedLinks = map[string]uint{}

	result := library.MemCache.Get("fixedLinks")
	if result != nil {
		var ok bool
		fixedLinks, ok = result.(map[string]uint)
		if ok {
			return fixedLinks
		}
	}

	var archives []model.Archive
	dao.DB.Model(model.Archive{}).Where("`fixed_link` != ''").Select("fixed_link", "id").Scan(&archives)
	for i := range archives {
		fixedLinks[archives[i].FixedLink] = archives[i].Id
	}

	library.MemCache.Set("fixedLinks", fixedLinks, 0)

	return fixedLinks
}

func GetFixedLinkFromCache(fixedLink string) uint {
	links := GetCacheFixedLinks()

	archiveId, ok := links[fixedLink]
	if ok {
		return archiveId
	}

	return 0
}

// PublishPlanArchives 发布计划文章，单次最多处理100篇
func PublishPlanArchives() {
	timeStamp := time.Now().Unix()

	var archives []*model.Archive
	dao.DB.Model(&model.Archive{}).Where("`status` = ? and created_time < ?", config.ContentStatusPlan, timeStamp).Limit(100).Find(&archives)
	if len(archives) > 0 {
		for _, archive := range archives {
			archive.Status = config.ContentStatusOK
			dao.DB.Save(archive)

			link := GetUrl("archive", archive, 0)

			//提取锚文本
			if config.JsonData.PluginAnchor.KeywordWay == 1 {
				go AutoInsertAnchor(archive.Id, archive.Keywords, link)
			}
			go PushArchive(link)
			if config.JsonData.PluginSitemap.AutoBuild == 1 {
				_ = AddonSitemap("archive", link, time.Unix(archive.UpdatedTime, 0).Format("2006-01-02"))
			}
		}
	}
}

// CleanArchives 计划任务删除存档，30天前被删除的
func CleanArchives() {
	if dao.DB == nil {
		return
	}
	var archives []model.Archive
	dao.DB.Debug().Model(&model.Archive{}).Unscoped().Where("`deleted_at` is not null AND `deleted_at` < ?", time.Now().AddDate(0, 0, -30)).Find(&archives)
	if len(archives) > 0 {
		modules := GetCacheModules()
		var mapModules = map[uint]model.Module{}
		for _, v := range modules {
			mapModules[v.Id] = v
		}
		for _, archive := range archives {
			dao.DB.Unscoped().Where("id = ?", archive.Id).Delete(model.ArchiveData{})
			if module, ok := mapModules[archive.ModuleId]; ok {
				dao.DB.Unscoped().Where("id = ?", archive.Id).Delete(module.TableName)
			}
			dao.DB.Unscoped().Where("id = ?", archive.Id).Delete(model.Archive{})
		}
	}
}

func VerifyArchiveUrlToken(urlToken string, id uint) string {
	index := 0
	for {
		tmpToken := urlToken
		if index > 0 {
			tmpToken = fmt.Sprintf("%s-%d", urlToken, index)
		}
		// 判断分类
		_, err := GetCategoryByUrlToken(tmpToken)
		if err == nil {
			index++
			continue
		}
		// 判断archive
		tmpArc, err := GetArchiveByUrlToken(tmpToken)
		if err == nil && tmpArc.Id != id {
			index++
			continue
		}
		urlToken = tmpToken
		break
	}

	return urlToken
}

func CheckArchiveHasOrder(userId uint, archiveId uint) bool {
	if userId == 0 || archiveId == 0 {
		return false
	}
	var exist int64
	dao.DB.Table("`orders` as o").Joins("INNER JOIN `order_details` as d ON o.order_id = d.order_id AND d.`goods_id` = ?", archiveId).Where("o.user_id = ? AND o.`status` IN(?)", userId, []int{
		config.OrderStatusPaid,
		config.OrderStatusDelivering,
		config.OrderStatusCompleted}).Count(&exist)
	if exist > 0 {
		return true
	}

	//var orderIds []string
	//dao.DB.Model(&model.OrderDetail{}).Where("`user_id` = ? and `goods_id` = ?", userId, archiveId).Pluck("order_id", &orderIds)
	//if len(orderIds) == 0 {
	//	return false
	//}
	//
	//var exist int64
	//dao.DB.Model(&model.Order{}).Where("`order_id` IN(?) and `status` > ?", orderIds, 0).Count(&exist)
	//if exist > 0 {
	//	return true
	//}

	return false
}
