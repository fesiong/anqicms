package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
)

// ================================================================
// P8: 遥测与成本归因 (仿 atomcode atomcode-telemetry + model-cost-attribution)
//
// 研究报告要求:
//   1. 新增 model.AiUsageLog 表: 记录 session_id, turn_id, prompt_tokens,
//      completion_tokens, total_tokens, model_name, cost, created_time
//   2. 每轮 SSE usage 事件时同步写一行
//   3. 后台统计页: 按会话/agent/天聚合 token 与成本
// ================================================================

// ── 模型定价表 (USD per 1K tokens, 参考 models.dev 公开定价) ──
// input = prompt token 单价, output = completion token 单价
type modelPricing struct {
	inputPer1K  float64
	outputPer1K float64
}

// defaultPricings 内置常见模型定价 (USD/1K tokens)。
// 如未命中，按保守默认 (input 0.0015, output 0.006) 计算。
var defaultPricings = map[string]modelPricing{
	// OpenAI
	"gpt-4":            {0.03, 0.06},
	"gpt-4-turbo":      {0.01, 0.03},
	"gpt-4o":           {0.0025, 0.01},
	"gpt-4o-mini":      {0.00015, 0.0006},
	"gpt-3.5-turbo":    {0.0005, 0.0015},
	"o1":               {0.015, 0.06},
	"o1-mini":          {0.003, 0.012},
	"o3-mini":          {0.0011, 0.0044},
	// Anthropic
	"claude-3-opus":    {0.015, 0.075},
	"claude-3-sonnet":  {0.003, 0.015},
	"claude-3-haiku":   {0.00025, 0.00125},
	"claude-3.5-sonnet": {0.003, 0.015},
	// Google
	"gemini-1.5-pro":   {0.00125, 0.005},
	"gemini-1.5-flash": {0.000075, 0.0003},
	// DeepSeek
	"deepseek-chat":    {0.00014, 0.00028},
	"deepseek-reasoner": {0.00055, 0.00219},
	// 通义千问
	"qwen-max":         {0.0024, 0.0096},
	"qwen-plus":        {0.0004, 0.0012},
	"qwen-turbo":       {0.00005, 0.0002},
	// 默认 fallback
	"default":          {0.0015, 0.006},
}

// CalculateCost 根据模型名和 token 用量计算成本 (USD)。
func CalculateCost(modelName string, promptTokens, completionTokens int) float64 {
	p, ok := defaultPricings[normalizeModelName(modelName)]
	if !ok {
		p = defaultPricings["default"]
	}
	cost := float64(promptTokens)*p.inputPer1K/1000.0 + float64(completionTokens)*p.outputPer1K/1000.0
	return cost
}

// normalizeModelName 归一化模型名 (小写、去版本号后缀) 以匹配定价表。
func normalizeModelName(name string) string {
	if name == "" {
		return "default"
	}
	lower := strings.ToLower(name)
	// 去除常见日期/版本后缀: -2024-08-06, -latest, -001 等
	for _, sep := range []string{"-2024-", "-2023-", "-latest", "-preview", "-001", "-002"} {
		if idx := strings.Index(lower, sep); idx >= 0 {
			lower = lower[:idx]
		}
	}
	return lower
}

// ── 遥测记录器 ──

// TelemetryRecorder 负责将 token 用量与成本持久化到 DB。
type TelemetryRecorder struct {
	mu sync.Mutex
	db *gorm.DB
	// buffer 缓存待写入的 usage log，减少 DB 压力
	buffer []*model.AiUsageLog
	// flushInterval 缓冲刷盘间隔
	flushInterval time.Duration
	// quit 退出信号
	quit chan struct{}
}

// NewTelemetryRecorder 创建遥测记录器并启动后台刷盘 goroutine。
func NewTelemetryRecorder(db *gorm.DB) *TelemetryRecorder {
	r := &TelemetryRecorder{
		db:            db,
		flushInterval: 5 * time.Second,
		quit:          make(chan struct{}),
	}
	if db != nil {
		go r.flushLoop()
	}
	return r
}

// Record 记录一轮 AI 调用的 token 用量与成本 (非阻塞，缓冲写入)。
func (r *TelemetryRecorder) Record(
	sessionID string,
	agentID uint,
	turnID int,
	modelName string,
	promptTokens, completionTokens int,
	toolCalls int,
	durationMs int64,
) {
	if r == nil {
		return
	}
	totalTokens := promptTokens + completionTokens
	cost := CalculateCost(modelName, promptTokens, completionTokens)

	log := &model.AiUsageLog{
		SessionId:        sessionID,
		AgentId:          agentID,
		TurnId:           turnID,
		ModelName:        modelName,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		Cost:             cost,
		ToolCalls:        toolCalls,
		DurationMs:       durationMs,
	}

	r.mu.Lock()
	r.buffer = append(r.buffer, log)
	r.mu.Unlock()
}

