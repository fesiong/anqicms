package provider

import (
	"errors"
)

// 本文件是渠道聊天的「执行主体 + 身份与生命周期判定」纯数据层：
// 定义执行主体与可 errors.Is 的哨兵错误。渠道授权只认站点曝光与身份/生命周期，
// 不再有策略 JSON、接待白名单或后台菜单映射。
//
// 真正需要读库的判定在 channelPermissions.go。

// ExecutionPrincipal 是一次渠道请求的执行主体。
//
// 只由调用方（T6 的聊天执行器）在请求内构造并逐层传入，**不放 context 全局单例、
// 不写回服务级状态**：权限判定不得改 svc.Tools / 全局 MCP 配置 / 进程级缓存，
// 否则一个站点的身份会污染其它站点与其它并发请求（规格 §7）。
type ExecutionPrincipal struct {
	SiteId    uint
	AdminId   uint
	BindingId uint
	// ChannelId 是渠道隔离的关键：没有它就读不到该渠道的接待智能体与启用状态，
	// 判定只能退化成「站点默认放行」。
	ChannelId uint
	// AgentId 是**本次实际执行的智能体**（R35），不是渠道配置里那个接待智能体。
	// 门禁要求它非 0 且与渠道配置的接待智能体一致（见 channelForPrincipal）。
	AgentId   uint
	SessionId string
	// InGroup 标记本条消息来自群聊。首版群聊只允许群安全工具（规格 §7），
	// 由 T6 的群安全判定消费；身份判定本身不因群聊而放宽。
	InGroup bool
}

// 渠道门禁哨兵错误。全部用 %w 包装，调用方用 errors.Is 分流。
//
// 文案里不允许出现密钥、token、摘要、管理员/智能体内部 ID 等敏感值（全局约束）。
var (
	ErrChannelPrincipalInvalid = errors.New("渠道执行主体无效")
	ErrChannelAgentNotAllowed  = errors.New("执行智能体与渠道不一致")
	ErrChannelAdminInvalid     = errors.New("管理员不存在或已停用")
	// ErrChannelPermissionDenied 只作为安全兜底：身份校验通过后仍被拒绝时统一归到它。
	ErrChannelPermissionDenied = errors.New("渠道调用未通过权限校验")
	// ErrChannelDisabled：渠道停用（enabled = 0）或「仅通知」（chat_enabled = 0）。
	// 「仅通知」渠道只投递通知、不执行管理动作，权限层必须在读渠道时就把这类请求拒掉：
	// Enabled 由 T4 的 worker 生命周期兜住，ChatEnabled 没有任何其它读取点，
	// 漏判一次就是「仅通知渠道可执行管理动作」。与 ErrChannelPrincipalInvalid 分开，
	// 是为了排障时能区分「配置关了聊天」与「主体/绑定对不上」。
	ErrChannelDisabled = errors.New("渠道已停用或未开启聊天管理")
)
