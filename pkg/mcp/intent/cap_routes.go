package intent

import (
	"context"
	"fmt"
	"strings"
)

// cap_routes.go 是「底层能力 → 后台 REST 端点」的唯一映射表。
//
// 背景：意图层早先通过 switchCompose 把 action 路由到底层能力（cap）名，
// 而 cap 的实现是 provider/aiTools.go 里一批与后端平行、会逐渐漂移的 bespoke
// handler（典型如已废弃的 archive_list 全文检索分支）。绝大多数 cap 其实都有
// 对应的后台端点，没必要再维护第二套实现。
//
// 本表把 cap 名直接解析成真实端点，由 api_invoke 统一执行。这样：
//  1. 端点真相源只剩 route/manage.go 一份，AI 侧不再有平行实现；
//  2. 天然继承 G4 的两道闸门（硬规则 + ApiExposure 白名单）与显式身份；
//  3. 未列入本表的 cap 仍回落到真实 handler —— 那批是没有 REST 端点的能力
//     （agent_*/task/attachment_upload/template_reload/skill_search/skill_install
//     以及 read_file/bash 等内置工具），它们必须保留。
//
// 表内每个端点都受 TestCapEndpointsExist 穷举校验（method+path 必须真实存在于
// api_catalog.json），写错路径会在测试期失败，不会静默落到 404。

// capEndpoint 描述一个底层能力等价于哪个后台 REST 端点。
type capEndpoint struct {
	Method string // GET/POST/DELETE
	Path   string // 后台端点路径，可省略 /system/api 前缀（运行时补全）
	// Rename 意图参数名 → 端点参数名。未在此登记的字段会原样透传给端点，
	// 若端点不接受该参数将被静默忽略——新增/修改意图参数务必对照
	// provider/api_catalog.json 确认端点确有此字段，否则应在此补 Rename 或砍掉该参数。
	Rename map[string]string
	Fixed  map[string]any // 常量注入（如 page_* 的 type=CategoryTypePage）
	// Defaults 缺省值：仅当调用方未传该键时补齐。
	// 用于"approve 默认通过"这类意图语义，端点本身没有这个默认值。
	Defaults map[string]any
	// Arrays 声明哪些（重命名后的）参数必须是数组，标量时自动包一层。
	// 端点用切片接收（如 archive/status 的 ids、archive/detail 的 images），
	// 直接传标量会让 ReadJSON 反序列化失败。
	Arrays []string
}

// CategoryTypePage 与 config.CategoryTypePage 对齐：单页在 category 表里以 type 区分。
const CategoryTypePage = 3

// ⚠️ partial **不在这里注入**。
//
// 它由 provider 的 injectPartialUpdate 统一负责（provider/api_tools.go）——
// 那里是所有调用通道的必经之处，包括 `api` 意图（通用端点调用）能直接调到的
// 任何端点。放在本层注入会漏掉那条通道：实测 `api action=invoke` 调
// POST /archive/detail 传 {id,title} 时，端点因 module_id 被清零而回「未定义模型」，
// 说明它确实没拿到 partial。
//
// 保留 Fixed/Defaults 的区别备忘：Fixed 是无条件注入（调用方关不掉），
// Defaults 允许调用方覆盖。partial 需要的是前者——但它已在 provider 层实现，
// 且那里的注入同样无条件覆盖。

