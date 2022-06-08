package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
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
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	isSpider, _ := ctx.URLParamBool("is_spider")

	list, total, _ :=  provider.StatisticDetail(isSpider, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"total": total,
		"data": list,
	})
}