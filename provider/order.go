package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jinzhu/now"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	"github.com/go-pay/gopay/paypal"
	"github.com/go-pay/gopay/wechat"
	_ "github.com/matm/go-nowpayments/payments"
	"github.com/stripe/stripe-go/v82"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func (w *Website) GetOrderList(ops func(tx *gorm.DB) *gorm.DB, status string, page, pageSize int) ([]*model.Order, int64) {
	var orders []*model.Order
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.Order{})
	if ops != nil {
		tx = ops(tx)
	}
	if status != "" {
		// status 可能会传 waiting,paid,delivery,finished,refunding,closed
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
		if status == "closed" {
			tx = tx.Where("`status` = -1")
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
			var skuIds = make([]int64, 0, len(details))
			for i := range details {
				archiveIds = append(archiveIds, details[i].GoodsId)
				if details[i].GoodsSkuId > 0 {
					skuIds = append(skuIds, details[i].GoodsSkuId)
				}
			}
			archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				return tx.Where("`id` IN(?)", archiveIds)
			}, "", 0, len(archiveIds))
			var skus []*model.ArchiveSku
			w.DB.Where("id IN(?)", skuIds).Find(&skus)
			for i := range details {
				for x := range archives {
					if archives[x].Id == details[i].GoodsId {
						details[i].Goods = archives[x]
					}
				}
				for x := range skus {
					if skus[x].Id == details[i].GoodsSkuId {
						details[i].GoodsSku = skus[x]
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
		} else if order.Type == config.OrderTypeCommunication {
			communicationOrder, err := w.GetCommunicationOrderById(details[0].GoodsId)
			if err == nil {
				communicationOrder.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
				details[0].Communication = communicationOrder
			}
		} else {
			var archiveIds = make([]int64, 0, len(details))
			var skuIds = make([]int64, 0, len(details))
			for i := range details {
				archiveIds = append(archiveIds, details[i].GoodsId)
				if details[i].GoodsSkuId > 0 {
					skuIds = append(skuIds, details[i].GoodsSkuId)
				}
			}
			archives, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				return tx.Where("`id` IN(?)", archiveIds)
			}, "", 0, len(archiveIds))
			for i := range details {
				for x := range archives {
					if archives[x].Id == details[i].GoodsId {
						details[i].Goods = archives[x]
						details[i].OrderFields = archives[x].OrderFields
					}
				}
			}
			if len(skuIds) > 0 {
				var skus []*model.ArchiveSku
				w.DB.Where("id IN (?)", skuIds).Find(&skus)
				for i := range details {
					for x := range skus {
						if skus[x].Id == details[i].GoodsSkuId {
							details[i].GoodsSku = skus[x]
						}
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
	// order services
	order.Services, err = w.GetOrderServicesByOrderId(order.OrderId)
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

func (w *Website) GeneratePayment(order *model.Order, req *request.PaymentRequest) (*model.Payment, error) {
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
		PayWay:  req.PayWay,
	}
	err = w.DB.Save(payment).Error
	if err != nil {
		return nil, err
	}
	order.PaymentId = payment.PaymentId
	w.DB.Save(order)

	_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypePayCreate, Id: int64(order.Id)})

	// 自动生成 invoice
	template, exist := w.GetEmailTemplateInfo("invoice")
	if exist && template.Open && w.PluginSendmail.UseBrevo == false {
		invoicePath, err := w.GenerateInvoice(order)
		if err == nil {
			// 成功生成发票
			w.DB.Model(&order).Update("invoice_path", invoicePath)
			user, err := w.GetUserInfoById(order.UserId)
			// 发送邮件
			if err == nil && user.Email != "" {
				go w.SendInvoiceMail(user, order, invoicePath)
			}
		}
	}

	return payment, nil
}

func (w *Website) SetOrderDeliver(req *request.OrderDeliveryRequest) error {
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

	user, err := w.GetUserInfoById(order.UserId)
	if err == nil {
		w.SendDeliveryOrderEmail(user, order)
		// 推送事件
		w.CreateEvent(EventOrderDelivered, map[string]interface{}{
			"order_id": order.OrderId,
			"amount":   float32(order.Amount) / 100,
			"status":   "delivering",
		}, user)
	}

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
			var oldGroup model.UserGroup
			w.DB.Where("id = ?", user.GroupId).First(&oldGroup)

			var group model.UserGroup
			err = tx.Model(model.UserGroup{}).Where("`id` = ?", orderDetail.GoodsId).Take(&group).Error
			if err != nil || group.Setting.ExpireDay == 0 {
				group.Setting.ExpireDay = 365
			}
			if group.Setting.ExpireDay == 365 {
				startTime = time.Unix(startTime, 0).AddDate(1*orderDetail.Quantity, 0, 0).Unix()
			} else if group.Setting.ExpireDay == 30 {
				startTime = time.Unix(startTime, 0).AddDate(0, 1*orderDetail.Quantity, 0).Unix()
			} else {
				startTime += int64(group.Setting.ExpireDay*orderDetail.Quantity) * 86400
			}

			user.ExpireTime = startTime
			user.GroupId = group.Id
			tx.Save(&user)

			user.Group = &group
			// 发送用户组变更邮件
			w.SendUserGroupChangeEmail(&user, &oldGroup)
		}
	}
	tx.Commit()

	user, err := w.GetUserInfoById(order.UserId)
	if err == nil {
		w.SendCompletedOrderEmail(user, order)
		// 推送事件
		w.CreateEvent(EventOrderCompleted, map[string]interface{}{
			"order_id": order.OrderId,
			"amount":   float32(order.Amount) / 100,
			"status":   "completed",
		}, &model.User{Id: uint(order.UserId)})
		// 发送评价邀请
		w.SendReviewOrderPlan(user, order)
		w.SendRecoverAbandonedOrderPlan(user, order)
	}

	return nil
}

