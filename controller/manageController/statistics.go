package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
)

//蜘蛛爬行情况
func StatisticSpider(ctx iris.Context) {
	//支持按天，按小时区分
	separate := ctx.URLParam("separate")

	result := provider.StatisticSpider(separate)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

func StatisticTraffic(ctx iris.Context) {
	//支持按天，按小时区分
	separate := ctx.URLParam("separate")

	result := provider.StatisticTraffic(separate)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

func StatisticDetail(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("limit", 20)
	isSpider, _ := ctx.URLParamBool("is_spider")

	list, total, _ :=  provider.StatisticDetail(isSpider, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"count": total,
		"data": list,
	})
}