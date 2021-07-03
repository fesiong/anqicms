package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

func IndexPage(ctx iris.Context) {
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))

	var category *model.Category
	if categoryId > 0 {
		category, _ = provider.GetCategoryById(categoryId)
	}

	webTitle := config.JsonData.Index.SeoTitle
	if category != nil {
		webTitle += "_" + category.Title
		webInfo.NavBar = category.Id
	}
	webInfo.Title = webTitle
	webInfo.Keywords = config.JsonData.Index.SeoKeywords
	webInfo.Description = config.JsonData.Index.SeoDescription
	//设置页面名称，方便tags识别
	webInfo.PageName = "index"
	ctx.ViewData("webInfo", webInfo)

	ctx.ViewData("category", category)

	ctx.View(GetViewPath(ctx, "index/index.html"))
}
