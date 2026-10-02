package manageController

import (
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginRedirectList 获取 301 跳转列表，支持分页和来源链接搜索。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "from_url": 来源链接筛选条件。
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

// PluginRedirectDetailForm 新增或更新 301 跳转链接。
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
		"data": redirect,
	})
}

// PluginRedirectDelete 根据 ID 删除指定的 301 跳转链接。
func PluginRedirectDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginRedirectDeleteRequest
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

// PluginRedirectImport 批量导入 301 跳转地址配置。
//
// 参数说明：
//   - 表单参数 "file": 导入链接文件。文件内容格式一行一条，用逗号隔开，如：from_url, to_url
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
