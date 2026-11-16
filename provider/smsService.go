package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
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
// ⚠️ 这里有一条容易写错的边界：**站点挂 CDN 后，TCP 对端只能是 CDN 回源节点**。
// HTTP 是 CDN 与源站之间的第二条连接，真实客户端地址不可能从 TCP 层拿到，
// 只能靠 CDN 附加的 X-Forwarded-For 传递。所以「用对端地址」在 CDN 下不是
// 「更安全」，而是「把全站用户算成同一个 IP」——限额会被少数人吃光，全站发不出码。
//
// 反过来「无条件信头」也不行：本项目默认直连公网，客户端自带一个 X-Real-IP
// 就能每发一次换一个「IP」，IP 双窗口限额直接失效，而短信是付费的。
//
// 这两个诉求无法从报文本身区分，所以判据只能来自配置：
//   - 对端落在 trusted_proxies 里（或对端是环回/私有地址 = 本机反代）→ 可信转发，
//     从 X-Forwarded-For **最右往左**剥离，取第一个既不在 trusted 也不是私有的地址
//     （最右是最后加的一跳，客户端往左边追加的伪造值影响不到它）；
//   - 对端是公网且不在 trusted → 直连，忽略一切代理头，直接用对端地址。
//
// 第二支在 CDN 未配置时会退化成「全站共用一个桶」，所以额外返回一个 suspicious
// 标记，由调用方打一条限频告警，让站长能发现「忘了配 trusted_proxies」。
//
// 站点由调用方传入而不是这里自己取：CurrentSite 在站点注册表为空时会走
// createDefaultWebsite → ctx.GetLocale()，i18n 未就绪就 panic。取个限额用的 IP
// 不该把发码链路绑到站点/i18n 的初始化顺序上。
//
// 不支持 PROXY protocol：iris 的 listener 不解析它，要用它得在前面再套一层。
func SmsClientIP(ctx iris.Context, site *Website) string {
	if ctx == nil || ctx.Request() == nil {
		return ""
	}
	var trusted []netip.Prefix
	if site != nil && site.PluginSms != nil {
		trusted = smsParseTrustedProxies(site.PluginSms.TrustedProxies)
	}
	ip, warn := smsResolveClientIP(
		ctx.Request().RemoteAddr,
		ctx.GetHeader("X-Forwarded-For"),
		ctx.GetHeader("X-Real-IP"),
		ctx.GetHeader("CF-Connecting-IP"),
		trusted,
	)
	if warn != smsIPWarnNone {
		var siteId uint
		if site != nil {
			siteId = site.Id
		}
		switch warn {
		case smsIPWarnUntrustedPeer:
			smsWarnProxyMisconfig(siteId, fmt.Sprintf(
				"连接来自公网地址 %s，但请求带了转发头。若该地址是 CDN/反代回源节点，"+
					"IP 限额会把全站用户算成同一个 IP（额度被少数人吃光后全站发不出码）；"+
					"请在短信设置 trusted_proxies 中填入回源网段。", ip))
		case smsIPWarnMultiHop:
			smsWarnProxyMisconfig(siteId, fmt.Sprintf(
				"X-Forwarded-For 里有多跳未登记的代理，当前按最右一跳 %s 计限额。"+
					"若最右一跳是 CDN 节点（典型：CDN → 本机 nginx → 源站，而 nginx 未做 realip 归一化），"+
					"请在 nginx 配 set_real_ip_from + real_ip_header 后重写 X-Forwarded-For，"+
					"或把 CDN 回源段填进短信设置 trusted_proxies。", ip))
		}
	}
	return ip
}

// smsIPWarn* 是 smsResolveClientIP 的诊断码：非空表示取值可能不准，调用方据此打日志。
//
// multi-hop 有两种成因，报文层面无法区分：① 链条里有未登记的代理（典型是
// 「CDN → 本机 nginx → 源站」而 nginx 没做 realip 归一化，最右那跳其实是 CDN 节点）；
// ② 客户端自己往 X-Forwarded-For 里塞了假地址。两者都值得让站长看一眼，故共用一条告警。
const (
	smsIPWarnNone          = ""
	smsIPWarnUntrustedPeer = "untrusted-peer" // 对端公网且未登记，却带了指向别处的转发头
	smsIPWarnMultiHop      = "multi-hop"      // XFF 里被取到的那跳左边还有公网地址
)

