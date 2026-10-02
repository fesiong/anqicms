package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginGetMultiLangConfig 获取多语言站点插件配置。
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

// PluginSaveMultiLangConfig 保存多语言站点插件配置，并根据站点类型更新子站点和缓存。
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
	currentSite.MultiLanguage.ShowMainDir = req.ShowMainDir
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
	// 处理subSite
	if req.SiteType == config.MultiLangSiteTypeSingle {
		for i, v := range currentSite.MultiLanguage.SubSites {
			if v.Language == req.DefaultLanguage || v.Id == currentSite.Id {
				currentSite.MultiLanguage.SubSites = append(currentSite.MultiLanguage.SubSites[:i], currentSite.MultiLanguage.SubSites[i+1:]...)
				i--
				continue
			}
		}
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

// PluginGetMultiLangSites 获取当前站点的多语言子站点列表。
//
// 参数说明：
//   - 查询参数 "type": 站点数据类型，仅 type=multi 且配置项中 site_type=multi 时返回。
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

// GetValidWebsiteList 获取当前站点可用的多语言站点列表。
func GetValidWebsiteList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	sites := currentSite.GetMultiLangValidSites(currentSite.Id)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": sites,
	})
}

// PluginRemoveMultiLangSite 移除多语言子站点。
//
// 参数说明：
//   - 请求体 "id": 要移除的站点ID。
//   - 请求体 "language": 要移除的站点语言。
func PluginRemoveMultiLangSite(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMultiLangSiteDeleteRequest
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

// PluginSaveMultiLangSite 保存（新增或更新）多语言子站点。
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

// PluginSyncMultiLangSiteContent 在后台异步同步多语言子站点内容，仅站点数据类型是 multi 的时候可用。
//
// 参数说明：
//   - 请求体 "id": 站点ID。
func PluginSyncMultiLangSiteContent(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginMultiLangSiteSyncRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Id == currentSite.Id {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("CannotSyncSiteContent"),
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

// PluginMultiSiteSyncStatus 获取多语言站点内容同步任务的状态。
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

// GetTranslateHtmlLogs 分页获取多语言站点页面翻译日志列表。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
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

// GetTranslateHtmlCaches 分页获取页面翻译缓存列表，支持按语言和 URI 筛选。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "lang": 按目标语言筛选。
//   - 查询参数 "uri": 按页面 URI 筛选。
func GetTranslateHtmlCaches(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	lang := ctx.URLParam("lang")
	uri := ctx.URLParam("uri")

	result, total := currentSite.GetTranslateHtmlCaches(lang, uri, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  result,
	})
}

// PluginRemoveTranslateHtmlCache 删除多语言站点页面翻译缓存。
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
