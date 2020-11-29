package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"irisweb/model"
	"irisweb/provider"
	"math"
	"strings"
)

func IndexPage(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("page", 1)
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))
	//一页显示20条
	pageSize := 20
	//文章列表
	articles, total, _ := provider.GetArticleList(categoryId, "id desc", 1, 20)
	//读取列表的分类
	categories, _ := provider.GetCategories()
	for i, v := range articles {
		if v.CategoryId > 0 {
			for _, c := range categories {
				if c.Id == v.CategoryId {
					articles[i].Category = *c
				}
			}
		}
	}
	//热门文章
	populars, _, _ := provider.GetArticleList(categoryId, "views desc", 1, 10)

	totalPage := math.Ceil(float64(total)/float64(pageSize))

	prevPage := ""
	nextPage := ""
	urlPfx := "/policy?"
	var category *model.Category
	if categoryId > 0 {
		urlPfx += fmt.Sprintf("category_id=%d&", categoryId)
		category, _ = provider.GetCategoryById(categoryId)
	}
	if currentPage > 1 {
		prevPage = fmt.Sprintf("%spage=%d", urlPfx, currentPage-1)
	}

	if currentPage < int(totalPage) {
		nextPage = fmt.Sprintf("%spage=%d", urlPfx, currentPage+1)
	}
	if currentPage == 2 {
		prevPage = strings.TrimRight(prevPage, "page=1")
	}
	webTitle := ""
	if category != nil {
		webTitle = category.Title
		webInfo.NavBar = category.Id
	}
	webInfo.Title = webTitle
	ctx.ViewData("webInfo", webInfo)

	ctx.ViewData("total", total)
	ctx.ViewData("articles", articles)
	ctx.ViewData("populars", populars)
	ctx.ViewData("totalPage", totalPage)
	ctx.ViewData("prevPage", prevPage)
	ctx.ViewData("nextPage", nextPage)
	ctx.ViewData("category", category)

	ctx.View("index.html")
}
