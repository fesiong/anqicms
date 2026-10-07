package manageController

import (
	"os"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/ai/eino"
	"kandaoni.com/anqicms/pkg/mcp/intent"
	"kandaoni.com/anqicms/provider"
)

// SettingSystem 获取站点系统配置，包括站点 Logo、Favicon 及可用的语言列表。
func SettingSystem(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	system := currentSite.System
	if system.SiteLogo != "" && !strings.HasPrefix(system.SiteLogo, "http") && !strings.HasPrefix(system.SiteLogo, "//") {
		system.SiteLogo = currentSite.PluginStorage.StorageUrl + system.SiteLogo
	}

	// 检测Favicon
	system.Favicon = ""
	_, err := os.Stat(currentSite.PublicPath + "favicon.ico")
	if err == nil {
		system.Favicon = currentSite.System.BaseUrl + "/favicon.ico"
	}

	// 读取language列表
	var languages []string
	readerInfos, err := os.ReadDir(config.ExecPath + "/locales")
	if err == nil {
		for _, info := range readerInfos {
			if info.IsDir() {
				languages = append(languages, info.Name())
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"system":    system,
			"languages": languages,
		},
	})
}

// SettingSystemForm 保存站点系统配置，涉及域名变更时同步相关配置并重载模板。
func SettingSystemForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.SystemConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	req.AdminUrl = strings.TrimSpace(req.AdminUrl)
	if req.AdminUrl != "" && !strings.HasPrefix(req.AdminUrl, "http") {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheCorrectBackendDomainName"),
		})
		return
	}

	req.SiteLogo = strings.TrimPrefix(req.SiteLogo, currentSite.PluginStorage.StorageUrl)

	changed := false
	if currentSite.System.AdminUrl != req.AdminUrl {
		changed = true
	}
	if req.ExtraFields != nil {
		for i := range req.ExtraFields {
			req.ExtraFields[i].Name = library.Case2Camel(req.ExtraFields[i].Name)
		}
	}
	req.BaseUrl = strings.TrimRight(req.BaseUrl, "/")
	if req.FrontUrl != "" {
		req.FrontUrl = strings.TrimRight(req.FrontUrl, "/")
	}
	currentSite.System.SiteName = req.SiteName
	currentSite.System.SiteLogo = req.SiteLogo
	currentSite.System.SiteIcp = req.SiteIcp
	currentSite.System.SiteCopyright = req.SiteCopyright
	currentSite.System.AdminUrl = req.AdminUrl
	currentSite.System.SiteClose = req.SiteClose
	currentSite.System.SiteCloseTips = req.SiteCloseTips
	currentSite.System.BanSpider = req.BanSpider
	if currentSite.System.BaseUrl != req.BaseUrl {
		// 如果本来storageUrl = baseUrl
		if currentSite.PluginStorage.StorageUrl == currentSite.System.BaseUrl {
			currentSite.PluginStorage.StorageUrl = req.BaseUrl
			currentSite.SaveSettingValue(provider.StorageSettingKey, currentSite.PluginStorage)
		}
		if currentSite.PluginJsonLd != nil && currentSite.PluginJsonLd.OrganizationUrl == currentSite.System.BaseUrl {
			currentSite.PluginJsonLd.OrganizationUrl = req.BaseUrl
			currentSite.SaveSettingValue(provider.JsonLdSettingKey, currentSite.PluginJsonLd)
		}
	}
	currentSite.System.BaseUrl = req.BaseUrl
	currentSite.System.FrontUrl = req.FrontUrl
	currentSite.System.MobileUrl = req.MobileUrl
	currentSite.System.Language = req.Language
	currentSite.System.ExtraFields = req.ExtraFields

	err := currentSite.SaveSettingValue(provider.SystemSettingKey, currentSite.System)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if currentSite.MultiLanguage != nil {
		currentSite.MultiLanguage.DefaultLanguage = currentSite.System.Language
	}

	accounts := currentSite.GetPaymentAccounts(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("pay_way = ? and status = 1", config.PayWayPaypal)
	})
	if len(accounts) > 0 {
		for _, account := range accounts {
			currentSite.UpdatePaypalWebhook(account)
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSystemConfiguration"))

	// 如果切换了模板，则需要重启
	if changed {
		config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}
		time.Sleep(1 * time.Second)
	}
	currentSite.RemoveHtmlCache()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// SettingContent 获取内容设置，包括默认缩略图、缩略图、编辑器等配置。
func SettingContent(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	system := currentSite.Content

	for i := range system.DefaultThumbs {
		if !strings.HasPrefix(system.DefaultThumbs[i], "http") && !strings.HasPrefix(system.DefaultThumbs[i], "//") {
			system.DefaultThumbs[i] = currentSite.PluginStorage.StorageUrl + system.DefaultThumbs[i]
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

// SettingContentForm 保存内容设置，包括默认缩略图、缩略图、编辑器等配置。
func SettingContentForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.ContentConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	needUpgrade := false
	// 如果切换到多分类，则更新多分类
	if req.MultiCategory == 1 && currentSite.Content.MultiCategory != req.MultiCategory {
		needUpgrade = true
	}

	for i := range req.DefaultThumbs {
		req.DefaultThumbs[i] = strings.TrimPrefix(req.DefaultThumbs[i], currentSite.PluginStorage.StorageUrl)
	}

	currentSite.Content.RemoteDownload = req.RemoteDownload
	currentSite.Content.FilterOutlink = req.FilterOutlink
	currentSite.Content.UrlTokenType = req.UrlTokenType
	currentSite.Content.MultiCategory = req.MultiCategory
	currentSite.Content.UseSort = req.UseSort
	currentSite.Content.UseWebp = req.UseWebp
	currentSite.Content.MatchTag = req.MatchTag
	currentSite.Content.ConvertGif = req.ConvertGif
	currentSite.Content.Quality = req.Quality
	currentSite.Content.ResizeImage = req.ResizeImage
	currentSite.Content.ResizeWidth = req.ResizeWidth
	currentSite.Content.ThumbCrop = req.ThumbCrop
	currentSite.Content.ThumbWidth = req.ThumbWidth
	currentSite.Content.ThumbHeight = req.ThumbHeight
	currentSite.Content.DefaultThumb = req.DefaultThumb
	currentSite.Content.DefaultThumbType = req.DefaultThumbType
	currentSite.Content.DefaultThumbs = req.DefaultThumbs
	currentSite.Content.ThumbCategoryId = req.ThumbCategoryId
	currentSite.Content.Editor = req.Editor
	currentSite.Content.MaxPage = req.MaxPage
	currentSite.Content.MaxLimit = req.MaxLimit

	err := currentSite.SaveSettingValue(provider.ContentSettingKey, currentSite.Content)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()
	if needUpgrade {
		go currentSite.UpgradeMultiCategory()
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateContentConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// SettingThumbRebuild 执行重建所有的缩略图，会根据设置的缩略图样式、尺寸等参数生成新的缩略图。
func SettingThumbRebuild(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	go currentSite.ThumbRebuild()

	currentSite.AddAdminLog(ctx, ctx.Tr("RegenerateAllThumbnails"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ThumbnailsAreBeingAutomaticallyGenerated"),
	})
}

// SettingIndex 获取首页 SEO 设置，包括标题、关键词、描述及分隔符。
func SettingIndex(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	system := currentSite.Index

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

// SettingIndexForm 保存首页 SEO 设置并清理缓存索引。
//
// 参数说明：
//   - 请求体 "seo_title": 首页 SEO 标题。
//   - 请求体 "seo_keywords": 首页 SEO 关键词。
//   - 请求体 "seo_description": 首页 SEO 描述。
//   - 请求体 "sep": 标题分隔符。
func SettingIndexForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.IndexConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.Index.SeoTitle = req.SeoTitle
	currentSite.Index.SeoKeywords = req.SeoKeywords
	currentSite.Index.SeoDescription = req.SeoDescription
	currentSite.Index.Sep = req.Sep

	err := currentSite.SaveSettingValue(provider.IndexSettingKey, currentSite.Index)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateHomepageTdk"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// SettingContact 获取联系方式设置，包括联系人、电话、邮箱、社交账号及二维码。
func SettingContact(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	system := currentSite.Contact
	if system.Qrcode != "" && !strings.HasPrefix(system.Qrcode, "http") && !strings.HasPrefix(system.Qrcode, "//") {
		system.Qrcode = currentSite.PluginStorage.StorageUrl + system.Qrcode
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

// SettingContactForm 保存联系方式设置并清理缓存索引。
func SettingContactForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.ContactConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	req.Qrcode = strings.TrimPrefix(req.Qrcode, currentSite.PluginStorage.StorageUrl)

	if req.ExtraFields != nil {
		for i := range req.ExtraFields {
			req.ExtraFields[i].Name = library.Case2Camel(req.ExtraFields[i].Name)
		}
	}

	currentSite.Contact.UserName = req.UserName
	currentSite.Contact.Cellphone = req.Cellphone
	currentSite.Contact.Address = req.Address
	currentSite.Contact.Email = req.Email
	currentSite.Contact.Wechat = req.Wechat
	currentSite.Contact.Qrcode = req.Qrcode
	currentSite.Contact.QQ = req.QQ
	currentSite.Contact.WhatsApp = req.WhatsApp
	currentSite.Contact.Facebook = req.Facebook
	currentSite.Contact.Twitter = req.Twitter
	currentSite.Contact.Tiktok = req.Tiktok
	currentSite.Contact.Pinterest = req.Pinterest
	currentSite.Contact.Linkedin = req.Linkedin
	currentSite.Contact.Instagram = req.Instagram
	currentSite.Contact.Youtube = req.Youtube
	currentSite.Contact.ExtraFields = req.ExtraFields

	err := currentSite.SaveSettingValue(provider.ContactSettingKey, currentSite.Contact)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateContact"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// SettingCache 获取缓存设置，包括最近一次清理缓存的时间和缓存类型。
func SettingCache(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	filePath := currentSite.CachePath + "cache_clear.log"
	info, err := os.Stat(filePath)
	var lastUpdate int64
	if err == nil {
		lastUpdate = info.ModTime().Unix()
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"last_update": lastUpdate,
			"cache_type":  currentSite.GetSettingValue(provider.CacheTypeKey),
		},
	})
}

// SettingCacheForm 更新缓存设置，可选择更换缓存类型或手动清理缓存。
func SettingCacheForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.CacheConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Update {
		// 更新
		oldCacheType := currentSite.GetSettingValue(provider.CacheTypeKey)
		setting := model.Setting{
			Key:   provider.CacheTypeKey,
			Value: req.CacheType,
		}
		currentSite.DB.Save(&setting)
		if oldCacheType != req.CacheType {
			// 重新初始化缓存
			w2 := provider.GetWebsite(currentSite.Id)
			w2.InitCache()
		}
		currentSite.AddAdminLog(ctx, ctx.Tr("ChangeCacheType"))
	} else {
		currentSite.DeleteCache()

		currentSite.AddAdminLog(ctx, ctx.Tr("UpdateCacheManually"))

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("CacheUpdated"),
		})
	}
}

// SettingSafe 获取安全设置，包括验证码、内容/频率限制、封禁规则及 API 开关。
func SettingSafe(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	system := currentSite.Safe

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

// SettingSafeForm 保存安全设置并清理缓存索引。
func SettingSafeForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.SafeConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.Safe.Captcha = req.Captcha
	currentSite.Safe.DailyLimit = req.DailyLimit
	currentSite.Safe.ContentLimit = req.ContentLimit
	currentSite.Safe.IntervalLimit = req.IntervalLimit
	currentSite.Safe.ContentForbidden = req.ContentForbidden
	currentSite.Safe.IPForbidden = req.IPForbidden
	currentSite.Safe.UAForbidden = req.UAForbidden
	currentSite.Safe.APIOpen = req.APIOpen
	currentSite.Safe.APIPublish = req.APIPublish
	currentSite.Safe.AdminCaptchaOff = req.AdminCaptchaOff

	err := currentSite.SaveSettingValue(provider.SafeSettingKey, currentSite.Safe)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSecuritySettings"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// SettingDiyField 获取模板自定义字段设置。
func SettingDiyField(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	fields := currentSite.GetDiyFieldSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": fields,
	})
}

// SettingDiyFieldForm 保存模板自定义字段设置并清理相关缓存。
//
// 参数说明：
//   - 请求体: 模板自定义字段列表（config.CustomField 数组）。
func SettingDiyFieldForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req []config.CustomField
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveSettingValue(provider.DiyFieldsKey, req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.Cache.Delete(provider.DiyFieldsKey)
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSecuritySettings"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// SaveSystemFavicon 上传并保存网站 Favicon 图标。
//
// 参数说明：
//   - 表单文件 "file": Favicon 图标文件。
func SaveSystemFavicon(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	file, _, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()

	err = currentSite.SaveFavicon(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("FaviconUploadFailed"),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UploadFavicon"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("FileUploadCompleted"),
		"data": iris.Map{
			"favicon": currentSite.System.BaseUrl + "/favicon.ico",
		},
	})
}

// DeleteSystemFavicon 删除网站 Favicon 图标文件。
func DeleteSystemFavicon(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	_, err := os.Stat(currentSite.PublicPath + "favicon.ico")
	if err == nil {
		err = os.Remove(currentSite.PublicPath + "favicon.ico")
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("FaviconDeletionFailed"),
			})
			return
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteFavicon"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("IcoIconDeleted"),
	})
}

