package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"kandaoni.com/anqicms/config"
)

// 品牌验收：对**运行中的站点**做黑盒检查。
//
// 为什么不靠源码扫描：品牌替换发生在运行时出口（后端的 Tr、前端的
// brandizeMessages），源码里保留原始文案是设计的一部分。静态扫描必然误报，
// 只有抓真实响应才算数。
//
// 覆盖边界（必须说清楚，否则会给出虚假的安心）：
//   - 覆盖：服务端渲染的页面（首页、sitemap、robots）、后台 HTML 外壳与响应头；
//   - 不覆盖：后台 SPA 里由 JS 在浏览器端替换的文案——那需要真实执行 JS。
//     前端目前没有测试框架，这部分只能人工登录后台核一遍，别以为本工具通过
//     就等于后台没问题。
type verifyTarget struct {
	path string
	// optional 为 true 时 4xx 不算失败（站点可能没开 sitemap / robots）。
	optional bool
	// needBrand 为 true 时要求页面里出现本品牌名（用来抓「替换过头」或空品牌）。
	needBrand bool
}

var verifyTargets = []verifyTarget{
	{path: "/", needBrand: false},
	{path: "/system/", needBrand: true},
	{path: "/robots.txt", optional: true},
	{path: "/sitemap.xml", optional: true},
}

type leak struct {
	token string
	where string
}

func runVerify(baseURL string, brand config.OEMBrand) int {
	base := strings.TrimSuffix(baseURL, "/")
	if _, err := url.Parse(base); err != nil {
		fatal("地址无法解析：%v", err)
	}

	client := newVerifyClient(strings.Contains(base, "//127.0.0.1") || strings.Contains(base, "//localhost"))

	forbidden, allowed := forbiddenTokens(brand, otherBrands(brand.ID))
	fmt.Printf("检查 %s\n当前品牌 %s（%s）\n", base, brand.Name, brand.ID)
	if len(forbidden) == 0 {
		fmt.Printf("没有其它品牌登记，只能做存在性检查（加了 OEM 品牌后才会检查泄漏）\n")
	}

	failed := 0
	for _, target := range verifyTargets {
		code, body, headers, finalURL, err := fetch(client, base+target.path)
		if err != nil {
			fmt.Printf("  %-16s 请求失败：%v\n", target.path, err)
			failed++
			continue
		}
		if target.optional && code >= 400 {
			fmt.Printf("  %-16s %d（可选页面，跳过）\n", target.path, code)
			continue
		}
		if code != http.StatusOK {
			fmt.Printf("  %-16s %d 期望 200\n", target.path, code)
			failed++
			continue
		}

		leaks := scanBody(body, forbidden, allowed)
		for _, h := range scanHeaders(headers, forbidden, allowed) {
			leaks = append(leaks, h)
		}

		note := ""
		if finalURL != base+target.path {
			note = " → " + strings.TrimPrefix(finalURL, base)
		}
		if !target.needBrand {
			fmt.Printf("  %-16s 200 %6d 字节  %s%s\n", target.path, len(body), leakSummary(leaks), note)
		} else if !containsAny(body, brandTokens(brand)) {
			fmt.Printf("  %-16s 200 %6d 字节  缺少本品牌标识  %s\n", target.path, len(body), note)
			if other := detectBrand(body, otherBrands(brand.ID)); other != "" {
				// 最常见的原因不是泄漏，而是 -oem 传错了：检查者默认用官方品牌。
				fmt.Printf("      这个站点看起来是 %q，请加 -oem %s 重新检查\n", other, other)
			}
			failed++
		} else {
			fmt.Printf("  %-16s 200 %6d 字节  %s%s\n", target.path, len(body), leakSummary(leaks), note)
		}

		for _, l := range leaks {
			fmt.Printf("      %s：%s\n", l.token, l.where)
		}
		if len(leaks) > 0 {
			failed++
		}
	}

	if failed > 0 {
		fmt.Printf("\n有 %d 项未通过。若是文章正文里本来就在讨论官方产品，属于预期内容，人工确认即可。\n", failed)
		return 1
	}
	fmt.Println("\n通过：这些页面上看不到其它品牌的痕迹。")
	return 0
}

// newVerifyClient 对本地站点禁用代理：开发者机器上常开着全局代理，
// 走代理访问 127.0.0.1 会拿到 502，看起来像站点挂了。
func newVerifyClient(isLocal bool) *http.Client {
	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	if isLocal {
		transport.Proxy = nil
	}
	return &http.Client{Transport: transport, Timeout: 20 * time.Second}
}

func fetch(client *http.Client, target string) (int, string, http.Header, string, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return 0, "", nil, "", err
	}
	// 部分站点按 UA 做拦截，这里用常见 UA，避免把 403 当成品牌问题。
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; oemgen-verify)")
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", nil, "", err
	}
	defer resp.Body.Close()
	// 页面通常几十 KB，限制上限防止误抓大附件把内存打满。
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, "", resp.Header, resp.Request.URL.String(), err
	}
	return resp.StatusCode, string(body), resp.Header, resp.Request.URL.String(), nil
}

