package provider

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// 虎皮椒（xunhupay.com）聚合支付：个人/小微商户走它的网关收微信、支付宝。
// 官方文档把网关地址列为「可配置变量」——主站故障时切到备用域名，所以 api_url 走支付账号配置。
const (
	XunhupayDefaultApiUrl = "https://api.xunhupay.com"
	XunhupayVersion       = "1.1"
	XunhupayPathPay       = "/payment/do.html"
	XunhupayPathQuery     = "/payment/query.html"
	XunhupayPathRefund    = "/payment/refund.html"

	// 订单/退款状态，通知与查询接口共用
	XunhupayStatusPaid         = "OD" // 已支付
	XunhupayStatusRefunded     = "CD" // 已退款
	XunhupayStatusRefunding    = "RD" // 退款中
	XunhupayStatusRefundFailed = "UD" // 退款失败
)

// XunhupayApiUrl 支付网关地址。账号里没配置就用官方主站。
func XunhupayApiUrl(account *model.PaymentAccount) string {
	apiUrl := ""
	if account != nil {
		apiUrl = strings.TrimSpace(account.PayConfig.ApiUrl)
	}
	if apiUrl == "" {
		apiUrl = XunhupayDefaultApiUrl
	}
	return strings.TrimRight(apiUrl, "/")
}

// XunhupaySign 生成虎皮椒签名。
//
// 文档「HASH生成的步骤如下」：除 hash 外的**非空**参数按参数名 ASCII 升序拼成 key=value&key=value，
// 末尾**直接**拼上 APPSECRET（中间没有任何连接符），再做 MD5，取 32 位小写。
// 参数名区分大小写，值为空不参与签名 —— 这两点决定了不能用 map 顺序遍历，必须显式排序。
func XunhupaySign(params map[string]string, appSecret string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "hash" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(params[k])
	}
	sb.WriteString(appSecret)

	sum := md5.Sum([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// XunhupayVerifySign 校验参数里的 hash。params 可含 hash，签名时会自动排除。
// 返回的错误会被写进回调日志 —— 静默返回 false 是这类回调最典型的失效形态。
func XunhupayVerifySign(params map[string]string, appSecret string) error {
	if appSecret == "" {
		return errors.New("xunhupay app secret is empty")
	}
	got := strings.ToLower(strings.TrimSpace(params["hash"]))
	if got == "" {
		return errors.New("xunhupay sign (hash) is empty")
	}
	want := XunhupaySign(params, appSecret)
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return errors.New("xunhupay sign mismatch")
	}
	return nil
}

// XunhupayFormParams 把 form 表单（回调通知是 form 提交）转成 map[string]string。
// 多值字段取第一个：虎皮椒没有重复参数，取第一个足以覆盖。
func XunhupayFormParams(form url.Values) map[string]string {
	params := make(map[string]string, len(form))
	for k, v := range form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	return params
}

// XunhupayAmountFen 把网关通知里的「元」金额换成「分」。
// 用 +0.5 四舍五入而不是直接截断：19.99*100 在浮点里是 1998.9999999999998，截断会少一分。
func XunhupayAmountFen(amount string) (int64, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(amount), 64)
	if err != nil {
		return 0, fmt.Errorf("xunhupay amount %q is not a number: %v", amount, err)
	}
	return int64(v*100 + 0.5), nil
}

// XunhupayNotifySignVerify 校验回调可信度：签名 + appid 归属。
// 两步都要做——签名只证明「知道密钥」，多站点共用一个网关时还要证明「是发给我的」。
func XunhupayNotifySignVerify(params map[string]string, appSecret, appId string) error {
	if err := XunhupayVerifySign(params, appSecret); err != nil {
		return err
	}
	if appId != "" && params["appid"] != appId {
		return fmt.Errorf("xunhupay notify appid %q does not match account %q", params["appid"], appId)
	}
	return nil
}

