package controller

import (
    "github.com/kataras/iris/v12"
    "irisweb/model"
    "irisweb/provider"
)

func PagePage(ctx iris.Context) {
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

    //修正，如果这里读到的的category，则跳到category中
    if category.Type != model.CategoryTypePage {
        ctx.StatusCode(301)
        ctx.Redirect(provider.GetUrl("category", category, 0))
        return
    }

    ctx.ViewData("page", category)

    //列出所有的page
    allPages, _ := provider.GetCategories(model.CategoryTypePage)
    ctx.ViewData("allPages", allPages)

    ctx.View(GetViewPath(ctx, "page/detail.html"))
}