var capEndpoints = map[string]capEndpoint{
	// ── 内容：文档 ──
	"archive_list":    {Method: "GET", Path: "/archive/list", Rename: map[string]string{"page": "current", "page_size": "pageSize", "keyword": "title", "order_by": "sort", "order_dir": "order"}},
	"archive_get":     {Method: "GET", Path: "/archive/detail"},
	"archive_create":  {Method: "POST", Path: "/archive/detail", Rename: map[string]string{"logo": "images"}, Arrays: []string{"images"}},
	"archive_update":  {Method: "POST", Path: "/archive/detail", Rename: map[string]string{"logo": "images"}, Arrays: []string{"images"}},
	"archive_publish": {Method: "POST", Path: "/archive/status", Rename: map[string]string{"id": "ids"}, Arrays: []string{"ids"}},
	"archive_delete":  {Method: "POST", Path: "/archive/delete"},

	// ── 内容：分类 ──
	"category_list":   {Method: "GET", Path: "/category/list"},
	"category_get":    {Method: "GET", Path: "/category/detail"},
	"category_create": {Method: "POST", Path: "/category/detail"},
	"category_update": {Method: "POST", Path: "/category/detail"},
	"category_delete": {Method: "POST", Path: "/category/delete"},

	// ── 内容：标签 ──
	"tag_list":   {Method: "GET", Path: "/plugin/tag/list"},
	"tag_get":    {Method: "GET", Path: "/plugin/tag/detail"},
	"tag_create": {Method: "POST", Path: "/plugin/tag/detail"},
	"tag_update": {Method: "POST", Path: "/plugin/tag/detail"},
	"tag_delete": {Method: "POST", Path: "/plugin/tag/delete"},

	// ── 结构：单页（复用 category，靠 type 区分）──
	"page_list":   {Method: "GET", Path: "/category/list", Fixed: map[string]any{"type": CategoryTypePage}},
	"page_get":    {Method: "GET", Path: "/category/detail"},
	"page_create": {Method: "POST", Path: "/category/detail", Fixed: map[string]any{"type": CategoryTypePage}},
	"page_update": {Method: "POST", Path: "/category/detail", Fixed: map[string]any{"type": CategoryTypePage}},
	"page_delete": {Method: "POST", Path: "/category/delete"},

	// ── 结构：模型 ──
	"module_list":   {Method: "GET", Path: "/module/list"},
	"module_get":    {Method: "GET", Path: "/module/detail"},
	"module_create": {Method: "POST", Path: "/module/detail"},
	"module_update": {Method: "POST", Path: "/module/detail"},
	"module_delete": {Method: "POST", Path: "/module/delete"},

	// ── 媒体：附件 ──
	// attachment_upload 不在此表：端点要 multipart 文件，而 AI 侧传的是
	// base64/URL/本地路径，且带路径穿越校验，属于真正没有等价端点的能力。
	"attachment_list":   {Method: "GET", Path: "/attachment/list", Rename: map[string]string{"page": "current", "page_size": "pageSize", "keyword": "q"}},
	"attachment_get":    {Method: "GET", Path: "/attachment/detail"},
	"attachment_delete": {Method: "POST", Path: "/attachment/delete"},

	// ── 结构：导航 ──
	"nav_list":   {Method: "GET", Path: "/setting/nav"},
	"nav_create": {Method: "POST", Path: "/setting/nav"},
	"nav_update": {Method: "POST", Path: "/setting/nav"},
	"nav_delete": {Method: "POST", Path: "/setting/nav/delete"},

	// ── 结构：友情链接 ──
	"friendlink_list":   {Method: "GET", Path: "/plugin/link/list"},
	"friendlink_create": {Method: "POST", Path: "/plugin/link/detail"},
	"friendlink_delete": {Method: "POST", Path: "/plugin/link/delete"},

	// ── 结构：301 重定向 ──
	// update 与 detail 是同一个端点（靠 id 是否为空区分新建/更新），delete 独立。
	// 补 update/delete 是因为实测发现：只暴露 create 的话，AI 建完就删不掉
	// （POST /plugin/redirect/detail 与 /plugin/redirect/delete 端点一直存在）。
	"redirect_list":   {Method: "GET", Path: "/plugin/redirect/list", Rename: map[string]string{"from": "from_url"}},
	"redirect_create": {Method: "POST", Path: "/plugin/redirect/detail", Rename: map[string]string{"from": "from_url", "to": "to_url"}},
	"redirect_update": {Method: "POST", Path: "/plugin/redirect/detail", Rename: map[string]string{"from": "from_url", "to": "to_url"}},
	"redirect_delete": {Method: "POST", Path: "/plugin/redirect/delete"},

	// ── SEO：关键词 / 锚文本 ──
	"keyword_list":   {Method: "GET", Path: "/plugin/keyword/list"},
	"keyword_create": {Method: "POST", Path: "/plugin/keyword/detail"},
	"keyword_delete": {Method: "POST", Path: "/plugin/keyword/delete"},
	"anchor_list":    {Method: "GET", Path: "/plugin/anchor/list"},
	"anchor_create":  {Method: "POST", Path: "/plugin/anchor/detail"},
	"anchor_delete":  {Method: "POST", Path: "/plugin/anchor/delete"},

	// ── SEO：sitemap / 推送 ──
	"sitemap_rebuild": {Method: "POST", Path: "/plugin/sitemap/build"},
	"url_push":        {Method: "POST", Path: "/plugin/push/push"},

	// ── 统计 ──
	"statistic_dashboard": {Method: "GET", Path: "/statistic/summary"},
	"statistic_spider":    {Method: "GET", Path: "/statistic/spider"},
	"statistic_traffic":   {Method: "GET", Path: "/statistic/traffic"},

	// ── 互动：评论 / 留言 ──
	"comment_list": {Method: "GET", Path: "/plugin/comment/list", Rename: map[string]string{"page": "current", "page_size": "pageSize"}},
	// approve 语义默认"通过"；端点本身没有默认值，不补会导致状态被置 0。
	"comment_approve": {Method: "POST", Path: "/plugin/comment/check", Defaults: map[string]any{"status": 1}},
	"comment_delete":  {Method: "POST", Path: "/plugin/comment/delete"},
	"guestbook_list":  {Method: "GET", Path: "/plugin/guestbook/list", Rename: map[string]string{"page": "current", "page_size": "pageSize"}},

	// ── 商务：用户 / 订单 ──
	"user_list":   {Method: "GET", Path: "/plugin/user/list", Rename: map[string]string{"page": "current", "page_size": "pageSize"}},
	"user_get":    {Method: "GET", Path: "/plugin/user/detail"},
	"user_create": {Method: "POST", Path: "/plugin/user/detail"},
	"user_update": {Method: "POST", Path: "/plugin/user/detail"},
	"user_delete": {Method: "POST", Path: "/plugin/user/delete"},
	"order_list":  {Method: "GET", Path: "/plugin/order/list", Rename: map[string]string{"page": "current", "page_size": "pageSize"}},

	// ── 系统：设置分区（system_setting 意图按 section 选取）──
	"setting_system":         {Method: "GET", Path: "/setting/system"},
	"setting_system_form":    {Method: "POST", Path: "/setting/system"},
	"setting_content":        {Method: "GET", Path: "/setting/content"},
	"setting_content_form":   {Method: "POST", Path: "/setting/content"},
	"setting_contact":        {Method: "GET", Path: "/setting/contact"},
	"setting_contact_form":   {Method: "POST", Path: "/setting/contact"},
	"setting_diy_field":      {Method: "GET", Path: "/setting/diyfield"},
	"setting_diy_field_form": {Method: "POST", Path: "/setting/diyfield"},
	"setting_index":          {Method: "GET", Path: "/setting/index"},
	"setting_index_form":     {Method: "POST", Path: "/setting/index"},
	"setting_safe":           {Method: "GET", Path: "/setting/safe"},
	"setting_safe_form":      {Method: "POST", Path: "/setting/safe"},
	"setting_migrate_db":     {Method: "POST", Path: "/setting/migratedb"},

	// ── 系统：伪静态 / 插件 ──
	"rewrite_get":             {Method: "GET", Path: "/plugin/rewrite"},
	"rewrite_form":            {Method: "POST", Path: "/plugin/rewrite"},
	"plugin_robots_get":       {Method: "GET", Path: "/plugin/robots"},
	"plugin_robots_set":       {Method: "POST", Path: "/plugin/robots", Rename: map[string]string{"content": "robots"}},
	"plugin_htmlcache_build":  {Method: "POST", Path: "/plugin/htmlcache/build"},
	"plugin_fulltext_rebuild": {Method: "POST", Path: "/plugin/fulltext/rebuild"},
	"plugin_backup_dump":      {Method: "POST", Path: "/plugin/backup/dump"},

	// ── 系统：模板 ──
	// template_reload 不在此表：它发的是 RestartChan 重载信号，无 REST 等价物。
	"template_get_info": {Method: "GET", Path: "/design/info"},

	// ── 系统：技能 ──
	// skill_* 走真实端点（route/manage.go 的 /anqi/skill/*，handler 在
	// controller/manageController/skill.go）。skill_delete 端点自带字符白名单校验
	// （只允许字母数字中划线下划线，/ 与 . 均不在其中），已挡住路径穿越，
	// 故这里不需要额外的内存实现。
	"skill_list":   {Method: "GET", Path: "/anqi/skill/list"},
	"skill_get":    {Method: "GET", Path: "/anqi/skill/detail"},
	"skill_save":   {Method: "POST", Path: "/anqi/skill/edit"},
	"skill_reload": {Method: "POST", Path: "/anqi/skill/reload"},
	"skill_delete": {Method: "POST", Path: "/anqi/skill/delete"},

	// ── 系统：站点信息 ──
	"website_info": {Method: "GET", Path: "/siteinfo"},
	"version":      {Method: "GET", Path: "/version/info"},
	"anqi_info":    {Method: "GET", Path: "/anqi/info"},
}

