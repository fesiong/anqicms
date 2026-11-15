package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
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

// SmsBusinessMessage 判定 err.Error() 是否是一枚真实存在的语言键，命中则返回译文。
//
// 前后台短信错误出口必须共用这一判据（字典即白名单），否则两边会漂移：
// 前台曾用 TplTr、后台曾用 Tr，而 Tr = Brandize(tr(x)) —— OEM 构建下任何含
// 官方品牌名/域名的普通错误文本都会被 Brandize 改写，于是「译文 != 原文」
// 被误判成语言键，把内部原文送出门（本缺陷的靶心）。
//
// 这里固定用 TplTr（不做品牌替换），并先对含 % 的文本短路：TplTr 的兜底是
// fmt.Sprintf，含 % 的文本会被改写成 %!c(MISSING) 之类，改写后必然 != 原文，
// 不加短路就会被误判成键。
func SmsBusinessMessage(site *Website, err error) (string, bool) {
	if site == nil || err == nil {
		return "", false
	}
	raw := err.Error()
	if strings.Contains(raw, "%") {
		return "", false
	}
	// 方法值：TplTr 内部兜底是 fmt.Sprintf，被 Go 1.24+ printf vet 识别为
	// printf wrapper；对「无可变参数的非常量格式串」直接调用会报
	// non-constant format string。经方法值调用可规避，翻译语义不变。
	translate := site.TplTr
	translated := translate(raw)
	if translated == raw {
		return "", false
	}
	return translated, true
}

// SmsClientIP 取短信 IP 限额使用的客户端地址。
//
// 不能无条件信任 ctx.RemoteAddr()：bootstrap.go 启用了
// iris.WithRemoteAddrHeader("X-Real-IP", "X-Forwarded-For", "CF-Connecting-IP")，
// 而本项目默认是「站点直连公网」。直连时客户端可以自己带一个 X-Real-IP，
// 每发一次换一个「IP」，IP 双窗口限额直接失效，只剩号码日限——而短信是付费的。
//
// 也不能无条件用 TCP 对端：本机/内网反代后面，对端永远是 127.0.0.1 或 10.x，
// 所有用户会被算成同一个 IP，限额变成全站共享。
//
// 判据：对端是公网地址 → 直连场景，忽略一切代理头；对端是环回/私有地址 →
// 说明前面有本机或内网反代，才采信 iris 已按代理头解析出的地址。
func SmsClientIP(ctx iris.Context) string {
	if ctx == nil || ctx.Request() == nil {
		return ""
	}
	peer := smsHostOnly(ctx.Request().RemoteAddr)
	if addr, err := netip.ParseAddr(peer); err == nil {
		addr = addr.Unmap()
		if !addr.IsLoopback() && !addr.IsPrivate() {
			return addr.String()
		}
	}
	if resolved := smsHostOnly(ctx.RemoteAddr()); resolved != "" {
		return resolved
	}
	return peer
}

// smsHostOnly 去掉地址里的端口并统一 IPv6 表示。
// 同时接受 "1.2.3.4:80"、"[2001:db8::1]:443"、"1.2.3.4"、"2001:db8::1"，
// 任何形态都不 panic（解析失败原样返回，交给上层按「非公网」处理）。
func smsHostOnly(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return ""
	}
	if addrPort, err := netip.ParseAddrPort(remoteAddr); err == nil {
		return addrPort.Addr().Unmap().String()
	}
	if addr, err := netip.ParseAddr(remoteAddr); err == nil {
		return addr.Unmap().String()
	}
	// 裸 IPv6 带方括号但没有端口（"[::1]"）：去掉方括号再试一次。
	if strings.HasPrefix(remoteAddr, "[") && strings.HasSuffix(remoteAddr, "]") {
		if addr, err := netip.ParseAddr(remoteAddr[1 : len(remoteAddr)-1]); err == nil {
			return addr.Unmap().String()
		}
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

// GetSmsLogList 分页查询短信发送流水（任务 13 / R115）。
// 恒带 site_id = w.Id 基底（多站点隔离）：六个筛选都通过 filter 叠加在这个基底上，
// 任何调用方都无法绕过站点隔离。按 created_time DESC 排序命中 idx_site_created，分页。
// 返回 []*model.SmsLog：SmsLog 本身无码字段，也不含渲染后正文；端到端 JSON 复核见
// controller/manageController/pluginSmsOps_test.go。
func (w *Website) GetSmsLogList(filter func(*gorm.DB) *gorm.DB, page, pageSize int) ([]*model.SmsLog, int64) {
	var logs []*model.SmsLog
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.SmsLog{}).Where("site_id = ?", w.Id)
	if filter != nil {
		tx = filter(tx)
	}
	tx.Count(&total).Order("created_time DESC").Limit(pageSize).Offset(offset).Find(&logs)
	return logs, total
}
