package manageController

import (
    "github.com/kataras/iris/v12"
    "irisweb/config"
    "irisweb/request"
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
    var req request.PluginRewriteConfig
    if err := ctx.ReadJSON(&req); err != nil {
        ctx.JSON(iris.Map{
            "code": config.StatusFailed,
            "msg":  err.Error(),
        })
        return
    }

    config.JsonData.PluginRewrite.Mode = req.Mode
    config.JsonData.PluginRewrite.Patten = req.Patten

    err := config.WriteConfig()
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
