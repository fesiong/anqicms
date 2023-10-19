package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"strings"
)

func PagePage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	cacheFile, ok := currentSite.LoadCachedHtml(ctx)
	if ok {
		ctx.ServeFile(cacheFile)
		return
	}
	categoryId := ctx.Params().GetUintDefault("id", 0)
	urlToken := ctx.Params().GetString("filename")
	catId := ctx.Params().GetUintDefault("catid", 0)
	if catId > 0 {
		categoryId = catId
	}
	var category *model.Category
	var err error
	if urlToken != "" {
		//优先使用urlToken
		category, err = currentSite.GetCategoryByUrlToken(urlToken)
	} else {
		category, err = currentSite.GetCategoryById(categoryId)
	}
	if err != nil || category.Status != config.ContentStatusOK {
		NotFound(ctx)
		return
	}

	//修正，如果这里读到的的category，则跳到category中
	if category.Type != config.CategoryTypePage {
		ctx.StatusCode(301)
		ctx.Redirect(currentSite.GetUrl("category", category, 0))
		return
	}

	ctx.ViewData("page", category)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = category.Title
		if category.SeoTitle != "" {
			webInfo.Title = category.SeoTitle
		}
		webInfo.Keywords = category.Keywords
		webInfo.Description = category.Description
		webInfo.NavBar = category.Id
		webInfo.PageName = "pageDetail"
		webInfo.CanonicalUrl = currentSite.GetUrl("page", category, 0)
		ctx.ViewData("webInfo", webInfo)
	}
	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板
	tplName := "page/detail.html"
	if ViewExists(ctx, "page_detail.html") {
		tplName = "page_detail.html"
	}
	tmpTpl := fmt.Sprintf("page/detail-%d.html", category.Id)
	if ViewExists(ctx, tmpTpl) {
		tplName = tmpTpl
	} else if ViewExists(ctx, fmt.Sprintf("page-%d.html", category.Id)) {
		tplName = fmt.Sprintf("page-%d.html", category.Id)
	} else {
		categoryTemplate := currentSite.GetCategoryTemplate(category)
		if categoryTemplate != nil {
			tplName = categoryTemplate.Template
		}
	}
	if !strings.HasSuffix(tplName, ".html") {
		tplName += ".html"
	}
	recorder := ctx.Recorder()
	err = ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	} else {
		if currentSite.PluginHtmlCache.Open && currentSite.PluginHtmlCache.IndexCache > 0 {
			_ = currentSite.CacheHtmlData(ctx.RequestPath(false), ctx.Request().URL.RawQuery, ctx.IsMobile(), recorder.Body())
		}
	}
}
