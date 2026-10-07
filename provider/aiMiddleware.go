package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"kandaoni.com/anqicms/pkg/mcp/intent"

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
	// BeforeDenyTurn P0-3: 终止**整轮**（不仅是这一个调用），对应 atomcode
	// BeforeOutcome::DenyTurn。用于「再继续下去只会更糟」的硬边界，
	// 例如熔断判定为死循环、或连续越权被拒。
	BeforeDenyTurn
)

// AfterOutcome 是 After 的后续决策，对应 atomcode AfterOutcome。
type AfterOutcome int

const (
	AfterProceed AfterOutcome = iota // 继续下一个 after middleware
	AfterStop                        // 终止本轮剩余工具 (已发生严重问题)
	// AfterBlock P0-3: 阻断并把**原因回灌给模型**，对应 atomcode
	// AfterOutcome::Block{reason}。与 AfterStop 的区别：Stop 只是不再跑后续
	// middleware，Block 会把 reason 作为 tool_result 交给模型，让它自我修正
	// （而不是拿到一个空结果或原始输出后继续猜）。
	AfterBlock
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
	// RootPath P0-1: 当前站点根目录 (各站点独立，见 provider/website.go)。
	// 路径安全门以它为锚判定「站内 / 站外」；为空则无法判定 → fail closed。
	RootPath string
	// PathClass / PathTargets 由 path_gate 写入的分类结果与解析出的目标路径，
	// 供后续 middleware 与链本身消费（例如决定授权键、是否可记住授权）。
	PathClass   PathClass
	PathTargets []string
	// AllowOnce 本会话授权存储 (主会话审批用)
	// P0-2: 由「整工具放行」改为「按目标键控」+ 可选「完全控制」
	AllowOnce *SessionAllowSet
	// ApprovalFn 审批回调，返回 ("allow"|"deny"|"once_allow"|"full_control", reason)
	// 主会话由 controller 注入 SSE 同步审批；Agent 会话为 nil。
	// P0-1: 传入 exec 本身，让前端能拿到路径分类/目标/原因（用于展示"为什么需要审批"
	// 以及是否该提供"完全控制"选项）。
	ApprovalFn func(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) (decision string, reason string)
	// DeniedReason 由 BeforeDeny 设置，After 可读取用于日志
	DeniedReason string
	// P2-7: 结构化追踪记录器（可为 nil，nil 时中间件不记录）
	Trace *TraceRecorder
	// PreApproved / PreDenied 回合级合并审批的结论（按 tool_call_id 索引）。
	//
	// 模型一轮里并发发起多个写调用时，逐个 Ask 会让确认弹窗串联刷屏（用户反馈的
	// 问题 1）。控制器改为「先扫整轮批次 → 一次 tool_confirm_batch 问完」，
	// 结论预先写进这两个 map，门禁只消费、不再各自弹窗。
	// nil 时行为与旧版一致（逐次 Ask）。
	PreApproved map[string]bool
	PreDenied   map[string]string
}

// IsPreApproved 该 tool_call 是否已被回合级审批放行。
func (e *ToolExecContext) IsPreApproved(callID string) bool {
	if e == nil || callID == "" {
		return false
	}
	return e.PreApproved[callID]
}

// PreDenyReason 该 tool_call 是否被回合级审批拒绝，以及拒绝原因。
func (e *ToolExecContext) PreDenyReason(callID string) (string, bool) {
	if e == nil || callID == "" || e.PreDenied == nil {
		return "", false
	}
	reason, ok := e.PreDenied[callID]
	return reason, ok
}

// ================================================================
// SessionAllowSet —— 会话级授权存储 (P0-2)
//
// 原实现是 map[toolName]bool：一旦"允许 edit_file"，该工具对**任意文件**的
// 后续调用全部放行 —— 用户只想改 A 文件，却放行了 B/C。仿 atomcode 改为：
//   - 按目标键控：key = 工具 + 解析出的目标路径（同一文件不同读取窗口共享授权，
//     不同文件/不同密钥各有各的授权）
//   - 完全控制 (full control)：本会话放行所有**非敏感**操作，是用户显式升级的逃生舱
//   - 敏感目标永不记住：即使开了完全控制也不覆盖敏感路径（fail closed）
//
// ================================================================
type SessionAllowSet struct {
	mu sync.RWMutex
	// grants key = GrantKey(toolName, targets, args)
	grants map[string]bool
	// fullControl 本会话「完全控制」：放行所有非敏感操作
	fullControl bool
}

