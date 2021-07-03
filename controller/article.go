package controller

import (
	"fmt"
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

	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板
	tplName := "article/detail.html"
	if article.Category != nil {
		tmpTpl := fmt.Sprintf("article/detail-%d.html", article.Id)
		if ViewExists(ctx, tmpTpl) {
			tplName = tmpTpl
		} else {
			categoryTemplate := provider.GetCategoryTemplate(article.Category)
			if categoryTemplate != nil {
				tplName = categoryTemplate.Template
			}
		}
	}

	ctx.View(GetViewPath(ctx, tplName))
}
