package manageController

import (
	"os"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginGetLLMsSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginLLMs

	// 检查文件状态
	llmsFile := currentSite.PublicPath + "/llms.txt"
	if info, err := os.Stat(llmsFile); os.IsNotExist(err) {
		setting.FileStatus = false
	} else {
		setting.FileStatus = true
		setting.LastUpdate = info.ModTime().Unix()
		setting.FileUrl = currentSite.System.BaseUrl + "/llms.txt"
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginSaveLLMsSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginLLMsConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	w2 := provider.GetWebsite(currentSite.Id)
	w2.PluginLLMs = &req
	err := currentSite.SaveSettingValue(provider.LLMsSettingKey, w2.PluginLLMs)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateLLMsConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginLLMsBuild(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginLLMs

	if setting == nil || !setting.Open {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("LLMsNotEnabled"),
		})
		return
	}

	err := currentSite.LLMsBuild()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LLMsBuildProgressing"),
	})
}

func PluginGetLLMsStatus(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	status := currentSite.GetLLMsBuildStatus()
	if status == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("LLMsNotEnabled"),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": status,
	})
}
