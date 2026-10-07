package manageController

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginGetRetailers 获取分销员列表，支持分页和按用户信息筛选。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
//   - 查询参数 "id": 用户 ID，大于 0 时精确过滤。
//   - 查询参数 "user_name": 用户名模糊搜索。
//   - 查询参数 "real_name": 真实姓名模糊搜索。
func PluginGetRetailers(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	userId := uint(ctx.URLParamIntDefault("id", 0))
	userName := ctx.URLParam("user_name")
	realName := ctx.URLParam("user_name")

	ops := func(tx *gorm.DB) *gorm.DB {
		if currentSite.PluginRetailer.BecomeRetailer == 1 {
			tx = tx.Where("`is_retailer` = ?", 1)
		}
		if userId > 0 {
			tx = tx.Where("`id` = ?", userId)
		}
		if userName != "" {
			tx = tx.Where("`user_name` like ?", "%"+userName+"%")
		}
		if realName != "" {
			tx = tx.Where("`real_name` like ?", "%"+realName+"%")
		}
		tx = tx.Order("id desc")
		return tx
	}
	users, total := currentSite.GetUserList(ops, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  users,
	})
}

// PluginRetailerConfig 获取当前站点的分销员插件配置。
func PluginRetailerConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	retailer := currentSite.PluginRetailer

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": retailer,
	})
}

// PluginRetailerConfigForm 保存分销员插件配置。
func PluginRetailerConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginRetailerConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.PluginRetailer.AllowSelf = req.AllowSelf
	currentSite.PluginRetailer.BecomeRetailer = req.BecomeRetailer

	err := currentSite.SaveSettingValue(provider.RetailerSettingKey, currentSite.PluginRetailer)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateDistributor"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// PluginRetailerSetRealName 设置分销员的真实姓名。
func PluginRetailerSetRealName(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.RetailerRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.UpdateUserRealName(req.Id, req.RealName)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateDistributorUser"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

// PluginRetailerApply 根据指定用户ID设置其是否成为分销员。
func PluginRetailerApply(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.RetailerApplyRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SetRetailerInfo(req.Id, req.IsRetailer)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateDistributorUser"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}
