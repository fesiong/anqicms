package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginGetWishlist(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	userId := ctx.URLParamInt64Default("user_id", 0)

	wishes, total := currentSite.GetWishlist(userId, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"data":  wishes,
		"total": total,
	})
}
