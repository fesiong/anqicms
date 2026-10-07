package intent

import "strings"

// domain.go 提供「命名空间 → 能力域」的单一真相源映射。
//
// 背景：后台 394 个端点里 /plugin/* 一个菜单组就占了 229 个（58%），
// 若按菜单分组直接当域用，白名单（ApiExposure 的 allow_ns / deny_ns）无从下手。
// 因此这里按语义把每个 ns 归到一个细分域，使策略可以按域裁剪。
//
// 划分依据与完整映射表见 doc/intent-domain-split-proposal.md 第四节。
// 合计划分可校验：各域端点数相加 == 394。

// domainByNS 是一级命名空间的归属（不含 plugin，plugin 见 domainByPlugin）。
var domainByNS = map[string]Domain{
	"archive":    DomainContent,
	"module":     DomainContent,
	"category":   DomainContent,
	"attachment": DomainMedia,
	"setting":    DomainSystem,
	"statistic":  DomainTraffic,
	"siteinfo":   DomainTraffic,
	"collector":  DomainContentOps,
	"aigenerate": DomainContentOps,
	"design":     DomainDesign,
	"admin":      DomainAccount,
	"password":   DomainAccount,
	"login":      DomainAccount,
	"captcha":    DomainAccount,
	"website":    DomainSiteOps,
	"version":    DomainSiteOps,
	"anqi":       DomainSiteOps,
}

// domainByPlugin 是 /plugin/<二级模块> 的归属，覆盖全部 223 个 plugin 端点。
var domainByPlugin = map[string]Domain{
	// 站点运维（高危：备份恢复、静态缓存、全站替换、索引重建）
	"htmlcache": DomainSiteOps,
	"backup":    DomainSiteOps,
	"fulltext":  DomainSiteOps,
	"replace":   DomainSiteOps,
	// 交易
	"user":       DomainCommerce,
	"order":      DomainCommerce,
	"pay":        DomainCommerce,
	"retailer":   DomainCommerce,
	"withdraw":   DomainCommerce,
	"commission": DomainCommerce,
	"finance":    DomainCommerce,
	// 触达渠道
	"wechat":     DomainChannel,
	"weapp":      DomainChannel,
	"sendmail":   DomainChannel,
	"subscriber": DomainChannel,
	// 推广与 SEO
	"anchor":   DomainSeo,
	"keyword":  DomainSeo,
	"redirect": DomainSeo,
	"llms":     DomainSeo,
	"push":     DomainSeo,
	"sitemap":  DomainSeo,
	"robots":   DomainSeo,
	"jsonld":   DomainSeo,
	// 内容生产
	"material":   DomainContentOps,
	"translate":  DomainContentOps,
	"transfer":   DomainContentOps,
	"titleimage": DomainContentOps,
	"watermark":  DomainContentOps,
	"import":     DomainContentOps,
	"timefactor": DomainContentOps,
	// 互动
	"guestbook": DomainInteraction,
	"comment":   DomainInteraction,
	// 内容（plugin 下与文档/分类同族的模块）
	"tag":   DomainContent,
	"place": DomainContent,
	// 站点结构
	"link": DomainStructure,
	// 系统配置
	"multilang":    DomainSystem,
	"limiter":      DomainSystem,
	"storage":      DomainSystem,
	"fileupload":   DomainSystem,
	"rewrite":      DomainSystem,
	"interference": DomainSystem,
	"akismet":      DomainSystem,
	"google":       DomainSystem,
}

// nsSubdomain 是「同一 ns 内需要按二级路径区分」的例外。
//
// 目前只有一例：setting 下绝大多数是系统配置，但 /setting/nav（导航设置）
// 语义上属于站点结构，必须拆出去，否则 structure 域只剩友链 4 个端点。
var nsSubdomain = map[string]map[string]Domain{
	"setting": {"nav": DomainStructure},
}

// DomainOfPath 按后台端点路径判定能力域。
//
// path 既可以是完整路径（/system/api/plugin/push/list），
// 也可以是去掉 /system/api 前缀的相对路径（plugin/push/list）。
// 未登记到映射表时返回 DomainUnknown —— 这不是可容忍的降级，而是需要补登记的缺口。
func DomainOfPath(path string) Domain {
	segs := splitAdminPath(path)
	if len(segs) == 0 {
		return DomainUnknown
	}
	ns := segs[0]

	// 先判 ns 内的二级例外（如 setting/nav）
	if sub, ok := nsSubdomain[ns]; ok && len(segs) > 1 {
		if d, ok := sub[segs[1]]; ok {
			return d
		}
	}
	// plugin 必须看二级模块，一级 plugin 本身无语义
	if ns == "plugin" {
		if len(segs) < 2 {
			return DomainUnknown
		}
		if d, ok := domainByPlugin[segs[1]]; ok {
			return d
		}
		return DomainUnknown
	}
	if d, ok := domainByNS[ns]; ok {
		return d
	}
	return DomainUnknown
}

// splitAdminPath 把端点路径切成语义段，去掉 /system/api 前缀。
//
// 后台路由的 Party 结构是 system.Party("/system") → .Party("/api")，
// 因此所有后台端点都以 /system/api 开头；这里只剥这一层固定前缀，
// 不做其它猜测，避免把真实的 ns 吃掉。
func splitAdminPath(path string) []string {
	p := strings.TrimSpace(path)
	p = strings.TrimPrefix(p, "/system/api")
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return nil
	}
	var segs []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}
