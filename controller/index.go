package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"log"
)

func IndexPage(ctx iris.Context) {
	currentPage := ctx.Values().GetIntDefault("page", 1)
	// 只缓存首页
	if currentPage == 1 {
		body := provider.GetIndexCache()
		if body != nil {
			log.Println("Load index from cache.")
			ctx.Write(body)
			return
		}
	}

	webTitle := config.JsonData.Index.SeoTitle
	webInfo.Title = webTitle
	webInfo.Keywords = config.JsonData.Index.SeoKeywords
	webInfo.Description = config.JsonData.Index.SeoDescription
	//设置页面名称，方便tags识别
	webInfo.PageName = "index"
	webInfo.CanonicalUrl = provider.GetUrl("", nil, 0)
	ctx.ViewData("webInfo", webInfo)

	// 支持2种文件结构，一种是目录式的，一种是扁平式的
	tplName := "index/index.html"
	if ViewExists(ctx, "index.html") {
		tplName = "index.html"
	}

	recorder := ctx.Recorder()
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	} else if currentPage == 1 {
		provider.CacheIndex(recorder.Body())
	}
}
