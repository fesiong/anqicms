package middleware

import (
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
)

// 账号相关端点的防探测预算。三个端点共用一份（按 IP 计数），
// 故意不拆成三个独立额度：它们都能判断「某个邮箱是否已注册」，
// 拆分只会让攻击者在 exist / 发信 / 注册之间轮换，各自都碰不到上限。
const (
	accountProbeLimit  = 20
	accountProbeWindow = 10 * time.Minute
)

// accountProbeLimiter 为包级单例：限额按 IP 计，跨站点共享即可，
// 也让多站点部署时同一 IP 无法靠切换站点重置额度。
var accountProbeLimiter = library.NewRateLimiter(accountProbeLimit, accountProbeWindow)

// AccountProbeGuard 按客户端 IP 限制「会泄漏邮箱注册状态 / 会触发发信」的端点：
//
//	GET  /api/verify/email?state=exist   邮箱是否注册
//	POST /api/verify/email               不存在时报 UserDoesNotExist，且会发信
//	POST /api/register                   重复邮箱报 TheEmailHasBeenRegistered
//
// 这三个端点任何一个放开，邮箱枚举就成立；只删返回值里的 id/email 只是减小泄漏面，
// 所以必须三处一起挂。
//
// IP 取自 ctx.RemoteAddr()：启动时配置了 iris.WithRemoteAddrHeader("X-Real-IP",
// "X-Forwarded-For", "CF-Connecting-IP")，反代后得到的是真实客户端 IP。
// 注意 iris 在解析 XFF 时会跳过内网地址、取最右侧的公网地址，因此客户端自行塞进
// 请求头的伪造值不会被采信（左侧条目被忽略）。
func AccountProbeGuard(ctx iris.Context) {
	ip := ctx.RemoteAddr()
	if ip == "" {
		// 取不到 IP 时按同一个 key 计数（宁可共享额度，也不放开限制）。
		ip = "unknown"
	}
	if accountProbeLimiter.Allow(ip) {
		ctx.Next()
		return
	}

	ctx.StatusCode(iris.StatusTooManyRequests)
	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  provider.CurrentSite(ctx).TplTr("TooManyRequests"),
	})
}
