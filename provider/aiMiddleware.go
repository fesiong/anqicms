package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

// ================================================================
// ToolMiddleware 层 (P0)
// 仿 atomcode kernel::ToolMiddleware:
//   - Before: 拦截/审批/改写参数 (本实现仅做审批门 Ask/Deny)
//   - After:  统一截断超大结果 + 统一脱敏
// 链式执行: before 按注册顺序，第一个 Deny 阻止；Allow 短路剩余 before。
// ================================================================

// BeforeOutcome 是 Before 的门禁决策，对应 atomcode BeforeOutcome。
type BeforeOutcome int

const (
	BeforeProceed BeforeOutcome = iota // 继续下一个 middleware / 正常执行
	BeforeAllow                        // 短路剩余 before，直接放行
	BeforeDeny                         // 阻止执行，返回 reason 作为 tool_result
	BeforeAsk                          // 挂起等待人工审批 (仅主会话)
)

// AfterOutcome 是 After 的后续决策，对应 atomcode AfterOutcome。
type AfterOutcome int

const (
	AfterProceed AfterOutcome = iota // 继续下一个 after middleware
	AfterStop                        // 终止本轮剩余工具 (已发生严重问题)
)

// ToolMiddleware 是围绕工具执行的 composable wrapper。
// before 在工具执行前运行 (参数已解析，工具已定位)；after 在工具执行后运行。
// 实现必须 not-panic: 要阻止调用请返回 BeforeDeny，不要 panic。
type ToolMiddleware interface {
	Name() string
	Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome
	After(ctx context.Context, result *ToolExecResult, exec *ToolExecContext) AfterOutcome
}

// ToolExecContext 是传递给 middleware 的执行上下文，携带审批回调。
type ToolExecContext struct {
	// SessionID 当前会话 ID
	SessionID string
	// IsAgent true 表示这是 Agent 自动执行 (AiAgentChat / ExecuteAgent)，跳过审批
	IsAgent bool
	// ToolName 工具名称 (便于 middleware 不必解析 call)
	ToolName string
	// AllowOnce 本会话已"本次允许"的工具集合 (主会话审批用)
	AllowOnce *SessionAllowSet
	// ApprovalFn 审批回调，返回 ("allow"|"deny"|"once_allow", reason)
	// 主会话由 controller 注入 SSE 同步审批；Agent 会话为 nil
	ApprovalFn func(ctx context.Context, call *schema.ToolCall, toolName string) (decision string, reason string)
	// DeniedReason 由 BeforeDeny 设置，After 可读取用于日志
	DeniedReason string
}

// SessionAllowSet 记录一个会话内已"本次允许"的工具，本会话允许后续相同工具不再拦截。
type SessionAllowSet struct {
	mu   sync.RWMutex
	sets map[string]bool // key = toolName (本会话允许该工具的所有后续调用)
}

func NewSessionAllowSet() *SessionAllowSet {
	return &SessionAllowSet{sets: make(map[string]bool)}
}

// Allow 标记某工具在本会话内已允许。
func (s *SessionAllowSet) Allow(toolName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets[toolName] = true
}

// IsAllowed 判断某工具是否已在本会话被允许。
func (s *SessionAllowSet) IsAllowed(toolName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sets[toolName]
}

// ToolExecResult 是 After middleware 处理的结果对象。
type ToolExecResult struct {
	CallID    string
	ToolName  string
	Content   string
	IsError   bool
	Truncated bool // P2: result_truncator 标记结果被截断
}

// MiddlewareChain 是有序 middleware 链，提供 Execute 方法。
type MiddlewareChain struct {
	middlewares []ToolMiddleware
}

// NewMiddlewareChain 创建链。顺序敏感: before 按注册顺序执行。
func NewMiddlewareChain(mws ...ToolMiddleware) *MiddlewareChain {
	return &MiddlewareChain{middlewares: mws}
}

