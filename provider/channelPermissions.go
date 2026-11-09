package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/pkg/mcp/intent"
)

// channelPermissions.go 是渠道聊天的权限层：能力（cap）→ 后台菜单权限的显式映射，
// 以及「意图级 + 每次 cap 调用」两道门禁。
//
// 设计要点（对应规格 §7）：
//  1. 每次实际调用取交集：站点曝光（ExposedIntents）、智能体接待白名单、渠道 action 策略、
//     管理员当前状态与菜单权限。审批不扩大这个交集。
//  2. 模型可见性过滤与执行前拦截同时存在：只靠提示词约束等于没约束。
//  3. **默认拒绝**：后台登录（middleware.AdminPermission）对「不在菜单里的路径」是放行的，
//     渠道必须反过来——无法映射权限的 cap、未登记的 cap、黑名单路径一律拒绝。
//  4. 判定不写回任何服务级状态：不改 svc.Tools、不设 MCP scope、不缓存到进程级。
//     所有门禁按请求读库，执行主体由调用方逐层传入。
//
// 两道门禁的分工：
//   - CheckIntentPermission：意图执行前的粗粒度闸门（曝光/白名单/策略/管理员）。
//     意图的 Compose 可能一次委托多个 cap，甚至委托到哪个端点取决于动作参数，
//     所以这里只按「意图声明的可达端点」做一次入口校验。
//   - CheckCapPermission：**每个** cap 真正调用前复核（含动态解析 api_invoke 的最终
//     method/path）。规格要求「覆盖 Compose 多次委托与 api 通用调用」，靠的就是它。

// channelAPIMetaMenuPath 是 api_list / api_schema / api_invoke 的入口权限菜单键。
//
// 三个 api_* 元能力等价于「通用 REST 入口」，本身没有独立后台页面，但也绝不能默认放行：
// 取插件首页（/plugin/index）作为入口权限，能进插件区才允许发现端点。
// api_invoke 的真正权限仍按最终 method/path 逐次解析（见 channelResolveInvokePath），
// 入口权限只用于 api_list / api_schema 这类纯元数据能力。
const channelAPIMetaMenuPath = "/plugin/index"

// channelBuiltinBlockedCaps 首版固定关闭的主机级内置能力（规格 §7：fs/shell/联网首版关闭）。
//
// 这些能力直接读写服务器文件、执行命令或访问外网，与「后台菜单权限」不是同一个风险维度：
// 给它硬套一个菜单键（例如把 bash 挂到 /plugin/index）会让任何能进插件区的管理员间接拿到
// 主机执行权。首版一律拒绝，等出现明确的「主机能力 ↔ 管理员权限键」设计再放开。
var channelBuiltinBlockedCaps = map[string]string{
	"read_file":      "主机文件读取首版关闭（规格 §7）",
	"write_file":     "主机文件写入首版关闭（规格 §7）",
	"edit_file":      "主机文件编辑首版关闭（规格 §7）",
	"search_replace": "主机文件批量替换首版关闭（规格 §7）",
	"bash":           "shell 执行首版关闭（规格 §7）",
	"grep":           "主机文件检索首版关闭（规格 §7）",
	"glob":           "主机文件遍历首版关闭（规格 §7）",
	"list_directory": "主机目录遍历首版关闭（规格 §7）",
	"web_fetch":      "联网抓取首版关闭（规格 §7）",
	"web_search":     "联网搜索首版关闭（规格 §7）",
}

// channelUnmappedBlockedCaps 没有后台菜单权限键、首版显式关闭的能力。
//
// 它们不是「忘了写映射」，而是**没有可用的权限键**：agent_*/task/skill_* 属于 AI 运行管理，
// 后台没有独立菜单页；attachment_upload / template_reload 没有等价的 REST 端点
// （见 pkg/mcp/intent/cap_routes.go 的说明），也就没有可继承的菜单权限。
// 规格 §7：无法映射权限的 cap 默认拒绝，必须显式建立映射才可配置——
// 这张表就是「显式登记为关闭」，而不是留白让它们落到「未登记→也拒绝」的模糊状态：
// 留白与关闭在行为上一致，但排障时看不出是有意关闭还是漏配。
var channelUnmappedBlockedCaps = map[string]string{
	"agent_create":      "AI 运行管理接口没有后台菜单权限键（首版关闭）",
	"agent_edit":        "AI 运行管理接口没有后台菜单权限键（首版关闭）",
	"agent_list":        "AI 运行管理接口没有后台菜单权限键（首版关闭）",
	"agent_delete":      "AI 运行管理接口没有后台菜单权限键（首版关闭）",
	"agent_toggle":      "AI 运行管理接口没有后台菜单权限键（首版关闭）",
	"agent_run":         "AI 运行管理接口没有后台菜单权限键（首版关闭）",
	"agent_chat":        "AI 运行管理接口没有后台菜单权限键（首版关闭）",
	"task":              "任务编排接口没有后台菜单权限键（首版关闭）",
	"skill_search":      "技能接口没有后台菜单权限键（首版关闭）",
	"skill_install":     "技能安装没有后台菜单权限键（首版关闭）",
	"attachment_upload": "上传能力没有等价 REST 端点，无法继承菜单权限（首版关闭）",
	"template_reload":   "模板重载无 REST 等价端点，无法继承菜单权限（首版关闭）",
}

