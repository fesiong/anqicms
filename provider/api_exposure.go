package provider

import (
	"sort"
	"strings"

	"kandaoni.com/anqicms/pkg/ai/eino"
)

// 本文件是 G4 的「开放范围白名单」判定层：决定 394 个后台端点中，
// 哪些允许被 api_invoke 通用调用触达。
//
// 它与意图白名单（ExposedIntents）是两道不同粒度的闸门：
//   - 意图白名单决定「要不要开放 api_list/api_schema/api_invoke 这类能力」；
//   - 本层决定「开放之后，能碰到哪些端点」。
//   即使前者被显式开启，端点仍需过这一层。零值配置 = 全部拒绝。
//
// 设计原则：**fail closed**。任何无法明确判为允许的情况都拒绝，
// 且拒绝理由要能回传给模型（模型据此换个只读端点，而不是反复重试）。

// 开放模式。零值（""）等同于 off。
const (
	ExposureModeOff       = "off"
	ExposureModeRead      = "read"
	ExposureModeReadWrite = "read_write"
	ExposureModeAll       = "all"
)

// riskRank 风险等级排序，用于与 mode 比较。未知风险按最高处理。
var riskRank = map[string]int{"read": 0, "write": 1, "destructive": 2}

// modeMaxRisk 各模式允许的最高风险等级。
var modeMaxRisk = map[string]int{
	ExposureModeOff:       -1, // 全部拒绝
	ExposureModeRead:      0,
	ExposureModeReadWrite: 1,
	ExposureModeAll:       2,
}

// nsHardRule 内置硬规则，不可通过配置解除——它们是代码级风险，不是偏好问题。
type nsHardRule struct {
	// AllowRisk 为 nil 表示该命名空间一律拒绝；否则仅允许列出的风险等级。
	AllowRisk map[string]bool
	Reason    string
}

// nsHardRules 硬规则表。命中顺序无关，取最严格的一条。
var nsHardRules = map[string]nsHardRule{
	"login":    {nil, "登录认证流程：AI 介入会引入凭证处理风险，禁止通用调用"},
	"captcha":  {nil, "验证码校验：AI 无需也无法合理介入，禁止通用调用"},
	"password": {nil, "密码重置：涉及凭证变更，应交由用户自助完成，禁止通用调用"},
	// VA-012：/system/api/aigenerate/* 因路由挂错父级而完全免登录，
	// 且 GET /aigenerate/setting 原样返回 OpenAI / 讯飞星火密钥。
	// 在该缺陷修复前，无论配置如何都不允许通用调用。
	"aigenerate": {nil, "aigenerate 模块存在未修复的免登录缺陷（VA-012），修复前禁止通用调用"},
	// 管理员与权限组变更是提权路径：只读允许，写/删除一律拒绝。
	"admin": {map[string]bool{"read": true}, "管理员与权限组变更属提权路径，仅允许只读查询"},
}

// ExposureDecision 是一次开放判定的结果。
type ExposureDecision struct {
	Allowed bool   `json:"allowed"`
	Stage   string `json:"stage,omitempty"`  // hard_block / deny_ns / deny_endpoint / ns_not_allowed / risk_not_allowed / mode_off
	Reason  string `json:"reason,omitempty"` // 拒绝原因，直接回传给模型
}

// apiExposureOverride 允许离线验证与单测注入策略，避免改动线上站点配置。
// 生产恒为 nil，判定走 GetMcpConfig().ApiExposure。
var apiExposureOverride *eino.ApiExposureConfig

// SetApiExposureOverride 仅供离线验证（cmd/apilive）与单测注入，**勿在生产调用**。
// 传 nil 表示恢复读取站点配置。
func SetApiExposureOverride(cfg *eino.ApiExposureConfig) { apiExposureOverride = cfg }

// CurrentApiExposure 取得当前生效的开放策略。
func CurrentApiExposure() eino.ApiExposureConfig {
	if apiExposureOverride != nil {
		return *apiExposureOverride
	}
	return GetMcpConfig().ApiExposure
}