// Stop 停止后台刷盘 goroutine，并刷盘剩余数据。
func (r *TelemetryRecorder) Stop() {
	if r == nil {
		return
	}
	close(r.quit)
	r.flush()
}

// flushLoop 后台定时刷盘。
func (r *TelemetryRecorder) flushLoop() {
	ticker := time.NewTicker(r.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.flush()
		case <-r.quit:
			return
		}
	}
}

// flush 将缓冲区数据批量写入 DB。
func (r *TelemetryRecorder) flush() {
	if r == nil || r.db == nil {
		return
	}
	r.mu.Lock()
	if len(r.buffer) == 0 {
		r.mu.Unlock()
		return
	}
	batch := r.buffer
	r.buffer = nil
	r.mu.Unlock()

	// 批量插入 (最多 100 条一批)
	for i := 0; i < len(batch); i += 100 {
		end := i + 100
		if end > len(batch) {
			end = len(batch)
		}
		if err := r.db.Create(batch[i:end]).Error; err != nil {
			// 写入失败，丢弃这一批，避免无限累积
			fmt.Printf("telemetry flush error: %v\n", err)
		}
	}
}

// ── 统计查询 (供后台统计页使用) ──

// UsageStats 按 session_id / agent_id / 日期 聚合的 token 与成本统计。
type UsageStats struct {
	SessionId        string  `json:"session_id"`
	AgentId          uint    `json:"agent_id"`
	Date             string  `json:"date"` // YYYY-MM-DD
	ModelName        string  `json:"model_name"`
	Turns            int     `json:"turns"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
	ToolCalls        int     `json:"tool_calls"`
}

// AggregateByDay 按天聚合指定时间范围内的 token 用量与成本。
func AggregateByDay(db *gorm.DB, startTime, endTime int64) ([]UsageStats, error) {
	if db == nil {
		return nil, fmt.Errorf("db not available")
	}
	var results []UsageStats
	err := db.Table("ai_usage_logs").
		Select(`
			'common' as session_id,
			0 as agent_id,
			DATE(FROM_UNIXTIME(created_time)) as date,
			model_name,
			COUNT(*) as turns,
			SUM(prompt_tokens) as prompt_tokens,
			SUM(completion_tokens) as completion_tokens,
			SUM(total_tokens) as total_tokens,
			SUM(cost) as cost,
			SUM(tool_calls) as tool_calls
		`).
		Where("created_time >= ? AND created_time < ?", startTime, endTime).
		Group("date, model_name").
		Order("date DESC, model_name").
		Scan(&results).Error
	return results, err
}

// AggregateBySession 按会话聚合指定时间范围内的 token 用量与成本。
func AggregateBySession(db *gorm.DB, startTime, endTime int64) ([]UsageStats, error) {
	if db == nil {
		return nil, fmt.Errorf("db not available")
	}
	var results []UsageStats
	err := db.Table("ai_usage_logs").
		Select(`
			session_id,
			agent_id,
			'' as date,
			model_name,
			COUNT(*) as turns,
			SUM(prompt_tokens) as prompt_tokens,
			SUM(completion_tokens) as completion_tokens,
			SUM(total_tokens) as total_tokens,
			SUM(cost) as cost,
			SUM(tool_calls) as tool_calls
		`).
		Where("created_time >= ? AND created_time < ?", startTime, endTime).
		Group("session_id, agent_id, model_name").
		Order("total_tokens DESC").
		Scan(&results).Error
	return results, err
}

// GetTotalCost 计算指定时间范围内的总成本。
func GetTotalCost(db *gorm.DB, startTime, endTime int64) (float64, error) {
	if db == nil {
		return 0, fmt.Errorf("db not available")
	}
	var totalCost float64
	err := db.Table("ai_usage_logs").
		Where("created_time >= ? AND created_time < ?", startTime, endTime).
		Select("COALESCE(SUM(cost), 0)").
		Scan(&totalCost).Error
	return totalCost, err
}

// ── AiChatService 集成 ──

// telemetryRecorder 每个站点一个遥测记录器。
// 在 AiChatService 中通过 *TelemetryRecorder 字段引用。
// 主循环每轮 SSE usage 事件时调用 Record。

// RecordUsage 是 AiChatService 的便捷方法，记录一轮 token 用量。
func (svc *AiChatService) RecordUsage(
	ctx context.Context,
	sessionID string,
	agentID uint,
	turnID int,
	modelName string,
	promptTokens, completionTokens int,
	toolCalls int,
	durationMs int64,
) {
	if svc.telemetryRecorder == nil {
		return
	}
	svc.telemetryRecorder.Record(
		sessionID, agentID, turnID, modelName,
		promptTokens, completionTokens,
		toolCalls, durationMs,
	)
}
