package provider

import (
	"errors"
	"fmt"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"log"
	"strconv"
	"strings"
	"time"
)

type MultiLangSyncStatus struct {
	w           *Website
	Finished    bool   `json:"finished"` // true | false
	FinishCount int64  `json:"finish_count"`
	TotalCount  int64  `json:"total_count"`
	Percent     int64  `json:"percent"` // 0-100
	Message     string `json:"message"` // current message
}

var multiLangSyncStatus *MultiLangSyncStatus

func (w *Website) GetMultiLangSyncStatus() *MultiLangSyncStatus {
	return multiLangSyncStatus
}

func (w *Website) NewMultiLangSync() (*MultiLangSyncStatus, error) {
	if multiLangSyncStatus != nil && multiLangSyncStatus.Finished == false {
		return nil, errors.New(w.Tr("TaskIsRunningPleaseWait"))
	}

	multiLangSyncStatus = &MultiLangSyncStatus{
		w:        w,
		Finished: false,
		Percent:  0,
		Message:  "",
	}

	return multiLangSyncStatus, nil
}

func (w *Website) GetMainWebsite() *Website {
	if w.ParentId > 0 {
		return GetWebsite(w.ParentId)
	}

	return w
}

func (w *Website) GetMultiLangSites(mainId uint) []*model.Website {
	db := GetDefaultDB()
	// 用于读取真实的主站点ID
	if mainId == 0 {
		mainId = w.Id
	}
	var defaultSite *model.Website
	err := db.Model(&model.Website{}).Where("id = ?", mainId).Omit("token_secret", "mysql", "root_path").Take(&defaultSite).Error
	if err != nil {
		return nil
	}
	mainSite := GetWebsite(mainId)
	if mainSite == nil || mainSite.MultiLanguage.Open == false {
		return nil
	}
	defaultSite.IsMain = true
	var sites []*model.Website
	db.Model(&model.Website{}).Where("parent_id = ?", mainId).Omit("token_secret", "mysql", "root_path").Order("id asc").Find(&sites)
	sites = append(sites)
	newSites := make([]*model.Website, len(sites)+1)
	newSites[0] = defaultSite
	copy(newSites[1:], sites)
	for i := range newSites {
		newSites[i].IsCurrent = w.Id == newSites[i].Id
		currentSite := GetWebsite(newSites[i].Id)
		if currentSite != nil {
			newSites[i].Language = currentSite.System.Language
			newSites[i].BaseUrl = currentSite.System.BaseUrl
			newSites[i].ErrorMsg = currentSite.ErrorMsg
			if !currentSite.Initialed {
				newSites[i].Status = 0
			} else {
				// 根据选择的模式来生成链接
			}
		} else {
			newSites[i].Status = 0
		}
	}
	return newSites
}

func (w *Website) GetMultiLangValidSites(mainId uint) []*model.Website {
	db := GetDefaultDB()
	var sites []*model.Website
	// 排除所有的主站点
	var parentIds []uint
	db.Model(&model.Website{}).Where("parent_id != 0").Pluck("parent_id", &parentIds)
	parentIds = append(parentIds, mainId)
	// 排除所有的主站点以及其他主站点的子站点
	db.Model(&model.Website{}).Where("(parent_id = ? or parent_id = 0) and id NOT IN (?)", mainId, parentIds).Order("id asc").Find(&sites)
	for i := range sites {
		currentSite := GetWebsite(sites[i].Id)
		if currentSite != nil {
			sites[i].Language = currentSite.System.Language
			sites[i].BaseUrl = currentSite.System.BaseUrl
			sites[i].ErrorMsg = currentSite.ErrorMsg
			if !currentSite.Initialed {
				sites[i].Status = 0
			}
		} else {
			sites[i].Status = 0
		}
	}
	return sites
}

func (w *Website) RemoveMultiLangSite(siteId uint) error {
	// 先移除parentId
	db := GetDefaultDB()
	err := db.Model(&model.Website{}).Where("id = ?", siteId).Update("parent_id", 0).Error
	if err != nil {
		return err
	}
	// 修改运行中的状态
	targetSite := GetWebsite(siteId)
	if targetSite != nil {
		// 移除当前语言
		mainSite := GetWebsite(siteId)
		delete(mainSite.MultiLanguage.SubSites, targetSite.System.Language)

		targetSite.ParentId = 0
	}
	// todo

	return nil
}

