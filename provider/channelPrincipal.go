package provider

import (
	"encoding/json"
	"errors"
	"strings"
)

// 本文件是渠道聊天的「执行主体 + 渠道策略」纯数据层：
// 解析策略 JSON、定义裁决取值与可 errors.Is 的哨兵错误。
//
// 真正需要读库的权限判定在 channelPermissions.go。这里刻意保持纯函数/纯类型，
// 因为策略解析既会出现在执行路径上（T6），也会出现在审批复核路径上（T7），
// 两边必须用同一份解析结果，而不是各写一份「怎么读这个 JSON」的猜测。

// PolicyDecision 是渠道策略对「意图 + 动作」的裁决。
type PolicyDecision string

const (
	// PolicyDecisionAllow 直接执行。
	PolicyDecisionAllow PolicyDecision = "allow"
	// PolicyDecisionConfirm 需要管理员在聊天中确认后才执行（T7 负责落审批）。
	PolicyDecisionConfirm PolicyDecision = "confirm"
	// PolicyDecisionDeny 直接拒绝。
	PolicyDecisionDeny PolicyDecision = "deny"
)

// ChannelPolicy 是渠道策略的解析结果。
//
// 线上格式（R30，T9 写、T6 读）：
//
//	{"intents":{"content_article":{"save":"confirm","detail":"allow","*":"deny"}}}
//
// 查找顺序：精确 action → "*" → deny。空串/未配置、解析失败、未知取值一律 deny。
// 未知字段直接忽略；这里不发明意图级通配符——"*" 只作用于 action 维度，
// 「新增能力自动进入已有策略」正是规格 §7 明确禁止的行为。
type ChannelPolicy struct {
	intents map[string]map[string]PolicyDecision
}

// ParseChannelPolicy 解析渠道策略 JSON。
//
// 返回值里的 error 只表示「原文不合法」，供调用方记录与提示；判定语义不受影响——
// 解析失败的策略其 Decision 仍一律 deny。反过来做（解析失败就当没配置、没配置就放行）
// 等于让一个格式错误变成权限提升。
func ParseChannelPolicy(raw string) (ChannelPolicy, error) {
	if strings.TrimSpace(raw) == "" {
		return ChannelPolicy{}, nil
	}
	var payload struct {
		Intents map[string]map[string]string `json:"intents"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		// 返回空策略（全部 deny），同时把解析错误交给调用方。
		return ChannelPolicy{}, err
	}
	intents := make(map[string]map[string]PolicyDecision, len(payload.Intents))
	for name, actions := range payload.Intents {
		name = strings.TrimSpace(name)
		if name == "" || len(actions) == 0 {
			continue
		}
		parsed := make(map[string]PolicyDecision, len(actions))
		for action, value := range actions {
			switch PolicyDecision(value) {
			case PolicyDecisionAllow, PolicyDecisionConfirm, PolicyDecisionDeny:
				parsed[action] = PolicyDecision(value)
			default:
				// 未知取值直接丢弃：读取时落到 deny，不猜测意图。
			}
		}
		if len(parsed) > 0 {
			intents[name] = parsed
		}
	}
	return ChannelPolicy{intents: intents}, nil
}

// Decision 返回「意图 + 动作」的裁决：精确 action → "*" → deny。
// 未配置的意图、未配置的动作、非法取值一律 deny（缺省拒绝）。
func (p ChannelPolicy) Decision(intentName, action string) PolicyDecision {
	actions, ok := p.intents[intentName]
	if !ok {
		return PolicyDecisionDeny
	}
	if d, ok := actions[action]; ok {
		return d
	}
	if d, ok := actions["*"]; ok {
		return d
	}
	return PolicyDecisionDeny
}

// ExecutionPrincipal 是一次渠道请求的执行主体。
//
// 只由调用方（T6 的聊天执行器）在请求内构造并逐层传入，**不放 context 全局单例、
// 不写回服务级状态**：权限判定不得改 svc.Tools / 全局 MCP 配置 / 进程级缓存，
// 否则一个站点的身份会污染其它站点与其它并发请求（规格 §7）。
type ExecutionPrincipal struct {
	SiteId    uint
	AdminId   uint
	BindingId uint
	// ChannelId 是 R28 追加的字段：没有它就读不到该渠道的策略与接待智能体，
	// 判定只能退化成「站点默认放行」，与渠道级隔离的初衷正好相反。
	ChannelId      uint
	SessionId      string
	PolicyRevision int64
	// InGroup 标记本条消息来自群聊。首版群聊只允许群安全工具（规格 §7），
	// 由 T6 的群安全判定消费；权限门禁本身不因群聊而放宽。
	InGroup bool
}

// 渠道门禁哨兵错误。全部用 %w 包装，调用方（T6/T7）用 errors.Is 分流：
//   - ErrChannelPolicyNeedsConfirm → 创建审批并回复确认卡片，不是失败；
//   - ErrChannelPolicyStale → 提示重新发起（策略修订号已变）；
//   - 其余 → 拒绝并回执原因。
//
// 文案里不允许出现密钥、token、摘要等敏感值（全局约束）。
var (
	ErrChannelPrincipalInvalid   = errors.New("渠道执行主体无效")
	ErrChannelIntentUnknown      = errors.New("意图不存在")
	ErrChannelIntentNotExposed   = errors.New("意图未在当前站点开放")
	ErrChannelAgentNotAllowed    = errors.New("接待智能体未授权该意图")
	ErrChannelPolicyDenied       = errors.New("渠道策略拒绝该操作")
	ErrChannelPolicyNeedsConfirm = errors.New("该操作需要管理员确认")
	ErrChannelPolicyStale        = errors.New("渠道策略已变更，请重新发起")
	ErrChannelAdminInvalid       = errors.New("管理员不存在或已停用")
	ErrChannelPermissionDenied   = errors.New("管理员缺少该操作的后台权限")
	ErrChannelCapBlocked         = errors.New("底层能力未开放或未映射后台权限")
	ErrChannelAPIPathBlocked     = errors.New("通用调用禁止访问该后台路径")
)
