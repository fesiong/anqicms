package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func TagIndexPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	cacheFile, ok := currentSite.LoadCachedHtml(ctx)
	if ok {
		ctx.ServeFile(cacheFile)
		return
	}
	currentPage := ctx.Values().GetIntDefault("page", 1)
	if currentPage > currentSite.Content.MaxPage {
		// 最大1000页
		NotFound(ctx)
		return
	}
	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = currentSite.TplTr("TagList")
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "tagIndex"
		webInfo.CanonicalUrl = currentSite.GetUrl("tagIndex", nil, currentPage)
		ctx.ViewData("webInfo", webInfo)
	}

	tplName := "tag/index.html"
	if ViewExists(ctx, "tag_index.html") {
		tplName = "tag_index.html"
	}
	recorder := ctx.Recorder()
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	} else {
		if currentSite.PluginHtmlCache.Open && currentSite.PluginHtmlCache.ListCache > 0 {
			mobileTemplate := ctx.Values().GetBoolDefault("mobileTemplate", false)
			_ = currentSite.CacheHtmlData(ctx.RequestPath(false), ctx.Request().URL.RawQuery, mobileTemplate, recorder.Body())
		}
	}
}

func TagPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	cacheFile, ok := currentSite.LoadCachedHtml(ctx)
	if ok {
		ctx.ServeFile(cacheFile)
		return
	}
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
	currentPage := ctx.Values().GetIntDefault("page", 1)
	if currentPage > currentSite.Content.MaxPage {
		// 最大1000页
		NotFound(ctx)
		return
	}

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = tag.Title
		if tag.SeoTitle != "" {
			webInfo.Title = tag.SeoTitle
		}
		webInfo.CurrentPage = currentPage
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
	recorder := ctx.Recorder()
	err = ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	} else {
		if currentSite.PluginHtmlCache.Open && currentSite.PluginHtmlCache.ListCache > 0 {
			mobileTemplate := ctx.Values().GetBoolDefault("mobileTemplate", false)
			_ = currentSite.CacheHtmlData(ctx.RequestPath(false), ctx.Request().URL.RawQuery, mobileTemplate, recorder.Body())
		}
	}
}