// capEndpointOf 返回能力对应的端点；ok=false 表示必须回落到能力 handler。
func capEndpointOf(name string) (capEndpoint, bool) {
	ep, ok := capEndpoints[name]
	return ep, ok
}

// CapEndpoint 是 capEndpoint 的导出视图，供包外校验/审计读取本表。
type CapEndpoint struct {
	Method   string
	Path     string
	Rename   map[string]string
	Fixed    map[string]any
	Defaults map[string]any
	Arrays   []string
}

// CapEndpoints 返回「能力名 → 端点」映射的副本。
func CapEndpoints() map[string]CapEndpoint {
	out := make(map[string]CapEndpoint, len(capEndpoints))
	for name, ep := range capEndpoints {
		out[name] = CapEndpoint{
			Method:   ep.Method,
			Path:     ep.Path,
			Rename:   ep.Rename,
			Fixed:    ep.Fixed,
			Defaults: ep.Defaults,
			Arrays:   ep.Arrays,
		}
	}
	return out
}

// callCap 是意图层调用底层能力的唯一入口。
//
// 命中 capEndpoints 的能力改由 api_invoke 走真实端点；未命中的（无 REST 等价物）
// 仍调用真实 handler。这样 switchCompose/Delegate/自定义 Compose 都不必关心
// 某个能力到底是端点还是内存实现。
func callCap(ctx context.Context, inv CapInvoker, name string, args map[string]any) (string, error) {
	if inv == nil {
		return "", fmt.Errorf("底层能力 %s 无法调用：CapInvoker 为空", name)
	}
	if ep, ok := capEndpoints[name]; ok {
		return invokeCapEndpoint(ctx, inv, ep, args)
	}
	return inv(ctx, name, args)
}

