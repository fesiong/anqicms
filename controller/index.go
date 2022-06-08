package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
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
	webInfo.CanonicalUrl = provider.GetUrl("", nil, 0)
	ctx.ViewData("webInfo", webInfo)

	ctx.ViewData("category", category)

	// 支持2种文件结构，一种是目录式的，一种是扁平式的
	tplName := "index/index.html"
	if ViewExists(ctx, "index.html") {
		tplName = "index.html"
	}
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}
