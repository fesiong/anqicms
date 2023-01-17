package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginFulltextConfig(ctx iris.Context) {
	setting := config.JsonData.PluginFulltext

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginFulltextConfigForm(ctx iris.Context) {
	var req config.PluginFulltextConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.PluginFulltext.Open = req.Open

	err := provider.SaveSettingValue(provider.FulltextSettingKey, config.JsonData.PluginFulltext)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新全文索引配置信息"))
	if req.Open {
		go provider.InitFulltext()
	} else {
		provider.CloseFulltext()
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
