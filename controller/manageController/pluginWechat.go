package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginWechatConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginWechat

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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新服务号信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除微信留言：%d => %s", req.Id, req.Content))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("微信留言：%d => %s", req.Id, req.Reply))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "操作成功",
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除微信自动回复规则：%d => %s", req.Id, req.Keyword))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新微信自动回复规则：%d => %s", req.Id, req.Keyword))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "操作成功",
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除微信菜单：%d => %s", req.Id, req.Name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("保存微信菜单：%d => %s", req.Id, req.Name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "操作成功",
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新微信菜单"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "操作成功",
	})
}
