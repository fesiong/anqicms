# 站点智能体接入飞书／钉钉：第一版实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为每个站点提供飞书长连接、钉钉 Stream 两个企业应用渠道，实现一次性 token 绑定后的聊天管理，以及新留言／新订单／支付成功／智能体失败四类即时通知。

**架构：** 平台适配器负责连接与收发；渠道服务负责租约、去重、绑定、目标与会话；授权聊天执行器复用现有智能体与意图工具，但在每个能力调用前按实际管理员权限校验；通知服务使用数据库持久化事件、规则、受限模板和逐目标投递，不引入外部消息队列。

**技术栈：** Go 1.26、Iris、GORM、Eino、飞书开放平台 Go SDK、钉钉 Stream SDK（版本在任务 3 锁定）。

**规格：** `docs/superpowers/specs/2026-11-05-im-agent-channels-design.md`

## 全局约束

- 规格第 5 节运行默认值逐字生效：租约 30 秒、每 10 秒续租、退避 1 秒起 60 秒封顶；绑定 token 有效期 3600 秒；审批有效期 10 分钟；投递最多 6 次，间隔 10 秒、30 秒、2 分钟、10 分钟、30 分钟。
- 所有新表带 `site_id`，所有查询带站点过滤；ID 唯一性使用规范化身份的 SHA-256 摘要。
- 后台响应不得包含应用密钥、token 明文或摘要；密钥字段沿用掩码哨兵回写保护。
- 管理能力默认拒绝：未配置策略的 intent 不可执行；cap 无权限映射即拒绝，包括超管。
- 复用现有 `AiChatService`、意图工具与风险分类，但不走 `IsAgent` 免审批路径，不修改服务级 `Tools`、技能范围或进程级 MCP scope。
- `provider` 包测试必须用 `-run` 分批执行，禁止整包跑（沙箱 SIGTERM）；新增文件必须 gofmt。
- 修改 `route/manage.go` 后必须运行 `go run ./cmd/apicatalog`，并执行 `-check`。
- 不修改当前工作区里他人未提交的 `controller/common.go`、`provider/archive.go`、`provider/sitemap.go`、`provider/timefactor.go` 及已有未跟踪测试。
- 未取得真实测试应用凭据时，只能报告平台联调阻塞，不得用 mock 宣称平台通过。

## 审查重点（Review Focus）

1. 管理员在会话中途被禁用或降权，已排队任务与待确认卡片仍可执行 → 每个 cap 执行前重读管理员状态，`TestChannelTurnRechecksAdminBeforeEachCap`、`TestApprovalRejectedAfterAdminDisabled`。
2. 通用 `api` 意图被用来访问渠道密钥、绑定、审批或策略接口 → 按最终 method/path 拦截，`TestAPIInvokeBlocksChannelAdminPaths`。
3. 群聊回复或模板覆盖泄露联系方式、地址、留言原文 → 群模板字段白名单与群安全工具限制，`TestGroupTemplateCannotRenderForbiddenFields`、`TestGroupRejectsNonGroupSafeTool`。
4. 绑定 token 并发消费、重复消费或跨站点使用成功 → 事务内条件更新，`TestBindingTokenConcurrentConsume`、`TestBindingTokenRejectsCrossChannel`。
5. 支付并发重复产生事件，或通知 worker 阻塞下单／支付 → 状态迁移赢家唯一，worker 异步，`TestSuccessPaidOrderEmitsEventOnce`、`TestNotificationFailureDoesNotBlockCreateOrder`。

---

### 任务 1：数据模型与迁移注册

**文件：**
- 创建：`model/channel.go`、`model/notification.go`
- 修改：`provider/dao.go:134-226`
- 测试：`provider/channel_model_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestChannelTablesRegistered(t *testing.T)
// 断言 model.MessageChannel、ChannelBindingToken、ChannelBinding、ChannelTarget、
// ChannelInbox、ChannelSession、ChannelApproval、NotificationRule、
// NotificationTemplate、NotificationEvent、NotificationDelivery、ChannelLease、ChannelAudit
// 均在 AutoMigrateDB 的 models 切片中出现（用 go/ast 解析 dao.go，不写死数量）。
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run TestChannelTablesRegistered -count=1`
预期：FAIL，类型未定义或未注册。

