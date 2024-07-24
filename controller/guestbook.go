package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"strings"
	"time"
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
		webInfo.Title = ctx.Tr("OnlineMessage")
		webInfo.PageName = "guestbook"
		webInfo.CanonicalUrl = currentSite.GetUrl("/guestbook.html", nil, 0)
		ctx.ViewData("webInfo", webInfo)
	}

	tplName := "guestbook/index.html"
	if ViewExists(ctx, "guestbook.html") {
		tplName = "guestbook.html"
	}
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func GuestbookForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
			file, info, err := ctx.FormFile(item.FieldName)
			if err == nil {
				attach, err := currentSite.AttachmentUpload(file, info, 0, 0)
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
			msg := ctx.Tr("ItIsRequired", item.Name)
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
		msg := ctx.Tr("SaveFailed")
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

	//发送邮件
	subject := ctx.Tr("HasNewMessageFromWhere", currentSite.System.SiteName, guestbook.UserName)
	var contents []string
	for _, item := range fields {
		content := ctx.Tr("s:s", item.Name, req[item.FieldName]) + "\n"

		contents = append(contents, content)
	}
	// 增加来路和IP返回
	contents = append(contents, fmt.Sprintf("%s：%s\n", ctx.Tr("SubmitIp"), guestbook.Ip))
	contents = append(contents, fmt.Sprintf("%s：%s\n", ctx.Tr("SourcePage"), guestbook.Refer))
	contents = append(contents, fmt.Sprintf("%s：%s\n", ctx.Tr("SubmitTime"), time.Now().Format("2006-01-02 15:04:05")))

	if currentSite.SendTypeValid(provider.SendTypeGuestbook) {
		// 后台发信
		go currentSite.SendMail(subject, strings.Join(contents, ""))
		// 回复客户
		recipient, ok := req["email"]
		if !ok {
			recipient = req["contact"]
		}
		go currentSite.ReplyMail(recipient)
	}

	msg := currentSite.PluginGuestbook.ReturnMessage
	if msg == "" {
		msg = ctx.Tr("ThankYouForYourMessage!")
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
			{Name: ctx.Tr("ClickToContinue"), Link: link},
		})
	}
}
