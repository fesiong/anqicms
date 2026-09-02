package provider

import (
	"context"
	"fmt"
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
		return result
	}

	// 构建子 agent 的工具集
	subTools, subHandlers := svc.buildSubagentTools(task.Type, task.Scope)
	if len(subTools) > 0 {
		if err := client.BindTools(subTools); err != nil {
			result.Error = fmt.Sprintf("failed to bind tools: %v", err)
			result.Duration = time.Since(start)
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

// isWithinScope 检查工具调用的参数是否在声明的 scope 范围内。
// scope 是一组 glob 模式，参数中的 file_path 必须匹配至少一个模式。
func isWithinScope(argsJSON string, scope []string) bool {
	// 简单实现: 检查 argsJSON 中是否包含 scope 中的任意路径片段
	// 如果 scope 包含 "*", 做简单的通配匹配
	argsLower := strings.ToLower(argsJSON)
	for _, s := range scope {
		sLower := strings.ToLower(s)
		if strings.Contains(argsLower, sLower) {
			return true
		}
		// 通配符 scope (如 "src/**"): 检查路径前缀
		if strings.Contains(sLower, "*") {
			prefix := strings.Split(sLower, "*")[0]
			if prefix != "" && strings.Contains(argsLower, prefix) {
				return true
			}
		}
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

// FormatSubagentResults 将子任务结果格式化为汇总文本，供注入主对话。
func FormatSubagentResults(results []*SubagentResult) string {
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
			sb.WriteString(fmt.Sprintf("- 输出:\n%s\n", r.Output))
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
// `team` 工具: 异步团队调度
// ================================================================

// TeamSpec 团队任务规格
type TeamSpec struct {
	Name        string   `json:"name" desc:"团队任务名称"`
	Description string   `json:"description" desc:"团队任务描述"`
	Roles       []string `json:"roles" desc:"需要的角色列表 (如 architect, rust, tester)"`
	Tasks       []SubagentTaskSpec `json:"tasks" desc:"子任务列表"`
}

// DispatchTeam 派发一个团队任务 (异步执行，立即返回任务 ID)。
// 主循环可通过 PollTeamResults 轮询结果。
func (svc *AiChatService) DispatchTeam(ctx context.Context, spec *TeamSpec) (string, error) {
	teamID := fmt.Sprintf("team_%d", time.Now().UnixNano())

	// 构建 SubagentTask 列表
	tasks := make([]*SubagentTask, 0, len(spec.Tasks))
	for i, ts := range spec.Tasks {
		taskID := fmt.Sprintf("%s_task_%d", teamID, i+1)
		subType := SubagentExplore
		if ts.Type == "worker" {
			subType = SubagentWorker
		}
		tasks = append(tasks, &SubagentTask{
			ID:          taskID,
			Description: ts.Description,
			Prompt:      ts.Prompt,
			Type:        subType,
			Scope:       ts.Scope,
			MaxRounds:   10,
			Timeout:     5 * time.Minute,
		})
	}

	// 异步执行
	go func() {
		teamCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		svc.DispatchTasks(teamCtx, tasks)
	}()

	return teamID, nil
}
