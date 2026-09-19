package provider

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

// ================================================================
// P3: Provider 重试分层 (仿 atomcode transport → open → stream → partial)
//
// 研究报告要求:
//   1. transport fast backoff: eino client 层 HTTP 重试保留
//   2. open retry: DEFAULT_MAX_PROVIDER_RETRIES=3, 3/6/9s 线性退避
//      覆盖 5xx/timeout/connection reset
//   3. stream reconnect: MAX_STREAM_RETRIES=5, 流中断时保留已收内容,
//      从 history 整轮重发, 指数退避
//   4. partial stream resume: MAX_PARTIAL_STREAM_RECOVERIES=1,
//      已收到部分 assistant 内容时保留, 注入 PARTIAL_STREAM_RESUME_NUDGE
//   5. rate limit 精细化: 首 429 静默 1s, sustained 按 Retry-After 头,
//      无头则 RATE_LIMIT_AUTO_WAIT_SECS=120, MAX_RATE_LIMIT_WAITS=5
// ================================================================

// 重试常量 (仿 atomcode)
const (
	// open retry
	DefaultMaxProviderRetries = 3
	openRetryDelays           = 3 * time.Second // 3/6/9s 线性

	// stream reconnect
	MaxStreamRetries = 5
	streamBaseDelay  = 1 * time.Second

	// partial stream resume
	MaxPartialStreamRecoveries = 1

	// rate limit 精细化
	rateLimitSilentDelay     = 1 * time.Second // 首 429 静默 1s
	RateLimitAutoWaitSecs    = 120             // 无 Retry-After 头时等待秒数
	MaxRateLimitWaits        = 5              // 最多等待次数
)

// PARTIAL_STREAM_RESUME_NUDGE 注入到 messages 末尾，让模型从断点继续。
const PartialStreamResumeNudge = "[系统提示] 你上一次的回复被中断了。请从中断处继续，不要重复已输出的内容。"

// RetryLayer 标识重试发生的层。
type RetryLayer int

const (
	RetryLayerNone RetryLayer = iota
	RetryLayerTransport
	RetryLayerOpen
	RetryLayerStream
	RetryLayerPartial
)

func (l RetryLayer) String() string {
	switch l {
	case RetryLayerTransport:
		return "transport"
	case RetryLayerOpen:
		return "open"
	case RetryLayerStream:
		return "stream"
	case RetryLayerPartial:
		return "partial"
	default:
		return "none"
	}
}

// ClassifyError 将错误分类到对应的重试层。
func ClassifyError(err error) RetryLayer {
	if err == nil {
		return RetryLayerNone
	}
	msg := strings.ToLower(err.Error())

	// ── transport: 网络层错误 ──
	transportPatterns := []string{
		"connection refused", "connection reset", "no such host",
		"deadline exceeded", "context deadline exceeded",
		"i/o timeout", "tls handshake", "broken pipe",
		"network is unreachable",
	}
	for _, p := range transportPatterns {
		if strings.Contains(msg, p) {
			return RetryLayerTransport
		}
	}

	// ── open: API 认证/配额/5xx 错误 ──
	if IsRateLimitError(err) {
		return RetryLayerOpen
	}
	openPatterns := []string{
		"401", "403", "500", "502", "503", "504",
		"unauthorized", "forbidden", "invalid api key",
		"internal server error", "bad gateway",
		"service unavailable", "gateway timeout",
	}
	for _, p := range openPatterns {
		if strings.Contains(msg, p) {
			return RetryLayerOpen
		}
	}

	// ── stream: 流式接收中断 ──
	// io.EOF 是正常结束，不算 stream 错误
	if err != io.EOF {
		streamPatterns := []string{
			"unexpected eof", "stream error", "read on closed body",
		}
		for _, p := range streamPatterns {
			if strings.Contains(msg, p) {
				return RetryLayerStream
			}
		}
	}

	// ── partial: 部分结果不完整 ──
	partialPatterns := []string{"incomplete", "malformed", "invalid json", "missing field"}
	for _, p := range partialPatterns {
		if strings.Contains(msg, p) {
			return RetryLayerPartial
		}
	}

	return RetryLayerTransport
}

// IsNonRetryable 判断错误是否不可重试 (如 400 Bad Request、context overflow)。
func IsNonRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "400") || strings.Contains(msg, "bad request") {
		return true
	}
	if IsContextOverflowError(err) {
		return true
	}
	return false
}

// isEOF 判断错误是否为 io.EOF (正常流结束)。
func isEOF(err error) bool {
	return err == io.EOF
}

