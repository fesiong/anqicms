package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginWechatConfig 获取微信公众号插件配置，并附带服务端回调地址 ServerUrl。
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

// PluginWechatConfigForm 保存微信公众号插件配置。
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

// PluginWechatMessages 分页获取微信公众号消息列表。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
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

// PluginWechatMessageDelete 删除微信公众号消息。
//
// 参数说明：
//   - 请求体 "id": 要删除的消息ID。
func PluginWechatMessageDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WechatMessageDeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	message, err := currentSite.GetWechatMessage(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	err = currentSite.DeleteWechatMessage(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteWechatMessageLog", req.Id, message.Content))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

// PluginWechatMessageReply 回复微信公众号消息。
//
// 参数说明：
//   - 请求体 "id": 要回复的消息ID。
//   - 请求体 "reply": 回复内容。
func PluginWechatMessageReply(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WechatMessageReplyRequest
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

// PluginWechatReplyRules 分页获取微信公众号自动回复规则列表。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
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

// PluginWechatReplyRuleDelete 删除微信公众号自动回复规则。
//
// 参数说明：
//   - 请求体 "id": 要删除的规则ID。
func PluginWechatReplyRuleDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WechatReplyRuleDeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	rule, err := currentSite.GetWechatReplyRuleById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	err = currentSite.DeleteWechatReplyRule(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteWechatReplyRuleLog", req.Id, rule.Keyword))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

// PluginWechatReplyRuleForm 保存（新增或更新）微信公众号自动回复规则。
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

	rule, err := currentSite.SaveWechatReplyRule(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateWechatReplyRuleLog", rule.Id, rule.Keyword))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
		"data": rule,
	})
}

// PluginWechatMenus 获取微信公众号菜单列表。
func PluginWechatMenus(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	menus := currentSite.GetWechatMenus()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": menus,
	})
}

// PluginWechatMenuDelete 删除微信公众号菜单。
func PluginWechatMenuDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WechatMenuDeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	menu, err := currentSite.GetWechatMenuById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	err = currentSite.DeleteWechatMenu(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteWechatMenuLog", req.Id, menu.Name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

// PluginWechatMenuSave 保存（新增或更新）微信公众号菜单。
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

// PluginWechatMenuSync 推送配置的公众号菜单到微信服务器
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