- [ ] **步骤 3：实现模型**

`model/channel.go` 定义规格第 4 节的结构体；字段名与规格一致（如 `MessageChannel`、`ChannelBindingToken.TokenHash`、`ChannelBinding.AdminId`、`ChannelTarget.ManagementEnabled`、`ChannelSession.Generation`、`ChannelApproval.ArgsHash`、`ChannelLease.FencingVersion`）。用 `gorm:"column:...;type:...;index:...;uniqueIndex:..."` 注册 `TokenHash`、`ChannelBinding.IdentityHash`、`ChannelTarget.TargetHash`、`ChannelInbox.EventHash`、`ChannelSession.SessionHash`、`ChannelAudit` 普通索引；时间字段沿用仓库整数时间戳习惯。敏感字段标 `json:"-"`。

`model/notification.go` 定义 `NotificationRule`、`NotificationTemplate`、`NotificationEvent`、`NotificationDelivery`，唯一约束分别为事件键、`(EventId, RuleId, TargetId)`。

- [ ] **步骤 4：注册迁移并运行测试**

在 `provider/dao.go` 的 models 切片追加全部新模型。运行：`go test ./provider/ -run TestChannelTablesRegistered -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
gofmt -w model/channel.go model/notification.go provider/channel_model_test.go
git add model/channel.go model/notification.go provider/dao.go provider/channel_model_test.go
git commit -m "feat(渠道): 新增站点消息渠道与通知数据模型"
```

### 任务 2：一次性绑定 token

**文件：**
- 创建：`provider/channelBinding.go`
- 测试：`provider/channelBinding_test.go`

**接口：**
- `type ChannelPlatformIdentity struct { ChannelId uint; OrganizationId, AppId, UserId string; IsGroup bool }`
- `func (w *Website) CreateBindingToken(channelId, adminId uint) (plain string, expiresAt int64, err error)`
- `func (w *Website) ConsumeBindingToken(plain string, ident ChannelPlatformIdentity) (*model.ChannelBinding, error)`
- `func (w *Website) RevokeBinding(bindingId, operatorAdminId uint) error`

- [ ] **步骤 1：编写失败的测试**

```go
func TestBindingTokenExpiresAfter3600Seconds(t *testing.T)
func TestBindingTokenConsumeIsSingleUse(t *testing.T)
func TestBindingTokenConcurrentConsume(t *testing.T)   // 10 goroutines，仅 1 个成功
func TestBindingTokenRejectsCrossChannel(t *testing.T)
func TestBindingTokenRejectsExistingBinding(t *testing.T) // 不覆盖已有绑定
func TestRevokeBindingCancelsPendingWork(t *testing.T)    // 待确认审批与排队 inbox 失效
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestBindingToken|TestRevokeBinding' -count=1`
预期：FAIL，未定义。

- [ ] **步骤 3：实现**

