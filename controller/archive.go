package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"strings"
	"time"
)

func ArchiveDetail(ctx iris.Context) {
	id := ctx.Params().GetUintDefault("id", 0)
	urlToken := ctx.Params().GetString("filename")
	var archive *model.Archive
	var err error
	if urlToken != "" {
		//优先使用urlToken
		archive, err = provider.GetArchiveByUrlToken(urlToken)
	} else {
		archive, err = provider.GetArchiveById(id)
	}
	if err != nil || archive.Status != config.ContentStatusOK {
		NotFound(ctx)
		return
	}
	multiCatNames := ctx.Params().GetString("multicatname")
	if multiCatNames != "" {
		chunkCatNames := strings.Split(multiCatNames, "/")
		var prev *model.Category
		for _, catName := range chunkCatNames {
			tmpCat := provider.GetCategoryFromCacheByToken(catName)
			if tmpCat == nil || (prev != nil && tmpCat.ParentId != prev.Id) {
				NotFound(ctx)
				return
			}
			prev = tmpCat
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

	// check the archive had paid if the archive need to pay.
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId > 0 {
		if archive.Price > 0 {
			archive.HasOrdered = provider.CheckArchiveHasOrder(userId, archive.Id)
			// check price can make any discount？
			userInfo, _ := ctx.Values().Get("userInfo").(*model.User)
			discount := provider.GetUserDiscount(userId, userInfo)
			if discount > 0 {
				archive.FavorablePrice = archive.Price * discount / 100
			}
		}
		if archive.ReadLevel > 0 && !archive.HasOrdered {
			userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
			if userGroup != nil && userGroup.Level >= archive.ReadLevel {
				archive.HasOrdered = true
			}
		}
	}

	_ = archive.AddViews(dao.DB)

	webInfo.Title = archive.Title
	if archive.SeoTitle != "" {
		webInfo.Title = archive.SeoTitle
	}
	webInfo.Keywords = archive.Keywords
	webInfo.Description = archive.Description
	webInfo.NavBar = archive.CategoryId
	//设置页面名称，方便tags识别
	webInfo.PageName = "archiveDetail"
	webInfo.CanonicalUrl = archive.CanonicalUrl
	if webInfo.CanonicalUrl == "" {
		webInfo.CanonicalUrl = provider.GetUrl("archive", archive, 0)
	}
	ctx.ViewData("webInfo", webInfo)
	ctx.ViewData("archive", archive)
	//设置页面名称，方便tags识别
	ctx.ViewData("pageName", "archiveDetail")

	module := provider.GetModuleFromCache(archive.ModuleId)
	if module == nil {
		ShowMessage(ctx, fmt.Sprintf("%s: %d", config.Lang("未定义模型"), archive.ModuleId), nil)
		return
	}
	// 默认模板规则：表名 / index,list, detail .html
	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板
	var tplName string
	if archive.Template != "" {
		tplName = archive.Template
	} else {
		category := provider.GetCategoryFromCache(archive.CategoryId)
		if category != nil {
			categoryTemplate := provider.GetCategoryTemplate(category)
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

	err = ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func ArchiveIndex(ctx iris.Context) {
	urlToken := ctx.Params().GetString("module")
	module := provider.GetModuleFromCacheByToken(urlToken)
	if module == nil {
		NotFound(ctx)
		return
	}

	webInfo.Title = module.Title

	//设置页面名称，方便tags识别
	webInfo.PageName = "archiveIndex"
	webInfo.NavBar = module.Id
	webInfo.CanonicalUrl = provider.GetUrl("archiveIndex", module, 0)
	ctx.ViewData("webInfo", webInfo)
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

	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}
