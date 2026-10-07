package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

// PluginFinanceList 获取财务流水列表，支持分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
func PluginFinanceList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	orders, total := currentSite.GetFinanceList(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

// PluginFinanceDetail 查询单个财务记录详情，按 id 返回对应的财务明细。
//
// 参数说明：
//   - 查询参数 "id": 财务记录 ID，默认为 0（未命中任何记录）。
func PluginFinanceDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	finance, err := currentSite.GetFinanceById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": finance,
	})
}
