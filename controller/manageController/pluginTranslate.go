package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

func PluginGetTranslateConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginTranslate

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginSaveTranslateConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginTranslateConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	w2 := provider.GetWebsite(currentSite.Id)
	w2.PluginTranslate = &req
	err := currentSite.SaveSettingValue(provider.TranslateSettingKey, w2.PluginTranslate)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateTranslateConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginTranslateLogList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	var total int64
	var logs []*model.TranslateLog
	tx := currentSite.DB.Model(&model.TranslateLog{})
	offset := 0
	if currentPage > 0 {
		offset = (currentPage - 1) * pageSize
	}
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&logs)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  logs,
	})
}
