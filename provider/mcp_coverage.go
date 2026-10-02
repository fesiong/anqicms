package provider

// mcp_coverage.go —— REST 端点（route/manage.go）与「AI 可达性」的覆盖审计。
//
// 背景：anqicms 后台 REST API 有 400 个端点（route/manage.go），而 AI 侧早年有一批
// 与后端平行手写的能力实现（provider/aiTools.go 的 84 个 bespoke cap），两者从未对账
// —— 即设计文档所称的"能力包漂移"。
//
// H 阶段（2026-09-29）删掉了那批平行实现：意图层现在通过
// pkg/mcp/intent 的两张声明表直连真实端点（invokeRoutes / capEndpoints），
// 只保留 11 个确实没有 REST 等价物的能力。
//
// 于是覆盖口径必须跟着换，否则会出现"删得越干净、覆盖率越低"的荒谬结论
// （实测从 20.8% 掉到 0.8%，因为旧口径把 getEinoTools() 的剩余条目当成能力全貌）：
//
//   - 端点真相源：route/manage.go（本文件解析，与 api_catalog.json 交叉校验）
//   - AI 可达性真相源：intent.DeclaredEndpointTargets()（意图/cap 声明的端点，单一来源）
//
// 审计回答的是可行动的问题：**哪些端点 AI 已有语义化入口，哪些只能靠 api_invoke 手填
// 路径，哪些永远不可达**。

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// CoverageLevel 表示单个端点对 AI 的可达性分层。
type CoverageLevel string

const (
	// CovDirect：意图/cap 已声明该端点，且至少一个声明它的意图默认开放（开箱可用）。
	CovDirect CoverageLevel = "direct"
	// CovGated：意图/cap 已声明该端点，但声明方全部 DefaultOff，需 ExposedIntents 显式开启。
	CovGated CoverageLevel = "gated"
	// CovGeneric：无任何意图直连，只能由 api_invoke 手填 method+path 调用（同样受闸门约束）。
	CovGeneric CoverageLevel = "generic"
	// CovBlocked：命中内置硬规则，无论配置如何都不会被 AI 调用（凭证/提权/未修缺陷）。
	CovBlocked CoverageLevel = "blocked"
)

// Endpoint 是从 route/manage.go 解析出的一个后台 REST 端点。
type Endpoint struct {
	Method   string `json:"method"`
	Path     string `json:"path"`
	Handler  string `json:"handler"`
	NS       string `json:"ns"`       // 一级命名空间：archive / plugin / setting ...
	Resource string `json:"resource"` // plugin 取二级，其余取一级
	Line     int    `json:"line"`
}

// NsCoverage 是单个命名空间的覆盖统计。
type NsCoverage struct {
	NS     string `json:"ns"`
	Total  int    `json:"total"`
	Direct int    `json:"direct"` // 默认开放且已直连
	Gated  int    `json:"gated"`  // 已直连但需显式开启
	// Generic 无直连，只能走 api_invoke 通用调用。
	Generic int `json:"generic"`
	// Blocked 命中内置硬规则，永远不对 AI 开放。
	Blocked int `json:"blocked"`
	// CoverRate 有语义化入口的比例 = (direct+gated)/total。
	// 刻意把 gated 计入：它是"配一行白名单就能用"，与"压根没接"的 generic 不是一回事。
	CoverRate float64 `json:"cover_rate"`
}

// CoveragePair 记录单个端点的判定结果与声明来源，使分层结论可被逐条复核。
type CoveragePair struct {
	Method  string        `json:"method"`
	Path    string        `json:"path"`
	Handler string        `json:"handler"`
	Level   CoverageLevel `json:"level"`
	// Via 是声明来源（"intent:<name>" / "cap:<name>"），未直连时为空。
	Via string `json:"via"`
	// BlockReason 命中硬规则时的原因，空串表示未被拦。
	BlockReason string `json:"block_reason,omitempty"`
}

