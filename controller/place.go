package controller

import (
	"fmt"
	"strings"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func PlacePage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if !currentSite.PluginPlace.Open {
		NotFound(ctx)
		return
	}
	cacheFile, ok := currentSite.LoadCachedHtml(ctx)
	if ok {
		ctx.ContentType(context.ContentHTMLHeaderValue)
		ctx.Write(cacheFile)
		return
	}

	urlToken := ctx.Params().GetString("filename")

	var place *model.Place
	place = currentSite.GetPlaceFromCacheByToken(urlToken)
	if place == nil {
		NotFound(ctx)
		return
	}

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = place.Title
		if place.SeoTitle != "" {
			webInfo.Title = place.SeoTitle
		}
		webInfo.Keywords = place.Keywords
		webInfo.Description = place.Description
		webInfo.NavBar = int64(place.Id)
		webInfo.PageName = "placeDetail"
		webInfo.CanonicalUrl = place.Link
		ctx.ViewData("webInfo", webInfo)
	}

	ctx.ViewData("place", place)

	tplName := "place/detail.html"
	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板，如果发现上一级不继承，则不需要处理
	tmpName := fmt.Sprintf("%s/detail-%d.html", place.Id)
	if place.Template != "" {
		tplName = place.Template
	} else if ViewExists(ctx, tmpName) {
		tplName = tmpName
	} else {
		placeTemplate := currentSite.GetPlaceTemplate(place)
		if placeTemplate != "" {
			tplName = placeTemplate
		}
	}
	if !strings.HasSuffix(tplName, ".html") {
		tplName += ".html"
	}

	recorder := ctx.Recorder()
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	} else {
		if currentSite.PluginHtmlCache.Open && currentSite.PluginHtmlCache.ListCache > 0 {
			mobileTemplate := ctx.Values().GetBoolDefault("mobileTemplate", false)
			_ = currentSite.CacheHtmlData(ctx.RequestPath(false), ctx.Request().URL.RawQuery, mobileTemplate, recorder.Body())
		}
	}
}

func PlaceIndex(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if !currentSite.PluginPlace.Open {
		NotFound(ctx)
		return
	}
	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = currentSite.TplTr("PlaceIndex")
		webInfo.PageName = "placeIndex"
		webInfo.CanonicalUrl = currentSite.GetUrl("placeIndex", nil, 0)
		ctx.ViewData("webInfo", webInfo)
	}

	tplName := "place/index.html"

	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}
