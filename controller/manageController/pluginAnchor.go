package manageController

import (
	"encoding/json"
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginAnchorList 分页获取锚文本列表，支持关键词搜索。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "keyword": 搜索关键词。
//   - 查询参数 "title": 兼容参数，非空时作为关键词使用。
func PluginAnchorList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	keyword := ctx.URLParam("keyword")
	title := ctx.URLParam("title")
	if title != "" {
		keyword = title
	}

	linkList, total, err := currentSite.GetAnchorList(keyword, currentPage, pageSize)
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
		"data":  linkList,
	})
}

// PluginAnchorDetail 根据ID获取锚文本详情。
//
// 参数说明：
//   - 查询参数 "id": 锚文本ID。
func PluginAnchorDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	anchor, err := currentSite.GetAnchorById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": anchor,
	})
}

// PluginAnchorDetailForm 保存锚文本，支持新增和修改，标题或链接变更时异步更新已有内容中的锚文本。
func PluginAnchorDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginAnchor
	if !readJSONPayload(ctx, &req) {
		return
	}
	frontUrl := currentSite.System.BaseUrl
	if currentSite.System.FrontUrl != "" {
		frontUrl = currentSite.System.FrontUrl
	}
	req.Link = strings.TrimPrefix(req.Link, frontUrl)

	var anchor *model.Anchor
	var err error

	var changeTitle bool
	var changeLink bool

	if req.Id > 0 {
		anchor, err = currentSite.GetAnchorById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		//去重
		exists, err := currentSite.GetAnchorByTitle(req.Title)
		if err == nil && exists.Id != anchor.Id {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("AnchorTextKeywordAlreadyExists", req.Title),
			})
			return
		}
		//只有旧的才需要处理
		if anchor.Title != req.Title {
			changeTitle = true
		}
		if anchor.Link != req.Link {
			changeLink = true
		}

	} else {
		anchor, err = currentSite.GetAnchorByTitle(req.Title)
		if err != nil {
			//不存在，则创建它
			anchor = &model.Anchor{
				Status: 1,
			}
		}
	}

	anchor.Title = req.Title
	anchor.Link = req.Link
	anchor.Weight = req.Weight

	// 走 provider.SaveAnchor：它会顺手失效锚文本列表缓存。
	// 直接用 anchor.Save 的话，新加的锚文本要等缓存过期（1 小时）才参与替换。
	err = currentSite.SaveAnchor(anchor)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if changeTitle || changeLink {
		//锚文本名称更改了，不管连接有没有更改，都删掉旧的
		// 必须走 Async：后台任务不能挂请求 ctx，否则 handler 返回后
		// 所有 WithContext(w.Ctx()) 的查询都会 context canceled。
		go currentSite.Async().ChangeAnchor(anchor, changeTitle)
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyAnchorTextLog", anchor.Id, anchor.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LinkUpdated"),
		"data": anchor,
	})
}

// PluginAnchorReplace 执行锚文本批量替换任务，指定ID时替换单个锚文本，否则替换全部。
//
// 参数说明：
//   - 请求体 "id": 锚文本ID，为0时替换全部锚文本。
func PluginAnchorReplace(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginAnchorReplaceRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Id > 0 {
		//更新单个
		anchor, err := currentSite.GetAnchorById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		// 异步任务必须摘掉请求 ctx（见 Website.Async 的注释）：批量替换要遍历
		// 全站文章、跑几十秒，而 handler 一返回请求 ctx 就被 cancel，任务会在
		// 头几篇之后静默失败 —— 这就是「点了批量更新，只有很少文章变了」的成因。
		go currentSite.Async().ReplaceAnchor(anchor)
	} else {
		go currentSite.Async().ReplaceAnchor(nil)
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("PerformAnchorTextBatchReplacement"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("AnchorTextReplacementTaskHasBeenExecuted"),
	})
}

// PluginAnchorDelete 删除锚文本，支持按单个ID或批量ID列表删除。
//
// 参数说明：
//   - 请求体 "id": 单个锚文本ID，大于0时删除该条。
//   - 请求体 "ids": 锚文本ID列表，批量删除时使用。
func PluginAnchorDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginAnchorDelete
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !requireAnyID(ctx, req.Id, req.Ids) {
		return
	}

	if req.Id > 0 {
		//删一条
		anchor, err := currentSite.GetAnchorById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		err = currentSite.DeleteAnchor(anchor)
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
			anchor, err := currentSite.GetAnchorById(id)
			if err != nil {
				continue
			}

			_ = currentSite.DeleteAnchor(anchor)
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteAnchorTextLog", req.Id, req.Ids))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteOperationHasBeenPerformed"),
	})
}

// PluginAnchorExport 导出全部锚文本数据（标题、链接、权重）。
func PluginAnchorExport(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	anchors, err := currentSite.GetAllAnchors()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//header
	header := []string{"title", "link", "weight"}
	var content [][]interface{}
	//content
	for _, v := range anchors {
		content = append(content, []interface{}{v.Title, v.Link, v.Weight})
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ExportAnchorText"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"header":  header,
			"content": content,
		},
	})
}

// PluginAnchorImport 通过上传文件导入锚文本数据。
//
// 参数说明：
//   - 请求体 "file": 上传的锚文本文件。
func PluginAnchorImport(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()

	result, err := currentSite.ImportAnchors(file, info)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ImportAnchorTextLog", result))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("UploadCompleted"),
		"data": result,
	})
}

// PluginAnchorSetting 获取锚文本插件的配置信息。
func PluginAnchorSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	pluginAnchor := currentSite.PluginAnchor

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginAnchor,
	})
}

// PluginAnchorSettingForm 保存锚文本插件配置，包括锚文本密度、替换方式等。
func PluginAnchorSettingForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginAnchorConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 部分更新：以现值为基底，仅覆盖请求体中显式出现的字段，未传字段保持原配置。
	body, err := ctx.GetBody()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req = *currentSite.PluginAnchor
	if err := library.MergeJSONPayload(payload, &req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.AnchorDensity < 10 {
		req.AnchorDensity = 100
	}

	currentSite.PluginAnchor.AnchorDensity = req.AnchorDensity
	currentSite.PluginAnchor.ReplaceWay = req.ReplaceWay
	currentSite.PluginAnchor.KeywordWay = req.KeywordWay
	currentSite.PluginAnchor.NoStrongTag = req.NoStrongTag

	err = currentSite.SaveSettingValue(provider.AnchorSettingKey, currentSite.PluginAnchor)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyAnchorTextSetting"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// PluginAnchorAddFromTitle 批量将标题导入锚文本数据。支持文档/分类/标签。
func PluginAnchorAddFromTitle(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginAnchorAddFromTitle
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.InsertTitleToAnchor(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ImportAnchorTextLog", req))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("UploadCompleted"),
	})
}
