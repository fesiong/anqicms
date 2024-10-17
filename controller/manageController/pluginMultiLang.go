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

	currentSite.MultiLanguage.Open = req.Open
	currentSite.MultiLanguage.Type = req.Type
	currentSite.MultiLanguage.AutoTranslate = req.AutoTranslate
	// language
	currentSite.System.Language = req.DefaultLanguage

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

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateMultiLangConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginGetMultiLangSites(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	// 读取当前站点的多语言站点
	sites := currentSite.GetMultiLangSites(currentSite.Id)

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

	err := currentSite.RemoveMultiLangSite(req.Id)
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
