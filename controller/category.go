package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

func ArticleIndexPage(ctx iris.Context) {
	CategoryArticlePage(ctx)
}

func ProductIndexPage(ctx iris.Context) {
	CategoryProductPage(ctx)
}

func CategoryPage(ctx iris.Context) {
	categoryId := ctx.Params().GetUintDefault("id", 0)
	urlToken := ctx.Params().GetString("filename")
	var category *model.Category
	var err error
	if urlToken != "" {
		//优先使用urlToken
		category, err = provider.GetCategoryByUrlToken(urlToken)
	} else {
		category, err = provider.GetCategoryById(categoryId)
	}
	if err != nil {
		NotFound(ctx)
		return
	}

	//修正，如果这里读到的的page，则跳到page中
	if category.Type == model.CategoryTypePage {
		ctx.StatusCode(301)
		ctx.Redirect(provider.GetUrl("page", category, 0))
		return
	}

	ctx.Values().Set("category", category)
	if category.Type == model.CategoryTypeArticle {
		CategoryArticlePage(ctx)
	} else if category.Type == model.CategoryTypeProduct {
		CategoryProductPage(ctx)
	}
}

func CategoryArticlePage(ctx iris.Context) {
	var category *model.Category
	categoryVal := ctx.Values().Get("category")

	if categoryVal != nil {
		category, _ = categoryVal.(*model.Category)
	}

	if category != nil {
		webInfo.Title = category.Title
		webInfo.Description = category.Description
		webInfo.NavBar = category.Id
		webInfo.PageName = "articleList"
	} else {
		webInfo.Title = config.JsonData.Index.SeoTitle
		webInfo.Keywords = config.JsonData.Index.SeoKeywords
		webInfo.Description = config.JsonData.Index.SeoDescription
		webInfo.PageName = "articleIndex"
	}
	ctx.ViewData("webInfo", webInfo)

	ctx.ViewData("category", category)

	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板
	tplName := "category/article.html"
	if category != nil {
		if category.Template != "" {
			tplName = category.Template
		} else if ViewExists(ctx, fmt.Sprintf("category/article-%d.html", category.Id)) {
			tplName = fmt.Sprintf("category/article-%d.html", category.Id)
		} else {
			categoryTemplate := provider.GetCategoryTemplate(category)
			if categoryTemplate != nil {
				tplName = categoryTemplate.Template
			}
		}
	}

	ctx.View(GetViewPath(ctx, tplName))
}

func CategoryProductPage(ctx iris.Context) {

	var category *model.Category
	categoryVal := ctx.Values().Get("category")

	if categoryVal != nil {
		category, _ = categoryVal.(*model.Category)
	}

	if category != nil {
		webInfo.Title = category.Title
		webInfo.Description = category.Description
		webInfo.NavBar = category.Id
		webInfo.PageName = "productList"
	} else {
		webInfo.Title = config.JsonData.Index.SeoTitle
		webInfo.Keywords = config.JsonData.Index.SeoKeywords
		webInfo.Description = config.JsonData.Index.SeoDescription
		webInfo.PageName = "productIndex"
	}
	ctx.ViewData("webInfo", webInfo)

	ctx.ViewData("category", category)

	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板
	tplName := "category/product.html"
	if category != nil {
		if category.Template != "" {
			tplName = category.Template
		} else if ViewExists(ctx, fmt.Sprintf("category/product-%d.html", category.Id)) {
			tplName = fmt.Sprintf("category/product-%d.html", category.Id)
		} else {
			categoryTemplate := provider.GetCategoryTemplate(category)
			if categoryTemplate != nil {
				tplName = categoryTemplate.Template
			}
		}
	}

	ctx.View(GetViewPath(ctx, tplName))
}