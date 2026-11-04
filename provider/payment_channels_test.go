package provider

import (
	"net/url"
	"strings"
	"testing"

	"kandaoni.com/anqicms/model"
)

// 签名向量由「按官方文档用 Python 独立实现一遍」算出，不是拿本仓 Go 实现自证。
// 虎皮椒签名规则：除 hash 外的非空参数按参数名 ASCII 升序拼 k=v&k=v，末尾直接接 APPSECRET，MD5 小写。
// 向量含 empty_field（空值不参与）与 hash（不参与），一并对齐官方 PHP 示例的两条 skip 规则。
func TestXunhupaySignMatchesSpec(t *testing.T) {
	params := map[string]string{
		"appid":          "201906100001",
		"trade_order_id": "202611041200000001",
		"total_fee":      "19.90",
		"title":          "Test Order",
		"time":           "1762300000",
		"nonce_str":      "abcdef123456",
		"empty_field":    "",
		"hash":           "should_be_ignored",
	}
	const want = "93556284bcbe382e0b238cb5fb2b72bb"
	if got := XunhupaySign(params, "secret_key_123"); got != want {
		t.Fatalf("签名不符：got %s, want %s", got, want)
	}

	// 反向对照：空值/hash 是否真被跳过 —— 去掉它们后结果必须一致
	delete(params, "empty_field")
	delete(params, "hash")
	if got := XunhupaySign(params, "secret_key_123"); got != want {
		t.Fatalf("空值与 hash 应不参与签名，got %s, want %s", got, want)
	}
	// 反向对照：换密钥必须变
	if XunhupaySign(params, "secret_key_124") == want {
		t.Fatal("换密钥后签名不应相同")
	}
	// 反向对照：参数名区分大小写，改一个 key 必须变
	params["Trade_order_id"] = params["trade_order_id"]
	delete(params, "trade_order_id")
	if XunhupaySign(params, "secret_key_123") == want {
		t.Fatal("参数名区分大小写，改动后签名不应相同")
	}
}

func TestXunhupayVerifySign(t *testing.T) {
	params := map[string]string{
		"appid":          "201906100001",
		"trade_order_id": "202611041200000001",
		"total_fee":      "19.90",
		"title":          "Test Order",
		"time":           "1762300000",
		"nonce_str":      "abcdef123456",
	}
	params["hash"] = XunhupaySign(params, "secret_key_123")
	if err := XunhupayVerifySign(params, "secret_key_123"); err != nil {
		t.Fatalf("正确签名应通过，got %v", err)
	}

	// 反向对照：逐项改坏都必须报错，且错误信息能区分是哪一类问题
	params["total_fee"] = "0.01"
	if err := XunhupayVerifySign(params, "secret_key_123"); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("金额被篡改应校验失败，got %v", err)
	}
	params["total_fee"] = "19.90"
	if err := XunhupayVerifySign(params, "wrong_secret"); err == nil {
		t.Fatal("错误密钥应校验失败")
	}
	// 空密钥是配置问题，必须给出可诊断信息而不是静默 false
	if err := XunhupayVerifySign(params, ""); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("空密钥应明确报错，got %v", err)
	}
	// 缺 hash
	delete(params, "hash")
	if err := XunhupayVerifySign(params, "secret_key_123"); err == nil {
		t.Fatal("缺 hash 应校验失败")
	}
}

func TestXunhupayFormParamsAndApiUrl(t *testing.T) {
	form := url.Values{
		"trade_order_id": {"202611041200000001"},
		"total_fee":      {"19.90"},
		"status":         {"OD", "ignored"},
	}
	params := XunhupayFormParams(form)
	if params["status"] != "OD" {
		t.Fatalf("多值字段应取第一个，got %q", params["status"])
	}
	if params["trade_order_id"] != "202611041200000001" {
		t.Fatalf("参数丢失，got %q", params["trade_order_id"])
	}

	// 网关地址：默认主站；账号里配了就用配置，并去掉尾部斜杠
	if got := XunhupayApiUrl(nil); got != XunhupayDefaultApiUrl {
		t.Fatalf("默认网关应为 %s，got %s", XunhupayDefaultApiUrl, got)
	}
	account := &model.PaymentAccount{PayConfig: model.PayConfig{ApiUrl: "https://api.dpweixin.com/"}}
	if got := XunhupayApiUrl(account); got != "https://api.dpweixin.com" {
		t.Fatalf("应使用配置的网关并去掉尾斜杠，got %s", got)
	}

	// NOWPayments：没配 api_url 时按 sandbox 开关选官方/沙盒地址
	if got := NowpaymentApiUrl(&model.PaymentAccount{}); got != NowpaymentDefaultApiUrl {
		t.Fatalf("默认网关应为 %s，got %s", NowpaymentDefaultApiUrl, got)
	}
	if got := NowpaymentApiUrl(&model.PaymentAccount{PayConfig: model.PayConfig{Sandbox: true}}); got != NowpaymentSandboxApiUrl {
		t.Fatalf("沙盒网关应为 %s，got %s", NowpaymentSandboxApiUrl, got)
	}
	if got := NowpaymentPriceCurrency(&model.PaymentAccount{}); got != "usd" {
		t.Fatalf("计价币种缺省应为 usd，got %s", got)
	}
	if got := NowpaymentPriceCurrency(&model.PaymentAccount{PayConfig: model.PayConfig{PriceCurrency: " CNY "}}); got != "cny" {
		t.Fatalf("计价币种应小写去空白，got %s", got)
	}
}

