package manageController

import (
	"os"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

// PluginGetLLMsSetting 获取 llms.txt 插件设置，并检查 llms.txt 文件的存在状态、更新时间和访问地址。
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

// PluginSaveLLMsSetting 保存 llms.txt 插件设置。
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

// PluginLLMsBuild 触发生成站点的 llms.txt 文件，需先开启 llms.txt 插件。
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

// PluginGetLLMsStatus 获取 llms.txt 文件的生成状态。
func PluginGetLLMsStatus(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	status := currentSite.GetLLMsBuildStatus()
	if status == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Finished",
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": status,
	})
}
