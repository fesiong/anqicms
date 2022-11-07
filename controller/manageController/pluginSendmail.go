package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginSendmailList(ctx iris.Context) {
	//不需要分页，只显示最后20条
	list, err := provider.GetLastSendmailList()
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
	setting := config.JsonData.PluginSendmail
	if setting.Account == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "请先设置邮件发送账号",
		})
		return
	}

	subject := "测试邮件"
	content := "这是一封测试邮件。收到邮件表示配置正常"

	err := provider.SendMail(subject, content)
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
	setting := config.JsonData.PluginSendmail

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginSendmailSettingForm(ctx iris.Context) {
	var req config.PluginSendmail
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.PluginSendmail.Server = req.Server
	config.JsonData.PluginSendmail.UseSSL = req.UseSSL
	config.JsonData.PluginSendmail.Port = req.Port
	config.JsonData.PluginSendmail.Account = req.Account
	config.JsonData.PluginSendmail.Password = req.Password
	config.JsonData.PluginSendmail.Recipient = req.Recipient

	err := provider.SaveSettingValue(provider.SendmailSettingKey, config.JsonData.PluginSendmail)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新发送邮件配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