func (w *Website) SetOrderCanceled(order *model.Order) error {
	order.Status = config.OrderStatusCanceled
	order.FinishedTime = time.Now().Unix()
	w.DB.Model(order).UpdateColumns(map[string]interface{}{
		"status":        order.Status,
		"finished_time": order.FinishedTime,
	})
	w.DB.Model(&model.Payment{}).Where("order_id = ?", order.OrderId).Update("status", config.OrderStatusCanceled)
	// 退还库存
	if order.Type != config.OrderTypeVip {
		var details []*model.OrderDetail
		w.DB.Where("order_id = ?", order.OrderId).Find(&details)
		for _, detail := range details {
			if detail.GoodsSkuId > 0 {
				w.DB.Model(&model.ArchiveSku{}).Where("id = ?", detail.GoodsSkuId).UpdateColumn("stock", gorm.Expr("stock + ?", detail.Quantity))
			}
			if detail.GoodsId > 0 {
				w.DB.Model(&model.Archive{}).Where("id = ?", detail.GoodsId).UpdateColumn("stock", gorm.Expr("stock + ?", detail.Quantity))
			}
			// 退回属性值扣减的库存
			var records []*model.OrderOptionStock
			w.DB.Where("order_detail_id = ?", detail.Id).Find(&records)
			for _, record := range records {
				w.DB.Model(&model.ArchiveOptionValue{}).Where("id = ?", record.OptionValueId).UpdateColumn("stock", gorm.Expr("stock + ?", record.Quantity))
			}
		}
	}

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
			account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
			if err != nil {
				return err
			}
			client := wechat.NewClient(account.PayConfig.AppId, account.PayConfig.Account, account.PayConfig.ApiKey, !account.PayConfig.Sandbox)
			err = client.AddCertPemFilePath(w.DataPath+"cert/"+account.PayConfig.CertPath, w.DataPath+"cert/"+account.PayConfig.PublicCertPath)
			if err != nil {
				log.Println("微信证书错误：", err.Error())
				return err
			}

			bm := make(gopay.BodyMap)
			bm.Set("nonce_str", config.GenerateRandString(32)).
				Set("out_trade_no", order.PaymentId).
				Set("out_refund_no", refund.RefundId).
				Set("total_fee", order.Amount).
				Set("refund_fee", refund.Amount).
				Set("sign_type", wechat.SignType_HMAC_SHA256)

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
			account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
			if err != nil {
				return err
			}
			client := wechat.NewClient(account.PayConfig.AppId, account.PayConfig.Account, account.PayConfig.ApiKey, !account.PayConfig.Sandbox)
			err = client.AddCertPemFilePath(w.DataPath+"cert/"+account.PayConfig.CertPath, w.DataPath+"cert/"+account.PayConfig.PublicCertPath)
			if err != nil {
				log.Println("微信证书错误：", err.Error())
				return err
			}

			bm := make(gopay.BodyMap)
			bm.Set("nonce_str", config.GenerateRandString(32)).
				Set("out_trade_no", order.PaymentId).
				Set("out_refund_no", refund.RefundId).
				Set("total_fee", order.Amount).
				Set("refund_fee", refund.Amount).
				Set("sign_type", wechat.SignType_HMAC_SHA256)

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
			account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
			if err != nil {
				return err
			}
			client, err := alipay.NewClient(account.PayConfig.AppId, account.PayConfig.AppSecret, !account.PayConfig.Sandbox)
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
			certPath := w.DataPath + "cert/" + account.PayConfig.CertPath
			rootCertPath := w.DataPath + "cert/" + account.PayConfig.RootCertPath
			publicCertPath := w.DataPath + "cert/" + account.PayConfig.PublicCertPath
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
			bm.Set("refund_amount", fmt.Sprintf("%.2f", float64(refund.Amount)/100))

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
			account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
			if err != nil {
				return err
			}
			client, err := paypal.NewClient(account.PayConfig.AppId, account.PayConfig.AppSecret, !account.PayConfig.Sandbox)
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
		} else if payment.PayWay == config.PayWayStripe {
			// stripe refund
			// todo stripe
			account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
			if err != nil {
				return err
			}
			client := stripe.NewClient(account.PayConfig.AppId)

			params := &stripe.RefundCreateParams{
				//Amount: stripe.Int64(payment.Amount),
				//Currency: stripe.String(string(stripe.CurrencyUSD)),
				PaymentIntent: stripe.String(payment.TerraceId),
			}
			r, err := client.V1Refunds.Create(context.Background(), params)
			if err != nil {
				return fmt.Errorf("stripe create refund payment error: %v", err)
			}
			if r.Status == "failed" {
				return errors.New("stripe create refund payment error")
			}
			// 更新id
			// refund success
		} else if payment.PayWay == config.PayWayNowpayment {
			// nowpayment refund
			// todo nowpayment
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

	// 推送事件
	w.CreateEvent(EventOrderRefund, map[string]interface{}{
		"order_id":      order.OrderId,
		"amount":        float64(order.Amount) / 100,
		"refund_amount": float64(refund.Amount) / 100,
		"refund_reason": refund.Remark,
		"status":        "refunded",
	}, &model.User{Id: uint(order.UserId)})

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

	// 发送邮件
	if order.UserId > 0 {
		user, err := w.GetUserInfoById(order.UserId)
		if err == nil {
			// 推送事件
			w.CreateEvent(EventOrderRefund, map[string]interface{}{
				"order_id":      order.OrderId,
				"amount":        float64(order.Amount) / 100,
				"refund_amount": float64(refund.Amount) / 100,
				"refund_reason": refund.Remark,
				"status":        "request",
			}, user)

			w.SendRefundOrderEmail(user, order)
		}
	}

	return nil
}