type xunhupayResponse struct {
	Errcode   int         `json:"errcode"`
	Errmsg    string      `json:"errmsg"`
	Hash      string      `json:"hash"`
	Url       string      `json:"url"`
	UrlQrcode string      `json:"url_qrcode"`
	Openid    string      `json:"openid"` // 历史遗留字段名，值是虎皮椒内部订单号
	Data      interface{} `json:"data"`
}

type xunhupayQueryData struct {
	Status        payFlexString `json:"status"`
	OpenOrderId   payFlexString `json:"open_order_id"`
	TransactionId payFlexString `json:"transaction_id"`
	TotalFee      payFlexString `json:"total_fee"`
}

// verifyXunhupayResponseHash 校验接口返回的 hash（防中间人篡改支付链接）。
// 只在返回体是「扁平对象」时校验：查询接口带嵌套 data，文档没规定其签名算法，
// 不去猜；创建支付这一路是本函数唯一用得上的地方，且它的返回是扁平的。
func verifyXunhupayResponseHash(raw []byte, appSecret string) error {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return err
	}
	signHash, _ := obj["hash"].(string)
	if signHash == "" {
		return nil
	}
	params := make(map[string]string, len(obj))
	for k, v := range obj {
		switch val := v.(type) {
		case string:
			params[k] = val
		case float64:
			// errcode 这类整数会被解成 float64，用 'f' 去掉多余小数位还原
			params[k] = strconv.FormatFloat(val, 'f', -1, 64)
		case bool:
			params[k] = strconv.FormatBool(val)
		case nil:
			// 空值不参与签名
		default:
			// 出现嵌套结构，不去猜签名算法
			return nil
		}
	}
	return XunhupayVerifySign(params, appSecret)
}

func (w *Website) xunhupayApi(account *model.PaymentAccount, path string, params map[string]string) ([]byte, *xunhupayResponse, error) {
	if account == nil {
		return nil, nil, errors.New("payment account is nil")
	}
	if account.PayConfig.AppId == "" || account.PayConfig.AppSecret == "" {
		return nil, nil, errors.New("xunhupay appid or appsecret is empty")
	}
	params["appid"] = account.PayConfig.AppId
	params["time"] = strconv.FormatInt(time.Now().Unix(), 10)
	if params["nonce_str"] == "" {
		params["nonce_str"] = config.GenerateRandString(32)
	}
	params["hash"] = XunhupaySign(params, account.PayConfig.AppSecret)

	body, err := json.Marshal(params)
	if err != nil {
		return nil, nil, err
	}
	raw, status, err := paymentPostJSON(XunhupayApiUrl(account)+path, body, nil)
	if err != nil {
		return nil, nil, err
	}
	if status != 200 {
		return raw, nil, fmt.Errorf("xunhupay http status %d: %s", status, string(raw))
	}

	var res xunhupayResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return raw, nil, fmt.Errorf("xunhupay response decode failed: %v, body: %s", err, string(raw))
	}
	if res.Errcode != 0 {
		return raw, &res, fmt.Errorf("xunhupay error %d: %s", res.Errcode, res.Errmsg)
	}
	if err := verifyXunhupayResponseHash(raw, account.PayConfig.AppSecret); err != nil {
		return raw, &res, fmt.Errorf("xunhupay response sign invalid: %v", err)
	}

	return raw, &res, nil
}

// xunhupayTitle 订单标题。文档限制「不超过 127 字符或 42 个汉字，不能有表情和 %」，
// 超长或带 % 会被网关直接拒单，这里按 40 个字符截断并去掉 %。
func xunhupayTitle(remark string) string {
	return paymentOrderTitle(remark, 40)
}

