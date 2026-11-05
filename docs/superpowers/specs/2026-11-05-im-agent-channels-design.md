# 站点智能体接入飞书／钉钉：第一版规格

状态：待审阅；不代表已实现或已通过平台联调。
日期：2026-11-05。

## 1. 目标与已确认范围

[KNOWN] 用户已确认以下产品决策：

- 功能作为 AnQiCMS 通用能力，各站长自行配置企业自建应用，网站直连平台，无安企中转。
- 智能体在 AnQiCMS 内配置；渠道由站点统一配置并指定一个接待智能体。
- 第一版只支持飞书长连接、钉钉 Stream，不提供 HTTP 回调入口。
- 已登录后台管理员生成一次性绑定 token，有效期为 1 小时；私聊完成绑定后无需重复输入。
- 管理能力像 MCP 管理一样动态配置，不将可用功能范围写死；执行受管理员当前权限约束。
- 私聊允许管理；群聊默认只通知，逐群显式开启后仅响应已绑定用户的 @消息。
- 通知接收目标同时支持批准的群聊、已绑定管理员私聊。
- 保留邮件配置与发送行为，复用模板机制，不直接通用化邮件 HTML 输出。
- 第一版即时通知：新留言、新订单、支付成功、智能体任务失败。定时摘要不在首版。

[INFERRED] 本文剩余接口、表结构及运行默认值是供审阅的工程设计，不是用户逐项指定的既有事实。批准本文后才将其作为实现契约。

## 2. 现有实现与必须解决的缺口

[KNOWN] 以下位置已核对源码：

| 位置 | 现状及约束 |
|---|---|
| `config/plugin.go:300` | EmailTemplate 有 open/type/key/name/description/readonly/delay/subject/content；type 不是渠道。 |
| `provider/sendmail.go:661` | Pongo2 替换变量后转换 HTML，聊天输出不能直接调用完整邮件渲染器。 |
| `provider/sendmail.go:995` | 新订单模板同时面向买家和管理员，open 对两类发送的控制不同；本项目不顺带修改旧语义。 |
| `provider/brevo.go:136` | CreateEvent 是营销平台事件上报，不是通知事件总线。 |
| `model/aiAgent.go:4` | 智能体主要保存策略、调度与轮数，没有独立模型、技能或能力白名单字段。不得假称这些配置已经存在。 |
| `provider/aiChat.go:127` | 服务具有站点上下文及共用工具表；技能范围有服务级状态，新增渠道不能通过修改共享状态实现用户隔离。 |
| `provider/aiMiddleware.go:407` | IsAgent 会跳过审批；渠道不能直接调用免审批路径。 |
| `middleware/auth.go:80` | ValidateToken 只校验登录凭据，不提供资源权限判定。 |
| `middleware/auth.go:148` | 后台权限依赖菜单路径映射，未知菜单路径默认不拦截；渠道必须采用未知能力拒绝的策略。 |
| `controller/guestbook.go:147` | 留言先保存，随后异步反垃圾校验和 ProcessGuestbook。通知不能只挂邮件分支，也不能忽略反垃圾结果。 |
| `provider/order.go:792,1020` | 支付成功与订单创建落点；接入前须检查条件更新影响行数与并发，避免重复事件。 |
| `provider/dao.go:121` | 迁移受版本变化或 force 控制；同版本部署需要显式迁移步骤。 |
| `config/menu.go`、`anqicms-admin/config/routes.ts` | 菜单同时作为权限键，新增页面与接口必须同步。 |

## 3. 架构与边界

[INFERRED] 分成四个可单测模块，通过明确接口连接，不引入外部消息队列：

1. 平台适配器：凭据鉴权、长连接、事件标准化、发送文本／卡片与错误分类。
2. 渠道服务：连接所有权、消息去重、身份绑定、目标批准、会话调度。
3. 授权聊天执行器：读取接待智能体配置，以实际管理员身份执行工具、确认和审计。
4. 通知服务：事件持久化、规则匹配、受限模板渲染、逐目标投递和重试。

