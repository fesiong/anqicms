package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

func GuestbookPage(ctx iris.Context) {
	fields := config.GetGuestbookFields()

	ctx.ViewData("fields", fields)

	//热门文章
	populars, _, _ := provider.GetArticleList(0, "views desc", 1, 10)
	ctx.ViewData("populars", populars)

	ctx.View("guestbook/index.html")
}

func GuestbookForm(ctx iris.Context) {
	fields := config.GetGuestbookFields()
	var req map[string]interface{}
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	extraData := map[string]interface{}{}
	for _, item := range fields {
		if item.Required && req[item.FieldName] == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  fmt.Sprintf("%s必填", item.Name),
			})
			return
		}
		if !item.IsSystem {
			extraData[item.Name] = req[item.FieldName]
		}
	}

	//先填充默认字段
	guestbook := &model.Guestbook{
		UserName:  req["user_name"].(string),
		Contact:   req["contact"].(string),
		Content:   req["content"].(string),
		Ip:        ctx.RemoteAddr(),
		Refer:     ctx.Request().Referer(),
		ExtraData: extraData,
	}

	err := config.DB.Save(guestbook).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "保存失败",
		})
		return
	}

	msg := config.JsonData.PluginGuestbook.ReturnMessage
	if msg == "" {
		msg = "感谢您的留言！"
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg": msg,
	})
}