创建时用 `crypto/rand` 生成 32 字节并 `base64.RawURLEncoding`，仅存 SHA-256；生成前把同一管理员同一渠道未消费 token 置失效。消费在 `tx.Transaction` 内以 `WHERE token_hash = ? AND consumed_at = 0 AND expires_at > ?` 条件更新，`RowsAffected != 1` 即失败；校验站点、渠道、平台应用与组织，随后创建 `ChannelBinding` 和私聊 `ChannelTarget`。撤销在事务内标记绑定、撤销其未执行审批与 inbox。

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./provider/ -run 'TestBindingToken|TestRevokeBinding' -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
gofmt -w provider/channelBinding.go provider/channelBinding_test.go
git add provider/channelBinding.go provider/channelBinding_test.go
git commit -m "feat(渠道): 一次性绑定 token 与解绑失效"
```

### 任务 3：平台适配器契约与两个实现

**文件：**
- 创建：`provider/channelPlatform.go`、`provider/channelFeishu.go`、`provider/channelDingTalk.go`
- 测试：`provider/channelPlatform_test.go`

**接口：**

```go
type PlatformAdapter interface {
    Name() string
    Run(ctx context.Context, creds PlatformCredentials, sink InboundSink) error
    Send(ctx context.Context, target TargetRef, msg OutboundMessage) (SendResult, error)
}
type InboundSink func(context.Context, InboundEvent) error
type InboundEvent struct { EventId, Kind, OrganizationId, AppId, UserId, ChatId, ChatType, Text string }
type SendError struct { Class ErrorClass; Message string } // permanent | transient
```

- [ ] **步骤 1：编写失败的测试**

```go
func TestInboundEventRequiresVerifiedIdentity(t *testing.T)
func TestSendErrorClassifiesAuthAsPermanent(t *testing.T)
func TestAdapterRegistryRejectsUnknownPlatform(t *testing.T)
func TestPlatformIdentityHashIsSiteScoped(t *testing.T)
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestInboundEvent|TestSendError|TestAdapterRegistry|TestPlatformIdentityHash' -count=1`
预期：FAIL。

- [ ] **步骤 3：实现适配器**

先按规格第 12 节核对飞书开放平台 Go SDK 与钉钉 Stream SDK 的实际签名、模块版本和回调订阅方式，并把结论写入提交说明；`go get` 锁定版本。`Run` 只负责验签／验身份、标准化事件、调用 sink；`Send` 只负责文本／卡片与长度分段；错误必须归入 permanent 或 transient，禁止返回裸错误让上层猜。未连接真实租户前不实现未经文档确认的接口。

- [ ] **步骤 4：运行测试与编译**

运行：`go test ./provider/ -run 'TestInboundEvent|TestSendError|TestAdapterRegistry|TestPlatformIdentityHash' -count=1 && go build ./...`
预期：PASS、编译通过。

- [ ] **步骤 5：Commit**

```bash
gofmt -w provider/channelPlatform.go provider/channelFeishu.go provider/channelDingTalk.go provider/channelPlatform_test.go
git add provider/channelPlatform.go provider/channelFeishu.go provider/channelDingTalk.go provider/channelPlatform_test.go go.mod go.sum
git commit -m "feat(渠道): 飞书长连接与钉钉 Stream 适配器契约"
```

### 任务 4：连接租约、生命周期与消息去重

**文件：**
- 创建：`provider/channelManager.go`、`provider/channelWorker.go`
- 修改：`bootstrap.go:53-99`、`provider/website.go:335-430`、`provider/anqi.go:1588`
- 测试：`provider/channelManager_test.go`

**接口：**
- `func (w *Website) StartChannelManager(ctx context.Context)`
- `func (w *Website) RebuildChannel(channelId uint) error`
- `func (w *Website) StopChannelManager()`

- [ ] **步骤 1：编写失败的测试**

```go
func TestChannelLeasePreventsSecondOwner(t *testing.T)
func TestLostLeaseCancelsRunningChannel(t *testing.T)
func TestChannelRebuildOnRevisionChange(t *testing.T)
func TestChannelDuplicateEventIsIgnored(t *testing.T) // 相同 EventHash 只入队一次
func TestShutdownWaitsForChannelWorkers(t *testing.T)
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestChannelLease|TestLostLease|TestChannelRebuild|TestChannelDuplicateEvent|TestShutdownWaits' -count=1`
预期：FAIL。

- [ ] **步骤 3：实现**

`StartChannelManager` 只对已启用渠道启动 goroutine；每 10 秒续租，租用失败即取消上下文；配置 revision 变化时先停旧连接再重建。入站事件先写 `ChannelInbox`（唯一键去重）再应答；绑定消息直接消费，不落 token 正文。把启动挂到 `InitWebsite`，关闭挂到 `Shutdown`，进程关闭时等待 worker 退出。

- [ ] **步骤 4：运行测试与竞态检测**

运行：`go test ./provider/ -race -run 'TestChannelLease|TestLostLease|TestChannelRebuild|TestChannelDuplicateEvent|TestShutdownWaits' -count=1`
预期：PASS，无竞态报告。

