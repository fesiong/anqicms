package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func ApiWishlist(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusNoLogin,
			"msg":  currentSite.TplTr("PleaseLogIn"),
		})
		return
	}
	wishlists, total := currentSite.GetWishlist(int64(userId), currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"data":  wishlists,
		"total": total,
	})
}

func ApiWishlistAdd(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WishListRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	userId := ctx.Values().GetUintDefault("userId", 0)
	req.UserId = int64(userId)
	err := currentSite.WishlistAdd(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("OperationSuccessful"),
	})
}

func ApiWishlistDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.WishListRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	userId := ctx.Values().GetUintDefault("userId", 0)
	req.UserId = int64(userId)
	err := currentSite.WishlistDelete(&req)

	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("OperationSuccessful"),
	})
}
