package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"strings"
	"time"
)

func GuestbookPage(ctx iris.Context) {
	fields := config.GetGuestbookFields()

	ctx.ViewData("fields", fields)

	webInfo.Title = config.Lang("在线留言")
	webInfo.PageName = "guestbook"
	webInfo.CanonicalUrl = provider.GetUrl("/guestbook.html", nil, 0)
	ctx.ViewData("webInfo", webInfo)

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
	// 支持返回为 json 或html， 默认 html
	returnType := ctx.PostValueTrim("return")
	fields := config.GetGuestbookFields()
	var req = map[string]string{}
	// 采用post接收
	extraData := map[string]interface{}{}
	for _, item := range fields {
		var val string
		if item.Type == config.CustomFieldTypeCheckbox {
			tmpVal := ctx.PostValues(item.FieldName + "[]")
			val = strings.Trim(strings.Join(tmpVal, ","), ",")
		} else if item.Type == config.CustomFieldTypeImage {
			file, info, err := ctx.FormFile(item.FieldName)
			if err == nil {
				attach, err := provider.AttachmentUpload(file, info, 0, 0)
				if err == nil {
					val = attach.Logo
				}
			}
		} else {
			val = ctx.PostValueTrim(item.FieldName)
		}

		if item.Required && val == "" {
			msg := fmt.Sprintf("%s必填", item.Name)
			if returnType == "json" {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  msg,
				})
			} else {
				ShowMessage(ctx, msg, "")
			}
			return
		}
		if !item.IsSystem {
			extraData[item.Name] = val
		}
		req[item.FieldName] = val
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

	err := dao.DB.Save(guestbook).Error
	if err != nil {
		msg := config.Lang("保存失败")
		if returnType == "json" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  msg,
			})
		} else {
			ShowMessage(ctx, msg, "")
		}
		return
	}

	//发送邮件
	subject := fmt.Sprintf(config.Lang("%s有来自%s的新留言"), config.JsonData.System.SiteName, guestbook.UserName)
	var contents []string
	for _, item := range fields {
		content := fmt.Sprintf("%s：%s\n", item.Name, req[item.FieldName])

		contents = append(contents, content)
	}
	// 增加来路和IP返回
	contents = append(contents, fmt.Sprintf("%s：%s\n", config.Lang("提交IP"), guestbook.Ip))
	contents = append(contents, fmt.Sprintf("%s：%s\n", config.Lang("来源页面"), guestbook.Refer))
	contents = append(contents, fmt.Sprintf("%s：%s\n", config.Lang("提交时间"), time.Now().Format("2006-01-02 15:04:05")))

	// 后台发信
	go provider.SendMail(subject, strings.Join(contents, ""))

	msg := config.JsonData.PluginGuestbook.ReturnMessage
	if msg == "" {
		msg = config.Lang("感谢您的留言！")
	}

	if returnType == "json" {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  msg,
		})
	} else {
		link := provider.GetUrl("/guestbook.html", nil, 0)
		refer := ctx.GetReferrer()
		if refer.URL != "" {
			link = refer.URL
		}

		ShowMessage(ctx, msg, link)
	}
}