// EvaluateExposure 判定一个端点是否允许被通用调用。
//
// 判定顺序（任一不通过即拒绝，不再往下走）：
//  1. 内置硬规则 —— 凭证类、未修复缺陷模块、管理员变更的写操作；
//  2. DenyEndpoints —— 端点级黑名单；
//  3. DenyNS —— 命名空间黑名单；
//  4. Mode —— 关闭则全拒；
//  5. AllowNS —— 非空时为白名单，不在其内拒绝；
//  6. 风险等级是否超过 Mode 允许的上限。
func EvaluateExposure(cfg eino.ApiExposureConfig, ep EndpointMeta) ExposureDecision {
	if reason := invokeBlockReason(ep); reason != "" {
		return ExposureDecision{Stage: "hard_block", Reason: reason}
	}
	if hit, entry := matchDenyEndpoint(cfg.DenyEndpoints, ep); hit {
		return ExposureDecision{
			Stage:  "deny_endpoint",
			Reason: "端点被显式列入黑名单：" + entry,
		}
	}
	if hit, ns := matchNS(cfg.DenyNS, ep); hit {
		return ExposureDecision{
			Stage:  "deny_ns",
			Reason: "命名空间 " + ns + " 被显式列入黑名单",
		}
	}
	mode := normalizeExposureMode(cfg.Mode)
	if mode == ExposureModeOff {
		return ExposureDecision{
			Stage:  "mode_off",
			Reason: "未配置开放策略（mode 为空或 off）：请先把 mode 设为 read / read_write / all 再调用",
		}
	}
	if len(cfg.AllowNS) > 0 && !matchAnyNS(cfg.AllowNS, ep) {
		return ExposureDecision{
			Stage:  "ns_not_allowed",
			Reason: "命名空间 " + ep.NS + " 不在开放白名单内（allow_ns）",
		}
	}
	risk := normalizeRisk(ep.Risk)
	if riskRank[risk] > modeMaxRisk[mode] {
		return ExposureDecision{
			Stage: "risk_not_allowed",
			Reason: "端点风险等级为 " + risk + "，超出当前模式 " + mode +
				" 允许的范围；删除类端点需 mode=all，写操作需 mode=read_write 及以上",
		}
	}
	return ExposureDecision{Allowed: true, Stage: "ok"}
}

// invokeBlockReason 返回内置硬规则的拒绝原因，空串表示未被硬规则拒绝。
//
// 硬规则与可配置策略分离的意义：凭证流程、未修复的鉴权缺陷、提权路径
// 属于"开错了就会出事"的类别，不应因为一次配置失误被放开。
func invokeBlockReason(ep EndpointMeta) string {
	for _, key := range []string{ep.NS, ep.Resource} {
		rule, ok := nsHardRules[key]
		if !ok {
			continue
		}
		if rule.AllowRisk == nil {
			return rule.Reason
		}
		if !rule.AllowRisk[normalizeRisk(ep.Risk)] {
			return rule.Reason
		}
	}
	return ""
}

// EndpointBlockReason 返回端点被通用调用拦截的原因，空串表示可调用。
// 供生成工具的待补报告排除这类端点——它们永远不会被调用，给它们补注释是白费力气。
func EndpointBlockReason(ep EndpointMeta) string {
	return invokeBlockReason(ep)
}

// normalizeExposureMode 归一化模式名，未知值按 off 处理（fail closed）。
func normalizeExposureMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ExposureModeRead:
		return ExposureModeRead
	case ExposureModeReadWrite, "write":
		return ExposureModeReadWrite
	case ExposureModeAll:
		return ExposureModeAll
	default:
		return ExposureModeOff
	}
}

// normalizeRisk 未知风险按 destructive 处理（fail closed）。
func normalizeRisk(risk string) string {
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case "read":
		return "read"
	case "write":
		return "write"
	case "destructive":
		return "destructive"
	default:
		return "destructive"
	}
}

