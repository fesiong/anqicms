package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"kandaoni.com/anqicms/pkg/ai/eino"
)

// ================================================================
// P7: Subagent / Team 并行调度 (仿 atomcode `task` tool + subagent tiers)
//
// 研究报告要求:
//   1. 新增 `task` 工具: 模型可派发 explore (只读) 或 worker (可写) 子任务
//   2. 子任务通过 goroutine 并行执行, 结果汇总到主对话
//   3. 安全约束: worker 子 agent 的 scope 必须声明且非重叠
// ================================================================

// SubagentType 子 agent 类型
type SubagentType string

const (
	SubagentExplore SubagentType = "explore" // 只读子 agent (调查/搜索)
	SubagentWorker  SubagentType = "worker"  // 可写子 agent (实现/修改)
)

// SubagentTask 描述一个子任务
type SubagentTask struct {
	ID          string        // 子任务唯一 ID
	Description string        // 任务描述 (3-5 词)
	Prompt      string        // 子任务完整 prompt
	Type        SubagentType  // explore / worker
	Scope       []string      // worker 允许写的文件 scope (globs)
	MaxRounds   int           // 子 agent 最大轮次
	Timeout     time.Duration // 子 agent 超时
}

// SubagentResult 子任务执行结果
type SubagentResult struct {
	TaskID      string
	Description string
	Type        SubagentType
	Success     bool
	Output      string // 子 agent 的最终回复
	Error       string
	Duration    time.Duration
}

// SubagentManager 管理子任务的派发、并行执行和 scope 校验
type SubagentManager struct {
	mu sync.Mutex
	// activeScopes 当前活跃 worker 的 scope 集合，用于非重叠校验
	activeScopes map[string]bool // key = scope glob
	// results 已完成的子任务结果
	results []*SubagentResult
}

// NewSubagentManager 创建子任务管理器
func NewSubagentManager() *SubagentManager {
	return &SubagentManager{
		activeScopes: make(map[string]bool),
	}
}

// ValidateScope 校验 worker 子 agent 的 scope 是否与现有活跃 scope 重叠。
// 返回 nil 表示通过，返回 error 说明 scope 冲突。
func (m *SubagentManager) ValidateScope(scope []string) error {
	if len(scope) == 0 {
		return fmt.Errorf("worker 子任务必须声明 scope (允许写的文件范围)")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range scope {
		if m.activeScopes[s] {
			return fmt.Errorf("scope %s 与正在执行的 worker 子任务重叠，请等待其完成或换一个 scope", s)
		}
	}
	return nil
}

// ReserveScope 占用 worker 的 scope（调用方已通过 ValidateScope）
func (m *SubagentManager) ReserveScope(scope []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range scope {
		m.activeScopes[s] = true
	}
}

// ReleaseScope 释放 worker 的 scope
func (m *SubagentManager) ReleaseScope(scope []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range scope {
		delete(m.activeScopes, s)
	}
}

// AddResult 记录已完成的子任务结果
func (m *SubagentManager) AddResult(r *SubagentResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, r)
}

// GetResults 返回所有子任务结果
func (m *SubagentManager) GetResults() []*SubagentResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*SubagentResult, len(m.results))
	copy(out, m.results)
	return out
}

// ================================================================
// Subagent 执行逻辑
// ================================================================