// ExecuteTool 带中间件链执行一个工具调用。
// 返回 (execResult, isDenied, deniedReason)。
// execResult.Content 是最终工具结果（可能被截断/脱敏），execResult.Truncated 标记是否被截断。
func (c *MiddlewareChain) ExecuteTool(
	ctx context.Context,
	call *schema.ToolCall,
	handler toolHandler,
	exec *ToolExecContext,
) (*ToolExecResult, bool, string) {
	toolName := call.Function.Name
	exec.ToolName = toolName

	// ── Before 链 ──
	for _, mw := range c.middlewares {
		outcome := mw.Before(ctx, call, exec)
		switch outcome {
		case BeforeAllow:
			// 短路剩余 before，直接执行
			goto execute
		case BeforeDeny:
			return &ToolExecResult{
				CallID:   call.ID,
				ToolName: toolName,
				Content:  exec.DeniedReason,
				IsError:  true,
			}, true, exec.DeniedReason
		case BeforeAsk:
			// 审批门挂起：调用 ApprovalFn
			if exec.ApprovalFn == nil {
				// Agent 会话或无审批回调 → 默认放行 (Agent 不需要审批)
				continue
			}
			decision, reason := exec.ApprovalFn(ctx, call, toolName)
			switch decision {
			case "deny":
				msg := "用户拒绝了此操作"
				if reason != "" {
					msg += ": " + reason
				}
				return &ToolExecResult{
					CallID:   call.ID,
					ToolName: toolName,
					Content:  msg,
					IsError:  true,
				}, true, msg
			case "allow":
				// 本次允许 (仅此一次)，继续走 before 链
				continue
			case "once_allow":
				// 本会话允许，记录并短路
				if exec.AllowOnce != nil {
					exec.AllowOnce.Allow(toolName)
				}
				goto execute
			default:
				// 未知决策 → 安全起见拒绝
				return &ToolExecResult{
					CallID:   call.ID,
					ToolName: toolName,
					Content:  "审批决策未知，已拒绝",
					IsError:  true,
				}, true, "审批决策未知"
			}
		case BeforeProceed:
			// 继续下一个 middleware
			continue
		}
	}

execute:
	// ── 执行工具 ──
	var result string
	if handler == nil {
		result = fmt.Sprintf("错误：未知工具 %s", toolName)
	} else {
		r, err := handler(ctx, call.Function.Arguments)
		if err != nil {
			result = fmt.Sprintf("工具执行错误: %s", err.Error())
		} else {
			result = r
		}
	}

	// ── After 链 ──
	execResult := &ToolExecResult{
		CallID:   call.ID,
		ToolName: toolName,
		Content:  result,
		IsError:  strings.HasPrefix(result, "错误") || strings.HasPrefix(result, "工具执行错误"),
	}
	for _, mw := range c.middlewares {
		outcome := mw.After(ctx, execResult, exec)
		if outcome == AfterStop {
			break
		}
	}

	return execResult, false, ""
}

// ================================================================
// 1. WriteGateMiddleware (write-gate 审批门)
// 主会话: HasWriteOperation 工具需要审批
//   - 若本会话已 allow 该工具 → BeforeAllow 短路
//   - 否则 → BeforeAsk 挂起等审批
// Agent 会话: ApprovalFn 为 nil → BeforeProceed 直接放行
// 对话中明确要求允许的工具 (由 ApprovalFn 返回 "allow") → 不拦截
// ================================================================

type WriteGateMiddleware struct{}

func (m *WriteGateMiddleware) Name() string { return "write_gate" }

func (m *WriteGateMiddleware) Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome {
	toolName := exec.ToolName

	// 只对写操作工具做审批
	if !HasWriteOperation([]string{toolName}) {
		return BeforeProceed
	}

	// Agent 会话 (AiAgentChat/ExecuteAgent) 不需要审批
	if exec.IsAgent {
		return BeforeProceed
	}

	// 本会话已允许该工具 → 短路
	if exec.AllowOnce != nil && exec.AllowOnce.IsAllowed(toolName) {
		return BeforeAllow
	}

	// 主会话 + 写操作 → 挂起审批
	return BeforeAsk
}

func (m *WriteGateMiddleware) After(ctx context.Context, result *ToolExecResult, exec *ToolExecContext) AfterOutcome {
	return AfterProceed
}

