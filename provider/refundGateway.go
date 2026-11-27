package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/wechat"
	"github.com/stripe/stripe-go/v82"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

// 本文件处理「所有支付渠道共有的退款状态收敛」问题。
//
// PayPal 那次事故的本质是三条，跟渠道无关：
//  1. 网关受理退款 ≠ 退款完成（PayPal PENDING、微信 PROCESSING、Stripe pending）；
//  2. 网关用**异步通知**告知最终结果，而通知分支可能根本到达不了（微信/支付宝的通知
//     先被「payment.PaidTime > 0 → 直接 return success」挡在前面）；
//  3. 通知和后台退款并发时，同一笔退款会被记两次账。
//
// 所以这里把「网关退款状态 → 本地三态」抽出来，退款和通知都只调用 ApplyRefundGatewayState，
// 绝不再在各渠道分支里自己写「退款完成」。

// refundSettleState 是网关退款的收敛态。
type refundSettleState int

const (
	// refundSettlePending 网关已受理但还没出结果：本地必须保持「待处理」，
	// 绝不能标成已退款——那是账上凭空多出来的一笔退款。
	refundSettlePending refundSettleState = iota
	// refundSettleDone 网关确认退款完成。
	refundSettleDone
	// refundSettleFailed 网关明确拒绝/关闭了这次退款。
	refundSettleFailed
)

func (s refundSettleState) String() string {
	switch s {
	case refundSettleDone:
		return "done"
	case refundSettleFailed:
		return "failed"
	default:
		return "pending"
	}
}

// wechatRefundState 微信 V2 退款状态：SUCCESS / PROCESSING / CHANGE / REFUNDCLOSE。
// CHANGE 表示退款异常（需人工处理），不能当成完成。
func wechatRefundState(status string) refundSettleState {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "SUCCESS":
		return refundSettleDone
	case "REFUNDCLOSE":
		return refundSettleFailed
	default:
		return refundSettlePending
	}
}

// alipayRefundState 支付宝退款通知里的 refund_status 取值是 REFUND_SUCCESS（不是 SUCCESS）。
// 历史上代码比较的是 "SUCCESS"，于是一笔真实成功的退款会被写成「退款失败」。
// 非 REFUND_SUCCESS 一律按「还在处理」处理：本地凭这个值判失败会让订单永远停在已付款。
func alipayRefundState(status string) refundSettleState {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "REFUND_SUCCESS", "SUCCESS":
		return refundSettleDone
	default:
		return refundSettlePending
	}
}

// xunhupayRefundState 虎皮椒：CD 已退款 / RD 退款中 / UD 退款失败。
// 原来 RD（退款中）会让订单直接变已退款——钱还在路上就先记账。
func xunhupayRefundState(status string) refundSettleState {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case XunhupayStatusRefunded:
		return refundSettleDone
	case XunhupayStatusRefundFailed:
		return refundSettleFailed
	default:
		return refundSettlePending
	}
}

// stripeRefundState Stripe 退款状态：succeeded / pending / failed / canceled / requires_action。
func stripeRefundState(status stripe.RefundStatus) refundSettleState {
	switch status {
	case stripe.RefundStatusSucceeded:
		return refundSettleDone
	case stripe.RefundStatusFailed, stripe.RefundStatusCanceled:
		return refundSettleFailed
	default:
		return refundSettlePending
	}
}

