package provider

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
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

func (w *Website) GetMultiLangSites(mainId uint, all bool) []config.MultiLangSite {
	// 用于读取真实的主站点ID
	if mainId == 0 {
		mainId = w.Id
	}

	mainSite := GetWebsite(mainId)
	if mainSite == nil || !mainSite.Initialed || mainSite.MultiLanguage.Open == false {
		return nil
	}

	var multiLangSites = make([]config.MultiLangSite, 0, 100)
	// 先添加主站点
	var link string
	tmpSite := config.MultiLangSite{
		Id:           mainSite.Id,
		RootPath:     mainSite.RootPath,
		Name:         mainSite.Name,
		Status:       mainSite.Initialed,
		ParentId:     0,
		LanguageIcon: mainSite.LanguageIcon,
		IsCurrent:    w.Id == mainSite.Id,
	}

	// more
	link = mainSite.GetUrl("", nil, 0)
	// 如果是同链接，则是一个跳转链接
	if mainSite.MultiLanguage.Type == config.MultiLangTypeSame {
		if strings.Contains(link, "?") {
			link = link + "&lang=" + mainSite.System.Language
		} else {
			link += "?lang=" + mainSite.System.Language
		}
	}
	tmpSite.Link = link
	tmpSite.LanguageEmoji = library.GetLanguageIcon(mainSite.System.Language)
	tmpSite.LanguageName = library.GetLanguageName(mainSite.System.Language)
	tmpSite.Language = mainSite.System.Language
	tmpSite.BaseUrl = mainSite.System.BaseUrl
	// end

	multiLangSites = append(multiLangSites, tmpSite)
	// 继续添加子站点
	if mainSite.MultiLanguage.SiteType == config.MultiLangSiteTypeMulti {
		allSites := GetWebsites()
		for i := range allSites {
			if allSites[i].ParentId == mainId {
				if allSites[i].Initialed != true && !all {
					// 如果不是获取全部，则跳过那些不正确的站点
					continue
				}
				subSite := config.MultiLangSite{
					Id:           allSites[i].Id,
					RootPath:     allSites[i].RootPath,
					Name:         allSites[i].Name,
					Status:       allSites[i].Initialed,
					ParentId:     allSites[i].ParentId,
					LanguageIcon: allSites[i].LanguageIcon,
					IsCurrent:    w.Id == allSites[i].Id,
					ErrorMsg:     allSites[i].ErrorMsg,
				}
				if allSites[i].Initialed {
					link = allSites[i].GetUrl("", nil, 0)
					// 如果是同链接，则是一个跳转链接
					if mainSite.MultiLanguage.Type == config.MultiLangTypeSame {
						if strings.Contains(link, "?") {
							link = link + "&lang=" + allSites[i].System.Language
						} else {
							link += "?lang=" + allSites[i].System.Language
						}
					}
					subSite.Link = link
					subSite.LanguageEmoji = library.GetLanguageIcon(allSites[i].System.Language)
					subSite.LanguageName = library.GetLanguageName(allSites[i].System.Language)
					subSite.Language = allSites[i].System.Language
					subSite.BaseUrl = allSites[i].System.BaseUrl
				}
				multiLangSites = append(multiLangSites, subSite)
			}
		}
	} else {
		// single 模式
		for i := range mainSite.MultiLanguage.SubSites {
			subSite := config.MultiLangSite{
				Id:           mainSite.MultiLanguage.SubSites[i].Id,
				RootPath:     mainSite.RootPath,
				Name:         mainSite.MultiLanguage.SubSites[i].LanguageName,
				Status:       true,
				ParentId:     mainId,
				LanguageIcon: mainSite.MultiLanguage.SubSites[i].LanguageIcon,
				IsCurrent:    w.Id == mainSite.MultiLanguage.SubSites[i].Id,
				Language:     mainSite.MultiLanguage.SubSites[i].Language,
				BaseUrl:      mainSite.MultiLanguage.SubSites[i].BaseUrl,
			}
			var link string
			if mainSite.MultiLanguage.Type == config.MultiLangTypeDomain {
				link = subSite.BaseUrl + "/"
			} else if mainSite.MultiLanguage.Type == config.MultiLangTypeDirectory {
				link = mainSite.System.BaseUrl + "/" + subSite.Language + "/"
			} else if mainSite.MultiLanguage.Type == config.MultiLangTypeSame {
				link += mainSite.GetUrl("", nil, 0) + "?lang=" + subSite.Language
			}
			subSite.Link = link
			subSite.LanguageEmoji = library.GetLanguageIcon(subSite.Language)
			subSite.LanguageName = library.GetLanguageName(subSite.Language)
			subSite.Name = subSite.LanguageName

			multiLangSites = append(multiLangSites, subSite)
		}
	}

	return multiLangSites
}

