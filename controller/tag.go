package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

func TagIndexPage(ctx iris.Context) {
	webInfo.Title = config.Lang("标签列表")
	webInfo.PageName = "tagIndex"
	currentPage := ctx.Values().GetIntDefault("page", 1)
	webInfo.CanonicalUrl = provider.GetUrl("tagIndex", nil, currentPage)
	ctx.ViewData("webInfo", webInfo)

	tplName := "tag/index.html"
	if ViewExists(ctx, "tag_index.html") {
		tplName = "tag_index.html"
	}
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func TagPage(ctx iris.Context) {
	tagId := ctx.Params().GetUintDefault("id", 0)
	urlToken := ctx.Params().GetString("filename")
	var tag *model.Tag
	var err error
	if urlToken != "" {
		//优先使用urlToken
		tag, err = provider.GetTagByUrlToken(urlToken)
	} else {
		tag, err = provider.GetTagById(tagId)
	}
	if err != nil {
		NotFound(ctx)
		return
	}

	webInfo.Title = tag.Title
	if tag.SeoTitle != "" {
		webInfo.Title = tag.SeoTitle
	}
	webInfo.Keywords = tag.Keywords
	webInfo.Description = tag.Description
	webInfo.NavBar = tag.Id
	webInfo.PageName = "tag"
	currentPage := ctx.Values().GetIntDefault("page", 1)
	webInfo.CanonicalUrl = provider.GetUrl("tag", tag, currentPage)
	ctx.ViewData("webInfo", webInfo)

	ctx.ViewData("tag", tag)

	var tplName string

	tplName = "tag/list.html"
	if ViewExists(ctx, "tag_list.html") {
		tplName = "tag_list.html"
	}

	err = ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}
