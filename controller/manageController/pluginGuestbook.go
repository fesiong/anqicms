package manageController

import (
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginGuestbookList 分页获取留言列表，支持关键词和状态筛选。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "keyword": 搜索关键词，模糊匹配用户名、联系方式或内容。
//   - 查询参数 "status": 状态筛选，可选 default、ok、spam。
func PluginGuestbookList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	keyword := ctx.URLParam("keyword")

	guestbookList, total, err := currentSite.GetGuestbookList(func(tx *gorm.DB) *gorm.DB {
		if keyword != "" {
			tx = tx.Where("user_name like ? or contact like ? or content like ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}
		tmpStatus := ctx.URLParam("status")
		status := -1
		if tmpStatus == "default" {
			status = 0
		} else if tmpStatus == "ok" {
			status = 1
		} else if tmpStatus == "spam" {
			status = 2
		}
		if status != -1 {
			tx = tx.Where("`status` = ?", status)
		}
		return tx
	}, currentPage, pageSize)
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

// PluginGuestbookDetail 根据ID获取留言详情。
//
// 参数说明：
//   - 查询参数 "id": 留言ID。
func PluginGuestbookDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	//需要支持分页，还要支持搜索
	id := ctx.URLParamIntDefault("id", 0)

	guestbook, err := currentSite.GetGuestbookById(uint(id))

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
		"data": guestbook,
	})
}

// PluginGuestbookUpdateStatus 更新留言状态，支持单个或批量操作。
//
// 参数说明：
//   - 请求体 "id": 单个留言ID，大于0时更新该条。
//   - 请求体 "ids": 留言ID列表，批量更新时使用。
//   - 请求体 "status": 目标状态：0=待审，1=正常，2=垃圾。
func PluginGuestbookUpdateStatus(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginGuestbookStatus
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Id > 0 {
		//一条
		guestbook, err := currentSite.GetGuestbookById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		err = currentSite.UpdateGuestbookStatus(guestbook.Id, req.Status)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else if len(req.Ids) > 0 {
		//多条
		for _, id := range req.Ids {
			guestbook, err := currentSite.GetGuestbookById(id)
			if err != nil {
				continue
			}

			_ = currentSite.UpdateGuestbookStatus(guestbook.Id, req.Status)
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateGuestbookStatusLog", req.Id, req.Ids))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteOperationHasBeenPerformed"),
	})
}

// PluginGuestbookDelete 删除留言，支持单个或批量操作。\n//\n// 参数说明：
//   - 请求体 "id": 单个留言ID，大于0时删除该条。
//   - 请求体 "ids": 留言ID列表，批量删除时使用。
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

// PluginGuestbookExport 导出全部留言数据，包含自定义字段、时间、IP等信息。
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
	// append createdTime, IP
	header = append(header, "Created Time", "IP", "Refer", "Site Id")
	header = append(header, "Spam")

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
		itemTime := time.Unix(v.CreatedTime, 0).Format(time.DateTime)
		item = append(item, itemTime, v.Ip, v.Refer, v.SiteId)
		item = append(item, v.Status == 2)

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

// PluginGuestbookSetting 获取留言插件配置及自定义字段列表。
func PluginGuestbookSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginGuestbook
	setting.Fields = currentSite.GetGuestbookFields()
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

// PluginGuestbookSettingForm 更新留言插件配置及自定义字段列表。
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
	currentSite.PluginGuestbook.PushWay = req.PushWay
	currentSite.PluginGuestbook.SiteId = req.SiteId
	currentSite.PluginGuestbook.ApiMethod = req.ApiMethod
	currentSite.PluginGuestbook.ApiURL = req.ApiURL
	currentSite.PluginGuestbook.HeaderKey = req.HeaderKey
	currentSite.PluginGuestbook.HeaderValue = req.HeaderValue
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