// CoverageReport 是覆盖审计结果。
type CoverageReport struct {
	ManagePath string `json:"manage_path"`
	// TotalEndpint 后台 REST 端点总数（route/manage.go 解析结果）。
	TotalEndpint int `json:"total_endpoint"`
	// TotalDeclared 意图层声明可达的端点数（invokeRoutes + capEndpoints 去重）。
	TotalDeclared int `json:"total_declared"`
	// TotalIntent 意图总数（IntentCatalog，含 DefaultOff）。
	TotalIntent int `json:"total_intent"`
	// TotalCap 意图引用的底层能力中，没有 REST 等价端点的数量（agent_*/skill_*/内置工具）。
	TotalCap int `json:"total_cap"`
	// NoEndpointCaps 是这批能力的名字（已排序）。它们不是缺口 —— 本来就没有端点可映射，
	// 只能走 handler。列出来是为了让人一眼看出"某个 cap 是不是忘了配端点映射"。
	NoEndpointCaps []string `json:"no_endpoint_caps"`
	// TotalBuiltin 本地非 REST 能力数（文件/shell/web）。
	TotalBuiltin int          `json:"total_builtin"`
	Direct       int          `json:"direct"`
	Gated        int          `json:"gated"`
	Generic      int          `json:"generic"`
	Blocked      int          `json:"blocked"`
	NsStats      []NsCoverage `json:"ns_stats"`
	// Uncovered 没有语义化入口的端点（generic + blocked），是补齐意图的对象。
	Uncovered []Endpoint `json:"uncovered"`
	// Pairs 全部判定明细，用于复核。
	Pairs []CoveragePair `json:"pairs"`
	// OrphanTargets 声明了但真实路由表里不存在的端点 —— 写错路径的强信号。
	OrphanTargets []string `json:"orphan_targets"`
}

// ────────────────────────── 路由解析 ──────────────────────────

var (
	coverPartyRe = regexp.MustCompile(`(\w+)\s*:=\s*(\w+)\.Party\(\s*"([^"]*)"`)
	coverRegRe   = regexp.MustCompile(`(\w+)\.(Get|Post|Put|Delete|Any|Head|Patch|Options|HandleMany)\s*\(`)
	coverHandler = regexp.MustCompile(`^(\w+)\.(\w+)$`)
	coverPathRe  = regexp.MustCompile(`^"([^"]*)"`)
	coverSlashRe = regexp.MustCompile(`//+`)
)

// coverStripComments 去掉 // 行注释（路由文件中存在被注释掉的历史注册），
// 需要跳过字符串字面量内的 // 。
func coverStripComments(line string) string {
	inStr := rune(0)
	escaped := false
	for i, ch := range line {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if inStr != 0 {
			if ch == inStr {
				inStr = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			inStr = ch
			continue
		}
		if ch == '/' && i+1 < len(line) && line[i+1] == '/' {
			return line[:i]
		}
	}
	return line
}

// coverBalancedArgs 返回位于 openIdx 的 '(' 到其匹配 ')' 之间的参数原文。
func coverBalancedArgs(src string, openIdx int) string {
	depth := 0
	inStr := rune(0)
	escaped := false
	for i := openIdx; i < len(src); i++ {
		ch := rune(src[i])
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if inStr != 0 {
			if ch == inStr {
				inStr = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			inStr = ch
		} else if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return src[openIdx+1 : i]
			}
		}
	}
	return ""
}

// coverSplitArgs 按顶层逗号切分参数列表。
func coverSplitArgs(args string) []string {
	var parts []string
	var cur []rune
	depth := 0
	inStr := rune(0)
	escaped := false
	for _, ch := range args {
		if escaped {
			cur = append(cur, ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			cur = append(cur, ch)
			escaped = true
			continue
		}
		if inStr != 0 {
			cur = append(cur, ch)
			if ch == inStr {
				inStr = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			inStr = ch
			cur = append(cur, ch)
		case '(', '[', '{':
			depth++
			cur = append(cur, ch)
		case ')', ']', '}':
			depth--
			cur = append(cur, ch)
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(string(cur)))
				cur = nil
			} else {
				cur = append(cur, ch)
			}
		default:
			cur = append(cur, ch)
		}
	}
	if len(cur) > 0 {
		parts = append(parts, strings.TrimSpace(string(cur)))
	}
	return parts
}