func (w *Website) GetOrderRefundByOrderId(orderId string) (*model.OrderRefund, error) {
	var order model.OrderRefund
	if err := w.DB.Model(&model.OrderRefund{}).Where("`order_id` = ?", orderId).First(&order).Error; err != nil {
		return nil, err
	}

	return &order, nil
}

func (w *Website) SuccessPaidOrder(order *model.Order, payment *model.Payment) error {
	// 统计payment账号
	if payment.PaymentAccountId > 0 {
		go func() {
			var statistic model.PaymentStatistic
			err := w.DB.Where("account_id = ?", payment.PaymentAccountId).Last(&statistic).Error
			if err != nil {
				statistic = model.PaymentStatistic{
					AccountId: payment.PaymentAccountId,
				}
			}
			monthStamp := now.BeginningOfMonth().Unix()
			todayStamp := now.BeginningOfDay().Unix()
			if statistic.StatTime < monthStamp {
				// 重新统计
				statistic = model.PaymentStatistic{
					AccountId: payment.PaymentAccountId,
				}
			}
			if statistic.StatTime < todayStamp {
				// 统计今天
				statistic = model.PaymentStatistic{
					AccountId:     payment.PaymentAccountId,
					MonthlyAmount: statistic.MonthlyAmount,
				}
			}
			statistic.AccountId = payment.PaymentAccountId
			statistic.StatTime = todayStamp
			statistic.DailyAmount += payment.Amount
			statistic.MonthlyAmount += payment.Amount
			statistic.DailyCount += 1
			w.DB.Save(&statistic)
			// 更新Account的统计信息
			w.DB.Model(model.PaymentAccount{}).Where("id = ?", payment.PaymentAccountId).UpdateColumns(map[string]interface{}{
				"last_used_time": time.Now().Unix(),
				"used_count":     gorm.Expr("`used_count` + 1"),
				"used_amount":    gorm.Expr("`used_amount` + ?", payment.Amount),
			})
		}()
	}

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
	// 更新用户字段
	w.DB.Model(model.User{}).Where("id = ?", order.UserId).UpdateColumn("order_count", gorm.Expr("`order_count` + 1"))
	// 支付成功
	_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypePayment, Id: int64(order.Id)})
	// 复购率，新客，老客户
	// 检查之前有没有付款成功的订单
	var existPaidOrder int64
	w.DB.Model(&model.Order{}).Where("user_id = ? and paid_time > 0 and order_id != ?", order.UserId, order.OrderId).Count(&existPaidOrder)
	if existPaidOrder > 0 {
		// 重复购买
		_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypePayment, Id: int64(order.Id)})
	} else {
		// 新客
		_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypePayment, Id: int64(order.Id)})
	}
	// end
	user, err := w.GetUserInfoById(order.UserId)
	if err != nil {
		user = &model.User{Id: uint(order.UserId)}
		orderAddress, err := w.GetOrderAddressById(order.AddressId)
		if err == nil {
			user.Email = orderAddress.Email
			user.Phone = orderAddress.Phone
			user.UserName = orderAddress.Name
		}
	}
	w.SendPayOrderEmail(user, order)
	w.CancelMailPlan("recover_abandoned_checkout_"+order.OrderId, true)
	w.SendOrderCustomFormEmail(user, order)

	// 发送一张未付款的发票
	template, exist := w.GetEmailTemplateInfo("invoice")
	if exist && template.Open && w.PluginSendmail.UseBrevo == false {
		invoicePath, err := w.GenerateInvoice(order)
		if err == nil {
			// 成功生成发票
			w.DB.Model(&order).Update("invoice_path", invoicePath)
			// 发送邮件
			if user.Email != "" {
				go w.SendInvoiceMail(user, order, invoicePath)
			}
		}
	}
	// 推送事件
	w.CreateEvent(EventOrderPaid, map[string]interface{}{
		"order_id": order.OrderId,
		"amount":   float32(order.Amount) / 100,
		"status":   "paid",
	}, user)

	// communication order
	if order.Type == config.OrderTypeCommunication {
		communicationOrder, err := w.GetCommunicationOrderByOrderId(order.OrderId)
		if err == nil {
			w.DB.Model(communicationOrder).UpdateColumn("status", config.CommunicationStatusFinished)
		}
	}
	w.SendPayOrderEmail(user, order)

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
	// 退还库存
	if order.Type != config.OrderTypeVip {
		var details []*model.OrderDetail
		w.DB.Where("order_id = ?", order.OrderId).Find(&details)
		for _, detail := range details {
			if detail.GoodsSkuId > 0 {
				w.DB.Model(&model.ArchiveSku{}).Where("id = ?", detail.GoodsSkuId).UpdateColumn("stock", gorm.Expr("stock + ?", detail.Quantity))
			}
			if detail.GoodsId > 0 {
				w.DB.Model(&model.Archive{}).Where("id = ?", detail.GoodsId).UpdateColumn("stock", gorm.Expr("stock + ?", detail.Quantity))
			}
			// 退回属性值扣减的库存
			var records []*model.OrderOptionStock
			w.DB.Where("order_detail_id = ?", detail.Id).Find(&records)
			for _, record := range records {
				w.DB.Model(&model.ArchiveOptionValue{}).Where("id = ?", record.OptionValueId).UpdateColumn("stock", gorm.Expr("stock + ?", record.Quantity))
			}
		}
	}

	tx.Commit()

	// 发送邮件
	if order.UserId > 0 {
		user, err := w.GetUserInfoById(order.UserId)
		if err == nil {
			w.SendRefundOrderEmail(user, order)
		}
	}

	return err
}

