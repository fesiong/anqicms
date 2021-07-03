package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

func ArticleDetail(ctx iris.Context) {
	id := ctx.Params().GetUintDefault("id", 0)
	urlToken := ctx.Params().GetString("filename")
	var article *model.Article
	var err error
	if urlToken != "" {
		//优先使用urlToken
		article, err = provider.GetArticleByUrlToken(urlToken)
	} else {
		article, err = provider.GetArticleById(id)
	}
	if err != nil {
		NotFound(ctx)
		return
	}

	_ = article.AddViews(config.DB)

	webInfo.Title = article.Title
	webInfo.Keywords = article.Keywords
	webInfo.Description = article.Description
	//设置页面名称，方便tags识别
	webInfo.PageName = "articleDetail"
	ctx.ViewData("webInfo", webInfo)
	ctx.ViewData("article", article)
	//设置页面名称，方便tags识别
	ctx.ViewData("pageName", "articleDetail")

	ctx.View(GetViewPath(ctx, "article/detail.html"))
}
