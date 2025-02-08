package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	"github.com/go-pay/gopay/paypal"
	"github.com/go-pay/gopay/wechat"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func (w *Website) GetOrderList(userId uint, orderId, userName, status string, page, pageSize int) ([]*model.Order, int64) {
	var orders []*model.Order
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.Order{})
	if userId > 0 {
		tx = tx.Where("`user_id` = ?", userId)
	}
	if orderId != "" {
		tx = tx.Where("`order_id` = ?", orderId)
	}
	if userName != "" {
		var userIds []uint
		w.DB.Model(&model.User{}).Where("`user_name` LIKE ?", "%"+userName+"%").Pluck("id", &userIds)
		tx = tx.Where("`user_id` IN (?)", userIds)
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
		w.DB.Where("`order_id` IN(?)", orderIds).Find(&details)
		if len(details) > 0 {
			var archiveIds = make([]int64, 0, len(details))
			for i := range details {
				archiveIds = append(archiveIds, details[i].GoodsId)
			}
			archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				return tx.Where("`id` IN(?)", archiveIds)
			}, "", 0, len(archiveIds))
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
						if orders[i].Type == config.OrderTypeVip {
							group, err := w.GetUserGroupInfo(uint(details[j].GoodsId))
							if err == nil {
								details[i].Group = group
							}
						}
						orders[i].Details = append(orders[i].Details, details[j])
					}
				}
			}
		}
		users := w.GetUsersInfoByIds(userIds)
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

func (w *Website) GetOrderInfoByOrderId(orderId string) (*model.Order, error) {
	var order model.Order
	err := w.DB.Where("`order_id` = ?", orderId).Take(&order).Error

	if err != nil {
		return nil, err
	}
	var details []*model.OrderDetail
	w.DB.Where("`order_id` = ?", order.OrderId).Find(&details)
	if len(details) > 0 {
		if order.Type == config.OrderTypeVip {
			group, err := w.GetUserGroupInfo(uint(details[0].GoodsId))
			if err == nil {
				details[0].Group = group
			}
		} else {
			var archiveIds = make([]int64, 0, len(details))
			for i := range details {
				archiveIds = append(archiveIds, details[i].GoodsId)
			}
			archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				return tx.Where("`id` IN(?)", archiveIds)
			}, "", 0, len(archiveIds))
			for i := range details {
				for x := range archives {
					if archives[x].Id == details[i].GoodsId {
						details[i].Goods = archives[x]
					}
				}
			}
		}
		order.Details = details
	}
	orderAddress, err := w.GetOrderAddressById(order.AddressId)
	if err == nil {
		order.OrderAddress = orderAddress
	}

	return &order, nil
}