func (w *Website) CreateOrder(userId uint, req *request.OrderRequest) (*model.Order, error) {
	user, err := w.GetUserInfoById(userId)
	if err != nil && w.PluginOrder.NoNeedLogin == false {
		return nil, err
	}
	if user == nil {
		// 匿名用户
		user = &model.User{}
		if req.Address != nil {
			user.UserName = req.Address.Name + " " + req.Address.LastName
			user.Email = req.Address.Email
			user.Phone = req.Address.Phone
			if user.Email != "" {
				existUser, err := w.GetUserInfoByEmail(user.Email)
				if err == nil {
					user = existUser
				}
			}
			if user.Phone != "" {
				existUser, err := w.GetUserInfoByPhone(user.Phone)
				if err == nil {
					user = existUser
				}
			}
			// 如果用户没有注册，则这里给它注册一下
			if user.Id == 0 {
				// 未注册
				user.FirstName = req.Address.Name
				user.LastName = req.Address.LastName
				user.RealName = req.Address.Name + " " + req.Address.LastName
				user.ResetPassword = true // 需要重置密码
				if req.Address.Subscribed != "" && req.Address.Subscribed != "false" {
					user.Subscribed = true
				}

				w.DB.Save(user)
			}
			userId = user.Id
		}
	}
	if len(req.Details) == 0 && req.GoodsId == 0 {
		return nil, errors.New(w.Tr("PleaseSelectTheProduct"))
	}
	if len(req.Details) == 0 {
		req.Details = []request.OrderDetail{{GoodsId: req.GoodsId, GoodsSkuId: req.GoodsSkuId, Quantity: req.Quantity}}
	}
	var couponCode *model.CouponCode
	// 使用优惠券
	if req.CouponCode != "" {
		couponCode, err = w.GetCouponCodeByCode(req.CouponCode)
		if err != nil {
			return nil, err
		}
	} else if req.CouponCodeId > 0 {
		couponCode, err = w.GetCouponCodeById(req.CouponCodeId)
		if err != nil {
			return nil, err
		}
	} else if req.CouponId > 0 {
		// 优惠券
		coupon, err := w.GetCouponById(req.CouponId)
		if err != nil {
			return nil, err
		}
		// 生成一个优惠券码
		couponCode, err = w.MakeCouponCode(coupon.Id, int64(userId), 1)
		if err != nil || couponCode == nil {
			return nil, err
		}
		couponCode.Coupon = coupon
	}
	var activity *model.Activity
	if req.ActivityId > 0 {
		// 参加活动，只能购买单个商品
		activity, err = w.GetActivityById(req.ActivityId)
		if err != nil {
			return nil, err
		}
		for _, detail := range req.Details {
			if detail.GoodsId != activity.GoodsId {
				return nil, errors.New(w.Tr("PleaseSelectTheProduct"))
			}
		}
		if couponCode != nil && activity.CanUseCoupon == false {
			return nil, errors.New(w.Tr("CouponCannotBeUsed"))
		}
	}
	tx := w.DB.Begin()
	var orderAddress *model.OrderAddress
	if w.PluginOrder.NoProcess == false && req.Address != nil {
		//保存订单地址
		orderAddress, err = w.SaveOrderAddress(tx, userId, req.Address)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if req.Type == config.OrderTypeCommunication {
		// 先接受报价
		err = w.CommunicationAcceptQuote(req.Details[0].GoodsId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	var amount int64
	var originAmount int64
	var userDiscountAmount int64
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
		} else if req.Type == config.OrderTypeCommunication {
			communicationOrder, err := w.GetCommunicationOrderById(req.Details[0].GoodsId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			remark += communicationOrder.Title
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
					remark += archive.Title
					if len(req.Details) > 1 {
						remark += "..."
					}
					break
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
	} else if req.AddressId > 0 {
		orderAddress, _ = w.GetOrderAddressById(req.AddressId)
	}
	err = tx.Save(&order).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	var details = []map[string]interface{}{}

	// 如果商品标记了包邮，则不参与计算运费
	var calcQuantity int = 0
	var calcWeight float64 = 0
	var calcVolume float64 = 0
	userDiscount, shippingDiscount := w.GetUserDiscount(userId, user)
	// 计算商品总价
	if req.Type == config.OrderTypeVip {
		group, err := w.GetUserGroupInfo(uint(req.Details[0].GoodsId))
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if req.Details[0].Quantity == 0 {
			req.Details[0].Quantity = 1
		}
		//计算价格
		price := group.Price
		originPrice := price
		if activity != nil {
			price = activity.Price
		}
		if userDiscount > 0 {
			price = price * userDiscount / 100
		}
		detailAmount := price * int64(req.Details[0].Quantity)
		originDetailAmount := originPrice * int64(req.Details[0].Quantity)
		userDiscountAmount += originDetailAmount - detailAmount
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
		details = append(details, map[string]interface{}{
			"id":       orderDetail.GoodsId,
			"title":    group.Title,
			"sku_id":   orderDetail.GoodsSkuId,
			"link":     "",
			"logo":     "",
			"price":    float32(price) / 100,
			"amount":   detailAmount,
			"quantity": orderDetail.Quantity,
		})
	} else if req.Type == config.OrderTypeCommunication {
		communicationOrder, err := w.GetCommunicationOrderById(req.Details[0].GoodsId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		w.DB.Model(&model.CommunicationOrder{}).Where("id = ?", communicationOrder.Id).UpdateColumn("order_id", order.OrderId)
		if req.Details[0].Quantity == 0 {
			req.Details[0].Quantity = 1
		}
		//计算价格
		price := communicationOrder.QuotePrice
		originPrice := price
		if activity != nil {
			price = activity.Price
		}
		if userDiscount > 0 {
			price = price * userDiscount / 100
		}
		detailAmount := price * int64(req.Details[0].Quantity)
		originDetailAmount := originPrice * int64(req.Details[0].Quantity)
		userDiscountAmount += originDetailAmount - detailAmount
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
			SkuValues:    req.Details[0].SkuValues,
			Status:       1,
		}
		err = tx.Save(&orderDetail).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		details = append(details, map[string]interface{}{
			"id":       orderDetail.GoodsId,
			"title":    communicationOrder.Title,
			"sku_id":   orderDetail.GoodsSkuId,
			"link":     "",
			"logo":     communicationOrder.Logo,
			"price":    float32(price) / 100,
			"amount":   detailAmount,
			"quantity": orderDetail.Quantity,
		})
	} else {
		for _, v := range req.Details {
			archive, err := w.GetArchiveById(v.GoodsId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			if v.Quantity == 0 {
				v.Quantity = 1
			}
			if archive.MinQuantity > 0 && v.Quantity < archive.MinQuantity {
				v.Quantity = archive.MinQuantity
			}
			if archive.MaxQuantity > 0 && v.Quantity > archive.MaxQuantity {
				v.Quantity = archive.MaxQuantity
			}
			//计算价格
			price := archive.Price
			originPrice := price
			if activity != nil {
				price = activity.Price
			}
			// sku_id
			var archiveSku *model.ArchiveSku
			var skuValues []model.ArchiveOptionValue
			if v.GoodsSkuId > 0 {
				archiveSku, err = w.GetArchiveSkuById(v.GoodsId, v.GoodsSkuId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				// 如果数量超了，则不允许下单
				if archive.AllowOversold == false && int(archiveSku.Stock) < v.Quantity {
					tx.Rollback()
					return nil, errors.New("out of stock")
				}
				price = archiveSku.Price
				originPrice = archiveSku.Price
				if activity != nil {
					activitySku, err := w.GetActivitySkuBySkuId(activity.Id, v.GoodsSkuId)
					if err == nil {
						price = activitySku.Price
						// 减库存
						tx.Model(&model.ActivitySku{}).Where("id = ?", activitySku.Id).UpdateColumns(map[string]interface{}{
							"stock":      gorm.Expr("stock - ?", v.Quantity),
							"sold_count": gorm.Expr("sold_count + ?", v.Quantity),
						})
					}
				}
				// 减库存
				tx.Model(&model.ArchiveSku{}).Where("id = ?", archiveSku.Id).UpdateColumns(map[string]interface{}{
					"stock":      gorm.Expr("stock - ?", v.Quantity),
					"sold_count": gorm.Expr("sold_count + ?", v.Quantity),
				})
				skuValues = archiveSku.Values
			} else {
				// 如果数量超了,则不允许下单
				if archive.AllowOversold == false && int(archive.Stock) < v.Quantity {
					tx.Rollback()
					return nil, errors.New("out of stock")
				}
			}
			if len(v.SkuValues) > 0 {
				if len(skuValues) == 0 {
					skuValues = v.SkuValues
				} else {
					for _, sv := range v.SkuValues {
						exist := false
						for _, xv := range skuValues {
							if sv.Id == xv.Id || sv.Title == xv.Title {
								exist = true
								break
							}
						}
						if !exist {
							skuValues = append(skuValues, sv)
						}
					}
				}
			}
			// 计算运费，非包邮商品才计算运费
			weight, volume := 0.0, 0.0
			if archive.NeedLogistics && !archive.IsFreeShipping {
				// 计算重量和体积
				tmpVolume, tmpWeight := w.CalculateShippingAttrs(archive, v.Quantity)
				if archiveSku != nil {
					tmpVolume2, tmpWeight2 := w.CalculateSkuShippingAttrs(archiveSku, v.Quantity)
					if tmpVolume2 > 0 {
						tmpVolume = tmpVolume2
					}
					if tmpWeight2 > 0 {
						tmpWeight = tmpWeight2
					}
				}
				weight = tmpWeight
				volume = tmpVolume
				calcWeight += weight
				calcQuantity += v.Quantity
				calcVolume += volume
			}
			// 先将detail入库，得到id
			orderDetail := model.OrderDetail{
				OrderId:     order.OrderId,
				UserId:      userId,
				GoodsId:     v.GoodsId,
				GoodsSkuId:  v.GoodsSkuId,
				Price:       price,
				OriginPrice: originPrice,
				Quantity:    v.Quantity,
				Weight:      weight,
				Volume:      volume,
				Status:      1,
				OrderExtra:  v.OrderExtra,
				SkuValues:   skuValues,
			}
			err = tx.Save(&orderDetail).Error
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			// end
			// 处理 skuValues
			if archive.OptionType == config.OptionTypePrice && len(skuValues) > 0 {
				// 加价属性处理
				var optionPrice int64 = 0
				var originOptionPrice int64 = 0
				for _, sv := range skuValues {
					// 如果option是属性，不需要处理库存
					option, err := w.GetArchiveOption(archive.Id, sv.OptionId)
					if err != nil {
						// 不存在属性
						tx.Rollback()
						return nil, errors.New("option not found")
					}
					// 获取optionValue
					optionValue, err := w.GetArchiveOptionValue(archive.Id, sv.Id)
					if err != nil {
						// 不存在属性值
						tx.Rollback()
						return nil, errors.New("option value not found")
					}
					if !option.IsAttribute && archive.AllowOversold == false {
						// 需要处理库存
						if int(optionValue.Stock) < v.Quantity {
							tx.Rollback()
							return nil, errors.New("out of stock")
						}
					}
					if optionValue.Price > 0 {
						optionPrice += optionValue.Price
						originOptionPrice += optionValue.Price
					}
					// 减库存
					tx.Model(&model.ArchiveOptionValue{}).Where("id = ?", optionValue.Id).UpdateColumns(map[string]interface{}{
						"stock":      gorm.Expr("stock - ?", v.Quantity),
						"sold_count": gorm.Expr("sold_count + ?", v.Quantity),
					})
					// 3. 记录扣减记录（用于退款恢复）
					w.CreateOrderOptionStockRecords(order.OrderId, orderDetail.Id, optionValue.Id, v.Quantity)
				}
				// 更新价格
				price += optionPrice
				originPrice += originOptionPrice
			}
			// 减库存：stock 留在主表、sold_count 已迁到 archive_products，
			// 一条 UPDATE 跨不了两张表，所以拆成两步（同一事务）。
			tx.Model(&model.Archive{}).Where("id = ?", archive.Id).
				UpdateColumn("stock", gorm.Expr("stock - ?", v.Quantity))
			// 用 upsert：侧表没有行时（商品没配任何属性）也要能记销量，
			// 直接 Update 会命中 0 行，销量永远不动。
			tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "archive_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{"sold_count": gorm.Expr("sold_count + ?", v.Quantity)}),
			}).Create(&model.ArchiveProduct{
				ArchiveId:   archive.Id,
				SoldCount:   int64(v.Quantity),
				ProductType: config.ProductTypeNormal,
			})
			if activity != nil {
				tx.Model(&model.Activity{}).Where("id = ?", activity.Id).UpdateColumns(map[string]interface{}{
					"stock":      gorm.Expr("stock - ?", v.Quantity),
					"sold_count": gorm.Expr("sold_count + ?", v.Quantity),
				})
			}
			// 优惠
			// 批发价格设置
			if archive.ProductType == config.ProductTypeWholesale {
				wholesaleRules, _ := w.GetArchiveWholesaleRules(archive.Id)
				var percentage int64 = 100 // 默认没折扣
				for _, rule := range wholesaleRules {
					if v.Quantity >= rule.MinQuantity {
						percentage = int64(rule.DiscountRate)
					}
				}
				if percentage > 0 {
					if userDiscount == 0 {
						userDiscount = percentage
					} else {
						userDiscount = percentage * userDiscount / 100
					}
				}
			}
			if userDiscount > 0 {
				price = price * userDiscount / 100
			}
			detailAmount := price * int64(v.Quantity)
			originDetailAmount := originPrice * int64(v.Quantity)
			userDiscountAmount += originDetailAmount - detailAmount

			amount += detailAmount
			originAmount += originDetailAmount
			//更新子订单
			orderDetail.Amount = detailAmount
			orderDetail.OriginAmount = originDetailAmount
			err = tx.Save(&orderDetail).Error
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			details = append(details, map[string]interface{}{
				"id":       orderDetail.GoodsId,
				"title":    archive.Title,
				"sku_id":   orderDetail.GoodsSkuId,
				"link":     archive.Link,
				"logo":     archive.Logo,
				"price":    float32(price) / 100,
				"amount":   detailAmount,
				"quantity": orderDetail.Quantity,
				"weight":   weight,
				"volume":   volume,
			})
		}
	}
	order.Amount = amount
	order.OriginAmount = originAmount
	order.UserDiscount = userDiscountAmount
	if couponCode != nil {
		if couponCode.Status != 0 {
			tx.Rollback()
			return nil, err
		}
		couponMoney := couponCode.GetMoney(order.Amount)
		order.CouponCodeId = couponCode.Code
		order.CouponDiscount = couponMoney
		order.Amount = order.Amount - couponMoney
		// 使用coupon Code
		couponCode.OrderId = order.OrderId
		couponCode.UserId = int64(userId)
		_ = w.UseCouponCode(couponCode)
	}

	if calcQuantity > 0 {
		order.ShippingQuantity = calcQuantity
		order.ShippingWeight = calcWeight
		order.ShippingVolume = calcVolume
		// 计算运费
		var orderAddress *model.OrderAddress
		if req.Address != nil {
			orderAddress = &model.OrderAddress{
				CountryCode:  req.Address.CountryCode,
				Country:      req.Address.Country,
				ProvinceCode: req.Address.ProvinceCode,
				Province:     req.Address.Province,
				City:         req.Address.City,
				Postcode:     req.Address.Postcode,
			}
		} else if req.AddressId > 0 {
			orderAddress, _ = w.GetOrderAddressById(req.AddressId)
		}
		order.ShippingFee = w.CalculateShippingFee(orderAddress, order.Amount, calcQuantity, calcWeight, calcVolume)
		if shippingDiscount > 0 {
			originShippingFee := order.ShippingFee
			order.ShippingFee = int64(float64(originShippingFee) * (float64(shippingDiscount) / 100))
			order.ShippingDiscount = originShippingFee - order.ShippingFee
		}
		// 更新订单金额
		order.Amount += order.ShippingFee
	}
	order.DiscountAmount = order.UserDiscount + order.CouponDiscount + order.ShippingDiscount

	// orderService
	if len(req.ServiceIds) > 0 {
		services, _ := w.GetOrderServices(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("id in (?)", req.ServiceIds)
		}, 1, len(req.ServiceIds))
		var serviceFee int64
		for _, service := range services {
			if service.FeeType == config.OrderServiceTypePercentage {
				// 订单金额的百分比
				tmpFee := int64(float64(order.Amount) * (float64(service.Fee) / 100))
				if service.MaxFee > 0 && tmpFee > service.MaxFee {
					tmpFee = service.MaxFee
				}
				serviceFee += tmpFee
			} else {
				serviceFee += service.Fee
			}
		}
		order.ServiceFee = serviceFee
		order.Amount += serviceFee
	}

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
	// 清空购物车
	if userId > 0 {
		w.DB.Where("user_id = ?", userId).Delete(&model.Cart{})
		if err == nil {
			// 推送事件
			w.CreateEvent(EventOrderCreated, map[string]interface{}{
				"order_id": order.OrderId,
				"amount":   float32(order.Amount) / 100,
				"status":   "pending",
				"items":    details,
			}, user)
		}
	}
	_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypeOrderCreate, Id: int64(order.Id)})

	// 发送邮件
	w.SendNewOrderEmail(user, &order)
	// 发送 未支付订单召回

	return &order, nil
}

