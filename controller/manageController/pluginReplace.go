package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginReplaceValues(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginReplaceRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	total := currentSite.ReplaceValues(&req)
	
	currentSite.AddAdminLog(ctx, fmt.Sprintf("全站替换 %v, %v", req.Places, req.Keywords))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "替换已完成",
		"data": total,
	})
}
