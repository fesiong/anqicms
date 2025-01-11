package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginReplaceValues(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginReplaceRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	total := currentSite.ReplaceValues(&req)

	currentSite.AddAdminLog(ctx, ctx.Tr("ReplaceTheEntireSiteLog", req.Places, req.Keywords))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ReplacementCompleted"),
		"data": total,
	})
}
