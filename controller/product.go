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
	//最新
	newest, _, _ := provider.GetProductList(product.CategoryId, "id desc", 1, 10)
	//相邻相关产品
	relationList, _ := provider.GetRelationProductList(product.CategoryId, id, 10)
	//获取上一篇
	prev, _ := provider.GetPrevProductById(product.CategoryId, id)
	//获取下一篇
	next, _ := provider.GetNextProductById(product.CategoryId, id)
	//获取评论内容
	comments, _, _ := provider.GetCommentList(model.ItemTypeProduct, product.Id, "id desc", 1, 10)

	webInfo.Title = product.Title
	webInfo.Keywords = product.Keywords
	webInfo.Description = product.Description
	ctx.ViewData("webInfo", webInfo)
	ctx.ViewData("product", product)
	ctx.ViewData("newest", newest)
	ctx.ViewData("relationList", relationList)
	ctx.ViewData("prev", prev)
	ctx.ViewData("next", next)
	ctx.ViewData("comments", comments)

	ctx.View(GetViewPath(ctx, "product/detail.html"))
}