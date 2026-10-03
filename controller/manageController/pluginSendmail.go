package manageController

import (
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// SendmailPasswordMask 是 SMTP 授权码的掩码占位符。
//
// 背景：PluginSendmailSetting 直接返回含明文授权码的整个 setting 结构，
// 经 MCP / API 暴露等于把邮箱凭证交给模型（实测可读到明文 password）。
//
// 为什么用「固定哨兵值」而不是「留空表示不修改」：
// PluginSendmailSettingForm 是逐字段覆盖（不是合并），若读取时置空、
// 保存时空值又被当作新密码写回，授权码就被清掉了。固定哨兵值让
// 「未修改」这件事在响应里可见、可回传，且不与任何真实授权码冲突
// （真实授权码是 16 位随机字符，不会恰好等于本串）。
const SendmailPasswordMask = "********"


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
	// 必须解引用做**值拷贝**再掩码。
	//
	// PluginSendmail 的类型是 *config.PluginSendmail（provider/website.go:85），
	// 所以 `setting := currentSite.PluginSendmail` 只是拷贝指针，
	// 随后 `setting.Password = mask` 改的就是内存里的全局对象 ——
	// 一次 setting_get 调用就会把运行中的真实授权码抹成哨兵串，
	// 之后所有发信失败，且没有任何报错（2026-10-03 实际踩过）。
	//
	// 结构体字段全是值类型（string/int），解引用即得独立副本；
	// Templates 切片虽共享底层数组，但这里不修改它，无需深拷贝。
	setting := *currentSite.PluginSendmail
	if setting.Password != "" {
		setting.Password = SendmailPasswordMask
	}

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
	// 掩码回写保护：前端/AI 拿到的是 SendmailPasswordMask，原样回传表示
	// 「授权码没变」，此时必须沿用内存里的真实值，否则会被哨兵串覆盖成
	// 一个无效授权码，站点就再也发不出邮件了（且没有任何报错）。
	if req.Password != SendmailPasswordMask {
		currentSite.PluginSendmail.Password = req.Password
	}
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
	// 只传了 key、没传 content 时，用库里的模板补全后再渲染。
	//
	// 背景：本端点要的是「待渲染的模板内容」（config.EmailTemplate 含
	// content/subject），但 AI 侧手里通常只有 key（detail 能查到），
	// 直接调用会渲染出一个空模板——返回结构完整但内容全空，极像成功。
	if req.Content == "" && req.Key != "" {
		if tpl, exist := currentSite.GetEmailTemplateInfo(req.Key); exist {
			if req.Subject == "" {
				req.Subject = tpl.Subject
			}
			req.Content = tpl.Content
			req.Delay = tpl.Delay
			if req.Type == "" {
				req.Type = tpl.Type
			}
		}
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