// ExecuteSubagent 执行一个子 agent 任务。
//
// 子 agent 拥有独立上下文，通过非流式 LLM 调用 + 工具循环完成任务。
// explore 子 agent 只能调用只读工具；worker 子 agent 可写但限定在 scope 内。
func (svc *AiChatService) ExecuteSubagent(ctx context.Context, task *SubagentTask) *SubagentResult {
	start := time.Now()
	result := &SubagentResult{
		TaskID:      task.ID,
		Description: task.Description,
		Type:        task.Type,
	}

	// 设置超时
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// 获取 Eino client
	client, err := eino.GetClient()
	if err != nil {
		result.Error = fmt.Sprintf("AI client not available: %v", err)
		result.Duration = time.Since(start)
		RecordAIError("subagent[%s] AI client 不可用: %v", task.ID, err)
		return result
	}

	// 构建子 agent 的工具集
	subTools, subHandlers := svc.buildSubagentTools(task.Type, task.Scope)
	if len(subTools) > 0 {
		if err := client.BindTools(subTools); err != nil {
			result.Error = fmt.Sprintf("failed to bind tools: %v", err)
			result.Duration = time.Since(start)
			RecordAIError("subagent[%s] 绑定工具失败: %v", task.ID, err)
			return result
		}
	}

	// 构建系统提示
	systemPrompt := buildSubagentSystemPrompt(task)

	// 构建消息: 系统 + 任务
	messages := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(task.Prompt),
	}

	maxRounds := task.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 10 // 子 agent 默认最多 10 轮
	}

	var finalResponse string

	for round := 0; round < maxRounds; round++ {
		select {
		case <-ctx.Done():
			result.Error = "子任务超时或被取消"
			result.Duration = time.Since(start)
			RecordAIError("subagent[%s] 超时或被取消", task.ID)
			return result
		default:
		}

		// 非流式调用 LLM
		msg, err := client.Generate(ctx, messages)
		if err != nil {
			if IsContextOverflowError(err) {
				messages = CompactMessages(messages, 3)
				continue
			}
			result.Error = fmt.Sprintf("AI generate failed: %v", err)
			result.Duration = time.Since(start)
			RecordAIError("subagent[%s] AI 生成失败: %v", task.ID, err)
			return result
		}

		// 无工具调用 → 最终回复
		if len(msg.ToolCalls) == 0 {
			finalResponse = msg.Content
			break
		}

		messages = append(messages, msg)

		// 执行每个工具
		for _, tc := range msg.ToolCalls {
			toolName := tc.Function.Name
			argsJSON := tc.Function.Arguments

			handler, exists := subHandlers[toolName]
			var toolResult string
			if !exists {
				toolResult = fmt.Sprintf("错误：子 agent 无权调用工具 %s", toolName)
			} else {
				toolResult, err = handler(ctx, argsJSON)
				if err != nil {
					toolResult = fmt.Sprintf("工具执行错误: %s", err.Error())
				}
			}

			messages = append(messages, schema.ToolMessage(toolResult, tc.ID))
		}

		// 上下文压缩
		if len(messages) > 10 && round%3 == 2 {
			messages = CompactMessages(messages, 3)
		}
	}

	if finalResponse == "" {
		finalResponse = "子任务执行完毕，但未生成总结。"
	}

	result.Success = true
	result.Output = finalResponse
	result.Duration = time.Since(start)
	return result
}

// buildSubagentTools 根据子 agent 类型构建受限工具集。
//
// explore: 只包含只读工具 (HasWriteOperation 返回 false 的工具)
// worker:  包含只读工具 + 写工具，但写工具的执行受 scope 约束
func (svc *AiChatService) buildSubagentTools(subType SubagentType, scope []string) ([]*schema.ToolInfo, map[string]toolHandler) {
	filteredTools := make([]*schema.ToolInfo, 0)
	filteredHandlers := make(map[string]toolHandler)

	for _, ti := range svc.Tools {
		isWrite := HasWriteOperation([]string{ti.Name})

		// explore 子 agent 只能用只读工具
		if subType == SubagentExplore && isWrite {
			continue
		}

		filteredTools = append(filteredTools, ti)

		// worker 子 agent 的写工具需要 scope 校验
		if subType == SubagentWorker && isWrite {
			originalHandler := svc.Handlers[ti.Name]
			scopedHandler := func(ctx context.Context, argsJSON string) (string, error) {
				// 简单 scope 校验: 检查 argsJSON 是否包含 scope 中的路径
				// 实际生产中可解析 file_path 字段做精确校验
				if !isWithinScope(argsJSON, scope) {
					return "错误：操作超出声明的 scope，被拒绝", nil
				}
				return originalHandler(ctx, argsJSON)
			}
			filteredHandlers[ti.Name] = scopedHandler
		} else {
			filteredHandlers[ti.Name] = svc.Handlers[ti.Name]
		}
	}

	return filteredTools, filteredHandlers
}