- [ ] **步骤 5：Commit**

```bash
gofmt -w provider/channelManager.go provider/channelWorker.go provider/channelManager_test.go
git add provider/channelManager.go provider/channelWorker.go provider/channelManager_test.go bootstrap.go provider/website.go provider/anqi.go
git commit -m "feat(渠道): 连接租约、生命周期与入站去重"
```

### 任务 5：能力权限映射与执行主体

**文件：**
- 创建：`provider/channelPermissions.go`、`provider/channelPrincipal.go`
- 测试：`provider/channelPermissions_test.go`

**接口：**
- `type ExecutionPrincipal struct { SiteId uint; AdminId uint; BindingId uint; SessionId string; PolicyRevision int64; InGroup bool }`
- `func (w *Website) CheckIntentPermission(ctx context.Context, p ExecutionPrincipal, intentName, action string) error`
- `func (w *Website) CheckCapPermission(ctx context.Context, p ExecutionPrincipal, capName, argsJson string) error`

- [ ] **步骤 1：编写失败的测试**

```go
func TestEveryCapHasPermissionDecision(t *testing.T)          // 遍历 capHandlers，未映射必须显式 blocked
func TestUnknownIntentIsDenied(t *testing.T)
func TestAdminWithoutMenuPermissionIsDenied(t *testing.T)
func TestSuperAdminStillBoundByCapMapping(t *testing.T)
func TestChannelPolicyDefaultsToDeny(t *testing.T)
func TestAPIInvokeBlocksChannelAdminPaths(t *testing.T)        // /channels 下秘密、绑定、审批、策略接口
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestEveryCapHasPermissionDecision|TestUnknownIntentIsDenied|TestAdminWithoutMenuPermission|TestSuperAdminStillBoundByCapMapping|TestChannelPolicyDefaultsToDeny|TestAPIInvokeBlocksChannelAdminPaths' -count=1`
预期：FAIL。

- [ ] **步骤 3：实现**

在 `channelPermissions.go` 建立 cap → 后台权限路径的显式映射表，并列出首版明确 blocked 的 cap（fs、shell、权限与凭据操作）。`CheckIntentPermission` 依次校验站点曝光门禁、智能体白名单、渠道策略（deny/confirm/allow，缺省 deny）、管理员有效状态与菜单权限；`CheckCapPermission` 供委托调用前复核，`api_invoke` 解析最终 method/path 后再次校验。权限判定不得写回服务级状态，`ExecutionPrincipal` 只经 context／会话局部传递。

- [ ] **步骤 4：运行测试**

运行：`go test ./provider/ -run 'TestEveryCapHasPermissionDecision|TestUnknownIntentIsDenied|TestAdminWithoutMenuPermission|TestSuperAdminStillBoundByCapMapping|TestChannelPolicyDefaultsToDeny|TestAPIInvokeBlocksChannelAdminPaths' -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
gofmt -w provider/channelPermissions.go provider/channelPrincipal.go provider/channelPermissions_test.go
git add provider/channelPermissions.go provider/channelPrincipal.go provider/channelPermissions_test.go
git commit -m "feat(渠道): 能力权限映射与执行主体"
```

### 任务 6：授权聊天执行器

**文件：**
- 创建：`provider/channelChat.go`
- 修改：`provider/aiChat.go`（仅新增对外入口，不改既有主会话路径）
- 测试：`provider/channelChat_test.go`

**接口：**
- `func (svc *AiChatService) RunChannelTurn(ctx context.Context, req ChannelTurnRequest) (ChannelTurnResult, error)`
- `type ChannelTurnRequest struct { Principal ExecutionPrincipal; AgentId uint; Text string; Group bool; Generation int64 }`

- [ ] **步骤 1：编写失败的测试**