func NewSessionAllowSet() *SessionAllowSet {
	return &SessionAllowSet{grants: make(map[string]bool)}
}

// GrantKey 计算「工具 + 目标」的授权键。
// 无目标时回落到原始参数（仿 atomcode grant_scope：不因缺少目标就放宽到整个工具，
// 那样等于把 once_allow 放大成整工具放行，正是要修掉的缺陷）。
func GrantKey(toolName string, targets []string, args string) string {
	if len(targets) == 0 {
		return toolName + "::" + args
	}
	return toolName + "::" + strings.Join(targets, "\x1f")
}

// ApprovalGrantKey 计算一次审批对应的会话级授权键，粒度按可靠性递减：
//  1. 路径目标 —— 文件类工具按「工具+文件」记住，改 A 文件不会放行 B 文件；
//  2. action —— 合并意图按「工具+动作」记住。整工具记会一次放行 save/delete，
//     整参数 JSON 记则同样内容的第二次保存又要问一遍（问题 1 的放大器）。
//     action 粒度既不放大权限也不重复打扰；
//  3. 原始参数 —— 两者都没有时回落（宁严勿松）。
func ApprovalGrantKey(toolName string, exec *ToolExecContext, argsJSON string) string {
	if exec != nil && len(exec.PathTargets) > 0 {
		return GrantKey(toolName, exec.PathTargets, argsJSON)
	}
	if action := actionOfArgsJSON(argsJSON); action != "" {
		return toolName + "::" + action
	}
	return GrantKey(toolName, nil, argsJSON)
}

// actionOfArgsJSON 取出参数里的 action 分派键（合并意图用它区分读/写动作）。
func actionOfArgsJSON(argsJSON string) string {
	s := strings.TrimSpace(argsJSON)
	if s == "" {
		return ""
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(s), &args); err != nil {
		return ""
	}
	return intent.ActionGrantSuffix(args)
}

// GrantTarget 记录「工具 + 目标」的本会话授权。
func (s *SessionAllowSet) GrantTarget(key string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.grants[key] = true
}

// IsTargetGranted 判断「工具 + 目标」是否已授权。
func (s *SessionAllowSet) IsTargetGranted(key string) bool {
	if key == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.grants[key]
}

// GrantFullControl 授予本会话「完全控制」（放行所有非敏感操作）。
func (s *SessionAllowSet) GrantFullControl() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fullControl = true
}

// HasFullControl 是否已授予本会话完全控制。
func (s *SessionAllowSet) HasFullControl() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fullControl
}

