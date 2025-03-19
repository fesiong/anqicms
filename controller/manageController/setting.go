package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"os"
	"strings"
	"time"
)

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
	readerInfos, err := os.ReadDir(fmt.Sprintf("%slocales", config.ExecPath))
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
	currentSite.System.SiteName = req.SiteName
	currentSite.System.SiteLogo = req.SiteLogo
	currentSite.System.SiteIcp = req.SiteIcp
	currentSite.System.SiteCopyright = req.SiteCopyright
	currentSite.System.AdminUrl = req.AdminUrl
	currentSite.System.SiteClose = req.SiteClose
	currentSite.System.SiteCloseTips = req.SiteCloseTips
	currentSite.System.BanSpider = req.BanSpider
	// 如果本来storageUrl = baseUrl
	if currentSite.PluginStorage.StorageUrl == currentSite.System.BaseUrl {
		currentSite.PluginStorage.StorageUrl = req.BaseUrl
		currentSite.SaveSettingValue(provider.StorageSettingKey, currentSite.PluginStorage)
	}
	currentSite.System.BaseUrl = req.BaseUrl
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

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSystemConfiguration"))

	// 如果切换了模板，则需要重启
	if changed {
		config.RestartChan <- 0
		time.Sleep(1 * time.Second)
	}
	currentSite.RemoveHtmlCache()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func SettingContent(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	system := currentSite.Content
	if system.DefaultThumb != "" && !strings.HasPrefix(system.DefaultThumb, "http") && !strings.HasPrefix(system.DefaultThumb, "//") {
		system.DefaultThumb = currentSite.PluginStorage.StorageUrl + system.DefaultThumb
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

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

	req.DefaultThumb = strings.TrimPrefix(req.DefaultThumb, currentSite.PluginStorage.StorageUrl)

	currentSite.Content.RemoteDownload = req.RemoteDownload
	currentSite.Content.FilterOutlink = req.FilterOutlink
	currentSite.Content.UrlTokenType = req.UrlTokenType
	currentSite.Content.MultiCategory = req.MultiCategory
	currentSite.Content.UseSort = req.UseSort
	currentSite.Content.UseWebp = req.UseWebp
	currentSite.Content.ConvertGif = req.ConvertGif
	currentSite.Content.Quality = req.Quality
	currentSite.Content.ResizeImage = req.ResizeImage
	currentSite.Content.ResizeWidth = req.ResizeWidth
	currentSite.Content.ThumbCrop = req.ThumbCrop
	currentSite.Content.ThumbWidth = req.ThumbWidth
	currentSite.Content.ThumbHeight = req.ThumbHeight
	currentSite.Content.DefaultThumb = req.DefaultThumb
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

// 重建所有的thumb
func SettingThumbRebuild(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	go currentSite.ThumbRebuild()

	currentSite.AddAdminLog(ctx, ctx.Tr("RegenerateAllThumbnails"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ThumbnailsAreBeingAutomaticallyGenerated"),
	})
}

func SettingIndex(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	system := currentSite.Index

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

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

func SettingSafe(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	system := currentSite.Safe

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

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

func SettingDiyField(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	fields := currentSite.GetDiyFieldSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": fields,
	})
}

func SettingDiyFieldForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req []config.ExtraField
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
	})
}

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

func SettingBanner(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": currentSite.Banner.Banners,
	})
}

func DeleteSettingBanner(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.BannerItem
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
