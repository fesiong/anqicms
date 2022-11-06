package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func ApiGetRetailerInfo(ctx iris.Context) {
	retailerId := uint(ctx.URLParamIntDefault("retailer_id", 0))
	userId := ctx.Values().GetUintDefault("userId", 0)
	if retailerId == 0 {
		retailerId = userId
	}
	user, err := provider.GetUserInfoById(retailerId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "查询失败",
		})
		return
	}
	user.Group, _ = provider.GetUserGroupInfo(user.GroupId)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": user,
	})
}

func ApiGetRetailerStatistics(ctx iris.Context) {
	userId := ctx.Values().GetUintDefault("userId", 0)
	user, err := provider.GetUserInfoById(userId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "登录失败",
		})
		return
	}

	// 可提现佣金
	var canWithdrawAmount response.SumAmount
	dao.DB.Model(&model.Commission{}).Where("`user_id` = ? and `status` = ?", user.Id, config.CommissionStatusWait).Select("SUM(`amount`) as total").Take(&canWithdrawAmount)
	// 已提现佣金
	var paidWithdrawAmount response.SumAmount
	dao.DB.Model(&model.Commission{}).Where("`user_id` = ? and `status` = ?", user.Id, config.CommissionStatusPaid).Select("SUM(`amount`) as total").Take(&paidWithdrawAmount)
	// 未结算佣金
	// 未结算佣金从订单中计算
	var unfinishedWithdrawAmount response.SumAmount
	dao.DB.Model(&model.Order{}).Where("(`share_user_id` = ? or `share_parent_user_id` = ?) and `status` IN(?)", user.Id, user.Id, []int{config.OrderStatusPaid, config.OrderStatusDelivering}).Select("SUM(`amount`) as total").Take(&unfinishedWithdrawAmount)
	// 我的团队人数
	var memberCount int64
	dao.DB.Model(&model.User{}).Where("`parent_id` = ?", user.Id).Count(&memberCount)

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
	var req request.UserRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	err := dao.DB.Model(model.User{}).Where("`id` = ?", userId).UpdateColumn("real_name", req.RealName).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "更新信息失败",
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
	})
}

func ApiGetRetailerOrders(ctx iris.Context) {
	userId := ctx.Values().GetUintDefault("userId", 0)

	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	orders, total := provider.GetRetailerOrders(userId, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

func ApiGetRetailerWithdraws(ctx iris.Context) {
	userId := ctx.Values().GetUintDefault("userId", 0)

	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	withdraws, total := provider.GetRetailerWithdraws(userId, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  withdraws,
	})
}

func ApiRetailerWithdraw(ctx iris.Context) {
	userId := ctx.Values().GetUintDefault("userId", 0)

	// 执行提现操作
	err := provider.RetailerApplyWithdraw(userId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  "提现申请已提交",
	})
}

func ApiGetRetailerMembers(ctx iris.Context) {
	userId := ctx.Values().GetUintDefault("userId", 0)

	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	members, total := provider.GetRetailerMembers(userId, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  members,
	})
}

func ApiGetRetailerCommissions(ctx iris.Context) {
	userId := ctx.Values().GetUintDefault("userId", 0)

	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	commissions, total := provider.GetRetailerCommissions(userId, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  commissions,
	})
}