// SettingBanner 获取网站的 Banner 配置列表。
func SettingBanner(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": currentSite.Banner.Banners,
	})
}

// DeleteSettingBanner 从 Banner 列表中删除指定 Banner 项并保存设置。
func DeleteSettingBanner(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.BannerItemDeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Id == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("BannerDoesNotExist"),
		})
		return
	}
	for i := range currentSite.Banner.Banners {
		if req.Type == "" {
			req.Type = currentSite.Banner.Banners[i].Type
		}
		if req.Type == currentSite.Banner.Banners[i].Type {
			for j := range currentSite.Banner.Banners[i].List {
				if currentSite.Banner.Banners[i].List[j].Id == req.Id {
					currentSite.Banner.Banners[i].List = append(currentSite.Banner.Banners[i].List[:j], currentSite.Banner.Banners[i].List[j+1:]...)
					if len(currentSite.Banner.Banners[i].List) == 0 && req.Type != "default" {
						currentSite.Banner.Banners = append(currentSite.Banner.Banners[:i], currentSite.Banner.Banners[i+1:]...)
					}
					break
				}
			}
			break
		}
	}

	err := currentSite.SaveSettingValue(provider.BannerSettingKey, currentSite.Banner)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteBanner"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// SettingBannerForm 新增或更新 Banner 项并保存设置。
