package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func ApiGetRetailerInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	retailerId := uint(ctx.URLParamIntDefault("retailer_id", 0))
	userId := ctx.Values().GetUintDefault("userId", 0)
	if retailerId == 0 {
		retailerId = userId
	}
	user, err := currentSite.GetUserInfoById(retailerId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("QueryFailed"),
		})
		return
	}
	user.Group, _ = currentSite.GetUserGroupInfo(user.GroupId)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": user,
	})
}

func ApiGetRetailerStatistics(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)
	user, err := currentSite.GetUserInfoById(userId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("LoginFailed"),
		})
		return
	}

	// 可提现佣金
	var canWithdrawAmount response.SumAmount
	currentSite.DB.Model(&model.Commission{}).Where("`user_id` = ? and `status` = ?", user.Id, config.CommissionStatusWait).Select("SUM(`amount`) as total").Take(&canWithdrawAmount)
	// 已提现佣金
	var paidWithdrawAmount response.SumAmount
	currentSite.DB.Model(&model.Commission{}).Where("`user_id` = ? and `status` = ?", user.Id, config.CommissionStatusPaid).Select("SUM(`amount`) as total").Take(&paidWithdrawAmount)
	// 未结算佣金
	// 未结算佣金从订单中计算
	var unfinishedWithdrawAmount response.SumAmount
	currentSite.DB.Model(&model.Order{}).Where("(`share_user_id` = ? or `share_parent_user_id` = ?) and `status` IN(?)", user.Id, user.Id, []int{config.OrderStatusPaid, config.OrderStatusDelivering}).Select("SUM(`amount`) as total").Take(&unfinishedWithdrawAmount)
	// 我的团队人数
	var memberCount int64
	currentSite.DB.Model(&model.User{}).Where("`parent_id` = ?", user.Id).Count(&memberCount)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"can_withdraw_amount":        canWithdrawAmount.Total,
			"paid_withdraw_amount":       paidWithdrawAmount.Total,
			"unfinished_withdraw_amount": unfinishedWithdrawAmount.Total,
			"member_count":               memberCount,
		},
	})
}

func ApiUpdateRetailerInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.UserRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	err := currentSite.DB.Model(model.User{}).Where("`id` = ?", userId).UpdateColumn("real_name", req.RealName).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UpdateInfoFailed"),
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("SaveSuccessfully"),
	})
}

func ApiGetRetailerOrders(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)

	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	orders, total := currentSite.GetRetailerOrders(userId, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

func ApiGetRetailerWithdraws(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)

	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	withdraws, total := currentSite.GetRetailerWithdraws(userId, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  withdraws,
	})
}

func ApiRetailerWithdraw(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)

	// 执行提现操作
	err := currentSite.RetailerApplyWithdraw(userId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  currentSite.TplTr("WithdrawalApplicationSubmitted"),
	})
}

func ApiGetRetailerMembers(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)

	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	members, total := currentSite.GetRetailerMembers(userId, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  members,
	})
}

func ApiGetRetailerCommissions(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)

	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	commissions, total := currentSite.GetRetailerCommissions(userId, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  commissions,
	})
}
