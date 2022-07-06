package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"strings"
)

func CategoryPage(ctx iris.Context) {
	currentPage := ctx.Values().GetIntDefault("page", 1)
	categoryId := ctx.Params().GetUintDefault("id", 0)
	catId := ctx.Params().GetUintDefault("catid", 0)
	if catId > 0 {
		categoryId = catId
	}
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
	if category.Type == config.CategoryTypePage {
		ctx.StatusCode(301)
		ctx.Redirect(provider.GetUrl("page", category, 0))
		return
	}

	module := provider.GetModuleFromCache(category.ModuleId)
	if module == nil {
		ShowMessage(ctx, config.Lang("未定义模型"), "")
		return
	}

	webInfo.Title = category.Title
	if category.SeoTitle != "" {
		webInfo.Title = category.SeoTitle
	}
	webInfo.Keywords = category.Keywords
	webInfo.Description = category.Description
	webInfo.NavBar = category.Id
	webInfo.PageName = "archiveList"
	webInfo.CanonicalUrl = provider.GetUrl("category", category, currentPage)

	ctx.ViewData("webInfo", webInfo)

	ctx.ViewData("category", category)

	tplName := module.TableName + "/list.html"
	tplName2 := module.TableName + "_list.html"
	if ViewExists(ctx, tplName2) {
		tplName = tplName2
	}
	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板，如果发现上一级不继承，则不需要处理
	if category.Template != "" {
		tplName = category.Template
	} else if ViewExists(ctx, fmt.Sprintf("%s/list-%d.html", module.TableName, category.Id)) {
		tplName = fmt.Sprintf("%s/list-%d.html", module.TableName, category.Id)
	} else {
		categoryTemplate := provider.GetCategoryTemplate(category)
		if categoryTemplate != nil {
			tplName = categoryTemplate.Template
		}
	}
	if !strings.HasSuffix(tplName, ".html") {
		tplName += ".html"
	}

	err = ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func SearchPage(ctx iris.Context) {
	q := ctx.URLParam("q")
	webInfo.Title = fmt.Sprintf("搜索: %s", q)
	webInfo.PageName = "search"
	currentPage := ctx.Values().GetIntDefault("page", 1)
	webInfo.CanonicalUrl = provider.GetUrl(fmt.Sprintf("/search?q=%s(&page={page})", q), nil, currentPage)
	ctx.ViewData("webInfo", webInfo)
	ctx.ViewData("q", q)

	tplName := "search/index.html"
	if ViewExists(ctx, "search.html") {
		tplName = "search.html"
	}
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}
