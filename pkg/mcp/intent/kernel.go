package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AuditFunc 由 provider 注入的审计回调。内核在每次意图执行后调用，
// 传入风险等级与执行结果，便于落库（token 仅存掩码，见 provider/mcp.go）。
type AuditFunc func(ctx context.Context, name, risk string, argsJSON string, callErr error, start time.Time)

// Config 内核配置（来自站点 McpConfig，经 provider 转换）。
type Config struct {
	// ExposedIntents 意图级白名单：意图名、"domain:*"、"content"(整域)、"*" 全量。
	// 非空时仅暴露命中项（未知意图默认拒绝）。
	ExposedIntents []string
	// ExposedTools 旧版能力名白名单（兼容 C 阶段）：按意图的 Caps 命中决定暴露。
	ExposedTools []string
	// ToolListMode "summary" 时 tools/list 仅返回轻量 schema（描述含域/风险），降低 token 消耗。
	ToolListMode string
}

// Kernel 是"意图层"内核：负责意图的注册状态、调用路由、两阶段 scope 与执行。
// 它不依赖任何通道 SDK（MCP / Eino 适配器在 mcp.go / eino.go 中），底层能力通过
// CapInvoker 回调注入，避免与 provider 循环依赖。
type Kernel struct {
	cfg     Config
	cap     CapInvoker
	audit   AuditFunc
	mu      sync.Mutex
	intents map[string]*IntentSpec
	order   []string
	scope   map[Domain]bool // 两阶段：非 nil 时仅暴露这些域
	reg     []string        // 当前已注册的意图名（含 meta），供通道适配器重注册
	// scopeTouched 是最近一次 set_scope 的时间，用于自动恢复（见 RestoreScopeIfStale）。
	// 零值表示从未设置过。
	scopeTouched time.Time
}

// NewKernel 构建内核并做启动自检（重名 / 漏标 Risk / 既无 Compose 也无 Caps 直接 panic，把漂移消灭在启动期）。
func NewKernel(cfg Config, cap CapInvoker, audit AuditFunc) *Kernel {
	k := &Kernel{cfg: cfg, cap: cap, audit: audit, intents: map[string]*IntentSpec{}}
	for _, s := range IntentCatalog {
		if _, dup := k.intents[s.Name]; dup {
			panic("intent 重名: " + s.Name)
		}
		if s.Risk == "" {
			panic(fmt.Sprintf("intent %s 未标注 Risk", s.Name))
		}
		if s.Compose == nil && len(s.Caps) == 0 {
			panic(fmt.Sprintf("intent %s 既无 Compose 也无 Caps", s.Name))
		}
		k.intents[s.Name] = s
		k.order = append(k.order, s.Name)
	}
	return k
}

// Execute 执行一个已注册的意图，返回中性结果（不依赖任何通道 SDK）。
// 通道适配器（MCP / Eino）负责把它包装成各自的返回形态。
func (k *Kernel) Execute(ctx context.Context, name, argsJSON string) (*Result, error) {
	spec, ok := k.intents[name]
	if !ok {
		return nil, fmt.Errorf("意图 %s 不存在", name)
	}
	// 准入检查（纵深防御）：allowed 原本只在"注册到模型面"时生效，
	// 这意味着任何绕过注册直接调 Execute 的内部路径都能执行未开放意图——
	// 包括备份/升级/迁移这类 DefaultOff 的高危能力。这里再拦一次，
	// 让"未开放"成为意图本身的性质，而不是某个通道的实现细节。
	if !k.allowed(spec) {
		return nil, fmt.Errorf("意图 %s 未开放：DefaultOff 意图须由 ExposedIntents 显式开启", name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, fmt.Errorf("参数解析失败: %w", err)
	}
	if args == nil {
		args = map[string]any{}
	}
	return spec.Compose(ctx, args, k.cap)
}

// SetScope 两阶段 tools/list：设置后仅暴露指定能力域；传空恢复全部。
// 具体在 server 上移除 / 重注册工具由通道适配器负责（见 mcp.go 的 Reregister）。
//
// ⚠️ scope 是进程级全局的（原因见 Reregister 的注释），不是会话级。
func (k *Kernel) SetScope(domains []string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.scopeTouched = time.Now()
	if len(domains) == 0 {
		k.scope = nil
		return
	}
	m := map[Domain]bool{}
	for _, d := range domains {
		m[Domain(d)] = true
	}
	k.scope = m
}

// ScopeStaleAfter 是 scope 自动恢复的空闲阈值。
//
// 为什么需要自动恢复：scope 是全局的，一次临时设置会永久影响所有客户端
// （实测：设窄之后新会话也恢复不了，必须重启服务）。而它的用途只是
// 「省 token 的视图裁剪」，没有任何需要长期保持的理由——所以一旦
// 超过这个时长没人再动它，就自动还原为全部工具。
//
// 取 30 分钟：足够覆盖一次连续的 AI 会话（会话中间不会空闲半小时），
// 又不至于让一次误设永久生效。
const ScopeStaleAfter = 30 * time.Minute

// ScopeExpired 判断当前 scope 是否已空闲超时（该恢复为全部了）。
// 从未设置过 scope 时返回 false（无需恢复）。
func (k *Kernel) ScopeExpired(now time.Time) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.scope == nil {
		return false // 本来就是全部
	}
	return k.scopeTouched.IsZero() || now.Sub(k.scopeTouched) > ScopeStaleAfter
}

