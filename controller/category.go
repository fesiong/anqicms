package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
	"math"
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
	currentPage := ctx.URLParamIntDefault("page", 1)
	paramPage := ctx.Params().GetIntDefault("page", 0)
	if paramPage > 0 {
		currentPage = paramPage
	}

	//一页显示10条
	pageSize := 10
	categoryId := uint(0)
	var category *model.Category
	categoryVal := ctx.Values().Get("category")

	if categoryVal == nil {
		categoryId = 0
	} else {
		category, _ = categoryVal.(*model.Category)
		categoryId = category.Id
	}


	//文章列表
	articles, total, _ := provider.GetArticleList(categoryId, "id desc", currentPage, pageSize)
	//读取列表的分类
	articleCategories, _ := provider.GetCategories(model.CategoryTypeArticle)
	for i, v := range articles {
		if v.CategoryId > 0 {
			for _, c := range articleCategories {
				if c.Id == v.CategoryId {
					articles[i].Category = c
				}
			}
		}
	}
	//热门文章
	populars, _, _ := provider.GetArticleList(categoryId, "views desc", 1, 10)

	totalPage := math.Ceil(float64(total)/float64(pageSize))

	prevPage := ""
	nextPage := ""
	urlMatch := "category"
	if category == nil {
		urlMatch = "articleIndex"
	}
	if currentPage > 1 {
		prevPage = provider.GetUrl(urlMatch, category, currentPage-1)
	}

	if currentPage < int(totalPage) {
		nextPage = provider.GetUrl(urlMatch, category, currentPage+1)
	}

	if category != nil {
		webInfo.Title = category.Title
		webInfo.Description = category.Description
		webInfo.NavBar = category.Id
	} else {
		webInfo.Title = config.JsonData.Index.SeoTitle
		webInfo.Keywords = config.JsonData.Index.SeoKeywords
		webInfo.Description = config.JsonData.Index.SeoDescription
	}
	ctx.ViewData("webInfo", webInfo)

	//首页显示友情链接
	links, _ := provider.GetLinkList()

	ctx.ViewData("total", total)
	ctx.ViewData("articles", articles)
	ctx.ViewData("populars", populars)
	ctx.ViewData("totalPage", totalPage)
	ctx.ViewData("prevPage", prevPage)
	ctx.ViewData("nextPage", nextPage)
	ctx.ViewData("category", category)
	ctx.ViewData("links", links)

	ctx.View(GetViewPath(ctx, "category/article.html"))
}

func CategoryProductPage(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("page", 1)
	paramPage := ctx.Params().GetIntDefault("page", 0)
	if paramPage > 0 {
		currentPage = paramPage
	}
	//一页显示10条
	pageSize := 10
	categoryId := uint(0)
	var category *model.Category
	categoryVal := ctx.Values().Get("category")

	if categoryVal == nil {
		categoryId = 0
	} else {
		category, _ = categoryVal.(*model.Category)
		categoryId = category.Id
	}


	//产品列表
	products, total, _ := provider.GetProductList(categoryId, "id desc", currentPage, pageSize)
	//读取列表的分类
	productCategories, _ := provider.GetCategories(model.CategoryTypeProduct)
	for i, v := range products {
		if v.CategoryId > 0 {
			for _, c := range productCategories {
				if c.Id == v.CategoryId {
					products[i].Category = c
				}
			}
		}
	}
	//热门产品
	populars, _, _ := provider.GetProductList(categoryId, "views desc", 1, 10)

	totalPage := math.Ceil(float64(total)/float64(pageSize))

	prevPage := ""
	nextPage := ""
	urlMatch := "category"
	if category == nil {
		urlMatch = "productIndex"
	}
	if currentPage > 1 {
		prevPage = provider.GetUrl(urlMatch, category, currentPage-1)
	}

	if currentPage < int(totalPage) {
		nextPage = provider.GetUrl(urlMatch, category, currentPage+1)
	}

	if category != nil {
		webInfo.Title = category.Title
		webInfo.Description = category.Description
		webInfo.NavBar = category.Id
	} else {
		webInfo.Title = config.JsonData.Index.SeoTitle
		webInfo.Keywords = config.JsonData.Index.SeoKeywords
		webInfo.Description = config.JsonData.Index.SeoDescription
	}
	ctx.ViewData("webInfo", webInfo)

	//首页显示友情链接
	links, _ := provider.GetLinkList()

	ctx.ViewData("total", total)
	ctx.ViewData("products", products)
	ctx.ViewData("populars", populars)
	ctx.ViewData("totalPage", totalPage)
	ctx.ViewData("prevPage", prevPage)
	ctx.ViewData("nextPage", nextPage)
	ctx.ViewData("category", category)
	ctx.ViewData("links", links)

	ctx.View(GetViewPath(ctx, "category/product.html"))
}