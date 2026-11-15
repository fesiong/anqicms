package provider

import (
	"errors"
	"log"
	"math/rand"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// 短信落库事务的有界重试（缺陷：8 路不同号码并发发码时，sms_codes 的
// 「按号码范围 Delete + Create」在空表/冷表上互相持有 gap 锁与插入意向锁，
// 实测大量 Error 1213 Deadlock，前端只看到「短信发送失败」且无法自愈）。
//
// 只重试数据库层的瞬时并发冲突：MySQL 1213（deadlock）与 1205（lock wait timeout）。
// 业务错误（校验失败、ErrSmsPhoneLimited、*sms.WaitError 等）必须原样立即返回：
// 尤其 WaitError 携带 Seconds，被包成通用错误后前端就拿不到「还需 N 秒」。
const smsTxMaxAttempts = 3

// smsTxRetryDelays 是第 1、2 次重试前的退避基数（20ms / 60ms），实际还会加抖动。
var smsTxRetryDelays = [...]time.Duration{20 * time.Millisecond, 60 * time.Millisecond}

// smsIsRetryableTxError 判断 err 是否是可以靠「整段事务重跑」消解的并发冲突。
// 用 *mysql.MySQLError.Number 判定，绝不用字符串匹配（错误文本会随驱动/语言变化）。
//
// 1213 deadlock、1205 lock wait timeout 是显式死锁/锁等待；1020
// "Record has changed since last read" 是实测中同一段「范围 Delete + 并发 Create」
// 触发的瞬时读一致性冲突（8 路并发实测偶发），整段重跑即可消解，不是业务错误。
func smsIsRetryableTxError(err error) bool {
	if err == nil {
		return false
	}
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) {
		switch myErr.Number {
		case 1213, 1205, 1020:
			return true
		}
	}
	return false
}

// smsTxBackoff 在退避基数上叠加 0~20ms 随机抖动，避免并发请求在同一时刻再次撞车（惊群）。
func smsTxBackoff(attempt int) time.Duration {
	base := smsTxRetryDelays[len(smsTxRetryDelays)-1]
	if attempt >= 1 && attempt <= len(smsTxRetryDelays) {
		base = smsTxRetryDelays[attempt-1]
	}
	return base + time.Duration(rand.Int63n(int64(20*time.Millisecond)))
}

// smsRunTxWithRetry 执行 fn；仅对 1213/1205 重试，最多 smsTxMaxAttempts 次。
// fn 必须是「整段事务」：重试会重新执行回调（发码场景因此会重新生成验证码）。
// tag 只用于日志定位（区号 + 号码后四位），绝不打印号码明文。
func smsRunTxWithRetry(db *gorm.DB, tag string, fn func(tx *gorm.DB) error) error {
	var lastErr error
	for attempt := 1; attempt <= smsTxMaxAttempts; attempt++ {
		lastErr = db.Transaction(fn)
		if lastErr == nil {
			if attempt > 1 {
				log.Printf("数据库事务重试成功（%s，第 %d 次尝试）", tag, attempt)
			}
			return nil
		}
		if !smsIsRetryableTxError(lastErr) {
			return lastErr
		}
		if attempt == smsTxMaxAttempts {
			log.Printf("数据库事务重试 %d 次仍失败（%s）：%v", attempt, tag, lastErr)
			return lastErr
		}
		log.Printf("数据库事务遇到可重试错误（%s，第 %d 次尝试），退避后重试：%v", tag, attempt, lastErr)
		time.Sleep(smsTxBackoff(attempt))
	}
	return lastErr
}

// smsPhoneLogTag 把号码脱敏成「区号+****+后四位」，只用于服务端日志。
func smsPhoneLogTag(country, local string) string {
	if len(local) > 4 {
		local = local[len(local)-4:]
	}
	return country + "****" + local
}