// smsResolveClientIP 是 SmsClientIP 的纯函数内核：不读全局、不打日志，便于表驱动测试。
func smsResolveClientIP(peerRaw, xff, xRealIP, cfIP string, trusted []netip.Prefix) (string, string) {
	peer := smsHostOnly(peerRaw)
	peerAddr, peerErr := netip.ParseAddr(peer)
	if peerErr == nil {
		peerAddr = peerAddr.Unmap()
	}
	peerTrusted := peerErr == nil &&
		(peerAddr.IsLoopback() || peerAddr.IsPrivate() || smsInPrefixes(peerAddr, trusted))

	if !peerTrusted {
		// 公网直连（或我们并不知道的 CDN 回源）：任何转发头都不可采信。
		if smsHasForeignForwardHeader(peer, xff, xRealIP, cfIP) {
			return peer, smsIPWarnUntrustedPeer
		}
		return peer, smsIPWarnNone
	}
	if ip, hasEarlier := smsFirstUntrustedXFF(xff, trusted); ip != "" {
		// 取到的那一跳左边还有别的公网地址：说明链条里至少有一个我们没登记的代理。
		// 典型形态是「CDN → 本机 nginx → 源站」而 nginx 没做 realip 归一化 ——
		// 此时最右那一跳是 CDN 回源节点，取它等于按节点计限额。
		if hasEarlier {
			return ip, smsIPWarnMultiHop
		}
		return ip, smsIPWarnNone
	}
	// 自建反代常见做法是只设单值头；nginx 的 proxy_set_header X-Real-IP $remote_addr
	// 会覆盖客户端自带值，所以在「对端已可信」的前提下可以采信。
	if ip := smsPublicHeaderAddr(xRealIP); ip != "" {
		return ip, smsIPWarnNone
	}
	if ip := smsPublicHeaderAddr(cfIP); ip != "" {
		return ip, smsIPWarnNone
	}
	return peer, smsIPWarnNone
}

// smsFirstUntrustedXFF 从 X-Forwarded-For 的最右往左剥离，返回第一个「可信链条之外」的
// 公网地址；hasEarlier 表示这一跳的左边**还有**别的公网地址（= 链条里有未登记的代理）。
// 跳过私有/环回/trusted 三类（它们都是中间的转发者）。
// 全部跳完仍没有结果时返回空串，交给调用方回落单值头。
//
// ⚠️ 依赖 CDN **追加**真实地址（主流 CDN 都这么做）。若某个 CDN 只透传客户端的值、
// 自己不追加，最右就成了客户端写的值——这种部署下 IP 限额不可能做对，只能靠号码维度。
func smsFirstUntrustedXFF(xff string, trusted []netip.Prefix) (string, bool) {
	if strings.TrimSpace(xff) == "" {
		return "", false
	}
	picked := ""
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.Trim(strings.TrimSpace(parts[i]), `"`))
		if err != nil {
			continue
		}
		addr = addr.Unmap()
		if addr.IsLoopback() || addr.IsPrivate() || smsInPrefixes(addr, trusted) {
			continue
		}
		if picked == "" {
			picked = addr.String()
			continue
		}
		return picked, true
	}
	return picked, false
}

// smsPublicHeaderAddr 取单值头里的公网地址（多值时取第一个可解析的公网项）。
func smsPublicHeaderAddr(v string) string {
	for _, part := range strings.Split(v, ",") {
		addr, err := netip.ParseAddr(strings.Trim(strings.TrimSpace(part), `"`))
		if err != nil {
			continue
		}
		addr = addr.Unmap()
		if addr.IsLoopback() || addr.IsPrivate() {
			continue
		}
		return addr.String()
	}
	return ""
}

// smsHasForeignForwardHeader 判断转发头里是否出现了与对端不同的公网地址。
// 只用于诊断（是否该提示站长配 trusted_proxies），不参与限额取值。
func smsHasForeignForwardHeader(peer, xff, xRealIP, cfIP string) bool {
	x, _ := smsFirstUntrustedXFF(xff, nil)
	for _, ip := range []string{
		x,
		smsPublicHeaderAddr(xRealIP),
		smsPublicHeaderAddr(cfIP),
	} {
		if ip != "" && ip != peer {
			return true
		}
	}
	return false
}

// smsParseTrustedProxies 把配置里的 IP / CIDR 解析成前缀列表，非法项静默跳过
// （保存路径已做校验；运行期不能因为一条脏配置就把发码打挂）。
func smsParseTrustedProxies(items []string) []netip.Prefix {
	if len(items) == 0 {
		return nil
	}
	out := make([]netip.Prefix, 0, len(items))
	for _, raw := range items {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.Contains(raw, "/") {
			if p, err := netip.ParsePrefix(raw); err == nil {
				out = append(out, p.Masked())
			}
			continue
		}
		if addr, err := netip.ParseAddr(raw); err == nil {
			out = append(out, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
		}
	}
	return out
}

func smsInPrefixes(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

var (
	smsProxyWarnMu   sync.Mutex
	smsProxyWarnLast = map[uint]time.Time{}
)

// smsWarnProxyMisconfig 每小时每站点最多一条。
// 两种形态都会走到这里：① 对端是公网但没登记 trusted_proxies（直连还是 CDN 分不清，
// 一律按直连处理）；② 对端可信但 XFF 里还有未登记的中间代理（典型是
// 「CDN → 本机 nginx → 源站」而 nginx 没做 realip 归一化）。两者都会让 IP 限额
// 算错桶，但站长只会在「莫名被限流」时才发现，所以必须主动提示。
func smsWarnProxyMisconfig(siteId uint, msg string) {
	smsProxyWarnMu.Lock()
	if last, ok := smsProxyWarnLast[siteId]; ok && time.Since(last) < time.Hour {
		smsProxyWarnMu.Unlock()
		return
	}
	smsProxyWarnLast[siteId] = time.Now()
	smsProxyWarnMu.Unlock()
	log.Printf("短信 IP 限额：站点 %d %s", siteId, msg)
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
