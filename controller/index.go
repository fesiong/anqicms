package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func IndexPage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	cacheFile, ok := currentSite.LoadCachedHtml(ctx)
	if ok {
		ctx.ServeFile(cacheFile)
		return
	}
	currentPage := ctx.Values().GetIntDefault("page", 1)
	webTitle := currentSite.Index.SeoTitle
	if currentPage > 1 {
		webTitle += " - " + currentSite.TplTr("PageNum", currentPage)
	}

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = webTitle
		webInfo.Keywords = currentSite.Index.SeoKeywords
		webInfo.Description = currentSite.Index.SeoDescription
		//设置页面名称，方便tags识别
		webInfo.PageName = "index"
		webInfo.CanonicalUrl = currentSite.GetUrl("", nil, 0)
		ctx.ViewData("webInfo", webInfo)
	}

	//八字查询
	lastHoroscopes, _, _ := currentSite.GetHoroscopeList(1, 12, "")
	ctx.ViewData("lastHoroscopes", lastHoroscopes)

	lastDetails, _, _ := currentSite.GetNameDetailList(1, 22, "")
	ctx.ViewData("lastDetails", lastDetails)

	// 支持2种文件结构，一种是目录式的，一种是扁平式的
	tplName := "index/index.html"
	if ViewExists(ctx, "index.html") {
		tplName = "index.html"
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
