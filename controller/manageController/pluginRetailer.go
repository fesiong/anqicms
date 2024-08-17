package manageController

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginGetRetailers(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	userId := uint(ctx.URLParamIntDefault("id", 0))
	userName := ctx.URLParam("user_name")
	realName := ctx.URLParam("realName")

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

func PluginRetailerConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	retailer := currentSite.PluginRetailer

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": retailer,
	})
}

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

func PluginRetailerSetRealName(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserRequest
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

func PluginRetailerApply(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserRequest
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
