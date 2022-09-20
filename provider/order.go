package provider

import (
	"context"
	"errors"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/pkg/util"
	"github.com/go-pay/gopay/wechat"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"log"
	"time"
)

func GetOrderList(userId uint, status string, page, pageSize int) ([]*model.Order, int64) {
	var orders []*model.Order
	var total int64
	offset := (page - 1) * pageSize
	tx := dao.DB.Model(&model.Order{})
	if userId > 0 {
		tx = tx.Where("`user_id` = ?", userId)
	}
	if status != "" {
		// status 可能会传 waiting,delivery,finished
		if status == "waiting" {
			tx = tx.Where("`status` = 0")
		}
		if status == "paid" {
			tx = tx.Where("`status` = 1")
		}
		if status == "delivery" {
			tx = tx.Where("`status` = 2")
		}
		if status == "finished" {
			tx = tx.Where("`status` = 3")
		}
		if status == "refunding" {
			tx = tx.Where("`status` = 8")
		}
	}

	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&orders)
	// order 还需要获取detail
	if len(orders) > 0 {
		var orderIds = make([]string, 0, len(orders))
		var userIds = make([]uint, 0, len(orders))
		for i := range orders {
			orderIds = append(orderIds, orders[i].OrderId)
			userIds = append(userIds, orders[i].UserId)
		}
		var details []*model.OrderDetail
		dao.DB.Where("`order_id` IN(?)", orderIds).Find(&details)
		if len(details) > 0 {
			var archiveIds = make([]uint, 0, len(details))
			for i := range details {
				archiveIds = append(archiveIds, details[i].GoodsId)
			}
			var archives []*model.Archive
			dao.DB.Where("`id` IN(?)", archiveIds).Find(&archives)
			for i := range details {
				for x := range archives {
					if archives[x].Id == details[i].GoodsId {
						details[i].Goods = archives[x]
					}
				}
			}
			for i := range orders {
				for j := range details {
					if orders[i].OrderId == details[j].OrderId {
						orders[i].Details = append(orders[i].Details, details[j])
					}
				}
			}
		}
		users := GetUsersInfoByIds(userIds)
		for i := range orders {
			for u := range users {
				if orders[i].UserId == users[u].Id {
					orders[i].User = users[u]
				}
			}
		}
	}

	return orders, total
}

func GetOrderInfoByOrderId(orderId string) (*model.Order, error) {
	var order model.Order
	err := dao.DB.Where("`order_id` = ?", orderId).Take(&order).Error

	if err != nil {
		return nil, err
	}
	var details []*model.OrderDetail
	dao.DB.Where("`order_id` = ?", order.OrderId).Find(&details)
	if len(details) > 0 {
		var archiveIds = make([]uint, 0, len(details))
		for i := range details {
			archiveIds = append(archiveIds, details[i].GoodsId)
		}
		var archives []*model.Archive
		dao.DB.Where("`id` IN(?)", archiveIds).Find(&archives)
		for i := range details {
			for x := range archives {
				if archives[x].Id == details[i].GoodsId {
					details[i].Goods = archives[x]
				}
			}
		}
		order.Details = details
	}
	orderAddress, err := GetOrderAddressById(order.AddressId)
	if err == nil {
		order.OrderAddress = orderAddress
	}

	return &order, nil
}

