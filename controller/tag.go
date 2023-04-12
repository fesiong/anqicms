package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func TagIndexPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		currentPage := ctx.Values().GetIntDefault("page", 1)
		webInfo.Title = currentSite.Lang("标签列表")
		if currentPage > 1 {
			webInfo.Title += " - " + fmt.Sprintf(currentSite.Lang("第%d页"), currentPage)
		}
		webInfo.PageName = "tagIndex"
		webInfo.CanonicalUrl = currentSite.GetUrl("tagIndex", nil, currentPage)
		ctx.ViewData("webInfo", webInfo)
	}

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
	currentSite := provider.CurrentSite(ctx)
	tagId := ctx.Params().GetUintDefault("id", 0)
	urlToken := ctx.Params().GetString("filename")
	var tag *model.Tag
	var err error
	if urlToken != "" {
		//优先使用urlToken
		tag, err = currentSite.GetTagByUrlToken(urlToken)
	} else {
		tag, err = currentSite.GetTagById(tagId)
	}
	if err != nil {
		NotFound(ctx)
		return
	}

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		currentPage := ctx.Values().GetIntDefault("page", 1)
		webInfo.Title = tag.Title
		if tag.SeoTitle != "" {
			webInfo.Title = tag.SeoTitle
		}
		if currentPage > 1 {
			webInfo.Title += " - " + fmt.Sprintf(currentSite.Lang("第%d页"), currentPage)
		}
		webInfo.Keywords = tag.Keywords
		webInfo.Description = tag.Description
		webInfo.NavBar = tag.Id
		webInfo.PageName = "tag"
		webInfo.CanonicalUrl = currentSite.GetUrl("tag", tag, currentPage)
		ctx.ViewData("webInfo", webInfo)
	}

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
