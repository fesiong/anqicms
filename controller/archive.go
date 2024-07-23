package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"strconv"
	"strings"
	"time"
)

func ArchiveDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	cacheFile, ok := currentSite.LoadCachedHtml(ctx)
	if ok {
		ctx.ServeFile(cacheFile)
		return
	}
	id := ctx.Params().GetUintDefault("id", 0)
	urlToken := ctx.Params().GetString("filename")
	var archive *model.Archive
	var err error
	if urlToken != "" {
		//优先使用urlToken
		archive, err = currentSite.GetArchiveByUrlToken(urlToken)
	} else {
		archive, err = currentSite.GetArchiveById(id)
	}
	if err != nil {
		NotFound(ctx)
		return
	}
	var category *model.Category
	multiCatNames := ctx.Params().GetString("multicatname")
	if multiCatNames != "" {
		chunkCatNames := strings.Split(multiCatNames, "/")
		for _, catName := range chunkCatNames {
			tmpCat := currentSite.GetCategoryFromCacheByToken(catName, category)
			if tmpCat == nil || (category != nil && tmpCat.ParentId != category.Id) {
				// 则跳到正确的链接上
				ctx.Redirect(currentSite.GetUrl("archive", archive, 0), 301)
				return
			}
			category = tmpCat
		}
	}

	createTime := time.Unix(archive.CreatedTime, 0)
	year := ctx.Params().GetString("year")
	month := ctx.Params().GetString("month")
	day := ctx.Params().GetString("day")
	hour := ctx.Params().GetString("hour")
	minute := ctx.Params().GetString("minute")
	second := ctx.Params().GetString("second")
	if year != "" && year != createTime.Format("2006") {
		NotFound(ctx)
		return
	}
	if month != "" && month != createTime.Format("01") {
		NotFound(ctx)
		return
	}
	if day != "" && day != createTime.Format("02") {
		NotFound(ctx)
		return
	}
	if hour != "" && hour != createTime.Format("15") {
		NotFound(ctx)
		return
	}
	if minute != "" && minute != createTime.Format("04") {
		NotFound(ctx)
		return
	}
	if second != "" && second != createTime.Format("05") {
		NotFound(ctx)
		return
	}
	// 支持 combine
	combineName := ctx.Params().GetString("combine")
	var combineArchive *model.Archive
	if combineName != "" {
		// 需要先验证是否是archive
		tmpId, err := strconv.Atoi(combineName)
		if err == nil {
			combineArchive, err = currentSite.GetArchiveById(uint(tmpId))
		} else {
			combineArchive, err = currentSite.GetArchiveByUrlToken(combineName)
		}
		if err != nil {
			// 不存在
			NotFound(ctx)
			return
		}
		ctx.ViewData("combineId", combineArchive.Id)
	}
	ctx.ViewData("combineArchive", combineArchive)

	// check the archive had paid if the archive need to pay.
	userId := ctx.Values().GetUintDefault("userId", 0)
	userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
	archive = currentSite.CheckArchiveHasOrder(userId, archive, userGroup)
	if archive.Price > 0 {
		userInfo, _ := ctx.Values().Get("userInfo").(*model.User)
		discount := currentSite.GetUserDiscount(userId, userInfo)
		if discount > 0 {
			archive.FavorablePrice = archive.Price * discount / 100
		}
	}

	go archive.AddViews(currentSite.DB)

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = archive.Title
		if archive.SeoTitle != "" {
			webInfo.Title = archive.SeoTitle
		}
		webInfo.Keywords = archive.Keywords
		webInfo.Description = archive.Description
		webInfo.NavBar = archive.CategoryId
		webInfo.PageId = archive.Id
		//设置页面名称，方便tags识别
		webInfo.PageName = "archiveDetail"
		webInfo.CanonicalUrl = archive.CanonicalUrl
		if webInfo.CanonicalUrl == "" {
			if combineArchive != nil {
				webInfo.CanonicalUrl = currentSite.GetUrl("archive", archive, 0, combineArchive)
			} else {
				webInfo.CanonicalUrl = currentSite.GetUrl("archive", archive, 0)
			}
		}
		ctx.ViewData("webInfo", webInfo)
	}
	ctx.ViewData("archive", archive)
	//设置页面名称，方便tags识别
	ctx.ViewData("pageName", "archiveDetail")

	module := currentSite.GetModuleFromCache(archive.ModuleId)
	if module == nil {
		ctx.StatusCode(404)
		ShowMessage(ctx, ctx.Tr("未定义模型: %d", archive.ModuleId), nil)
		return
	}
	// 默认模板规则：表名 / index,list, detail .html
	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板
	var tplName string
	if archive.Template != "" {
		tplName = archive.Template
	} else {
		if category == nil {
			category = currentSite.GetCategoryFromCache(archive.CategoryId)
		}
		if category != nil {
			categoryTemplate := currentSite.GetCategoryTemplate(category)
			if categoryTemplate != nil {
				tplName = categoryTemplate.DetailTemplate
			}
		}
	}
	if tplName == "" {
		tplName = module.TableName + "/detail.html"
		tplName2 := module.TableName + "_detail.html"
		if ViewExists(ctx, tplName2) {
			tplName = tplName2
		}
	}

	if !strings.HasSuffix(tplName, ".html") {
		tplName += ".html"
	}

	tmpTpl := fmt.Sprintf("%s/detail-%d.html", module.TableName, archive.Id)
	if ViewExists(ctx, tmpTpl) {
		tplName = tmpTpl
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

func ArchiveIndex(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	cacheFile, ok := currentSite.LoadCachedHtml(ctx)
	if ok {
		ctx.ServeFile(cacheFile)
		return
	}
	urlToken := ctx.Params().GetString("module")
	module := currentSite.GetModuleFromCacheByToken(urlToken)
	if module == nil {
		NotFound(ctx)
		return
	}

	if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
		webInfo.Title = module.Title

		//设置页面名称，方便tags识别
		webInfo.PageName = "archiveIndex"
		webInfo.NavBar = module.Id
		webInfo.CanonicalUrl = currentSite.GetUrl("archiveIndex", module, 0)
		ctx.ViewData("webInfo", webInfo)
	}
	ctx.ViewData("module", module)
	//设置页面名称，方便tags识别
	ctx.ViewData("pageName", "archiveIndex")

	// 默认模板规则：表名 / index,list, detail .html
	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板
	tplName := module.TableName + "/index.html"
	tplName2 := module.TableName + "_index.html"
	if ViewExists(ctx, tplName2) {
		tplName = tplName2
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