//
// 参数说明：
//   - 请求体 "id": Banner ID，为 0 时新增，否则更新对应项。
func SettingBannerForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.BannerItem
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Logo == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseSelectAnImage"),
		})
		return
	}
	if req.Type == "" {
		req.Type = "default"
	}
	req.Logo = strings.TrimPrefix(req.Logo, currentSite.PluginStorage.StorageUrl)
	if req.Id == 0 {
		var exist bool
		for i := range currentSite.Banner.Banners {
			if req.Type == currentSite.Banner.Banners[i].Type {
				exist = true
				if len(currentSite.Banner.Banners[i].List) > 0 {
					req.Id = currentSite.Banner.Banners[i].List[len(currentSite.Banner.Banners[i].List)-1].Id + 1
				} else {
					req.Id = 1
				}
				currentSite.Banner.Banners[i].List = append(currentSite.Banner.Banners[i].List, req)
				break
			}

		}
		if !exist {
			req.Id = 1
			currentSite.Banner.Banners = append(currentSite.Banner.Banners, config.Banner{
				Type: req.Type,
				List: []config.BannerItem{
					req,
				},
			})
		}
	} else {
		for i := range currentSite.Banner.Banners {
			if req.Type == currentSite.Banner.Banners[i].Type {
				for j := range currentSite.Banner.Banners[i].List {
					if currentSite.Banner.Banners[i].List[j].Id == req.Id {
						currentSite.Banner.Banners[i].List[j] = req
						break
					}
				}
				break
			}
		}
	}

	err := currentSite.SaveSettingValue(provider.BannerSettingKey, currentSite.Banner)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateBanner"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// SettingMigrateDB 对当前站点数据库执行自动迁移，更新数据表结构。
