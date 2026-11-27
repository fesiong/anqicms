package provider

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// 微信 V2 的「退款结果通知」跟「支付结果通知」长得不一样：
// 只有 return_code / appid / mch_id / nonce_str / req_info 几个明文字段，
// 真正的退款结果（out_refund_no、refund_status、refund_id、success_time）在 req_info 里，
// 用商户 API 密钥派生的 AES-256-ECB 加密。gopay 没有提供解密，不自己解就永远读不到状态。
//
// 这也是「退款通知分支写了但永远收不到有效数据」的根源：原来那段代码直接从明文里取
// refund_id / refund_status，就算前面的「已支付就返回」不挡，也拿不到任何值。

// DecryptWechatRefundNotify 解密退款通知里的 req_info。
// key = MD5(apiKey) 的小写十六进制字符串（32 字节），AES-256-ECB，PKCS#7 填充。
func DecryptWechatRefundNotify(reqInfo, apiKey string) (map[string]string, error) {
	reqInfo = strings.TrimSpace(reqInfo)
	if reqInfo == "" {
		return nil, errors.New("wechat refund notify: req_info is empty")
	}
	if apiKey == "" {
		return nil, errors.New("wechat refund notify: api key is empty")
	}
	cipherText, err := base64.StdEncoding.DecodeString(reqInfo)
	if err != nil {
		return nil, fmt.Errorf("wechat refund notify: base64 decode failed: %w", err)
	}
	if len(cipherText) == 0 || len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("wechat refund notify: invalid cipher length %d", len(cipherText))
	}
	sum := md5.Sum([]byte(apiKey))
	key := []byte(hex.EncodeToString(sum[:]))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("wechat refund notify: new cipher failed: %w", err)
	}
	plain := make([]byte, len(cipherText))
	// ECB：逐块独立解密（微信这条协议就是这么定的，别改成 CBC）
	for i := 0; i < len(cipherText); i += aes.BlockSize {
		block.Decrypt(plain[i:i+aes.BlockSize], cipherText[i:i+aes.BlockSize])
	}
	plain, err = pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("wechat refund notify: unpad failed: %w", err)
	}

	var payload struct {
		XMLName      xml.Name `xml:"root"`
		OutTradeNo   string   `xml:"out_trade_no"`
		OutRefundNo  string   `xml:"out_refund_no"`
		RefundId     string   `xml:"refund_id"`
		RefundStatus string   `xml:"refund_status"`
		SuccessTime  string   `xml:"success_time"`
		RefundFee    string   `xml:"refund_fee"`
		TotalFee     string   `xml:"total_fee"`
	}
	if err = xml.Unmarshal(plain, &payload); err != nil {
		return nil, fmt.Errorf("wechat refund notify: xml decode failed: %w", err)
	}
	return map[string]string{
		"out_trade_no":  payload.OutTradeNo,
		"out_refund_no": payload.OutRefundNo,
		"refund_id":     payload.RefundId,
		"refund_status": payload.RefundStatus,
		"success_time":  payload.SuccessTime,
		"refund_fee":    payload.RefundFee,
		"total_fee":     payload.TotalFee,
	}, nil
}

func pkcs7Unpad(b []byte, blockSize int) ([]byte, error) {
	if len(b) == 0 || len(b)%blockSize != 0 {
		return nil, errors.New("invalid padded length")
	}
	pad := int(b[len(b)-1])
	if pad == 0 || pad > blockSize || pad > len(b) {
		return nil, errors.New("invalid padding")
	}
	for _, c := range b[len(b)-pad:] {
		if int(c) != pad {
			return nil, errors.New("invalid padding byte")
		}
	}
	return b[:len(b)-pad], nil
}

// DecryptWechatRefundNotifyWithAccount 退款通知的明文里**没有** out_trade_no
// （只有 return_code / appid / mch_id / nonce_str / req_info），
// 而解密又需要商户 API 密钥 —— 所以只能拿站点里所有微信支付账号挨个试，
// 能解开的那把密钥就是收款账号的那把。站点一般只有 1~3 个微信账号，成本可忽略。
func (w *Website) DecryptWechatRefundNotifyWithAccount(reqInfo string) (map[string]string, *model.PaymentAccount, error) {
	if strings.TrimSpace(reqInfo) == "" {
		return nil, nil, errors.New("wechat refund notify: req_info is empty")
	}
	accounts := w.GetPaymentAccounts(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`pay_way` IN ? AND `status` = 1", []string{config.PayWayWechat, config.PayWayWeapp})
	})
	var lastErr error
	for _, account := range accounts {
		if account.PayConfig.ApiKey == "" {
			continue
		}
		info, err := DecryptWechatRefundNotify(reqInfo, account.PayConfig.ApiKey)
		if err != nil {
			lastErr = err
			continue
		}
		return info, account, nil
	}
	if lastErr == nil {
		lastErr = errors.New("wechat refund notify: 没有可用的微信支付账号或账号未配置 API 密钥")
	}
	return nil, nil, lastErr
}

// HandleWechatRefundNotify 把解密出来的退款结果落到本地。
//
// 微信退款是异步的，这条通知才是「钱到底退没退」的第一手结论；
// 拿不到通知时由 SyncPendingRefunds 主动回查兜底。
func (w *Website) HandleWechatRefundNotify(outTradeNo, refundStatus, refundId, successTime string) error {
	if outTradeNo == "" {
		return errors.New("wechat refund notify: out_trade_no is empty")
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
	state := wechatRefundState(refundStatus)
	if state == refundSettleDone && refund.RefundTime == 0 && successTime != "" {
		if t, pErr := time.ParseInLocation("2006-01-02 15:04:05", successTime, time.Local); pErr == nil {
			refund.RefundTime = t.Unix()
		}
	}
	return w.ApplyRefundGatewayState(refund, order, state, refundId, "微信退款通知："+refundStatus)
}

// WechatRefundNotifyURL 退款结果通知的地址。
// 微信 V2 退款请求里的 notify_url 必须显式传，否则它不会推送退款结果
// （新版商户平台已经取消了「退款回调地址」的默认配置）。
func (w *Website) WechatRefundNotifyURL() string {
	base := strings.TrimSuffix(w.System.BaseUrl, "/")
	if base == "" {
		return ""
	}
	return base + "/notify/wechat/pay"
}
