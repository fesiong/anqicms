package manageController

import (
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

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
	currentSite.PluginSendmail.UseBrevo = req.UseBrevo
	if req.UseBrevo == false {
		currentSite.PluginSendmail.BrevoApiKey = ""
	}
	// 需要先验证 brevo账号
	if req.UseBrevo && req.BrevoApiKey != "" {
		client := currentSite.BrevoClient(req.BrevoApiKey)
		if client == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "Invalid Brevo API Key",
			})
			return
		}
		currentSite.PluginSendmail.BrevoApiKey = req.BrevoApiKey
	}
	if currentSite.PluginSendmail.BrevoApiKey == "" {
		currentSite.PluginSendmail.UseBrevo = false
	}

	err := currentSite.SaveSettingValue(provider.SendmailSettingKey, currentSite.PluginSendmail)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.BrevoApiKey != "" {
		go currentSite.SyncBrevo()
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSendingEmailConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginGetEmailTemplates(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	templates := currentSite.GetEmailTemplates()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": templates,
	})
}

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

	err := currentSite.SaveEmailTemplateInfo(&req)
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
	})
}

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
			"Id":           123456,
			"OrderId":      "ORD20240915001",
			"Amount":       1999,
			"QuotePrice":   2499,
			"QuoteContent": "This is a quoted content for testing purposes.",
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
		"cart": map[string]interface{}{
			"Items": []model.Archive{
				{
					Id:          123456,
					Title:       "Test Product 1",
					Description: "This is a test product for testing purposes.",
					Thumb:       currentSite.System.SiteLogo,
					Price:       1999,
					Quantity:    1,
					Amount:      1999,
				},
				{
					Id:          123456,
					Title:       "Test Product 1",
					Description: "This is a test product for testing purposes.",
					Thumb:       currentSite.System.SiteLogo,
					Price:       1999,
					Quantity:    1,
					Amount:      1999,
				},
			},
			"Amount": 3998,
			"Count":  2,
		},
		"oldGroup": model.UserGroup{
			Id:          1,
			Title:       "Test Group 1",
			Description: "This is a test group for testing purposes.",
			Price:       1999,
			Level:       0,
		},
		"newGroup": model.UserGroup{
			Id:          2,
			Title:       "Test Group 2",
			Description: "This is a test group for testing purposes.",
			Price:       1999,
			Level:       1,
		},
		"integralLog": model.IntegralLog{
			Id:            1,
			UserId:        1,
			UserName:      "TestUser",
			Integral:      1000,
			AfterIntegral: 1999,
			OrderId:       "ORD20240915001",
			Status:        1,
			Type:          1,
			TaskId:        1,
			Remark:        "This is a test integral log for testing purposes.",
		},
	}
	res, err := currentSite.RenderEmailTemplate(&req, data)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  err,
		"data": res,
	})
}
