package provider

// 站点级放行判定（启动期）。
//
// 为什么需要它：一次升级要处理 106 个站点，串行迁移要几十分钟。过去的闸门是
// **进程级**的（provider.StartupReady），于是处理到第 3 个站点时，前 2 个已经
// 迁移完、能正常服务的站点也被挡成 503 —— 全站停机，时长等于所有站点之和。
//
// 现在的口径：
//   - 正在迁移的站点（未注册）→ 503；
//   - 还没轮到 / 迁移失败的站点（未注册）→ 503；
//   - 已迁移完成并注册的站点 → 正常访问。
//
// 判据是「站点在注册表里、Initialed=true、且打了可服务标记」。InitWebsite 的顺序是
// 迁移 → InitSetting → InitModelData → 注册 → 建 rewrite/limiter/cache → 打标记，
// 所以"可服务"成立就意味着库表已迁完、System 已加载、对外依赖都建好了。
// 不要改成"看站点在不在 websites 表里"：那张表在迁移之前就存在。

import (
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/library"
)

// servableSites 是「初始化彻底完成、可以对外服务」的站点集合。
//
// 不能只看注册表：InitWebsite 在 `websites.Set(&w)` 之后还要建 rewrite/limiter/
// cache，而 GetRewritePattern 又反过来要求"先 Set 才能回写"，所以 Set 必须早、
// 可服务必须晚。两者混在一起就会出现"站点已在表里、Cache 还是 nil"的窗口 ——
// 过去这段被全局闸门挡着，现在逐站点放行，挡不住了。
//
// 用 sync.Map 而不是 Website 上的字段：Website 会被 cloneWithContext 按值拷贝，
// 带锁/原子字段会触发 copylocks，且拷贝出去的副本写回不到原对象。
var servableSites sync.Map // map[uint]struct{}

// markSiteServable 只在 InitWebsite 彻底结束时调用。
func markSiteServable(siteID uint) {
	if siteID == 0 {
		return
	}
	servableSites.Store(siteID, struct{}{})
}

// unmarkSiteServable 在重新初始化开始、以及站点被删除时调用。
func unmarkSiteServable(siteID uint) {
	if siteID == 0 {
		return
	}
	servableSites.Delete(siteID)
}

func siteServable(siteID uint) bool {
	_, ok := servableSites.Load(siteID)
	return ok
}

// RequestServable 判断"整体还没就绪时，这个请求能不能放行"。
//
// 只给 middleware.StartupGate 用。判定顺序有讲究：
//  1. Failed 阶段一律不放行 —— panic 之后注册表里可能是"部分就绪、部分半初始化"，
//     这时候按站点放行等于赌某个站点没被 panic 影响，赌输就是线上读到半迁移的表；
//  2. 请求显式声明的站点（后台在同域名下切站）优先于域名 —— 只看域名会得出
//     "站点 1 已就绪"而放行一个要操作未迁移站点的请求，那种请求会落回站点 1，
//     把写操作打到错的库上；
//  3. 其余按域名反查归属站点，该站点已初始化完就放行。
func RequestServable(ctx iris.Context) bool {
	if StartPhase() == StartupPhaseFailed {
		return false
	}
	if target, declared := RequestDeclaredSite(ctx); declared {
		return SiteReady(target)
	}
	return RequestHostReady(ctx)
}

// SiteReady 判断某个站点是否已经完成初始化（含库表迁移），可用于对外服务。
func SiteReady(siteID uint) bool {
	if siteID == 0 {
		return false
	}
	w, ok := websites.Get(siteID)
	if !ok || w == nil {
		return false
	}
	// System 判空不能省：连库失败/未 InitSetting 的站点会以 System == nil
	// 留在注册表里（见 website.go 里 InitWebsite 的注释）。
	return w.Initialed && w.System != nil && siteServable(siteID)
}

// RequestHostReady 判断「按域名归属的那个站点」是否已就绪。
func RequestHostReady(ctx iris.Context) bool {
	if ctx == nil {
		return false
	}
	return SiteReadyForHost(library.GetHost(ctx), ctx.RequestPath(false))
}

// SiteReadyForHost 用请求的 host（+ 二级目录前缀）反查站点，判断它是否已就绪。
//
// 只认已注册的站点。host 匹配不到任何已就绪站点时返回 false ——
// 这是刻意的 fail closed：匹配不到意味着它可能是"还没迁移的站点"，
// 一旦放行就会落到 CurrentSite 的站点 1 兜底上，拿 A 站的内容回应 B 站的域名。
func SiteReadyForHost(host, path string) bool {
	if host == "" {
		return false
	}
	for _, w := range websites.Values() {
		if w == nil || !w.Initialed || w.System == nil || !siteServable(w.Id) {
			continue
		}
		if siteServesHost(w, host, path) {
			return true
		}
	}
	return false
}

// siteServesHost 复刻 matchByURIAndHost 的域名/目录判定，但不做 ctx 克隆，
// 也不触发多语言切换 —— 闸门阶段只需要一个是/否。
func siteServesHost(w *Website, host, path string) bool {
	for _, raw := range []string{w.System.BaseUrl, w.System.FrontUrl, w.System.MobileUrl, w.System.AdminUrl} {
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || !isHostMatch(parsed, host) {
			continue
		}
		if sitePathMatches(parsed, path) {
			return true
		}
	}
	// 多语言按域名分站时，语言域名不在 System.* 上，而在 MultiLanguage.SubSites，
	// 且 SubSites 要等 InitWebsites 收尾才填。窗口期内它可能是空的 →
	// 语言域名会 503 到全站就绪为止。这个方向的误差可以接受：
	// 放行一个归属不明的域名，比让某个语言站晚几分钟可访问危险得多。
	if w.MultiLanguage != nil && w.MultiLanguage.Open {
		for _, sub := range w.MultiLanguage.SubSites {
			if sub.BaseUrl == "" {
				continue
			}
			parsed, err := url.Parse(sub.BaseUrl)
			if err == nil && isHostMatch(parsed, host) {
				return true
			}
		}
	}
	return false
}

// sitePathMatches 处理同域名下的二级目录站点：BaseUrl 带路径时，
// 只放行该前缀下的请求，否则会拿 A 站的就绪状态放行 B 站的请求。
func sitePathMatches(parsed *url.URL, path string) bool {
	uri := parsed.RequestURI()
	if uri == "" || uri == "/" {
		return true
	}
	if path == "" {
		path = "/"
	}
	return strings.HasPrefix(path, uri)
}

// RequestDeclaredSite 读请求里显式声明的目标站点。
//
// 后台可以在同一个域名下切换站点（前端带 Site-Id / X-Site-Id / Sub-Site-Id），
// 这类请求的真实目标由 ID 决定、和域名无关。只看域名会得出"站点 1 已就绪"
// 而放行一个要操作未迁移站点的请求 —— 那种请求会落回站点 1，把写操作打到错的库上。
func RequestDeclaredSite(ctx iris.Context) (uint, bool) {
	if ctx == nil {
		return 0, false
	}
	for _, key := range []string{"Site-Id", "X-Site-Id", "Sub-Site-Id"} {
		raw := strings.TrimSpace(ctx.GetHeader(key))
		if raw == "" {
			continue
		}
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			continue
		}
		return uint(id), true
	}
	return 0, false
}
