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
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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
			"msg":  ctx.Tr("后台域名请填写正确的域名，并做好解析，否则可能会导致后台无法访问"),
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

	currentSite.AddAdminLog(ctx, ctx.Tr("更新系统配置"))

	// 如果切换了模板，则需要重启
	if changed {
		config.RestartChan <- 0
		time.Sleep(1 * time.Second)
	}
	currentSite.RemoveHtmlCache()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("配置已更新"),
	})
}

func SettingContent(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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
	currentSite.Content.Quality = req.Quality
	currentSite.Content.ResizeImage = req.ResizeImage
	currentSite.Content.ResizeWidth = req.ResizeWidth
	currentSite.Content.ThumbCrop = req.ThumbCrop
	currentSite.Content.ThumbWidth = req.ThumbWidth
	currentSite.Content.ThumbHeight = req.ThumbHeight
	currentSite.Content.DefaultThumb = req.DefaultThumb
	currentSite.Content.Editor = req.Editor

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

	currentSite.AddAdminLog(ctx, ctx.Tr("更新内容配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("配置已更新"),
	})
}

// 重建所有的thumb
func SettingThumbRebuild(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	go currentSite.ThumbRebuild()

	currentSite.AddAdminLog(ctx, ctx.Tr("重新生成所有缩略图"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("缩略图正在自动生成中，请稍后查看结果"),
	})
}

func SettingIndex(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	system := currentSite.Index

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

func SettingIndexForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, ctx.Tr("更新首页TDK"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("配置已更新"),
	})
}

func SettingContact(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, ctx.Tr("更新联系人信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("配置已更新"),
	})
}

func SettingCache(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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
			currentSite.InitCache()
		}
		currentSite.AddAdminLog(ctx, ctx.Tr("更改缓存类型"))
	} else {
		currentSite.DeleteCache()

		currentSite.AddAdminLog(ctx, ctx.Tr("手动更新缓存"))

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("缓存已更新"),
		})
	}
}

func SettingSafe(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	system := currentSite.Safe

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

func SettingSafeForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, ctx.Tr("更新安全设置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("配置已更新"),
	})
}

func SaveSystemFavicon(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

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
			"msg":  ctx.Tr("Favicon上传失败"),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("上传Favicon"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("文件已上传完成"),
	})
}

func DeleteSystemFavicon(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	_, err := os.Stat(currentSite.PublicPath + "favicon.ico")
	if err == nil {
		err = os.Remove(currentSite.PublicPath + "favicon.ico")
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("Favicon删除失败"),
			})
			return
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("删除Favicon"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("Ico图标已删除"),
	})
}

func SettingBanner(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	type Banner struct {
		Type string              `json:"type"`
		List []config.BannerItem `json:"list"`
	}
	var banners []*Banner
	var mapBanners = map[string][]config.BannerItem{}

	for i := range currentSite.Banner {
		if currentSite.Banner[i].Type == "" {
			currentSite.Banner[i].Type = "default"
		}
	}
	for _, v := range currentSite.Banner {
		if !strings.HasPrefix(v.Logo, "http") && !strings.HasPrefix(v.Logo, "//") {
			v.Logo = currentSite.PluginStorage.StorageUrl + v.Logo
		}
		if _, ok := mapBanners[v.Type]; !ok {
			mapBanners[v.Type] = []config.BannerItem{}
		}
		mapBanners[v.Type] = append(mapBanners[v.Type], v)
	}
	for i := range currentSite.Banner {
		if item, ok := mapBanners[currentSite.Banner[i].Type]; ok {
			banner := &Banner{
				Type: currentSite.Banner[i].Type,
				List: item,
			}
			banners = append(banners, banner)
			delete(mapBanners, currentSite.Banner[i].Type)
		}
	}
	if len(banners) == 0 {
		banners = append(banners, &Banner{
			Type: "default",
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": banners,
	})
}

func DeleteSettingBanner(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
			"msg":  ctx.Tr("Banner 不存在"),
		})
		return
	}
	for i := range currentSite.Banner {
		if currentSite.Banner[i].Id == req.Id {
			currentSite.Banner = append(currentSite.Banner[:i], currentSite.Banner[i+1:]...)
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

	currentSite.AddAdminLog(ctx, ctx.Tr("删除Banner"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("配置已更新"),
	})
}

func SettingBannerForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
			"msg":  ctx.Tr("请选择图片"),
		})
		return
	}
	req.Logo = strings.TrimPrefix(req.Logo, currentSite.PluginStorage.StorageUrl)
	if req.Id == 0 {
		if len(currentSite.Banner) > 0 {
			req.Id = currentSite.Banner[len(currentSite.Banner)-1].Id + 1
		} else {
			req.Id = 1
		}
		currentSite.Banner = append(currentSite.Banner, req)
	} else {
		for i := range currentSite.Banner {
			if currentSite.Banner[i].Id == req.Id {
				currentSite.Banner[i] = req
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

	currentSite.AddAdminLog(ctx, ctx.Tr("更新Banner"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("配置已更新"),
	})
}

func SettingMigrateDB(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

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
		"msg":  ctx.Tr("数据库表已更新"),
	})
}
