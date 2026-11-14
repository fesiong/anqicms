package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"kandaoni.com/anqicms/library"
)

// smsIpBudget 是某站点在给定限额配置下的 IP 限流预算。
// 必须是独立单例，不复用 middleware 的邮箱枚举探测预算：共用会让短信请求把
// 邮箱枚举探测的预算吃掉，两个功能互相污染（R62）。
type smsIpBudget struct {
	mu   sync.Mutex
	hour *library.RateLimiter
	day  *library.RateLimiter
}

// Allow 对小时 / 日两个窗口做原子的双阶段判定（R69）：
// 全程持 b.mu，先对两个窗口各做一次 Check（只判断、不记账），都通过才对两个窗口
// 各做一次 Allow（放行才记账）。任一窗口拒绝则直接返回 false、两个窗口都不记账，
// 消除「被一个窗口拒绝的请求仍消耗另一个窗口名额」的过度拒绝。
// 持 b.mu 消除 Check→Allow 之间的 TOCTOU（同一 siteId+限额配置共用同一实例）；
// 加锁顺序固定 b.mu → hour.mu → day.mu，避免死锁。
func (b *smsIpBudget) Allow(ip string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	hourOK := b.hour.Check(ip)
	dayOK := b.day.Check(ip)
	if !hourOK || !dayOK {
		return false
	}
	b.hour.Allow(ip)
	b.day.Allow(ip)
	return true
}

// smsIpBudgets 是独立的 sync.Map 单例（R62）。
var smsIpBudgets sync.Map

// smsIpBudgetFor 返回 (siteId, hourLimit, dayLimit) 对应的预算单例。
// 限额进 key：改配置会重置窗口 —— 旧限额下的计数不会被新限额复用。
func smsIpBudgetFor(siteId uint, hourLimit, dayLimit int) *smsIpBudget {
	key := fmt.Sprintf("%d:%d:%d", siteId, hourLimit, dayLimit)
	if v, ok := smsIpBudgets.Load(key); ok {
		return v.(*smsIpBudget)
	}
	v, _ := smsIpBudgets.LoadOrStore(key, &smsIpBudget{
		hour: library.NewRateLimiter(hourLimit, time.Hour),
		day:  library.NewRateLimiter(dayLimit, 24*time.Hour),
	})
	return v.(*smsIpBudget)
}

// smsCodeHash：HMAC-SHA256(TokenSecret, code) 的 hex 输出。
// 只存摘要不存验证码明文（规格 §4.2／§10.5）。
func (w *Website) smsCodeHash(code string) string {
	mac := hmac.New(sha256.New, []byte(w.TokenSecret))
	_, _ = mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

// smsErrSnippetMaxBytes 是 sms_logs.err_msg 片段上限（R61：≤200 字节合法 UTF-8）。
const smsErrSnippetMaxBytes = 200

// smsErrSnippet 把平台错误片段截到 200 字节并回到合法 UTF-8 边界。
// 与 provider/sms/httpbody.go 的 smsBodySnippet 同口径：ToValidUTF8 只丢弃非法字节
// （含被切开的半个 rune），其余现场原样保留，且字节数只减不增。
// 该片段只放平台元数据，绝不含验证码明文或渲染后的正文（R61）。
func smsErrSnippet(s string) string {
	b := []byte(s)
	if len(b) > smsErrSnippetMaxBytes {
		b = b[:smsErrSnippetMaxBytes]
	}
	return strings.ToValidUTF8(string(b), "")
}
