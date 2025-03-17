package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginGetMultiLangConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.MultiLanguage
	// DefaultLanguage 该参数只是显示使用
	setting.DefaultLanguage = currentSite.System.Language

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginSaveMultiLangConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginMultiLangConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.SiteType == "" {
		req.SiteType = config.MultiLangSiteTypeMulti
	}

	currentSite.MultiLanguage.Open = req.Open
	currentSite.MultiLanguage.Type = req.Type
	currentSite.MultiLanguage.AutoTranslate = req.AutoTranslate
	currentSite.MultiLanguage.SiteType = req.SiteType
	// language
	currentSite.System.Language = req.DefaultLanguage
	currentSite.MultiLanguage.DefaultLanguage = currentSite.System.Language

	err := currentSite.SaveSettingValue(provider.SystemSettingKey, currentSite.System)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.SaveSettingValue(provider.MultiLangSettingKey, currentSite.MultiLanguage)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 切换站点类型的时候，需要更新 storageUrl
	mainBaseUrl := currentSite.System.BaseUrl
	if req.SiteType == config.MultiLangSiteTypeMulti && currentSite.MultiLanguage.Type != config.MultiLangTypeDomain {
		for _, v := range currentSite.MultiLanguage.SubSites {
			curSite := provider.GetWebsite(v.Id)
			if curSite != nil {
				var baseUrl string
				if currentSite.MultiLanguage.Type == config.MultiLangTypeDirectory {
					baseUrl = mainBaseUrl + "/" + v.Language
				} else {
					baseUrl = mainBaseUrl
				}
				curSite.PluginStorage.StorageUrl = baseUrl
			}
		}
	}
	currentSite.DeleteCache()
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateMultiLangConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginGetMultiLangSites(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	siteType := ctx.URLParam("type")
	if siteType == config.MultiLangSiteTypeMulti {
		// 如果站点是 single，则不返回
		if currentSite.MultiLanguage == nil || currentSite.MultiLanguage.SiteType == config.MultiLangSiteTypeSingle {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  "",
			})
			return
		}
	}
	// 读取当前站点的多语言站点
	sites := currentSite.GetMultiLangSites(currentSite.Id, true)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": sites,
	})
}

func GetValidWebsiteList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	sites := currentSite.GetMultiLangValidSites(currentSite.Id)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": sites,
	})
}

func PluginRemoveMultiLangSite(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMultiLangSiteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.RemoveMultiLangSite(req.Id, req.Language)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("RemoveMultiLangSite"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

func PluginSaveMultiLangSite(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMultiLangSiteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req.ParentId = currentSite.Id
	err := currentSite.SaveMultiLangSite(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateMultiLangSite"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}

func PluginSyncMultiLangSiteContent(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMultiLangSiteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req.ParentId = currentSite.Id

	status, err := currentSite.NewMultiLangSync()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	go status.SyncMultiLangSiteContent(&req)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SyncSiteDataIsRunningInBackend"),
	})
}

func PluginMultiSiteSyncStatus(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	status := currentSite.GetMultiLangSyncStatus()
	if status == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("ThereAreNoActiveTask"),
			"data": nil,
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": status,
	})
}

func GetTranslateHtmlLogs(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	result, total := currentSite.GetTranslateHtmlLogs(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  result,
	})
}

func GetTranslateHtmlCaches(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	lang := ctx.URLParam("lang")

	result, total := currentSite.GetTranslateHtmlCaches(lang, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  result,
	})
}

func PluginRemoveTranslateHtmlCache(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMultiLangCacheRemoveRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.All {
		currentSite.DeleteMultiLangCacheAll()
	} else {
		currentSite.DeleteMultiLangCache(req.Uris)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}