func (w *Website) GetPaymentInfoByPaymentId(paymentId string) (*model.Payment, error) {
	var payment model.Payment
	err := w.DB.Where("`payment_id` = ?", paymentId).Take(&payment).Error

	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func (w *Website) GetPaymentInfoByTerraceId(terraceId string) (*model.Payment, error) {
	var payment model.Payment
	err := w.DB.Where("`terrace_id` = ?", terraceId).Take(&payment).Error

	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func (w *Website) GetPaymentInfoByOrderId(orderId string) (*model.Payment, error) {
	var payment model.Payment
	err := w.DB.Where("`order_id` = ?", orderId).Take(&payment).Error

	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func (w *Website) GeneratePayment(order *model.Order, payWay string) (*model.Payment, error) {
	payment, err := w.GetPaymentInfoByOrderId(order.OrderId)
	if err == nil {
		return payment, nil
	}

	payment = &model.Payment{
		UserId:  order.UserId,
		OrderId: order.OrderId,
		Amount:  order.Amount,
		Status:  0,
		Remark:  order.Remark,
		PayWay:  payWay,
	}
	err = w.DB.Save(payment).Error
	if err != nil {
		return nil, err
	}
	order.PaymentId = payment.PaymentId
	w.DB.Save(order)

	return payment, nil
}

func (w *Website) SetOrderDeliver(req *request.OrderRequest) error {
	order, err := w.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		return err
	}

	order.Status = config.OrderStatusDelivering
	order.DeliverTime = time.Now().Unix()
	order.ExpressCompany = req.ExpressCompany
	order.TrackingNumber = req.TrackingNumber
	order.EndTime = time.Now().AddDate(0, 0, w.PluginOrder.AutoFinishDay).Unix()
	w.DB.Save(order)

	return nil
}

func (w *Website) SetOrderFinished(order *model.Order) error {
	tx := w.DB.Begin()
	order.Status = config.OrderStatusCompleted
	order.FinishedTime = time.Now().Unix()
	err := tx.Save(order).Error
	if err != nil {
		tx.Rollback()
		return err
	}
	// 处理订单完成，并开始分钱
	// seller
	if order.SellerAmount > 0 {
		sellerCommission := model.Commission{
			UserId:      order.SellerId,
			OrderId:     order.OrderId,
			OrderAmount: order.Amount,
			Amount:      order.SellerAmount,
			Status:      0,
			WithdrawId:  0,
			Remark:      w.Tr("SalesRevenue"),
		}
		err = tx.Save(&sellerCommission).Error
		if err != nil {
			tx.Rollback()
			return err
		}
		tx.Model(model.User{}).Where("`id` = ?", order.SellerId).UpdateColumn("balance", gorm.Expr("`balance` + ?", order.SellerAmount))
		var userBalance int64
		err = tx.Model(&model.User{}).Where("`id` = ?", order.SellerId).Pluck("balance", &userBalance).Error
		//状态更改了，增加一条记录到用户
		finance := model.Finance{
			UserId:      order.SellerId,
			Direction:   config.FinanceIncome,
			Amount:      order.SellerAmount,
			AfterAmount: userBalance,
			Action:      config.FinanceActionSale,
			OrderId:     order.OrderId,
			Status:      1,
		}
		err = tx.Create(&finance).Error
		if err != nil {
			//
		}
		var totalReward response.SumAmount
		tx.Model(model.Commission{}).Where("`user_id` = ?", order.SellerId).Select("SUM(`amount`) as total").Take(&totalReward)
		tx.Model(model.User{}).Where("`id` = ?", order.SellerId).UpdateColumn("total_reward", totalReward.Total)
	}
	if order.ShareAmount > 0 {
		//
		shareUser, err := w.GetUserInfoById(order.ShareUserId)
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
			tx.Model(model.Commission{}).Where("`user_id` = ?", shareUser.Id).Select("SUM(`amount`) as total").Take(&totalReward)
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
			err = tx.Save(&shareAmount).Error
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
			tx.Model(model.Commission{}).Where("`user_id` = ?", order.ShareParentUserId).Select("SUM(`amount`) as total").Take(&totalReward)
			tx.Model(model.User{}).Where("`id` = ?", order.ShareParentUserId).UpdateColumn("total_reward", totalReward.Total)
		}
	}
	// 如果是vip订单，则还需要处理VIP信息
	if order.Type == config.OrderTypeVip {
		var user model.User
		var orderDetail model.OrderDetail
		err = tx.Model(model.User{}).Where("`id` = ?", order.UserId).Take(&user).Error
		err2 := tx.Model(model.OrderDetail{}).Where("`order_id` = ?", order.OrderId).Take(&orderDetail).Error
		if err == nil && err2 == nil {
			startTime := user.ExpireTime
			if startTime < time.Now().Unix() {
				startTime = time.Now().Unix()
			}
			var group model.UserGroup
			err = tx.Model(model.UserGroup{}).Where("`id` = ?", orderDetail.GoodsId).Take(&group).Error
			if err != nil {
				group.Setting.ExpireDay = 365
			}
			startTime += int64(group.Setting.ExpireDay) * 86400

			user.ExpireTime = startTime
			user.GroupId = group.Id
			tx.Save(&user)
		}
	}
	tx.Commit()

	return nil
}

func (w *Website) SetOrderCanceled(order *model.Order) error {
	order.Status = config.OrderStatusCanceled
	order.FinishedTime = time.Now().Unix()
	w.DB.Save(order)

	return nil
}

func (w *Website) SetOrderRefund(order *model.Order, status int) error {
	refund, err := w.GetOrderRefundByOrderId(order.OrderId)
	if err != nil {
		return err
	}
	// todo 金钱原路退回
	if status == 1 {
		payment, err := w.GetPaymentInfoByPaymentId(order.PaymentId)
		if err != nil {
			return err
		}
		if payment.PayWay == config.PayWayWechat {
			// 公众号支付
			client := wechat.NewClient(w.PluginPay.WechatAppId, w.PluginPay.WechatMchId, w.PluginPay.WechatApiKey, true)
			err := client.AddCertPemFilePath(w.DataPath+"cert/"+w.PluginPay.WechatCertPath, w.DataPath+"cert/"+w.PluginPay.WechatKeyPath)
			if err != nil {
				log.Println("微信证书错误：", err.Error())
				return err
			}

			bm := make(gopay.BodyMap)
			bm.Set("nonce_str", library.GenerateRandString(32)).
				Set("out_trade_no", order.PaymentId).
				Set("out_refund_no", refund.RefundId).
				Set("total_fee", order.Amount).
				Set("refund_fee", refund.Amount).
				Set("sign_type", wechat.SignType_MD5)

			wxRsp, _, err := client.Refund(context.Background(), bm)
			if err != nil {
				refund.Remark = err.Error()
				w.DB.Model(refund).UpdateColumn("remark", refund.Remark)
				return err
			}

			refund.Remark = wxRsp.ErrCodeDes
			w.DB.Model(refund).UpdateColumn("remark", refund.Remark)

			if wxRsp.ReturnCode == gopay.FAIL {
				return errors.New(wxRsp.ReturnMsg)
			}
			if wxRsp.ResultCode == gopay.FAIL {
				refund.Status = config.OrderRefundStatusFailed
				w.DB.Model(refund).UpdateColumn("status", refund.Status)
				return errors.New(wxRsp.ErrCodeDes)
			}
		} else if payment.PayWay == config.PayWayWeapp {
			// 小程序支付
			client := wechat.NewClient(w.PluginPay.WeappAppId, w.PluginPay.WechatMchId, w.PluginPay.WechatApiKey, true)
			err := client.AddCertPemFilePath(w.DataPath+"cert/"+w.PluginPay.WechatCertPath, w.DataPath+"cert/"+w.PluginPay.WechatKeyPath)
			if err != nil {
				log.Println("微信证书错误：", err.Error())
				return err
			}

			bm := make(gopay.BodyMap)
			bm.Set("nonce_str", library.GenerateRandString(32)).
				Set("out_trade_no", order.PaymentId).
				Set("out_refund_no", refund.RefundId).
				Set("total_fee", order.Amount).
				Set("refund_fee", refund.Amount).
				Set("sign_type", wechat.SignType_MD5)

			wxRsp, _, err := client.Refund(context.Background(), bm)
			if err != nil {
				refund.Remark = err.Error()
				w.DB.Model(refund).UpdateColumn("remark", refund.Remark)
				return err
			}

			refund.Remark = wxRsp.ErrCodeDes
			w.DB.Model(refund).UpdateColumn("remark", refund.Remark)

			if wxRsp.ReturnCode == gopay.FAIL {
				return errors.New(wxRsp.ReturnMsg)
			}
			if wxRsp.ResultCode == gopay.FAIL {
				refund.Status = config.OrderRefundStatusFailed
				w.DB.Model(refund).UpdateColumn("status", refund.Status)
				return errors.New(wxRsp.ErrCodeDes)
			}
		} else if payment.PayWay == config.PayWayAlipay {
			// 支付宝支付
			client, err := alipay.NewClient(w.PluginPay.AlipayAppId, w.PluginPay.AlipayPrivateKey, true)
			if err != nil {
				refund.Remark = err.Error()
				w.DB.Model(refund).UpdateColumn("remark", refund.Remark)
				return err
			}
			//配置公共参数
			client.SetCharset("utf-8").
				SetSignType(alipay.RSA2).
				SetNotifyUrl(w.System.BaseUrl + "/notify/alipay/pay")

			// 自动同步验签（只支持证书模式）
			certPath := w.DataPath + "cert/" + w.PluginPay.AlipayCertPath
			rootCertPath := w.DataPath + "cert/" + w.PluginPay.AlipayRootCertPath
			publicCertPath := w.DataPath + "cert/" + w.PluginPay.AlipayPublicCertPath
			publicKey, err := os.ReadFile(publicCertPath)
			if err != nil {
				refund.Remark = err.Error()
				w.DB.Model(refund).UpdateColumn("remark", refund.Remark)
				return err
			}
			client.AutoVerifySign(publicKey)

			// 传入证书内容
			err = client.SetCertSnByPath(certPath, rootCertPath, publicCertPath)
			if err != nil {
				refund.Remark = err.Error()
				w.DB.Model(refund).UpdateColumn("remark", refund.Remark)
				return err
			}
			//请求参数
			bm := make(gopay.BodyMap)
			bm.Set("out_trade_no", order.PaymentId)
			bm.Set("out_request_no", refund.RefundId)
			bm.Set("refund_amount", fmt.Sprintf("%.2f", float32(refund.Amount)/100))

			//创建订单
			resp, err := client.TradeRefund(context.Background(), bm)

			if err != nil {
				refund.Remark = err.Error()
				w.DB.Model(refund).UpdateColumn("remark", refund.Remark)
				return err
			}
			refund.Remark = resp.Response.Msg
			if resp.Response.SubCode != "" {
				refund.Remark = resp.Response.SubMsg
			}
			w.DB.Model(refund).UpdateColumn("remark", refund.Remark)

			if resp.Response.Code != "10000" || resp.Response.SubCode != "" {
				refund.Status = config.OrderRefundStatusFailed
				w.DB.Model(refund).UpdateColumn("status", refund.Status)
				return errors.New(refund.Remark)
			}
		} else if payment.PayWay == config.PayWayPaypal {
			// paypal refund
			client, err := paypal.NewClient(w.PluginPay.PaypalClientId, w.PluginPay.PaypalClientSecret, true)
			if err != nil {
				return err
			}
			bm := make(gopay.BodyMap)
			if payment.Amount > refund.Amount {
				// refund amount is less than capture amount
				bm.Set("note_to_payer", refund.Remark).
					SetBodyMap("amount", func(bm gopay.BodyMap) {
						bm.Set("currency_code", "USD"). // the same as capture
										Set("value", fmt.Sprintf("%.2f", float32(refund.Amount)/100))
					})
			}
			ppRsp, err := client.PaymentCaptureRefund(context.Background(), payment.TerraceId, bm)
			if err != nil {
				return err
			}
			if ppRsp.Code != http.StatusOK && ppRsp.Code != http.StatusCreated {
				return errors.New(ppRsp.ErrorResponse.Message)
			}

			// refund success
		} else {
			// 线下支付，的退款流程
			// 不用处理
		}

		refund.Status = config.OrderRefundStatusDone
		err = w.SuccessRefundOrder(refund, order)
		if err != nil {
			return err
		}
	} else {
		// 不同意
		order.RefundStatus = 0
		order.FinishedTime = time.Now().Unix()
		w.DB.Save(order)
		refund.Status = config.OrderStatusCanceled
		w.DB.Save(refund)
	}

	return nil
}

func (w *Website) ApplyOrderRefund(order *model.Order) error {
	// 用户申请退款
	refund, err := w.GetOrderRefundByOrderId(order.OrderId)
	if err == nil {
		return nil
	}

	refund = &model.OrderRefund{
		OrderId:  order.OrderId,
		DetailId: 0,
		UserId:   order.UserId,
		Amount:   order.Amount,
		Status:   0,
		Remark:   w.Tr("UserAppliesForARefund"),
	}
	w.DB.Save(refund)

	//order.Status = config.OrderStatusRefunding
	order.RefundStatus = config.OrderStatusRefunding
	w.DB.Save(order)

	return nil
}

func (w *Website) GetOrderRefundByOrderId(orderId string) (*model.OrderRefund, error) {
	var order model.OrderRefund
	if err := w.DB.Model(&model.OrderRefund{}).Where("`order_id` = ?", orderId).First(&order).Error; err != nil {
		return nil, err
	}

	return &order, nil
}

func (w *Website) SuccessPaidOrder(order *model.Order) error {
	if order.Status == config.OrderStatusPaid {
		//支付成功
		return nil
	}
	originStatus := order.Status

	order.PaidTime = time.Now().Unix()
	order.Status = config.OrderStatusPaid

	db := w.DB.Begin()
	if err := w.DB.Model(order).Where("status  = ?", originStatus).Select("paid_time", "status").Updates(order).Error; err != nil {
		db.Rollback()
		return err
	}

	db.Commit()

	// 支付成功
	if w.SendTypeValid(SendTypePayOrder) {
		subject := w.System.SiteName + "(" + w.System.BaseUrl + ")" + w.Tr("OrderPaymentSuccessNotification")
		content := w.Tr("OrderPaymentSuccessNotification:") + "\n" + w.Tr("OrderNumber:") + order.OrderId +
			"\n" + w.Tr("Amount:") + strconv.FormatFloat(float64(order.Amount)/100, 'f', 2, 64) +
			"\n" + w.Tr("PaymentTime:") + time.Unix(order.PaidTime, 0).Format("2006-01-02 15:04:05") +
			"\n" + w.Tr("PayingMemberId:") + strconv.Itoa(int(order.UserId))
		_ = w.sendMail(subject, content, nil, false, false)
	}

	if w.PluginOrder.NoProcess || order.Type == config.OrderTypeVip {
		// 如果订单自动完成，则在这里处理
		w.SetOrderFinished(order)
	}

	return nil
}

func (w *Website) SuccessRefundOrder(refund *model.OrderRefund, order *model.Order) error {
	var err error
	if order == nil {
		order, err = w.GetOrderInfoByOrderId(refund.OrderId)
		if err != nil {
			return err
		}
	}
	if order.Status == config.OrderStatusRefunded {
		// already refunded
		return nil
	}

	tx := w.DB.Begin()
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
	w.DB.Updates(refund)
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

func (w *Website) CreateOrder(userId uint, req *request.OrderRequest) (*model.Order, error) {
	user, err := w.GetUserInfoById(userId)
	if err != nil {
		return nil, err
	}
	if len(req.Details) == 0 && req.GoodsId == 0 {
		return nil, errors.New(w.Tr("PleaseSelectTheProduct"))
	}
	if len(req.Details) == 0 {
		req.Details = []request.OrderDetail{{GoodsId: req.GoodsId, Quantity: req.Quantity}}
	}
	tx := w.DB.Begin()
	var orderAddress *model.OrderAddress
	if w.PluginOrder.NoProcess == false || req.Address != nil {
		//保存订单地址
		orderAddress, err = w.SaveOrderAddress(tx, userId, req.Address)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	var amount int64
	var originAmount int64
	var remark = req.Remark
	var sellerId uint = 0
	if remark == "" {
		if req.Type == config.OrderTypeVip {
			group, err := w.GetUserGroupInfo(uint(req.Details[0].GoodsId))
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			remark += group.Title
		} else {
			for _, v := range req.Details {
				archive, err := w.GetArchiveById(v.GoodsId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				if archive.UserId > 0 {
					if sellerId == 0 {
						sellerId = archive.UserId
					}
					if sellerId != archive.UserId {
						tx.Rollback()
						return nil, errors.New(w.Tr("CrossStoreOrderingIsNotSupported"))
					}
				}
				if remark == "" {
					remark += archive.Title + w.Tr("Wait")
				}
			}
		}
	}

	order := model.Order{
		UserId:      userId,
		Remark:      remark,
		Type:        req.Type,
		Status:      0,
		ShareUserId: user.ParentId,
	}
	if orderAddress != nil {
		order.AddressId = orderAddress.Id
	}
	err = tx.Save(&order).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// 计算商品总价
	if req.Type == config.OrderTypeVip {
		group, err := w.GetUserGroupInfo(uint(req.Details[0].GoodsId))
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		//计算价格
		price := group.Price
		originPrice := price
		discount := w.GetUserDiscount(userId, user)
		if discount > 0 {
			price = originPrice * discount / 100
		}
		detailAmount := price * int64(req.Details[0].Quantity)
		originDetailAmount := originPrice * int64(req.Details[0].Quantity)
		amount += detailAmount
		originAmount += originDetailAmount
		//给每条子订单入库
		orderDetail := model.OrderDetail{
			OrderId:      order.OrderId,
			UserId:       userId,
			GoodsId:      req.Details[0].GoodsId,
			Price:        price,
			OriginPrice:  originPrice,
			Amount:       detailAmount,
			OriginAmount: originDetailAmount,
			Quantity:     req.Details[0].Quantity,
			Status:       1,
		}
		err = tx.Save(&orderDetail).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		for _, v := range req.Details {
			archive, err := w.GetArchiveById(v.GoodsId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			//计算价格
			price := archive.Price
			originPrice := price
			discount := w.GetUserDiscount(userId, user)
			if discount > 0 {
				price = originPrice * discount / 100
			}
			detailAmount := price * int64(v.Quantity)
			originDetailAmount := originPrice * int64(req.Details[0].Quantity)
			amount += detailAmount
			originAmount += originDetailAmount
			//给每条子订单入库
			orderDetail := model.OrderDetail{
				OrderId:      order.OrderId,
				UserId:       userId,
				GoodsId:      v.GoodsId,
				Price:        price,
				OriginPrice:  originPrice,
				Amount:       detailAmount,
				OriginAmount: originDetailAmount,
				Quantity:     v.Quantity,
				Status:       1,
			}
			err = tx.Save(&orderDetail).Error
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}
	order.Amount = amount
	order.OriginAmount = originAmount

	shareId := user.ParentId
	if w.PluginRetailer.AllowSelf == 1 && (w.PluginRetailer.BecomeRetailer == 1 || user.IsRetailer == 1) {
		shareId = user.Id
	}
	if shareId > 0 {
		shareUser, err := w.GetUserInfoById(shareId)
		if err == nil {
			if w.PluginRetailer.BecomeRetailer == 1 || shareUser.IsRetailer == 1 {
				shareGroup, err := w.GetUserGroupInfo(shareUser.GroupId)
				if err == nil && shareGroup.Setting.ShareReward > 0 {
					order.ShareAmount = order.Amount * shareGroup.Setting.ShareReward / 100
					// 如果上级也是分销员，则上级也获得推荐奖励
					if shareUser.ParentId > 0 && shareGroup.Setting.ParentReward > 0 {
						parent, err := w.GetUserInfoById(shareUser.ParentId)
						if err == nil {
							// 需要分给上级
							if w.PluginRetailer.BecomeRetailer == 1 || parent.IsRetailer == 1 {
								order.ShareParentAmount = order.Amount * shareGroup.Setting.ParentReward / 100
								order.ShareParentUserId = parent.Id
							}
						}
					}
				}
			}
		}
	}
	order.SellerId = sellerId
	if sellerId > 0 && w.PluginOrder.SellerPercent > 0 {
		sellerAmount := (order.Amount - order.ShareAmount - order.ShareParentAmount) * w.PluginOrder.SellerPercent / 100
		order.SellerAmount = sellerAmount
	}

	tx.Save(&order)

	tx.Commit()

	// 下单
	if w.SendTypeValid(SendTypeNewOrder) {
		subject := w.System.SiteName + "(" + w.System.BaseUrl + ")" + w.Tr("NewOrderNotification")
		content := w.Tr("NewOrderNotification:") + "\n" + w.Tr("OrderNumber:") + order.OrderId +
			"\n" + w.Tr("Amount:") + strconv.FormatFloat(float64(order.Amount)/100, 'f', 2, 64) +
			"\n" + w.Tr("OrderTime:") + time.Unix(order.CreatedTime, 0).Format("2006-01-02 15:04:05") +
			"\n" + w.Tr("OrderingMember:") + user.UserName +
			"\n" + w.Tr("OrderingMemberId:") + strconv.Itoa(int(order.UserId))
		_ = w.sendMail(subject, content, nil, false, false)
	}

	return &order, nil
}

func (w *Website) GetOrderAddressByUserId(userId uint) (*model.OrderAddress, error) {
	var orderAddress model.OrderAddress
	err := w.DB.Where("`user_id` = ?", userId).Order("id desc").Take(&orderAddress).Error
	if err != nil {
		return nil, err
	}

	return &orderAddress, nil
}

func (w *Website) GetOrderAddressById(id uint) (*model.OrderAddress, error) {
	if id == 0 {
		return nil, nil
	}
	var orderAddress model.OrderAddress
	err := w.DB.Where("`id` = ?", id).Take(&orderAddress).Error
	if err != nil {
		return nil, err
	}

	return &orderAddress, nil
}

func (w *Website) SaveOrderAddress(tx *gorm.DB, userId uint, req *request.OrderAddressRequest) (*model.OrderAddress, error) {
	if req == nil {
		return nil, nil
	}
	var orderAddress model.OrderAddress
	var err error
	if req.Id > 0 {
		err = tx.Where("`id` = ?", req.Id).Take(&orderAddress).Error
		if err != nil || orderAddress.UserId != userId {
			return nil, errors.New(w.Tr("AddressDoesNotExist"))
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

func (w *Website) GetRetailerOrders(retailerId uint, page, pageSize int) ([]*model.Order, int64) {
	var orders []*model.Order
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.Order{}).Where("(`share_user_id` = ? or `share_parent_user_id` = ?)", retailerId, retailerId)
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&orders)
	if len(orders) > 0 {
		var orderIds = make([]string, 0, len(orders))
		var userIds = make([]uint, 0, len(orders))
		for i := range orders {
			orderIds = append(orderIds, orders[i].OrderId)
			userIds = append(userIds, orders[i].UserId)
		}
		var details []*model.OrderDetail
		w.DB.Where("`order_id` IN(?)", orderIds).Find(&details)
		if len(details) > 0 {
			var archiveIds = make([]int64, 0, len(details))
			for i := range details {
				archiveIds = append(archiveIds, details[i].GoodsId)
			}
			archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				return tx.Where("`id` IN(?)", archiveIds)
			}, "", 0, len(archiveIds))
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
						if orders[i].Type == config.OrderTypeVip {
							group, err := w.GetUserGroupInfo(uint(details[j].GoodsId))
							if err == nil {
								details[i].Group = group
							}
						}
						orders[i].Details = append(orders[i].Details, details[j])
					}
				}
			}
		}
		users := w.GetUsersInfoByIds(userIds)
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

func (w *Website) GetRetailerWithdraws(retailerId uint, page, pageSize int) ([]*model.UserWithdraw, int64) {
	var withdraws []*model.UserWithdraw
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.UserWithdraw{}).Where("`user_id` = ?", retailerId)
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&withdraws)

	return withdraws, total
}

func (w *Website) GetRetailerCommissions(retailerId uint, page, pageSize int) ([]*model.Commission, int64) {
	var commissions []*model.Commission
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.Commission{}).Where("`user_id` = ?", retailerId)
	tx.Count(&total).Order("id desc").Limit(pageSize).Offset(offset).Find(&commissions)

	return commissions, total
}

func (w *Website) RetailerApplyWithdraw(retailerId uint) error {
	// 查询可提现金额
	var commissions []model.Commission
	var total int64
	w.DB.Model(&model.Commission{}).Where("`user_id` = ? and `status` = ?", retailerId, config.CommissionStatusWait).Find(&commissions)
	for i := range commissions {
		total += commissions[i].Amount
	}

	if total <= 0 {
		return errors.New(w.Tr("NoAmountAvailableForWithdrawal"))
	}

	// todo执行提现操作
	// 低于2元不可提现到微信
	if total < 200 {
		return errors.New(w.Tr("WithdrawalsBelow2YuanCannotBeMadeToWechatChange"))
	}
	tx := w.DB.Begin()
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

func (w *Website) ExportOrders(req *request.OrderExportRequest) (header []string, content [][]interface{}) {
	var total int64
	tx := w.DB.Model(&model.Order{}).Order("id asc")

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
	header = []string{w.Tr("OrderTime"), w.Tr("PaymentTime"), w.Tr("OrderId"), w.Tr("OrderStatus"), w.Tr("OrderAmount"), w.Tr("OrderedGoods"), w.Tr("OrderQuantity"), w.Tr("PurchasingUser"), w.Tr("DistributionUser"), w.Tr("DistributionCommission"), w.Tr("InvitedUser"), w.Tr("InvitationReward"), w.Tr("Recipient"), w.Tr("RecipientPhoneNumber"), w.Tr("ReceivingAddress"), w.Tr("CourierCompany"), w.Tr("CourierNumber")}
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
		w.DB.Where("`order_id` IN(?)", orderIds).Find(&details)
		if len(details) > 0 {
			var archiveIds = make([]int64, 0, len(details))
			for i := range details {
				archiveIds = append(archiveIds, details[i].GoodsId)
			}
			archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				return tx.Where("`id` IN(?)", archiveIds)
			}, "", 0, len(archiveIds))
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
						if orders[i].Type == config.OrderTypeVip {
							group, err := w.GetUserGroupInfo(uint(details[j].GoodsId))
							if err == nil {
								details[i].Group = group
							}
						}
						orders[i].Details = append(orders[i].Details, details[j])
					}
				}
			}
		}
		var addresses []model.OrderAddress
		w.DB.Where("`id` IN(?)", addressIds).Find(&addresses)

		users := w.GetUsersInfoByIds(userIds)
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
			//[]string{"下单时间", "支付时间", "订单ID", "订单状态", "订单金额", "订购商品", "订购数量", "购买用户", "分销用户", "分销佣金", "邀请用户", "邀请奖励", "收件人", "收件人电话", "收件地址", "快递公司", "快递单号"}
			for d := range orders[i].Details {
				var goodsTitle string
				if orders[i].Details[d].Group != nil {
					goodsTitle = w.Tr("Vip:") + orders[i].Details[d].Group.Title
				} else if orders[i].Details[d].Goods != nil {
					goodsTitle = w.Tr("Goods:") + orders[i].Details[d].Goods.Title
				}

				content = append(content, []interface{}{
					time.Unix(orders[i].CreatedTime, 0).Format("2006-01-02 15:04:05"),
					time.Unix(orders[i].PaidTime, 0).Format("2006-01-02 15:04:05"),
					"," + orders[i].OrderId,
					w.getOrderStatus(orders[i].Status),
					orders[i].Details[d].Amount / 100,
					goodsTitle,
					orders[i].Details[d].Quantity,
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

var checkOrderRunning = false

func (w *Website) AutoCheckOrders() {
	if w.DB == nil {
		return
	}
	if checkOrderRunning {
		return
	}
	checkOrderRunning = true
	defer func() {
		checkOrderRunning = false
	}()

	currentStamp := time.Now().Unix()
	// auto close order
	if w.PluginOrder.AutoCloseMinute > 0 {
		closeStamp := currentStamp - w.PluginOrder.AutoCloseMinute*60
		// check the order that was paid or not
		var orders []model.Order
		w.DB.Model(&model.Order{}).Where("`status` = ? and created_time < ?", config.OrderStatusWaiting, closeStamp).Find(&orders)
		for _, order := range orders {
			payment, err := w.GetPaymentInfoByOrderId(order.OrderId)
			if err != nil {
				w.DB.Model(&order).Update("status", config.OrderStatusCanceled)
				continue
			}
			_ = w.TraceQuery(payment)
			if payment.PaidTime > 0 {
				// 支付成功
				// 这里不需要操作
			} else {
				w.DB.Model(&order).Update("status", config.OrderStatusCanceled)
			}
		}
	}
	// auto finish order
	var orders []*model.Order
	w.DB.Where("`status` = ? and end_time < ?", config.OrderStatusDelivering, currentStamp).Find(&orders)
	if len(orders) > 0 {
		for _, v := range orders {
			_ = w.SetOrderFinished(v)
		}
	}
}

func (w *Website) TraceQuery(payment *model.Payment) error {
	if payment.PayWay == config.PayWayAlipay {
		client, err := alipay.NewClient(w.PluginPay.AlipayAppId, w.PluginPay.AlipayPrivateKey, true)
		if err != nil {
			return err
		}
		// 请求参数
		bm := make(gopay.BodyMap)
		bm.Set("out_trade_no", payment.PaymentId)

		// 查询订单
		aliRsp, err := client.TradeQuery(context.Background(), bm)
		if err != nil {
			if bizErr, ok := alipay.IsBizError(err); ok {
				log.Printf("%+v", bizErr)
				// do something
				return err
			}
			return err
		}

		// 自动同步验签（只支持证书模式）
		certPath := fmt.Sprintf(w.DataPath + "cert/" + w.PluginPay.AlipayCertPath)
		rootCertPath := fmt.Sprintf(w.DataPath + "cert/" + w.PluginPay.AlipayRootCertPath)
		publicCertPath := fmt.Sprintf(w.DataPath + "cert/" + w.PluginPay.AlipayPublicCertPath)
		publicKey, err := os.ReadFile(publicCertPath)
		if err != nil {
			return err
		}
		client.AutoVerifySign(publicKey)

		// 传入证书内容
		err = client.SetCertSnByPath(certPath, rootCertPath, publicCertPath)
		if err != nil {
			return err
		}

		if aliRsp.Response.TradeStatus == "TRADE_SUCCESS" {
			// this is a pay order
			payment.PaidTime = time.Now().Unix()
			payment.TerraceId = aliRsp.Response.TradeNo
			payment.BuyerId = aliRsp.Response.BuyerUserId
			if aliRsp.Response.BuyerOpenId != "" {
				payment.BuyerId = aliRsp.Response.BuyerOpenId
			}
			payment.BuyerInfo = aliRsp.Response.BuyerLogonId
			w.DB.Save(payment)
			order, err2 := w.GetOrderInfoByOrderId(payment.OrderId)
			if err2 != nil {
				return err2
			}
			order.PaymentId = payment.PaymentId
			w.DB.Save(order)
			//生成用户支付记录
			var userBalance int64
			err = w.DB.Model(&model.User{}).Where("`id` = ?", payment.UserId).Pluck("balance", &userBalance).Error
			//状态更改了，增加一条记录到用户
			finance := model.Finance{
				UserId:      payment.UserId,
				Direction:   config.FinanceOutput,
				Amount:      payment.Amount,
				AfterAmount: userBalance,
				Action:      config.FinanceActionBuy,
				OrderId:     payment.OrderId,
				Status:      1,
			}
			w.DB.Create(&finance)
			//支付成功逻辑处理
			_ = w.SuccessPaidOrder(order)
		}
	} else if payment.PayWay == config.PayWayWechat {
		// 微信就不管了
	} else if payment.PayWay == config.PayWayWeapp {
		// 微信就不管了
	} else if payment.PayWay == config.PayWayPaypal {
		client, err := paypal.NewClient(w.PluginPay.PaypalClientId, w.PluginPay.PaypalClientSecret, true)
		if err != nil {
			log.Println("client err", err)
			return err
		}
		bm := make(gopay.BodyMap)
		ppRsp, err := client.OrderDetail(context.Background(), payment.TerraceId, bm)
		if err != nil {
			return err
		}
		if ppRsp.Code == 0 {
			if ppRsp.Response.Status == "APPROVED" {
				// 需要confirm
				_, _ = client.OrderCapture(context.Background(), payment.TerraceId, nil)
			}
			// 更新payment
			payment.PayWay = config.PayWayPaypal
			payment.PaidTime = time.Now().Unix()
			payment.BuyerId = ppRsp.Response.Payer.PayerId
			buf, _ := json.Marshal(ppRsp.Response.PaymentSource)
			payment.BuyerInfo = string(buf)
			w.DB.Save(payment)
			order, err2 := w.GetOrderInfoByOrderId(payment.OrderId)
			if err2 != nil {
				return err2
			}
			order.PaymentId = payment.PaymentId
			w.DB.Save(order)
			//生成用户支付记录
			var userBalance int64
			err = w.DB.Model(&model.User{}).Where("`id` = ?", payment.UserId).Pluck("balance", &userBalance).Error
			//状态更改了，增加一条记录到用户
			finance := model.Finance{
				UserId:      payment.UserId,
				Direction:   config.FinanceOutput,
				Amount:      payment.Amount,
				AfterAmount: userBalance,
				Action:      config.FinanceActionBuy,
				OrderId:     payment.OrderId,
				Status:      1,
			}
			w.DB.Create(&finance)
			//支付成功逻辑处理
			_ = w.SuccessPaidOrder(order)
		}
	}

	return nil
}

func (w *Website) getOrderStatus(status int) string {
	var text string
	switch status {
	case -1:
		text = w.Tr("OrderClosed")
		break
	case 0:
		text = w.Tr("WaitingForPayment")
		break
	case 1:
		text = w.Tr("WaitingForShipment")
		break
	case 2:
		text = w.Tr("WaitingForPayment")
		break
	case 3:
		text = w.Tr("Completed")
		break
	case 8:
		text = w.Tr("RefundInProgress")
		break
	case 9:
		text = w.Tr("Refunded")
		break
	}

	return text
}
