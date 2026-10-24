package controller

import (
	"github.com/google/uuid"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"time"
)

func ApiCartList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	userId := ctx.Values().GetUintDefault("userId", 0)

	cartToken := ctx.GetCookie("cart")
	if cartToken == "" {
		cartToken = ctx.GetHeader("cart")
	}

	wishlists, total := currentSite.GetCartList(int64(userId), cartToken, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"data":  wishlists,
		"total": total,
	})
}

func ApiCartAdd(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CartRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	cartToken := ctx.GetCookie("cart")
	if cartToken == "" {
		cartToken = ctx.GetHeader("cart")
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId == 0 {
		if cartToken == "" {
			// 生成一个
			tokenId, _ := uuid.NewUUID()
			cartToken = tokenId.String()
			// 设置 cookie
			ctx.SetCookieKV("cart", cartToken, iris.CookieExpires(30*24*time.Hour))
		}
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}
	req.Token = cartToken
	req.UserId = int64(userId)
	err := currentSite.CartAdd(&req)
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
		"data": iris.Map{
			"cart": cartToken,
		},
	})
}

func ApiCartUpdate(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CartRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	cartToken := ctx.GetCookie("cart")
	if cartToken == "" {
		cartToken = ctx.GetHeader("cart")
	}

	userId := ctx.Values().GetUintDefault("userId", 0)
	req.UserId = int64(userId)
	req.Token = cartToken
	err := currentSite.CartUpdate(&req)
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

func ApiCartDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CartRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	cartToken := ctx.GetCookie("cart")
	if cartToken == "" {
		cartToken = ctx.GetHeader("cart")
	}
	req.Token = cartToken
	userId := ctx.Values().GetUintDefault("userId", 0)
	req.UserId = int64(userId)
	err := currentSite.CartDelete(&req)

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