// invokeCapEndpoint 把意图参数换算成端点参数后交给 api_invoke。
func invokeCapEndpoint(ctx context.Context, inv CapInvoker, ep capEndpoint, args map[string]any) (string, error) {
	params := make(map[string]any, len(args)+len(ep.Fixed)+len(ep.Defaults))
	for k, v := range args {
		if nk, ok := ep.Rename[k]; ok {
			k = nk
		}
		params[k] = v
	}
	// 统一分页词表：意图对外说 page/page_size，列表端点读 current/pageSize。
	// 各 cap 的 Rename 若已显式换算过，这里不会重复覆盖（目标键已存在即跳过）。
	if !paginationRenameExempt(ep.Path) {
		for from, to := range paginationRenames {
			if v, ok := params[from]; ok {
				if _, exists := params[to]; !exists {
					params[to] = v
				}
				delete(params, from)
			}
		}
	}
	for k, v := range ep.Fixed {
		params[k] = v
	}
	for k, v := range ep.Defaults {
		if _, ok := params[k]; !ok {
			params[k] = v
		}
	}
	for _, k := range ep.Arrays {
		if v, ok := params[k]; ok && !isSliceValue(v) {
			params[k] = []any{v}
		}
	}
	return inv(ctx, "api_invoke", map[string]any{
		"method": ep.Method,
		"path":   normalizePath(ep.Path),
		"params": params,
	})
}

