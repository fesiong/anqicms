package manageController

import (
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginSendmailList 获取邮件发送记录列表，支持分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
func PluginSendmailList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	//不需要分页，只显示最后20条
	list, total := currentSite.GetLastSendmailList(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"data":  list,
		"total": total,
	})
}

// PluginSendmailTest 发送测试邮件，验证邮件发送配置是否可用。
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
		err := currentSite.SendMail(req.Subject, req.Message, nil, req.Recipient)
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

	err := currentSite.SendMail(subject, content, nil)
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

// PluginSendmailSetting 获取当前站点的邮件发送配置。
func PluginSendmailSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginSendmail

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginSendmailSettingForm 保存邮件发送配置。
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

// PluginGetEmailTemplates 获取邮件模板列表。
func PluginGetEmailTemplates(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	templates := currentSite.GetEmailTemplates()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": templates,
	})
}

// PluginGetEmailTemplateDetail 根据模板标识获取邮件模板详情。
//
// 参数说明：
//   - 查询参数 "key": 邮件模板标识。
func PluginGetEmailTemplateDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	key := ctx.URLParam("key")
	if key == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Missing key parameter",
		})
		return
	}

	template, exist := currentSite.GetEmailTemplateInfo(key)
	if !exist {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Template not found",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": template,
	})
}

// PluginSendmailSaveTemplate 新增或更新邮件模板。
func PluginSendmailSaveTemplate(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.EmailTemplate
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	template, err := currentSite.SaveEmailTemplateInfo(&req)
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
		"msg":  "",
		"data": template,
	})
}

// PluginSendmailTemplatePreview 根据提供的模板内容渲染预览邮件内容
func PluginSendmailTemplatePreview(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.EmailTemplate
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	data := map[string]interface{}{
		"website": currentSite,
		"user": &model.User{
			UserName: "TestUser",
			Email:    "test@example.com",
		},
		"url": "https://example.com",
		"order": map[string]interface{}{
			"Id":          123456,
			"OrderId":     "ORD20240915001",
			"Amount":      1999,
			"CreatedTime": time.Now().Unix(),
			"PaidTime":    time.Now().Unix(),
		},
		"message": map[string]interface{}{
			"CommOrderId": "ORD20240915001",
			"MessageType": "admin",
			"SenderName":  "TestUser",
			"Content":     "This is a message content for testing purposes.",
		},
		"guestbook": map[string]interface{}{
			"Id":       123456,
			"UserName": "TestUser",
			"Email":    "test@example.com",
			"Content":  "This is a guestbook content for testing purposes.",
			"ExtraData": map[string]interface{}{
				"Extra1": "Extra1Value",
				"Extra2": "Extra2Value",
			},
			"Contact":     "1234567890",
			"Referer":     "https://example.com",
			"Ip":          "192.168.1.1",
			"CreatedTime": time.Now().Unix(),
		},
	}
	res, err := currentSite.RenderEmailTemplate(&req, data)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  err,
		"data": res,
	})
}
