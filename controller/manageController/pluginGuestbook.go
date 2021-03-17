package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
	"irisweb/request"
)

func PluginGuestbookList(ctx iris.Context) {
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("limit", 20)
	keyword := ctx.URLParam("keyword")

	guestbookList, total, err := provider.GetGuestbookList(keyword, currentPage, pageSize)
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
		"count": total,
		"data": guestbookList,
	})
}

func PluginGuestbookDelete(ctx iris.Context) {
	var req request.PluginGuestbookDelete
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Id > 0 {
		//删一条
		guestbook, err := provider.GetGuestbookById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		err = provider.DeleteGuestbook(guestbook)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else if len(req.Ids) > 0 {
		//删除多条
		for _, id := range req.Ids {
			guestbook, err := provider.GetGuestbookById(id)
			if err != nil {
				continue
			}

			_ = provider.DeleteGuestbook(guestbook)
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已执行删除操作",
	})
}

func PluginGuestbookExport(ctx iris.Context) {
	guestbooks, err := provider.GetAllGuestbooks()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	fields := config.GetGuestbookFields()
	//header
	var header []string
	for _, v := range fields {
		header = append(header, v.Name)
	}

	var content [][]interface{}
	//content
	for _, v := range guestbooks {
		var item []interface{}
		for _, f := range fields {
			if f.IsSystem {
				if f.FieldName == "user_name" {
					item = append(item, v.UserName)
				} else if f.FieldName == "contact" {
					item = append(item, v.Contact)
				} else if f.FieldName == "content" {
					item = append(item, v.Content)
				} else {
					item = append(item, "")
				}
			} else {
				item = append(item, v.ExtraData[f.Name])
			}
		}

		content = append(content, item)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"header": header,
			"content": content,
		},
	})
}

func PluginGuestbookSetting(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"return_message": config.JsonData.PluginGuestbook.ReturnMessage,
			"fields": config.GetGuestbookFields(),
		},
	})
}

func PluginGuestbookSettingForm(ctx iris.Context) {
	var req request.PluginGuestbookSetting
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var fields []*config.GuestbookField
	for _, v := range req.Fields {
		if !v.IsSystem {
			fields = append(fields, v)
		}
	}

	config.JsonData.PluginGuestbook.ReturnMessage = req.ReturnMessage
	config.JsonData.PluginGuestbook.Fields = fields

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}