// channelAPIPathBlacklist 是 api_invoke 的最终路径黑名单（R33）。
//
// 通用调用等价 shell：能列端点、能执行任意端点。即使 api_invoke 本身被开放，
// 也绝不允许借它进入渠道秘密、绑定、审批、策略/能力配置这些接口——
// 那等于「用渠道权限给自己发一张后台万能钥匙」，审批也拦不住（审批只管渠道侧写操作）。
//
// 匹配方式：normalize + path.Clean 后按**段边界**前缀匹配（大小写不敏感，见
// channelNormalizeInvokePath）。必须带边界，否则 "/system/api/channelsfoo" 会被误判。
var channelAPIPathBlacklist = []string{
	"/system/api/channels",
	"/system/api/admin",
	"/system/api/website",
	"/system/api/setting",
	"/system/api/anqi",
}

// CapPermission 是单个底层能力（cap）的权限判定结果。
type CapPermission struct {
	// MenuPath 是执行该能力所需的后台菜单权限路径（config.MenuGroups 的 Menu.Path）。
	MenuPath string
	// Blocked 为 true 表示首版不允许渠道调用，与管理员权限无关。
	Blocked bool
	// Reason 说明 blocked 或动态解析的原因，供审计与排障（不含任何敏感值）。
	Reason string
	// Dynamic 为 true 表示权限取决于调用参数——目前只有 api_invoke：
	// 必须按最终 method/path 解析，不能只看这个静态入口。
	Dynamic bool
	// Builtin 标记这是内置主机级能力（非端点能力）。
	Builtin bool
	// MissingEndpoint 为 true 表示这是**端点能力**，但该端点无法在端点目录中定位
	// （route/ 改名、api_catalog 过期或目录不可用）。这是最容易发生的漂移：
	// 行为上等同 blocked（默认拒绝），但必须与「有意关闭」区分开，否则
	// 「路由改名导致整类能力静默变成 blocked」不会让任何测试变红。
	MissingEndpoint bool
	// Method / Path 是能力等价的端点（Path 带 /system/api 前缀，仅端点能力有值）。
	Method string
	Path   string
}

// ChannelCapPermission 查询某个 cap 的权限映射，供 T6（调用前复核）与 T9（后台展示）使用。
// ok=false 表示该 cap 未登记：调用方必须按拒绝处理（未知 cap 一律拒绝）。
func ChannelCapPermission(capName string) (CapPermission, bool) {
	name := strings.TrimSpace(capName)
	if name == "" {
		return CapPermission{}, false
	}
	if reason, ok := channelBuiltinBlockedCaps[name]; ok {
		return CapPermission{Blocked: true, Builtin: true, Reason: reason}, true
	}
	if reason, ok := channelUnmappedBlockedCaps[name]; ok {
		return CapPermission{Blocked: true, Reason: reason}, true
	}
	switch name {
	case capAPIList, capAPISchema:
		return CapPermission{MenuPath: channelAPIMetaMenuPath, Reason: "通用入口权限（只读元数据）"}, true
	case capAPIInvoke:
		return CapPermission{MenuPath: channelAPIMetaMenuPath, Dynamic: true, Reason: "权限按最终 method/path 解析"}, true
	}

	// 端点能力：路径取自 pkg/mcp/intent 的 capEndpoints（能力 → 端点的唯一映射表），
	// 再到真实端点目录里确认该端点存在。目录缺失或查不到就拒绝——
	// 权限映射建立在「这个端点真的存在」之上，否则映射到的是一个可能被改成别的东西的路径。
	endpoint, ok := intent.CapEndpoints()[name]
	if !ok {
		return CapPermission{}, false
	}
	full := channelNormalizeEndpointPath(endpoint.Path)
	method := strings.ToUpper(strings.TrimSpace(endpoint.Method))
	cat, err := BuildAPICatalog()
	if err != nil {
		// 目录不可用与「端点查不到」是同一个问题的两种表现：映射失去了依据。
		// 都标 MissingEndpoint，让护栏测试把它们当漂移处理，而不是一条日志了事。
		return CapPermission{Blocked: true, MissingEndpoint: true, Method: method, Path: full, Reason: "API 目录不可用，无法映射权限"}, true
	}
	if _, found := cat.FindEndpoint(method, full); !found {
		return CapPermission{Blocked: true, MissingEndpoint: true, Method: method, Path: full, Reason: "端点目录中不存在 " + full + "，无法映射权限"}, true
	}
	menuPath, found := channelMenuPathForURI(strings.TrimPrefix(full, "/system/api"))
	if !found {
		return CapPermission{Blocked: true, Reason: "端点 " + full + " 没有对应的后台菜单权限键"}, true
	}
	return CapPermission{MenuPath: menuPath, Method: method, Path: full}, true
}