func SettingMigrateDB(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	err := provider.AutoMigrateDB(currentSite.DB, true)

	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DatabaseTableUpdated"),
	})
}

// SettingAi 获取 AI 设置，包括 AI 写作配置、AI 对话模型配置及 MCP 配置。
func SettingAi(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	defaultSite := provider.CurrentSite(nil)
	// 返回 AiGenerate 的配置，以及 eino 的配置
	aiConfig := currentSite.AiGenerateConfig
	chatConfig := defaultSite.LoadAiSetting("")

	// 意图工具清单：供后台设置页展示"暴露的工具列表"可选值。
	// 直接读 IntentCatalog 声明，避免与内核注册状态漂移。
	type mcpToolItem struct {
		Name       string `json:"name"`
		Title      string `json:"title"`
		Domain     string `json:"domain"`
		Risk       string `json:"risk"`
		Desc       string `json:"desc"`
		DefaultOff bool   `json:"default_off"`
	}
	tools := make([]mcpToolItem, 0, len(intent.IntentCatalog)+2)
	for _, spec := range intent.IntentCatalog {
		if spec == nil {
			continue
		}
		tools = append(tools, mcpToolItem{
			Name:       spec.Name,
			Title:      spec.Title,
			Domain:     string(spec.Domain),
			Risk:       string(spec.Risk),
			Desc:       spec.Desc,
			DefaultOff: spec.DefaultOff,
		})
	}
	tools = append(tools,
		mcpToolItem{Name: "mcp_list_intents", Title: "列出全部意图", Domain: "meta", Risk: "read", Desc: "返回当前站点所有意图工具的摘要"},
		mcpToolItem{Name: "mcp_set_scope", Title: "设置能力域范围", Domain: "meta", Risk: "read", Desc: "两阶段 tools/list，按能力域裁剪返回的 schema"},
	)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"write":     aiConfig,
			"chat":      chatConfig.Configs,
			"mcp":       chatConfig.Mcp,
			"mcp_tools": tools,
		},
	})
}

