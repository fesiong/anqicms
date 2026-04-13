package provider

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/paypal"
	"github.com/go-pay/xlog"
	"github.com/jinzhu/now"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

type PaypalWebhookResource struct {
	Id         string `json:"id"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
	State      string `json:"state"`
	Amount     struct {
		Total    string `json:"total"`
		Currency string `json:"currency"`
		Details  struct {
			Subtotal string `json:"subtotal"`
		} `json:"details"`
	} `json:"amount"`
	ParentPayment string `json:"parent_payment"`
	ValidUntil    string `json:"valid_until"`
}

func (w *Website) GetPaymentWays() []string {
	var payWays []string
	w.DB.Model(&model.PaymentAccount{}).Where("`status` = 1").Group("pay_way").Pluck("pay_way", &payWays)

	return payWays
}

func (w *Website) GetPaymentAccounts(ops func(tx *gorm.DB) *gorm.DB) []*model.PaymentAccount {
	var accounts []*model.PaymentAccount
	tx := w.DB.Model(model.PaymentAccount{}).Order("id desc")
	if ops != nil {
		tx = ops(tx)
	}
	tx.Find(&accounts)

	return accounts
}

func (w *Website) GetPaymentAccountStatistic(accountId int64, currentPage int, pageSize int) ([]*model.PaymentStatistic, int64) {
	var result []*model.PaymentStatistic
	var total int64
	offset := 0
	if currentPage > 1 {
		offset = (currentPage - 1) * pageSize
	}
	tx := w.DB.Model(model.PaymentStatistic{}).Order("id desc")
	if accountId > 0 {
		tx = tx.Where("account_id = ?", accountId)
	}
	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&result)

	return result, total
}

// GetSingleValidPaymentAccount 获取单个可用的支付账号，根据组合规则选出可用的支付账号
func (w *Website) GetSingleValidPaymentAccount(payWay string, amount int64) *model.PaymentAccount {
	// 先获取指定类型可用的账户
	accounts := w.GetPaymentAccounts(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("pay_way = ? and status = 1", payWay)
	})
	if len(accounts) == 0 {
		return nil
	}
	// 如果只有一个，直接返回
	if len(accounts) == 1 {
		return accounts[0]
	}
	todayStamp := now.BeginningOfDay().Unix()
	monthStamp := now.BeginningOfMonth().Unix()
	// 如果有多个，则需要判断是否达到限额了
	var validAccounts []*model.PaymentAccount
	for _, account := range accounts {
		// 跳过金额范围不符合的账号
		if account.MinAmount > 0 && amount < account.MinAmount {
			continue
		}
		if account.MaxAmount > 0 && amount > account.MaxAmount {
			continue
		}
		// 不限额的有效
		if account.DailyAmountLimit == 0 && account.MonthlyAmountLimit == 0 && account.DailyCountLimit == 0 {
			validAccounts = append(validAccounts, account)
			continue
		}
		var statistic model.PaymentStatistic
		err := w.DB.Where("account_id = ?", account.Id).Last(&statistic).Error
		if err != nil {
			continue
		}
		// 当月没有成交，有效
		if statistic.StatTime < monthStamp {
			validAccounts = append(validAccounts, account)
			continue
		}
		// 先判断月额度是否达标，跳过当月已达标的账号
		if statistic.MonthlyAmount >= account.MonthlyAmountLimit {
			continue
		}
		// 当天没有成交，有效
		if statistic.StatTime < todayStamp {
			validAccounts = append(validAccounts, account)
			continue
		}
		// 跳过当日已达标的账号
		if statistic.DailyCount >= account.DailyCountLimit || statistic.DailyAmount >= account.DailyAmountLimit {
			continue
		}
		// 满足条件
		validAccounts = append(validAccounts, account)
	}
	if len(validAccounts) == 1 {
		return validAccounts[0]
	}
	// 如果没有有效账号，则返回 isDefault
	if len(validAccounts) == 0 {
		for _, account := range accounts {
			if account.IsDefault {
				return account
			}
		}
		// 否则，返回第一个
		return validAccounts[0]
	}
	// 随机返回其中一个
	rd := rand.New(rand.NewSource(time.Now().UnixNano()))

	return validAccounts[rd.Intn(len(validAccounts))]
}

func (w *Website) GetPaymentAccountById(id int64) (*model.PaymentAccount, error) {
	var account model.PaymentAccount
	err := w.DB.Where("id = ?", id).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (w *Website) DeletePaymentAccount(id int64) error {
	var account model.PaymentAccount
	err := w.DB.Where("id = ?", id).First(&account).Error
	if err != nil {
		return err
	}
	err = w.DB.Delete(&account).Error
	if err != nil {
		return err
	}

	return nil
}

func (w *Website) SavePaymentAccount(req *model.PaymentAccount) error {
	var account model.PaymentAccount
	if req.Id > 0 {
		err := w.DB.Where("id = ?", req.Id).First(&account).Error
		if err != nil {
			return err
		}
	}
	account.PayWay = req.PayWay
	account.AccountName = req.AccountName
	account.Status = req.Status
	account.HealthScore = req.HealthScore
	account.PayConfig = req.PayConfig
	account.MinAmount = req.MinAmount
	account.MaxAmount = req.MaxAmount
	account.DailyCountLimit = req.DailyCountLimit
	account.DailyAmountLimit = req.DailyAmountLimit
	account.MonthlyAmountLimit = req.MonthlyAmountLimit
	account.Weight = req.Weight
	account.IsDefault = req.IsDefault

	err := w.DB.Save(&account).Error
	if err != nil {
		return err
	}
	// 处理 webhook
	if req.PayWay == config.PayWayPaypal {
		// 处理 paypal webhook
		if req.Status == 1 && req.PayConfig.AppId != "" && req.PayConfig.AppSecret != "" {
			w.UpdatePaypalWebhook(&account)
		}
	}

	return nil
}

func (w *Website) ProcessPaypalEvent(event *paypal.WebhookEvent) {
	// 添加超时控制
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 使用事件ID进行幂等性检查
	if w.isDuplicateEvent(ctx, event.Id) {
		xlog.Warnf("Duplicate event detected: %s", event.Id)
		return
	}
	// 获取payment信息
	var resp PaypalWebhookResource
	err := json.Unmarshal(event.Resource, &resp)
	if err != nil {
		xlog.Errorf("Failed to unmarshal resource: %v, %v", err, string(event.Resource))
		return
	}
	payment, err := w.GetPaymentInfoByTerraceId(resp.Id)
	if err != nil {
		xlog.Errorf("Failed to get payment info: %v", err)
		return
	}

	account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		xlog.Errorf("Failed to get account info: %v", err)
		return
	}
	var client *paypal.Client
	client, err = paypal.NewClient(account.PayConfig.AppId, account.PayConfig.AppSecret, account.PayConfig.Sandbox == false)
	if err != nil {
		// 处理token获取失败，换另一个账号
		return
	}
	library.DebugLog(w.CachePath, "paypal_webhook", string(event.Resource))
	// 根据事件类型路由处理
	switch event.EventType {
	case "CHECKOUT.ORDER.APPROVED":
		// 执行capture
		captureRes, err := client.OrderCapture(context.Background(), payment.TerraceId, nil)
		if err != nil {
			log.Println("capture err", err)
			return
		}
		library.DebugLog(w.CachePath, "paypalCapture", captureRes.Response)
		if captureRes.Code == 0 {
			// todo
		} else {
			log.Println("captureRes err", captureRes.Error)
		}
	case "PAYMENT.CAPTURE.COMPLETED":
		// 付款捕获完成（资金已到账）
		payment.PayWay = config.PayWayPaypal
		err = w.TraceQuery(payment)
	case "PAYMENT.CAPTURE.DENIED":
		order, err := w.GetOrderInfoByOrderId(payment.OrderId)
		if err == nil {
			_ = w.SetOrderCanceled(order)
		}
	case "PAYMENT.CAPTURE.REFUNDED":
		//order, err := w.GetOrderInfoByOrderId(payment.OrderId)
		//if err == nil {
		//	err = w.ApplyOrderRefund(order)
		//	err = w.SetOrderRefund(order, 1)
		//}
	case "CHECKOUT.ORDER.COMPLETED":
		payment.PayWay = config.PayWayPaypal
		err = w.TraceQuery(payment)
	default:
		xlog.Warnf("Unhandled event type: %s", event.EventType)
	}
}

func (w *Website) isDuplicateEvent(ctx context.Context, eventId string) bool {
	var eventIdExists bool
	err := w.Cache.Get(eventId, &eventIdExists)
	if err != nil {
		return false
	}
	// 保留30秒的缓存
	_ = w.Cache.Set(eventId, true, 30)

	return false
}

func (w *Website) UpdatePaypalWebhook(account *model.PaymentAccount) {
	if account.PayConfig.AppId == "" || account.PayConfig.AppSecret == "" {
		return
	}
	if strings.Contains(w.System.BaseUrl, "127.0.0.1") {
		return
	}
	// 不支持http
	if strings.HasPrefix(w.System.BaseUrl, "http://") {
		return
	}
	client, err := paypal.NewClient(account.PayConfig.AppId, account.PayConfig.AppSecret, account.PayConfig.Sandbox == false)
	if err != nil {
		// 处理token获取失败
		return
	}
	resp, err := client.ListWebhook(context.Background())
	if err != nil {
		// 处理获取失败
		log.Println("paypal webhook list error", err.Error())
		return
	}
	if resp.Code != paypal.Success {
		// 不处理
		log.Println("paypal webhook list error", resp.Error)
		return
	}

	var eventTypes = []*paypal.WebhookEventType{
		{Name: "PAYMENT.CAPTURE.COMPLETED"},
		{Name: "PAYMENT.CAPTURE.DENIED"},
		{Name: "PAYMENT.CAPTURE.REFUNDED"},
		{Name: "CHECKOUT.ORDER.APPROVED"},
		{Name: "CHECKOUT.ORDER.COMPLETED"},
	}
	// anqicms 的 webhook url 是 /notify/paypal/pay
	var webhookId = account.PayConfig.WebhookId
	var webhookUrl = w.System.BaseUrl + "/notify/paypal/pay"
	var findUrl string
	if len(resp.Response.Webhooks) > 0 {
		var exist bool
		for _, webhook := range resp.Response.Webhooks {
			if webhook.Url == webhookUrl || webhook.Id == webhookId {
				webhookId = webhook.Id
				findUrl = webhook.Url
				exist = true
				break
			}
		}
		if !exist {
			webhookId = ""
		}
	}
	if webhookId == "" || findUrl != webhookUrl {
		if webhookId == "" {
			// 创建 webhook
			bm := make(gopay.BodyMap)
			bm.Set("url", webhookUrl).
				Set("event_types", eventTypes)
			createRsp, err := client.CreateWebhook(context.Background(), bm)
			if err != nil {
				// 处理创建失败
				log.Println("paypal webhook create error", err.Error())
				return
			}
			if createRsp.Code != paypal.Success {
				log.Println("paypal webhook create error", createRsp.ErrorResponse.Message)
				return
			}
			account.PayConfig.WebhookId = createRsp.Response.Id
			log.Println("paypal webhook create success", createRsp.Response.Id)
		} else {
			// 更新
			var ps = []*paypal.Patch{
				{
					Op:    "replace",
					Path:  "/url",
					Value: webhookUrl,
				},
				{
					Op:    "replace",
					Path:  "/event_types",
					Value: eventTypes,
				},
			}
			ppRsp, err := client.UpdateWebhook(context.Background(), webhookId, ps)
			if err != nil {
				xlog.Error(err)
				return
			}
			if ppRsp.Code != paypal.Success {
				xlog.Debugf("paypal webhook update error: %+v", ppRsp.Error)
				return
			}
			account.PayConfig.WebhookId = webhookId
			log.Println("paypal webhook update success", ppRsp.Response.Id)
		}
		// 保存
		w.DB.Model(account).Where("id = ?", account.Id).Save(account)
	}
}