// ApplyRefundGatewayState 把网关侧的退款状态落到本地退款单。
//
// 只有 refundSettleDone 才会推进订单与退款流水；Pending/Failed 只更新退款单本身，
// 这样「后台点退款」和「网关通知」谁先到都不会算出两笔账（SuccessRefundOrder 里还有条件更新闸门）。
func (w *Website) ApplyRefundGatewayState(refund *model.OrderRefund, order *model.Order, state refundSettleState, terraceRefundId, remark string) error {
	if refund == nil {
		return errors.New("refund is nil")
	}
	updates := make(map[string]interface{}, 4)
	if terraceRefundId != "" && refund.TerraceId != terraceRefundId {
		refund.TerraceId = terraceRefundId
		updates["terrace_id"] = terraceRefundId
	}
	if remark = truncateRunes(strings.TrimSpace(remark), 255); remark != "" && remark != refund.Remark {
		refund.Remark = remark
		updates["remark"] = remark
	}
	switch state {
	case refundSettleDone:
		refund.Status = config.OrderRefundStatusDone
		if refund.RefundTime == 0 {
			refund.RefundTime = time.Now().Unix()
			updates["refund_time"] = refund.RefundTime
		}
		updates["status"] = refund.Status
	case refundSettleFailed:
		refund.Status = config.OrderRefundStatusFailed
		updates["status"] = refund.Status
	default:
		refund.Status = config.OrderRefundStatusWaiting
		updates["status"] = refund.Status
	}
	if len(updates) > 0 && refund.Id > 0 && w.DB != nil {
		if err := w.DB.Model(&model.OrderRefund{}).Where("id = ?", refund.Id).Updates(updates).Error; err != nil {
			return err
		}
	}
	if state != refundSettleDone {
		return nil
	}
	if order == nil {
		o, err := w.GetOrderInfoByOrderId(refund.OrderId)
		if err != nil {
			return err
		}
		order = o
	}
	return w.SuccessRefundOrder(refund, order)
}

// wechatPayClient 按支付账号构造带证书的微信客户端。
// 微信退款走 /secapi/pay/refund，必须带商户证书；证书缺失时提前报错，
// 别等微信返回一句「证书错误」让运营去猜。
func (w *Website) wechatPayClient(account *model.PaymentAccount) (*wechat.Client, error) {
	if account == nil {
		return nil, errors.New("payment account is nil")
	}
	client := wechat.NewClient(account.PayConfig.AppId, account.PayConfig.Account, account.PayConfig.ApiKey, !account.PayConfig.Sandbox)
	if err := client.AddCertPemFilePath(w.DataPath+"cert/"+account.PayConfig.CertPath, w.DataPath+"cert/"+account.PayConfig.PublicCertPath); err != nil {
		return nil, fmt.Errorf("微信证书错误：%w", err)
	}
	return client, nil
}

// RefundWechatPayment 发起微信退款并回查一次真实状态。
//
// 三个要点：
//   - total_fee 必须是**原订单实付金额**（payment.Amount）。原来传的是 order.Amount，
//     订单被改价/用券后两者不等，微信会直接报「退款金额大于订单金额」。
//   - out_refund_no 用本地 refund.RefundId：重试同一个退款单时微信按它幂等，不会退两次。
//   - 微信 V2 退款是**异步**的，接口返回 result_code=SUCCESS 只代表受理成功，
//     真实结果要看 refundquery 的 refund_status 或退款结果通知。这里立刻回查一次，
//     查到 PROCESSING 就保持待处理，交给 SyncPendingRefunds 收尾。
func (w *Website) RefundWechatPayment(ctx context.Context, payment *model.Payment, refund *model.OrderRefund) (refundSettleState, error) {
	if payment == nil || refund == nil {
		return refundSettlePending, errors.New("payment or refund is nil")
	}
	if payment.PayWay != config.PayWayWechat && payment.PayWay != config.PayWayWeapp {
		return refundSettlePending, fmt.Errorf("payment %s 不是微信支付", payment.PaymentId)
	}
	if payment.PaidTime == 0 {
		return refundSettlePending, fmt.Errorf("支付单 %s 未支付，无法退款", payment.PaymentId)
	}
	if refund.Amount <= 0 || refund.Amount > payment.Amount {
		return refundSettlePending, fmt.Errorf("退款金额 %d 非法（支付金额 %d）", refund.Amount, payment.Amount)
	}
	account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		return refundSettlePending, err
	}
	client, err := w.wechatPayClient(account)
	if err != nil {
		return refundSettlePending, err
	}

	bm := make(gopay.BodyMap)
	bm.Set("nonce_str", config.GenerateRandString(32)).
		Set("out_trade_no", payment.PaymentId).
		Set("out_refund_no", refund.RefundId).
		Set("total_fee", payment.Amount).
		Set("refund_fee", refund.Amount).
		Set("sign_type", wechat.SignType_HMAC_SHA256)
	// 退款结果通知必须在这里显式给地址：微信不会推到下单时的 notify_url，
	// 报文是加密的 req_info（见 wechatRefundNotify.go），所以两者复用同一个端点。
	if notifyURL := w.WechatRefundNotifyURL(); notifyURL != "" {
		bm.Set("notify_url", notifyURL)
	}

	wxRsp, _, err := client.Refund(ctx, bm)
	if err != nil {
		return refundSettlePending, err
	}
	if wxRsp.ReturnCode == gopay.FAIL {
		return refundSettlePending, errors.New(wxRsp.ReturnMsg)
	}
	if wxRsp.ResultCode == gopay.FAIL {
		return refundSettleFailed, fmt.Errorf("%s（%s）", wxRsp.ErrCodeDes, wxRsp.ErrCode)
	}
	if wxRsp.RefundId != "" {
		refund.TerraceId = wxRsp.RefundId
	}

	state, qErr := w.queryWechatRefundState(ctx, client, payment, refund)
	if qErr != nil {
		// 受理已经成功，查不到就保持待处理，别把退款判成失败
		w.debugRefund("wechat_refund", "refund query failed", refund.RefundId, qErr)
		return refundSettlePending, nil
	}
	return state, nil
}

