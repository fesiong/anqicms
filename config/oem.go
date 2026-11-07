package config

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
)

// DefaultOEMID 是官方版的标识，同时是编译期标识的默认值。
const DefaultOEMID = "anqicms"

// OEMID 是编译期固化的 OEM 标识。
//
// 默认 anqicms（官方版）；构建其他品牌时由构建脚本注入：
//
//	go build -ldflags "-X kandaoni.com/anqicms/config.OEMID=acme" ./main
//
// 为什么是编译期常量而不是运行时配置：品牌一旦能在运行时被改写，
// “这份安装包是给谁做的”就不再可判定，产物哈希也无法追责。
// 编译期注入让同一份源码产出 N 个互不影响的二进制，不必维护 N 个 fork。
var OEMID = DefaultOEMID

// oemFS 把品牌数据编译进二进制，而不是运行时从磁盘读。
//
// 这是刻意的：内网部署、首次启动、官网不可达时都必须能完整显示客户品牌。
// 任何“从远端拉品牌、拉不到退回默认”的设计，在最需要它的场景里会退回官方品牌，
// 等于把客户后台变成竞品广告位。
//
//go:embed oem/*/brand.json
var oemFS embed.FS

// OEMBrand 描述一个 OEM 品牌的全部可替换内容。
//
// 唯一真相源是 config/oem/<id>/brand.json；前端注入文件由 cmd/oemgen 从同一批
// JSON 生成，两侧不可能漂移。加品牌只需加一个目录，不改代码。
type OEMBrand struct {
	// ID 编译期标识，会出现在 ?oem= 查询参数、升级包文件名与二进制名中。
	// 只能是 [a-z0-9][a-z0-9_-]*，因为它要进 URL 与文件名。
	ID string `json:"id"`
	// Name 品牌主名（拉丁字母），用于 User-Agent、document.title 兜底。
	Name string `json:"name"`
	// NameCN 中文名，为空时回退 Name。
	NameCN string `json:"name_cn"`
	// Vendor 版权持有方，用于页脚版权行。
	Vendor string `json:"vendor"`
	// Website 官网地址。
	Website string `json:"website"`
	// Docs 帮助文档根地址，provider/designDoc.go 的链接由它拼出。
	Docs string `json:"docs"`
	// AuthBase 账号/授权/AI 网关根地址。
	AuthBase string `json:"auth_base"`
	// DownloadBase 升级包下载根地址（不含具体文件名）。
	DownloadBase string `json:"download_base"`
	// Binary 主程序文件名（升级包内），用于升级时识别该替换哪个文件。
	Binary string `json:"binary"`
	// Database 安装向导的默认数据库名。
	Database string `json:"database"`
	// SupportEmail 支持邮箱，可为空。
	SupportEmail string `json:"support_email"`
}

// Validate 校验字段完备性。
//
// 由 config/oem_test.go 在测试期调用：缺字段的品牌不允许进入构建产物 ——
// 宁可测试失败，也不要交付一个半截品牌（例如有名字没官网，客户点一下就 404）。
func (b OEMBrand) Validate() []string {
	var errs []string
	if !ValidOEMID(b.ID) {
		errs = append(errs, fmt.Sprintf("id 必须匹配 [a-z0-9][a-z0-9_-]*，当前为 %q", b.ID))
	}
	for _, f := range []struct {
		name string
		val  string
	}{
		{"name", b.Name},
		{"vendor", b.Vendor},
		{"website", b.Website},
		{"auth_base", b.AuthBase},
		{"download_base", b.DownloadBase},
		{"binary", b.Binary},
	} {
		if strings.TrimSpace(f.val) == "" {
			errs = append(errs, f.name+" 不能为空")
		}
	}
	for _, f := range []struct {
		name string
		val  string
	}{
		{"website", b.Website},
		{"docs", b.Docs},
		{"auth_base", b.AuthBase},
		{"download_base", b.DownloadBase},
	} {
		if f.val != "" && !strings.HasPrefix(f.val, "https://") && !strings.HasPrefix(f.val, "http://") {
			errs = append(errs, fmt.Sprintf("%s 必须是 http(s) 绝对地址，当前为 %q", f.name, f.val))
		}
	}
	if strings.ContainsAny(b.ID, " ") {
		errs = append(errs, "id 不能含空格")
	}
	return errs
}

