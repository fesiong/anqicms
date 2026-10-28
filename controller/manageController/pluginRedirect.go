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
//
// 更新（传了 id）时**空值不覆盖**：from_url / to_url 任一为空都表示"这次不改它"。
// 两条字段都是规则的必要组成部分（缺 from 没有任何请求会命中，缺 to 无从跳转），
// 清空它们不构成任何合法意图，只可能是调用方漏参数。
//
// 2026-10-27 实测的严重缺陷：此前是 `redirect.FromUrl = req.FromUrl` 无条件覆盖，
// 再叠加下面的"补前导斜杠"逻辑，空串被补成 "/" —— 于是
// `redirect_update{id, to:"/new"}` 会把 from_url 写成 "/"，一条 301 就把
// **全站首页**永久跳到 /new，且回执是 ok。同类端点（分类/模型/导航）早已改成
// `if !req.Partial { UpdateAll = true }` 的 PATCH 语义，这里补上同等保护。
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

	// 先记录调用方是否真的给了值，再做补前缀——补前缀本身会把 "" 变成 "/"，
	// 顺序反了就无法区分"没传"和"传了 /"。
	fromProvided := strings.TrimSpace(req.FromUrl) != ""
	toProvided := strings.TrimSpace(req.ToUrl) != ""

	if fromProvided && toProvided && req.FromUrl == req.ToUrl {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("SourceLinkAndJumpLinkCannotBeTheSame"),
		})
		return
	}
	if fromProvided && !strings.HasPrefix(req.FromUrl, "http") && !strings.HasPrefix(req.FromUrl, "/") {
		req.FromUrl = "/" + req.FromUrl
	}
	if toProvided && !strings.HasPrefix(req.ToUrl, "http") && !strings.HasPrefix(req.ToUrl, "/") {
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
		// 去重只在来源真的要改时才做（拿空值去查会把"未传"当成"要改成空"）。
		if fromProvided {
			exists, err := currentSite.GetRedirectByFromUrl(req.FromUrl)
			if err == nil && exists.Id != redirect.Id {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("LinkAlreadyExists", req.FromUrl),
				})
				return
			}
		}
	} else {
		// 新建必须有来源地址，否则无从命中。
		if !fromProvided {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  "来源路径不能为空",
			})
			return
		}
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
	if fromProvided {
		redirect.FromUrl = req.FromUrl
	}
	if toProvided {
		redirect.ToUrl = req.ToUrl
	}

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