// ================================================================
// 1b. SensitivePathGateMiddleware (敏感路径审批门)
// 仿 atomcode sensitive path gate:
//   - edit_file/write_file/read_file 命中 config.toml/*.env/.git/ 时返回 BeforeAsk
//   - bash 命令命中敏感路径时返回 BeforeAsk
//   - Agent 会话跳过 (IsAgent=true)
// ================================================================

// gateSensitivePathPatterns 敏感路径匹配模式 (小写匹配)
// 与 aiBuiltinTools_helpers.go 的 sensitivePathPatterns (系统路径) 不同，
// 这里关注的是配置文件、密钥等敏感文件
var gateSensitivePathPatterns = []string{
	"config.toml",
	"config.yaml",
	"config.json",
	".env",
	".git/",
	".git\\",
	"secrets",
	"credentials",
	"private.key",
	"id_rsa",
	"wp-config.php",
	"database.yml",
}

// gateSensitiveTools 需要做敏感路径检查的工具
var gateSensitiveTools = map[string]bool{
	"edit_file":      true,
	"write_file":     true,
	"read_file":      true,
	"bash":           true,
	"search_replace": true,
}

// extractPathFromArgs 从工具参数 JSON 中提取路径相关字段。
// edit_file/write_file/read_file/search_replace: file_path
// bash: command (从中提取路径)
func extractPathFromArgs(toolName, args string) string {
	// 简单 JSON 字段提取，避免引入 encoding/json 的开销
	argsLower := strings.ToLower(args)

	if toolName == "bash" {
		// 从 command 字段提取
		idx := strings.Index(argsLower, "command")
		if idx < 0 {
			return ""
		}
		// 找到 command 后的值
		rest := args[idx:]
		// 提取引号内的内容
		start := strings.IndexAny(rest, "\"'")
		if start < 0 {
			return ""
		}
		quote := rest[start]
		end := strings.IndexByte(rest[start+1:], quote)
		if end < 0 {
			return ""
		}
		return rest[start+1 : start+1+end]
	}

	// file_path / path / old_string / new_string 中提取路径
	for _, field := range []string{"file_path", "path", "search", "glob"} {
		idx := strings.Index(argsLower, "\""+field+"\"")
		if idx < 0 {
			continue
		}
		rest := args[idx:]
		// 找到值
		colon := strings.IndexByte(rest, ':')
		if colon < 0 {
			continue
		}
		afterColon := rest[colon+1:]
		// 跳过空白
		trimmed := strings.TrimLeft(afterColon, " \t\n\r")
		if len(trimmed) == 0 {
			continue
		}
		// 提取引号内的内容
		if trimmed[0] == '"' || trimmed[0] == '\'' {
			quote := trimmed[0]
			end := strings.IndexByte(trimmed[1:], quote)
			if end < 0 {
				continue
			}
			return trimmed[1 : 1+end]
		}
	}

	return ""
}

// isSensitivePath 判断路径是否命中敏感模式。
func isSensitivePath(path string) bool {
	if path == "" {
		return false
	}
	pathLower := strings.ToLower(path)
	for _, pattern := range gateSensitivePathPatterns {
		if strings.Contains(pathLower, pattern) {
			return true
		}
	}
	return false
}

type SensitivePathGateMiddleware struct{}

func (m *SensitivePathGateMiddleware) Name() string { return "sensitive_path_gate" }

func (m *SensitivePathGateMiddleware) Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome {
	toolName := exec.ToolName

	// 只对敏感工具做检查
	if !gateSensitiveTools[toolName] {
		return BeforeProceed
	}

	// Agent 会话不需要审批
	if exec.IsAgent {
		return BeforeProceed
	}

	// 本会话已允许该工具 → 短路
	if exec.AllowOnce != nil && exec.AllowOnce.IsAllowed(toolName) {
		return BeforeAllow
	}

	// 从参数中提取路径
	path := extractPathFromArgs(toolName, call.Function.Arguments)

	// 命中敏感路径 → 挂起审批
	if isSensitivePath(path) {
		exec.DeniedReason = fmt.Sprintf("敏感路径: %s", path)
		return BeforeAsk
	}

	return BeforeProceed
}