// queryWechatRefundState 用 out_refund_no 查单笔退款的真实状态。
func (w *Website) queryWechatRefundState(ctx context.Context, client *wechat.Client, payment *model.Payment, refund *model.OrderRefund) (refundSettleState, error) {
	bm := make(gopay.BodyMap)
	bm.Set("nonce_str", config.GenerateRandString(32)).
		Set("out_trade_no", payment.PaymentId).
		Set("out_refund_no", refund.RefundId)
	_, resBm, err := client.QueryRefund(ctx, bm)
	if err != nil {
		return refundSettlePending, err
	}
	if resBm.GetString("return_code") == gopay.FAIL {
		return refundSettlePending, errors.New(resBm.GetString("return_msg"))
	}
	if resBm.GetString("result_code") == gopay.FAIL {
		return refundSettlePending, errors.New(resBm.GetString("err_code_des"))
	}
	status := resBm.GetString("refund_status_0")
	if status == "" {
		return refundSettlePending, nil
	}
	return wechatRefundState(status), nil
}

// RefundStripePayment 发起 Stripe 退款。
//
// 修掉的两处：
//   - Amount 原来是注释掉的，于是无论退多少都按**全额**退（后台填了部分退款金额照样全退）。
//   - 没有幂等键、也不落退款单 id：接口超时重试就会退第二笔。现在用 refund.RefundId 作幂等键，
//     并且重试时先 Retrieve 已有的退款单，只有查不到（说明上次确实没建成）才重新发起。
func (w *Website) RefundStripePayment(ctx context.Context, payment *model.Payment, refund *model.OrderRefund) (refundSettleState, error) {
	if payment == nil || refund == nil {
		return refundSettlePending, errors.New("payment or refund is nil")
	}
	if payment.PayWay != config.PayWayStripe {
		return refundSettlePending, fmt.Errorf("payment %s 不是 Stripe 支付", payment.PaymentId)
	}
	if payment.PaidTime == 0 {
		return refundSettlePending, fmt.Errorf("支付单 %s 未支付，无法退款", payment.PaymentId)
	}
	if payment.TerraceId == "" {
		return refundSettlePending, fmt.Errorf("支付单 %s 缺少 PaymentIntent id", payment.PaymentId)
	}
	if refund.Amount <= 0 || refund.Amount > payment.Amount {
		return refundSettlePending, fmt.Errorf("退款金额 %d 非法（支付金额 %d）", refund.Amount, payment.Amount)
	}
	account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		return refundSettlePending, err
	}
	client := stripe.NewClient(account.PayConfig.AppId)

	if refund.TerraceId != "" {
		if r, rErr := client.V1Refunds.Retrieve(ctx, refund.TerraceId, nil); rErr == nil {
			return stripeRefundState(r.Status), nil
		}
	}

	params := &stripe.RefundCreateParams{
		PaymentIntent: stripe.String(payment.TerraceId),
	}
	// 部分退款必须显式带金额，否则 Stripe 按整笔 PaymentIntent 全额退
	if refund.Amount < payment.Amount {
		params.Amount = stripe.Int64(refund.Amount)
	}
	params.SetIdempotencyKey("anqicms-refund-" + refund.RefundId)
	params.AddMetadata("payment_id", payment.PaymentId)
	params.AddMetadata("refund_id", refund.RefundId)
	if refund.Remark != "" {
		params.AddMetadata("remark", truncateRunes(refund.Remark, 200))
	}

	r, err := client.V1Refunds.Create(ctx, params)
	if err != nil {
		return refundSettlePending, fmt.Errorf("stripe create refund error: %w", err)
	}
	if r.ID != "" {
		refund.TerraceId = r.ID
	}
	if r.Amount > 0 && r.Amount != refund.Amount && refund.Amount < payment.Amount {
		// 退的金额跟申请的不一致：钱已经出去了，不能因此把退款判成失败
		// （失败状态会挡住 SuccessRefundOrder，订单永远退不掉），只记日志待人工核对。
		w.debugRefund("stripe_refund", "amount mismatch", refund.RefundId, r.Amount, refund.Amount)
	}
	return stripeRefundState(r.Status), nil
}