// ValidOEMID 判断标识能否安全地进入 URL 与文件名。
func ValidOEMID(id string) bool {
	if id == "" {
		return false
	}
	for i, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case (r == '-' || r == '_') && i > 0:
		default:
			return false
		}
	}
	return true
}

var (
	oemBrands  = loadOEMBrands()
	oemLoadErr error
)

func loadOEMBrands() map[string]OEMBrand {
	brands := make(map[string]OEMBrand)
	paths, err := fs.Glob(oemFS, "oem/*/brand.json")
	if err != nil {
		oemLoadErr = err
		return brands
	}
	var problems []string
	for _, path := range paths {
		raw, err := oemFS.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		var b OEMBrand
		if err := json.Unmarshal(raw, &b); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		// 目录名与 id 必须一致：两者都会进 URL 和产物名，不一致时排查成本极高。
		dirName := strings.TrimSuffix(strings.TrimPrefix(path, "oem/"), "/brand.json")
		if b.ID != dirName {
			problems = append(problems, fmt.Sprintf("%s: id %q 与目录名 %q 不一致", path, b.ID, dirName))
		}
		brands[b.ID] = b
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		oemLoadErr = errors.New(strings.Join(problems, "; "))
	}
	return brands
}

// OEMLoadError 返回品牌数据加载期的问题，供测试与排障使用。
func OEMLoadError() error { return oemLoadErr }

// CurrentOEMID 返回去除首尾空白后的标识，为空时回退官方标识。
func CurrentOEMID() string {
	id := strings.TrimSpace(OEMID)
	if id == "" {
		return DefaultOEMID
	}
	return id
}

// IsOfficial 判断当前是否为官方版构建。用于关闭仅官方版才提供的入口
// （例如模板市场、官网账号绑定），避免 OEM 包露出官方品牌。
func IsOfficial() bool {
	return CurrentOEMID() == DefaultOEMID
}

// Brand 返回当前构建的品牌。
//
// 未知标识**不会**回落到官方品牌：打错一个字母就交付一个满屏官方 logo 的包
// 是最坏的结果，比功能不可用严重得多。此时返回中性品牌（展示名取标识本身）
// 并打印一次错误日志，让问题在启动日志里立刻可见。
func Brand() OEMBrand {
	id := CurrentOEMID()
	if b, ok := oemBrands[id]; ok {
		return b
	}
	warnUnknownOEM(id)
	return neutralBrand(id)
}

func warnUnknownOEM(id string) {
	log.Printf("OEM: 标识 %q 未登记，已启用中性品牌。请检查 config/oem/%s/brand.json 后重新构建", id, id)
}

// neutralBrand 是标识未登记时的兜底：展示名一律取标识本身，绝不出现官方品牌；
// 端点保留官方地址，使请求仍可发出（并带 ?oem=<id>），故障可从服务端日志定位，
// 而不是变成一堆无地址的静默失败。
func neutralBrand(id string) OEMBrand {
	return OEMBrand{
		ID:           id,
		Name:         id,
		NameCN:       id,
		Vendor:       id,
		Website:      "https://www.anqicms.com",
		Docs:         "https://docs.anqicms.com",
		AuthBase:     "https://auth.anqicms.com/auth",
		DownloadBase: "https://www.anqicms.com/downloads",
		Binary:       "anqicms",
		Database:     strings.NewReplacer("-", "_", ".", "_").Replace(id),
	}
}

// NameForLocale 按语言返回品牌名。lang 形如 zh-CN / zh-TW / en-US。
func (b OEMBrand) NameForLocale(lang string) string {
	if strings.HasPrefix(strings.ToLower(lang), "zh") && b.NameCN != "" {
		return b.NameCN
	}
	return b.Name
}

