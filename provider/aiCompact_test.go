package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// buildLongHistory 构造交替的 user/assistant 历史，长度约 2*n+1。
func buildLongHistory(n int) []*schema.Message {
	msgs := []*schema.Message{schema.SystemMessage("你是站点助手")}
	for i := 0; i < n; i++ {
		msgs = append(msgs, schema.UserMessage(fmt.Sprintf("第 %d 个请求", i)))
		msgs = append(msgs, schema.AssistantMessage(fmt.Sprintf("第 %d 个回复", i), nil))
	}
	return msgs
}

// 无摘要器 → 回落截断式压缩（行为与改造前一致）。
func TestCompactMessagesFallbackTruncation(t *testing.T) {
	msgs := buildLongHistory(15)
	got := CompactMessagesWithSummary(context.Background(), msgs, 5, nil)
	if len(got) >= len(msgs) {
		t.Fatalf("压缩后应显著变短: got %d, want < %d", len(got), len(msgs))
	}
	if got[0].Role != schema.System {
		t.Fatalf("第一条必须是 system prompt, got %v", got[0].Role)
	}
	found := false
	for _, m := range got {
		if strings.Contains(m.Content, "[系统压缩]") {
			found = true
		}
	}
	if !found {
		t.Fatal("截断式压缩应产生带 [系统压缩] 标记的摘要消息")
	}
}

// 语义摘要可用时，应使用它而不是暴力截断。
func TestCompactMessagesWithSummaryUsesSemanticSummary(t *testing.T) {
	msgs := buildLongHistory(15)
	sum := SummarizerFunc(func(ctx context.Context, prompt string) (string, error) {
		return "用户要求优化站点模板；已完成 index.html 与 list.html 的修改；待办：验证构建。", nil
	})
	got := CompactMessagesWithSummary(context.Background(), msgs, 5, sum)

	var summaryMsg string
	for _, m := range got {
		if strings.Contains(m.Content, "语义压缩") {
			summaryMsg = m.Content
		}
	}
	if summaryMsg == "" {
		t.Fatalf("应使用语义摘要（带「语义压缩」标记），got %d 条消息", len(got))
	}
	if !strings.Contains(summaryMsg, "index.html") {
		t.Fatalf("语义摘要内容应被保留, got %q", summaryMsg)
	}
}

// 净损失守卫：摘要把原文照抄回来（没有真的变短）→ 退回截断方案。
func TestCompactMessagesWithSummaryNetLossGuard(t *testing.T) {
	msgs := buildLongHistory(15)
	// 返回一个超长「摘要」——比截断方案还长
	echo := SummarizerFunc(func(ctx context.Context, prompt string) (string, error) {
		return strings.Repeat("这是一段没有信息量的冗长复述。", 200), nil
	})
	got := CompactMessagesWithSummary(context.Background(), msgs, 5, echo)
	truncated := CompactMessages(msgs, 5)

	if messagesContentLen(got) > messagesContentLen(truncated) {
		t.Fatalf("净损失守卫应拒绝该摘要，结果不得比截断方案更长: got %d, truncated %d",
			messagesContentLen(got), messagesContentLen(truncated))
	}
	// 且应当退回成截断方案（不是语义摘要）
	for _, m := range got {
		if strings.Contains(m.Content, "语义压缩") {
			t.Fatal("净损失时应退回截断方案，不应保留语义摘要")
		}
	}
}

// 守卫 1：消息都很短时，截断方案反而比原文更长 —— 此时应干脆不压缩。
// （否则"压缩"只是在往上下文里塞前缀和分隔符。）
func TestCompactMessagesWithSummarySkipsWhenTruncationWouldGrow(t *testing.T) {
	// 每条内容极短，加 "role: " 前缀与 "---" 分隔符后必然变长
	msgs := buildLongHistory(15)
	// 先把内容压到最短，制造"压缩反而变大"的场景
	for i := range msgs {
		msgs[i].Content = "x"
	}
	got := CompactMessagesWithSummary(context.Background(), msgs, 5,
		SummarizerFunc(func(ctx context.Context, prompt string) (string, error) {
			return "短摘要", nil
		}))
	if messagesContentLen(got) > messagesContentLen(msgs) {
		t.Fatalf("压缩后不得比原文更长: got %d, orig %d",
			messagesContentLen(got), messagesContentLen(msgs))
	}
}

// 摘要器出错 / 返回空 → 回落，不能因为模型不可用就失去压缩能力。
func TestCompactMessagesWithSummaryErrorFallsBack(t *testing.T) {
	msgs := buildLongHistory(15)

	errSum := SummarizerFunc(func(ctx context.Context, prompt string) (string, error) {
		return "", errors.New("model unavailable")
	})
	got := CompactMessagesWithSummary(context.Background(), msgs, 5, errSum)
	if len(got) >= len(msgs) {
		t.Fatal("摘要失败时应回落截断，压缩仍应发生")
	}

	emptySum := SummarizerFunc(func(ctx context.Context, prompt string) (string, error) {
		return "   ", nil
	})
	got2 := CompactMessagesWithSummary(context.Background(), msgs, 5, emptySum)
	if len(got2) >= len(msgs) {
		t.Fatal("摘要为空时应回落截断")
	}
}