func GetPaymentInfoByPaymentId(paymentId string) (*model.Payment, error) {
	var payment model.Payment
	err := dao.DB.Where("`payment_id` = ?", paymentId).Take(&payment).Error

	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func GetPaymentInfoByOrderId(orderId string) (*model.Payment, error) {
	var payment model.Payment
	err := dao.DB.Where("`order_id` = ?", orderId).Take(&payment).Error

	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func GeneratePayment(order *model.Order) (*model.Payment, error) {
	payment, err := GetPaymentInfoByOrderId(order.OrderId)
	if err == nil {
		return payment, nil
	}

	payment = &model.Payment{
		UserId:  order.UserId,
		OrderId: order.OrderId,
		Amount:  order.Amount,
		Status:  0,
		Remark:  order.Remark,
	}
	err = dao.DB.Save(payment).Error
	if err != nil {
		return nil, err
	}

	return payment, nil
}

func SetOrderDeliver(req *request.OrderRequest) error {
	order, err := GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		return err
	}

	order.Status = config.OrderStatusDelivering
	order.DeliverTime = time.Now().Unix()
	order.ExpressCompany = req.ExpressCompany
	order.TrackingNumber = req.TrackingNumber
	order.EndTime = time.Now().AddDate(0, 0, 10).Unix()
	dao.DB.Save(order)

	return nil
}

func SetOrderFinished(order *model.Order) error {
	tx := dao.DB.Begin()
	order.Status = config.OrderStatusCompleted
	order.FinishedTime = time.Now().Unix()
	err := tx.Save(order).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	// 处理订单完成，并开始分钱
	if order.ShareAmount > 0 {
		//
		shareUser, err := GetUserInfoById(order.ShareUserId)
		if err == nil {
			shareAmount := model.Commission{
				UserId:      shareUser.Id,
				OrderId:     order.OrderId,
				OrderAmount: order.Amount,
				Amount:      order.ShareAmount,
				Status:      0,
				WithdrawId:  0,
				Remark:      "",
			}
			err = tx.Save(&shareAmount).Error
			if err != nil {
				tx.Rollback()
				return err
			}
			tx.Model(model.User{}).Where("`id` = ?", shareUser.Id).UpdateColumn("balance", gorm.Expr("`balance` + ?", order.ShareAmount))
			var userBalance int64
			err = tx.Model(model.User{}).Where("`id` = ?", order.UserId).Pluck("balance", &userBalance).Error
			//状态更改了，增加一条记录到用户
			finance := model.Finance{
				UserId:      shareUser.Id,
				Direction:   config.FinanceIncome,
				Amount:      order.ShareAmount,
				AfterAmount: userBalance,
				Action:      config.FinanceActionCommission,
				OrderId:     order.OrderId,
				Status:      1,
			}
			err = tx.Create(&finance).Error
			if err != nil {
				//
			}
			var totalReward response.SumAmount
			tx.Model(model.Commission{}).Where("`user_id` = ?").Select("SUM(`amount`) as total").Take(&totalReward)
			tx.Model(model.User{}).Where("`id` = ?", shareUser.Id).UpdateColumn("total_reward", totalReward.Total)
		}

		if order.ShareParentAmount > 0 {
			shareAmount := model.Commission{
				UserId:      order.ShareParentUserId,
				OrderId:     order.OrderId,
				OrderAmount: order.Amount,
				Amount:      order.ShareParentAmount,
				Status:      0,
				WithdrawId:  0,
				Remark:      "",
			}
			err = dao.DB.Save(&shareAmount).Error
			if err != nil {
				tx.Rollback()
				return err
			}
			tx.Model(model.User{}).Where("`id` = ?", order.ShareParentUserId).UpdateColumn("balance", gorm.Expr("`balance` + ?", order.ShareParentAmount))
			var userBalance int64
			err = tx.Model(&model.User{}).Where("`id` = ?", order.ShareParentUserId).Pluck("balance", &userBalance).Error
			//状态更改了，增加一条记录到用户
			finance := model.Finance{
				UserId:      order.ShareParentUserId,
				Direction:   config.FinanceIncome,
				Amount:      order.ShareParentAmount,
				AfterAmount: userBalance,
				Action:      config.FinanceActionCommission,
				OrderId:     order.OrderId,
				Status:      1,
			}
			err = tx.Create(&finance).Error
			if err != nil {
				//
			}
			var totalReward response.SumAmount
			tx.Model(model.Commission{}).Where("`user_id` = ?").Select("SUM(`amount`) as total").Take(&totalReward)
			tx.Model(model.User{}).Where("`id` = ?", order.ShareParentUserId).UpdateColumn("total_reward", totalReward.Total)
		}
	}
	tx.Commit()

	return nil
}

func SetOrderCanceled(order *model.Order) error {
	order.Status = config.OrderStatusCanceled
	order.FinishedTime = time.Now().Unix()
	dao.DB.Save(order)

	return nil
}

func SetOrderRefund(order *model.Order, status int) error {
	refund, err := GetOrderRefundByOrderId(order.OrderId)
	if err != nil {
		return err
	}
	// todo 金钱原路退回
	if status == 1 {
		client := wechat.NewClient(config.JsonData.PluginPay.WeixinAppId, config.JsonData.PluginPay.WeixinMchId, config.JsonData.PluginPay.WeixinApiKey, true)
		err := client.AddCertPemFilePath(config.ExecPath + config.JsonData.PluginPay.WeixinCertPath, config.ExecPath + config.JsonData.PluginPay.WeixinKeyPath)
		if err != nil {
			log.Println("微信证书错误：", err.Error())
			return err
		}

		bm := make(gopay.BodyMap)
		bm.Set("nonce_str", util.RandomString(32)).
			Set("out_trade_no", order.PaymentId).
			Set("out_refund_no", refund.RefundId).
			Set("total_fee", order.Amount).
			Set("refund_fee", refund.Amount).
			Set("sign_type", wechat.SignType_MD5)

		wxRsp, _, err := client.Refund(context.Background(), bm)
		if err != nil {
			refund.Remark = err.Error()
			dao.DB.Model(refund).UpdateColumn("remark", refund.Remark)
			return err
		}

		refund.Remark = wxRsp.ErrCodeDes
		dao.DB.Model(refund).UpdateColumn("remark", refund.Remark)

		if wxRsp.ReturnCode == gopay.FAIL {
			return errors.New(wxRsp.ReturnMsg)
		}
		if wxRsp.ResultCode == gopay.FAIL {
			refund.Status = config.OrderRefundStatusFailed
			dao.DB.Model(refund).UpdateColumn("status", refund.Status)
			return errors.New(wxRsp.ErrCodeDes)
		}
		refund.Status = config.OrderRefundStatusDone
		err = SuccessRefundOrder(refund, order)
		if err != nil {
			return err
		}
	} else {
		// 不同意
		order.RefundStatus = 0
		order.FinishedTime = time.Now().Unix()
		dao.DB.Save(order)
		refund.Status = config.OrderStatusCanceled
		dao.DB.Save(refund)
	}

	return nil
}

func ApplyOrderRefund(order *model.Order) error {
	// 用户申请退款
	refund, err := GetOrderRefundByOrderId(order.OrderId)
	if err == nil {
		return nil
	}

	refund = &model.OrderRefund{
		OrderId:  order.OrderId,
		DetailId: 0,
		UserId:   order.UserId,
		Amount:   order.Amount,
		Status:   0,
		Remark:   "用户申请退款",
	}
	dao.DB.Save(refund)

	//order.Status = config.OrderStatusRefunding
	order.RefundStatus = config.OrderStatusRefunding
	dao.DB.Save(order)

	return nil
}

func GetOrderRefundByOrderId(orderId string) (*model.OrderRefund, error) {
	var order model.OrderRefund
	if err := dao.DB.Model(&model.OrderRefund{}).Where("`order_id` = ?", orderId).First(&order).Error; err != nil {
		return nil, err
	}

	return &order, nil
}

func SuccessPaidOrder(order *model.Order) error {
	if order.Status == config.OrderStatusPaid {
		//支付成功
		return nil
	}
	originStatus := order.Status

	order.PaidTime = time.Now().Unix()
	order.Status = config.OrderStatusPaid

	db := dao.DB.Begin()
	if err := dao.DB.Model(order).Where("status  = ?", originStatus).Select("paid_time", "status").Updates(order).Error; err != nil {
		db.Rollback()
		return err
	}

	db.Commit()

	return nil
}

func SuccessRefundOrder(refund *model.OrderRefund, order *model.Order) error {
	var err error
	if order == nil {
		order, err = GetOrderInfoByOrderId(refund.OrderId)
		if err != nil {
			return err
		}
	}
	if order.Status == config.OrderStatusRefunded {
		// already refunded
		return nil
	}

	tx := dao.DB.Begin()
	//退款成功，则标记订单完成
	order.Status = config.OrderStatusRefunded
	tx.Model(order).UpdateColumn("status", order.Status)
	//refund
	if refund.Status == config.OrderRefundStatusWaiting {
		refund.Status = config.OrderRefundStatusDone
	}
	if refund.RefundTime == 0 {
		refund.RefundTime = time.Now().Unix()
	}
	dao.DB.Updates(refund)
	// todo 如果订单已成功的退款，需要额外处理佣金问题
	// 退款后，如果有赠送佣金，要扣除
	var userCommission model.Commission
	err1 := tx.Where("order_id = ?", order.OrderId).Take(&userCommission).Error
	if err1 == nil {
		// 有记录
		if userCommission.Status == config.CommissionStatusWait {
			// 未提现，退款
			userCommission.Status = config.CommissionStatusCancel
			tx.Save(&userCommission)
			//生成用户支付记录
			var userBalance int64
			err = tx.Model(&model.User{}).Where("`id` = ?", userCommission.UserId).Pluck("balance", &userBalance).Error
			//状态更改了，增加一条记录到用户
			finance := model.Finance{
				UserId:      userCommission.UserId,
				Direction:   config.FinanceOutput,
				Amount:      userCommission.Amount,
				AfterAmount: userBalance,
				Action:      config.FinanceActionRefund,
				OrderId:     order.OrderId,
				Status:      1,
			}
			err = tx.Create(&finance).Error
			if err != nil {
				//
			}
		}
	}
	//生成用户支付记录
	var userBalance int64
	err = tx.Model(&model.User{}).Where("`id` = ?", order.UserId).Pluck("balance", &userBalance).Error
	//状态更改了，增加一条记录到用户
	finance := model.Finance{
		UserId:      order.UserId,
		Direction:   config.FinanceOutput,
		Amount:      refund.Amount,
		AfterAmount: userBalance,
		Action:      config.FinanceActionRefund,
		OrderId:     order.OrderId,
		Status:      1,
	}
	err = tx.Create(&finance).Error
	if err != nil {
		//
	}

	tx.Commit()

	return err
}

func CreateOrder(userId uint, req *request.OrderRequest) (*model.Order, error) {
	user, err := GetUserInfoById(userId)
	if err != nil {
		return nil, err
	}
	tx := dao.DB.Begin()
	//保存订单地址
	orderAddress, err := SaveOrderAddress(tx, userId, &req.Address)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	var amount int64
	var remark = req.Remark
	if remark == "" {
		for _, v := range req.Details {
			archive, err := GetArchiveById(v.GoodsId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			if remark == "" {
				remark += archive.Title + "等"
			}
		}
	}

	order := model.Order{
		UserId:      userId,
		AddressId:   orderAddress.Id,
		Remark:      remark,
		Origin:      req.Origin,
		Color:       req.Color,
		Status:      0,
		ShareUserId: user.ParentId,
	}
	err = tx.Save(&order).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// 计算商品总价
	for _, v := range req.Details {
		archive, err := GetArchiveById(v.GoodsId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		//计算价格
		price := archive.Price
		originPrice := price
		detailAmount := price * int64(v.Quantity)
		amount += detailAmount
		//给每条子订单入库
		orderDetail := model.OrderDetail{
			OrderId:      order.OrderId,
			UserId:       userId,
			GoodsId:      v.GoodsId,
			Price:        price,
			OriginPrice:  originPrice,
			Amount:       detailAmount,
			OriginAmount: detailAmount,
			Quantity:     v.Quantity,
			Status:       1,
		}
		err = tx.Save(&orderDetail).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	order.Amount = amount
	order.OriginAmount = amount

	shareId := user.ParentId
	if config.JsonData.PluginRetailer.AllowSelf == 1 && (config.JsonData.PluginRetailer.BecomeRetailer == 1 || user.IsRetailer == 1) {
		shareId = user.Id
	}
	if shareId > 0 {
		shareUser, err := GetUserInfoById(shareId)
		if err == nil {
			if config.JsonData.PluginRetailer.BecomeRetailer == 1 || shareUser.IsRetailer == 1 {
				order.ShareAmount = order.Amount * config.JsonData.PluginRetailer.ShareReward / 100
				// 如果上级也是分销员，则上级也获得推荐奖励
				if shareUser.ParentId > 0 && config.JsonData.PluginRetailer.ParentReward > 0 {
					parent, err := GetUserInfoById(shareUser.ParentId)
					if err == nil {
						// 需要分给上级
						if config.JsonData.PluginRetailer.BecomeRetailer == 1 || parent.IsRetailer == 1 {
							order.ShareParentAmount = order.Amount * config.JsonData.PluginRetailer.ParentReward / 100
							order.ShareParentUserId = parent.Id
						}
					}
				}
			}
		}
	}

	tx.Save(&order)

	tx.Commit()

	return &order, nil
}

func GetOrderAddressByUserId(userId uint) (*model.OrderAddress, error) {
	var orderAddress model.OrderAddress
	err := dao.DB.Where("`user_id` = ?", userId).Order("id desc").Take(&orderAddress).Error
	if err != nil {
		return nil, err
	}

	return &orderAddress, nil
}

func GetOrderAddressById(id uint) (*model.OrderAddress, error) {
	var orderAddress model.OrderAddress
	err := dao.DB.Where("`id` = ?", id).Take(&orderAddress).Error
	if err != nil {
		return nil, err
	}

	return &orderAddress, nil
}

func SaveOrderAddress(tx *gorm.DB, userId uint, req *request.OrderAddressRequest) (*model.OrderAddress, error) {
	var orderAddress model.OrderAddress
	var err error
	if req.Id > 0 {
		err = tx.Where("`id` = ?", req.Id).Take(&orderAddress).Error
		if err != nil || orderAddress.UserId != userId {
			return nil, errors.New("地址不存在")
		}
	} else {
		orderAddress = model.OrderAddress{
			UserId: userId,
		}
	}
	orderAddress.Name = req.Name
	orderAddress.Phone = req.Phone
	orderAddress.Province = req.Province
	orderAddress.City = req.City
	orderAddress.Country = req.Country
	orderAddress.AddressInfo = req.AddressInfo
	orderAddress.Postcode = req.Postcode
	orderAddress.Status = 1

	err = tx.Save(&orderAddress).Error
	if err != nil {
		return nil, err
	}

	return &orderAddress, nil
}

func GetRetailerOrders(retailerId uint, page, pageSize int) ([]*model.Order, int64) {
	var orders []*model.Order
	var total int64
	offset := (page - 1) * pageSize
	tx := dao.DB.Model(&model.Order{}).Where("(`share_user_id` = ? or `share_parent_user_id` = ?)", retailerId, retailerId)
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&orders)

	return orders, total
}

func GetRetailerWithdraws(retailerId uint, page, pageSize int) ([]*model.UserWithdraw, int64) {
	var withdraws []*model.UserWithdraw
	var total int64
	offset := (page - 1) * pageSize
	tx := dao.DB.Model(&model.UserWithdraw{}).Where("`user_id` = ?", retailerId)
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&withdraws)

	return withdraws, total
}

func GetRetailerCommissions(retailerId uint, page, pageSize int) ([]*model.Commission, int64) {
	var commissions []*model.Commission
	var total int64
	offset := (page - 1) * pageSize
	tx := dao.DB.Model(&model.Commission{}).Where("`user_id` = ?", retailerId)
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&commissions)

	return commissions, total
}

func RetailerApplyWithdraw(retailerId uint) error {
	// 查询可提现金额
	var commissions []model.Commission
	var total int64
	dao.DB.Model(&model.Commission{}).Where("`user_id` = ? and `status` = ?", retailerId, config.CommissionStatusWait).Find(&commissions)
	for i := range commissions {
		total += commissions[i].Amount
	}

	if total <= 0 {
		return errors.New("没有可提现金额")
	}

	// todo执行提现操作
	// 低于2元不可提现到微信
	if total < 200 {
		return errors.New("低于2元无法提现到微信零钱")
	}
	tx := dao.DB.Begin()
	var err error
	withdraw := model.UserWithdraw{
		UserId:      retailerId,
		Amount:      total,
		WithdrawWay: 1,
		Status:      0,
		SuccessTime: 0,
		Remark:      "",
	}
	err = tx.Save(&withdraw).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	for _, val := range commissions {
		val.WithdrawId = withdraw.Id
		val.Status = config.CommissionStatusPaid
		err = tx.Where("id = ? and status = ?", val.Id, config.CommissionStatusWait).Updates(&val).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	tx.Commit()
	// 等待计划任务去处理
	return nil
}

func ExportOrders(req *request.OrderExportRequest) (header []string, content [][]interface{}) {
	var total int64
	tx := dao.DB.Model(&model.Order{}).Order("id asc")

	if req.Status != "" {
		// status 可能会传 waiting,delivery,finished
		if req.Status == "waiting" {
			tx = tx.Where("`status` = 0")
		}
		if req.Status == "paid" {
			tx = tx.Where("`status` = 1")
		}
		if req.Status == "delivery" {
			tx = tx.Where("`status` = 2")
		}
		if req.Status == "finished" {
			tx = tx.Where("`status` = 3")
		}
		if req.Status == "refunding" {
			tx = tx.Where("`status` = 8")
		}
	}
	if req.StartTime > 0 {
		tx = tx.Where("`created_time` >= ?", req.StartTime)
	}
	if req.EndTime > 0 {
		tx = tx.Where("`created_time` < ?", req.EndTime)
	}

	tx.Count(&total)

	//header
	header = []string{"下单时间", "支付时间", "订单ID", "订单状态", "订单金额", "订购数量", "图片地址", "购买用户", "分销用户", "分销佣金", "邀请用户", "邀请奖励", "收件人", "收件人电话", "收件地址", "快递公司", "快递单号"}
	content = [][]interface{}{}
	// 一次读取1000条
	var lastId uint = 0
	for {
		var orders []model.Order
		tx.Where("`id` > ?", lastId).Limit(1000).Find(&orders)
		if len(orders) == 0 {
			break
		}
		lastId = orders[len(orders)-1].Id

		var orderIds = make([]string, 0, len(orders))
		var userIds = make([]uint, 0, len(orders))
		var addressIds = make([]uint, 0, len(orders))
		for i := range orders {
			orderIds = append(orderIds, orders[i].OrderId)
			userIds = append(userIds, orders[i].UserId)
			if orders[i].ShareUserId > 0 {
				userIds = append(userIds, orders[i].UserId)
			}
			if orders[i].ShareParentUserId > 0 {
				userIds = append(userIds, orders[i].UserId)
			}
			addressIds = append(addressIds, orders[i].AddressId)
		}
		var details []*model.OrderDetail
		dao.DB.Where("`order_id` IN(?)", orderIds).Find(&details)
		if len(details) > 0 {
			var archiveIds = make([]uint, 0, len(details))
			for i := range details {
				archiveIds = append(archiveIds, details[i].GoodsId)
			}
			var archives []*model.Archive
			dao.DB.Where("`id` IN(?)", archiveIds).Find(&archives)
			for i := range details {
				for x := range archives {
					if archives[x].Id == details[i].GoodsId {
						details[i].Goods = archives[x]
					}
				}
			}
			for i := range orders {
				for j := range details {
					if orders[i].OrderId == details[j].OrderId {
						orders[i].Details = append(orders[i].Details, details[j])
					}
				}
			}
		}
		var addresses []model.OrderAddress
		dao.DB.Where("`id` IN(?)", addressIds).Find(&addresses)

		users := GetUsersInfoByIds(userIds)
		for i := range orders {
			var userName, shareName, parentName string
			for u := range users {
				if orders[i].UserId == users[u].Id {
					userName = users[u].UserName
				}
				if orders[i].ShareUserId == users[u].Id {
					shareName = users[u].UserName
				}
				if orders[i].UserId == users[u].Id {
					parentName = users[u].UserName
				}
			}
			var address model.OrderAddress
			for a := range addresses {
				if addresses[a].Id == orders[i].AddressId {
					address = addresses[a]
					break
				}
			}
			//content
			//[]string{"下单时间", "支付时间", "订单ID", "订单状态", "订单金额", "订购数量", "图片地址", "购买用户", "分销用户", "分销佣金", "邀请用户", "邀请奖励", "收件人", "收件人电话", "收件地址", "快递公司", "快递单号"}
			for d := range orders[i].Details {
				var logo string
				if orders[i].Details[d].Goods != nil {
					logo = orders[i].Details[d].Goods.Logo
				}

				content = append(content, []interface{}{
					time.Unix(orders[i].CreatedTime, 0).Format("2006-01-02 15:04:05"),
					time.Unix(orders[i].PaidTime, 0).Format("2006-01-02 15:04:05"),
					"," + orders[i].OrderId,
					getOrderStatus(orders[i].Status),
					orders[i].Details[d].Amount / 100,
					orders[i].Details[d].Quantity,
					logo,
					userName,
					shareName,
					orders[i].ShareAmount / 100,
					parentName,
					orders[i].ShareParentAmount / 100,
					address.Name,
					address.Phone,
					address.Province + address.City + address.Country + address.AddressInfo,
					orders[i].ExpressCompany,
					orders[i].TrackingNumber,
				})
			}
		}
	}

	return header, content
}

func getOrderStatus(status int) string {
	var text string
	switch status {
	case -1:
		text = "订单关闭"
		break
	case 0:
		text = "待支付"
		break
	case 1:
		text = "待发货"
		break
	case 2:
		text = "待支付"
		break
	case 3:
		text = "已完成"
		break
	case 8:
		text = "退款中"
		break
	case 9:
		text = "已退款"
		break
	}

	return text
}
