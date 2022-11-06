package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginWechatConfig(ctx iris.Context) {
	setting := config.JsonData.PluginWechat

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginWechatConfigForm(ctx iris.Context) {
	var req request.PluginWeappConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.PluginWechat.AppID = req.AppID
	config.JsonData.PluginWechat.AppSecret = req.AppSecret
	config.JsonData.PluginWechat.Token = req.Token
	config.JsonData.PluginWechat.EncodingAESKey = req.EncodingAESKey
	config.JsonData.PluginWechat.VerifyKey = req.VerifyKey
	config.JsonData.PluginWechat.VerifyMsg = req.VerifyMsg

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 强制更新信息
	provider.GetWechatServer(true)

	provider.AddAdminLog(ctx, fmt.Sprintf("更新服务号信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func PluginWechatMessages(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	messages, total := provider.GetWechatMessages(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  messages,
	})
}

func PluginWechatMessageDelete(ctx iris.Context) {
	var req request.WechatMessageRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.DeleteWechatMessage(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.AddAdminLog(ctx, fmt.Sprintf("删除微信留言：%d => %s", req.Id, req.Content))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func PluginWechatMessageReply(ctx iris.Context) {
	var req request.WechatMessageRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.ReplyWechatMessage(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.AddAdminLog(ctx, fmt.Sprintf("微信留言：%d => %s", req.Id, req.Reply))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "操作成功",
	})
}

func PluginWechatReplyRules(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	rules, total := provider.GetWechatReplyRules(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  rules,
	})
}

func PluginWechatReplyRuleDelete(ctx iris.Context) {
	var req request.WechatReplyRuleRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.DeleteWechatReplyRule(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.AddAdminLog(ctx, fmt.Sprintf("删除微信自动回复规则：%d => %s", req.Id, req.Keyword))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func PluginWechatReplyRuleForm(ctx iris.Context) {
	var req request.WechatReplyRuleRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.SaveWechatReplyRule(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.AddAdminLog(ctx, fmt.Sprintf("更新微信自动回复规则：%d => %s", req.Id, req.Keyword))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "操作成功",
	})
}

func PluginWechatMenus(ctx iris.Context) {
	menus := provider.GetWechatMenus()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": menus,
	})
}

func PluginWechatMenuDelete(ctx iris.Context) {
	var req request.WechatMenuRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.DeleteWechatMenu(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.AddAdminLog(ctx, fmt.Sprintf("删除微信菜单：%d => %s", req.Id, req.Name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func PluginWechatMenuSave(ctx iris.Context) {
	var req request.WechatMenuRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.SaveWechatMenu(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.AddAdminLog(ctx, fmt.Sprintf("保存微信菜单：%d => %s", req.Id, req.Name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "操作成功",
	})
}

func PluginWechatMenuSync(ctx iris.Context) {
	err := provider.SyncWechatMenu()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	provider.AddAdminLog(ctx, fmt.Sprintf("更新微信菜单"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "操作成功",
	})
}