// ToolExecResult 是 After middleware 处理的结果对象。
type ToolExecResult struct {
	CallID    string
	ToolName  string
	Content   string
	IsError   bool
	Truncated bool // P2: result_truncator 标记结果被截断
	// TurnTerminated P0-3: 命中 BeforeDenyTurn —— 本轮应整体终止，
	// 调用方需跳出本轮剩余工具并把原因注入给模型。
	TurnTerminated bool
	// Blocked P0-3: 命中 AfterBlock —— Content 已被替换为阻断原因，
	// 该原因会作为 tool_result 回灌给模型让它自我修正。
	Blocked bool
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
			if exec.Trace != nil {
				exec.Trace.RecordGuardrail(mw.Name(), "allow", exec.DeniedReason)
			}
			goto execute
		case BeforeDeny:
			if exec.Trace != nil {
				exec.Trace.RecordGuardrail(mw.Name(), "deny", exec.DeniedReason)
			}
			return &ToolExecResult{
				CallID:   call.ID,
				ToolName: toolName,
				Content:  exec.DeniedReason,
				IsError:  true,
			}, true, exec.DeniedReason
		case BeforeDenyTurn:
			// P0-3: 终止整轮（atomcode DenyTurn）。调用方据此跳出本轮剩余工具。
			reason := exec.DeniedReason
			if reason == "" {
				reason = "操作已终止本轮执行"
			}
			if exec.Trace != nil {
				exec.Trace.RecordGuardrail(mw.Name(), "deny_turn", reason)
			}
			return &ToolExecResult{
				CallID:         call.ID,
				ToolName:       toolName,
				Content:        reason,
				IsError:        true,
				TurnTerminated: true,
			}, true, reason
		case BeforeAsk:
			// 审批门挂起：调用 ApprovalFn
			if exec.ApprovalFn == nil {
				// Agent 会话或无审批回调 → 默认放行 (Agent 不需要审批)
				continue
			}
			decision, reason := exec.ApprovalFn(ctx, call, exec)
			switch decision {
			case "deny":
				msg := "用户拒绝了此操作"
				if reason != "" {
					msg += ": " + reason
				}
				if exec.Trace != nil {
					exec.Trace.RecordGuardrail(mw.Name(), "deny", msg)
				}
				return &ToolExecResult{
					CallID:   call.ID,
					ToolName: toolName,
					Content:  msg,
					IsError:  true,
				}, true, msg
			case "allow":
				// 本次允许 (仅此一次)，继续走 before 链
				if exec.Trace != nil {
					exec.Trace.RecordGuardrail(mw.Name(), "allow", "用户单次允许")
				}
				continue
			case "full_control":
				// P0-2 新增：本会话「完全控制」—— 放行所有非敏感操作。
				// 敏感目标不在此列：path_gate 对 PathSensitive 始终返回 Ask，
				// 所以完全控制也无法预授权密钥读取（fail closed）。
				if exec.Trace != nil {
					exec.Trace.RecordGuardrail(mw.Name(), "full_control", "本会话完全控制")
				}
				if exec.AllowOnce != nil {
					exec.AllowOnce.GrantFullControl()
				}
				goto execute
			case "once_allow":
				// P0-2: 由「整工具放行」改为按「工具 + 目标」记住授权。
				// 敏感目标永不记住 → 自动退化为单次放行。
				if exec.Trace != nil {
					exec.Trace.RecordGuardrail(mw.Name(), "once_allow", "按目标键控授权")
				}
				if exec.AllowOnce != nil && exec.PathClass != PathSensitive {
					exec.AllowOnce.GrantTarget(ApprovalGrantKey(toolName, exec, call.Function.Arguments))
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
		switch outcome {
		case AfterBlock:
			// P0-3: 阻断并把原因回灌给模型（atomcode AfterOutcome::Block）。
			// 区别于 AfterStop：Stop 只是不再跑后续 middleware，模型拿到的还是
			// 原始输出；Block 会把 reason 变成 tool_result，让模型知道「为什么被拦」。
			if exec.Trace != nil {
				exec.Trace.RecordGuardrail(mw.Name(), "block", exec.DeniedReason)
			}
			if exec.DeniedReason != "" {
				execResult.Content = exec.DeniedReason
			}
			execResult.Blocked = true
			execResult.IsError = true
			return execResult, false, ""
		case AfterStop:
			return execResult, false, ""
		}
	}

	return execResult, false, ""
}

// ================================================================
// 1. WriteGateMiddleware (write-gate 审批门)
// 主会话: 需要审批的调用（write/destructive/system）在这里挂起
//   - 回合级合并审批已给出结论 → 直接放行 / 拒绝，不再弹窗
//   - 本会话已按「工具+目标+action」授权 → BeforeAllow 短路
//   - 否则 → BeforeAsk 挂起等审批
// Agent 会话: ApprovalFn 为 nil → BeforeProceed 直接放行
// ================================================================

type WriteGateMiddleware struct{}

func (m *WriteGateMiddleware) Name() string { return "write_gate" }

func (m *WriteGateMiddleware) Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome {
	toolName := exec.ToolName

	// action 粒度判定：合并意图（content_article 的 list/get/save/delete）里
	// 只读的动作不再打扰用户。
	if !CallNeedsApproval(toolName, call.Function.Arguments) {
		return BeforeProceed
	}

	// Agent 会话 (AiAgentChat/ExecuteAgent) 不需要审批
	if exec.IsAgent {
		return BeforeProceed
	}

	// 回合级合并审批的结论优先：控制器已就这一批统一问过用户。
	if exec.IsPreApproved(call.ID) {
		return BeforeAllow
	}
	if reason, denied := exec.PreDenyReason(call.ID); denied {
		if reason == "" {
			reason = "用户拒绝了此操作"
		}
		exec.DeniedReason = reason
		return BeforeDeny
	}

	if exec.AllowOnce == nil {
		return BeforeAsk
	}
	// 「完全控制」对无路径的站点写操作同样生效（此前只有路径门认它，
	// 于是选了完全控制后 CMS 写操作仍会逐个弹窗）。敏感目标除外——
	// 敏感路径由 path_gate 恒定返回 Ask，full_control 无法覆盖。
	if exec.AllowOnce.HasFullControl() && exec.PathClass != PathSensitive {
		return BeforeAllow
	}
	// 本会话已授权（工具+目标/action 粒度）→ 短路
	if exec.AllowOnce.IsTargetGranted(ApprovalGrantKey(toolName, exec, call.Function.Arguments)) {
		return BeforeAllow
	}

	// 主会话 + 需要审批 → 挂起审批
	return BeforeAsk
}

func (m *WriteGateMiddleware) After(ctx context.Context, result *ToolExecResult, exec *ToolExecContext) AfterOutcome {
	return AfterProceed
}

// ================================================================
// 1b. PathGateMiddleware (路径安全门)
// P0-1/P0-2: 取代原 SensitivePathGateMiddleware。
//
// 原实现只做「原始 JSON 子串匹配」，两个致命问题：
//   1. 无路径规范化 —— `a/../../etc/passwd`、符号链接都能绕过；
//   2. 授权是整工具级 —— 允许一次 edit_file 就放行了它对任意文件的写。
//
// 新实现（判定逻辑见 aiPathGate.go）：
//   - 先 filepath.Abs/Clean + EvalSymlinks 解析，再判定，堵住 `..` 与符号链接逃逸
//   - 锚定**当前站点 RootPath**（各站点独立）判定站内 / 站外
//   - 站内 + 非敏感 + 文件类工具 → 自动放行（作用域 = 目标路径，且底层
//     safePathResolve 已把所有文件操作硬约束在 projectRoot 内）
//   - 敏感 / 站外 / 无法判定 → 需审批；敏感目标的授权永不记住
//   - 命令执行类 (bash) 永不自动放行 —— 它的作用域不等于它提到的某个路径
// ================================================================

// PathGateMiddleware 路径安全门：解析目标路径 → 分类 → 决定放行 / 审批。
type PathGateMiddleware struct{}

func (m *PathGateMiddleware) Name() string { return "path_gate" }

func (m *PathGateMiddleware) Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome {
	// 意图模式下模型调用的是 fs_write / fs_edit / shell_exec 这类意图名，
	// 而路径分类的字段与规则按能力名（write_file / bash）定义。不解析这层委托关系，
	// 意图化的文件工具会整体跳过路径门——站外/敏感路径不再被识别，站内也拿不到
	// 「自动放行」，只能退化成逐个弹窗。
	toolName := pathGateCapName(exec.ToolName)
	if toolName == "" {
		return BeforeProceed
	}

	// Agent 会话不审批（保持既有行为）
	if exec.IsAgent {
		return BeforeProceed
	}

	root := exec.RootPath
	class, targets := ClassifyToolTargets(toolName, call.Function.Arguments, root)
	// 写回 exec 供链本身使用（授权键、是否可记住授权）
	exec.PathClass = class
	exec.PathTargets = targets

	// ── 已有授权 → 放行 ──
	// 注意：敏感目标**永不**因既有授权而放行（即使开了完全控制），
	// 这是 atomcode 的 fail-closed 语义：一次密钥授权不能覆盖另一个密钥。
	// 授权键用 exec.ToolName（模型看到的那个名字）而不是解析出的能力名，
	// 否则 write_gate 记的 `fs_write::…` 与这里查的 `write_file::…` 对不上。
	if exec.AllowOnce != nil && class != PathSensitive {
		if exec.AllowOnce.HasFullControl() {
			return BeforeAllow
		}
		if exec.AllowOnce.IsTargetGranted(ApprovalGrantKey(exec.ToolName, exec, call.Function.Arguments)) {
			return BeforeAllow
		}
	}

	// 回合级合并审批的结论同样要在这里消费：路径门常在写门之前返回 Ask，
	// 只让 write_gate 认批等于没批。敏感路径例外——它必须逐次确认。
	if class != PathSensitive && exec.IsPreApproved(call.ID) {
		return BeforeAllow
	}
	if reason, denied := exec.PreDenyReason(call.ID); denied {
		if reason == "" {
			reason = "用户拒绝了此操作"
		}
		exec.DeniedReason = reason
		return BeforeDeny
	}

	joined := strings.Join(targets, ", ")
	switch class {
	case PathInRoot:
		// 站内 + 非敏感。仅文件类工具自动放行；命令执行类交回 write_gate 继续审批。
		if ToolCanAutoApproveInRoot(toolName) {
			return BeforeAllow
		}
		return BeforeProceed

	case PathSensitive:
		exec.DeniedReason = fmt.Sprintf("敏感路径（需逐次确认，授权不会被记住）: %s", joined)
		return BeforeAsk

	case PathOutOfRoot:
		exec.DeniedReason = fmt.Sprintf("目标路径在站点目录（%s）之外: %s", root, joined)
		return BeforeAsk

	default: // PathUndeterminable
		// 无法判定归属 → fail closed。
		// 文件类工具必须问；命令执行类交回 write_gate（它本来就会问）。
		exec.DeniedReason = fmt.Sprintf("无法判定目标路径归属（按需要审批处理）: %s", joined)
		if ToolCanAutoApproveInRoot(toolName) {
			return BeforeAsk
		}
		return BeforeProceed
	}
}

func (m *PathGateMiddleware) After(ctx context.Context, result *ToolExecResult, exec *ToolExecContext) AfterOutcome {
	return AfterProceed
}

// ================================================================
// 审批预判 (ApprovalPreview) —— 回合级合并审批的地基
//
// 一轮里多个写调用各自 Ask，用户就会看到串联弹窗（问题 1）。控制器改为：
// 执行整轮之前先扫一遍批次，把要问的统一成一次 tool_confirm_batch。
//
// 这里的判定必须与 PathGateMiddleware / WriteGateMiddleware 的 Before 逐条对应
// （共用 CallNeedsApproval / ClassifyToolTargets / ApprovalGrantKey 同一套谓词），
// 否则预判漏掉的调用会在执行时再弹一次，合并就白做了。
// ================================================================

// ApprovalPreview 一次调用在执行前的审批预判结果。
type ApprovalPreview struct {
	ID        string // tool_call_id
	ToolName  string
	Title     string // 意图展示名（没有则为空，前端回落工具名）
	Arguments string
	Risk      string // read/write/destructive/system
	Needs     bool   // 会不会被门禁拦下
	Reason    string // 拦下的原因（展示用）

	PathClass        PathClass
	PathTargets      []string
	Sensitive        bool // 敏感目标：批了「完全控制」也不该记住授权
	GrantKey         string
	AllowFullControl bool
}

// Mutates 该调用是否会改变站点数据（写/删除/主机级）。
// 供调度使用：只读调用可并行，变更类调用串行执行。
func (p ApprovalPreview) Mutates() bool {
	return p.Risk != string(intent.RiskRead)
}

// PreviewCallApproval 预判一次调用是否需要人工审批。
// rootPath 为空时路径归属无法判定，按门禁的 fail-closed 语义处理。
func PreviewCallApproval(id, toolName, argsJSON, rootPath string, allow *SessionAllowSet) ApprovalPreview {
	p := ApprovalPreview{
		ID: id, ToolName: toolName, Arguments: argsJSON,
		Risk:             CallRisk(toolName, argsJSON),
		PathClass:        PathUndeterminable,
		AllowFullControl: true,
	}
	if spec, ok := intent.SpecByName(strings.TrimSpace(toolName)); ok {
		p.Title = spec.Title
	}

	gated := pathGateCapName(toolName)
	if gated == "" {
		// ── 无路径语义：只由 write_gate 决定 ──
		p.Needs = CallNeedsApproval(toolName, argsJSON)
		p.GrantKey = ApprovalGrantKey(toolName, nil, argsJSON)
		if p.Needs && alreadyGranted(allow, p.GrantKey) {
			p.Needs = false
		}
		return p
	}

	// ── 有路径语义：path_gate 先判，命中放行条件就整条链都放行 ──
	class, targets := ClassifyToolTargets(gated, argsJSON, rootPath)
	p.PathClass = class
	p.PathTargets = targets
	p.GrantKey = ApprovalGrantKey(toolName, &ToolExecContext{PathTargets: targets}, argsJSON)
	joined := strings.Join(targets, ", ")

	switch class {
	case PathInRoot:
		if ToolCanAutoApproveInRoot(gated) {
			return p // BeforeAllow 短路：门禁整体放行
		}
		p.Needs = CallNeedsApproval(toolName, argsJSON)
	case PathSensitive:
		p.Sensitive = true
		p.AllowFullControl = false
		p.Needs = true
		p.Reason = fmt.Sprintf("敏感路径（需逐次确认，授权不会被记住）: %s", joined)
		return p
	case PathOutOfRoot:
		p.Needs = true
		p.Reason = fmt.Sprintf("目标路径在站点目录（%s）之外: %s", rootPath, joined)
	default: // PathUndeterminable
		if ToolCanAutoApproveInRoot(gated) {
			p.Needs = true
			p.Reason = fmt.Sprintf("无法判定目标路径归属（按需要审批处理）: %s", joined)
		} else {
			p.Needs = CallNeedsApproval(toolName, argsJSON)
			p.Reason = "命令执行类工具：授权范围无法收敛到单个路径，需逐次确认"
		}
	}

	if p.Needs && alreadyGranted(allow, p.GrantKey) {
		p.Needs = false
	}
	return p
}

// alreadyGranted 会话级授权是否已覆盖这次调用。
// 敏感目标在上游已单独返回，不会走到这里 —— 它们永不记住授权。
func alreadyGranted(allow *SessionAllowSet, key string) bool {
	if allow == nil {
		return false
	}
	if allow.HasFullControl() {
		return true
	}
	return key != "" && allow.IsTargetGranted(key)
}

// ================================================================
// 2. ResultTruncatorMiddleware (统一截断超大结果)
// 仿 atomcode cap_tool_result: 执行后统一截断，避免超大 tool_result 撑爆上下文。
// 阈值 10000 字符 (与 aiChat.go 原有 SSE 截断一致)。
// ================================================================

type ResultTruncatorMiddleware struct {
	// MaxBytes 截断上限（字符数）。<=0 时回落 DefaultMaxToolResultBytes。
	// P1-6: 由调用方按 ChatTuning 注入，不再写死。
	MaxBytes int
}

// limit 返回生效的截断上限。
func (m *ResultTruncatorMiddleware) limit() int {
	if m.MaxBytes > 0 {
		return m.MaxBytes
	}
	return DefaultMaxToolResultBytes
}

func (m *ResultTruncatorMiddleware) Name() string { return "result_truncator" }

func (m *ResultTruncatorMiddleware) Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome {
	return BeforeProceed
}

func (m *ResultTruncatorMiddleware) After(ctx context.Context, result *ToolExecResult, exec *ToolExecContext) AfterOutcome {
	limit := m.limit()
	if len(result.Content) > limit {
		// 回退到 UTF-8 字符边界，避免把多字节字符（如中文）切成半个
		cut := limit
		for cut > 0 && !utf8.RuneStart(result.Content[cut]) {
			cut--
		}
		// 这是兜底，不是常规路径：分页类工具（read_file/grep/glob/api list）已按同一
		// 预算自行切窗并在结果里给出续读坐标，落到这里的都是没有坐标可给的结果
		// （bash 输出、网页正文、外部 API 响应）。标记必须写清原始规模与上限，模型
		// 才知道自己看到的只是前段、并去收窄查询。
		result.Content = result.Content[:cut] +
			fmt.Sprintf("\n\n[结果共 %d 字节，超出 %d 字节上限，尾部已丢弃且无法续读；"+
				"请缩小查询范围，或改用支持 offset 的工具]", len(result.Content), limit)
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
