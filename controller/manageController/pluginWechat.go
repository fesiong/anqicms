package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginWechatConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginWechat
	// 增加serverUrl
	setting.ServerUrl = currentSite.System.BaseUrl + "/api/wechat"

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginWechatConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginWeappConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.PluginWechat.AppID = req.AppID
	currentSite.PluginWechat.AppSecret = req.AppSecret
	currentSite.PluginWechat.Token = req.Token
	currentSite.PluginWechat.EncodingAESKey = req.EncodingAESKey
	currentSite.PluginWechat.VerifyKey = req.VerifyKey
	currentSite.PluginWechat.VerifyMsg = req.VerifyMsg

	err := currentSite.SaveSettingValue(provider.WechatSettingKey, currentSite.PluginWechat)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 强制更新信息
	currentSite.GetWechatServer(true)

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateServiceAccount"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginWechatMessages(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	messages, total := currentSite.GetWechatMessages(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  messages,
	})
}

func PluginWechatMessageDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WechatMessageRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteWechatMessage(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteWechatMessageLog", req.Id, req.Content))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

func PluginWechatMessageReply(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WechatMessageRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.ReplyWechatMessage(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("WechatMessageLog", req.Id, req.Reply))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
	})
}

func PluginWechatReplyRules(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	rules, total := currentSite.GetWechatReplyRules(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  rules,
	})
}

func PluginWechatReplyRuleDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WechatReplyRuleRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteWechatReplyRule(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteWechatReplyRuleLog", req.Id, req.Keyword))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

func PluginWechatReplyRuleForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WechatReplyRuleRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveWechatReplyRule(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateWechatReplyRuleLog", req.Id, req.Keyword))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
	})
}

func PluginWechatMenus(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	menus := currentSite.GetWechatMenus()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": menus,
	})
}

func PluginWechatMenuDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WechatMenuRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteWechatMenu(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteWechatMenuLog", req.Id, req.Name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

func PluginWechatMenuSave(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WechatMenuRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveWechatMenu(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("SaveWechatMenuLog", req.Id, req.Name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
	})
}

func PluginWechatMenuSync(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	err := currentSite.SyncWechatMenu()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateWechatMenu"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
	})
}