// NOWPayments 签名用「按 key 升序重排后的 JSON 原文」，值必须保持原字面量。
// 向量同样由 Python 独立实现算出（json.dumps(sort_keys=True, separators=(',',':'))）。
func TestNowpaymentSortedJSONAndVerifyIPN(t *testing.T) {
	body := []byte(`{"payment_id":5077125051,"payment_status":"finished","pay_address":"0xd1cDE08A07cD25adEbEd35c3867a59228C09B606","price_amount":19.9,"price_currency":"usd","pay_amount":0.00028571,"actually_paid":0.00028571,"pay_currency":"btc","order_id":"202611041200000001","order_description":"Test Order","purchase_id":"6084744717","created_at":"2026-11-04T05:00:00.000Z","updated_at":"2026-11-04T05:01:00.000Z","outcome_amount":19.5,"outcome_currency":"usd"}`)
	const wantSorted = `{"actually_paid":0.00028571,"created_at":"2026-11-04T05:00:00.000Z","order_description":"Test Order","order_id":"202611041200000001","outcome_amount":19.5,"outcome_currency":"usd","pay_address":"0xd1cDE08A07cD25adEbEd35c3867a59228C09B606","pay_amount":0.00028571,"pay_currency":"btc","payment_id":5077125051,"payment_status":"finished","price_amount":19.9,"price_currency":"usd","purchase_id":"6084744717","updated_at":"2026-11-04T05:01:00.000Z"}`

	sorted, err := NowpaymentSortedJSON(body)
	if err != nil {
		t.Fatalf("重排失败: %v", err)
	}
	if string(sorted) != wantSorted {
		t.Fatalf("重排结果不符：\n got %s\nwant %s", sorted, wantSorted)
	}
	// 反向断言：数值字面量不能被改写（19.9 变 19.90000 或 19 就签名不一致）
	if !strings.Contains(string(sorted), `"price_amount":19.9`) {
		t.Fatal("数值字面量应原样保留")
	}

	const ipnSecret = "ipn_secret_456"
	const wantSig = "20f31742c22847169d1b1b577e65d20670838ea767f23a6bed24850d9af9f240d0056531c74e947a97b74c86c43ca9eeb1685418bd835cf7f07f643da3aecceb"
	if err := VerifyNowpaymentIPN(body, wantSig, ipnSecret); err != nil {
		t.Fatalf("正确签名应通过，got %v", err)
	}
	// 反向对照：签名大小写不敏感（网关给的是小写，别因为大小写把真回调拒了）
	if err := VerifyNowpaymentIPN(body, strings.ToUpper(wantSig), ipnSecret); err != nil {
		t.Fatalf("签名比对应不区分大小写，got %v", err)
	}

	if err := VerifyNowpaymentIPN(body, wantSig, "wrong_secret"); err == nil {
		t.Fatal("错误 IPN Secret 应校验失败")
	}
	tampered := []byte(strings.Replace(string(body), `"price_amount":19.9`, `"price_amount":1.99`, 1))
	if err := VerifyNowpaymentIPN(tampered, wantSig, ipnSecret); err == nil {
		t.Fatal("报文被篡改后应校验失败")
	}
	if err := VerifyNowpaymentIPN(body, "", ipnSecret); err == nil || !strings.Contains(err.Error(), "x-nowpayments-sig") {
		t.Fatalf("缺签名头应明确报错，got %v", err)
	}
	if err := VerifyNowpaymentIPN(body, wantSig, ""); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("缺 IPN Secret 应明确报错，got %v", err)
	}
}

// 哪些状态算「钱到账」必须精确：多算 = 没付钱就发货，少算 = 付了钱不给单。
func TestNowpaymentIsPaid(t *testing.T) {
	paid := []string{NowpaymentStatusFinished, NowpaymentStatusConfirmed, NowpaymentStatusSending}
	for _, s := range paid {
		if !NowpaymentIsPaid(s) {
			t.Fatalf("%s 应判定为已到账", s)
		}
	}
	for _, s := range []string{
		NowpaymentStatusWaiting,
		NowpaymentStatusConfirming,
		NowpaymentStatusPartiallyPaid, // 少付必须人工处理
		NowpaymentStatusFailed,
		NowpaymentStatusExpired,
		NowpaymentStatusRefunded,
		"",
		"FINISHED", // 区分大小写
	} {
		if NowpaymentIsPaid(s) {
			t.Fatalf("%q 不能判定为已到账", s)
		}
	}
}

