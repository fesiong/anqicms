package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
)

func PluginGuestbookList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	keyword := ctx.URLParam("keyword")

	guestbookList, total, err := currentSite.GetGuestbookList(keyword, currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  guestbookList,
	})
}

func PluginGuestbookDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
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
		guestbook, err := currentSite.GetGuestbookById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		err = currentSite.DeleteGuestbook(guestbook)
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
			guestbook, err := currentSite.GetGuestbookById(id)
			if err != nil {
				continue
			}

			_ = currentSite.DeleteGuestbook(guestbook)
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteMessageLog", req.Id, req.Ids))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteOperationHasBeenPerformed"),
	})
}

func PluginGuestbookExport(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	guestbooks, err := currentSite.GetAllGuestbooks()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	fields := currentSite.GetGuestbookFields()
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

	currentSite.AddAdminLog(ctx, ctx.Tr("ExportMessage"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"header":  header,
			"content": content,
		},
	})
}

func PluginGuestbookSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"return_message": currentSite.PluginGuestbook.ReturnMessage,
			"fields":         currentSite.GetGuestbookFields(),
		},
	})
}

func PluginGuestbookSettingForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginGuestbookConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var fields []*config.CustomField
	var existsFields = map[string]struct{}{}
	for _, v := range req.Fields {
		if !v.IsSystem {
			if v.FieldName == "" {
				v.FieldName = strings.ReplaceAll(library.GetPinyin(v.Name, currentSite.Content.UrlTokenType == config.UrlTokenTypeSort), "-", "_")
			}
		}
		if _, ok := existsFields[v.FieldName]; !ok {
			existsFields[v.FieldName] = struct{}{}
			fields = append(fields, v)
		}
	}

	currentSite.PluginGuestbook.ReturnMessage = req.ReturnMessage
	currentSite.PluginGuestbook.Fields = fields

	err := currentSite.SaveSettingValue(provider.GuestbookSettingKey, currentSite.PluginGuestbook)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyMessageSetting"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}