func (m *SensitivePathGateMiddleware) After(ctx context.Context, result *ToolExecResult, exec *ToolExecContext) AfterOutcome {
	return AfterProceed
}

// ================================================================
// 2. ResultTruncatorMiddleware (统一截断超大结果)
// 仿 atomcode cap_tool_result: 执行后统一截断，避免超大 tool_result 撑爆上下文。
// 阈值 10000 字符 (与 aiChat.go 原有 SSE 截断一致)。
// ================================================================

const maxToolResultBytes = 10000

type ResultTruncatorMiddleware struct{}

func (m *ResultTruncatorMiddleware) Name() string { return "result_truncator" }

func (m *ResultTruncatorMiddleware) Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome {
	return BeforeProceed
}

func (m *ResultTruncatorMiddleware) After(ctx context.Context, result *ToolExecResult, exec *ToolExecContext) AfterOutcome {
	if len(result.Content) > maxToolResultBytes {
		// 回退到 UTF-8 字符边界，避免把多字节字符（如中文）切成半个
		cut := maxToolResultBytes
		for cut > 0 && !utf8.RuneStart(result.Content[cut]) {
			cut--
		}
		result.Content = result.Content[:cut] + "\n... [结果已截断]"
		result.Truncated = true
	}
	return AfterProceed
}

// ================================================================
// 3. RedactionMiddleware (统一脱敏)
// 对工具结果中的密码、Token、API Key、私钥等敏感信息做掩码。
// 仿 atomcode display-only redaction 原则: 只对结果脱敏，不修改可执行参数。
// ================================================================

type RedactionMiddleware struct{}

func (m *RedactionMiddleware) Name() string { return "redaction" }

func (m *RedactionMiddleware) Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome {
	return BeforeProceed
}

func (m *RedactionMiddleware) After(ctx context.Context, result *ToolExecResult, exec *ToolExecContext) AfterOutcome {
	result.Content = redactSecrets(result.Content)
	return AfterProceed
}

// ================================================================
// Pending approval registry (P0 审批流后端支持)
// 主会话 write_gate 挂起时，由 controller 注册一个 decision channel；
// 前端 POST /ai/chat/confirm 唤醒该 channel，完成同步审批闭环。
// ================================================================

// RegisterPendingApproval 注册一个待审批的工具调用，返回 decision channel。
// 调用方阻塞读取 channel 获取前端审批决策 ("allow"|"deny"|"once_allow")。
func (svc *AiChatService) RegisterPendingApproval(toolCallID string) chan string {
	ch := make(chan string, 1)
	svc.pendingApprovalsMu.Lock()
	svc.pendingApprovals[toolCallID] = ch
	svc.pendingApprovalsMu.Unlock()
	return ch
}

// ResolvePendingApproval 唤醒一个挂起的审批，返回 false 表示无此 pending。
func (svc *AiChatService) ResolvePendingApproval(toolCallID, decision string) bool {
	svc.pendingApprovalsMu.Lock()
	ch, ok := svc.pendingApprovals[toolCallID]
	if ok {
		delete(svc.pendingApprovals, toolCallID)
	}
	svc.pendingApprovalsMu.Unlock()
	if !ok {
		return false
	}
	ch <- decision
	return true
}

// CancelPendingApproval 取消一个挂起的审批 (如会话中止)，返回 false 表示无此 pending。
func (svc *AiChatService) CancelPendingApproval(toolCallID string) bool {
	svc.pendingApprovalsMu.Lock()
	ch, ok := svc.pendingApprovals[toolCallID]
	if ok {
		delete(svc.pendingApprovals, toolCallID)
	}
	svc.pendingApprovalsMu.Unlock()
	if !ok {
		return false
	}
	ch <- "deny"
	return true
}

