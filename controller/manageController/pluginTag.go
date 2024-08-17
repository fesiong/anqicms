package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginTagList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	title := ctx.URLParam("title")
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	tags, total, err := currentSite.GetTagList(0, title, "", currentPage, pageSize, 0, "id desc")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 生成链接
	for i := range tags {
		tags[i].Link = currentSite.GetUrl("tag", tags[i], 0)
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  tags,
	})
}

func PluginTagDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.Params().GetUintDefault("id", 0)

	tag, err := currentSite.GetTagById(id)
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
		"data": tag,
	})
}

func PluginTagDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginTag
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	tag, err := currentSite.SaveTag(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 更新缓存
	go func() {
		currentSite.BuildTagIndexCache(ctx)
		currentSite.BuildSingleTagCache(ctx, tag)
		// 上传到静态服务器
		_ = currentSite.SyncHtmlCacheToStorage("", "")
	}()

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateDocumentTagLog", tag.Id, tag.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": tag,
	})
}

func PluginTagDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginTag
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	tag, err := currentSite.GetTagById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.DeleteTag(tag.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteDocumentTagLog", tag.Id, tag.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("TagDeleted"),
	})
}
