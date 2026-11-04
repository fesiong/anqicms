package provider

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"kandaoni.com/anqicms/model"
)

// NOWPayments（nowpayments.io）加密货币收款。
// 走「发票（invoice）」模式：用户在托管页选币种付款，我们只需要拿到 invoice_url 并等 IPN。
const (
	NowpaymentDefaultApiUrl = "https://api.nowpayments.io"
	NowpaymentSandboxApiUrl = "https://api.sandbox.nowpayments.io"

	// IPN / 查询返回的 payment_status
	NowpaymentStatusWaiting       = "waiting"
	NowpaymentStatusConfirming    = "confirming"
	NowpaymentStatusConfirmed     = "confirmed"
	NowpaymentStatusSending       = "sending"
	NowpaymentStatusPartiallyPaid = "partially_paid"
	NowpaymentStatusFinished      = "finished"
	NowpaymentStatusFailed        = "failed"
	NowpaymentStatusRefunded      = "refunded"
	NowpaymentStatusExpired       = "expired"
)

// NowpaymentApiUrl 网关地址。账号里填了 api_url 优先（指向沙盒），否则按 sandbox 开关选默认。
func NowpaymentApiUrl(account *model.PaymentAccount) string {
	if account == nil {
		return NowpaymentDefaultApiUrl
	}
	if apiUrl := strings.TrimSpace(account.PayConfig.ApiUrl); apiUrl != "" {
		return strings.TrimRight(apiUrl, "/")
	}
	if account.PayConfig.Sandbox {
		return NowpaymentSandboxApiUrl
	}
	return NowpaymentDefaultApiUrl
}

// NowpaymentPriceCurrency 发票的法币计价币种。payment.Amount 只有数值没有币种，
// 由支付账号显式声明（usd/eur/cny…），缺省 usd。
func NowpaymentPriceCurrency(account *model.PaymentAccount) string {
	if account != nil {
		if c := strings.ToLower(strings.TrimSpace(account.PayConfig.PriceCurrency)); c != "" {
			return c
		}
	}
	return "usd"
}

// NowpaymentSortedJSON 按 IPN 文档把参数重排成待签名串。
//
// 官方要求等价于 JS 的 JSON.stringify(params, Object.keys(params).sort())：只重排顶层键、值原样保留。
// 这里用 json.RawMessage 保留每个值的**原始字面量**（170 不会变成 170.0、字符串不会被重新转义），
// 重新解码再编码数值是签名对不上的经典原因。
func NowpaymentSortedJSON(body []byte) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		sb.Write(key)
		sb.WriteByte(':')
		sb.Write(raw[k])
	}
	sb.WriteByte('}')
	return []byte(sb.String()), nil
}

// VerifyNowpaymentIPN 校验 IPN 回调的 x-nowpayments-sig。
// 算法：参数按 key 升序 → JSON 序列化 → 用 IPN Secret 做 HMAC-SHA512 → 十六进制小写。
func VerifyNowpaymentIPN(body []byte, signature, ipnSecret string) error {
	if strings.TrimSpace(ipnSecret) == "" {
		return errors.New("nowpayment ipn secret is empty")
	}
	signature = strings.ToLower(strings.TrimSpace(signature))
	if signature == "" {
		return errors.New("nowpayment ipn signature (x-nowpayments-sig) is empty")
	}
	sorted, err := NowpaymentSortedJSON(body)
	if err != nil {
		return fmt.Errorf("nowpayment ipn body invalid: %v", err)
	}
	mac := hmac.New(sha512.New, []byte(strings.TrimSpace(ipnSecret)))
	mac.Write(sorted)
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return errors.New("nowpayment ipn signature mismatch")
	}
	return nil
}

// NowpaymentNotify IPN 报文。字段见官方 IPN 文档示例；order_id 就是下单时传入的
// payment.PaymentId，是唯一能直接映射回本地支付单的字段（payment_id 是 NOWPayments 自己的支付号，
// 与发票 id 又不是同一个值，别拿它当主键）。
type NowpaymentNotify struct {
	PaymentId     payFlexString `json:"payment_id"`
	PaymentStatus string        `json:"payment_status"`
	PayAddress    string        `json:"pay_address"`
	PriceAmount   payFlexString `json:"price_amount"`
	PriceCurrency string        `json:"price_currency"`
	PayAmount     payFlexString `json:"pay_amount"`
	ActuallyPaid  payFlexString `json:"actually_paid"`
	PayCurrency   string        `json:"pay_currency"`
	OrderId       payFlexString `json:"order_id"`
	PurchaseId    payFlexString `json:"purchase_id"`
}

// ParseNowpaymentNotify 解析 IPN 报文（不校验签名，签名要用原始 body 另算）。
func ParseNowpaymentNotify(body []byte) (*NowpaymentNotify, error) {
	if len(body) == 0 {
		return nil, errors.New("nowpayment ipn body is empty")
	}
	var notify NowpaymentNotify
	if err := json.Unmarshal(body, &notify); err != nil {
		return nil, fmt.Errorf("nowpayment ipn decode failed: %v", err)
	}
	if notify.OrderId == "" {
		return nil, errors.New("nowpayment ipn order_id is empty")
	}
	return &notify, nil
}

