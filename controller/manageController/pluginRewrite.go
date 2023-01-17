package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"time"
)

func PluginRewrite(ctx iris.Context) {
	pluginRewrite := config.JsonData.PluginRewrite

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginRewrite,
	})
}

func PluginRewriteForm(ctx iris.Context) {
	var req config.PluginRewriteConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if config.JsonData.PluginRewrite.Mode != req.Mode || config.JsonData.PluginRewrite.Patten != req.Patten {
		config.JsonData.PluginRewrite.Mode = req.Mode
		config.JsonData.PluginRewrite.Patten = req.Patten
		err := provider.SaveSettingValue(provider.RewriteSettingKey, config.JsonData.PluginRewrite)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		config.ParsePatten(true)
		config.RestartChan <- true
		time.Sleep(2 * time.Second)
		provider.DeleteCacheIndex()
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("调整伪静态配置：%d", req.Mode))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