```go
func TestChannelTurnRechecksAdminBeforeEachCap(t *testing.T)   // 第 2 个 cap 前禁用管理员 → 拒绝
func TestChannelTurnDoesNotBypassApprovalAsAgent(t *testing.T)
func TestChannelSessionsAreIsolatedByUserAndChat(t *testing.T)
func TestChannelTurnRejectsStaleGeneration(t *testing.T)
func TestGroupRejectsNonGroupSafeTool(t *testing.T)           // 群安全清单仅允许状态与脱敏计数
func TestChannelTurnSerializesSameSession(t *testing.T)
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestChannelTurn|TestChannelSessionsAreIsolated|TestGroupRejectsNonGroupSafeTool' -count=1`
预期：FAIL。

- [ ] **步骤 3：实现**

`RunChannelTurn` 使用请求级 principal 构造 `ToolExecContext`（`IsAgent=false`，`ApprovalFn` 持久化审批回调），在 `ExecuteTool` 前后接权限复核；同一会话用 generation 与互斥串行；群聊仅执行群安全工具。首版只处理文本，不接入语音／图片；留言与网页内容作为不可信数据标记。

- [ ] **步骤 4：运行测试**

运行：`go test ./provider/ -run 'TestChannelTurn|TestChannelSessionsAreIsolated|TestGroupRejectsNonGroupSafeTool' -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
gofmt -w provider/channelChat.go provider/channelChat_test.go provider/aiChat.go
git add provider/channelChat.go provider/channelChat_test.go provider/aiChat.go
git commit -m "feat(渠道): 以实际操作者身份执行聊天管理"
```

### 任务 7：持久化审批

**文件：**
- 创建：`provider/channelApproval.go`
- 测试：`provider/channelApproval_test.go`

**接口：**
- `func (w *Website) CreateApproval(p ExecutionPrincipal, toolName, argsJson string, ttl time.Duration) (*model.ChannelApproval, error)`
- `func (w *Website) DecideApproval(publicId string, bindingId uint, decision string) error`
- `func (w *Website) MarkApprovalExecuting(publicId string) error`
- `func (w *Website) CompleteApproval(publicId, result string) error`

- [ ] **步骤 1：编写失败的测试**

```go
func TestApprovalRejectsOtherBinding(t *testing.T)
func TestApprovalExpiresAfter10Minutes(t *testing.T)
func TestApprovalDoubleClickExecutesOnce(t *testing.T)
func TestApprovalRejectedWhenArgsChange(t *testing.T)
func TestApprovalRejectedAfterAdminDisabled(t *testing.T)
func TestRestartMarksExecutingAsUnknown(t *testing.T)  // 不自动重放
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestApproval|TestRestartMarksExecutingAsUnknown' -count=1`
预期：FAIL。

- [ ] **步骤 3：实现**

公开 ID 用 16 字节随机值；`ArgsHash` 为规范化参数 SHA-256；`DecideApproval` 以条件更新消费，校验绑定、管理员状态、权限、政策 revision 与参数哈希；启动初始化把 `executing` 置 `unknown`。

- [ ] **步骤 4：运行测试**

运行：`go test ./provider/ -run 'TestApproval|TestRestartMarksExecutingAsUnknown' -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
gofmt -w provider/channelApproval.go provider/channelApproval_test.go
git add provider/channelApproval.go provider/channelApproval_test.go
git commit -m "feat(渠道): 持久化一次性操作审批"
```

### 任务 8：通知事件、模板与投递

**文件：**
- 创建：`provider/notifyEvent.go`、`provider/notifyTemplate.go`、`provider/notifyDispatch.go`、`provider/notifyWorker.go`
- 修改：`controller/guestbook.go:147-171`、`provider/guestbook.go:136-235`、`provider/order.go:792-914,1633-1656`
- 测试：`provider/notifyEvent_test.go`、`provider/notifyDispatch_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestGuestbookEventWaitsForClassification(t *testing.T)     // 垃圾留言不投递，正常后投递
func TestNotificationFailureDoesNotBlockCreateOrder(t *testing.T)
func TestSuccessPaidOrderEmitsEventOnce(t *testing.T)            // 并发支付只产生 1 条
func TestAnonymousOrderEmitsCreatedEvent(t *testing.T)           // 不依赖邮箱与 Brevo
func TestGroupTemplateCannotRenderForbiddenFields(t *testing.T)  // contact/address/content 不可见
func TestTemplateRejectsSystemAndIncludeTags(t *testing.T)
func TestDeliveryRetriesTransientNotPermanent(t *testing.T)
func TestDeliveryIsIdempotentPerTarget(t *testing.T)
func TestRevokedTargetCancelsPendingDelivery(t *testing.T)
func TestFailedDeliveryDoesNotEmitAgentFailedRecursively(t *testing.T)
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestGuestbookEvent|TestNotificationFailure|TestSuccessPaidOrderEmits|TestAnonymousOrder|TestGroupTemplate|TestTemplateRejects|TestDelivery|TestRevokedTarget|TestFailedDelivery' -count=1`
预期：FAIL。