// channelCapHardBlocked 判断 cap 是否属于「首版显式关闭」。
//
// 与 ChannelCapPermission.Blocked 的区别：端点能力也可能因为查不到菜单键而 blocked，
// 那种情况下意图级门禁只把它记为「无候选菜单」，真正拒绝留给调用该 cap 时——
// 否则一条包含「发布/删除」端点的意图会在意图级被整条否掉，连其中的只读动作都不可用。
func channelCapHardBlocked(capName string) bool {
	if _, ok := channelBuiltinBlockedCaps[capName]; ok {
		return true
	}
	_, ok := channelUnmappedBlockedCaps[capName]
	return ok
}

// CheckIntentPermission 是意图执行前的完整门禁，校验顺序（不可重排）：
//
//	执行主体有效期 → 意图存在 → 站点曝光 → 智能体接待白名单 → 渠道策略 → 管理员状态与菜单权限
//
// 返回的 error 都可用 errors.Is 分流（见 channelPrincipal.go 的哨兵）。
// 判定只读库、不写任何服务级状态。
func (w *Website) CheckIntentPermission(ctx context.Context, p ExecutionPrincipal, intentName, action string) error {
	return w.checkIntentPermission(ctx, p, intentName, action, false)
}

// CheckIntentPermissionApproved 是「审批已通过」后的复核入口：必须先用审批凭据自证，
// 然后除 confirm 之外的门禁全部照旧生效（曝光/白名单/渠道策略其余取值/管理员状态与菜单权限）。
//
// 为什么单独给一个入口：审批通过后 T7 必须再复核一次权限（规格 §8「权限仍允许」），
// 若仍走 CheckIntentPermission，同一个 confirm 决策会让它再次返回 ErrChannelPolicyNeedsConfirm，
// 执行器只能二选一——死循环，或者干脆跳过整段复核。后者就是「确认即提权」。
//
// 自证的内容（R34 + R58）：approvalId + argsJson + 工具身份（intentName/action）必须对得上
// 该绑定在此策略修订号下的一条未过期的 approved/executing 记录。**不接收凭据就跳过 confirm
// 等于把 confirm 整条策略变成无效**——任何调用方只要改调本函数，就能绕过所有确认。
//
// R58 为什么必须比对工具身份：只比参数摘要时，两个工具的参数恰好一致（例如都只带
// {"action":"list","page":1}）就能「确认 seo_keyword、执行 content_article」。审批行本身
// 就记了 tool/action，它们是「管理员批准了哪一次操作」的身份字段，不是元数据。
//
// 边界：审批状态机的推进（消费 approved → executing）仍由 T7 负责，本函数只读复核。
// 因此 T7 必须先用同一 public_id 把记录条件更新为 executing（消费），再调本函数复核。
func (w *Website) CheckIntentPermissionApproved(ctx context.Context, p ExecutionPrincipal, approvalId string, argsJson string, intentName, action string) error {
	if err := w.checkChannelPrincipal(p); err != nil {
		return err
	}
	if err := w.checkChannelApprovalProof(ctx, p, approvalId, argsJson, intentName, action); err != nil {
		return err
	}
	return w.checkIntentPermission(ctx, p, intentName, action, true)
}

