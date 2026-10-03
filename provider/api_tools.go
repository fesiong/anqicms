package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// 本文件实现 G3 的三个「REST 通用调用」元能力（caps）。
//
// 定位：它们不是给模型直接使用的工具，而是供 api_list / api_schema / api_invoke
// 三个意图委托调用的底层能力，因此注册在 capHandlers（非模型可见的 Tools）。
//
// 为什么需要它们：39 个精选意图只覆盖了 394 个后台端点的一小部分（F 阶段实测 21.1%），
// 其中 plugin 命名空间独占 223 个端点却几乎没有 cap。与其手写上百个 cap 去追平这套
// 平行实现，不如让模型按需「发现 → 取 schema → 调用」，从而一次覆盖全部端点。
//
// 安全边界：
//  1. api_invoke 的执行身份只能来自显式配置，模型**无法通过参数指定 admin_id**，
//     且绝不隐式回落到超级管理员（AdminPermission 对 adminId==1 直接放行）。
//  2. 高风险命名空间（登录/验证码/找回密码）默认拒绝，见 nsInvokeBlocked。
//  3. 三个 api_* 意图在 intent 层标记 DefaultOff，默认不出现在任何工具清单里。

// 底层能力名，必须与 pkg/mcp/intent 里 api_* 意图的 Caps 字段保持一致。
const (
	capAPIList   = "api_list"
	capAPISchema = "api_schema"
	capAPIInvoke = "api_invoke"
)

// 硬拒绝规则与可配置的开放策略都在 provider/api_exposure.go（G4）。
// 这里的硬规则（凭证类、VA-012 缺陷模块、管理员变更写操作）不受任何配置影响。

// defaultListLimit 缺省返回条数上限，防止一次把 394 条全部塞进上下文。
const defaultListLimit = 30

// apiListArgs 是 api_list 的入参。
type apiListArgs struct {
	Keyword string `json:"keyword"` // 路径/处理器/资源名关键词
	// NS 按命名空间前缀过滤，只填一级：archive / plugin / seo …
	//
	// 不要写 "plugin/keyword" 这种 ns/resource 组合——ns 与 resource 是两个独立
	// 字段（实测 plugin/keyword/* 的端点 ns=plugin、resource=keyword），
	// 组合写法匹配不到任何端点，matched 会是 0。要定位细分资源用 Keyword。
	NS string `json:"ns"`
	// Domain 按能力域过滤，如 content / contentops / siteops。
	// 推荐优先用它而非 ns：ns 是路由结构（/plugin/* 一个 ns 装了 229 个端点，过滤了等于没过滤），
	// domain 才是语义分组，能一次收敛到"备份缓存"或"内容生产"这类可决策的范围。
	Domain string `json:"domain"`
	Risk   string `json:"risk"`   // read / write / destructive
	Method string `json:"method"` // GET / POST ...
	Source string `json:"source"` // struct / urlparam / form / multipart / none
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	// OnlyAllowed 为 true 时只返回当前策略允许调用的端点。
	// 默认 true：把不可调用的端点推给模型只会诱导它反复重试。
	OnlyAllowed *bool `json:"only_allowed"`
}

// apiInvokeArgs 是 api_invoke 的入参。
type apiInvokeArgs struct {
	Method string         `json:"method"`
	Path   string         `json:"path"`
	Params map[string]any `json:"params"`
}