// ParseManageRoutes 解析 route/manage.go，产出后台 REST 端点清单。
func ParseManageRoutes(path string) ([]Endpoint, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	for i, l := range lines {
		lines[i] = coverStripComments(l)
	}
	src := strings.Join(lines, "\n")

	// party 变量 -> (父变量, 前缀)，按变量继承解析嵌套
	type party struct{ parent, prefix string }
	parties := map[string]party{}
	for _, m := range coverPartyRe.FindAllStringSubmatch(src, -1) {
		if m[1] == "app" {
			continue
		}
		parties[m[1]] = party{parent: m[2], prefix: m[3]}
	}
	resolve := func(v string) string {
		var chain []string
		seen := map[string]bool{}
		for !seen[v] {
			seen[v] = true
			p, ok := parties[v]
			if !ok {
				break
			}
			chain = append(chain, p.prefix)
			v = p.parent
		}
		if len(chain) == 0 {
			return ""
		}
		var sb strings.Builder
		for i := len(chain) - 1; i >= 0; i-- {
			if seg := strings.Trim(chain[i], "/"); seg != "" {
				sb.WriteString("/")
				sb.WriteString(seg)
			}
		}
		return sb.String()
	}

	var out []Endpoint
	for _, m := range coverRegRe.FindAllStringSubmatchIndex(src, -1) {
		varName := src[m[2]:m[3]]
		kind := src[m[4]:m[5]]
		// m[1] 指向紧跟在 '(' 之后的位置，故 '(' 在 m[1]-1
		openIdx := m[1] - 1
		if openIdx < 0 || src[openIdx] != '(' {
			continue
		}
		args := coverSplitArgs(coverBalancedArgs(src, openIdx))
		if len(args) == 0 {
			continue
		}
		method, pathArg := "", ""
		if kind == "HandleMany" {
			if len(args) < 3 {
				continue
			}
			method = strings.Trim(args[0], `"`)
			pathArg = args[1]
		} else {
			method = strings.ToUpper(kind)
			pathArg = args[0]
		}
		if method == "" {
			continue
		}
		handlerArg := strings.TrimSpace(args[len(args)-1])

		pm := coverPathRe.FindStringSubmatch(pathArg)
		if pm == nil {
			continue
		}
		lit := strings.TrimSpace(pm[1])
		if lit == "" {
			continue
		}
		first := strings.Fields(lit)[0]
		// 通配捕获路由（如 /{path:path}）不是具体端点
		if strings.Contains(first, "{") && strings.Contains(first, "path:") {
			continue
		}
		base := resolve(varName)
		full := strings.TrimRight(coverSlashRe.ReplaceAllString(base+"/"+strings.TrimLeft(first, "/"), "/"), "/")
		if full == "" {
			continue
		}

		handler := ""
		if hm := coverHandler.FindStringSubmatch(handlerArg); hm != nil {
			handler = hm[2]
		}
		line := strings.Count(src[:m[0]], "\n") + 1

		ns, resource := "", ""
		trimmed := strings.TrimPrefix(full, "/system/api/")
		if segs := strings.Split(strings.Trim(trimmed, "/"), "/"); len(segs) > 0 {
			ns = segs[0]
			resource = segs[0]
			// plugin 命名空间下再取一级作为真实资源（plugin/<plugin-name>/...）
			if ns == "plugin" && len(segs) > 1 {
				resource = segs[1]
			}
		}
		out = append(out, Endpoint{
			Method: method, Path: full, Handler: handler,
			NS: ns, Resource: resource, Line: line,
		})
	}
	return out, nil
}