// StreamFn 是打开流式连接的函数类型。
type StreamFn func(ctx context.Context, messages []*schema.Message) (*schema.StreamReader[*schema.Message], error)

// StreamResult 保存流式接收的结果。
type StreamResult struct {
	Response        string
	Reasoning       string
	ToolCalls       []schema.ToolCall
	PromptTokens    int
	CompletionTokens int
	// FinishReason: 流结束原因 ("stop"/"length"/"tool_calls"/...)
	// "length" 表示 AI 响应被 max_tokens 截断，需注入 TRUNCATION_RESUME_NUDGE
	FinishReason    string
}

// StreamCallbacks 是 StreamWithRetry 的回调集合。
type StreamCallbacks struct {
	OnChunk    func(chunk string)
	OnReasoning func(content string)
	OnWarning  func(msg string)
}

// StreamWithRetry 带四层重试的流式接收 (仿 atomcode)。
//
// 重试分层:
//   - transport: eino client 层 HTTP 重试 (保留, 不在此处理)
//   - open:      DEFAULT_MAX_PROVIDER_RETRIES=3, 3/6/9s 线性退避
//   - stream:    MAX_STREAM_RETRIES=5, 流中断时保留已收内容, 整轮重发
//   - partial:   MAX_PARTIAL_STREAM_RECOVERIES=1, 保留已收内容, 注入 NUDGE
//
// rate limit 精细化:
//   - 首 429 静默 1s 重试
//   - sustained 按 Retry-After 头等待, 无头则 120s
//   - MAX_RATE_LIMIT_WAITS=5 后强制失败
func StreamWithRetry(
	ctx context.Context,
	streamFn StreamFn,
	messages []*schema.Message,
	callbacks *StreamCallbacks,
) (*StreamResult, error) {
	if callbacks == nil {
		callbacks = &StreamCallbacks{}
	}

	result := &StreamResult{}

	// ── rate limit 精细化状态 ──
	rateLimitWaits := 0
	rateLimitFirstAttempt := true

	// ── open retry 状态 ──
	openAttempt := 0

	// ── stream reconnect 状态 ──
	streamAttempt := 0

	// ── partial stream resume 状态 ──
	partialAttempt := 0
	// partial resume: 保留已收的 response/reasoning，注入 NUDGE 后继续
	partialResumedResponse := ""
	partialResumedReasoning := ""

	for {
		// ── Layer 1+2: transport + open (client.Stream) ──
		stream, err := streamFn(ctx, messages)
		if err != nil {
			// 不可重试错误直接返回
			if IsNonRetryable(err) {
				return result, fmt.Errorf("stream open failed (non-retryable): %w", err)
			}

			// rate limit 精细化
			if IsRateLimitError(err) {
				if rateLimitFirstAttempt {
					// 首 429 静默 1s 重试
					rateLimitFirstAttempt = false
					if callbacks.OnWarning != nil {
						callbacks.OnWarning("请求频率限制，1s 后静默重试...")
					}
					select {
					case <-time.After(rateLimitSilentDelay):
					case <-ctx.Done():
						return result, ctx.Err()
					}
					continue
				}

				rateLimitWaits++
				if rateLimitWaits > MaxRateLimitWaits {
					return result, fmt.Errorf("rate limit exceeded max waits (%d): %w", MaxRateLimitWaits, err)
				}

				// 解析 Retry-After 头 (如果 err 包含)
				waitSecs := parseRetryAfter(err)
				if waitSecs <= 0 {
					waitSecs = RateLimitAutoWaitSecs
				}
				wait := time.Duration(waitSecs) * time.Second
				if callbacks.OnWarning != nil {
					callbacks.OnWarning(fmt.Sprintf("请求频率限制，%v 后重试 (%d/%d)...", wait, rateLimitWaits, MaxRateLimitWaits))
				}
				select {
				case <-time.After(wait):
				case <-ctx.Done():
					return result, ctx.Err()
				}
				continue
			}

			layer := ClassifyError(err)
			if layer == RetryLayerOpen && openAttempt < DefaultMaxProviderRetries {
				// open retry: 3/6/9s 线性退避
				openAttempt++
				delay := openRetryDelays * time.Duration(openAttempt)
				if callbacks.OnWarning != nil {
					callbacks.OnWarning(fmt.Sprintf("open 错误，%v 后重试 (%d/%d)...", delay, openAttempt, DefaultMaxProviderRetries))
				}
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return result, ctx.Err()
				}
				continue
			}

			return result, fmt.Errorf("stream open failed: %w", err)
		}

		// ── Layer 3: stream (流式接收) ──
		var fullResponse strings.Builder
		var fullReasoning strings.Builder
		var toolCallChunks []*schema.Message
		var promptTokens, completionTokens int
		streamErr := error(nil)

		// 如果是 partial resume，把已收内容加到 builder
		if partialResumedResponse != "" {
			fullResponse.WriteString(partialResumedResponse)
		}
		if partialResumedReasoning != "" {
			fullReasoning.WriteString(partialResumedReasoning)
		}

		for {
			msg, recvErr := stream.Recv()
			if recvErr != nil {
				streamErr = recvErr
				break
			}

			if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
				promptTokens = msg.ResponseMeta.Usage.PromptTokens
				completionTokens = msg.ResponseMeta.Usage.CompletionTokens
			}
			// 提取 finish_reason ("stop"/"length"/"tool_calls"/...)
			if msg.ResponseMeta != nil && msg.ResponseMeta.FinishReason != "" {
				result.FinishReason = msg.ResponseMeta.FinishReason
			}

			if msg.Content != "" {
				fullResponse.WriteString(msg.Content)
				if callbacks.OnChunk != nil {
					callbacks.OnChunk(msg.Content)
				}
			}

			if msg.ReasoningContent != "" {
				fullReasoning.WriteString(msg.ReasoningContent)
				if callbacks.OnReasoning != nil {
					callbacks.OnReasoning(msg.ReasoningContent)
				}
			}

			if len(msg.ToolCalls) > 0 {
				toolCallChunks = append(toolCallChunks, msg)
			}
		}
		stream.Close()

		// 保存已接收内容
		result.Response = fullResponse.String()
		result.Reasoning = fullReasoning.String()
		result.PromptTokens = promptTokens
		result.CompletionTokens = completionTokens

		// 合并 tool_calls
		if len(toolCallChunks) > 0 {
			merged, mergeErr := schema.ConcatMessages(toolCallChunks)
			if mergeErr == nil && merged != nil && len(merged.ToolCalls) > 0 {
				result.ToolCalls = merged.ToolCalls
			}
		}

		// 正常结束 (io.EOF)
		if streamErr == nil || isEOF(streamErr) {
			// ── Layer 4: partial stream resume ──
			// 检查 tool_calls 是否完整
			if len(result.ToolCalls) > 0 {
				incomplete := false
				for _, tc := range result.ToolCalls {
					if tc.ID == "" || tc.Function.Name == "" || tc.Function.Arguments == "" {
						incomplete = true
						break
					}
				}
				if incomplete && partialAttempt < MaxPartialStreamRecoveries {
					partialAttempt++
					// 保留已收内容，注入 NUDGE 让模型从断点继续
					partialResumedResponse = result.Response
					partialResumedReasoning = result.Reasoning
					messages = append(messages, schema.AssistantMessage(result.Response, result.ToolCalls))
					messages = append(messages, schema.UserMessage(PartialStreamResumeNudge))
					if callbacks.OnWarning != nil {
						callbacks.OnWarning("检测到不完整的工具调用，注入续传提示...")
					}
					continue
				}
			}
			// 成功
			return result, nil
		}

		// stream 错误 (非 io.EOF)
		if !IsNonRetryable(streamErr) && streamAttempt < MaxStreamRetries {
			streamAttempt++
			// 流中断时保留已收内容，整轮重发
			// 已收内容在 result 中，重发时 messages 不变
			delay := streamBaseDelay * time.Duration(1<<uint(streamAttempt-1))
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
			if callbacks.OnWarning != nil {
				callbacks.OnWarning(fmt.Sprintf("stream 中断，%v 后重连 (已保留部分结果) (%d/%d)...", delay, streamAttempt, MaxStreamRetries))
			}
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return result, ctx.Err()
			}
			continue
		}

		if IsNonRetryable(streamErr) {
			return result, fmt.Errorf("stream recv failed (non-retryable): %w", streamErr)
		}

		return result, fmt.Errorf("stream recv failed: %w", streamErr)
	}
}

// parseRetryAfter 尝试从错误信息中解析 Retry-After 头的秒数。
func parseRetryAfter(err error) int {
	if err == nil {
		return 0
	}
	msg := err.Error()
	// 尝试解析 "retry-after: N" 格式
	lower := strings.ToLower(msg)
	idx := strings.Index(lower, "retry-after")
	if idx < 0 {
		return 0
	}
	// 从 idx 开始找数字
	for i := idx; i < len(msg); i++ {
		if msg[i] >= '0' && msg[i] <= '9' {
			// 提取数字
			j := i
			for j < len(msg) && msg[j] >= '0' && msg[j] <= '9' {
				j++
			}
			secs, _ := strconv.Atoi(msg[i:j])
			return secs
		}
	}
	return 0
}