[INFERRED] 普通通知不调用模型。接收消息先持久化或识别为重复，再应答平台；不能在平台回调中等待模型。若 token 绑定消息入站，仅消费并记录不含原文的处理结果，不持久化 token 正文。

[INFERRED] 现有技能与模型配置能复用多少，以真实字段为准：首版沿用站点模型，采用所选 AiAgent 的 Strategy、MaxRounds；新增接待能力配置。暂不新增按智能体切模型的编辑器，不扩大为智能体平台重构。

## 4. 数据契约

[INFERRED] 新数据均包含 site_id，所有读写携带站点过滤，即使当前每站点有独立数据库也不省略。ID 使用项目现有无符号主键习惯；平台字符串用足够长度字段，唯一约束使用规范化身份的 SHA-256 摘要，避免复合长索引超限。

| 模型／建议表 | 核心字段与唯一性 |
|---|---|
| MessageChannel / message_channels | site_id、platform(feishu/dingtalk)、name、enabled、chat_enabled、agent_id、app_id、组织标识、secret（不序列化）、policy_json、revision；同平台应用只允许归属一个站点连接，避免共享凭据导致平台随机分流。 |
| AgentReceptionPolicy / agent_reception_policies | site_id、agent_id、allowed_intents；首版沿用站点模型及原智能体策略，无授权配置时默认无管理能力。 |
| ChannelBindingToken / channel_binding_tokens | site_id、channel_id、admin_id、token_hash、expires_at、consumed_at；token_hash 唯一。 |
| ChannelBinding / channel_bindings | site_id、channel_id、组织与用户标识、admin_id、revoked_at、revision；完整平台身份摘要唯一。 |
| ChannelTarget / channel_targets | site_id、channel_id、kind(group/private)、platform_target_id、binding_id、approved、management_enabled；目标身份摘要唯一。私聊目标来自有效绑定。 |
| ChannelInbox / channel_inboxes | site_id、channel_id、平台事件摘要、event_kind、脱敏正文、状态、lease_until；平台事件摘要唯一，绑定消息不留 token。 |
| ChannelSession / channel_sessions | 站点、渠道、会话、用户摘要、内部 chat_session_id、generation；唯一摘要包含上述全部隔离维度。 |
| ChannelApproval / channel_approvals | 操作者 binding_id、session_id、tool/action、canonical_args、args_hash、expires_at、policy_revision、状态、执行结果摘要；随机公开确认 ID，原子消费。 |
| NotificationRule / notification_rules | site_id、name、event_type、enabled、target_ids、template_id；首版按事件类型匹配，不增加任意表达式条件语言。 |
| NotificationTemplate / notification_templates | site_id、key、name、subject、content、platform_overrides_json；保留通用聊天文案和平台覆盖，不存平台 JSON。 |
| NotificationEvent / notification_events | site_id、event_type、业务键、发生时间、白名单 payload、状态；业务事件键唯一。 |
| NotificationDelivery / notification_deliveries | site_id、event_id、rule_id、target_id、状态、attempts、next_attempt_at、lease_until、last_error、platform_message_id；event/rule/target 唯一。 |
| ChannelLease / channel_leases | 应用归属摘要、owner、expires_at、fencing_version；每个应用摘要一个活动所有者。 |
| ChannelAudit / channel_audits | site_id、channel_id、binding_id、task_id、action、目标摘要、结果、时间；不保存秘密和完整敏感正文。 |

[INFERRED] 后台 API 从不返回密钥和 token 摘要。绑定 token 明文仅创建响应返回一次，响应 no-store；令牌正文不进入访问日志。应用密钥沿用现有站点配置的存储安全边界，不宣称已具备静态加密；读取掩码，保存识别掩码哨兵保留原值。新表访问、备份需等同其他站点凭据保护。

