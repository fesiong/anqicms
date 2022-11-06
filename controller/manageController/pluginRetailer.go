package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginGetRetailers(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	userId := uint(ctx.URLParamIntDefault("id", 0))
	userName := ctx.URLParam("user_name")
	realName := ctx.URLParam("realName")

	ops := func(tx *gorm.DB) *gorm.DB {
		if config.JsonData.PluginRetailer.BecomeRetailer == 1 {
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
	users, total := provider.GetUserList(ops, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  users,
	})
}

func PluginRetailerConfig(ctx iris.Context) {
	retailer := config.JsonData.PluginRetailer

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": retailer,
	})
}

func PluginRetailerConfigForm(ctx iris.Context) {
	var req request.PluginRetailerConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.PluginRetailer.AllowSelf = req.AllowSelf
	config.JsonData.PluginRetailer.BecomeRetailer = req.BecomeRetailer

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新分销员信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func PluginRetailerSetRealName(ctx iris.Context) {
	var req request.UserRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.UpdateUserRealName(req.Id, req.RealName)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新分销员用户信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func PluginRetailerApply(ctx iris.Context) {
	var req request.UserRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.SetRetailerInfo(req.Id, req.IsRetailer)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新分销员用户信息"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
