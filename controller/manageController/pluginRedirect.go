package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
)

func PluginRedirectList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	fromUrl := ctx.URLParam("from_url")

	redirectList, total, err := currentSite.GetRedirectList(fromUrl, currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  redirectList,
	})
}

func PluginRedirectDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginRedirectRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.FromUrl == req.ToUrl {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("SourceLinkAndJumpLinkCannotBeTheSame"),
		})
		return
	}
	if !strings.HasPrefix(req.FromUrl, "http") && !strings.HasPrefix(req.FromUrl, "/") {
		req.FromUrl = "/" + req.FromUrl
	}
	if !strings.HasPrefix(req.ToUrl, "http") && !strings.HasPrefix(req.ToUrl, "/") {
		req.ToUrl = "/" + req.ToUrl
	}

	var redirect *model.Redirect
	var err error

	if req.Id > 0 {
		redirect, err = currentSite.GetRedirectById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		//去重
		exists, err := currentSite.GetRedirectByFromUrl(req.FromUrl)
		if err == nil && exists.Id != redirect.Id {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("LinkAlreadyExists", req.FromUrl),
			})
			return
		}
	} else {
		//新增支持批量插入
		redirect, err = currentSite.GetRedirectByFromUrl(req.FromUrl)
		if err != nil {
			//不存在
			redirect = &model.Redirect{
				FromUrl: req.FromUrl,
				ToUrl:   req.ToUrl,
			}
		}
	}
	redirect.FromUrl = req.FromUrl
	redirect.ToUrl = req.ToUrl

	err = currentSite.DB.Save(redirect).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("Update301JumpLink", redirect.FromUrl, redirect.ToUrl))

	currentSite.DeleteCacheRedirects()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LinkUpdated"),
	})
}

func PluginRedirectDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginRedirectRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	redirect, err := currentSite.GetRedirectById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.DeleteRedirect(redirect)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("Delete301JumpLink", redirect.FromUrl, redirect.ToUrl))

	currentSite.DeleteCacheRedirects()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteOperationHasBeenPerformed"),
	})
}

func PluginRedirectImport(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()

	result, err := currentSite.ImportRedirects(file, info)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("Import301JumpLink"))

	currentSite.DeleteCacheRedirects()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("UploadCompleted"),
		"data": result,
	})
}
