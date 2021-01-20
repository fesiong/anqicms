package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
	"irisweb/request"
)

func PluginRobots(ctx iris.Context) {
	robots := provider.GetRobots()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"robots": robots,
		},
	})
}

func PluginRobotsForm(ctx iris.Context) {
	var req request.PluginRobotsConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.SaveRobots(req.Robots)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}