// isWithinScope 检查工具调用的参数是否落在声明的 scope (一组 glob 模式) 内。
//
// 相对旧版纯子串匹配的安全改进:
//  1. 解析 JSON 参数，优先从 file_path / path / file / target / dir / directory
//     等字段抽取路径候选，避免把无关文本里的子串误判为命中；
//  2. 对候选路径做目录边界感知的 glob 匹配 (支持 * 与 **)，杜绝
//     scope="/a/b/" 命中 "/a/bc/x" 这类越界；
//  3. 未声明 scope 的写工具一律拒绝 (fail-closed)。
//
// 若解析不出任何路径字段，则回退到"对原始参数做子串匹配"作为兜底，
// 以免误伤那些把路径放在非标准字段的工具 (保持旧行为)。
func isWithinScope(argsJSON string, scope []string) bool {
	// 未声明 scope 的写操作 → 拒绝
	if len(scope) == 0 {
		return false
	}

	// 1) 解析已知路径字段
	candidates := extractPathCandidates(argsJSON)

	// 2) 有候选路径时，要求至少一个候选命中任一 scope (命中即放行，否则拒绝)
	if len(candidates) > 0 {
		for _, cand := range candidates {
			for _, s := range scope {
				if matchScopeGlob(s, cand) {
					return true
				}
			}
		}
		return false
	}

	// 3) 兜底：解析不出明确路径字段时，回退到原始参数子串匹配
	argsLower := strings.ToLower(argsJSON)
	for _, s := range scope {
		sLower := strings.ToLower(s)
		if strings.Contains(argsLower, sLower) {
			return true
		}
		if strings.Contains(sLower, "*") {
			prefix := strings.Split(sLower, "*")[0]
			if prefix != "" && strings.Contains(argsLower, prefix) {
				return true
			}
		}
	}
	return false
}

// extractPathCandidates 从工具参数 JSON 中抽取可能的文件路径候选。
// 优先读取常见路径字段；若没有，则扫描所有字符串值，收集绝对路径 (以 / 开头)。
// 解析失败时返回 nil (交由调用方兜底)。
func extractPathCandidates(argsJSON string) []string {
	var raw map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
		// 不是合法 JSON：返回 nil，调用方按原始文本兜底
		return nil
	}

	knownKeys := []string{"file_path", "path", "file", "target", "dir", "directory", "src", "dest", "destination", "url"}
	var candidates []string
	seen := make(map[string]bool)

	add := func(v any) {
		s, ok := v.(string)
		if !ok || s == "" {
			return
		}
		s = strings.TrimSpace(s)
		cleaned := path.Clean(s)
		key := strings.ToLower(cleaned)
		if seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, cleaned)
	}

	for _, k := range knownKeys {
		if v, ok := raw[k]; ok {
			add(v)
		}
	}

	// 扫描所有字符串值，收集绝对路径 (兜底，避免漏掉非标准字段里的路径)
	for _, v := range raw {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "/") {
			add(s)
		}
	}

	return candidates
}

// matchScopeGlob 判断候选路径 cand 是否匹配 scope glob 模式。
// 支持 * (单层) 与 ** (跨层)。匹配在路径规范化并小写化后进行，
// 且对前缀/后缀做目录边界约束，防止 "/a/b/" 误中 "/a/bc/x"。
func matchScopeGlob(pattern, cand string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	cand = strings.ToLower(path.Clean(cand))
	if pattern == "" {
		return false
	}
	if pattern == "**" || pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, cand)
	}

	// 无 ** 的普通通配：先整体匹配
	if ok, err := path.Match(pattern, cand); err == nil && ok {
		return true
	}
	// 再尝试基准名匹配 (如 *.go 匹配任意目录下的 .go 文件)
	if ok, err := path.Match(pattern, path.Base(cand)); err == nil && ok {
		return true
	}
	// 最后兜底：pattern 不含 * 时，视为路径片段出现
	if !strings.Contains(pattern, "*") {
		return containsPathSegment(cand, pattern)
	}
	return false
}