## 5. 生命周期与连接所有权

[INFERRED] 首版运行默认值：连接租约 30 秒，每 10 秒续租；失去租约立即取消本地接收与后续调度，所有持久化领取以 fencing_version 校验。发送前再次校验所有权；平台侧不支持栅栏时不能宣称绝无在途重复。

[INFERRED] 站点加载后启动连接管理器；保存配置后按 revision 重建；关闭、站点删除、站点重载及进程关闭均取消上下文并等待退出。认证错误进入错误状态，避免无限快速重连；网络重连采用带抖动指数退避，1 秒起、60 秒封顶。

[INFERRED] 相同应用跨多个站点配置必须在共享站点注册处拒绝；对不共享数据库的独立安装，无法全局阻止重复部署，界面明确要求一个企业应用仅交给一个安装使用。

## 6. 身份绑定

[INFERRED] 生成 32 字节安全随机 token，使用 URL 安全编码；expires_at = 创建时间 + 3600 秒，now >= expires_at 时拒绝。限定站点、渠道和生成者；重新生成使该管理员该渠道未消费 token 失效。输入尝试限速，不能回显 token。

[INFERRED] 仅在验证过的平台应用私聊中处理绑定；数据库事务原子完成校验、消费、绑定及私聊目标创建。重复消费、竞争消费、跨渠道和身份冲突均拒绝。不得静默覆盖已有管理员绑定；换绑先撤销。

[INFERRED] 平台用户不知道对应管理员前，除绑定与帮助外不进入模型。撤销绑定同时撤销其未执行审批和排队任务；管理员状态以当前数据判断，不猜 Status 数值，沿用真实后台登录有效状态规则。

## 7. 权限与聊天执行

[INFERRED] 每次实际调用取交集：站点 ExposedIntents/ApiExposure、智能体能力白名单、当前管理员权限、渠道 action 策略。审批不能扩大该交集。模型可见性过滤与执行前拦截同时存在，不能只靠提示词。

[INFERRED] 渠道策略按 intent + action 为 deny/confirm/allow；未配置默认 deny。新增能力不自动进入已有策略。风险等级来自现有风险分类并参与默认展示，不替代权限。

[INFERRED] 为委托端点解析现有后台菜单权限；同时在每个 cap 实际调用前校验，覆盖 Compose 多次委托与 api 通用调用。无法映射权限的 cap 默认拒绝，必须显式建立能力到权限键映射才可配置。fs/shell/权限与凭据操作首版默认关闭；允许配置也必须有明确的管理员权限映射，不因超管或人工确认而绕过站点门禁。通用 api 调用检查最终 method/path，禁止借其进入渠道秘密、绑定、审批或能力配置接口。

[INFERRED] 请求级 ExecutionPrincipal 持有 site/admin/binding/session/policy revision；所有策略放入 context 或会话局部数据，不修改服务级工具列表、技能范围或全局 MCP scope。执行前重新读取管理员有效状态与权限。不同站点、用户、群聊与私聊不共享上下文；会话切换智能体或新建会话增加 generation，使旧待执行工作失效。

[INFERRED] 同会话串行，取消终止后续模型与工具调用但不回滚已完成操作。首版只接收文本及确认交互，不处理语音、图片、文档命令。群聊只有批准且开启管理、明确 @应用且已绑定的消息能进入模型。

[INFERRED] 群聊执行使用群安全结果模式：未具有明确群安全输出契约的工具不在群中执行，提示用户私聊。首版群安全工具仅输出帮助、任务状态及脱敏业务计数；不得将任意工具原始结果或历史敏感内容交给群回复模型后再脱敏。

## 8. 审批