- [ ] **步骤 3：实现**

事件写入与业务同事务；订单创建、支付成功在状态迁移赢家处写 `order.created` / `order.paid`，并检查 `RowsAffected`。留言保存写 `waiting_classification`，反垃圾结果正常后才转为待投递。模板渲染使用受限上下文与过滤器白名单，群模板仅允许白名单字段。worker 以数据库租约领取任务、逐目标发送、按错误分类重试。智能体失败事件由任务 6 产生的失败结果写入，不反向通知自身。

- [ ] **步骤 4：运行测试**

运行：`go test ./provider/ -run 'TestGuestbookEvent|TestNotificationFailure|TestSuccessPaidOrderEmits|TestAnonymousOrder|TestGroupTemplate|TestTemplateRejects|TestDelivery|TestRevokedTarget|TestFailedDelivery' -count=1`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
gofmt -w provider/notifyEvent.go provider/notifyTemplate.go provider/notifyDispatch.go provider/notifyWorker.go provider/notifyEvent_test.go provider/notifyDispatch_test.go
git add provider/notifyEvent.go provider/notifyTemplate.go provider/notifyDispatch.go provider/notifyWorker.go provider/notifyEvent_test.go provider/notifyDispatch_test.go controller/guestbook.go provider/guestbook.go provider/order.go
git commit -m "feat(通知): 留言订单事件与逐目标投递"
```

### 任务 9：管理 API 与菜单权限

**文件：**
- 创建：`controller/manageController/channel.go`
- 修改：`route/manage.go`、`config/menu.go`
- 测试：`controller/manageController/channel_test.go`、`provider/channelRoutes_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
func TestChannelRoutesRequireAdminToken(t *testing.T)
func TestChannelRoutesRequireChannelMenuPermission(t *testing.T) // 未知菜单不得默认放行
func TestBindingTokenResponseIsNoStoreAndSinglePlaintext(t *testing.T)
func TestChannelDetailMasksSecrets(t *testing.T)
func TestSaveChannelKeepsMaskedSecret(t *testing.T)             // 哨兵回写沿用真值
func TestChannelMenuRegistered(t *testing.T)                    // go/ast 校验 menu.go 存在 /plugin/channels
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./controller/manageController/ -run 'TestChannel' -count=1 && go test ./provider/ -run 'TestChannelRoutes|TestChannelMenu' -count=1`
预期：FAIL。

- [ ] **步骤 3：实现**

在 `route/manage.go` 新增 `manage.Party("/channels", middleware.ParseAdminToken, middleware.AdminPermission)`，端点集合严格按规格第 10 节；对 `/channels` 增加显式菜单权限检查，未映射即拒绝。所有响应走 `{code,msg,data}`；详情掩码；保存识别掩码哨兵。`config/menu.go` 增加 `/plugin/channels` 菜单，Backend 为 `/channels`。

- [ ] **步骤 4：运行测试与端点清单**

运行：

```bash
go test ./controller/manageController/ -run 'TestChannel' -count=1
go test ./provider/ -run 'TestChannelRoutes|TestChannelMenu' -count=1
go run ./cmd/apicatalog
go run ./cmd/apicatalog -check
```

预期：测试 PASS，catalog 校验通过。

- [ ] **步骤 5：Commit**

```bash
gofmt -w controller/manageController/channel.go controller/manageController/channel_test.go provider/channelRoutes_test.go route/manage.go config/menu.go
git add controller/manageController/channel.go controller/manageController/channel_test.go provider/channelRoutes_test.go route/manage.go config/menu.go api_catalog.json
git commit -m "feat(渠道): 后台渠道管理与菜单权限"
```

### 任务 10：后台页面

**文件：**
- 创建：`anqicms-admin/src/pages/plugin/channels/index.tsx`、`components/{connect,binding,targets,notifications,records}.tsx`、`anqicms-admin/src/services/plugin/channels.ts`
- 修改：`anqicms-admin/config/routes.ts`、`anqicms-admin/src/locales/zh-CN.ts` 等现有语言文件
- 测试：`anqicms-admin/src/pages/plugin/channels/__tests__/`（若仓库无前端测试，改用 `esbuild` 语法校验并说明未做组件测试）

- [ ] **步骤 1：编写失败的测试或校验基线**

```ts
it('绑定 token 只展示一次且过期后不可复制')
it('保存渠道时回传掩码哨兵不会覆盖密钥')
```

- [ ] **步骤 2：运行验证失败**

运行：`cd anqicms-admin && ./node_modules/.bin/esbuild src/pages/plugin/channels/index.tsx --loader:.tsx=tsx --bundle=false --outfile=/dev/null`
预期：文件不存在而失败。

- [ ] **步骤 3：实现**

五个区域对应规格第 10 节；绑定弹窗显示倒计时与“仅显示一次”，不写 localStorage；多 tab 设置按项目约定用 `getFieldsValue(true)` 提交全量字段；错误区分凭据、主动发送权限、断线、管理员失效。同步 `routes.ts` 与后端菜单路径。

- [ ] **步骤 4：运行校验**

运行：`cd anqicms-admin && ./node_modules/.bin/esbuild src/pages/plugin/channels/index.tsx --loader:.tsx=tsx --bundle=false --outfile=/dev/null`
预期：无语法错误；不运行 `npm run format`。

- [ ] **步骤 5：Commit**

```bash
git add anqicms-admin/src/pages/plugin/channels anqicms-admin/src/services/plugin/channels.ts anqicms-admin/config/routes.ts anqicms-admin/src/locales
git commit -m "feat(后台): 消息渠道与通知配置页"
```

### 任务 11：真实平台联调与发布门槛

**文件：**
- 修改：`docs/superpowers/specs/2026-11-05-im-agent-channels-design.md`（仅在获得实测事实后补充平台核验结果）
- 测试：`provider/channel_live_test.go`（非默认运行，需环境变量显式开启）

- [ ] **步骤 1：编写联调检查清单与跳过测试**

`TestChannelLiveFeishu`、`TestChannelLiveDingTalk` 在缺少凭据时 `t.Skip`，并打印缺失项；覆盖接收文本、私聊绑定、主动群／私聊发送、群 @隔离、确认卡片、断线重连、长文本、撤权与错误凭据。

- [ ] **步骤 2：运行离线回归**

```bash
go build ./...
go vet ./...
go test ./provider/ -run 'TestChannel|TestBinding|TestApproval|TestNotification|TestDelivery' -count=1
go test ./controller/manageController/ -count=1
go test ./pkg/mcp/intent/ -count=1
go run ./cmd/apicatalog -check
```

预期：全部通过；未取得凭据时如实报告，不得勾选平台验收。

- [ ] **步骤 3：执行真实联调并记录事实**

用两个测试租户分别配置企业应用，记录 SDK 版本、权限名称、组织与用户标识来源、主动发送权限、卡片回调可用性及实测时间。只有全部通过才把规格第 12 节更新为实测结论。

- [ ] **步骤 4：Commit**

```bash
gofmt -w provider/channel_live_test.go
git add provider/channel_live_test.go docs/superpowers/specs/2026-11-05-im-agent-channels-design.md
git commit -m "test(渠道): 飞书钉钉联调清单与平台核验记录"
```

## 修订：任务 12（收尾）拆为 T12a / T12b

原计划写到任务 11 结束，实际收尾工作（终审提出的接线与诚实性问题）在实现阶段拆成两批，口径如下。这两批不引入新端点，`route/manage.go` 不变。

### 任务 12a：接线、写门禁与代次

- 入站任务消费：`ChannelInboxWorker` 真正消费 inbox 行（此前只落库不执行）。
- 审批执行：审批行按条件更新推进到 executing/succeeded/failed/unknown，执行前重新校验参数与权限。
- 渠道写门禁：写操作在 backend 侧统一校验（不只依赖前端按钮）。
- Generation 代次：会话切换智能体/新建会话使旧待执行工作失效。
- inbox 状态与 `LastError`：失败必须留下可观测原因。

### 任务 12b：终审 6 条发现中属实的 5 条 + 1 条否证

- **试发如实**：`ChannelTestSend(targetId=0)` 只检查必填项、没有发起任何平台请求，回执改为 `valid=false, verified=false` 并附「未连接平台验证」文案；只有真的发出消息（成功或拿到平台分类失败）才 `verified=true`。前端据 `verified` 显示「未验证」，不再提示「配置校验通过」。
- **agent.failed 只从绑定会话派生**：`deriveAgentFailedNotificationEvents` 加 `binding_id > 0`。管理员试发/后台操作失败的审计（`binding_id=0`）没有绑定身份，不属于智能体任务失败，不进群。
- **生命周期**：`RemoveWebsite` 与进程退出（抽出的 `shutdownWebsite`）都必须先停通知工作者再关库；顺序为停渠道管理器 → 停通知工作者 → 关库 → 关日志。
- **通知表缺失兜底**：升级窗口（旧库 + 新二进制、AutoMigrate 未跑）里 `notification_events` / `notification_deliveries` 缺失时，`emitNotificationEvent` / `fanOutNotificationEvent` 记一次日志并返回 nil，**不得回滚业务事务**；表存在时的数据库错误仍原样返回。
- **路由回归锁**：终审「`/channels/` 与 `/channels` 不匹配」是误报 —— Iris `cleanPath` 会去掉尾斜杠，`Party("/channels").Get("/")` 实际注册为 `/system/api/channels`（无斜杠 200，带斜杠 301）。不改路由，只补真实路由表断言（`GET /system/api/channels`、`GET /system/api/channels/records`、`POST /system/api/channels/test`）。

**审批确认口径修订**（同步到规格 §8）：首版用私聊文本指令 `批准 <公开ID>` / `拒绝 <公开ID>`，不发确认卡片 —— 飞书 ws SDK v3.12.2 无法接收卡片回调（`WithCardHandler` 被注释），钉钉 Stream 侧也只有归一化没有发送能力。群聊里不提供审批指令入口；卡片交互留到后续版本。

## 规格覆盖对照

| 规格章节 | 对应任务 |
|---|---|
| 4 数据契约 | 1 |
| 5 生命周期与租约 | 4 |
| 6 身份绑定 | 2 |
| 7 权限与聊天执行 | 5、6 |
| 8 审批 | 7 |
| 9 通知、模板与投递 | 8 |
| 10 管理 API 与前端 | 9、10 |
| 3 架构边界 | 3、4、6、8 |
| 11 验收与发布 | 全部任务，集中在 11 |
| 12 平台核验 | 3、11 |

## 已知风险与前置条件

- 智能体当前无独立模型／技能字段；首版沿用站点模型与所选 AiAgent 的 Strategy、MaxRounds，接待能力白名单落到新表，不声称已有字段可用。
- 飞书、钉钉 SDK 确切 API 未实测；任务 3 以官方文档与真实编译为准，禁止凭记忆写接口。
- 现有 `AdminPermission` 对未知菜单默认放行；`/channels` 必须在 handler 层补显式拒绝，测试锁死。
- 迁移受版本变化或 force 控制；同版本开发环境需在后台执行“修复数据库”，渠道缺表时保持停用。
- 通知与业务事务的一致性、支付并发只产生一次事件，是本计划最高风险项，任务 8 必须做反向对照测试。