func (w *Website) SaveMultiLangSite(req *request.PluginMultiLangSiteRequest) error {
	db := GetDefaultDB()
	// 语言不能重复，除other外
	mainDBSite, err := GetDBWebsiteInfo(req.ParentId)
	if err != nil {
		return err
	}

	// 不允许和主站点相同
	if req.Id == req.ParentId {
		return errors.New(w.Tr("ErrorSameSite"))
	}
	if req.Language != "other" {
		if req.Language == mainDBSite.Language {
			return errors.New(w.Tr("ErrorLanguageDuplicate"))
		}
		// 先获取所有的子站点
		sites := w.GetMultiLangSites(w.ParentId)
		for _, site := range sites {
			if site.Language == req.Language && site.Id != req.Id {
				return errors.New(w.Tr("ErrorLanguageDuplicate"))
			}
		}
	}
	// 设置语言
	targetSite := GetWebsite(req.Id)
	if targetSite == nil || targetSite.DB == nil {
		return errors.New(w.Tr("SiteDoesNotExist"))
	}
	targetDbSite, err := GetDBWebsiteInfo(req.Id)
	if err != nil {
		return err
	}
	err = db.Model(&model.Website{}).Where("id = ?", req.Id).UpdateColumn("parent_id", req.ParentId).Error
	if err != nil {
		return err
	}
	// 如果原来的parentId和当前不一致，则修改sync_time
	if targetDbSite.ParentId != req.ParentId {
		// 修改sync_time
		db.Model(&model.Website{}).Where("id = ?", req.Id).UpdateColumn("sync_time", 0)
	}

	targetSite.System.Language = req.Language
	err = targetSite.SaveSettingValue(SystemSettingKey, targetSite.System)
	if err != nil {
		return err
	}
	// 加入到语言列表
	mainSite := GetWebsite(req.ParentId)
	mainSite.MultiLanguage.SubSites[targetSite.System.Language] = targetSite.Id

	return nil
}