[INFERRED] 首版审批确认用**私聊文本指令**，不发确认卡片：绑定用户在私聊里回 `批准 <公开ID>` 或 `拒绝 <公开ID>`（公开 ID 是审批行的短标识，不含站点/绑定内部主键）。理由：飞书 ws SDK v3.12.2 无法接收卡片回调（`WithCardHandler` 在依赖里被注释掉），钉钉 Stream 侧也只有消息归一化、没有卡片发送能力，做卡片等于两端都缺一半。文本指令只认同一有效绑定身份的私聊消息；群聊里**不提供**审批指令入口（群里只做批准目标的任务提交，审批必须在私聊完成）。卡片交互（按钮、卡片状态回写）留到后续版本，届时仍需保留文本指令作为降级路径。

[INFERRED] 确认默认有效 10 分钟。确认内容仅展示白名单的操作摘要，不包括密钥；绑定用户、组织、应用、站点、会话、精确规范化参数和策略 revision。确认必须来自同一有效绑定身份，权限仍允许且对象前置条件仍满足；参数或策略变化必须重新确认。

[INFERRED] 状态：pending → approved → executing → succeeded/failed/unknown；pending 也可到 rejected/expired/cancelled。确认消费采用条件更新；重复点击不重复执行。重启将 pending/approved 取消，将 executing 标为 unknown，不自动重放写操作；提示到后台核查。能力调用前检查审批仍对应同一参数，后续多步写操作分别处理。

## 9. 通知事件、模板及投递

[INFERRED] 事件键：guestbook.created（留言 ID）、order.created（订单号）、order.paid（订单号）、agent.failed（运行 ID）。同一业务事件只创建一次；人工重复执行智能体任务有新的运行 ID。

[INFERRED] 订单事件不依赖邮件账号、买家邮箱、Brevo 或登录状态。对匿名订单同样产生新订单事件。支付只由真实状态迁移赢家产生 paid 事件；更新后检查 RowsAffected，不能将已完成订单重复转回 paid。

[INFERRED] 留言在保存时事务内写入 waiting_classification 事件；分类结果正常后才就绪，垃圾留言取消。待分类事件由后台工作者恢复处理，不依赖原请求对象或仅一个 goroutine；分类服务暂不可用则保持待处理并展示原因，不把未经确认的留言直接外发。已有邮件和推送路径行为不变。

[INFERRED] 有事务的业务写入与事件保存同事务，数据库故障按原业务错误返回；外部通知失败不会回滚已完成业务。没有启用规则时不创建外发任务。事件拆分任务过程可重试且有唯一约束，规则修改不补发历史事件；手动重试只能针对已有失败投递，并再次检查规则和权限。

[INFERRED] 同一基础聊天模板支持标题和正文、可选平台覆盖。沿用 Pongo2 变量语法，但使用受限上下文及允许的过滤器／标签集合，禁止 system 查询、文件加载、模板 include/import/extends、自定义业务查询标签。不能把完整 Website/User/Order 对象传入模板；渲染错误不发送。平台 Markdown 转义、消息长度限制和分段由适配器处理；访客内容不能触发用户提及或卡片操作。

[INFERRED] 群模板仅有站点名、事件、业务标识、时间、金额／状态等摘要；不提供联系方式、地址、留言原文或任意 ExtraData。管理员私聊按业务权限提供白名单字段。模板预览使用同样字段约束和示例数据，不允许通过模板覆盖绕过群脱敏。

[INFERRED] 逐目标任务状态 pending/sending/retry/sent/failed/cancelled/unknown。建议最多 6 次发送尝试，首次后重试间隔 10 秒、30 秒、2 分钟、10 分钟、30 分钟；平台 Retry-After 优先。可明确判定临时失败才重试；认证／权限／目标失效为永久失败。结果不确定时标 unknown，显示重复风险后才允许人工重试。不承诺端到端 exactly-once。

[INFERRED] 平台应用可能只能接收但没有主动发送权限；测试须覆盖不依赖临时回复凭据的主动群聊和私聊发送。未验证通过的目标不显示为已可用。任务失败事件不会因其自身通知失败再产生 agent.failed，避免递归。

## 10. 管理 API 与前端

