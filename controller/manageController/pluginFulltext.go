package manageController

import (
	"encoding/json"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
)

// PluginFulltextConfig 获取全文索引插件的配置信息。
func PluginFulltextConfig(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginFulltext

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginFulltextConfigForm 保存全文索引插件配置，切换开启时异步重建全文索引。
func PluginFulltextConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginFulltextConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 部分更新：以现值为基底，仅覆盖请求体中显式出现的字段，未传字段保持原配置。
	body, err := ctx.GetBody()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req = *currentSite.PluginFulltext
	if err := library.MergeJSONPayload(payload, &req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	oldEngine := currentSite.PluginFulltext.Engine

	currentSite.PluginFulltext.Open = req.Open
	currentSite.PluginFulltext.UseContent = req.UseContent
	currentSite.PluginFulltext.Modules = req.Modules
	currentSite.PluginFulltext.UseCategory = req.UseCategory
	currentSite.PluginFulltext.UseTag = req.UseTag
	currentSite.PluginFulltext.Engine = req.Engine
	currentSite.PluginFulltext.EngineUrl = req.EngineUrl
	currentSite.PluginFulltext.EngineUser = req.EngineUser
	currentSite.PluginFulltext.EnginePass = req.EnginePass
	currentSite.PluginFulltext.ContainLength = req.ContainLength
	currentSite.PluginFulltext.RankingScore = req.RankingScore

	err = currentSite.SaveSettingValue(provider.FulltextSettingKey, currentSite.PluginFulltext)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateFullTextIndexConfiguration"))
	w2 := provider.GetWebsite(currentSite.Id)
	if req.Open {
		w2.CloseFulltext()
		go w2.InitFulltext(oldEngine != req.Engine)
	} else {
		w2.CloseFulltext()
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// PluginFulltextRebuild 重建全文索引，异步执行。
func PluginFulltextRebuild(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	if !currentSite.PluginFulltext.Open {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Fulltext is not open",
		})
		return
	}
	w2 := provider.GetWebsite(currentSite.Id)
	w2.CloseFulltext()
	go w2.InitFulltext(true)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SubmittedForBackgroundProcessing"),
	})
}

// PluginFulltextStatus 获取全文索引插件的运行状态。
func PluginFulltextStatus(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	status := currentSite.GetFullTextStatus()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": status,
	})
}
