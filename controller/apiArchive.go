package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func ApiGetFavorites(ctx iris.Context) {
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

	favorites, total := currentSite.GetFavoriteList(int64(userId), currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"data":  favorites,
		"total": total,
	})
}

func ApiCheckFavorites(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchiveFavoriteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	userId := ctx.Values().GetUintDefault("userId", 0)
	if req.ArchiveId > 0 {
		req.ArchiveIds = append(req.ArchiveIds, req.ArchiveId)
	}
	result := currentSite.CheckFavorites(int64(userId), req.ArchiveIds)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"data": result,
	})
}

func ApiAddFavorite(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchiveFavoriteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	userId := ctx.Values().GetUintDefault("userId", 0)
	ok, err := currentSite.AddFavorite(int64(userId), req.ArchiveId, req.SkuId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
			"data": iris.Map{
				"is_favorite": ok,
			},
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("OperationSuccessful"),
		"data": iris.Map{
			"is_favorite": ok,
		},
	})
}

func ApiDeleteFavorite(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchiveFavoriteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	userId := ctx.Values().GetUintDefault("userId", 0)

	err := currentSite.DeleteFavorite(int64(userId), req.ArchiveId)

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
