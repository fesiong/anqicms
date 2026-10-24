package controller

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func ApiActivities(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	activities, total := currentSite.GetActivities(func(tx *gorm.DB) *gorm.DB {
		tx = tx.Where("status = ?", 1)
		return tx
	}, "id desc", currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  activities,
	})
}

func ApiActivityDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.URLParamInt64Default("id", 0)
	activity, err := currentSite.GetActivityById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("ActivityNotFound"),
		})
		return
	}
	activity.Skus, _ = currentSite.GetActivitySkus(id)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": activity,
	})
}

func ApiGetArchiveActivity(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := ctx.URLParamInt64Default("archive_id", 0)

	activity, err := currentSite.GetActivityByGoodsId(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("ActivityNotFound"),
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