// matchAnyNS 判断端点是否命中任一命名空间条目。
func matchAnyNS(list []string, ep EndpointMeta) bool {
	hit, _ := matchNS(list, ep)
	return hit
}

// matchNS 前缀匹配命名空间。
//
// 用 e+"/" 而非裸前缀，避免 "archive" 误命中 "archivex" 这类命名；
// 同时兼容二级命名空间（plugin/keyword）。
func matchNS(list []string, ep EndpointMeta) (bool, string) {
	for _, raw := range list {
		e := strings.Trim(strings.TrimSpace(raw), "/")
		if e == "" {
			continue
		}
		for _, ns := range []string{ep.NS, ep.Resource} {
			if ns == "" {
				continue
			}
			if ns == e || strings.HasPrefix(ns, e+"/") {
				return true, e
			}
		}
	}
	return false, ""
}

// matchDenyEndpoint 匹配端点黑名单条目，格式 "METHOD /path" 或 "/path"。
func matchDenyEndpoint(list []string, ep EndpointMeta) (bool, string) {
	epPath := normalizeAPIPath(ep.Path)
	epMethod := strings.ToUpper(strings.TrimSpace(ep.Method))
	for _, raw := range list {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		fields := strings.Fields(entry)
		var method, path string
		switch len(fields) {
		case 1:
			path = fields[0]
		default:
			method = strings.ToUpper(fields[0])
			path = strings.Join(fields[1:], " ")
		}
		if normalizeAPIPath(path) != epPath {
			continue
		}
		if method != "" && method != "*" && method != epMethod {
			continue
		}
		return true, entry
	}
	return false, ""
}

// ExposureSummary 把策略套到整个目录上，产出统计与命名空间分布。
func ExposureSummary(cfg eino.ApiExposureConfig, cat *APICatalog) map[string]any {
	out := map[string]any{
		"mode":               normalizeExposureMode(cfg.Mode),
		"total":              0,
		"allowed":            0,
		"blocked":            0,
		"blocked_by_hard":    0,
		"allowed_by_risk":    map[string]int{},
		"allowed_namespaces": []map[string]any{},
	}
	if cat == nil {
		return out
	}
	byRisk := map[string]int{}
	nsCount := map[string]int{}
	hard := 0
	for _, ep := range cat.Endpoints {
		d := EvaluateExposure(cfg, ep)
		if invokeBlockReason(ep) != "" {
			hard++
		}
		if d.Allowed {
			out["allowed"] = out["allowed"].(int) + 1
			byRisk[normalizeRisk(ep.Risk)]++
			nsCount[ep.NS]++
		} else {
			out["blocked"] = out["blocked"].(int) + 1
		}
	}
	out["total"] = len(cat.Endpoints)
	out["blocked_by_hard"] = hard
	out["allowed_by_risk"] = byRisk

	nsKeys := make([]string, 0, len(nsCount))
	for k := range nsCount {
		nsKeys = append(nsKeys, k)
	}
	sort.Strings(nsKeys)
	nsList := make([]map[string]any, 0, len(nsKeys))
	for _, k := range nsKeys {
		nsList = append(nsList, map[string]any{"ns": k, "count": nsCount[k]})
	}
	out["allowed_namespaces"] = nsList
	return out
}

// RecommendedExposure 推荐的起步策略：只读 + 内容相关命名空间。
//
// 为什么默认不给写：AI 产出内容的正确性问题（幻觉字段、批量误改）比"改不动"更难收拾，
// 起步阶段先把读的能力用稳，再按需要放开 write。
func RecommendedExposure() eino.ApiExposureConfig {
	return eino.ApiExposureConfig{
		Mode: ExposureModeRead,
		AllowNS: []string{
			"archive", "category", "module", "tag", "statistic",
			"attachment", "comment", "guestbook", "nav", "link",
		},
	}
}