// redactSecrets 对文本中的常见敏感模式做掩码处理。
// 保守策略: 只匹配高置信度模式，避免误伤正常内容。
var (
	// API Key / Token: 长度 >= 32 的字母数字字符串，常见前缀 sk-/pk-/token-/key-
	apiKeyPattern = regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,}|pk_[a-zA-Z0-9]{20,}|gl-[a-zA-Z0-9]{20,}|token[\s:=]+["']?[a-zA-Z0-9_\-]{32,}["']?|api[_-]?key[\s:=]+["']?[a-zA-Z0-9_\-]{32,}["']?)`)

	// Bearer Token
	bearerPattern = regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_\-\.]{32,}`)

	// 私钥 PEM 块
	privateKeyPattern = regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |PGP )?PRIVATE KEY-----[\s\S]*?-----END (RSA |EC |OPENSSH |PGP )?PRIVATE KEY-----`)

	// password = "xxx" / passwd: xxx (常见配置/数据库连接串)
	// 仅匹配等号/冒号后紧跟引号或非空白字符串，长度 6+
	passwordPattern = regexp.MustCompile(`(?i)(password|passwd|pwd|secret)[\s:=]+["']?[^\s"']{6,}["']?`)
)

func redactSecrets(content string) string {
	// PEM 私钥整块掩码
	content = privateKeyPattern.ReplaceAllString(content, "[REDACTED:PRIVATE KEY]")
	// Bearer Token
	content = bearerPattern.ReplaceAllStringFunc(content, func(s string) string {
		return "Bearer [REDACTED]"
	})
	// API Key / Token
	content = apiKeyPattern.ReplaceAllStringFunc(content, func(s string) string {
		return "[REDACTED]"
	})
	// password = xxx
	content = passwordPattern.ReplaceAllStringFunc(content, func(s string) string {
		// 保留键名，只掩码值
		lower := strings.ToLower(s)
		var key string
		switch {
		case strings.Contains(lower, "password"):
			key = "password"
		case strings.Contains(lower, "passwd"):
			key = "passwd"
		case strings.Contains(lower, "pwd"):
			key = "pwd"
		case strings.Contains(lower, "secret"):
			key = "secret"
		default:
			key = "secret"
		}
		return key + "=[REDACTED]"
	})
	return content
}

// ================================================================
// 4. CircuitBreakerMiddleware (重复工具调用熔断)
// 仿 atomcode round_tool_signature + MAX_REPEAT_ROUNDS
//
// 在主循环中维护 round_tool_signature（对 roundToolCalls 按
// name+arguments 排序后拼接），用 map[string]int 计数。
//   - 达到 REPEAT_NUDGE_AT=3 注入提示，让 AI 换策略但继续执行
//   - 达到 MAX_REPEAT_ROUNDS=6 终止并返回最后响应
//   - 与现有 consecutiveReads 正交，互不干扰
//
// 注意：此 middleware 不在 Before/After 链中工作，因为它是整轮级别的
// 检测，不是单工具调用级别。主循环在每轮结束后调用 RecordRound 和
// CheckRepeat 来决定是否注入 nudge 或终止。
// ================================================================

const (
	// REPEAT_NUDGE_AT: 同一 round_tool_signature 重复达到此次数时注入 nudge
	RepeatNudgeAt = 3
	// MAX_REPEAT_ROUNDS: 同一 round_tool_signature 重复达到此次数时终止
	MaxRepeatRounds = 6
	// maxTotalToolCalls 单轮会话工具调用总上限，防止无限循环
	maxTotalToolCalls = 50
)

// CircuitBreakerMiddleware 记录每个会话的整轮工具调用签名，检测死循环。
type CircuitBreakerMiddleware struct {
	mu sync.Mutex
	// key = sessionID, value = *circuitBreakerState
	states map[string]*circuitBreakerState
}

type circuitBreakerState struct {
	// totalCalls 该会话工具调用总数
	totalCalls int
	// roundSignatures 记录每轮的 round_tool_signature 及其出现次数
	roundSignatures map[string]int
	// lastSignature 上一次的整轮签名
	lastSignature string
	// consecutiveSameSignature 同一整轮签名连续重复计数
	consecutiveSameSignature int
}

func NewCircuitBreakerMiddleware() *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		states: make(map[string]*circuitBreakerState),
	}
}

func (m *CircuitBreakerMiddleware) Name() string { return "circuit_breaker" }