// SettingAiForm 更新 AI 设置，包括 AI 写作配置、AI 对话模型配置及 MCP 配置。
//
// 参数说明：
//   - 请求体 "write": AI 写作配置。
//   - 请求体 "chat": AI 对话模型配置。
//   - 请求体 "mcp": MCP 配置。
func SettingAiForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req struct {
		Write *config.AiGenerateConfig `json:"write"`
		Chat  []*eino.Config           `json:"chat"`
		Mcp   *eino.McpConfig          `json:"mcp"`
	}
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 写到AI generate 的配置里
	if req.Write != nil {
		currentSite.AiGenerateConfig.AiEngine = req.Write.AiEngine
		currentSite.AiGenerateConfig.OpenAIKeys = req.Write.OpenAIKeys
		currentSite.AiGenerateConfig.OpenAiApi = req.Write.OpenAiApi
		currentSite.AiGenerateConfig.OpenAIModel = req.Write.OpenAIModel
		currentSite.AiGenerateConfig.Spark = req.Write.Spark
		currentSite.SaveAiGenerateSetting(*currentSite.AiGenerateConfig, true)
	}

	defaultSite := provider.CurrentSite(nil)
	chatSettings := defaultSite.LoadAiSetting("")
	needSave := false
	if len(req.Chat) > 0 {
		chatSettings.Configs = req.Chat
		needSave = true
	}
	// 保存 MCP 配置
	if req.Mcp != nil {
		chatSettings.Mcp = *req.Mcp
		needSave = true
	}
	if needSave {
		if err := defaultSite.SaveSettingValue(provider.AiSettingKey, chatSettings); err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "save failed",
			})
			return
		}

		// cache new setting
		defaultSite.Cache.Set("ai_setting", chatSettings, 86400)
	}

	// MCP 配置变更后热重建所有站点的 mcp.Server（使 ExposedTools 白名单即时生效）
	if req.Mcp != nil {
		provider.RebuildMcpServers()
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateAISettings"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
		"data": iris.Map{
			"write": currentSite.AiGenerateConfig,
			"chat":  chatSettings.Configs,
			"mcp":   chatSettings.Mcp,
		},
	})
}
