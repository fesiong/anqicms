package manageController

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginActivityList 分页查询营销活动列表，支持按活动标题搜索。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "title": 活动标题模糊搜索。
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

// PluginActivityDetail 按 ID 查询单个营销活动详情，返回结果已带该活动的全部规格（skus）。
//
// 参数说明：
//   - 查询参数 "id": 活动 ID，必填；传 0 或活动不存在时返回错误信息。
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

// PluginActivityDetailForm 新建或更新营销活动。
//
// 默认全量覆盖（后台表单提交整个表单，没出现的字段 = 用户清空了它）；
// 传 partial=true 则走 PATCH，未传字段保持原值（AI/MCP 默认这样调）。
// 规格 skus 单独一套语义：不传=不动、传空数组=清空全部、非空=按列表同步。
func PluginActivityDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.ActivityRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !req.Partial {
		req.UpdateAll = true
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

// PluginActivityDelete 删除营销活动，并连带删除该活动的全部规格（activity_skus）。
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
