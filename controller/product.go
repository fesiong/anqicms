package controller

import (
	"fmt"
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

	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板
	tplName := "product/detail.html"
	if product.Category != nil {
		tmpTpl := fmt.Sprintf("product/detail-%d.html", product.Id)
		if ViewExists(ctx, tmpTpl) {
			tplName = tmpTpl
		} else {
			categoryTemplate := provider.GetCategoryTemplate(product.Category)
			if categoryTemplate != nil {
				tplName = categoryTemplate.Template
			}
		}
	}

	ctx.View(GetViewPath(ctx, tplName))
}