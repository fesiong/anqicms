package provider

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/go-sql-driver/mysql"
	"kandaoni.com/anqicms/provider/sms"
)

// TestSmsSendConcurrentDifferentPhonesNoDeadlock 是 1213 死锁修复的回归用例：
// 8 个**不同号码**并发发码。修复前实测 1~2 个成功、其余 Error 1213 Deadlock；
// 修复后（事务有界重试）应 8/8 成功。
//
// 断言两层：
//  1. 不允许任何 goroutine 把 1213/1205 的驱动原文透出（前端会看到「短信发送失败」
//     以外的内部措辞，且用户完全无法自愈）；
//  2. 全部成功（本用例的号码、IP、场景都合法，任何业务错误都说明链路被并发污染）。
func TestSmsSendConcurrentDifferentPhonesNoDeadlock(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig(sms.SceneLogin))

	const n = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			phone := fmt.Sprintf("%s%03d", s8IntlPhonePrefix, i+1)
			_, err := w.sendSmsCodeWithSender(SmsSendInput{
				PhoneCountry: s8IntlCountry,
				Phone:        phone,
				Scene:        sms.SceneLogin,
				// 每个 goroutine 用不同 IP，隔离 IP 双窗口限额，避免把限额拒绝误判成死锁。
				Ip: fmt.Sprintf("198.51.100.%d", i+1),
			}, &fakeSmsSender{res: sms.SendResult{BizId: fmt.Sprintf("B-conc-%d", i)}})
			errs[i] = err
		}(i)
	}
	close(start)
	wg.Wait()

	success, failed := 0, 0
	for i, err := range errs {
		if err == nil {
			success++
			continue
		}
		failed++
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "deadlock") || strings.Contains(err.Error(), "1213") || strings.Contains(err.Error(), "1205") {
			t.Errorf("第 %d 路并发发码把数据库死锁/锁等待原文透出：%v", i, err)
		}
	}
	t.Logf("并发发码 8 路：成功 %d / 失败 %d", success, failed)
	if failed != 0 {
		t.Fatalf("并发发码应 8/8 成功（事务重试已内置），实际失败 %d 路：%v", failed, errs)
	}
}

// TestSmsTxRetryOnlyRetriesDeadlock 钉住重试判据的边界：
// 只有 MySQL 1213/1205/1020 可重试；业务错误（含 WaitError）必须原样返回，
// 不能被包成通用错误。
func TestSmsTxRetryOnlyRetriesDeadlock(t *testing.T) {
	if !smsIsRetryableTxError(&mysql.MySQLError{Number: 1213, Message: "Deadlock found"}) {
		t.Fatal("1213 deadlock 应判定为可重试")
	}
	if !smsIsRetryableTxError(&mysql.MySQLError{Number: 1205, Message: "Lock wait timeout"}) {
		t.Fatal("1205 lock wait timeout 应判定为可重试")
	}
	if !smsIsRetryableTxError(&mysql.MySQLError{Number: 1020, Message: "Record has changed since last read"}) {
		t.Fatal("1020 读一致性冲突应判定为可重试")
	}
	if smsIsRetryableTxError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}) {
		t.Fatal("1062 重复键是业务冲突，不得重试")
	}
	if smsIsRetryableTxError(&sms.WaitError{Seconds: 37}) {
		t.Fatal("WaitError 不得判定为可重试（否则前端拿不到还需 N 秒）")
	}
	if smsIsRetryableTxError(nil) {
		t.Fatal("nil 不得判定为可重试")
	}
}