func (w *Website) GetMultiLangValidSites(mainId uint) []config.MultiLangSite {
	var sites []config.MultiLangSite
	values := websites.Values()
	// 排除所有的主站点
	// 排除所有的主站点以及其他主站点的子站点
	var parentMap = map[uint]struct{}{}
	for i := range values {
		if values[i].ParentId > 0 {
			parentMap[values[i].ParentId] = struct{}{}
		}
	}
	for i := range values {
		// 排除其他主站点的子站点
		if values[i].ParentId > 0 && values[i].ParentId != mainId {
			continue
		}
		// 排除主站点
		if _, ok := parentMap[values[i].Id]; ok {
			continue
		}
		// 排除不可用的站点
		if !values[i].Initialed {
			continue
		}
		// 剩下是可用的
		sites = append(sites, config.MultiLangSite{
			Id:       values[i].Id,
			ParentId: values[i].ParentId,
			RootPath: values[i].RootPath,
			Name:     values[i].Name,
			Status:   values[i].Initialed,
			Language: values[i].System.Language,
			BaseUrl:  values[i].System.BaseUrl,
		})
	}

	return sites
}

func (w *Website) RemoveMultiLangSite(siteId uint, lang string) error {
	if siteId > 0 {
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
			mainSite.MultiLanguage.RemoveSite(siteId, lang)

			targetSite.ParentId = 0
		}
	} else {
		// 移除语言
		w.MultiLanguage.RemoveSite(siteId, lang)
		// 更新setting
		_ = w.SaveSettingValue(MultiLangSettingKey, w.MultiLanguage)
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
		// 只能修改language_icon
		err = db.Model(&model.Website{}).Where("id = ?", req.Id).UpdateColumns(map[string]interface{}{
			"language_icon": req.LanguageIcon,
		}).Error
		if err == nil {
			w.LanguageIcon = req.LanguageIcon
		}
		return err
		//return errors.New(w.Tr("ErrorSameSite"))
	}
	if req.Language != "other" {
		if req.Language == mainDBSite.Language {
			return errors.New(w.Tr("ErrorLanguageDuplicate"))
		}
		// 先获取所有的子站点
		sites := w.GetMultiLangSites(w.ParentId, true)
		for _, site := range sites {
			if site.Language == req.Language && site.Id != req.Id {
				return errors.New(w.Tr("ErrorLanguageDuplicate"))
			}
		}
	}
	var targetSite *Website
	if w.MultiLanguage.SiteType == config.MultiLangSiteTypeMulti {
		// 设置语言
		targetSite = GetWebsite(req.Id)
		if targetSite == nil || targetSite.DB == nil {
			return errors.New(w.Tr("SiteDoesNotExist"))
		}
		targetDbSite, err := GetDBWebsiteInfo(req.Id)
		if err != nil {
			return err
		}
		err = db.Model(&model.Website{}).Where("id = ?", req.Id).UpdateColumns(map[string]interface{}{
			"parent_id":     req.ParentId,
			"language_icon": req.LanguageIcon,
		}).Error
		if err != nil {
			return err
		}
		// 如果原来的parentId和当前不一致，则修改sync_time
		if targetDbSite.ParentId != req.ParentId {
			// 修改sync_time
			db.Model(&model.Website{}).Where("id = ?", req.Id).UpdateColumn("sync_time", 0)
		}
		var baseUrl string
		if w.MultiLanguage.Type == config.MultiLangTypeDirectory {
			baseUrl = w.System.BaseUrl + "/" + req.Language
		} else if w.MultiLanguage.Type == config.MultiLangTypeSame {
			baseUrl = w.System.BaseUrl
		}
		targetSite.PluginStorage.StorageUrl = baseUrl

		targetSite.ParentId = req.ParentId
		targetSite.LanguageIcon = req.LanguageIcon
		targetSite.System.Language = req.Language
		err = targetSite.SaveSettingValue(SystemSettingKey, targetSite.System)
		if err != nil {
			return err
		}
	} else {
		targetSite = &Website{
			Id:           req.Id,
			ParentId:     req.ParentId,
			LanguageIcon: req.LanguageIcon,
			System: &config.SystemConfig{
				Language: req.Language,
				BaseUrl:  req.BaseUrl,
			},
		}
	}
	// 加入到语言列表
	mainSite := GetWebsite(req.ParentId)
	langSite := config.MultiLangSite{
		Id:           targetSite.Id,
		Language:     targetSite.System.Language,
		BaseUrl:      targetSite.System.BaseUrl,
		LanguageIcon: targetSite.LanguageIcon,
	}
	mainSite.MultiLanguage.SaveSite(langSite)
	// 更新setting
	_ = w.SaveSettingValue(MultiLangSettingKey, mainSite.MultiLanguage)

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

	var lastId int64 = 0
	var startId int64 = 0
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
		mainSite.DB.Model(&model.Attachment{}).Count(&total)
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
		// 同步图片资源
		var attachCategories []model.AttachmentCategory
		mainSite.DB.Model(&model.AttachmentCategory{}).Order("id ASC").Find(&attachCategories)
		for _, attachCat := range attachCategories {
			log.Println("sync navtype", attachCat.Id)
			ms.FinishCount++
			ms.Percent = ms.FinishCount * 100 / ms.TotalCount
			ms.Message = ms.w.Tr("Syncing%s:%s", "Attachment Category", attachCat.Title)
			targetSite.DB.Save(&attachCat)
		}
		startId = lastId
		for {
			var attachments []model.Attachment
			mainSite.DB.Model(&model.Attachment{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&attachments)
			if len(attachments) == 0 {
				break
			}
			startId = int64(attachments[len(attachments)-1].Id)
			for _, attachment := range attachments {
				log.Println("sync attachment", attachment.Id)
				ms.FinishCount++
				ms.Percent = ms.FinishCount * 100 / ms.TotalCount
				ms.Message = ms.w.Tr("Syncing%s:%s", "Attachment", attachment.FileName)
				targetSite.DB.Save(&attachment)
				// 还需要复制图片
				if attachment.FileLocation != "" {
					// 复制图片，只支持本地图片复制
					logoPath := ms.w.PublicPath + strings.TrimPrefix(attachment.FileLocation, "/")
					logoBuf, err := os.ReadFile(logoPath)
					if err == nil {
						_, err = ms.w.Storage.UploadFile(attachment.FileLocation, logoBuf)
					}
					// 复制 thumb
					paths, fileName := filepath.Split(attachment.FileLocation)
					thumbLocation := paths + "thumb_" + fileName
					thumbPath := ms.w.PublicPath + strings.TrimPrefix(thumbLocation, "/")
					thumbBuf, err := os.ReadFile(thumbPath)
					if err == nil {
						_, err = ms.w.Storage.UploadFile(thumbLocation, thumbBuf)
					}
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
		startId = lastId
		for {
			var tags []model.Tag
			mainSite.DB.Model(&model.Tag{}).Where("id > ?", startId).Limit(limitSize).Order("id ASC").Find(&tags)
			if len(tags) == 0 {
				break
			}
			startId = int64(tags[len(tags)-1].Id)
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
					startId = int64(tmpId)
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
			startId = int64(tagData[len(tagData)-1].Id)
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
					startId = int64(tmpId)
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
			startId = int64(tagData[len(tagData)-1].Id)
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

var uriLangLocks = sync.Map{}

// GetOrSetMultiLangCache siteType = single 模式下，翻译缓存的页面内容
// 缓存路径规则： /cache/multiLang/{lang}/{uri_hash}
// 每个翻译页面都是原子操作，单次只允许一个线程进行翻译
// 如果获取缓存时没有缓存，则进行翻译，并写入缓存，再返回
// 如果翻译失败，则回退到原始内容
// 存储的第一行，为文件的uri。第二行开始为内容，因为文件名不是真实的uri
func (w *Website) GetOrSetMultiLangCache(uri string, lang string) (string, error) {
	uriHash := library.Md5(uri)
	cachePath := w.CachePath + "multiLang/" + lang + "/" + uriHash
	lock, _ := uriLangLocks.LoadOrStore(uriHash, &sync.Mutex{})
	mutex := lock.(*sync.Mutex)
	// 加锁
	mutex.Lock()
	defer mutex.Unlock()
	defer uriLangLocks.Delete(uriHash)

	// 先检查缓存文件是否存在，如果存在，直接返回
	if _, err := os.Stat(cachePath); err == nil {
		// 读取缓存文件
		buf, err := os.ReadFile(cachePath)
		if err == nil {
			// 删除第一行的内容
			// 找到第一个\n的位置
			pos := bytes.Index(buf, []byte("\n"))
			return string(buf[pos+1:]), nil
		}
	}
	// 如果不存在，则先获取原始内容，并进行翻译
	buf, err := w.GetHtmlDataByLocal(uri, false)
	if err != nil {
		return "", err
	}
	req := &AnqiTranslateHtmlRequest{
		Uri:         uri,
		Html:        string(buf),
		Language:    w.System.Language,
		ToLanguage:  lang,
		IgnoreClass: []string{"languages"},
		IgnoreId:    []string{"languages"},
	}
	result, err := w.AnqiTranslateHtml(req)
	if err != nil {
		// 如果翻译失败，则回退到原始内容
		log.Println("translate html failed:", err)
		return string(buf), err
	}
	// 翻译完毕，修改lang
	re, _ := regexp.Compile(`(?i)<html.*?>`)
	result = re.ReplaceAllString(result, fmt.Sprintf(`<html lang="%s">`, req.ToLanguage))
	// 替换URL
	langSite := w.MultiLanguage.GetSite(lang)
	if langSite != nil {
		// rel="alternate" 和 class="languages" 部分不替换,为了防止被替换，先把他们替换成其它
		var replacedMap = map[string]string{}
		idxNum := 0
		re0, _ := regexp.Compile(`(?i)<link\s+[^>]*?\bhref="[^"]+"[^>]*>`)
		result = re0.ReplaceAllStringFunc(result, func(s string) string {
			// 如果是 rel="alternate" 和 class="languages" 部分，则跳过
			if strings.Contains(s, "rel=\"alternate\"") {
				idxNum++
				numText := fmt.Sprintf("$(num%d)", idxNum)
				replacedMap[numText] = s
				return numText
			}
			return s
		})
		// 替换 class="languages"
		locator := library.NewDivLocator("div", "languages")
		langCode := locator.FindDiv(result)
		if langCode != "" {
			idxNum++
			numText := fmt.Sprintf("$(num%d)", idxNum)
			replacedMap[numText] = langCode
			result = strings.Replace(result, langCode, numText, 1)
		}
		if w.MultiLanguage.Type == config.MultiLangTypeDomain {
			// 替换域名
			// rel="alternate" 和 class="languages" 部分不替换
			result = strings.ReplaceAll(result, w.System.BaseUrl, langSite.BaseUrl)
		} else if w.MultiLanguage.Type == config.MultiLangTypeDirectory {
			// 替换目录
			// rel="alternate" 和 class="languages" 部分不替换
			// 查找所有的链接
			re2, _ := regexp.Compile(w.System.BaseUrl + "[^\"]{1,10}")
			result = re2.ReplaceAllStringFunc(result, func(s string) string {
				if strings.HasPrefix(s, w.System.BaseUrl+"/"+w.System.Language) {
					s = strings.Replace(s, w.System.BaseUrl+"/"+w.System.Language, w.System.BaseUrl+"/"+langSite.Language, 1)
				} else {
					s = strings.ReplaceAll(s, w.System.BaseUrl, w.System.BaseUrl+"/"+langSite.Language)
				}
				return s
			})
		}
		// 最后替换回来
		for k, v := range replacedMap {
			result = strings.Replace(result, k, v, 1)
		}
	}

	// 对内容进行缓存
	// 文件夹可能不存在，需要先判断创建
	err = os.MkdirAll(filepath.Dir(cachePath), 0755)
	if err != nil {
		log.Println("create html cache dir failed:", err)
		return result, err
	}
	// 写入缓存文件
	// 第一行是uri，第二行开始是内容
	err = os.WriteFile(cachePath, []byte(uri+"\n"+result), 0644)
	if err != nil {
		log.Println("write html cache file failed:", err)
	}

	return result, nil
}

func (w *Website) DeleteMultiLangCache(uris []string) {
	// 获取所有的语言
	if w.MultiLanguage.Open && w.MultiLanguage.SiteType == config.MultiLangSiteTypeSingle {
		// 这种情况下才需要处理
		for i := range w.MultiLanguage.SubSites {
			lang := w.MultiLanguage.SubSites[i].Language
			for _, uri := range uris {
				uriHash := library.Md5(uri)
				cachePath := w.CachePath + "multiLang/" + lang + "/" + uriHash
				// 删除缓存文件
				_ = os.Remove(cachePath)
			}
		}
	}
}

func (w *Website) DeleteMultiLangCacheAll() {
	// 获取所有的语言
	if w.MultiLanguage.Open && w.MultiLanguage.SiteType == config.MultiLangSiteTypeSingle {
		// 这种情况下才需要处理
		for i := range w.MultiLanguage.SubSites {
			lang := w.MultiLanguage.SubSites[i].Language
			cachePath := w.CachePath + "multiLang/" + lang
			// 删除缓存文件
			_ = os.RemoveAll(cachePath)
		}
	}
}

func (w *Website) GetTranslateHtmlLogs(page, pageSize int) ([]*model.TranslateHtmlLog, int64) {
	var total int64
	var logs []*model.TranslateHtmlLog

	tx := w.DB.Model(&model.TranslateHtmlLog{})
	offset := 0
	if page > 0 {
		offset = (page - 1) * pageSize
	}
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&logs)

	return logs, total
}

type TranslateHtmlCacheFile struct {
	Uri     string `json:"uri"`
	Lang    string `json:"lang"`
	LastMod int64  `json:"last_mod"`
	Html    string `json:"html"`
}

// GetTranslateHtmlCaches 获取所有已缓存的翻译文件
func (w *Website) GetTranslateHtmlCaches(lang string, page, pageSize int) ([]*TranslateHtmlCacheFile, int64) {
	cachePath := w.CachePath + "multiLang/"
	// 读取已缓存的语言
	var caches []*TranslateHtmlCacheFile
	var total int64

	// 获取所有语言目录
	langDirs, err := os.ReadDir(cachePath)
	if err != nil {
		return caches, total
	}
	// 计算总文件数
	for _, langDir := range langDirs {
		if !langDir.IsDir() {
			continue
		}
		if lang != "" && langDir.Name() != lang {
			// 过滤特定语言
			continue
		}
		langPath := filepath.Join(cachePath, langDir.Name())
		files, err := os.ReadDir(langPath)
		if err != nil {
			continue
		}
		total += int64(len(files))
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= int(total) {
		return caches, total
	}
	if end > int(total) {
		end = int(total)
	}

	// 遍历文件直到找到需要的范围
	var current int
outer:
	for _, langDir := range langDirs {
		if !langDir.IsDir() {
			continue
		}
		langName := langDir.Name()
		if lang != "" && langName != lang {
			// 过滤特定语言
			continue
		}
		langPath := filepath.Join(cachePath, langName)

		files, err := os.ReadDir(langPath)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			// 如果当前文件不在需要的范围内，跳过
			if current < start {
				current++
				continue
			}
			if current >= end {
				break outer
			}

			filePath := filepath.Join(langPath, file.Name())
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				current++
				continue
			}

			// 读取文件第一行
			file, err := os.Open(filePath)
			if err != nil {
				current++
				continue
			}
			scanner := bufio.NewScanner(file)
			var firstLine string
			if scanner.Scan() {
				firstLine = scanner.Text()
			}
			file.Close()

			cache := &TranslateHtmlCacheFile{
				Uri:     firstLine,
				Lang:    langName,
				LastMod: fileInfo.ModTime().Unix(),
				Html:    "",
			}
			caches = append(caches, cache)
			current++
		}
	}

	return caches, total
}
