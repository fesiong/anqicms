package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginSendmailList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	//不需要分页，只显示最后20条
	list, err := currentSite.GetLastSendmailList()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": list,
	})
}

func PluginSendmailTest(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginSendmail
	if setting.Account == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseSetUpTheEmailSendingAccountFirst"),
		})
		return
	}
	var req request.PluginTestSendmailRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Recipient != "" {
		if req.Subject == "" || req.Message == "" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("PleaseFillInTheReplyTitleAndContent"),
			})
			return
		}
		err := currentSite.SendMail(req.Subject, req.Message, req.Recipient)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("EmailSentSuccessfully"),
		})
		return
	}

	subject := ctx.Tr("TestEmail")
	content := ctx.Tr("ThisIsATestEmail")

	err := currentSite.SendMail(subject, content)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("EmailSentSuccessfully"),
	})
}

func PluginSendmailSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginSendmail

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginSendmailSettingForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginSendmail
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.PluginSendmail.Server = req.Server
	currentSite.PluginSendmail.UseSSL = req.UseSSL
	currentSite.PluginSendmail.Port = req.Port
	currentSite.PluginSendmail.Account = req.Account
	currentSite.PluginSendmail.Password = req.Password
	currentSite.PluginSendmail.Recipient = req.Recipient
	currentSite.PluginSendmail.AutoReply = req.AutoReply
	currentSite.PluginSendmail.ReplySubject = req.ReplySubject
	currentSite.PluginSendmail.ReplyMessage = req.ReplyMessage
	currentSite.PluginSendmail.SendType = req.SendType

	err := currentSite.SaveSettingValue(provider.SendmailSettingKey, currentSite.PluginSendmail)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSendingEmailConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}