// SyncPendingRefunds 回查「已受理但未出结果」的退款。
//
// 只靠异步通知是不够的：通知可能被前面的「已支付」短路挡掉、被反代吞掉、或者商户号
// 根本没订阅。没有这个兜底，一笔退款会永远停在待处理。
func (w *Website) SyncPendingRefunds() {
	if w.DB == nil {
		return
	}
	var refunds []*model.OrderRefund
	w.DB.Model(&model.OrderRefund{}).
		Where("`status` = ?", config.OrderRefundStatusWaiting).
		Order("id asc").Limit(50).Find(&refunds)
	if len(refunds) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	for _, refund := range refunds {
		payment, err := w.GetPaymentInfoByOrderId(refund.OrderId)
		if err != nil || payment == nil {
			continue
		}
		var state refundSettleState
		var remark string
		switch payment.PayWay {
		case config.PayWayPaypal:
			state = w.syncPaypalRefundState(ctx, payment, refund)
		case config.PayWayWechat, config.PayWayWeapp:
			state = w.syncWechatRefundState(ctx, payment, refund)
		case config.PayWayStripe:
			state = w.syncStripeRefundState(ctx, payment, refund)
		default:
			// 虎皮椒等没有「查退款」接口的渠道，只能等它自己的退款通知
			continue
		}
		if state == refundSettlePending {
			continue
		}
		if err = w.ApplyRefundGatewayState(refund, nil, state, refund.TerraceId, remark); err != nil {
			w.debugRefund("refund_sync", "apply state failed", refund.RefundId, err)
		}
	}
}

func (w *Website) syncPaypalRefundState(ctx context.Context, payment *model.Payment, refund *model.OrderRefund) refundSettleState {
	if refund.TerraceId == "" {
		return refundSettlePending
	}
	account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		return refundSettlePending
	}
	client, err := newPaypalClient(account)
	if err != nil {
		return refundSettlePending
	}
	done, _, err := w.syncPaypalRefund(ctx, client, refund)
	if err != nil {
		w.debugRefund("refund_sync", "paypal refund query failed", refund.RefundId, err)
		return refundSettlePending
	}
	if done {
		return refundSettleDone
	}
	return refundSettlePending
}

func (w *Website) syncWechatRefundState(ctx context.Context, payment *model.Payment, refund *model.OrderRefund) refundSettleState {
	account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		return refundSettlePending
	}
	client, err := w.wechatPayClient(account)
	if err != nil {
		return refundSettlePending
	}
	state, err := w.queryWechatRefundState(ctx, client, payment, refund)
	if err != nil {
		w.debugRefund("refund_sync", "wechat refund query failed", refund.RefundId, err)
		return refundSettlePending
	}
	return state
}

