package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
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
	"kandaoni.com/anqicms/request"
)

// payFlexString 兼容支付网关返回的字符串/数字。
// 同一字段在不同接口里类型不一致（如 total_fee 可能是 "19.00" 也可能是 19.0），
// 声明成 string 或 json.Number 都会让整个响应解析失败。
type payFlexString string

func (f *payFlexString) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = payFlexString(s)
		return nil
	}
	*f = payFlexString(string(b))
	return nil
}

// paymentOrderTitle 把订单备注裁剪成网关可接受的标题。
// 各网关对标题的限制不一样（虎皮椒「不超 42 个汉字且不能带 %」、NOWPayments 限长），
// 统一在这里去掉 % 与空白并按字符数截断，避免下单被网关直接拒掉。
func paymentOrderTitle(remark string, maxRunes int) string {
	title := strings.ReplaceAll(strings.TrimSpace(remark), "%", "")
	if title == "" {
		title = "Order"
	}
	runes := []rune(title)
	if maxRunes > 0 && len(runes) > maxRunes {
		title = string(runes[:maxRunes])
	}
	return title
}

// paymentPostJSON 向支付网关发送 JSON 请求，返回响应体与状态码。
// header 用于 NOWPayments 这类把密钥放在请求头的网关。
func paymentPostJSON(apiUrl string, body []byte, header map[string]string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

// paymentGetJSON 同 paymentPostJSON，用于查询类接口。
func paymentGetJSON(apiUrl string, header map[string]string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiUrl, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range header {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

type PaypalWebhookResource struct {
	Id         string `json:"id"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
	State      string `json:"state"`
	Status     string `json:"status"`
	Amount     struct {
		Total    string `json:"total"`
		Currency string `json:"currency"`
		Details  struct {
			Subtotal string `json:"subtotal"`
		} `json:"details"`
	} `json:"amount"`
	ParentPayment string `json:"parent_payment"`
	ValidUntil    string `json:"valid_until"`
	// 捕获类事件（PAYMENT.CAPTURE.*）的 id 是 capture id，不是下单时的 order id，
	// 关联的订单号放在 supplementary_data.related_ids.order_id
	SupplementaryData struct {
		RelatedIds struct {
			OrderId string `json:"order_id"`
		} `json:"related_ids"`
	} `json:"supplementary_data"`
	// PAYMENT.CAPTURE.REFUNDED 的 resource 是 refund 对象，真实退款金额在
	// amount.total；capture 形态则在 seller_receivable_breakdown.total_refunded_amount
	// （部分退款时与 capture 金额不同）
	SellerReceivableBreakdown struct {
		TotalRefundedAmount struct {
			Value        string `json:"value"`
			CurrencyCode string `json:"currency_code"`
		} `json:"total_refunded_amount"`
	} `json:"seller_receivable_breakdown"`
	// 退款对象的 links 里 rel=up 指向所属 capture（…/v2/payments/captures/{capture_id}），
	// 这是退款通知定位支付单的**唯一**线索：refund id 既不是 order id 也不是 capture id。
	Links []PaypalWebhookLink `json:"links"`
}

type PaypalWebhookLink struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method"`
}

// UpLinkCaptureId 从 links[rel=up] 里解析出 capture id（退款事件用）。
func (res *PaypalWebhookResource) UpLinkCaptureId() string {
	if res == nil {
		return ""
	}
	const marker = "/captures/"
	for _, l := range res.Links {
		if l.Rel != "up" || l.Href == "" {
			continue
		}
		idx := strings.LastIndex(l.Href, marker)
		if idx < 0 {
			continue
		}
		id := l.Href[idx+len(marker):]
		if q := strings.IndexAny(id, "?#"); q >= 0 {
			id = id[:q]
		}
		if id = strings.Trim(id, "/ "); id != "" {
			return id
		}
	}
	return ""
}

// MarkPaymentPaid 把支付单标记为已支付，并推进订单状态。
//
// 这段逻辑每个渠道、两条路径（异步通知 / 主动查单）都要走一遍，字段或顺序错一个就会出现
// 「payment 有 paid_time、order 还停在待支付」或重复生成财务流水。集中在这里，别各写一份。
//
// paid_time 用 `paid_time = 0` 的条件更新做闸门：微信/虎皮椒都会重试通知，模板还会开着
// 轮询打 /api/payment/check（内部就是 TraceQuery），两条路径并发进来时只能有一条真正生效。
func (w *Website) MarkPaymentPaid(payment *model.Payment, terraceId, buyerId, buyerInfo string) error {
	if payment == nil {
		return errors.New("payment is nil")
	}
	if payment.PaidTime > 0 {
		return nil
	}
	order, err := w.GetOrderInfoByOrderId(payment.OrderId)
	if err != nil {
		return err
	}
	if order.Status == config.OrderStatusRefunded || order.RefundStatus == config.OrderStatusRefunded {
		// 迟到的付款通知：钱已经退给用户了，再把订单改回「已付款」会在财务上凭空多一笔收入，
		// 而且退款流水和付款流水会对不上。这里直接吞掉（返回 nil 让 PayPal 别再重投）。
		//
		// 这条必须排在「订单已支付 → 需要人工退款」之前：已退款订单的 paid_time 本来就有值，
		// 排在后面会先撞上那条 error，把一次正常的重复通知变成 500 让 PayPal 无限重投。
		logPaymentEvent(w.CachePath, payment.PaymentId, "payment", "订单已退款，忽略迟到的付款通知")
		return nil
	}
	if order.PaidTime > 0 {
		// 订单已被另一张支付单付掉，这张单得人工退款，不能当成支付成功
		return fmt.Errorf("order %s already paid, payment %s needs manual refund", order.OrderId, payment.PaymentId)
	}

	paidTime := time.Now().Unix()
	updates := map[string]interface{}{"paid_time": paidTime}
	if terraceId != "" {
		updates["terrace_id"] = terraceId
		payment.TerraceId = terraceId
	}
	if buyerId != "" {
		updates["buyer_id"] = buyerId
		payment.BuyerId = buyerId
	}
	if buyerInfo != "" {
		updates["buyer_info"] = buyerInfo
		payment.BuyerInfo = buyerInfo
	}
	res := w.DB.Model(&model.Payment{}).Where("id = ? and paid_time = 0", payment.Id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// 已被并发的另一条通知/查单处理过，直接返回成功，避免重复记账
		return nil
	}
	payment.PaidTime = paidTime

	order.PaymentId = payment.PaymentId
	w.DB.Model(order).UpdateColumn("payment_id", payment.PaymentId)

	//生成用户支付记录
	var userBalance int64
	err = w.DB.Model(&model.User{}).Where("`id` = ?", payment.UserId).Pluck("balance", &userBalance).Error
	finance := model.Finance{
		UserId:      payment.UserId,
		Direction:   config.FinanceOutput,
		Amount:      payment.Amount,
		AfterAmount: userBalance,
		Action:      config.FinanceActionBuy,
		OrderId:     payment.OrderId,
		Status:      1,
	}
	if err = w.DB.Create(&finance).Error; err != nil {
		return err
	}

	//支付成功逻辑处理
	return w.SuccessPaidOrder(order, payment)
}

// GetPaymentInfoByPaypalResource 按 webhook 资源定位支付单。
// CHECKOUT.ORDER.* 的 resource.id 就是 order id（= payment.terrace_id）；
// PAYMENT.CAPTURE.* 的 resource.id 是 capture id，得用 related_ids.order_id 换算，
// 再退一步用 payments.capture_id 兜底（某些捕获事件的 resource 不带 related_ids）；
// PAYMENT.CAPTURE.REFUNDED 的 resource 是 **refund**：id 是退款单 id，两者都不等，
// 只能顺着 links[rel=up] 找到 capture，或按 order_refunds.terrace_id 反查订单。
func (w *Website) GetPaymentInfoByPaypalResource(res *PaypalWebhookResource) (*model.Payment, error) {
	if res == nil {
		return nil, errors.New("paypal webhook resource is nil")
	}
	payment, err := w.GetPaymentInfoByTerraceId(res.Id)
	if err == nil {
		return payment, nil
	}
	if orderId := res.SupplementaryData.RelatedIds.OrderId; orderId != "" {
		if p, err2 := w.GetPaymentInfoByTerraceId(orderId); err2 == nil {
			return p, nil
		}
	}
	if p, err2 := w.GetPaymentInfoByCaptureId(res.Id); err2 == nil {
		return p, nil
	}
	// 退款通知：resource 是 refund，所属 capture 在 links[rel=up] 里
	if captureId := res.UpLinkCaptureId(); captureId != "" {
		if p, err2 := w.GetPaymentInfoByCaptureId(captureId); err2 == nil {
			return p, nil
		}
	}
	// 最后按退款单反查：后台退款时已把 PayPal Refund ID 写进 order_refunds.terrace_id
	if p, err2 := w.GetPaymentInfoByRefundTerraceId(res.Id); err2 == nil {
		return p, nil
	}
	return nil, err
}

// GetPaymentInfoByRefundTerraceId 按 PayPal 退款单号定位支付单。
func (w *Website) GetPaymentInfoByRefundTerraceId(refundId string) (*model.Payment, error) {
	if refundId == "" {
		return nil, errors.New("paypal refund id is empty")
	}
	var refund model.OrderRefund
	if err := w.DB.Where("`terrace_id` = ?", refundId).Take(&refund).Error; err != nil {
		return nil, err
	}
	return w.GetPaymentInfoByOrderId(refund.OrderId)
}

// GetPaymentInfoByCaptureId 按 PayPal capture id 找支付单（capture id 在付款/退款通知里出现）。
func (w *Website) GetPaymentInfoByCaptureId(captureId string) (*model.Payment, error) {
	if captureId == "" {
		return nil, errors.New("capture id is empty")
	}
	var payment model.Payment
	if err := w.DB.Where("`capture_id` = ?", captureId).Take(&payment).Error; err != nil {
		return nil, err
	}
	return &payment, nil
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

func (w *Website) SavePaymentAccount(req *request.PaymentAccountRequest) (*model.PaymentAccount, error) {
	var account model.PaymentAccount
	if req.Id > 0 {
		err := w.DB.Where("id = ?", req.Id).First(&account).Error
		if err != nil {
			return nil, err
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
		return nil, err
	}
	// 处理 webhook
	if req.PayWay == config.PayWayPaypal {
		// 处理 paypal webhook
		if req.Status == 1 && req.PayConfig.AppId != "" && req.PayConfig.AppSecret != "" {
			w.UpdatePaypalWebhook(&account)
		}
	}

	return &account, nil
}

// ProcessPaypalEvent 处理一条已验签通过的 PayPal 回调。
//
// 返回 error = 这条通知没处理成功（调用方应回非 2xx 让 PayPal 重投）；
// 返回 nil = 已处理，或确定无需处理（未知事件 / 重复事件）。
//
// 幂等靠「状态迁移」而不是内存标记：MarkPaymentPaid 用 paid_time = 0 做闸门，
// SuccessRefundOrder 用 order.status <> 已退款 做闸门。所以重投、后台操作与通知并发，
// 都只会产生一笔付款流水 / 一笔退款流水。
func (w *Website) ProcessPaypalEvent(event *paypal.WebhookEvent) error {
	if event == nil {
		return errors.New("paypal webhook event is nil")
	}
	if w.isDuplicateEvent(event.Id) {
		logPaymentEvent(w.CachePath, event.Id, event.EventType, "duplicate event, skipped")
		return nil
	}
	if err := w.handlePaypalEvent(event); err != nil {
		// 处理失败不写去重标记：否则 PayPal 重投回来的这条会被当成重复事件直接吞掉，
		// 一次临时故障就变成永久的丢单。
		return err
	}
	w.markEventProcessed(event.Id)
	return nil
}

func (w *Website) handlePaypalEvent(event *paypal.WebhookEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var resp PaypalWebhookResource
	if err := json.Unmarshal(event.Resource, &resp); err != nil {
		return fmt.Errorf("解析 paypal resource 失败: %w", err)
	}
	payment, err := w.GetPaymentInfoByPaypalResource(&resp)
	if err != nil {
		return fmt.Errorf("未找到支付单（resource id=%s）: %w", resp.Id, err)
	}
	account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		return fmt.Errorf("未找到支付账号: %w", err)
	}
	client, err := newPaypalClient(account)
	if err != nil {
		return fmt.Errorf("初始化 paypal client 失败: %w", err)
	}
	library.DebugLog(w.CachePath, "paypal_webhook", string(event.Resource))

	switch event.EventType {
	case "CHECKOUT.ORDER.APPROVED":
		// 用户已授权，需要立刻扣款（否则 PayPal 那边只是 APPROVED，钱没到账）。
		// ⚠️ PayPal 会重投本事件：钱已扣过时再 capture 会返回 ORDER_ALREADY_CAPTURED，
		// 老实现把它当失败回 500，PayPal 于是继续重投，形成死循环。先看订单状态。
		if detail, dErr := client.OrderDetail(ctx, payment.TerraceId, make(gopay.BodyMap)); dErr == nil &&
			detail.Code == paypal.Success && detail.Response != nil &&
			detail.Response.Status == "COMPLETED" {
			w.savePaypalCaptureId(payment, firstPaypalCaptureId(detail.Response))
			return w.finishPaypalApproved(payment)
		}
		captureRes, cErr := client.OrderCapture(ctx, payment.TerraceId, nil)
		if cErr != nil {
			return fmt.Errorf("capture 订单失败: %w", cErr)
		}
		if captureRes.Code != paypal.Success {
			// 并发的另一条通知/查单刚扣完：按已扣款收尾，不要回 500
			if paypalIssueIs(captureRes.ErrorResponse, "ORDER_ALREADY_CAPTURED") {
				return w.finishPaypalApproved(payment)
			}
			return fmt.Errorf("capture 订单失败: %s", captureRes.Error)
		}
		w.savePaypalCaptureId(payment, firstPaypalCaptureId(captureRes.Response))
		return nil
	case "PAYMENT.CAPTURE.COMPLETED":
		// 先记住 capture id —— 退款接口只认它
		w.savePaypalCaptureId(payment, resp.Id)
		payment.PayWay = config.PayWayPaypal
		return w.TraceQuery(payment)
	case "PAYMENT.CAPTURE.DENIED":
		order, oErr := w.GetOrderInfoByOrderId(payment.OrderId)
		if oErr != nil {
			return oErr
		}
		if order.PaidTime > 0 || order.Status == config.OrderStatusRefunded {
			// 已付款/已退款的订单不能因为一条 DENIED 通知被取消掉
			logPaymentEvent(w.CachePath, event.Id, event.EventType,
				fmt.Sprintf("order %s paid/refunded, ignore DENIED", order.OrderId))
			return nil
		}
		return w.SetOrderCanceled(order)
	case "PAYMENT.CAPTURE.REFUNDED":
		return w.handlePaypalCaptureRefunded(payment, &resp)
	case "CHECKOUT.ORDER.COMPLETED":
		payment.PayWay = config.PayWayPaypal
		return w.TraceQuery(payment)
	default:
		logPaymentEvent(w.CachePath, event.Id, event.EventType, "unhandled event type")
		return nil
	}
}

// isDuplicateEvent 事件是否已经处理过（只读）。
//
// 原来的实现「查到了也返回 false、没查到也只 Set 不返回」= 永远不去重。
// 现在读和写分开：处理成功后才 markEventProcessed，处理失败不写，
// 这样 PayPal 重投才有意义。缓存不是原子的（读与写之间有窗口），
// 最终一致性由 MarkPaymentPaid / SuccessRefundOrder 的状态闸门兜住。
func (w *Website) isDuplicateEvent(eventId string) bool {
	if eventId == "" || w.Cache == nil {
		return false
	}
	var seen bool
	if err := w.Cache.Get(w.paypalEventKey(eventId), &seen); err == nil && seen {
		return true
	}
	return false
}

func (w *Website) markEventProcessed(eventId string) {
	if eventId == "" || w.Cache == nil {
		return
	}
	// PayPal 的通知重投最长持续 3 天，缓存留 7 天
	_ = w.Cache.Set(w.paypalEventKey(eventId), true, 7*24*3600)
}

func (w *Website) paypalEventKey(eventId string) string {
	return "paypal_event_" + eventId
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
