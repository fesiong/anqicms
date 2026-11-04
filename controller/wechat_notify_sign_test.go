package controller

import (
	"testing"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/wechat"
	"kandaoni.com/anqicms/model"
)

// 这条测试锁的是 2026-11-04 线上回调失败的两个原因：
//  1. 验签密钥传了 appId（gopay 的第一个参数是商户 apiKey）；
//  2. 验签算法写死 MD5，而下单时 sign_type=HMAC-SHA256（微信通知的签名算法跟下单一致）。
//
// 两者都表现为 ok=false 且 err=nil，日志只有一行 `err <nil> gopay.BodyMap{...}`，
// 极易被误判成「回调没收到」或「订单不存在」。

const testWechatNotifyApiKey = "test_apikey_0123456789abcdef"

// testWechatNotifyBody 取自真实通知报文（去掉 sign 字段）
func testWechatNotifyBody() gopay.BodyMap {
	return gopay.BodyMap{
		"appid":          "wx45f85755824254e7",
		"bank_type":      "OTHERS",
		"cash_fee":       "19900",
		"fee_type":       "CNY",
		"is_subscribe":   "N",
		"mch_id":         "1607694099",
		"nonce_str":      "dtBCI8KIaeddzhaC",
		"openid":         "oM_Y0vuZzLVWtP4ysPtfJi46OruI",
		"out_trade_no":   "202611041104240420",
		"result_code":    "SUCCESS",
		"return_code":    "SUCCESS",
		"time_end":       "20261104110441",
		"total_fee":      "19900",
		"trade_type":     "NATIVE",
		"transaction_id": "4500000534202611047271207399",
	}
}

func testWechatNotifyAccount() *model.PaymentAccount {
	return &model.PaymentAccount{
		PayWay: "wechat",
		PayConfig: model.PayConfig{
			AppId:   "wx45f85755824254e7",
			Account: "1607694099",
			ApiKey:  testWechatNotifyApiKey,
		},
	}
}

// signWechatNotify 按指定密钥和算法对报文签名（不修改入参）
func signWechatNotify(bm gopay.BodyMap, apiKey, signType string) string {
	cp := make(gopay.BodyMap, len(bm))
	for k, v := range bm {
		cp[k] = v
	}
	cp.Remove("sign")
	return wechat.GetReleaseSign(apiKey, signType, cp)
}

func TestWechatNotifySignAcceptsHMACSHA256(t *testing.T) {
	account := testWechatNotifyAccount()
	bm := testWechatNotifyBody()
	bm.Set("sign", signWechatNotify(bm, testWechatNotifyApiKey, wechat.SignType_HMAC_SHA256))

	// 真实通知里 sign 是 64 位 HMAC-SHA256，不是 32 位 MD5；
	// 也就是说旧实现（写死 MD5）连长度都对不上，不可能验过。
	if got := len(bm.GetString("sign")); got != 64 {
		t.Fatalf("HMAC-SHA256 签名长度应为 64，实际 %d", got)
	}

	ok, err := verifyWechatNotifySign(account, bm)
	if err != nil {
		t.Fatalf("验签报错: %v", err)
	}
	if !ok {
		t.Fatal("正确的 HMAC-SHA256 签名应当验签通过")
	}
	// 副本保护：VerifySign 会 Remove("sign")，helper 不能把这个字段从调用方的 map 里删掉
	if bm.GetString("sign") == "" {
		t.Fatal("verifyWechatNotifySign 不应修改调用方的 BodyMap")
	}
}

// 反向对照：把密钥换回 appId（修复前的写法），helper 必须验不过。
// 如果谁把 helper 里的 ApiKey 改回 AppId，这条会红。
func TestWechatNotifySignRejectsAppIdAsKey(t *testing.T) {
	account := testWechatNotifyAccount()
	bm := testWechatNotifyBody()
	// 用 appId 当密钥签名，模拟修复前那种「自己签、自己验」都拿 appId 的情形
	bm.Set("sign", signWechatNotify(bm, account.PayConfig.AppId, wechat.SignType_HMAC_SHA256))

	ok, err := verifyWechatNotifySign(account, bm)
	if err != nil {
		t.Fatalf("验签报错: %v", err)
	}
	if ok {
		t.Fatal("用 appId 当密钥算出的签名不能通过校验，验签必须使用商户 apiKey")
	}
}

// 反向对照：内容被篡改（金额）必须验不过，证明校验不是恒真
func TestWechatNotifySignRejectsTamperedAmount(t *testing.T) {
	account := testWechatNotifyAccount()
	bm := testWechatNotifyBody()
	bm.Set("sign", signWechatNotify(bm, testWechatNotifyApiKey, wechat.SignType_HMAC_SHA256))

	bm.Set("total_fee", "1") // 签名后又改了金额
	ok, _ := verifyWechatNotifySign(account, bm)
	if ok {
		t.Fatal("报文被篡改后不应验签通过")
	}
}

// 旧订单兼容：按 MD5 下单的历史订单，通知 sign 是 32 位，仍应验签通过
func TestWechatNotifySignAcceptsLegacyMD5(t *testing.T) {
	account := testWechatNotifyAccount()
	bm := testWechatNotifyBody()
	bm.Set("sign", signWechatNotify(bm, testWechatNotifyApiKey, wechat.SignType_MD5))

	if got := len(bm.GetString("sign")); got != 32 {
		t.Fatalf("MD5 签名长度应为 32，实际 %d", got)
	}
	ok, err := verifyWechatNotifySign(account, bm)
	if err != nil {
		t.Fatalf("验签报错: %v", err)
	}
	if !ok {
		t.Fatal("32 位 MD5 签名应当按 MD5 回落校验通过")
	}
}

// 密钥没配置时要给出可诊断的错误，而不是静默 ok=false
func TestWechatNotifySignRequiresApiKey(t *testing.T) {
	account := testWechatNotifyAccount()
	account.PayConfig.ApiKey = ""
	bm := testWechatNotifyBody()
	bm.Set("sign", signWechatNotify(bm, testWechatNotifyApiKey, wechat.SignType_HMAC_SHA256))

	ok, err := verifyWechatNotifySign(account, bm)
	if ok {
		t.Fatal("缺少 apiKey 时不应验签通过")
	}
	if err == nil {
		t.Fatal("缺少 apiKey 时应返回可诊断的错误")
	}
}
