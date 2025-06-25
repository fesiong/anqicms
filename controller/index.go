package controller

import (
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func IndexPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	cacheFile, ok := currentSite.LoadCachedHtml(ctx)
	if ok {
		ctx.ContentType(context.ContentHTMLHeaderValue)
		ctx.ServeFile(cacheFile)
		return
	}
	currentPage := ctx.Values().GetIntDefault("page", 1)
	if currentPage > currentSite.Content.MaxPage {
		// 最大1000页
		NotFound(ctx)
		return
	}
	webTitle := currentSite.Index.SeoTitle
	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = webTitle
		webInfo.Keywords = currentSite.Index.SeoKeywords
		webInfo.Description = currentSite.Index.SeoDescription
		//设置页面名称，方便tags识别
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "index"
		webInfo.CanonicalUrl = currentSite.GetUrl("", nil, 0)
		ctx.ViewData("webInfo", webInfo)
	}

	// 支持2种文件结构，一种是目录式的，一种是扁平式的
	tplName, ok := currentSite.TemplateExist("index/index.html", "index.html")
	if !ok {
		NotFound(ctx)
		return
	}
	recorder := ctx.Recorder()
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	} else {
		if currentSite.PluginHtmlCache.Open && currentSite.PluginHtmlCache.IndexCache > 0 {
			mobileTemplate := ctx.Values().GetBoolDefault("mobileTemplate", false)
			_ = currentSite.CacheHtmlData(ctx.RequestPath(false), ctx.Request().URL.RawQuery, mobileTemplate, recorder.Body())
		}
	}
}
