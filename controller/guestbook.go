package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"strings"
)

func GuestbookPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	base := ctx.Params().Get("base")
	if base != strings.TrimLeft(currentSite.BaseURI, "/") {
		ctx.StatusCode(404)
		ShowMessage(ctx, "Not Found", nil)
		return
	}

	fields := currentSite.GetGuestbookFields()

	ctx.ViewData("fields", fields)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = currentSite.TplTr("OnlineMessage")
		webInfo.PageName = "guestbook"
		webInfo.CanonicalUrl = currentSite.GetUrl("/guestbook.html", nil, 0)
		ctx.ViewData("webInfo", webInfo)
	}

	tplName, ok := currentSite.TemplateExist("guestbook/index.html", "guestbook.html")
	if !ok {
		NotFound(ctx)
		return
	}
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func GuestbookForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)
	if !strings.HasPrefix(ctx.RequestPath(false), currentSite.BaseURI) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Not Found",
		})
		return
	}
	// 支持返回为 json 或html， 默认 html
	returnType := ctx.PostValueTrim("return")
	fields := currentSite.GetGuestbookFields()
	var req = map[string]string{}
	// 采用post接收
	extraData := map[string]interface{}{}
	for _, item := range fields {
		var val string
		if item.Type == config.CustomFieldTypeCheckbox {
			tmpVal, _ := ctx.PostValues(item.FieldName + "[]")
			val = strings.Trim(strings.Join(tmpVal, ","), ",")
		} else if item.Type == config.CustomFieldTypeImage || item.Type == config.CustomFieldTypeFile {
			// 如果有上传文件，则需要用户登录
			if userId == 0 {
				msg := currentSite.TplTr("ThisOperationRequiresLogin")
				if returnType == "json" {
					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  msg,
					})
				} else {
					ShowMessage(ctx, msg, nil)
				}
				return
			}
			file, info, err := ctx.FormFile(item.FieldName)
			if err == nil {
				attach, err := currentSite.AttachmentUpload(file, info, 0, 0, userId)
				if err == nil {
					val = attach.Logo
					if attach.Logo == "" {
						val = attach.FileLocation
					}
				}
			}
		} else {
			val = ctx.PostValueTrim(item.FieldName)
		}

		if item.Required && val == "" {
			msg := currentSite.TplTr("%sIsRequired", item.Name)
			if returnType == "json" {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  msg,
				})
			} else {
				ShowMessage(ctx, msg, nil)
			}
			return
		}
		if !item.IsSystem {
			extraData[item.Name] = val
		}
		req[item.FieldName] = val
	}
	hookCtx := &provider.HookContext{
		Point: provider.BeforeGuestbookPost,
		Site:  currentSite,
		Data:  req,
	}
	if err := provider.TriggerHook(hookCtx); err != nil {
		if returnType == "json" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
		} else {
			ShowMessage(ctx, err.Error(), nil)
		}
		return
	}
	if ok := SafeVerify(ctx, req, returnType, "guestbook"); !ok {
		return
	}

	//先填充默认字段
	guestbook := &model.Guestbook{
		UserName:  req["user_name"],
		Contact:   req["contact"],
		Content:   req["content"],
		Ip:        ctx.RemoteAddr(),
		Refer:     ctx.Request().Referer(),
		ExtraData: extraData,
	}

	err := currentSite.DB.Save(guestbook).Error
	if err != nil {
		msg := currentSite.TplTr("SaveFailed")
		if returnType == "json" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  msg,
			})
		} else {
			ShowMessage(ctx, msg, nil)
		}
		return
	}

	hookCtx.Point = provider.AfterGuestbookPost
	hookCtx.Data = guestbook
	_ = provider.TriggerHook(hookCtx)
	// akismet 验证
	go func() {
		spamStatus, isChecked := currentSite.AkismentCheck(ctx, provider.CheckTypeGuestbook, guestbook)
		if isChecked {
			currentSite.DB.Model(guestbook).UpdateColumn("status", spamStatus)
		}
		if spamStatus == 1 {
			// 1 是正常，可以发邮件
			currentSite.SendGuestbookToMail(guestbook)
			if currentSite.ParentId > 0 {
				mainSite := currentSite.GetMainWebsite()
				parentGuestbook := *guestbook
				parentGuestbook.Id = 0
				parentGuestbook.Status = spamStatus
				parentGuestbook.SiteId = currentSite.Id
				_ = mainSite.DB.Save(&parentGuestbook)
				mainSite.SendGuestbookToMail(&parentGuestbook)
			}
		}
	}()

	msg := currentSite.PluginGuestbook.ReturnMessage
	if msg == "" {
		msg = currentSite.TplTr("ThankYouForYourMessage!")
	}

	if returnType == "json" {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  msg,
		})
	} else {
		link := currentSite.GetUrl("/guestbook.html", nil, 0)
		refer := ctx.GetReferrer()
		if refer.URL != "" {
			link = refer.URL
		}

		ShowMessage(ctx, msg, []Button{
			{Name: currentSite.TplTr("ClickToContinue"), Link: link},
		})
	}
}