// 切点安全：不能把 tool_call 与它的 tool_result 拆开，
// 否则会产生非法的消息序列（模型 API 会直接报错）。
func TestCompactCutNeverSplitsToolPairs(t *testing.T) {
	msgs := []*schema.Message{schema.SystemMessage("sys")}
	for i := 0; i < 10; i++ {
		msgs = append(msgs, schema.UserMessage(fmt.Sprintf("q%d", i)))
		msgs = append(msgs, schema.AssistantMessage("", []schema.ToolCall{
			{ID: fmt.Sprintf("c%d", i), Function: schema.FunctionCall{Name: "read_file", Arguments: "{}"}},
		}))
		msgs = append(msgs, schema.ToolMessage(fmt.Sprintf("result %d", i), fmt.Sprintf("c%d", i)))
		msgs = append(msgs, schema.AssistantMessage(fmt.Sprintf("a%d", i), nil))
	}

	prefix, cut := compactCut(msgs, 5)
	if cut <= prefix {
		t.Fatalf("应存在可压缩区间, prefix=%d cut=%d", prefix, cut)
	}
	// 保留区第一条不能是孤儿 Tool 结果，也不能是带 tool_calls 的 assistant
	first := msgs[cut]
	if first.Role == schema.Tool {
		t.Fatalf("切点落在孤儿 Tool 结果上: cut=%d", cut)
	}
	if first.Role == schema.Assistant && len(first.ToolCalls) > 0 {
		t.Fatalf("切点落在带 tool_calls 的 assistant 上: cut=%d", cut)
	}
}

// 消息太少时不应压缩。
func TestCompactMessagesTooShortIsNoop(t *testing.T) {
	msgs := buildLongHistory(2) // 5 条
	got := CompactMessages(msgs, 5)
	if len(got) != len(msgs) {
		t.Fatalf("消息数不足时不应压缩: got %d, want %d", len(got), len(msgs))
	}
}

// Anti-nesting：已存在压缩摘要时，不应再套一层摘要。
func TestCompactMessagesAntiNesting(t *testing.T) {
	msgs := []*schema.Message{schema.SystemMessage("sys"),
		schema.SystemMessage("[系统压缩] 旧摘要")}
	for i := 0; i < 10; i++ {
		msgs = append(msgs, schema.UserMessage(fmt.Sprintf("q%d", i)))
		msgs = append(msgs, schema.AssistantMessage(fmt.Sprintf("a%d", i), nil))
	}
	got := CompactMessages(msgs, 5)
	// 结果中 [系统压缩] 标记最多出现一次（旧摘要被直接保留，不再包一层）
	count := 0
	for _, m := range got {
		count += strings.Count(m.Content, "[系统压缩]")
	}
	if count > 1 {
		t.Fatalf("压缩摘要不应嵌套: [系统压缩] 出现 %d 次", count)
	}
}

// 阈值配置化：零值应回落到与改造前一致的默认值。
// 逐项穷举（而非挑几个断言）：漏掉字段正是 < 0 / <= 0 笔误藏身的地方。
func TestChatTuningNormalizedDefaults(t *testing.T) {
	d := ChatTuning{}.Normalized()
	got := map[string]int{
		"MaxRounds":                          d.MaxRounds,
		"StagnationReadRounds":               d.StagnationReadRounds,
		"CompactMinMessages":                 d.CompactMinMessages,
		"CompactEveryNRounds":                d.CompactEveryNRounds,
		"CompactForceMessages":               d.CompactForceMessages,
		"CompactKeepCount":                   d.CompactKeepCount,
		"MaxResponseTruncationContinuations": d.MaxResponseTruncationContinuations,
		"EmptyResponseMaxRetries":            d.EmptyResponseMaxRetries,
		"MaxToolResultBytes":                 d.MaxToolResultBytes,
		"SubagentDistillMaxRunes":            d.SubagentDistillMaxRunes,
	}
	want := map[string]int{
		"MaxRounds":                          DefaultMaxRounds,
		"StagnationReadRounds":               DefaultStagnationReadRounds,
		"CompactMinMessages":                 DefaultCompactMinMessages,
		"CompactEveryNRounds":                DefaultCompactEveryNRounds,
		"CompactForceMessages":               DefaultCompactForceMessages,
		"CompactKeepCount":                   DefaultCompactKeepCount,
		"MaxResponseTruncationContinuations": DefaultMaxResponseTruncationContinuations,
		"EmptyResponseMaxRetries":            DefaultEmptyResponseMaxRetries,
		"MaxToolResultBytes":                 DefaultMaxToolResultBytes,
		"SubagentDistillMaxRunes":            DefaultSubagentDistillMaxRunes,
	}
	for field, wantVal := range want {
		if got[field] != wantVal {
			t.Errorf("零值 Normalized 后 %s = %d, want 默认值 %d", field, got[field], wantVal)
		}
	}

	// 显式配置应被保留
	custom := ChatTuning{MaxRounds: 8, CompactKeepCount: 3}.Normalized()
	if custom.MaxRounds != 8 || custom.CompactKeepCount != 3 {
		t.Fatalf("显式配置应保留: %+v", custom)
	}
	if custom.CompactMinMessages != 12 {
		t.Fatalf("未配置字段应回落默认: %+v", custom)
	}
}