// capAPIList 列出可用的后台 REST 端点，支持按命名空间/风险/方法/关键词过滤。
//
// 返回里三个计数语义不同，别混用：
//   - total   = 端点目录总数（恒定，与筛选无关）
//   - matched = 本次筛选命中的端点数
//   - returned= 本页实际返回条数（受 limit 约束）
//
// matched=0 说明筛选条件太严，最常见是 ns 写成了 "ns/resource" 组合（见 apiListArgs.NS）。
func (svc *AiChatService) capAPIList(_ context.Context, argsJSON string) (string, error) {
	var a apiListArgs
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", fmt.Errorf("参数解析失败: %w", err)
		}
	}
	cat, err := BuildAPICatalog()
	if err != nil {
		return "", fmt.Errorf("API 目录不可用: %w", err)
	}
	policy := CurrentApiExposure()

	matched := make([]EndpointMeta, 0, len(cat.Endpoints))
	for _, ep := range cat.Endpoints {
		if !matchEndpoint(ep, a) {
			continue
		}
		// 默认只展示可调用的端点：把被拦的端点推给模型只会诱导反复重试。
		if onlyAllowedDefault(a.OnlyAllowed) && !EvaluateExposure(policy, ep).Allowed {
			continue
		}
		matched = append(matched, ep)
	}

	limit := a.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	offset := a.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > len(matched) {
		offset = len(matched)
	}
	page := matched[offset:]
	if len(page) > limit {
		page = page[:limit]
	}

	// 命名空间分布统计，帮模型快速收敛搜索范围
	nsCount := map[string]int{}
	for _, ep := range matched {
		nsCount[ep.NS]++
	}
	nsList := make([]map[string]any, 0, len(nsCount))
	keys := make([]string, 0, len(nsCount))
	for k := range nsCount {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		nsList = append(nsList, map[string]any{"ns": k, "count": nsCount[k]})
	}

	// 能力域分布：帮模型按语义而非路由结构收敛搜索范围
	domainCount := map[string]int{}
	for _, ep := range matched {
		domainCount[ep.Domain]++
	}
	domainList := make([]map[string]any, 0, len(domainCount))
	domainKeys := make([]string, 0, len(domainCount))
	for k := range domainCount {
		domainKeys = append(domainKeys, k)
	}
	sort.Strings(domainKeys)
	for _, k := range domainKeys {
		domainList = append(domainList, map[string]any{
			"domain": k,
			"count":  domainCount[k],
			"label":  intent.DomainLabel(intent.Domain(k)),
		})
	}

	items := make([]map[string]any, 0, len(page))
	for _, ep := range page {
		d := EvaluateExposure(policy, ep)
		items = append(items, map[string]any{
			"method": ep.Method,
			"path":   ep.Path,
			// desc 是端点摘要。api_list 阶段模型只有 method+path 可看，
			// 没有它就得靠猜（/plugin/anchor/list 这类路径语义并不自明）。
			"desc":         truncateDesc(ep.Desc, 120),
			"ns":           ep.NS,
			"resource":     ep.Resource,
			"domain":       ep.Domain,
			"risk":         ep.Risk,
			"param_source": ep.ParamSource,
			"blocked":      invokeBlockReason(ep),
			"allowed":      d.Allowed,
			"stage":        d.Stage,
			"reason":       d.Reason,
		})
	}

	out := map[string]any{
		"total":      len(cat.Endpoints),
		"matched":    len(matched),
		"returned":   len(page),
		"limit":      limit,
		"offset":     offset,
		"namespaces": nsList,
		"domains":    domainList,
		"endpoints":  items,
		"exposure":   ExposureSummary(policy, cat),
	}
	if len(matched) == 0 {
		out["hint"] = "当前开放策略下没有可调用的端点：请检查 api_exposure 的 mode 与 allow_ns" +
			"（mode=off 或未配置时全部拒绝）；传 only_allowed=false 可查看被拦截的端点及原因"
	}
	return marshalJSON(out)
}

// onlyAllowedDefault 处理 only_allowed 的三态：未传 = true（默认只列可调用端点）。
func onlyAllowedDefault(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}

// capAPISchema 返回单个端点完整的参数 schema，供调用前确认字段。
func (svc *AiChatService) capAPISchema(_ context.Context, argsJSON string) (string, error) {
	var a struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	if a.Path == "" {
		return "", fmt.Errorf("path 为必填")
	}
	cat, err := BuildAPICatalog()
	if err != nil {
		return "", fmt.Errorf("API 目录不可用: %w", err)
	}
	method := strings.ToUpper(a.Method)
	if method == "" {
		method = "GET"
	}
	ep, ok := cat.FindEndpoint(method, normalizeAPIPath(a.Path))
	if !ok {
		return "", fmt.Errorf("未找到端点 %s %s，请先用 api_list 检索可用端点", method, a.Path)
	}
	policy := CurrentApiExposure()
	d := EvaluateExposure(policy, ep)
	out := map[string]any{
		"method":       ep.Method,
		"path":         ep.Path,
		"desc":         ep.Desc,
		"doc":          ep.Doc,
		"ns":           ep.NS,
		"domain":       ep.Domain,
		"risk":         ep.Risk,
		"param_source": ep.ParamSource,
		"struct_type":  ep.StructType,
		"blocked":      invokeBlockReason(ep),
		"allowed":      d.Allowed,
		"stage":        d.Stage,
		"reason":       d.Reason,
		"params":       ep.Params,
	}
	if !d.Allowed {
		out["hint"] = "该端点当前不允许调用：" + d.Reason
	}
	if ep.ParamSource == "multipart" {
		out["hint"] = "该文件端点接受 file 字段：可传 data URI（data:image/png;base64,...）或裸 base64 字符串；可用 file_name/filename 指定文件名"
	}
	// 请求体是 JSON 数组（[]pkg.Struct 或 type X []string）时明确告知，
	// 否则模型会把 params 里的字段套进一个对象里发出去
	switch {
	case strings.HasPrefix(ep.StructType, "[]"):
		out["body"] = "JSON 数组，元素类型为 " + strings.TrimPrefix(ep.StructType, "[]") + "（字段见 params）"
	case ep.ParamSource == "array":
		out["body"] = "JSON 数组，元素为标量值，不含对象字段"
	}
	return marshalJSON(out)
}

