package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginTagList(ctx iris.Context) {
	title := ctx.URLParam("title")
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	tags, total, err := provider.GetTagList(0, title, "", currentPage, pageSize, 0)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 生成链接
	for i := range tags {
		tags[i].Link = provider.GetUrl("tag", tags[i], 0)
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  tags,
	})
}

func PluginTagDetail(ctx iris.Context) {
	id := ctx.Params().GetUintDefault("id", 0)

	tag, err := provider.GetTagById(id)
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
	var req request.PluginTag
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	tag, err := provider.SaveTag(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新文档标签：%d => %s", tag.Id, tag.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
		"data": tag,
	})
}

func PluginTagDelete(ctx iris.Context) {
	var req request.PluginTag
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	tag, err := provider.GetTagById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}


	err = provider.DeleteTag(tag.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("删除文档标签：%d => %s", tag.Id, tag.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "标签已删除",
	})
}