func (w *Website) syncStripeRefundState(ctx context.Context, payment *model.Payment, refund *model.OrderRefund) refundSettleState {
	if refund.TerraceId == "" {
		return refundSettlePending
	}
	account, err := w.GetPaymentAccountById(payment.PaymentAccountId)
	if err != nil {
		return refundSettlePending
	}
	client := stripe.NewClient(account.PayConfig.AppId)
	r, err := client.V1Refunds.Retrieve(ctx, refund.TerraceId, nil)
	if err != nil {
		w.debugRefund("refund_sync", "stripe refund retrieve failed", refund.RefundId, err)
		return refundSettlePending
	}
	return stripeRefundState(r.Status)
}

// HandleAlipayRefundNotify 处理支付宝的退款到账通知。
//
// 注意 refund_status 的取值是 REFUND_SUCCESS（不是 SUCCESS）：原来拿 "SUCCESS" 去比，
// 一笔真实成功的退款会被写成「退款失败」，而失败状态又不会再被任何东西纠正。
func (w *Website) HandleAlipayRefundNotify(outTradeNo, refundStatus, outBizNo, gmtRefund string) error {
	if outTradeNo == "" {
		return errors.New("alipay refund notify: out_trade_no is empty")
	}
	payment, err := w.GetPaymentInfoByPaymentId(outTradeNo)
	if err != nil {
		return err
	}
	order, err := w.GetOrderInfoByOrderId(payment.OrderId)
	if err != nil {
		return err
	}
	refund, err := w.ensureOrderRefund(order, payment.Amount)
	if err != nil {
		return err
	}
	if term := strings.TrimSpace(gmtRefund); term != "" && refund.RefundTime == 0 {
		if t, pErr := time.ParseInLocation("2006-01-02 15:04:05", term, time.Local); pErr == nil {
			refund.RefundTime = t.Unix()
		}
	}
	// out_biz_no 是我们自己传的 out_request_no（= refund.RefundId），不是网关退款单号，
	// 不能写进 terrace_id（那一列存网关侧 id）。支付宝退款是同步的，不需要它回查。
	_ = outBizNo
	return w.ApplyRefundGatewayState(refund, order, alipayRefundState(refundStatus), "", "支付宝退款通知："+refundStatus)
}

// HandleXunhupayRefundNotify 处理虎皮椒的退款状态通知（CD/RD/UD）。
//
// 原来这里不分状态一律调 SuccessRefundOrder：RD（退款中）和 UD（退款失败）
// 都会把订单改成「已退款」并记一笔退款流水，钱没退出去账上先退了一笔。
func (w *Website) HandleXunhupayRefundNotify(outTradeNo, status string) error {
	if outTradeNo == "" {
		return errors.New("xunhupay refund notify: trade_order_id is empty")
	}
	payment, err := w.GetPaymentInfoByPaymentId(outTradeNo)
	if err != nil {
		return err
	}
	order, err := w.GetOrderInfoByOrderId(payment.OrderId)
	if err != nil {
		return err
	}
	refund, err := w.ensureOrderRefund(order, payment.Amount)
	if err != nil {
		return err
	}
	return w.ApplyRefundGatewayState(refund, order, xunhupayRefundState(status), refund.TerraceId, "虎皮椒退款通知："+status)
}

// debugRefund 把退款同步过程写进站点的 cache 日志，方便运营对账。
func (w *Website) debugRefund(file string, v ...interface{}) {
	library.DebugLog(w.CachePath, file, v...)
}

// wxRefundRemark / stripeRefundRemark 把三态翻成后台能看懂的一句话，
// 待处理时运营一眼就知道「不是在等我们，是在等通道」。
func wxRefundRemark(state refundSettleState) string {
	switch state {
	case refundSettleDone:
		return "微信退款成功"
	case refundSettleFailed:
		return "微信退款关闭/失败，请到商户平台核实"
	default:
		return "微信已受理退款，等待退款结果"
	}
}

func stripeRefundRemark(state refundSettleState) string {
	switch state {
	case refundSettleDone:
		return "Stripe 退款成功"
	case refundSettleFailed:
		return "Stripe 退款失败，请到 Stripe 后台核实"
	default:
		return "Stripe 已受理退款（pending），等待最终结果"
	}
}