// RestoreScopeIfStale 若 scope 已空闲超时则把它清为 nil（恢复为全部工具），
// 返回是否发生了恢复。**重注册由调用方负责**（内核不依赖通道 SDK，见 Kernel 的说明）。
//
// 由通道适配器在每次请求之前调用：先拿到 true 就调 Reregister（mcp.go）。
// 放在请求路径上而非定时器上，是因为：
//   - 不引入后台 goroutine 与其生命周期管理；
//   - 恢复时机天然对齐「有人真的在用」，避免无人访问时反复重注册。
func (k *Kernel) RestoreScopeIfStale() bool {
	if !k.ScopeExpired(time.Now()) {
		return false
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.scope == nil {
		return false // 已被其它路径恢复
	}
	k.scope = nil
	return true
}

// track 记录已注册意图名（供适配器重注册时使用）。
func (k *Kernel) track(name string) {
	k.mu.Lock()
	k.reg = append(k.reg, name)
	k.mu.Unlock()
}

// RegisteredCount 返回当前已注册意图数量（含 meta 意图）。
func (k *Kernel) RegisteredCount() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.reg)
}

// IsRegistered 返回意图是否已注册到当前内核（含 meta 意图）。
// 用于「全部开放」断言：ExposedIntents=["*"] 时，所有意图（含 DefaultOff）都应被点亮。
func (k *Kernel) IsRegistered(name string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, n := range k.reg {
		if n == name {
			return true
		}
	}
	return false
}

// allowed 依据配置判定意图是否暴露（未知意图默认拒绝）。
func (k *Kernel) allowed(spec *IntentSpec) bool {
	// DefaultOff 意图必须在 ExposedIntents 里被显式点名才开放，
	// 不受"白名单为空即全开"的兜底规则影响。见 IntentSpec.DefaultOff 的说明。
	if spec.DefaultOff {
		if len(k.cfg.ExposedIntents) > 0 {
			for _, p := range k.cfg.ExposedIntents {
				if matchIntentPattern(p, spec) {
					return true
				}
			}
		}
		return false
	}
	if len(k.cfg.ExposedIntents) > 0 {
		for _, p := range k.cfg.ExposedIntents {
			if matchIntentPattern(p, spec) {
				return true
			}
		}
		return false
	}
	if len(k.cfg.ExposedTools) > 0 {
		for _, c := range spec.Caps {
			if matchGlob(k.cfg.ExposedTools, c) {
				return true
			}
		}
		return false
	}
	return true
}

func matchIntentPattern(p string, spec *IntentSpec) bool {
	if p == "*" {
		return true
	}
	if p == string(spec.Domain) || p == "domain:"+string(spec.Domain) {
		return true
	}
	if strings.HasSuffix(p, "*") {
		return strings.HasPrefix(spec.Name, strings.TrimSuffix(p, "*"))
	}
	return p == spec.Name
}

func matchGlob(patterns []string, name string) bool {
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "*") {
			if strings.HasPrefix(name, strings.TrimSuffix(p, "*")) {
				return true
			}
		} else if p == name {
			return true
		}
	}
	return false
}
