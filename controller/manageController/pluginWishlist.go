package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

// PluginGetWishlist 分页查询用户收藏（心愿单）列表。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "user_id": 按用户 ID 筛选，0 表示不限（返回全部用户的收藏）。
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
