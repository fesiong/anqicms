package manageController

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginActivityList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	title := ctx.URLParam("title")

	coupons, total := currentSite.GetActivities(func(tx *gorm.DB) *gorm.DB {
		if title != "" {
			tx = tx.Where("title like ?", "%"+title+"%")
		}
		return tx
	}, "id desc", currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  coupons,
	})
}

func PluginActivityDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	id := ctx.URLParamInt64Default("id", 0)

	activity, err := currentSite.GetActivityById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	activity.Skus, _ = currentSite.GetActivitySkus(activity.Id)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": activity,
	})
}

func PluginActivityDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.ActivityRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveActivity(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}

func PluginActivityDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.ActivityRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteActivity(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessfully"),
	})
}
