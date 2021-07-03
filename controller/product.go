package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

func ProductDetail(ctx iris.Context) {
	id := ctx.Params().GetUintDefault("id", 0)
	urlToken := ctx.Params().GetString("filename")
	var product *model.Product
	var err error
	if urlToken != "" {
		//优先使用urlToken
		product, err = provider.GetProductByUrlToken(urlToken)
	} else {
		product, err = provider.GetProductById(id)
	}
	if err != nil {
		NotFound(ctx)
		return
	}
	_ = product.AddViews(config.DB)

	webInfo.Title = product.Title
	webInfo.Keywords = product.Keywords
	webInfo.Description = product.Description
	webInfo.PageName = "productDetail"

	ctx.ViewData("webInfo", webInfo)
	ctx.ViewData("product", product)

	ctx.View(GetViewPath(ctx, "product/detail.html"))
}