// capAPIInvoke 以配置的专用管理员身份，进程内调用一个后台端点。
//
// 身份来源仅有两个（按顺序），模型无法通过参数选择身份：
//  1. 目标站点配置的专用管理员（McpConfig.InvokeAdminId）；
//  2. 都没有则拒绝 —— 绝不隐式使用超级管理员。
func (svc *AiChatService) capAPIInvoke(ctx context.Context, argsJSON string) (string, error) {
	var a apiInvokeArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	method := strings.ToUpper(a.Method)
	if method == "" {
		return "", fmt.Errorf("method 为必填")
	}
	if a.Path == "" {
		return "", fmt.Errorf("path 为必填")
	}
	path := normalizeAPIPath(a.Path)

	// 安全校验必须排在所有实际操作之前，且失败即拒绝（fail closed）：
	// 目录不可用时宁可不调用，也不能退回"无校验执行"。
	cat, cerr := BuildAPICatalog()
	if cerr != nil {
		return "", fmt.Errorf("API 目录不可用，拒绝执行通用调用: %w", cerr)
	}
	ep, found := cat.FindEndpoint(method, path)
	if !found {
		return "", fmt.Errorf("未找到端点 %s %s，请先用 api_list 检索", method, path)
	}
	if reason := invokeBlockReason(ep); reason != "" {
		return "", fmt.Errorf("端点 %s %s 不允许通用调用：%s", method, path, reason)
	}
	// 第二道闸门：可配置的开放范围白名单（G4）。
	// 硬规则之上再过一次策略，未配置即全部拒绝。
	if d := EvaluateExposure(CurrentApiExposure(), ep); !d.Allowed {
		return "", fmt.Errorf("端点 %s %s 不在开放范围内（%s）：%s", method, path, d.Stage, d.Reason)
	}

	site := svc.site
	if site == nil {
		return "", fmt.Errorf("服务未绑定站点")
	}

	adminID, err := resolveInvokeAdminID(site)
	if err != nil {
		return "", err
	}

	params := injectPartialUpdate(method, path, a.Params)
	res, err := site.InvokeAdminAPI(adminID, method, path, params)
	if err != nil {
		return "", err
	}
	out := map[string]any{
		"ok":     res.OK,
		"status": res.Status,
		"code":   res.Code,
		"msg":    res.Msg,
		"data":   res.Data,
	}
	if !res.OK {
		out["hint"] = "调用失败：请核对参数后用 api_schema 确认字段定义再重试"
	}
	return marshalJSON(out)
}

// normalizeAPIPath 补全后台路径前缀，允许调用方传 /archive/list 或 archive/list。
func normalizeAPIPath(path string) string {
	p := strings.TrimSpace(path)
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "/system/api"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasPrefix(p, "/system/api") {
		p = "/system/api" + p
	}
	return p
}

// matchEndpoint 判断端点是否命中过滤条件。
func matchEndpoint(ep EndpointMeta, a apiListArgs) bool {
	if a.NS != "" {
		if !strings.HasPrefix(ep.NS, a.NS) && !strings.HasPrefix(ep.Resource, a.NS) {
			return false
		}
	}
	if a.Domain != "" && !strings.EqualFold(ep.Domain, a.Domain) {
		return false
	}
	if a.Risk != "" && !strings.EqualFold(ep.Risk, a.Risk) {
		return false
	}
	if a.Method != "" && !strings.EqualFold(ep.Method, a.Method) {
		return false
	}
	if a.Source != "" && !strings.EqualFold(ep.ParamSource, a.Source) {
		return false
	}
	if a.Keyword != "" {
		kw := strings.ToLower(a.Keyword)
		if !strings.Contains(strings.ToLower(ep.Path), kw) &&
			!strings.Contains(strings.ToLower(ep.Handler), kw) &&
			!strings.Contains(strings.ToLower(ep.Resource), kw) {
			return false
		}
	}
	return true
}