// otherBrands 返回除当前品牌之外所有已登记的品牌。
//
// 单独抽出来是为了可测：泄漏检测的正确性没法靠「官方站点跑一遍通过」证明
// （那时禁词集合是空的），只能用构造出来的两个品牌做反向对照。
func otherBrands(selfID string) []config.OEMBrand {
	var out []config.OEMBrand
	for _, id := range config.OEMIDs() {
		if id == selfID {
			continue
		}
		if b, ok := config.LookupOEM(id); ok {
			out = append(out, b)
		}
	}
	return out
}

// forbiddenTokens 返回「绝不该出现在本品牌站点上」的标识，以及需要放行的
// 子串。
//
// allowed 的存在理由：官方品牌叫 AnQiCMS，若某个 OEM 恰好叫 CMS，把所有
// 其它品牌的标识都当禁词会误伤。凡是本品牌标识的子串，都不算泄漏。
func forbiddenTokens(brand config.OEMBrand, others []config.OEMBrand) (forbidden, allowed []string) {
	allowed = lowerAll(brandTokens(brand))
	seen := map[string]bool{}
	for _, other := range others {
		if other.ID == brand.ID {
			continue
		}
		for _, token := range brandTokens(other) {
			low := strings.ToLower(token)
			// 太短的标识（如两个字母的缩写）满屏都是，检查没有意义。
			if len([]rune(low)) < 3 || seen[low] {
				continue
			}
			if isSubstringOfAny(low, allowed) {
				continue
			}
			seen[low] = true
			forbidden = append(forbidden, low)
		}
	}
	sort.Strings(forbidden)
	return forbidden, allowed
}

// brandTokens 提取一个品牌对外可识别的字面量。
func brandTokens(b config.OEMBrand) []string {
	out := []string{b.Name, b.NameCN, b.Vendor, hostOfURL(b.Website), hostOfURL(b.AuthBase)}
	tokens := make([]string, 0, len(out))
	for _, t := range out {
		if t = strings.TrimSpace(t); t != "" {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

func hostOfURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.TrimPrefix(u.Host, "www.")
}

func scanBody(body string, forbidden, allowed []string) []leak {
	if len(forbidden) == 0 {
		return nil
	}
	low := strings.ToLower(body)
	var leaks []leak
	reported := map[string]bool{}
	for _, token := range forbidden {
		idx := strings.Index(low, token)
		if idx < 0 {
			continue
		}
		if isSubstringOfAny(token, allowed) || reported[token] {
			continue
		}
		reported[token] = true
		leaks = append(leaks, leak{token: token, where: excerpt(body, idx)})
	}
	return leaks
}

func scanHeaders(headers http.Header, forbidden, allowed []string) []leak {
	var leaks []leak
	for name, values := range headers {
		joined := strings.ToLower(name + ": " + strings.Join(values, ", "))
		for _, token := range forbidden {
			if isSubstringOfAny(token, allowed) {
				continue
			}
			if strings.Contains(joined, token) {
				leaks = append(leaks, leak{token: token, where: "响应头 " + name})
			}
		}
	}
	return leaks
}

// excerpt 截取命中位置附近的片段，便于人工定位是不是正常内容。
func excerpt(body string, idx int) string {
	start := idx - 60
	if start < 0 {
		start = 0
	}
	end := idx + 80
	if end > len(body) {
		end = len(body)
	}
	snippet := strings.Join(strings.Fields(body[start:end]), " ")
	return "…" + snippet + "…"
}

func leakSummary(leaks []leak) string {
	if len(leaks) == 0 {
		return "ok"
	}
	return fmt.Sprintf("%d 处疑似泄漏", len(leaks))
}

func containsAny(body string, tokens []string) bool {
	low := strings.ToLower(body)
	for _, t := range tokens {
		if strings.Contains(low, strings.ToLower(t)) {
			return true
		}
	}
	return false
}

// detectBrand 在页面里找其它品牌的标识，用于提示「-oem 传错了」。
func detectBrand(body string, others []config.OEMBrand) string {
	low := strings.ToLower(body)
	for _, b := range others {
		for _, token := range brandTokens(b) {
			// 只认名字类标识；域名在页面里出现可能只是外链。
			if token != b.Name && token != b.NameCN {
				continue
			}
			if len([]rune(token)) >= 3 && strings.Contains(low, strings.ToLower(token)) {
				return b.ID
			}
		}
	}
	return ""
}

func lowerAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.ToLower(s)
	}
	return out
}

func isSubstringOfAny(s string, list []string) bool {
	for _, item := range list {
		if strings.Contains(item, s) {
			return true
		}
	}
	return false
}