// normalizePath 补全后台端点前缀，允许表里写 /archive/list 这类简写。
func normalizePath(p string) string {
	if strings.HasPrefix(p, "/system/api") {
		return p
	}
	if strings.HasPrefix(p, "/api") {
		return "/system" + p
	}
	return "/system/api" + p
}

// paginationRenames 是意图层对外的分页词表 → 后台列表端点的字段名。
//
// 后台 34 个列表端点一律读 current/pageSize，意图声明的却是 page/page_size；
// 裸路由不做换算时 page 会被静默丢弃，模型以为翻了页、实际永远停在第一页。
// 例外：sitemap 配置端点的 page_size 是业务字段（每页 URL 数），
// 但那些端点只经 cap 路由（sitemap_rebuild）且不传分页参数，不受这里影响。
var paginationRenames = map[string]string{"page": "current", "page_size": "pageSize"}

// paginationRenameExempt 报告某端点是否把 page_size 当业务字段用（sitemap 的
// "每页 URL 数"），这类端点不能套用分页词表换算。
func paginationRenameExempt(path string) bool {
	return strings.Contains(path, "/plugin/sitemap")
}

// mergeRenames 合并多个换算表（后者覆盖前者）。
func mergeRenames(maps []map[string]string) map[string]string {
	total := 0
	for _, m := range maps {
		total += len(m)
	}
	if total == 0 {
		return nil
	}
	out := make(map[string]string, total)
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// endpointParams 把意图参数整理成裸路由端点真正读得到的参数。
//
// 三件事，缺一件就是一类静默失效：
//  1. 去掉分派用的 action（端点不认识它）；
//  2. 展开 values —— 后台配置类端点收的是扁平字段（push 的 baidu_api、
//     guestbook setting 的开关），嵌套传过去 ReadJSON 直接忽略，写操作会"成功但没写进去"；
//  3. 换算参数名：先是各意图自己的 renames，再叠加分页词表。
//     目标键已存在时不覆盖（模型显式传了端点字段名就以它为准）。
//
// InputSchema 未设 additionalProperties:false，模型仍可传未声明字段；
// 具体字段定义请用 api_schema 查询，不要凭描述猜。
func endpointParams(args map[string]any, renames map[string]string) map[string]any {
	sub := make(map[string]any, len(args))
	for k, v := range args {
		switch k {
		case "action":
			continue
		case "values":
			if valuesMap, ok := v.(map[string]any); ok {
				for vk, vv := range valuesMap {
					sub[vk] = vv
				}
			}
		default:
			sub[k] = v
		}
	}
	rename := func(from, to string) {
		v, ok := sub[from]
		if !ok {
			return
		}
		if _, exists := sub[to]; !exists {
			sub[to] = v
		}
		delete(sub, from)
	}
	for from, to := range renames {
		rename(from, to)
	}
	for from, to := range paginationRenames {
		rename(from, to)
	}
	return sub
}

// isSliceValue 判断参数值是否已是切片（端点按切片接收时不需要再包一层）。
func isSliceValue(v any) bool {
	switch v.(type) {
	case []any, []string, []int, []int64, []int32, []float64, []bool:
		return true
	}
	return false
}