// resolveInvokeAdminID 确定通用调用使用的管理员身份。
//
// 这是有意设计得"不方便"：宁可调用失败，也不允许隐式取得超级管理员权限。
// 未配置专用管理员时，api_invoke 直接不可用。
//
// 身份只来自主站点配置（MCP 的 token 与管理员均只在主站点配一份，见 GetMcpConfig），
// 参数 site 只用于确认"服务已绑定站点"，不是身份来源。
func resolveInvokeAdminID(site *Website) (uint, error) {
	if site == nil {
		return 0, fmt.Errorf("站点未就绪")
	}
	return explicitInvokeAdminID(GetMcpConfig().InvokeAdminId)
}

// explicitInvokeAdminID 只接受显式配置的身份：0 一律拒绝。
// 单独成函数是为了让这条安全边界能在无数据库、无站点初始化状态下验证——
// 依赖当前站点配置的断言会随同包用例的执行顺序变绿或变红，等于没有断言。
func explicitInvokeAdminID(configured uint) (uint, error) {
	if configured > 0 {
		return configured, nil
	}
	return 0, fmt.Errorf("未配置通用调用的管理员身份：请先在 MCP 设置中指定 api_invoke 使用的管理员账号；" +
		"为避免越权，系统不会隐式使用超级管理员")
}

// truncateDesc 截断摘要，避免个别超长注释把 api_list 的返回撑爆。
func truncateDesc(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

func marshalJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("结果序列化失败: %w", err)
	}
	return string(raw), nil
}

// partialUpdateEndpoints 登记「支持 PATCH 语义」的 POST 端点。
//
// 背景（2026-10-03）：这些端点的控制器曾无条件 `req.UpdateAll = true`，
// 于是「只传一个字段」会把其余字段按零值写回——正文清空、标签清空、
// 导航链接被清、会员 status 归 0（账号被禁用）。实测三类都中过。
//
// 那个 true 是为**表单提交**准备的（前端提交整个表单，字段没出现 = 用户清空了它）。
// AI/程序化调用的语义正相反：没传 = 别碰。端点侧已改成
// `if !req.Partial { req.UpdateAll = true }`，这里负责把 partial 送进去。
//
// 为什么不放在 pkg/mcp/intent 侧统一注入：
// `api` 意图（通用端点调用通道）能调到本清单里的**任何**端点，
// 而它不走 capEndpoints、也不走 invokeRoutes，只经过本函数。
// 放这里是唯一能覆盖全部调用方（含未来新增的通道）的位置。
//
// ⚠️ 这里是**端点路径**白名单，不是意图参数白名单：
// 只有确认 provider 用 `if req.UpdateAll || …`（有开关）的端点才能加。
// 逐字段无条件赋值的端点（material/place/group）加了也没用——它们没有这个开关，
// 仍需意图层补齐，见 pkg/mcp/intent 的 invokeRouteOverwriteFields。
var partialUpdateEndpoints = map[string]bool{
	normalizeAPIPath("/archive/detail"):         true, // request.Archive
	normalizeAPIPath("/category/detail"):        true, // request.Category
	normalizeAPIPath("/module/detail"):          true, // request.ModuleRequest
	normalizeAPIPath("/setting/nav"):            true, // request.NavConfig
	normalizeAPIPath("/plugin/tag/detail"):      true, // request.PluginTag
	normalizeAPIPath("/plugin/user/detail"):     true, // request.UserRequest
	// ⚠️ /plugin/redirect/detail **不在**这里：PluginRedirectRequest 没有 UpdateAll
	// 字段，控制器也没有 `req.UpdateAll = true`（provider 是逐字段无条件赋值）。
	// 加了也没用——那种端点仍需意图层补齐，见 invokeRouteOverwriteFields。
}

// injectPartialUpdate 对支持 PATCH 的 POST 端点补上 partial=true。
//
// 无条件覆盖（不尊重调用方传的 partial）：这是最后一道闸门，
// 漏了就退回全量覆盖、静默清空数据，而回执仍是 ok=true。
// 想要全量覆盖的调用方应走显式动作，不该靠关掉这个开关。
//
// 非 POST、路径不在清单、params 为空时原样返回。
func injectPartialUpdate(method, path string, params map[string]any) map[string]any {
	if !strings.EqualFold(method, "POST") || len(params) == 0 {
		return params
	}
	if !partialUpdateEndpoints[normalizeAPIPath(path)] {
		return params
	}
	out := make(map[string]any, len(params)+1)
	for k, v := range params {
		out[k] = v
	}
	out["partial"] = true
	return out
}