// checkChannelApprovalProof 校验审批凭据确实属于该执行主体，且**工具身份、参数与策略**
// 都未被改动。
//
// 查询必须带 site_id：public_id 是随机串但仍然按站点隔离查询，
// 免得任何一个站点的记录被另一个站点当成自己批的。任一条件不满足一律
// ErrChannelApprovalInvalid（不区分「不存在」与「不匹配」，避免把审批单号当探测通道，
// 也不暴露是哪一条不符）。
//
// R58：`tool` / `action` 必须逐字比对审批行。action 用调用方给的**原始值**
// （即从 ArgsJSON 里解析出的 action，解析不出就是空串），这里不做任何兜底：
// "*" 是 ChannelPolicy.Decision 内部的策略兜底语义，不是审批单的身份字段——
// 把空 action 当成 "*" 会让「批准了一个无动作调用」变成「批准了该工具的全部动作」。
func (w *Website) checkChannelApprovalProof(ctx context.Context, p ExecutionPrincipal, approvalId, argsJson, intentName, action string) error {
	publicId := strings.TrimSpace(approvalId)
	if publicId == "" {
		return fmt.Errorf("%w：缺少审批标识", ErrChannelApprovalInvalid)
	}
	argsHash := ChannelArgsHash(argsJson)
	if argsHash == "" {
		return fmt.Errorf("%w：审批参数无法解析", ErrChannelApprovalInvalid)
	}
	var approval model.ChannelApproval
	err := w.DB.WithContext(ctx).
		Where("`site_id` = ? AND `public_id` = ? AND `binding_id` = ? AND `tool` = ? AND `action` = ? AND `status` IN ? AND `expires_at` > ? AND `policy_revision` = ? AND `args_hash` = ?",
			p.SiteId, publicId, p.BindingId, strings.TrimSpace(intentName), action,
			[]string{model.ApprovalStatusApproved, model.ApprovalStatusExecuting},
			time.Now().Unix(), p.PolicyRevision, argsHash).
		Take(&approval).Error
	if err != nil {
		return fmt.Errorf("%w：审批记录不存在、已过期或与本次请求不匹配", ErrChannelApprovalInvalid)
	}
	return nil
}

func (w *Website) checkIntentPermission(ctx context.Context, p ExecutionPrincipal, intentName, action string, approved bool) error {
	if err := w.checkChannelPrincipal(p); err != nil {
		return err
	}
	if strings.TrimSpace(intentName) == "" {
		return fmt.Errorf("%w：（空）", ErrChannelIntentUnknown)
	}
	spec := channelFindIntent(intentName)
	if spec == nil {
		return fmt.Errorf("%w：%s", ErrChannelIntentUnknown, intentName)
	}
	// 站点曝光门禁：读该站点自己的 Mcp 配置（不是进程级主站点配置）。
	if err := w.checkChannelIntentExposure(intentName); err != nil {
		return err
	}
	channel, err := w.channelForPrincipal(ctx, p)
	if err != nil {
		return err
	}
	if err := w.checkChannelAgentWhitelist(ctx, p.AgentId, intentName); err != nil {
		return err
	}
	if err := w.checkChannelPolicy(channel, p, intentName, action, approved); err != nil {
		return err
	}

	menuPaths, hardBlocked := channelIntentMenuTargets(spec)
	if hardBlocked != "" {
		return fmt.Errorf("%w：%s", ErrChannelCapBlocked, hardBlocked)
	}
	return w.checkChannelAdmin(ctx, p, menuPaths)
}

// CheckCapPermission 是**每个**底层能力调用前的复核（T6 在 capInvoker 里调用）。
//
// 只做三件事：cap 映射（含 api_invoke 的动态解析）→ 管理员有效状态 → 菜单权限。
// 它不做曝光/白名单/渠道策略（那些属于意图级），也不做审批（属于 T7）。
// 但它同样要过 channelForPrincipal：渠道停用 / 仅通知 / 智能体对不上必须在
// 每次 cap 调用前也拒绝——意图级检查发生在会话开始时，中间渠道可能被停用。
func (w *Website) CheckCapPermission(ctx context.Context, p ExecutionPrincipal, capName, argsJSON string) error {
	if err := w.checkChannelPrincipal(p); err != nil {
		return err
	}
	if _, err := w.channelForPrincipal(ctx, p); err != nil {
		return err
	}
	name := strings.TrimSpace(capName)
	perm, ok := ChannelCapPermission(name)
	if !ok {
		return fmt.Errorf("%w：未登记的能力 %s", ErrChannelCapBlocked, name)
	}
	if perm.Blocked {
		return fmt.Errorf("%w：%s（%s）", ErrChannelCapBlocked, name, perm.Reason)
	}
	if name == capAPIInvoke {
		full, err := channelResolveInvokePath(argsJSON)
		if err != nil {
			return err
		}
		menuPath, found := channelMenuPathForURI(strings.TrimPrefix(full, "/system/api"))
		if !found {
			return fmt.Errorf("%w：端点 %s 没有对应的后台菜单权限键", ErrChannelCapBlocked, full)
		}
		perm.MenuPath = menuPath
	}
	if perm.MenuPath == "" {
		return fmt.Errorf("%w：%s 未映射后台菜单权限", ErrChannelCapBlocked, name)
	}
	return w.checkChannelAdmin(ctx, p, []string{perm.MenuPath})
}

