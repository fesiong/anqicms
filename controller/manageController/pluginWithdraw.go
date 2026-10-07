package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginWithdrawList 获取用户提现记录列表，支持分页。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
func PluginWithdrawList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	orders, total := currentSite.GetWithdrawList(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

// PluginWithdrawDetail 获取指定 ID 的提现记录详情。
//
// 参数说明：
//   - 查询参数 "id": 提现记录 ID，默认为 0。
func PluginWithdrawDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	withdraw, err := currentSite.GetWithdrawById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": withdraw,
	})
}

// PluginWithdrawSetApply 为指定分销用户申请提现。
//
// 参数说明：
//   - 请求体 "user_id": 用户 ID。
func PluginWithdrawSetApply(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserWithdrawApplyRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	withdraw, err := currentSite.RetailerApplyWithdraw(req.UserId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ApplicationSuccessful"),
		"data": withdraw,
	})
}

// PluginWithdrawSetApproval 同意用户的提现申请。
func PluginWithdrawSetApproval(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserWithdrawApprovalRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SetUserWithdrawApproval(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SetSuccessfully"),
	})
}

// PluginWithdrawSetFinished 标记提现完成。
func PluginWithdrawSetFinished(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserWithdrawApprovalRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SetUserWithdrawFinished(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SetSuccessfully"),
	})
}
