package controller

import (
	"fmt"
	"strings"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func PagePage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	cacheFile, ok := currentSite.LoadCachedHtml(ctx)
	if ok {
		ctx.ContentType(context.ContentHTMLHeaderValue)
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
		category = currentSite.GetCategoryFromCacheByToken(urlToken)
	} else {
		category = currentSite.GetCategoryFromCache(categoryId)
	}
	if category == nil || category.Status != config.ContentStatusOK {
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
		webInfo.NavBar = int64(category.Id)
		webInfo.PageName = "pageDetail"
		webInfo.CanonicalUrl = currentSite.GetUrl("page", category, 0)
		ctx.ViewData("webInfo", webInfo)
	}
	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板
	tmpTpl := fmt.Sprintf("page/detail-%d.html", category.Id)
	categoryTemplate := currentSite.GetCategoryTemplate(category)
	var catTpl string
	if categoryTemplate != nil {
		catTpl = categoryTemplate.Template
		if !strings.HasSuffix(catTpl, ".html") {
			catTpl += ".html"
		}
	}
	tokenTpl := fmt.Sprintf("page/%s.html", category.UrlToken)

	tplName, ok := currentSite.TemplateExist(catTpl, tokenTpl, tmpTpl, fmt.Sprintf("page-%d.html", category.Id), "page/detail.html", "page_detail.html")
	if !ok {
		NotFound(ctx)
		return
	}
	recorder := ctx.Recorder()
	err = ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	} else {
		if currentSite.PluginHtmlCache.Open && currentSite.PluginHtmlCache.DetailCache > 0 {
			mobileTemplate := ctx.Values().GetBoolDefault("mobileTemplate", false)
			_ = currentSite.CacheHtmlData(ctx.RequestPath(false), ctx.Request().URL.RawQuery, mobileTemplate, recorder.Body())
		}
	}
}