// channelResolveInvokePath 解析 api_invoke 的最终端点路径并做黑名单/目录校验。
// 返回规范化后的完整路径（小写、带 /system/api 前缀、已 path.Clean）。
func channelResolveInvokePath(argsJSON string) (string, error) {
	var args apiInvokeArgs
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("%w：api_invoke 参数无法解析", ErrChannelCapBlocked)
		}
	}
	if strings.TrimSpace(args.Method) == "" || strings.TrimSpace(args.Path) == "" {
		return "", fmt.Errorf("%w：api_invoke 缺少最终 method/path，无法解析权限", ErrChannelCapBlocked)
	}
	full := channelNormalizeInvokePath(args.Path)
	if full == "" {
		return "", fmt.Errorf("%w：api_invoke 缺少最终 path", ErrChannelCapBlocked)
	}
	if channelAPIPathBlocked(full) {
		return "", fmt.Errorf("%w：%s", ErrChannelAPIPathBlocked, full)
	}
	method := strings.ToUpper(strings.TrimSpace(args.Method))
	cat, err := BuildAPICatalog()
	if err != nil {
		return "", fmt.Errorf("%w：API 目录不可用，拒绝解析权限", ErrChannelCapBlocked)
	}
	if _, found := cat.FindEndpoint(method, full); !found {
		return "", fmt.Errorf("%w：目录中没有端点 %s %s", ErrChannelCapBlocked, method, full)
	}
	return full, nil
}

// checkChannelPrincipal 校验执行主体本身的完整性（不读库）。
func (w *Website) checkChannelPrincipal(p ExecutionPrincipal) error {
	if w == nil || w.Id == 0 {
		return fmt.Errorf("%w：站点未初始化", ErrChannelPrincipalInvalid)
	}
	if p.SiteId == 0 || p.SiteId != w.Id {
		return fmt.Errorf("%w：执行主体站点与当前站点不一致", ErrChannelPrincipalInvalid)
	}
	if p.ChannelId == 0 || p.BindingId == 0 || p.AdminId == 0 {
		return fmt.Errorf("%w：渠道/绑定/管理员标识缺失", ErrChannelPrincipalInvalid)
	}
	return nil
}

// channelForPrincipal 取执行主体对应的渠道（站点内），并校验渠道处于「可管理」状态。
//
// 三道判定：
//  1. 渠道存在且属于当前站点；
//  2. enabled = 1（连接启用）且 chat_enabled = 1（开启聊天管理）。仅通知渠道
//     （chat_enabled = 0）只投递通知，不该执行任何管理动作。Enabled 虽然由 T4 的
//     worker 生命周期兜住，但租约与 worker 是「连接层」，权限层不能把 fail-closed
//     寄托在另一层没漏判上——这是本层最合适的拒绝点；
//  3. 执行主体声明的智能体必须非 0 且与渠道配置的接待智能体一致（R35）。否则
//     会话切到另一个智能体后，被执行的智能体的接待白名单完全不参与判定。
func (w *Website) channelForPrincipal(ctx context.Context, p ExecutionPrincipal) (*model.MessageChannel, error) {
	var ch model.MessageChannel
	err := w.DB.WithContext(ctx).
		Where("`id` = ? AND `site_id` = ?", p.ChannelId, w.Id).
		Take(&ch).Error
	if err != nil {
		return nil, fmt.Errorf("%w：渠道不存在或不属于当前站点", ErrChannelPrincipalInvalid)
	}
	if ch.Enabled != 1 || ch.ChatEnabled != 1 {
		return nil, fmt.Errorf("%w：渠道 %d", ErrChannelDisabled, ch.Id)
	}
	if p.AgentId == 0 || p.AgentId != ch.AgentId {
		return nil, fmt.Errorf("%w：执行主体智能体 %d 与渠道接待智能体不一致", ErrChannelAgentNotAllowed, p.AgentId)
	}
	return &ch, nil
}