[INFERRED] 路由前缀为 /system/api/channels，全部要求后台身份。配置管理使用新增菜单权限 /plugin/channels；个人绑定 API 仅允许操作当前管理员自身，管理他人绑定另需配置管理权限。不能因现有未匹配菜单默认放行而省略显式授权。

| 方法及相对路径 | 责任 |
|---|---|
| GET / | 当前站点连接列表，掩码 |
| GET /detail?id= | 连接详情、策略 |
| POST /detail | 新建／更新连接，带 revision 乐观锁 |
| POST /delete | 关闭并移除连接，取消未执行工作；审计保留 |
| GET /status?id= | 连接和工作者状态 |
| GET /capabilities | 能力、动作、权限映射、风险及当前可配置状态 |
| POST /test | 配置验证／向已批准目标发示例消息 |
| POST /binding/token | 当前管理员生成指定渠道的一次性 token |
| GET /bindings | 默认只返回本人；管理员查看全部需额外授权 |
| POST /binding/revoke | 本人解绑或获授权撤销他人 |
| GET /targets、POST /target | 目标列表、批准群、群管理开关 |
| GET /rules、POST /rule、POST /rule/delete | 通知规则管理 |
| GET /templates、POST /template、POST /template/preview | 模板管理、按目标安全级别预览 |
| GET /records | 投递、任务、审批、审计分页筛选；秘密不返回 |
| POST /delivery/retry | 对失败／未知结果的投递显式重试 |

[INFERRED] 标准响应沿用 {code,msg,data}。前端五个区域：渠道连接、身份绑定、会话目标、通知规则（含模板）、运行记录。绑定明文仅一次展示、可复制及倒计时，不存 localStorage。多 tab 表单按项目约定保留字段，提交全量相关字段；凭据错误、主动发送权限缺失、断线与管理员失效要有不同提示。

## 11. 验收与发布门槛

[INFERRED] 自动测试覆盖：token 3600 秒边界／竞争消费／跨站点、动态撤权、cap 绕过、群泄露、审批重复与重启、事件和业务回滚一致、匿名订单、支付并发、反垃圾恢复、重复事件、目标撤销、发送不确定、租约丢失和关闭清理。护栏测试必须做反向对照证明错误实现会失败。

[INFERRED] 实际平台验收分别验证两平台：接收文本、私聊绑定、主动群／私聊通知、群 @隔离、确认交互、重连、长文本、权限撤销、错误凭据；没有测试应用和凭据时只能报告阻塞，不得用 mock 宣称平台通过。

[INFERRED] 迁移使用现有模型注册机制；开发同版本运行明确执行后台修复数据库，发布版本迁移另按发布流程处理，不在功能开发时随意改版本。启动缺表不得悄悄对业务库自动破坏性修复；渠道保持停用并显示迁移要求。

[INFERRED] 首版验收不包含 HTTP 回调、定时摘要、语音／图片管理、平台托管智能体、多智能体路由、全量邮件迁移。既有邮件、后台对话、定时智能体任务不改变行为。

## 12. 平台事实与前置核验

[KNOWN] 官方文档描述长连接接收卡片回调和 Stream 接收机器人／事件／卡片回调。已做公开文档检索，未取得站长测试应用凭据，未做租户权限和主动发送真实探测。实现任务必须先核对 Go SDK 签名、权限、组织标识来源、用户与群标识，以及卡片回调是否可通过所选连接接收，不能凭经验猜 API。

- 飞书卡片回调：https://open.feishu.cn/document/uAjLw4CM/ukzMukzMukzM/feishu-cards/handle-card-callbacks?lang=zh-CN
- 钉钉 Stream：https://open.dingtalk.com/document/resourcedownload/Introduction-to-stream-mode

[INFERRED] SDK 版本、实际权限名称和租户测试结果记录到实现审查结果；没有真实凭据不阻止离线开发，但阻止宣布接入完成。