func TestPaymentOrderTitle(t *testing.T) {
	if got := paymentOrderTitle("  50% off 促销  ", 40); strings.Contains(got, "%") || got != "50 off 促销" {
		t.Fatalf("应去掉 %% 与首尾空白，got %q", got)
	}
	long := strings.Repeat("字", 60)
	if got := paymentOrderTitle(long, 40); len([]rune(got)) != 40 {
		t.Fatalf("应按字符数截断到 40，got %d", len([]rune(got)))
	}
	if got := paymentOrderTitle("   ", 40); got != "Order" {
		t.Fatalf("空标题应回落为 Order，got %q", got)
	}
}

// 金额换算差一分就是「付了钱没发货」或「少付也发货」，必须逐个卡住。
func TestXunhupayAmountFen(t *testing.T) {
	ok := map[string]int64{
		"19.90":  1990,
		"19.99":  1999, // 19.99*100 在浮点里是 1998.9999…，截断会少一分
		"0.19":   19,
		"199":    19900,
		" 12.5 ": 1250,
	}
	for in, want := range ok {
		got, err := XunhupayAmountFen(in)
		if err != nil || got != want {
			t.Fatalf("%q 应换算成 %d 分，got %d, err %v", in, want, got, err)
		}
	}
	for _, bad := range []string{"", "abc", "19.9元", "--1"} {
		if _, err := XunhupayAmountFen(bad); err == nil {
			t.Fatalf("%q 应报错", bad)
		}
	}
}

func TestXunhupayNotifySignVerify(t *testing.T) {
	params := map[string]string{
		"appid":          "201906100001",
		"trade_order_id": "202611041200000001",
		"total_fee":      "19.90",
		"status":         XunhupayStatusPaid,
		"time":           "1762300000",
		"nonce_str":      "abcdef123456",
	}
	params["hash"] = XunhupaySign(params, "secret_key_123")

	if err := XunhupayNotifySignVerify(params, "secret_key_123", "201906100001"); err != nil {
		t.Fatalf("正确回调应通过，got %v", err)
	}
	// 签名对但 appid 不是本账号：多站点共用一个网关时必须拒
	if err := XunhupayNotifySignVerify(params, "secret_key_123", "201906100002"); err == nil ||
		!strings.Contains(err.Error(), "appid") {
		t.Fatalf("appid 不匹配应拒绝，got %v", err)
	}
	// 账号没配 appid 时跳过归属校验（历史账号可能没填）
	if err := XunhupayNotifySignVerify(params, "secret_key_123", ""); err != nil {
		t.Fatalf("账号未配置 appid 时应跳过归属校验，got %v", err)
	}
	params["total_fee"] = "0.01"
	if err := XunhupayNotifySignVerify(params, "secret_key_123", "201906100001"); err == nil {
		t.Fatal("金额被改后签名应失败")
	}
}

func TestParseNowpaymentNotify(t *testing.T) {
	body := []byte(`{"payment_id":5077125051,"payment_status":"finished","pay_address":"0xabc","price_amount":19.9,"price_currency":"usd","pay_amount":0.00028571,"actually_paid":0.00028571,"pay_currency":"btc","order_id":"202611041200000001","purchase_id":"6084744717"}`)
	notify, err := ParseNowpaymentNotify(body)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	// payment_id 是数字，要能取成字符串（存 terrace_id）
	if string(notify.PaymentId) != "5077125051" {
		t.Fatalf("payment_id 应转成 5077125051，got %q", notify.PaymentId)
	}
	if string(notify.OrderId) != "202611041200000001" {
		t.Fatalf("order_id 解析错误，got %q", notify.OrderId)
	}
	if fen, err := notify.AmountFen(); err != nil || fen != 1990 {
		t.Fatalf("price_amount 应换算成 1990 分，got %d, err %v", fen, err)
	}

	// 反向对照：没有 order_id 的报文无法映射回本地支付单，必须当解析失败
	if _, err := ParseNowpaymentNotify([]byte(`{"payment_id":1,"payment_status":"finished"}`)); err == nil {
		t.Fatal("缺 order_id 应报错")
	}
	if _, err := ParseNowpaymentNotify(nil); err == nil {
		t.Fatal("空报文应报错")
	}
	bad := &NowpaymentNotify{PriceAmount: "not-a-number"}
	if _, err := bad.AmountFen(); err == nil {
		t.Fatal("非数字金额应报错")
	}
}