// checkChannelIntentExposure 是站点曝光门禁：读该站点自己的 ExposedIntents/ExposedTools。
//
// 必须用 w.LoadAiSetting("").Mcp，而不是 provider/mcp.go 的 GetMcpConfig()——
// 后者只返回主站点配置，多站点场景下等于让所有站点共用主站点的暴露面（R32）。
// 曝光规则本身复用 intent.IntentAllowed，与 MCP/Eino 通道同源（R29）。
func (w *Website) checkChannelIntentExposure(intentName string) error {
	if intent.IntentAllowed(w.channelExposureConfig(), intentName) {
		return nil
	}
	return fmt.Errorf("%w：%s", ErrChannelIntentNotExposed, intentName)
}

// channelExposureConfig 把站点自己的 MCP 配置转成内核可用的曝光配置。
func (w *Website) channelExposureConfig() intent.Config {
	if w == nil {
		return intent.Config{}
	}
	mcp := w.LoadAiSetting("").Mcp
	return intent.Config{
		ExposedIntents: mcp.ExposedIntents,
		ExposedTools:   mcp.ExposedTools,
	}
}

// checkChannelAgentWhitelist 校验**本次执行的智能体**（principal.AgentId）是否被授权使用该意图。
// 无记录 / 空数组 / 解析失败 / 未列名 → 拒绝；**不支持通配符**（不发明 "*"）。
//
// 按 principal.AgentId 而不是 channel.AgentId 查：两者已被 channelForPrincipal 强制相等，
// 但用执行主体取值能保证「白名单校验的主体 = 执行主体」这一条在代码上直接可见，
// 不会因为将来放宽渠道判定而静默换成另一个智能体的策略。
func (w *Website) checkChannelAgentWhitelist(ctx context.Context, agentId uint, intentName string) error {
	if agentId == 0 {
		return fmt.Errorf("%w：渠道未配置接待智能体", ErrChannelAgentNotAllowed)
	}
	var policy model.AgentReceptionPolicy
	err := w.DB.WithContext(ctx).
		Where("`site_id` = ? AND `agent_id` = ?", w.Id, agentId).
		Take(&policy).Error
	if err != nil {
		return fmt.Errorf("%w：智能体 %d 没有接待策略", ErrChannelAgentNotAllowed, agentId)
	}
	if strings.TrimSpace(policy.AllowedIntents) == "" {
		return fmt.Errorf("%w：智能体 %d 的接待白名单为空", ErrChannelAgentNotAllowed, agentId)
	}
	var allowed []string
	if err := json.Unmarshal([]byte(policy.AllowedIntents), &allowed); err != nil {
		return fmt.Errorf("%w：智能体 %d 的接待白名单无法解析", ErrChannelAgentNotAllowed, agentId)
	}
	for _, name := range allowed {
		if strings.TrimSpace(name) == intentName {
			return nil
		}
	}
	return fmt.Errorf("%w：智能体 %d 不允许 %s", ErrChannelAgentNotAllowed, agentId, intentName)
}

// checkChannelPolicy 校验渠道策略修订号与「意图 + 动作」裁决。
// approved=true 时 confirm 视为通过（审批后的复核）。
func (w *Website) checkChannelPolicy(ch *model.MessageChannel, p ExecutionPrincipal, intentName, action string, approved bool) error {
	if int64(ch.Revision) != p.PolicyRevision {
		return fmt.Errorf("%w：渠道策略修订号 %d 与执行主体 %d 不一致", ErrChannelPolicyStale, ch.Revision, p.PolicyRevision)
	}
	policy, err := ParseChannelPolicy(ch.PolicyJson)
	if err != nil {
		// 解析失败一律拒绝：格式错误不能被当成「没配置、没配置就放行」。
		return fmt.Errorf("%w：渠道策略无法解析，按拒绝处理", ErrChannelPolicyDenied)
	}
	switch policy.Decision(intentName, action) {
	case PolicyDecisionAllow:
		return nil
	case PolicyDecisionConfirm:
		if approved {
			return nil
		}
		return fmt.Errorf("%w：%s.%s", ErrChannelPolicyNeedsConfirm, intentName, action)
	default:
		return fmt.Errorf("%w：%s.%s", ErrChannelPolicyDenied, intentName, action)
	}
}

