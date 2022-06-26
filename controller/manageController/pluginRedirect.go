package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
)

func PluginRedirectList(ctx iris.Context) {
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	fromUrl := ctx.URLParam("from_url")

	redirectList, total, err := provider.GetRedirectList(fromUrl, currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"total": total,
		"data": redirectList,
	})
}

func PluginRedirectDetailForm(ctx iris.Context) {
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
			"msg":  "源链接和跳转链接不能一样。",
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
		redirect, err = provider.GetRedirectById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		//去重
		exists, err := provider.GetRedirectByFromUrl(req.FromUrl)
		if err == nil && exists.Id != redirect.Id {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  fmt.Errorf("已存在链接%s，修改失败", req.FromUrl),
			})
			return
		}
	} else {
		//新增支持批量插入
		redirect, err = provider.GetRedirectByFromUrl(req.FromUrl)
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

	err = dao.DB.Save(redirect).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新301跳转链接：%s => %s", redirect.FromUrl, redirect.ToUrl))

	provider.DeleteCacheRedirects()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "链接已更新",
	})
}

func PluginRedirectDelete(ctx iris.Context) {
	var req request.PluginRedirectRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	redirect, err := provider.GetRedirectById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = provider.DeleteRedirect(redirect)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("删除301跳转链接：%s => %s", redirect.FromUrl, redirect.ToUrl))

	provider.DeleteCacheRedirects()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已执行删除操作",
	})
}

func PluginRedirectImport(ctx iris.Context) {
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}
	defer file.Close()

	result, err := provider.ImportRedirects(file, info)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("导入301跳转链接"))

	provider.DeleteCacheRedirects()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "上传完毕",
		"data": result,
	})
}