// SyncMultiLangSiteContent 同步的内容有：modules categories tags archives
// 同步的时候，不同步进行翻译，如果启用了自动翻译，则添加到翻译的计划任务中
func (ms *MultiLangSyncStatus) SyncMultiLangSiteContent(req *request.PluginMultiLangSiteRequest) error {
	ms.Percent = 0
	defer func() {
		ms.FinishCount = ms.TotalCount
		ms.Finished = true
		time.AfterFunc(3*time.Second, func() {
			if ms.Finished {
				multiLangSyncStatus = nil
			}
		})
	}()

	targetSite := GetWebsite(req.Id)
	if targetSite == nil || targetSite.Initialed == false {
		ms.Message = ms.w.Tr("SiteNotInitialized")
		return errors.New(ms.w.Tr("SiteNotInitialized"))
	}
	targetDbSite, err := GetDBWebsiteInfo(req.Id)
	if err != nil {
		ms.Message = ms.w.Tr("SiteNotInitialized")
		return err
	}
	mainSite := GetWebsite(req.ParentId)
	if mainSite == nil || targetSite.Initialed == false {
		ms.Message = ms.w.Tr("SiteNotInitialized")
		return errors.New(ms.w.Tr("SiteNotInitialized"))
	}

	log.Println("start to sync content")

	var lastId uint = 0
	var startId uint = 0
	var limitSize = 5000
	// 如果没同步过，或使用强制同步，则同步所有，否则只同步新增的
	if targetDbSite.SyncTime == 0 || req.Focus {
		// 同步所有
		var total int64
		mainSite.DB.Model(&model.Module{}).Count(&total)
		ms.TotalCount += total
		mainSite.DB.Model(&model.NavType{}).Count(&total)
		ms.TotalCount += total
		mainSite.DB.Model(&model.Nav{}).Count(&total)
		ms.TotalCount += total
		mainSite.DB.Model(&model.Category{}).Count(&total)
		ms.TotalCount += total
		mainSite.DB.Model(&model.Tag{}).Count(&total)
		ms.TotalCount += total
		mainSite.DB.Model(&model.ArchiveData{}).Count(&total)
		ms.TotalCount += total
		mainSite.DB.Model(&model.Archive{}).Count(&total)
		ms.TotalCount += total
		mainSite.DB.Model(&model.TagData{}).Count(&total)
		ms.TotalCount += total
		mainSite.DB.Model(&model.ArchiveCategory{}).Count(&total)
		ms.TotalCount += total
		mainSite.DB.Model(&model.ArchiveFlag{}).Count(&total)
		ms.TotalCount += total
		mainSite.DB.Model(&model.ArchiveRelation{}).Count(&total)
		ms.TotalCount += total

		// 先同步模型
		var modules []model.Module
		mainSite.DB.Model(&model.Module{}).Order("id ASC").Find(&modules)
		for _, module := range modules {
			log.Println("sync module", module.Id)
			ms.FinishCount++
			ms.Percent = ms.FinishCount * 100 / ms.TotalCount
			ms.Message = ms.w.Tr("Syncing%s:%s", "module", module.Title)
			// 保存到目标站点
			targetSite.DB.Save(&module)
			// 自动翻译
			if mainSite.MultiLanguage.AutoTranslate {
				ms.FinishCount++
				ms.TotalCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Translating%s:%s", "Module", module.Title)
				transReq := AnqiAiRequest{
					Title:      module.Title,
					Language:   mainSite.System.Language,
					ToLanguage: targetSite.System.Language,
					Async:      false, // 同步返回结果
				}
				res, err := mainSite.AnqiTranslateString(&transReq)
				if err == nil {
					// 只处理成功的结果
					targetSite.DB.Model(&module).UpdateColumns(map[string]interface{}{
						"title":     res.Title,
						"seo_title": res.Content,
					})
				}
			}
		}
		// 同步导航
		var navTypes []model.NavType
		mainSite.DB.Model(&model.NavType{}).Order("id ASC").Find(&navTypes)
		for _, navType := range navTypes {
			log.Println("sync navtype", navType.Id)
			ms.FinishCount++
			ms.Percent = ms.FinishCount * 100 / ms.TotalCount
			ms.Message = ms.w.Tr("Syncing%s:%s", "Nav Type", navType.Title)
			targetSite.DB.Save(&navType)
		}
		var navs []model.Nav
		mainSite.DB.Model(&model.Nav{}).Order("id ASC").Find(&navs)
		for _, nav := range navs {
			log.Println("sync navs", nav.Id)
			ms.FinishCount++
			ms.Percent = ms.FinishCount * 100 / ms.TotalCount
			ms.Message = ms.w.Tr("Syncing%s:%s", "Nav", nav.Title)
			targetSite.DB.Save(&nav)
			// 自动翻译
			if mainSite.MultiLanguage.AutoTranslate {
				ms.FinishCount++
				ms.TotalCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Translating%s:%s", "Nav", nav.Title)
				transReq := AnqiAiRequest{
					Title:      nav.Title,
					Content:    nav.Description,
					Language:   mainSite.System.Language,
					ToLanguage: targetSite.System.Language,
					Async:      false, // 同步返回结果
				}
				res, err := mainSite.AnqiTranslateString(&transReq)
				if err == nil {
					// 只处理成功的结果
					targetSite.DB.Model(&nav).UpdateColumns(map[string]interface{}{
						"title":       res.Title,
						"description": res.Content,
					})
				}
			}
		}
		// 同步分类
		var categories []model.Category
		mainSite.DB.Model(&model.Category{}).Order("id ASC").Find(&categories)
		for _, category := range categories {
			log.Println("sync category", category.Id)
			ms.FinishCount++
			ms.Percent = ms.FinishCount * 100 / ms.TotalCount
			ms.Message = ms.w.Tr("Syncing%s:%s", "Category", category.Title)
			targetSite.DB.Save(&category)
			// 自动翻译
			if mainSite.MultiLanguage.AutoTranslate {
				ms.FinishCount++
				ms.TotalCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Translating%s:%s", "Category", category.Title)
				transReq := AnqiAiRequest{
					Title:      category.Title,
					Content:    category.Content,
					Language:   mainSite.System.Language,
					ToLanguage: targetSite.System.Language,
					Async:      false, // 同步返回结果
				}
				res, err := mainSite.AnqiTranslateString(&transReq)
				if err == nil {
					// 只处理成功的结果
					targetSite.DB.Model(&category).UpdateColumns(map[string]interface{}{
						"title":   res.Title,
						"content": res.Content,
					})
				}
				if len(category.Description) > 0 {
					transReq = AnqiAiRequest{
						Title:      "",
						Content:    category.Description,
						Language:   mainSite.System.Language,
						ToLanguage: targetSite.System.Language,
						Async:      false, // 同步返回结果
					}
					res, err = mainSite.AnqiTranslateString(&transReq)
					if err == nil {
						// 只处理成功的结果
						targetSite.DB.Model(&category).UpdateColumns(map[string]interface{}{
							"description": res.Content,
						})
					}
				}
			}
		}
		// 同步标签
		for {
			var tags []model.Tag
			mainSite.DB.Model(&model.Tag{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&tags)
			if len(tags) == 0 {
				break
			}
			startId = tags[len(tags)-1].Id
			for _, tag := range tags {
				log.Println("sync tag", tag.Id)
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Tag", tag.Title)
				targetSite.DB.Save(&tag)
				// 自动翻译
				if mainSite.MultiLanguage.AutoTranslate {
					ms.FinishCount++
					ms.TotalCount++
					ms.Percent = ms.FinishCount * 100 / ms.TotalCount
					ms.Message = ms.w.Tr("Translating%s:%s", "Tag", tag.Title)
					transReq := AnqiAiRequest{
						Title:      tag.Title,
						Content:    tag.Description,
						Language:   mainSite.System.Language,
						ToLanguage: targetSite.System.Language,
						Async:      false, // 同步返回结果
					}
					res, err := mainSite.AnqiTranslateString(&transReq)
					if err == nil {
						// 只处理成功的结果
						targetSite.DB.Model(&tag).UpdateColumns(map[string]interface{}{
							"title":       res.Title,
							"description": res.Content,
						})
					}
				}
			}
		}

		// 同步文章，以及文章的附表
		startId = lastId
		for {
			var archiveData []model.ArchiveData
			mainSite.DB.Model(&model.ArchiveData{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&archiveData)
			if len(archiveData) == 0 {
				break
			}
			startId = archiveData[len(archiveData)-1].Id
			for _, archive := range archiveData {
				log.Println("sync arcdata", archive.Id)
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Archive Data", strconv.Itoa(int(archive.Id)))
				targetSite.DB.Save(&archive)
			}
		}

		for _, module := range modules {
			if len(module.Fields) > 0 {
				module.Migrate(targetSite.DB, module.TableName, false)
				startId = lastId
				for {
					var extraData []map[string]interface{}
					mainSite.DB.Table(module.TableName).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Scan(&extraData)
					if len(extraData) == 0 {
						break
					}
					tmpId, _ := strconv.Atoi(fmt.Sprintf("%v", extraData[len(extraData)-1]["id"]))
					if tmpId == 0 {
						break
					}
					ms.TotalCount += int64(len(extraData))
					startId = uint(tmpId)
					for _, data := range extraData {
						log.Println("sync extra", data)
						ms.FinishCount++
						ms.Percent = ms.FinishCount * 100 / ms.TotalCount
						ms.Message = ms.w.Tr("Syncing%s:%s", "Archive Extra", tmpId)
						targetSite.DB.Table(module.TableName).Create(&data)
						// 不做翻译
					}
				}
			}
		}

		startId = lastId
		for {
			var archives []model.Archive
			mainSite.DB.Model(&model.Archive{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&archives)
			if len(archives) == 0 {
				break
			}
			startId = archives[len(archives)-1].Id
			for _, archive := range archives {
				log.Println("sync archive", archive.Id)
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Archive", archive.Title)
				targetSite.DB.Save(&archive)
				// 自动翻译
				if mainSite.MultiLanguage.AutoTranslate {
					// 文章的翻译，使用另一个接口
					// 读取 data
					archiveData, err := targetSite.GetArchiveDataById(archive.Id)
					if err != nil {
						continue
					}
					aiReq := &AnqiAiRequest{
						Title:      archive.Title,
						Content:    archiveData.Content,
						ArticleId:  archive.Id,
						Language:   mainSite.System.Language,
						ToLanguage: targetSite.System.Language,
						Async:      false, // 同步返回结果
					}
					result, err := mainSite.AnqiTranslateString(aiReq)
					if err != nil {
						continue
					}
					// 更新文档
					if result.Status == config.AiArticleStatusCompleted {
						archive.Title = result.Title
						archive.Description = library.ParseDescription(strings.ReplaceAll(library.StripTags(result.Content), "\n", " "))
						targetSite.DB.Save(archive)
						// 再保存内容
						archiveData.Content = result.Content
						targetSite.DB.Save(archiveData)
					}
					// 写入 plan
					_, _ = mainSite.SaveAiArticlePlan(result, result.UseSelf)
				}
			}
		}

		startId = lastId
		for {
			var tagData []model.TagData
			mainSite.DB.Model(&model.TagData{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&tagData)
			if len(tagData) == 0 {
				break
			}
			startId = tagData[len(tagData)-1].Id
			for _, tag := range tagData {
				log.Println("sync tagdata", tag.Id)
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Tag Data", strconv.Itoa(int(tag.Id)))
				targetSite.DB.Save(&tag)
			}
		}

		startId = lastId
		for {
			var archiveCategories []model.ArchiveCategory
			mainSite.DB.Model(&model.ArchiveCategory{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&archiveCategories)
			if len(archiveCategories) == 0 {
				break
			}
			startId = archiveCategories[len(archiveCategories)-1].Id
			for _, archiveCategory := range archiveCategories {
				log.Println("sync arccate", archiveCategory.Id)
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Archive Category", strconv.Itoa(int(archiveCategory.Id)))
				targetSite.DB.Save(&archiveCategory)
			}
		}

		startId = lastId
		for {
			var archiveFlags []model.ArchiveFlag
			mainSite.DB.Model(&model.ArchiveFlag{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&archiveFlags)
			if len(archiveFlags) == 0 {
				break
			}
			startId = archiveFlags[len(archiveFlags)-1].Id
			for _, archiveFlag := range archiveFlags {
				log.Println("sync arcflag", archiveFlag.Id)
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Archive Flag", strconv.Itoa(int(archiveFlag.Id)))
				targetSite.DB.Save(&archiveFlag)
			}
		}
		startId = lastId
		for {
			var archiveRelations []model.ArchiveRelation
			mainSite.DB.Model(&model.ArchiveRelation{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&archiveRelations)
			if len(archiveRelations) == 0 {
				break
			}
			startId = archiveRelations[len(archiveRelations)-1].Id
			for _, archiveRelation := range archiveRelations {
				log.Println("sync arcrelation", archiveRelation.Id)
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Archive Relation", strconv.Itoa(int(archiveRelation.Id)))
				targetSite.DB.Save(&archiveRelation)
			}
		}
	} else {
		// 只同步新增的，如果是同步新增的，则只查找缺少的ID。
		// 新增的只处理 modules,categories,archives
		// modules
		var modules []model.Module
		mainSite.DB.Model(&model.Module{}).Order("id ASC").Find(&modules)
		ms.TotalCount += int64(len(modules))
		for _, module := range modules {
			ms.FinishCount++
			ms.Percent = ms.FinishCount * 100 / ms.TotalCount
			ms.Message = ms.w.Tr("Syncing%s:%s", "Module", module.Title)
			// 检查目标站点是否已经存在相同的ID
			targetSite.DB.Where("id = ?", module.Id).FirstOrCreate(&module)
			// 自动翻译
			if mainSite.MultiLanguage.AutoTranslate {
				ms.FinishCount++
				ms.TotalCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Translating%s:%s", "Module", module.Title)
				transReq := AnqiAiRequest{
					Title:      module.Title,
					Language:   mainSite.System.Language,
					ToLanguage: targetSite.System.Language,
					Async:      false, // 同步返回结果
				}
				res, err := mainSite.AnqiTranslateString(&transReq)
				if err == nil {
					// 只处理成功的结果
					targetSite.DB.Model(&module).UpdateColumns(map[string]interface{}{
						"title":     res.Title,
						"seo_title": res.Content,
					})
				}
			}
		}
		// categories
		targetSite.DB.Model(&model.Category{}).Order("id DESC").Pluck("id", &startId)
		var categories []model.Category
		mainSite.DB.Model(&model.Category{}).Where("id > ?", startId).Order("id ASC").Find(&categories)
		ms.TotalCount += int64(len(categories))
		for _, category := range categories {
			ms.FinishCount++
			ms.Percent = ms.FinishCount * 100 / ms.TotalCount
			ms.Message = ms.w.Tr("Syncing%s:%s", "Category", category.Title)
			targetSite.DB.Where("id = ?", category.Id).Save(&category)
			// 自动翻译
			if mainSite.MultiLanguage.AutoTranslate {
				ms.FinishCount++
				ms.TotalCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Translating%s:%s", "Category", category.Title)
				transReq := AnqiAiRequest{
					Title:      category.Title,
					Content:    category.Content,
					Language:   mainSite.System.Language,
					ToLanguage: targetSite.System.Language,
					Async:      false, // 同步返回结果
				}
				res, err := mainSite.AnqiTranslateString(&transReq)
				if err == nil {
					// 只处理成功的结果
					targetSite.DB.Model(&category).UpdateColumns(map[string]interface{}{
						"title":   res.Title,
						"content": res.Content,
					})
				}
				if len(category.Description) > 0 {
					transReq = AnqiAiRequest{
						Title:      "",
						Content:    category.Description,
						Language:   mainSite.System.Language,
						ToLanguage: targetSite.System.Language,
						Async:      false, // 同步返回结果
					}
					res, err = mainSite.AnqiTranslateString(&transReq)
					if err == nil {
						// 只处理成功的结果
						targetSite.DB.Model(&category).UpdateColumns(map[string]interface{}{
							"description": res.Content,
						})
					}
				}
			}
		}

		targetSite.DB.Model(&model.Archive{}).Order("id DESC").Pluck("id", &lastId)
		// archiveData
		startId = lastId
		for {
			var archiveData []model.ArchiveData
			mainSite.DB.Model(&model.ArchiveData{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&archiveData)
			if len(archiveData) == 0 {
				break
			}
			ms.TotalCount += int64(len(archiveData))
			startId = archiveData[len(archiveData)-1].Id
			for _, archive := range archiveData {
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Archive Data", strconv.Itoa(int(archive.Id)))
				targetSite.DB.Save(&archive)
			}
		}

		// extraData
		for _, module := range modules {
			if len(module.Fields) > 0 {
				module.Migrate(targetSite.DB, module.TableName, false)
				startId = lastId
				for {
					var extraData []map[string]interface{}
					mainSite.DB.Table(module.TableName).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Scan(&extraData)
					if len(extraData) == 0 {
						break
					}
					tmpId, _ := strconv.Atoi(fmt.Sprintf("%v", extraData[len(extraData)-1]["id"]))
					if tmpId == 0 {
						break
					}
					ms.TotalCount += int64(len(extraData))
					startId = uint(tmpId)
					for _, data := range extraData {
						ms.FinishCount++
						ms.Percent = ms.FinishCount * 100 / ms.TotalCount
						ms.Message = ms.w.Tr("Syncing%s:%s", "Archive Extra", tmpId)
						targetSite.DB.Table(module.TableName).Create(&data)
					}
				}
			}
		}

		// archives
		startId = lastId
		for {
			var archives []model.Archive
			mainSite.DB.Model(&model.Archive{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&archives)
			if len(archives) == 0 {
				break
			}
			ms.TotalCount += int64(len(archives))
			startId = archives[len(archives)-1].Id
			for _, archive := range archives {
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Archive", archive.Title)
				targetSite.DB.Where("id = ?", archive.Id).Save(&archive)
				// 自动翻译
				if mainSite.MultiLanguage.AutoTranslate {
					// 文章的翻译，使用另一个接口
					// 读取 data
					archiveData, err := targetSite.GetArchiveDataById(archive.Id)
					if err != nil {
						continue
					}
					aiReq := &AnqiAiRequest{
						Title:      archive.Title,
						Content:    archiveData.Content,
						ArticleId:  archive.Id,
						Language:   mainSite.System.Language,
						ToLanguage: targetSite.System.Language,
						Async:      false, // 同步返回结果
					}
					result, err := mainSite.AnqiTranslateString(aiReq)
					if err != nil {
						continue
					}
					// 更新文档
					if result.Status == config.AiArticleStatusCompleted {
						archive.Title = result.Title
						archive.Description = library.ParseDescription(strings.ReplaceAll(library.StripTags(result.Content), "\n", " "))
						targetSite.DB.Save(archive)
						// 再保存内容
						archiveData.Content = result.Content
						targetSite.DB.Save(archiveData)
					}
					// 写入 plan
					_, _ = mainSite.SaveAiArticlePlan(result, result.UseSelf)
				}
			}
		}

		targetSite.DB.Model(&model.TagData{}).Order("id DESC").Pluck("id", &lastId)
		startId = lastId
		for {
			var tagData []model.TagData
			mainSite.DB.Model(&model.TagData{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&tagData)
			if len(tagData) == 0 {
				break
			}
			ms.TotalCount += int64(len(tagData))
			startId = tagData[len(tagData)-1].Id
			var tagIds = make([]uint, 0, len(tagData))
			for _, tag := range tagData {
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Tag Data", strconv.Itoa(int(tag.Id)))
				targetSite.DB.Save(&tag)
				existId := false
				for _, id := range tagIds {
					if id == tag.TagId {
						existId = true
						break
					}
				}
				if !existId {
					tagIds = append(tagIds, tag.Id)
				}
			}
			var tags []model.Tag
			mainSite.DB.Model(&model.Tag{}).Where("id IN(?)", tagIds).Order("id asc").Find(&tags)
			ms.TotalCount += int64(len(tags))
			for _, tag := range tags {
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Tag", tag.Title)
				targetSite.DB.Save(&tag)
				// 自动翻译
				if mainSite.MultiLanguage.AutoTranslate {
					ms.FinishCount++
					ms.TotalCount++
					ms.Percent = ms.FinishCount * 100 / ms.TotalCount
					ms.Message = ms.w.Tr("Translating%s:%s", "Category", tag.Title)
					transReq := AnqiAiRequest{
						Title:      tag.Title,
						Content:    tag.Description,
						Language:   mainSite.System.Language,
						ToLanguage: targetSite.System.Language,
						Async:      false, // 同步返回结果
					}
					res, err := mainSite.AnqiTranslateString(&transReq)
					if err == nil {
						// 只处理成功的结果
						targetSite.DB.Model(&tag).UpdateColumns(map[string]interface{}{
							"title":       res.Title,
							"description": res.Content,
						})
					}
				}
			}
		}

		targetSite.DB.Model(&model.ArchiveCategory{}).Order("id DESC").Pluck("id", &lastId)
		startId = lastId
		for {
			var archiveCategories []model.ArchiveCategory
			mainSite.DB.Model(&model.ArchiveCategory{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&archiveCategories)
			if len(archiveCategories) == 0 {
				break
			}
			ms.TotalCount += int64(len(archiveCategories))
			startId = archiveCategories[len(archiveCategories)-1].Id
			for _, archiveCategory := range archiveCategories {
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Tag Data", strconv.Itoa(int(archiveCategory.Id)))
				targetSite.DB.Save(&archiveCategory)
			}
		}

		targetSite.DB.Model(&model.ArchiveFlag{}).Order("id DESC").Pluck("id", &lastId)
		startId = lastId
		for {
			var archiveFlags []model.ArchiveFlag
			mainSite.DB.Model(&model.ArchiveFlag{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&archiveFlags)
			if len(archiveFlags) == 0 {
				break
			}
			ms.TotalCount += int64(len(archiveFlags))
			startId = archiveFlags[len(archiveFlags)-1].Id
			for _, archiveFlag := range archiveFlags {
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Archive Flag", strconv.Itoa(int(archiveFlag.Id)))
				targetSite.DB.Save(&archiveFlag)
			}
		}

		targetSite.DB.Model(&model.ArchiveRelation{}).Order("id DESC").Pluck("id", &lastId)
		startId = lastId
		for {
			var archiveRelations []model.ArchiveRelation
			mainSite.DB.Model(&model.ArchiveRelation{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&archiveRelations)
			if len(archiveRelations) == 0 {
				break
			}
			ms.TotalCount += int64(len(archiveRelations))
			startId = archiveRelations[len(archiveRelations)-1].Id
			for _, archiveRelation := range archiveRelations {
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Archive Relation", strconv.Itoa(int(archiveRelation.Id)))
				targetSite.DB.Save(&archiveRelation)
			}
		}
	}

	// 记录同步时间
	GetDefaultDB().Model(targetDbSite).Update("sync_time", time.Now().Unix())
	// 同步完成后，还需要进行缓存更新
	targetSite.DeleteCache()
	log.Println("finished synced content")

	return nil
}
