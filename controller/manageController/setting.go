package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"os"
	"strings"
	"time"
)

func SettingSystem(ctx iris.Context) {
	system := config.JsonData.System
	if system.SiteLogo != "" && !strings.HasPrefix(system.SiteLogo, "http") && !strings.HasPrefix(system.SiteLogo, "//") {
		system.SiteLogo = config.JsonData.PluginStorage.StorageUrl + system.SiteLogo
	}

	// 读取language列表
	var languages []string
	readerInfos, err := os.ReadDir(fmt.Sprintf("%slanguage", config.ExecPath))
	if err == nil {
		for _, info := range readerInfos {
			if strings.HasSuffix(info.Name(), ".yml") {
				languages = append(languages, strings.TrimSuffix(info.Name(), ".yml"))
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
			"msg":  "后台域名请填写正确的域名，并做好解析，否则可能会导致后台无法访问",
		})
		return
	}

	req.SiteLogo = strings.TrimPrefix(req.SiteLogo, config.JsonData.PluginStorage.StorageUrl)

	changed := false
	if config.JsonData.System.AdminUrl != req.AdminUrl {
		changed = true
	}
	if req.ExtraFields != nil {
		for i := range req.ExtraFields {
			req.ExtraFields[i].Name = library.Case2Camel(req.ExtraFields[i].Name)
		}
	}
	req.BaseUrl = strings.TrimRight(req.BaseUrl, "/")
	config.JsonData.System.SiteName = req.SiteName
	config.JsonData.System.SiteLogo = req.SiteLogo
	config.JsonData.System.SiteIcp = req.SiteIcp
	config.JsonData.System.SiteCopyright = req.SiteCopyright
	config.JsonData.System.AdminUrl = req.AdminUrl
	config.JsonData.System.SiteClose = req.SiteClose
	config.JsonData.System.SiteCloseTips = req.SiteCloseTips
	// 如果本来storageUrl = baseUrl
	if config.JsonData.PluginStorage.StorageUrl == config.JsonData.System.BaseUrl {
		config.JsonData.PluginStorage.StorageUrl = req.BaseUrl
		provider.SaveSettingValue(provider.StorageSettingKey, config.JsonData.PluginStorage)
	}
	config.JsonData.System.BaseUrl = req.BaseUrl
	config.JsonData.System.MobileUrl = req.MobileUrl
	config.JsonData.System.Language = req.Language
	config.JsonData.System.ExtraFields = req.ExtraFields

	err := provider.SaveSettingValue(provider.SystemSettingKey, config.JsonData.System)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 重载 language
	config.LoadLanguage()

	// 如果切换了模板，则重载模板
	if changed {
		config.RestartChan <- true
		time.Sleep(2 * time.Second)
	}
	provider.DeleteCacheIndex()

	provider.AddAdminLog(ctx, fmt.Sprintf("更新系统配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func SettingContent(ctx iris.Context) {
	system := config.JsonData.Content
	if system.DefaultThumb != "" && !strings.HasPrefix(system.DefaultThumb, "http") && !strings.HasPrefix(system.DefaultThumb, "//") {
		system.DefaultThumb = config.JsonData.PluginStorage.StorageUrl + system.DefaultThumb
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

func SettingContentForm(ctx iris.Context) {
	var req config.ContentConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	req.DefaultThumb = strings.TrimPrefix(req.DefaultThumb, config.JsonData.PluginStorage.StorageUrl)

	config.JsonData.Content.RemoteDownload = req.RemoteDownload
	config.JsonData.Content.FilterOutlink = req.FilterOutlink
	config.JsonData.Content.UrlTokenType = req.UrlTokenType
	config.JsonData.Content.UseWebp = req.UseWebp
	config.JsonData.Content.Quality = req.Quality
	config.JsonData.Content.ResizeImage = req.ResizeImage
	config.JsonData.Content.ResizeWidth = req.ResizeWidth
	config.JsonData.Content.ThumbCrop = req.ThumbCrop
	config.JsonData.Content.ThumbWidth = req.ThumbWidth
	config.JsonData.Content.ThumbHeight = req.ThumbHeight
	config.JsonData.Content.DefaultThumb = req.DefaultThumb

	err := provider.SaveSettingValue(provider.ContentSettingKey, config.JsonData.Content)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.DeleteCacheIndex()

	provider.AddAdminLog(ctx, fmt.Sprintf("更新内容配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

// 重建所有的thumb
func SettingThumbRebuild(ctx iris.Context) {
	go provider.ThumbRebuild()

	provider.AddAdminLog(ctx, fmt.Sprintf("重新生成所有缩略图"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "缩略图正在自动生成中，请稍后查看结果。",
	})
}

func SettingIndex(ctx iris.Context) {
	system := config.JsonData.Index

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

func SettingIndexForm(ctx iris.Context) {
	var req config.IndexConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.Index.SeoTitle = req.SeoTitle
	config.JsonData.Index.SeoKeywords = req.SeoKeywords
	config.JsonData.Index.SeoDescription = req.SeoDescription

	err := provider.SaveSettingValue(provider.IndexSettingKey, config.JsonData.Index)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.DeleteCacheIndex()

	provider.AddAdminLog(ctx, fmt.Sprintf("更新首页TDK"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func SettingContact(ctx iris.Context) {
	system := config.JsonData.Contact
	if system.Qrcode != "" && !strings.HasPrefix(system.Qrcode, "http") && !strings.HasPrefix(system.Qrcode, "//") {
		system.Qrcode = config.JsonData.PluginStorage.StorageUrl + system.Qrcode
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

func SettingContactForm(ctx iris.Context) {
	var req config.ContactConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	req.Qrcode = strings.TrimPrefix(req.Qrcode, config.JsonData.PluginStorage.StorageUrl)

	if req.ExtraFields != nil {
		for i := range req.ExtraFields {
			req.ExtraFields[i].Name = library.Case2Camel(req.ExtraFields[i].Name)
		}
	}

	config.JsonData.Contact.UserName = req.UserName
	config.JsonData.Contact.Cellphone = req.Cellphone
	config.JsonData.Contact.Address = req.Address
	config.JsonData.Contact.Email = req.Email
	config.JsonData.Contact.Wechat = req.Wechat
	config.JsonData.Contact.Qrcode = req.Qrcode
	config.JsonData.Contact.ExtraFields = req.ExtraFields

	err := provider.SaveSettingValue(provider.ContactSettingKey, config.JsonData.Contact)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.DeleteCacheIndex()

	provider.AddAdminLog(ctx, fmt.Sprintf("更新联系人信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func SettingCache(ctx iris.Context) {
	filePath := fmt.Sprintf("%scache/%s.log", config.ExecPath, "cache_clear")
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
		},
	})
}

func SettingCacheForm(ctx iris.Context) {
	provider.DeleteCache()

	provider.AddAdminLog(ctx, fmt.Sprintf("手动更新缓存"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "缓存已更新",
	})
}

func SettingSafe(ctx iris.Context) {
	system := config.JsonData.Safe

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": system,
	})
}

func SettingSafeForm(ctx iris.Context) {
	var req config.SafeConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.Safe.Captcha = req.Captcha
	config.JsonData.Safe.DailyLimit = req.DailyLimit
	config.JsonData.Safe.ContentLimit = req.ContentLimit
	config.JsonData.Safe.IntervalLimit = req.IntervalLimit
	config.JsonData.Safe.ContentForbidden = req.ContentForbidden
	config.JsonData.Safe.IPForbidden = req.IPForbidden
	config.JsonData.Safe.UAForbidden = req.UAForbidden

	err := provider.SaveSettingValue(provider.SafeSettingKey, config.JsonData.Safe)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.DeleteCacheIndex()

	provider.AddAdminLog(ctx, fmt.Sprintf("更新安全设置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