// checkChannelAdmin 校验绑定有效性、管理员当前状态与菜单权限。
//
// menuPaths 为空表示「意图没有可静态解析的后台入口」——此时只校验主体与管理员状态，
// 精确的菜单权限由每次 cap 调用前的 CheckCapPermission 负责（那一步拿得到最终端点）。
func (w *Website) checkChannelAdmin(ctx context.Context, p ExecutionPrincipal, menuPaths []string) error {
	// 绑定是执行主体的身份凭据：撤销、换绑、换渠道后必须立刻失效（规格 §6）。
	// 放在这里而不是只信 principal 里的 AdminId——principal 是调用方造的，
	// 而审批点击（T7）发生在很久之后，期间绑定可能已被撤销。
	if err := w.checkChannelBinding(ctx, p); err != nil {
		return err
	}
	admin, err := w.GetAdminInfoById(p.AdminId)
	if err != nil || admin == nil || admin.Status != 1 {
		return fmt.Errorf("%w：管理员 %d", ErrChannelAdminInvalid, p.AdminId)
	}
	// 超管口径与 middleware.AdminPermission 一致：Id==1 或 GroupId==1 免菜单前缀匹配。
	// 但免不了上面的 blocked cap / 曝光 / 策略门禁——那些在其它层已经拦过了。
	if admin.Id == 1 || admin.GroupId == 1 {
		return nil
	}
	if len(menuPaths) == 0 {
		return nil
	}
	if admin.Group == nil || admin.Group.Status != 1 {
		return fmt.Errorf("%w：管理员分组不存在或已停用", ErrChannelPermissionDenied)
	}
	for _, required := range menuPaths {
		for _, permission := range admin.Group.Setting.Permissions {
			if strings.TrimSpace(permission) == "" {
				continue
			}
			if strings.HasPrefix(required, permission) {
				return nil
			}
		}
	}
	sort.Strings(menuPaths)
	return fmt.Errorf("%w：需要菜单权限 %s", ErrChannelPermissionDenied, strings.Join(menuPaths, " 或 "))
}

// checkChannelBinding 校验绑定仍有效且属于当前站点/渠道/管理员。
func (w *Website) checkChannelBinding(ctx context.Context, p ExecutionPrincipal) error {
	var binding model.ChannelBinding
	err := w.DB.WithContext(ctx).
		Where("`id` = ? AND `site_id` = ? AND `channel_id` = ? AND `admin_id` = ? AND `revoked_at` = 0",
			p.BindingId, w.Id, p.ChannelId, p.AdminId).
		Take(&binding).Error
	if err != nil {
		return fmt.Errorf("%w：绑定已撤销、已换绑或不属于该渠道", ErrChannelPrincipalInvalid)
	}
	return nil
}

// channelIntentMenuTargets 汇总一条意图「可能触达的后台菜单权限路径」。
//
// 为什么要两条来源：
//   - 声明的 cap（Caps 字段）覆盖 switchCompose/Delegate 型意图；
//   - 意图声明的端点（invokeRoutes + capEndpoints 的 intent:<name> 归属）覆盖只写
//     Compose、不写 Caps 的合并意图——那批意图的端点对意图层是静态已知的，
//     这里读同一份声明，避免再维护一张平行的「意图 → 端点」表。
//
// 返回的 hardBlocked 非空表示意图包含「首版显式关闭」的能力（fs/shell/未映射 cap），
// 整条意图直接拒绝：这类能力不是权限不足，而是渠道根本不该有。
func channelIntentMenuTargets(spec *intent.IntentSpec) ([]string, string) {
	if spec == nil {
		return nil, ""
	}
	var paths []string
	seen := map[string]bool{}
	add := func(menuPath string) {
		if menuPath == "" || seen[menuPath] {
			return
		}
		seen[menuPath] = true
		paths = append(paths, menuPath)
	}

	for _, capName := range spec.Caps {
		if channelCapHardBlocked(capName) {
			return nil, fmt.Sprintf("意图 %s 包含首版关闭的能力 %s", spec.Name, capName)
		}
		perm, ok := ChannelCapPermission(capName)
		if !ok {
			return nil, fmt.Sprintf("意图 %s 声明了未登记的能力 %s", spec.Name, capName)
		}
		// blocked（端点查不到菜单键）与 dynamic（api_invoke 按参数解析）都不产生静态候选：
		// 前者在调用该 cap 时拒绝，后者在 CheckCapPermission 里按最终路径解析。
		if perm.Blocked || perm.Dynamic {
			continue
		}
		add(perm.MenuPath)
	}

	for key, vias := range intent.DeclaredEndpointTargets() {
		declared := false
		for _, via := range vias {
			if via == "intent:"+spec.Name {
				declared = true
				break
			}
		}
		if !declared {
			continue
		}
		parts := strings.Fields(key)
		if len(parts) != 2 {
			continue
		}
		if menuPath, ok := channelMenuPathForURI(strings.TrimPrefix(parts[1], "/system/api")); ok {
			add(menuPath)
		}
	}
	return paths, ""
}