// AmountFen 把 price_amount（法币金额，如 19.9 美元）换算成分，与 payment.Amount 对齐。
func (n *NowpaymentNotify) AmountFen() (int64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(string(n.PriceAmount)), 64)
	if err != nil {
		return 0, fmt.Errorf("nowpayment price_amount %q is not a number: %v", n.PriceAmount, err)
	}
	// 用 +0.5 四舍五入：19.99*100 在浮点里是 1998.9999999999998
	return int64(v*100 + 0.5), nil
}

// NowpaymentInvoice 发票 / 支付单。id 在部分接口是数字、部分接口是字符串，统一用 payFlexString。
type NowpaymentInvoice struct {
	Id            payFlexString `json:"id"`
	OrderId       payFlexString `json:"order_id"`
	OrderDesc     string        `json:"order_description"`
	PriceAmount   payFlexString `json:"price_amount"`
	PriceCurrency string        `json:"price_currency"`
	PayCurrency   string        `json:"pay_currency"`
	PayAmount     payFlexString `json:"pay_amount"`
	PayAddress    string        `json:"pay_address"`
	ActuallyPaid  payFlexString `json:"actually_paid"`
	// PaymentStatus 只在发票详情/支付状态里有，创建发票的返回不带
	PaymentStatus payFlexString `json:"payment_status"`
	InvoiceUrl    string        `json:"invoice_url"`
}

// NowpaymentIsPaid 判断状态是否表示「钱已经到账」。
//
// finished 是终态；confirmed/sending 表示链上已确认、正在结算到商户钱包——此时资金已经不可撤销，
// 可以放行订单。partially_paid（少付）与 confirming（等确认）**不能**算支付成功。
func NowpaymentIsPaid(status string) bool {
	switch status {
	case NowpaymentStatusFinished, NowpaymentStatusSending, NowpaymentStatusConfirmed:
		return true
	}
	return false
}

func (w *Website) nowpaymentHeaders(account *model.PaymentAccount) (map[string]string, error) {
	if account == nil {
		return nil, errors.New("payment account is nil")
	}
	if strings.TrimSpace(account.PayConfig.AppId) == "" {
		return nil, errors.New("nowpayment api key is empty")
	}
	return map[string]string{"x-api-key": strings.TrimSpace(account.PayConfig.AppId)}, nil
}

// CreateNowpaymentInvoice 创建发票，返回托管支付页地址。
// order_id 固定传 payment.PaymentId：IPN 回来时直接按它定位支付单，不用猜映射关系。
func (w *Website) CreateNowpaymentInvoice(payment *model.Payment, account *model.PaymentAccount, notifyUrl, successUrl, cancelUrl string) (*NowpaymentInvoice, error) {
	header, err := w.nowpaymentHeaders(account)
	if err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		// price_amount 是法币金额（元/美元），payment.Amount 是分
		"price_amount":      float64(payment.Amount) / 100,
		"price_currency":    NowpaymentPriceCurrency(account),
		"order_id":          payment.PaymentId,
		"order_description": paymentOrderTitle(payment.Remark, 100),
		"ipn_callback_url":  notifyUrl,
		"success_url":       successUrl,
		"cancel_url":        cancelUrl,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	raw, status, err := paymentPostJSON(NowpaymentApiUrl(account)+"/v1/invoice", body, header)
	if err != nil {
		return nil, err
	}
	if status != 200 && status != 201 {
		return nil, fmt.Errorf("nowpayment http status %d: %s", status, string(raw))
	}
	var invoice NowpaymentInvoice
	if err := json.Unmarshal(raw, &invoice); err != nil {
		return nil, fmt.Errorf("nowpayment invoice decode failed: %v, body: %s", err, string(raw))
	}
	if invoice.InvoiceUrl == "" {
		return nil, fmt.Errorf("nowpayment returned no invoice_url: %s", string(raw))
	}
	if invoice.Id != "" {
		w.DB.Model(payment).UpdateColumn("terrace_id", string(invoice.Id))
		payment.TerraceId = string(invoice.Id)
	}
	return &invoice, nil
}

// GetNowpaymentInvoice 查发票状态，用于回调丢失时补单。
func (w *Website) GetNowpaymentInvoice(account *model.PaymentAccount, invoiceId string) (*NowpaymentInvoice, error) {
	header, err := w.nowpaymentHeaders(account)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(invoiceId) == "" {
		return nil, errors.New("nowpayment invoice id is empty")
	}
	raw, status, err := paymentGetJSON(NowpaymentApiUrl(account)+"/v1/invoice/"+invoiceId, header)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("nowpayment http status %d: %s", status, string(raw))
	}
	var invoice NowpaymentInvoice
	if err := json.Unmarshal(raw, &invoice); err != nil {
		return nil, fmt.Errorf("nowpayment invoice decode failed: %v, body: %s", err, string(raw))
	}
	return &invoice, nil
}