// matchDoubleStar 处理含 ** 的模式：prefix**suffix 形式。
func matchDoubleStar(pattern, cand string) bool {
	parts := strings.Split(pattern, "**")
	prefix := parts[0]
	suffix := parts[len(parts)-1]
	if prefix != "" && !containsPathSegment(cand, prefix) {
		return false
	}
	if suffix != "" && !endsWithPathSegment(cand, suffix) {
		return false
	}
	return true
}

// containsPathSegment 判断 frag 是否作为完整路径片段出现在 haystack 中
// (边界为 / 或字符串两端)，避免子串误中。frag 可带尾斜杠 (表示目录)，
// 此时其子内容也视为命中。
func containsPathSegment(haystack, frag string) bool {
	if frag == "" {
		return true
	}
	dirFrag := strings.HasSuffix(frag, "/")
	searchFrom := 0
	for {
		idx := strings.Index(haystack[searchFrom:], frag)
		if idx < 0 {
			return false
		}
		abs := searchFrom + idx
		before := abs == 0 || haystack[abs-1] == '/'
		// 片段后必须是 / 或字符串结尾；若片段以 / 结尾 (目录)，其子内容自然合法
		after := abs+len(frag) == len(haystack) || haystack[abs+len(frag)] == '/' || dirFrag
		if before && after {
			return true
		}
		searchFrom = abs + 1
	}
}

// endsWithPathSegment 判断 haystack 是否以 frag 结尾 (frag 可为通配，如 /*.go)。
// 采用 basename 通配匹配：把 suffix 最后一段作为 basename 模式与 haystack 的
// basename 做 path.Match，从而正确处理 "/*.go" 命中 "c.go" 这类场景。
func endsWithPathSegment(haystack, suffix string) bool {
	suffix = strings.TrimSuffix(suffix, "/")
	if suffix == "" {
		return true
	}
	base := path.Base(haystack)
	suffixBase := path.Base(suffix)
	if ok, err := path.Match(suffixBase, base); err == nil && ok {
		return true
	}
	// 退化：haystack 以 suffix 整体结尾且前界为 /
	if strings.HasSuffix(haystack, suffix) {
		idx := len(haystack) - len(suffix)
		return idx <= 0 || haystack[idx-1] == '/'
	}
	return false
}

// buildSubagentSystemPrompt 根据子任务类型构建系统提示。
func buildSubagentSystemPrompt(task *SubagentTask) string {
	var sb strings.Builder

	sb.WriteString("你是一个 AnQiCMS AI 子任务执行器。\n\n")
	sb.WriteString(fmt.Sprintf("## 子任务类型\n%s\n\n", task.Type))
	sb.WriteString(fmt.Sprintf("## 任务描述\n%s\n\n", task.Description))

	switch task.Type {
	case SubagentExplore:
		sb.WriteString("## 规则\n")
		sb.WriteString("1. 你是只读子 agent，只能调用只读工具进行调查\n")
		sb.WriteString("2. 不要修改任何文件或数据\n")
		sb.WriteString("3. 完成调查后，用中文总结发现\n\n")
	case SubagentWorker:
		sb.WriteString("## 规则\n")
		sb.WriteString("1. 你是可写子 agent，可以调用工具修改文件\n")
		sb.WriteString(fmt.Sprintf("2. 你的写操作范围限定在: %s\n", strings.Join(task.Scope, ", ")))
		sb.WriteString("3. 不要修改 scope 之外的文件\n")
		sb.WriteString("4. 完成任务后，用中文总结做了什么\n\n")
	}

	sb.WriteString("## 任务\n")
	sb.WriteString(task.Prompt)
	sb.WriteString("\n\n请开始执行。完成后用中文总结。")

	return sb.String()
}

// ================================================================
// `task` 工具: 模型派发并行子任务
// ================================================================

// TaskToolArgs `task` 工具的参数
type TaskToolArgs struct {
	Tasks []SubagentTaskSpec `json:"tasks"`
}