// channelFindIntent 按名取意图声明（找不到返回 nil）。
func channelFindIntent(name string) *intent.IntentSpec {
	for _, spec := range intent.IntentCatalog {
		if spec != nil && spec.Name == name {
			return spec
		}
	}
	return nil
}

// channelMenuPathForURI 把后台 URI（已去掉 /system/api 前缀）解析成菜单权限路径。
//
// 口径与 middleware.AdminPermission 完全一致（按 config.MenuGroups() 的 Menu.Backend
// 前缀匹配，取第一个命中的 Menu.Path）。唯一区别：找不到菜单时这里返回 false，
// 由调用方按「默认拒绝」处理——后台登录对未知菜单是放行的，渠道必须反过来。
func channelMenuPathForURI(uri string) (string, bool) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", false
	}
	for _, group := range config.MenuGroups() {
		if group == nil {
			continue
		}
		for _, m := range group.Menus {
			if m == nil || m.Backend == "" || m.Path == "" {
				continue
			}
			if strings.HasPrefix(uri, m.Backend) {
				return m.Path, true
			}
		}
	}
	return "", false
}

// channelNormalizeEndpointPath 把端点路径补全成 /system/api 前缀并 Clean。
func channelNormalizeEndpointPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if p == "/system/api" || strings.HasPrefix(p, "/system/api/") {
		return path.Clean(p)
	}
	if p == "/api" || strings.HasPrefix(p, "/api/") {
		return path.Clean("/system" + p)
	}
	return path.Clean("/system/api" + p)
}

// channelNormalizeInvokePath 把 api_invoke 的 path 归一成「小写 + /system/api 前缀 + Clean」。
//
// 与 provider/api_tools.go 的 normalizeAPIPath 有两点刻意不同：
//  1. 前缀判断不区分大小写：调用方写 /SYSTEM/API/channels/detail 时，原实现会拼成
//     /system/api/SYSTEM/API/...，虽然执行时也会 404，但黑名单匹配就失效了；
//  2. 结果统一小写：黑名单比较不依赖调用方的大小写写法。
//
// 允许的侧信道：权限侧归一比执行侧更宽松时，最坏结果是「判为允许、执行 404」，
// 不会出现「判为拒绝、其实能执行」。
func channelNormalizeInvokePath(raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	lower := strings.ToLower(p)
	switch {
	case lower == "/system/api" || strings.HasPrefix(lower, "/system/api/"):
		// 已带完整前缀
	case lower == "/api" || strings.HasPrefix(lower, "/api/"):
		p = "/system" + p
	default:
		p = "/system/api" + p
	}
	return strings.ToLower(path.Clean(p))
}

// channelAPIPathBlocked 按段边界判断路径是否命中黑名单。
// 入参必须是 channelNormalizeInvokePath 的结果（已小写、已 Clean）。
func channelAPIPathBlocked(normalizedPath string) bool {
	for _, prefix := range channelAPIPathBlacklist {
		if normalizedPath == prefix || strings.HasPrefix(normalizedPath, prefix+"/") {
			return true
		}
	}
	return false
}

// assembleCapHandlers 装配「底层能力」表：端点/内置 handler + api_* 元能力。
//
// 与 NewAiChatService 用的是同一份装配逻辑（抽出本函数就是为了让权限映射的
// TestEveryCapHasPermissionDecision 能遍历真实 capHandlers，而不是在测试里
// 手抄一份能力清单——手抄的清单永远不会因为新增 cap 而变红）。
func (svc *AiChatService) assembleCapHandlers() map[string]toolHandler {
	_, endpointHandlers := svc.getEinoTools()
	_, builtinHandlers := svc.getBuiltinEinoTools()
	caps := make(map[string]toolHandler, len(endpointHandlers)+len(builtinHandlers)+3)
	for name, handler := range endpointHandlers {
		caps[name] = handler
	}
	for name, handler := range builtinHandlers {
		caps[name] = handler
	}
	caps[capAPIList] = svc.capAPIList
	caps[capAPISchema] = svc.capAPISchema
	caps[capAPIInvoke] = svc.capAPIInvoke
	return caps
}
