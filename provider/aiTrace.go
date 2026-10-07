package provider

import (
	"sync"
	"time"
)

// ================================================================
// P2-7: 结构化 Tracing（仿 OpenAI Agents SDK 的 Tracing）
//
// 为 aiChat 主循环 / Agent / 子代理 / 中间件决策提供统一的结构化事件流，
// 写入进程内环形缓冲，可随时查询。用途：
//   - 排障：某次响应为什么卡住 / 哪个工具被哪个护栏拦了 / 压缩发生在第几轮。
//   - 前端展示：把「轮次 · 工具 · 护栏」时间线呈现给用户（透明化 agent 行为）。
//
// 设计：进程内环形缓冲，固定容量、协程安全、零 IO，默认开启、零成本可关。
// （与 OpenAI Tracing 的区别：这里是轻量版，不接外部 collector，只落内存。）
// ================================================================

// TracePhase 事件阶段分类
type TracePhase string

const (
	TraceRound     TracePhase = "round"     // 主循环 / Agent 的一轮 LLM 调用
	TraceTool      TracePhase = "tool"      // 工具执行
	TraceGuardrail TracePhase = "guardrail" // 中间件决策（放行/拦截/审批/阻断）
	TraceRetry     TracePhase = "retry"     // 重试 / 续接
	TraceCompact   TracePhase = "compact"   // 上下文压缩
	TraceAgent     TracePhase = "agent"     // Agent / 子代理调度
	TraceNote      TracePhase = "note"      // 普通信息
)

// TraceEvent 一条结构化追踪事件
type TraceEvent struct {
	TS         time.Time  `json:"ts"`
	Phase      TracePhase `json:"phase"`
	Name       string     `json:"name"`
	Detail     string     `json:"detail,omitempty"`
	Decision   string     `json:"decision,omitempty"` // 仅 guardrail：proceed/allow/deny/ask/block
	DurationMs int64      `json:"duration_ms,omitempty"`
	Err        bool       `json:"err,omitempty"`
}

// TraceRecorder 进程内环形缓冲追踪器
type TraceRecorder struct {
	mu      sync.Mutex
	buf     []TraceEvent
	cap     int
	enabled bool
}

// NewTraceRecorder 创建容量为 cap 的追踪器（cap<=0 时用默认 500）。
func NewTraceRecorder(cap int) *TraceRecorder {
	if cap <= 0 {
		cap = 500
	}
	return &TraceRecorder{
		buf:     make([]TraceEvent, 0, cap),
		cap:     cap,
		enabled: true,
	}
}

// SetEnabled 开关追踪（排障关闭时 Record 直接 no-op）。
func (r *TraceRecorder) SetEnabled(on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = on
}

// Record 追加一条事件。
func (r *TraceRecorder) Record(e TraceEvent) {
	if e.TS.IsZero() {
		e.TS = time.Now()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.enabled {
		return
	}
	r.buf = append(r.buf, e)
	if len(r.buf) > r.cap {
		// 丢最旧的，保持环形
		r.buf = r.buf[len(r.buf)-r.cap:]
	}
}

// RecordPhase 便捷封装：记录一个阶段事件。
func (r *TraceRecorder) RecordPhase(phase TracePhase, name, detail string) {
	r.Record(TraceEvent{Phase: phase, Name: name, Detail: detail})
}

// RecordTool 便捷封装：记录一次工具执行（含耗时与被拒标记）。
func (r *TraceRecorder) RecordTool(name string, dur time.Duration, rejected bool, detail string) {
	r.Record(TraceEvent{
		Phase:      TraceTool,
		Name:       name,
		Detail:     detail,
		DurationMs: dur.Milliseconds(),
		Err:        rejected,
	})
}

// RecordGuardrail 便捷封装：记录一次中间件决策。
func (r *TraceRecorder) RecordGuardrail(middleware, decision, detail string) {
	r.Record(TraceEvent{
		Phase:    TraceGuardrail,
		Name:     middleware,
		Detail:   detail,
		Decision: decision,
	})
}

// Snapshot 返回最近 limit 条事件（limit<=0 返回全部）。
func (r *TraceRecorder) Snapshot(limit int) []TraceEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]TraceEvent, len(r.buf))
	copy(out, r.buf)
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// Reset 清空缓冲（用于单测或按会话隔离）。
func (r *TraceRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = r.buf[:0]
}
