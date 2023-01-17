package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginSendmailList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginSendmail
	if setting.Account == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请先设置邮件发送账号",
		})
		return
	}

	subject := "测试邮件"
	content := "这是一封测试邮件。收到邮件表示配置正常"

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
		"msg":  "邮件发送成功",
	})
}

func PluginSendmailSetting(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginSendmail

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginSendmailSettingForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

	err := currentSite.SaveSettingValue(provider.SendmailSettingKey, currentSite.PluginSendmail)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新发送邮件配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