func (w *Website) GetOrderAddressesByUserId(userId uint) ([]*model.OrderAddress, error) {
	var orderAddress []*model.OrderAddress
	err := w.DB.Where("`user_id` = ?", userId).Order("id desc").Find(&orderAddress).Error
	if err != nil {
		return nil, err
	}

	return orderAddress, nil
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
	orderAddress.LastName = req.LastName
	orderAddress.Phone = req.Phone
	orderAddress.Email = req.Email
	orderAddress.Province = req.Province
	orderAddress.City = req.City
	orderAddress.Town = req.Town
	orderAddress.Country = req.Country
	orderAddress.AddressInfo = req.AddressInfo
	orderAddress.Company = req.Company
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

func (w *Website) RetailerApplyWithdraw(retailerId uint) (*model.UserWithdraw, error) {
	// 查询可提现金额
	var commissions []model.Commission
	var total int64
	w.DB.Model(&model.Commission{}).Where("`user_id` = ? and `status` = ?", retailerId, config.CommissionStatusWait).Find(&commissions)
	for i := range commissions {
		total += commissions[i].Amount
	}

	if total <= 0 {
		return nil, errors.New(w.Tr("NoAmountAvailableForWithdrawal"))
	}

	// todo执行提现操作
	// 低于2元不可提现到微信
	if total < 200 {
		return nil, errors.New(w.Tr("WithdrawalsBelow2YuanCannotBeMadeToWechatChange"))
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
		return nil, err
	}
	for _, val := range commissions {
		val.WithdrawId = withdraw.Id
		val.Status = config.CommissionStatusPaid
		err = tx.Where("id = ? and status = ?", val.Id, config.CommissionStatusWait).Updates(&val).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	tx.Commit()
	// 等待计划任务去处理
	return &withdraw, nil
}

func (w *Website) ExportOrders(req *request.OrderExportRequest) (header []string, content [][]interface{}) {
	var total int64
	tx := w.DB.Model(&model.Order{}).Order("id asc")

	if req.Status != "" {
		// status 可能会传 waiting,paid,delivery,finished,refunding,closed
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
		if req.Status == "closed" {
			tx = tx.Where("`status` = -1")
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
	if w.DB == nil || w.PluginOrder == nil {
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
	if payment.PaidTime > 0 {
		return nil
	}
	if payment.PayWay == config.PayWayAlipay {
		account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
		if err != nil {
			return err
		}
		client, err := alipay.NewClient(account.PayConfig.AppId, account.PayConfig.AppSecret, !account.PayConfig.Sandbox)
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
		certPath := w.DataPath + "cert/" + account.PayConfig.CertPath
		rootCertPath := w.DataPath + "cert/" + account.PayConfig.RootCertPath
		publicCertPath := w.DataPath + "cert/" + account.PayConfig.PublicCertPath
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
			w.DB.Model(order).UpdateColumn("payment_id", payment.PaymentId)
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
			_ = w.SuccessPaidOrder(order, payment)
		}
	} else if payment.PayWay == config.PayWayWechat {
		// 微信就不管了
	} else if payment.PayWay == config.PayWayWeapp {
		// 微信就不管了
	} else if payment.PayWay == config.PayWayPaypal {
		account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
		if err != nil {
			return err
		}
		client, err := paypal.NewClient(account.PayConfig.AppId, account.PayConfig.AppSecret, !account.PayConfig.Sandbox)
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
			status := ppRsp.Response.Status
			if ppRsp.Response.Status == "APPROVED" {
				// 需要 capture
				captureRes, err := client.OrderCapture(context.Background(), payment.TerraceId, nil)
				if err != nil {
					log.Println("capture err", err)
					return err
				}
				library.DebugLog(w.CachePath, "paypalCapture", captureRes.Response)
				if captureRes.Code == 0 {
					status = captureRes.Response.Status
				} else {
					log.Println("captureRes err", captureRes.Error)
				}
			}
			if status == "COMPLETED" && payment.PaidTime == 0 {
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
				w.DB.Model(order).UpdateColumn("payment_id", payment.PaymentId)
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
				_ = w.SuccessPaidOrder(order, payment)
			}
		}
	} else if payment.PayWay == config.PayWayStripe {
		// 从Stripe获取最新状态
		account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
		if err != nil {
			return err
		}
		client := stripe.NewClient(account.PayConfig.AppId)

		pi, err := client.V1PaymentIntents.Retrieve(context.Background(), payment.TerraceId, nil)
		if err != nil {
			// 处理创建失败
			log.Println("stripe payment intent order not found", err.Error())
			return err
		}
		buf, _ := json.Marshal(pi)
		library.DebugLog(w.CachePath, "stripe", string(buf))
		if pi.Status == stripe.PaymentIntentStatusSucceeded {
			// 已完成
			// 更新payment
			payment.PayWay = config.PayWayStripe
			payment.PaidTime = time.Now().Unix()
			payment.BuyerId = pi.PaymentMethod.ID
			payment.BuyerInfo = "" // todo
			w.DB.Model(payment).UpdateColumns(map[string]interface{}{
				"paid_time":  payment.PaidTime,
				"buyer_id":   payment.BuyerId,
				"buyer_info": payment.BuyerInfo,
				"pay_way":    payment.PayWay,
			})
			order, err2 := w.GetOrderInfoByOrderId(payment.OrderId)
			if err2 != nil {
				return err2
			}
			order.PaymentId = payment.PaymentId
			w.DB.Model(order).UpdateColumn("payment_id", payment.PaymentId)
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
			_ = w.SuccessPaidOrder(order, payment)
		}
	}

	return nil
}

// 每小时执行一次
func (w *Website) CheckCartAndOrders() {
	if w.DB == nil || w.PluginOrder == nil {
		return
	}
	if w.PluginSendmail.UseBrevo == true || w.PluginSendmail.Account == "" {
		return
	}

	// 购物车
	template, exist := w.GetEmailTemplateInfo("recover_abandoned_cart")
	if exist && template.Open {
		currentStamp := time.Now().Unix()
		// 1小时间隔
		closeStamp := currentStamp - template.Delay
		// check the order that was paid or not
		var userIds []int64
		w.DB.Model(&model.Cart{}).Where("created_time >= ? and created_time < ?", config.OrderStatusWaiting, closeStamp-3600, closeStamp).Group("user_id").Pluck("user_id", &userIds)
		for _, userId := range userIds {
			if userId > 0 {
				user, _ := w.GetUserInfoById(uint(userId))
				if user.Email != "" {
					continue
				}
				items, _ := w.GetCartList(userId, "", 1, 100)
				var amount int64
				for _, item := range items {
					if item.Archive == nil {
						continue
					}
					amount += (item.Archive.Price) * int64(item.Quantity)
				}

				data := map[string]interface{}{
					"website": w,
					"user":    user,
					"cart": map[string]interface{}{
						"Items":  items,
						"Amount": amount,
					},
					"url": w.System.BaseUrl,
				}
				var err error
				template2, err := w.RenderEmailTemplate(template, data)
				if err != nil {
					continue
				}

				// 发给用户
				w.SendMail(template2.Subject, template2.Content, nil, user.Email)
			}
		}
	}
	// end 购物车

	// 订单未支付提醒
	// 已实现，不在这里处理
	// end 订单

	// 浏览未购提醒
	// todo
	// end 浏览未购提醒
}

func (w *Website) RecoverPurchase() {
	// 首次购物后回购营销
	if w.DB == nil || w.PluginOrder == nil {
		return
	}
	if w.PluginSendmail.UseBrevo == true || w.PluginSendmail.Account == "" {
		return
	}
	template, exist := w.GetEmailTemplateInfo("repurchase_promotion")
	if exist && template.Open {
		currentStamp := now.BeginningOfDay().Unix()
		checkStamp := currentStamp - template.Delay
		var userIds []int64
		w.DB.Model(&model.Order{}).Where("created_time >= ? and created_time < ?", checkStamp, checkStamp+86400).Group("user_id").Pluck("user_id", &userIds)
		for _, userId := range userIds {
			user, err := w.GetUserInfoById(uint(userId))
			if err != nil {
				continue
			}
			if user.Email != "" {
				continue
			}
			data := map[string]interface{}{
				"website": w,
				"user":    user,
				"url":     w.System.BaseUrl,
			}
			template2, err := w.RenderEmailTemplate(template, data)
			if err != nil {
				continue
			}

			// 发给用户
			w.SendMail(template2.Subject, template2.Content, nil, user.Email)
		}
	}

	// 流失用户召回
	template, exist = w.GetEmailTemplateInfo("recover_old_user")
	if exist && template.Open {
		currentStamp := now.BeginningOfDay().Unix()
		checkStamp := currentStamp - template.Delay
		var userIds []int64
		w.DB.Model(&model.User{}).Where("last_login >= ? and last_login < ?", checkStamp, checkStamp+86400).Pluck("id", &userIds)
		for _, userId := range userIds {
			user, err := w.GetUserInfoById(uint(userId))
			if err != nil {
				continue
			}
			if user.Email != "" {
				continue
			}
			data := map[string]interface{}{
				"website": w,
				"user":    user,
				"url":     w.System.BaseUrl,
			}
			template2, err := w.RenderEmailTemplate(template, data)
			if err != nil {
				continue
			}

			// 发给用户
			w.SendMail(template2.Subject, template2.Content, nil, user.Email)
		}
	}
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

// CreateOrderOptionStockRecords 记录属性值库存扣减记录（用于退款恢复）
func (w *Website) CreateOrderOptionStockRecords(orderId string, detailId uint, optionValueId int64, quantity int) error {
	if orderId == "" || optionValueId == 0 || quantity <= 0 {
		return nil
	}

	record := model.OrderOptionStock{
		OrderId:       orderId,
		OrderDetailId: detailId,
		OptionValueId: optionValueId,
		Quantity:      quantity,
	}

	return w.DB.Create(&record).Error
}
