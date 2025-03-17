package controller

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func CategoryPage(ctx iris.Context) {
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
	categoryId := ctx.Params().GetUintDefault("id", 0)
	catId := ctx.Params().GetUintDefault("catid", 0)
	if catId > 0 {
		categoryId = catId
	}
	var category *model.Category
	var err error
	urlToken := ctx.Params().GetString("filename")
	multiCatNames := ctx.Params().GetString("multicatname")
	if multiCatNames != "" {
		chunkCatNames := strings.Split(multiCatNames, "/")
		urlToken = chunkCatNames[len(chunkCatNames)-1]
		for _, catName := range chunkCatNames {
			tmpCat := currentSite.GetCategoryFromCacheByToken(catName, category)
			if tmpCat == nil || (category != nil && tmpCat.ParentId != category.Id) {
				NotFound(ctx)
				return
			}
			category = tmpCat
		}
	} else {
		if urlToken != "" {
			//优先使用urlToken
			category = currentSite.GetCategoryFromCacheByToken(urlToken)
		} else {
			category = currentSite.GetCategoryFromCache(categoryId)
		}
	}
	if category == nil || category.Status != config.ContentStatusOK {
		NotFound(ctx)
		return
	}

	//修正，如果这里读到的的page，则跳到page中
	if category.Type == config.CategoryTypePage {
		ctx.StatusCode(301)
		ctx.Redirect(currentSite.GetUrl("page", category, 0))
		return
	}

	module := currentSite.GetModuleFromCache(category.ModuleId)
	if module == nil {
		ctx.StatusCode(404)
		ShowMessage(ctx, currentSite.TplTr("UndefinedModel"), nil)
		return
	}

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = category.Title
		if category.SeoTitle != "" {
			webInfo.Title = category.SeoTitle
		}
		webInfo.CurrentPage = currentPage
		webInfo.Keywords = category.Keywords
		webInfo.Description = category.Description
		webInfo.NavBar = int64(category.Id)
		webInfo.PageName = "archiveList"
		webInfo.CanonicalUrl = currentSite.GetUrl("category", category, currentPage)
		ctx.ViewData("webInfo", webInfo)
	}

	ctx.ViewData("category", category)

	tmpTpl := fmt.Sprintf("%s/list-%d.html", module.TableName, category.Id)
	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板，如果发现上一级不继承，则不需要处理
	var catTpl string
	categoryTemplate := currentSite.GetCategoryTemplate(category)
	if categoryTemplate != nil && len(categoryTemplate.Template) > 0 {
		catTpl = categoryTemplate.Template
		if !strings.HasSuffix(catTpl, ".html") {
			catTpl += ".html"
		}
	}
	tokenTpl := fmt.Sprintf("%s/%s.html", module.TableName, category.UrlToken)
	tplName, ok := currentSite.TemplateExist(catTpl, tokenTpl, tmpTpl, module.TableName+"/list.html", module.TableName+"_list.html")
	if !ok {
		NotFound(ctx)
		return
	}
	recorder := ctx.Recorder()
	err = ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	} else {
		if currentSite.PluginHtmlCache.Open && currentSite.PluginHtmlCache.ListCache > 0 {
			mobileTemplate := ctx.Values().GetBoolDefault("mobileTemplate", false)
			_ = currentSite.CacheHtmlData(ctx.RequestPath(false), ctx.Request().URL.RawQuery, mobileTemplate, recorder.Body())
		}
	}
}

func SearchPage(ctx iris.Context) {
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
	q := strings.TrimSpace(ctx.URLParam("q"))
	moduleToken := ctx.Params().GetString("module")
	var module *model.Module
	if len(moduleToken) > 0 {
		module = currentSite.GetModuleFromCacheByToken(moduleToken)
		if module == nil {
			ctx.StatusCode(404)
			ShowMessage(ctx, currentSite.TplTr("UndefinedModel"), nil)
			return
		}
		ctx.ViewData("module", module)
	}
	if currentSite.Safe.ContentForbidden != "" {
		forbiddens := strings.Split(currentSite.Safe.ContentForbidden, "\n")
		for _, v := range forbiddens {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if strings.Contains(q, v) {
				ShowMessage(ctx, currentSite.TplTr("TheKeywordYouSearchedContainsCharactersThatAreNotAllowed"), nil)
				return
			}
		}
	}

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = currentSite.TplTr("SearchLog", "")
		if module != nil {
			webInfo.Title = module.Title + webInfo.Title
		}
		webInfo.CurrentPage = currentPage
		webInfo.PageName = "search"
		webInfo.CanonicalUrl = currentSite.GetUrl(fmt.Sprintf("/search?q=%s(&page={page})", url.QueryEscape(q)), nil, currentPage)
		ctx.ViewData("webInfo", webInfo)
	}

	ctx.ViewData("q", q)

	var tmpTpl string
	if module != nil {
		tmpTpl = "search/" + module.UrlToken + ".html"
	}
	tplName, ok := currentSite.TemplateExist(tmpTpl, "search/index.html", "search.html")
	if !ok {
		NotFound(ctx)
		return
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