// SubagentTaskSpec 子任务规格 (来自模型的 JSON 参数)
type SubagentTaskSpec struct {
	Description string   `json:"description" desc:"3-5 词任务标签"`
	Prompt      string   `json:"prompt" desc:"子任务完整指令"`
	Type        string   `json:"type" desc:"explore (只读) 或 worker (可写)"`
	Scope       []string `json:"scope,omitempty" desc:"worker 允许写的文件 scope (globs)"`
}

// DispatchTasks 并行派发多个子任务，等待全部完成后返回汇总结果。
//
// 研究报告要求:
//   - 子任务通过 goroutine 并行执行
//   - worker scope 非重叠校验
//   - 结果汇总到主对话
func (svc *AiChatService) DispatchTasks(ctx context.Context, tasks []*SubagentTask) []*SubagentResult {
	var wg sync.WaitGroup
	results := make([]*SubagentResult, len(tasks))
	manager := NewSubagentManager()

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t *SubagentTask) {
			defer wg.Done()

			// worker 需要校验和占用 scope
			if t.Type == SubagentWorker {
				if err := manager.ValidateScope(t.Scope); err != nil {
					results[idx] = &SubagentResult{
						TaskID:      t.ID,
						Description: t.Description,
						Type:        t.Type,
						Error:       err.Error(),
					}
					return
				}
				manager.ReserveScope(t.Scope)
				defer manager.ReleaseScope(t.Scope)
			}

			// 执行子任务
			result := svc.ExecuteSubagent(ctx, t)
			results[idx] = result
			manager.AddResult(result)
		}(i, task)
	}

	wg.Wait()
	return results
}

// DistillSubagentOutput 在子代理结果回传主上下文前做蒸馏 (P2-8)。
//
// 目的：子代理拥有独立上下文，其原始轨迹（多轮工具输入输出）若原样回灌主上下文，
// 会稀释主 agent 的注意力并迅速撑爆上下文窗口。仿 Anthropic「子代理仅回传相关摘要」。
//
// 做法（确定性、零额外 LLM 调用，避免成本与延迟）：
//  1. 超过 distillMaxRunes 的回传一律按 UTF-8 边界安全截断，并标注已被蒸馏。
//  2. 前端补一句结构化提示，引导主 agent 把这段当成「结论 + 关键依据」而非逐字原文。
//
// maxRunes <= 0 时退回默认上限，保证调用方不传也不会panic或无限增长。
func DistillSubagentOutput(output string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = DefaultSubagentDistillMaxRunes
	}
	runes := []rune(output)
	if len(runes) <= maxRunes {
		return output
	}
	cut := maxRunes
	// 尽量在句子边界（换行/句号）截断，避免半句话
	for cut > maxRunes-200 && cut > 0 {
		c := runes[cut-1]
		if c == '\n' || c == '.' || c == '。' || c == '；' || c == ';' {
			break
		}
		cut--
	}
	if cut <= 0 {
		cut = maxRunes
	}
	return string(runes[:cut]) + "\n…（子任务输出已蒸馏，仅保留结论与关键依据）"
}

// FormatSubagentResults 将子任务结果格式化为汇总文本，供注入主对话。
// distillMaxRunes 控制单个子任务回传的最大字符数（见 DistillSubagentOutput）。
func FormatSubagentResults(results []*SubagentResult, distillMaxRunes int) string {
	if len(results) == 0 {
		return "无子任务结果。"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 子任务执行结果汇总 (%d 个子任务)\n\n", len(results)))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("### 子任务 %d: %s (%s)\n", i+1, r.Description, r.Type))
		sb.WriteString(fmt.Sprintf("- 状态: %s\n", statusText(r)))
		sb.WriteString(fmt.Sprintf("- 耗时: %v\n", r.Duration))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("- 错误: %s\n", r.Error))
		}
		if r.Output != "" {
			sb.WriteString(fmt.Sprintf("- 输出:\n%s\n", DistillSubagentOutput(r.Output, distillMaxRunes)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func statusText(r *SubagentResult) string {
	if r.Error != "" {
		return "失败"
	}
	if r.Success {
		return "成功"
	}
	return "未知"
}

// ================================================================
// `team` 工具已移除：不需要支持团队调度。
// ================================================================
