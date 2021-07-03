package controller

import (
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

	ctx.View(GetViewPath(ctx, "category/article.html"))
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

	ctx.View(GetViewPath(ctx, "category/product.html"))
}