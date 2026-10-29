package middleware

import (
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"

	"github.com/kataras/iris/v12"
)

// ShopModeGuard 商城前台 API 模式守卫。
// 站点运行于普通内容模式（System.ShopMode == 0）时，商城前台接口一律返回 404，
// 保证存量普通站点的对外行为与 v3.6.7 一致（合并计划阶段 5.1）。
// 管理后台 /manage 路由不拦截：数据仍可管理，仅菜单隐藏。
// 新增商城前台路由时必须挂上本守卫，并在 provider/shop_guard_routes_test.go
// 的清单里登记，防止遗漏（风险 R7）。
func ShopModeGuard(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	if currentSite.System.ShopMode == 0 {
		ctx.StatusCode(iris.StatusNotFound)
		_ = ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "shop mode is not enabled",
		})
		return
	}
	ctx.Next()
}