// CreateXunhupayPayment 在网关下单，返回（跳转地址，二维码图片地址）。
// trade_order_id 用 payment.PaymentId，回调按它找回支付单。
func (w *Website) CreateXunhupayPayment(payment *model.Payment, account *model.PaymentAccount, notifyUrl, returnUrl string) (string, string, error) {
	params := map[string]string{
		"version":        XunhupayVersion,
		"trade_order_id": payment.PaymentId,
		// total_fee 单位是元，payment.Amount 是分
		"total_fee":  strconv.FormatFloat(float64(payment.Amount)/100, 'f', 2, 64),
		"title":      xunhupayTitle(payment.Remark),
		"notify_url": notifyUrl,
		"return_url": returnUrl,
		"plugins":    "anqicms",
		"attach":     payment.PaymentId,
	}
	_, res, err := w.xunhupayApi(account, XunhupayPathPay, params)
	if err != nil {
		return "", "", err
	}
	if res.Url == "" && res.UrlQrcode == "" {
		return "", "", errors.New("xunhupay returned no pay url")
	}
	// openid 字段实为虎皮椒内部订单号，存下来供退款/查单使用
	if res.Openid != "" {
		w.DB.Model(payment).UpdateColumn("terrace_id", res.Openid)
		payment.TerraceId = res.Openid
	}
	return res.Url, res.UrlQrcode, nil
}

// QueryXunhupayPayment 主动查单，返回订单状态（OD/CD/…）。用于回调丢失时补单。
func (w *Website) QueryXunhupayPayment(payment *model.Payment, account *model.PaymentAccount) (string, error) {
	params := map[string]string{
		"out_trade_order": payment.PaymentId,
	}
	raw, _, err := w.xunhupayApi(account, XunhupayPathQuery, params)
	if err != nil {
		return "", err
	}
	// 查询接口把业务数据放在 data 里，且 errcode 已由 xunhupayApi 判过
	var wrap struct {
		Data xunhupayQueryData `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return "", fmt.Errorf("xunhupay query decode failed: %v", err)
	}
	return string(wrap.Data.Status), nil
}

// RefundXunhupayPayment 原路退款。虎皮椒只支持整单退款，没有退款金额参数，
// 所以部分退款必须挡在这里 —— 否则会出现「后台显示已退款、实际只退了一部分」。
//
// 返回三态而不是 error：REFUNDING（RD）是**已经受理、钱还没到**，
// 原来它和 CD 一样 return nil，调用方于是立刻把订单标成已退款并记退款流水 ——
// 与 PayPal 的 PENDING 被当成完成是同一个错误。RD 现在保持待处理，
// 由虎皮椒的退款通知（同一个 notify_url）或人工跟进收敛。
func (w *Website) RefundXunhupayPayment(payment *model.Payment, account *model.PaymentAccount, refundAmount int64, reason string) (refundSettleState, error) {
	if refundAmount > 0 && refundAmount < payment.Amount {
		return refundSettlePending, fmt.Errorf("xunhupay does not support partial refund (order %s, refund %s)",
			strconv.FormatInt(payment.Amount, 10), strconv.FormatInt(refundAmount, 10))
	}
	params := map[string]string{
		"trade_order_id": payment.PaymentId,
		"reason":         reason,
	}
	raw, res, err := w.xunhupayApi(account, XunhupayPathRefund, params)
	if err != nil {
		return refundSettlePending, err
	}
	var result struct {
		RefundStatus payFlexString `json:"refund_status"`
		RefundFee    payFlexString `json:"refund_fee"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return refundSettlePending, fmt.Errorf("xunhupay refund decode failed: %v", err)
	}
	status := string(result.RefundStatus)
	switch xunhupayRefundState(status) {
	case refundSettleDone:
		return refundSettleDone, nil
	case refundSettleFailed:
		return refundSettleFailed, errors.New("xunhupay refund failed")
	default:
		if status != XunhupayStatusRefunding {
			// errcode=0 但状态不是预期值：不要静默当成功，也不要当失败（可能只是没见过的新状态）
			return refundSettlePending, fmt.Errorf("xunhupay refund unexpected status: %q (errcode %d, errmsg %s)", status, res.Errcode, res.Errmsg)
		}
		return refundSettlePending, nil
	}
}