// BuildRoundSignature 构建整轮工具调用的签名。
// 对 roundToolCalls 按 name+arguments 排序后拼接，确保调用顺序不影响签名。
func BuildRoundSignature(toolCalls []schema.ToolCall) string {
	if len(toolCalls) == 0 {
		return ""
	}
	// 提取 name+arguments 对
	type pair struct {
		name string
		args string
	}
	pairs := make([]pair, len(toolCalls))
	for i, tc := range toolCalls {
		pairs[i] = pair{tc.Function.Name, tc.Function.Arguments}
	}
	// 按 name+args 排序
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].name+pairs[i].args < pairs[j].name+pairs[j].args
	})
	// 拼接
	var sb strings.Builder
	for i, p := range pairs {
		if i > 0 {
			sb.WriteString("|")
		}
		sb.WriteString(p.name)
		sb.WriteString(":")
		sb.WriteString(hashArgs(p.args))
	}
	return sb.String()
}

// hashArgs 对工具参数做简单 hash，用于检测"相同参数重复调用"。
// 不用 crypto hash 是因为这里只需快速比较，不需防碰撞。
func hashArgs(args string) string {
	if len(args) == 0 {
		return ""
	}
	// FNV-1a 32-bit
	var h uint32 = 2166136261
	for i := 0; i < len(args); i++ {
		h ^= uint32(args[i])
		h *= 16777619
	}
	return fmt.Sprintf("%x", h)
}

// Before 在单工具调用级别检查总调用上限。
// 整轮级别的重复检测在主循环中通过 RecordRound/CheckRepeat 处理。
func (m *CircuitBreakerMiddleware) Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[exec.SessionID]
	if !exists {
		state = &circuitBreakerState{
			roundSignatures: make(map[string]int),
		}
		m.states[exec.SessionID] = state
	}

	// 检查总调用上限
	if state.totalCalls >= maxTotalToolCalls {
		exec.DeniedReason = fmt.Sprintf(
			"熔断: 本会话工具调用总数已达上限 %d 次，疑似陷入死循环。"+
				"请停止调用工具，基于已有信息给出最终结论或换一种思路。",
			maxTotalToolCalls,
		)
		return BeforeDeny
	}

	return BeforeProceed
}

// After 在单工具调用级别更新总调用计数。
func (m *CircuitBreakerMiddleware) After(ctx context.Context, result *ToolExecResult, exec *ToolExecContext) AfterOutcome {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[exec.SessionID]
	if !exists {
		state = &circuitBreakerState{
			roundSignatures: make(map[string]int),
		}
		m.states[exec.SessionID] = state
	}
	state.totalCalls++

	return AfterProceed
}

// RecordRound 记录一轮工具调用的签名，返回当前签名连续重复的次数。
// 主循环在每轮工具调用结束后调用此方法。
func (m *CircuitBreakerMiddleware) RecordRound(sessionID string, signature string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[sessionID]
	if !exists {
		state = &circuitBreakerState{
			roundSignatures: make(map[string]int),
		}
		m.states[sessionID] = state
	}

	if signature == state.lastSignature {
		state.consecutiveSameSignature++
	} else {
		state.consecutiveSameSignature = 1
	}
	state.lastSignature = signature
	state.roundSignatures[signature]++

	return state.consecutiveSameSignature
}

// CheckRepeat 检查重复签名是否达到阈值，返回应采取的行动。
// 返回值:
//   - "proceed": 未达到阈值，继续执行
//   - "nudge": 达到 REPEAT_NUDGE_AT，注入 nudge 提示
//   - "terminate": 达到 MAX_REPEAT_ROUNDS，终止并返回最后响应
func (m *CircuitBreakerMiddleware) CheckRepeat(sessionID string, signature string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[sessionID]
	if !exists {
		return "proceed"
	}

	count := state.roundSignatures[signature]
	if count >= MaxRepeatRounds {
		return "terminate"
	}
	if count >= RepeatNudgeAt {
		return "nudge"
	}
	return "proceed"
}

// ResetCircuitBreaker 清空指定会话的熔断状态 (新一轮 AI 响应开始时调用)。
func (m *CircuitBreakerMiddleware) ResetCircuitBreaker(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, sessionID)
}
