package provider

// ================================================================
// ChatTuning —— 主对话循环的可调阈值 (P1-6)
//
// 改造前这些阈值全部硬编码在 generateAIResponse 与中间件里
// （连续只读 4 轮空转、消息 >12 且每 3 轮压缩、截断上限 10000 字符…）。
// 它们都是「经验值」，不同站点/不同模型的合理取值并不一样，
// 写死意味着调一次就要改代码重编译。
//
// 约定：零值 = 未配置，Normalized() 会用默认值补齐。
// 所有默认值与改造前**逐项一致**，未做任何配置的部署行为完全不变。
// ================================================================

// ChatTuning 主对话循环的可调阈值集合。
type ChatTuning struct {
	// MaxRounds 主对话 / 智能体单次执行的最大轮数（一轮 = 一次 LLM 调用 + 工具执行）
	MaxRounds int
	// StagnationReadRounds 连续多少轮只有读操作后注入空转警告
	StagnationReadRounds int
	// CompactMinMessages 消息数超过多少才开始考虑压缩
	CompactMinMessages int
	// CompactEveryNRounds 每多少轮压缩一次（round % N == N-1 时触发）
	CompactEveryNRounds int
	// CompactForceMessages 消息数超过多少则无视轮次强制压缩
	CompactForceMessages int
	// CompactKeepCount 压缩时保留最近多少条消息不压缩
	CompactKeepCount int
	// MaxResponseTruncationContinuations 响应被 max_tokens 截断后最多续接几次
	MaxResponseTruncationContinuations int
	// EmptyResponseMaxRetries 模型返回空响应时最多重试几次
	EmptyResponseMaxRetries int
	// MaxToolResultBytes 工具结果截断上限（字符数，按 UTF-8 边界安全截断）
	MaxToolResultBytes int
	// SubagentDistillMaxRunes 子代理回传主上下文前蒸馏后的最大字符数
	// (P2-8：避免子代理原始轨迹污染主上下文，仿 Anthropic「仅回传相关摘要」)
	SubagentDistillMaxRunes int
}

// 默认值常量 —— 与改造前的硬编码值逐项一致。
const (
	DefaultMaxRounds                          = 20
	DefaultStagnationReadRounds               = 4
	DefaultCompactMinMessages                 = 12
	DefaultCompactEveryNRounds                = 3
	DefaultCompactForceMessages               = 20
	DefaultCompactKeepCount                   = 5
	DefaultMaxResponseTruncationContinuations = 4
	DefaultEmptyResponseMaxRetries            = 5
	DefaultMaxToolResultBytes                 = 10000
	DefaultSubagentDistillMaxRunes            = 2000
)

// Normalized 返回补齐默认值后的副本（零值字段用默认填充）。
// 未做任何配置时等价于全部默认值，行为与改造前一致。
func (t ChatTuning) Normalized() ChatTuning {
	out := t
	if out.MaxRounds <= 0 {
		out.MaxRounds = DefaultMaxRounds
	}
	if out.StagnationReadRounds <= 0 {
		out.StagnationReadRounds = DefaultStagnationReadRounds
	}
	if out.CompactMinMessages <= 0 {
		out.CompactMinMessages = DefaultCompactMinMessages
	}
	if out.CompactEveryNRounds <= 0 {
		out.CompactEveryNRounds = DefaultCompactEveryNRounds
	}
	if out.CompactForceMessages <= 0 {
		out.CompactForceMessages = DefaultCompactForceMessages
	}
	if out.CompactKeepCount <= 0 {
		out.CompactKeepCount = DefaultCompactKeepCount
	}
	// 以下两项必须用 <= 0 而非 < 0：Tuning 目前没有任何写入方，字段恒为零值，
	// 用 < 0 会让它们停在 0 —— 响应续接预算为 0 意味着第一次截断就放弃续写，
	// 空响应重试则整条恢复路径永不生效。
	if out.MaxResponseTruncationContinuations <= 0 {
		out.MaxResponseTruncationContinuations = DefaultMaxResponseTruncationContinuations
	}
	if out.EmptyResponseMaxRetries <= 0 {
		out.EmptyResponseMaxRetries = DefaultEmptyResponseMaxRetries
	}
	if out.MaxToolResultBytes <= 0 {
		out.MaxToolResultBytes = DefaultMaxToolResultBytes
	}
	if out.SubagentDistillMaxRunes <= 0 {
		out.SubagentDistillMaxRunes = DefaultSubagentDistillMaxRunes
	}
	return out
}

// DefaultChatTuning 返回全部取默认值的配置。
func DefaultChatTuning() ChatTuning {
	return ChatTuning{}.Normalized()
}