// FindManageRoute 在可执行文件目录与工作目录附近寻找 route/manage.go。
func FindManageRoute() (string, error) {
	var cands []string
	if ep := config.ExecPath; ep != "" {
		cands = append(cands, filepath.Join(ep, "route", "manage.go"))
	}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 5; i++ {
			cands = append(cands, filepath.Join(dir, "route", "manage.go"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("coverage: 未找到 route/manage.go（已尝试 %d 个候选路径）", len(cands))
}

// ────────────────────────── 覆盖比对 ──────────────────────────

// AuditCapabilityCoverage 解析 manage.go 得到端点真相源，再与意图层声明的
// 可达端点（intent.DeclaredEndpointTargets）对账，产出可达性分层报告。
//
// 判定顺序（先严后宽，避免"声明了但被硬规则拦"被算成可用）：
//  1. 命中内置硬规则 → blocked：配置无法解除，声明了也到不了；
//  2. 被意图/cap 声明 → direct（至少一个声明方默认开放）或 gated（声明方全部 DefaultOff）；
//  3. 其余 → generic：只能由 api_invoke 手填 method+path 调用。
//
// 与旧口径（拿 getEinoTools() 当 caps 真相源）的关键差别：旧口径衡量的是
// "有没有手写一份平行实现"，删掉平行实现反而会让覆盖率暴跌；新口径衡量的是
// "AI 有没有语义化入口"，补齐意图才会让它上升——这才是指标该有的方向。
func (svc *AiChatService) AuditCapabilityCoverage(managePath string) (*CoverageReport, error) {
	if managePath == "" {
		p, err := FindManageRoute()
		if err != nil {
			return nil, err
		}
		managePath = p
	}
	endpoints, err := ParseManageRoutes(managePath)
	if err != nil {
		return nil, err
	}

	// 端点元数据（NS/Resource/Risk）取自编译期嵌入的目录表，仅供硬规则判定。
	// 目录缺失（如新增端点尚未重新生成）时退回解析出的 ns/resource，不因此报错。
	metaIndex := map[string]EndpointMeta{}
	if cat, cerr := BuildAPICatalog(); cerr == nil {
		for _, e := range cat.Endpoints {
			metaIndex[e.Method+" "+e.Path] = e
		}
	}

	// AI 可达性真相源：意图层声明的端点（invokeRoutes + capEndpoints）。
	declared := intent.DeclaredEndpointTargets()
	caps := intent.CapEndpoints()

	offByIntent := map[string]bool{}
	intentTotal := 0
	noEndpointCap := map[string]bool{}
	for _, spec := range intent.IntentCatalog {
		if spec == nil {
			continue
		}
		intentTotal++
		offByIntent[spec.Name] = spec.DefaultOff
		for _, c := range spec.Caps {
			if c == "api_invoke" || c == "api_list" || c == "api_schema" {
				continue // 通用调用元能力，不指向某个具体端点
			}
			if _, ok := caps[c]; !ok {
				noEndpointCap[c] = true // 刻意保留的无 REST 等价能力
			}
		}
	}

	builtin := 0
	if binTools, _ := svc.getBuiltinEinoTools(); len(binTools) > 0 {
		builtin = len(binTools)
	}

	rep := &CoverageReport{
		ManagePath:    managePath,
		TotalEndpint:  len(endpoints),
		TotalDeclared: len(declared),
		TotalIntent:   intentTotal,
		TotalCap:      len(noEndpointCap),
		TotalBuiltin:  builtin,
	}
	nsIdx := map[string]int{}

	// levelOf 返回分层、声明来源与硬规则原因。
	levelOf := func(ep Endpoint) (CoverageLevel, string, string) {
		key := ep.Method + " " + ep.Path
		meta, ok := metaIndex[key]
		if !ok {
			meta = EndpointMeta{Method: ep.Method, Path: ep.Path, NS: ep.NS, Resource: ep.Resource}
		}
		if reason := EndpointBlockReason(meta); reason != "" {
			return CovBlocked, "", reason
		}
		vias := declared[key]
		if len(vias) == 0 {
			return CovGeneric, "", ""
		}
		// 只有 cap 声明（没有任何意图在 Caps 里引用它）：cap 是意图委托的底座，按默认可达计。
		var intentVias []string
		for _, v := range vias {
			if strings.HasPrefix(v, "intent:") {
				intentVias = append(intentVias, strings.TrimPrefix(v, "intent:"))
			}
		}
		if len(intentVias) == 0 {
			return CovDirect, vias[0], ""
		}
		allOff := true
		for _, n := range intentVias {
			if !offByIntent[n] {
				allOff = false
				break
			}
		}
		if allOff {
			return CovGated, vias[0], ""
		}
		return CovDirect, vias[0], ""
	}

	for _, ep := range endpoints {
		lv, via, reason := levelOf(ep)
		idx, ok := nsIdx[ep.NS]
		if !ok {
			rep.NsStats = append(rep.NsStats, NsCoverage{NS: ep.NS})
			idx = len(rep.NsStats) - 1
			nsIdx[ep.NS] = idx
		}
		rep.NsStats[idx].Total++
		rep.Pairs = append(rep.Pairs, CoveragePair{
			Method: ep.Method, Path: ep.Path, Handler: ep.Handler,
			Level: lv, Via: via, BlockReason: reason,
		})
		switch lv {
		case CovDirect:
			rep.Direct++
			rep.NsStats[idx].Direct++
		case CovGated:
			rep.Gated++
			rep.NsStats[idx].Gated++
		case CovBlocked:
			rep.Blocked++
			rep.NsStats[idx].Blocked++
			rep.Uncovered = append(rep.Uncovered, ep)
		default:
			rep.Generic++
			rep.NsStats[idx].Generic++
			rep.Uncovered = append(rep.Uncovered, ep)
		}
	}

	for i := range rep.NsStats {
		if rep.NsStats[i].Total > 0 {
			rep.NsStats[i].CoverRate = float64(rep.NsStats[i].Direct+rep.NsStats[i].Gated) /
				float64(rep.NsStats[i].Total)
		}
	}
	// 命名空间按总端点数降序，缺口聚拢便于优先治理
	sort.SliceStable(rep.NsStats, func(i, j int) bool {
		return rep.NsStats[i].Total > rep.NsStats[j].Total
	})
	sort.SliceStable(rep.Uncovered, func(i, j int) bool {
		if rep.Uncovered[i].NS != rep.Uncovered[j].NS {
			return rep.Uncovered[i].NS < rep.Uncovered[j].NS
		}
		return rep.Uncovered[i].Path < rep.Uncovered[j].Path
	})

	// 声明了却不在真实路由表里的端点：路径写错或端点已下线的强信号。
	live := make(map[string]bool, len(endpoints))
	for _, ep := range endpoints {
		live[ep.Method+" "+ep.Path] = true
	}
	for k := range declared {
		if !live[k] {
			rep.OrphanTargets = append(rep.OrphanTargets, k)
		}
	}
	sort.Strings(rep.OrphanTargets)
	for c := range noEndpointCap {
		rep.NoEndpointCaps = append(rep.NoEndpointCaps, c)
	}
	sort.Strings(rep.NoEndpointCaps)
	return rep, nil
}

// nsExposureGuidance 标注各命名空间对 AI 的**开放建议**。
//
// 并非所有缺口都值得填：认证类端点一旦对 AI 开放，会让模型有机会接触凭证流程，
// 属于"覆盖率越高风险越大"的反例。G 阶段补齐时应先读这张表再做取舍。
var nsExposureGuidance = map[string]string{
	"login":      "不建议开放：登录认证流程，AI 介入会引入凭证处理风险",
	"captcha":    "不建议开放：验证码是人机校验，AI 无需也无法合理介入",
	"password":   "不建议开放：找回密码涉及凭证重置，应交由用户自助完成",
	"admin":      "谨慎开放：管理员/管理员组的增删改涉及提权风险，如需开放建议只保留只读查询",
	"aigenerate": "开放前须先修鉴权：该命名空间此前存在路由挂载缺陷导致完全免登录（见安全审计 VA-012）",
	"design":     "待定：模板/设计类读写，建议 G 阶段按真实需求选择性开放",
	"collector":  "待定：采集任务管理，建议提供受限的任务级能力",
}

// Markdown 渲染审计报告，便于归档与人工复核。maxPerNS 限制每个命名空间打印的缺口条数（0 表示不限）。
func (r *CoverageReport) Markdown(maxPerNS int) string {
	var b strings.Builder
	b.WriteString("# AI 能力覆盖审计（route/manage.go × 意图层声明）\n\n")
	fmt.Fprintf(&b, "- 路由文件：`%s`\n", r.ManagePath)
	fmt.Fprintf(&b, "- 后台 REST 端点：**%d**\n", r.TotalEndpint)
	fmt.Fprintf(&b, "- 意图层声明可达的端点：**%d**（意图 %d 个；无 REST 等价能力 %d 个；本地能力 %d 个）\n\n",
		r.TotalDeclared, r.TotalIntent, r.TotalCap, r.TotalBuiltin)

	covered := r.Direct + r.Gated
	b.WriteString("## 总览\n\n| 层 | 含义 | 数量 | 占比 |\n|---|---|---|---|\n")
	rows := []struct {
		name, desc string
		n          int
	}{
		{"direct", "有意图/cap 直连，且默认开放", r.Direct},
		{"gated", "有直连但声明方全部 DefaultOff，需显式开启", r.Gated},
		{"generic", "无直连，只能走 api_invoke 通用调用", r.Generic},
		{"blocked", "命中内置硬规则，任何配置下都不可达", r.Blocked},
	}
	for _, rw := range rows {
		pct := 0.0
		if r.TotalEndpint > 0 {
			pct = float64(rw.n) / float64(r.TotalEndpint) * 100
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %.1f%% |\n", rw.name, rw.desc, rw.n, pct)
	}
	denom := maxInt(r.TotalEndpint, 1)
	fmt.Fprintf(&b, "\n**有语义化入口（direct+gated）：%d/%d = %.1f%%**\n\n",
		covered, r.TotalEndpint, float64(covered)/float64(denom)*100)

	b.WriteString("## 按命名空间\n\n| 命名空间 | 端点 | direct | gated | generic | blocked | 覆盖率 |\n|---|---|---|---|---|---|---|\n")
	for _, s := range r.NsStats {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %.1f%% |\n",
			s.NS, s.Total, s.Direct, s.Gated, s.Generic, s.Blocked, s.CoverRate*100)
	}

	b.WriteString("## 开放建议（并非所有缺口都该填）\n\n")
	b.WriteString("> 覆盖率不是越高越好：认证类端点一旦对 AI 开放，会让模型接触凭证流程。\n")
	b.WriteString("> 属于\"覆盖率越高风险越大\"的反例。补齐意图前请先读这张表。\n\n")
	b.WriteString("| 命名空间 | 缺口数 | 开放建议 |\n|---|---|---|\n")
	for _, s := range r.NsStats {
		gaps := s.Generic + s.Blocked
		g, ok := nsExposureGuidance[s.NS]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "| %s | %d | %s |\n", s.NS, gaps, g)
	}
	b.WriteString("\n其余未列出的命名空间无特殊限制，可按业务需求补齐意图。\n\n")

	b.WriteString("## 缺口清单（generic + blocked 层）\n\n")
	b.WriteString("> 补齐对象：优先治理端点数多且覆盖率低的命名空间。\n")
	b.WriteString("> blocked 层不在此列——它们被硬规则拦住，补了也不会被调用。\n\n")
	byNS := map[string][]Endpoint{}
	for _, ep := range r.Uncovered {
		byNS[ep.NS] = append(byNS[ep.NS], ep)
	}
	nsList := make([]string, 0, len(byNS))
	for ns := range byNS {
		nsList = append(nsList, ns)
	}
	sort.Strings(nsList)
	for _, ns := range nsList {
		eps := byNS[ns]
		fmt.Fprintf(&b, "### %s（缺口 %d）\n\n", ns, len(eps))
		b.WriteString("| 方法 | 路径 | handler | 行号 |\n|---|---|---|---|\n")
		limit := len(eps)
		if maxPerNS > 0 && limit > maxPerNS {
			limit = maxPerNS
		}
		for _, ep := range eps[:limit] {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %d |\n", ep.Method, ep.Path, ep.Handler, ep.Line)
		}
		if limit < len(eps) {
			fmt.Fprintf(&b, "\n_（省略其余 %d 条）_\n", len(eps)-limit)
		}
		b.WriteString("\n")
	}

	// 判定明细：让 direct / gated 两层的结论可被逐条复核
	b.WriteString("## 判定明细（direct / gated 层）\n\n")
	b.WriteString("> 列出每个被认为已直连的端点及其声明来源，用于人工复核。\n\n")
	b.WriteString("| 层 | 方法 | 路径 | handler | 声明来源 |\n|---|---|---|---|---|\n")
	for _, p := range r.Pairs {
		if p.Level != CovDirect && p.Level != CovGated {
			continue
		}
		fmt.Fprintf(&b, "| %s | %s | `%s` | %s | `%s` |\n", p.Level, p.Method, p.Path, p.Handler, p.Via)
	}
	b.WriteString("\n")

	if len(r.NoEndpointCaps) > 0 {
		fmt.Fprintf(&b, "## 无 REST 等价端点的底层能力（%d）\n\n", len(r.NoEndpointCaps))
		b.WriteString("> 不是缺口：这些能力本来就只能通过 handler 实现（智能体/技能/任务/上传/内置工具）。\n")
		b.WriteString("> 列在这里是为了复核——如果某个名字其实有端点，那就是漏配了 capEndpoints。\n\n")
		for _, c := range r.NoEndpointCaps {
			fmt.Fprintf(&b, "- `%s`\n", c)
		}
		b.WriteString("\n")
	}

	if len(r.OrphanTargets) > 0 {
		fmt.Fprintf(&b, "## 声明了但真实路由表里不存在的端点（%d）\n\n", len(r.OrphanTargets))
		b.WriteString("> 强信号：意图层写错了路径，或端点已下线而声明未清理。\n")
		b.WriteString("> 这些声明一旦被调用必然 404，必须修掉。\n\n")
		for _, k := range r.OrphanTargets {
			fmt.Fprintf(&b, "- `%s`\n", k)
		}
	}
	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