// SQLDatabaseName 返回可安全用于 CREATE DATABASE 的库名。
func (b OEMBrand) SQLDatabaseName() string {
	name := b.Database
	if name == "" {
		name = b.ID
	}
	var sb strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

// CopyrightLine 生成页脚版权行。
func (b OEMBrand) CopyrightLine(year string) string {
	vendor := b.Vendor
	if vendor == "" {
		vendor = b.Name
	}
	return "© " + year + " " + vendor
}

// OEMIDs 返回已登记的品牌标识（已排序），供构建脚本与前端生成使用。
func OEMIDs() []string {
	ids := make([]string, 0, len(oemBrands))
	for id := range oemBrands {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// LookupOEM 按标识查品牌表，未登记时 ok=false。
func LookupOEM(id string) (OEMBrand, bool) {
	b, ok := oemBrands[id]
	return b, ok
}

// 键用整个品牌结构而不是 id：id 相同但字段不同的两个值（测试里构造的品牌）
// 会共享替换器，导致「中文名缺失」这类分支被上一次的结果污染。
var brandReplacerCache sync.Map // OEMBrand -> *strings.Replacer

// replacer 返回该品牌的文案替换器（按品牌值缓存）。
func (b OEMBrand) replacer() *strings.Replacer {
	if v, ok := brandReplacerCache.Load(b); ok {
		return v.(*strings.Replacer)
	}
	cn := b.NameCN
	if cn == "" {
		// 中文名是可选的，缺省时用主名兜底；
		// 若替换成空串，界面上会剩下「欢迎使用」这种断句。
		cn = b.Name
	}
	r := strings.NewReplacer(
		"安企CMS", cn,
		"安企", cn,
		"AnQiCMS", b.Name,
		"AnqiCMS", b.Name,
		"anqicms", b.ID,
	)
	actual, _ := brandReplacerCache.LoadOrStore(b, r)
	return actual.(*strings.Replacer)
}

// Brandize 把文本里的官方品牌名替换成当前品牌。
//
// 官方版原样返回，逐字节不变，所以这个函数对主站行为零影响。
//
// 为什么在出口统一替换而不是改 locales：品牌文案分散在 9 个语言文件里
// （安装页标题、默认站点名、未登录提示、AI 助手自述），逐个改既会漏，
// 也会在升级覆盖 locales 时被冲掉。出口替换与凭据脱敏是同一套思路 ——
// 集中一处，上游新增文案自动继承。
func Brandize(s string) string {
	return BrandizeWith(Brand(), s)
}

// BrandizeWith 用指定品牌替换文案，供测试与按品牌渲染的场景使用。
//
// 顺序不能乱：先整段换地址，再换名字。反过来的话
// "https://www.anqicms.com" 会被名字规则拆成 "https://www.acme"，
// 变成一个打不开的地址；而 "auth.anqicms.com/auth" 会被规格化成
// 一个存在但错误的端点。
func BrandizeWith(b OEMBrand, s string) string {
	if s == "" || b.ID == DefaultOEMID {
		return s
	}
	host := hostOf(b.Website)
	if b.AuthBase != "" {
		s = strings.ReplaceAll(s, "https://auth.anqicms.com/auth", strings.TrimSuffix(b.AuthBase, "/"))
	}
	if b.Website != "" {
		s = strings.ReplaceAll(s, "https://www.anqicms.com", strings.TrimSuffix(b.Website, "/"))
	}
	// 其余子域（m. / admin. / 裸域）统一换成品牌站的主机名 ——
	// locales 里有几十处示例地址，逐个枚举补不全。
	if host != "" {
		s = strings.ReplaceAll(s, "anqicms.com", host)
	}
	return b.replacer().Replace(s)
}

// hostOf 从地址里取主机名，去掉 www. 前缀。
//
// 去前缀是必须的：locales 里既有 www.anqicms.com 也有 m./admin. 子域，
// 整段换掉 www. 之后剩下的子域要靠主机名替换，如果主机名是 www.acme.test，
// 就会拼出 admin.www.acme.test 这种不存在的地址。
func hostOf(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.TrimPrefix(u.Host, "www.")
}
