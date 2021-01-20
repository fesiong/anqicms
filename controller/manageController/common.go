package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
)

func Version(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"version": config.Version,
		},
	})
}

func Statistics(ctx iris.Context) {
	statistics := provider.Statistics()
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": statistics,
	})
}
