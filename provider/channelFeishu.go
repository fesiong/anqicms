package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"kandaoni.com/anqicms/model"
)

// 飞书适配器。SDK 与版本：github.com/larksuite/oapi-sdk-go/v3 v3.12.2。
//
// 长连接部分用 ws 包（larkws.NewClient(appId, appSecret, ...)，Start(ctx) 阻塞），
// 事件用 event/dispatcher 注册（OnP2MessageReceiveV1 对应 im.message.receive_v1），
// 发送用 service/im/v1（client.Im.Message.Create）。
//
// 已知平台/SDK 限制（v3.12.2 源码核对）：
//   - ws.Client 只有 WithEventHandler，WithCardHandler 在源码里是注释掉的，且
//     handleDataFrame 只处理 MessageTypeEvent（ws/client_message.go:79）。也就是说
//     卡片回调 frame（type=card）会被直接丢弃，本任务无法通过长连接接收飞书卡片回调。
//   - ws.Client 自带自动重连（默认 2 分钟固定间隔 + 首次 30 秒随机抖动），与规格要求的
//     「1 秒起、60 秒封顶、带抖动」不一致，因此这里关掉 SDK 重连，由本适配器统一控制。
type feishuAdapter struct {
	creds PlatformCredentials

	// createMessage 可注入：默认走 feishuClientFor(creds).Im.Message.Create。
	// 留出这个钩子是为了在没有真实租户的测试里断言「群聊用 chat_id、私聊用 open_id」。
	createMessage feishuCreateMessageFunc
	// fetchBotOpenId 可注入：默认走 bot/v3/info。测试里替换以免发真实请求。
	fetchBotOpenId func(ctx context.Context, creds PlatformCredentials) string
}

// feishuMaxTextRunes 是单条文本消息的分段上限（rune 数）。
// ⚠ 保守值：飞书文本消息的实际限制按请求体 150KB 计（im/v1/message/create 文档），
// 这里取 3500 是为了同时容纳多字节中文与 JSON 转义后的体积；未用真实租户校准，T11 校准。
const feishuMaxTextRunes = 3500

// feishuBotIdentityTimeout 是建连前同步拉取机器人身份的时限。
// lark.Client 默认 ReqTimeout=0（等价 http.DefaultClient，没有响应超时），
// 建连前的同步请求一旦撞上半开连接就会把整条重连循环永久卡住，因此必须自带超时。
const feishuBotIdentityTimeout = 5 * time.Second

// feishuSendRequest 是一次文本发送的参数。
//
// 不用 *larkim.CreateMessageReq 是因为 SDK 把 receive_id_type 藏在未导出的 apiReq 里，
// 包外既读不到也断言不了；抽成结构体后测试可以直接验证接收者 ID 类型。
type feishuSendRequest struct {
	ReceiveIdType string
	ReceiveId     string
	Text          string
}

type feishuCreateMessageFunc func(ctx context.Context, req feishuSendRequest) (*larkim.CreateMessageResp, error)

func (a *feishuAdapter) Name() string { return model.ChannelPlatformFeishu }

// Run 建立飞书长连接并把 im.message.receive_v1 事件标准化后交给 sink。
// ctx 取消时返回 nil（正常停止）；鉴权类错误直接返回错误，不做无限快速重连。
func (a *feishuAdapter) Run(ctx context.Context, creds PlatformCredentials, sink InboundSink) error {
	creds = normalizePlatformCredentials(creds)
	if err := validatePlatformCredentials(creds); err != nil {
		return err
	}
	if sink == nil {
		return errors.New("入站事件处理函数不能为空")
	}

	// attempt 用原子量：onConnected 由 SDK 的回调 goroutine 调用，runOnce 返回后在
	// 本 goroutine 读它做退避。连接一旦确认建立就复位，避免抖动几次后每次重连都等 30~60 秒。
	var attempt atomic.Int32
	for {
		err := a.runOnce(ctx, creds, sink, func() { attempt.Store(0) })
		if ctx.Err() != nil {
			// 正常停止：不要把自己的取消当成连接失败，否则 T4 会看到一条无意义的错误。
			return nil
		}
		if err == nil {
			return nil
		}
		if fatal, ok := feishuConnectFatalError(err); ok {
			// 鉴权/连接数超限重试也不会好：直接返回错误，由 T4 把渠道置为错误状态。
			return fmt.Errorf("飞书长连接无法建立（%s）：%s", fatal, sanitizePlatformMessage(err.Error()))
		}
		// 日志只记状态与文案：SDK 报错里不含 secret，但仍统一过一遍 sanitize。
		log.Printf("[channel] 飞书长连接中断：%s", sanitizePlatformMessage(err.Error()))
		if !waitBeforeReconnect(ctx, int(attempt.Load()), model.ChannelPlatformFeishu) {
			return nil
		}
		attempt.Add(1)
	}
}

// runOnce 是一次完整的连接生命周期：建连 → 收事件 → 断开/取消后返回。
// onConnected 在「连接确认建立」时被调用（SDK 的 OnReady / OnReconnected 回调）。
func (a *feishuAdapter) runOnce(ctx context.Context, creds PlatformCredentials, sink InboundSink, onConnected func()) error {
	// 回调是 SDK 用 context.Background() 派生的，与 Run 的 ctx 没有派生关系：
	// 这里再包一层，保证 Run 取消时 sink 里挂着的活也能一起停。
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 机器人 open_id 用来判断群聊消息是否 @ 了本应用（规格 §7 群聊门禁）。
	// 每轮建连取一次；取不到（含超时）就按「未 @」处理（fail closed），不因此中断连接。
	botOpenId := fetchFeishuBotIdentity(runCtx, a.fetchBotOpenId, creds, feishuBotIdentityTimeout)

	// 长连接模式下 SDK 的 Do 路径不做签名校验（signature 只用于 HTTP webhook），
	// 因此 verificationToken / encryptKey 都留空。
	dispatcher := larkdispatcher.NewEventDispatcher("", "")
	dispatcher.OnP2MessageReceiveV1(func(_ context.Context, event *larkim.P2MessageReceiveV1) error {
		// 回调跑在 SDK 的 goroutine 上：panic 会打穿到 SDK，必须自己兜住。
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("[channel] 飞书事件处理发生 panic，已忽略本次事件")
			}
		}()
		a.handleInboundEvent(runCtx, creds, event, botOpenId, sink)
		return nil
	})

	client := larkws.NewClient(creds.AppId, creds.AppSecret,
		larkws.WithEventHandler(dispatcher),
		// 只留 error 级：debug 级会打印事件体，正文与凭据都没有进日志的必要。
		larkws.WithLogLevel(larkcore.LogLevelError),
		larkws.WithAutoReconnect(false),
		// OnReady/OnReconnected 由 activateConnection 在连接被采纳之后触发
		// （ws/client_session.go:56-72），是「连接确实建立了」的直接证据。
		larkws.WithOnReady(onConnected),
		larkws.WithOnReconnected(onConnected),
	)
	return client.Start(ctx)
}

// fetchFeishuBotIdentity 在限定时间内取机器人 open_id。
//
// 建连前是同步调用：lark 客户端默认没有响应超时，一条半开连接会让整条重连循环永久卡住
// 且没有任何日志。这里强制套一层超时（只作用于本次请求，不动 SDK 全局配置），
// 超时按「拉取失败」处理：记一行日志并返回空串——调用方据此 fail closed
// （MentionedBot 恒 false），但不中断长连接。
//
// fetch 为 nil 时走生产实现 feishuBotOpenId（该参数同时是测试注入点）。
func fetchFeishuBotIdentity(ctx context.Context, fetch func(context.Context, PlatformCredentials) string, creds PlatformCredentials, timeout time.Duration) string {
	if fetch == nil {
		fetch = feishuBotOpenId
	}
	// 超时靠 ctx 传导：SDK 的请求用同一个 ctx，超时后连接被主动断掉，不会泄漏 goroutine。
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	openId := fetch(fetchCtx, creds)
	if errors.Is(fetchCtx.Err(), context.DeadlineExceeded) {
		// 超时只记这一条：底层失败分支在 ctx 已过期时不再重复写日志。
		log.Printf("[channel] 飞书获取机器人信息超过 %s 未返回，群聊 @ 门禁按未 @ 处理", timeout)
		return ""
	}
	return strings.TrimSpace(openId)
}

// feishuBotOpenId 通过 bot/v3/info 取本应用机器人的 open_id。
//
// 该接口在 SDK 里没有生成（v3.12.2 只有 channel 包内部用 raw Get 调它），
// 这里沿用同样的调用方式：lark.Client.Get + tenant_access_token。
// 失败只记一行日志并返回空串：调用方按 fail closed 处理，不影响长连接本身。
func feishuBotOpenId(ctx context.Context, creds PlatformCredentials) string {
	resp, err := feishuClientFor(creds).Get(ctx, "/open-apis/bot/v3/info", nil, larkcore.AccessTokenTypeTenant)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			// 超时由 fetchFeishuBotIdentity 统一记日志，这里不重复写第二行。
			return ""
		}
		log.Printf("[channel] 飞书获取机器人信息失败，群聊 @ 门禁按未 @ 处理：%s", sanitizePlatformMessage(err.Error()))
		return ""
	}
	if resp == nil || resp.StatusCode != 200 {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		log.Printf("[channel] 飞书获取机器人信息失败（HTTP %d），群聊 @ 门禁按未 @ 处理", status)
		return ""
	}
	var payload struct {
		Code int `json:"code"`
		Bot  struct {
			OpenId string `json:"open_id"`
		} `json:"bot"`
	}
	if err := json.Unmarshal(resp.RawBody, &payload); err != nil {
		log.Printf("[channel] 飞书机器人信息解析失败，群聊 @ 门禁按未 @ 处理")
		return ""
	}
	if payload.Code != 0 {
		log.Printf("[channel] 飞书机器人信息返回业务错误 code=%d，群聊 @ 门禁按未 @ 处理", payload.Code)
		return ""
	}
	return strings.TrimSpace(payload.Bot.OpenId)
}

// handleInboundEvent 做「标准化 → 验身份 → 调 sink」。任何失败只写日志，不回抛给 SDK：
// 回调返回错误会让 SDK 给平台回 500，而本层不掌握去重与重投语义（T4 的职责）。
func (a *feishuAdapter) handleInboundEvent(ctx context.Context, creds PlatformCredentials, event *larkim.P2MessageReceiveV1, botOpenId string, sink InboundSink) {
	// 事件头里的 app_id 是平台写入的，与凭据比对能识别「配置串了应用」；
	// 标准化结果里的 AppId 仍取凭据（T3 契约），这里是额外的显式校验。
	if headerAppId := feishuEventHeaderAppId(event); headerAppId != "" && headerAppId != creds.AppId {
		log.Printf("[channel] 飞书事件的 app_id 与渠道配置不一致，已丢弃")
		return
	}
	inbound, ok := normalizeFeishuInbound(event, creds.AppId)
	if !ok {
		log.Printf("[channel] 飞书事件缺少必要字段，已丢弃")
		return
	}
	if senderType := feishuEventSenderType(event); senderType == "bot" {
		// 机器人自己的消息（含回显）不进模型：否则会自激。
		return
	}
	inbound.MentionedBot = feishuMentionedBot(event.Event.Message, inbound.ChatType, botOpenId)
	if err := inbound.Validate(creds.AppId, creds.OrganizationId); err != nil {
		log.Printf("[channel] 飞书入站事件未通过身份校验，已丢弃：%v", err)
		return
	}
	if err := sink(ctx, inbound); err != nil {
		// 只记事件 ID 与错误，不记正文。
		log.Printf("[channel] 飞书入站事件处理失败 eventId=%s：%s", inbound.EventId, sanitizePlatformMessage(err.Error()))
	}
}

// feishuMentionedBot 判断群聊消息是否明确 @ 了本应用机器人。
//
// 私聊恒为 false（规格 §7 的 @ 门禁只针对群聊）；botOpenId 为空时也 false（fail closed，
// 取不到机器人身份就不放行需要 @ 的群聊消息）。
func feishuMentionedBot(message *larkim.EventMessage, chatType, botOpenId string) bool {
	if message == nil || chatType != ChatTypeGroup {
		return false
	}
	botOpenId = strings.TrimSpace(botOpenId)
	if botOpenId == "" {
		return false
	}
	for _, mention := range message.Mentions {
		if mention == nil || mention.Id == nil {
			continue
		}
		if strings.TrimSpace(feishuStringValue(mention.Id.OpenId)) == botOpenId {
			return true
		}
	}
	return false
}

// normalizeFeishuInbound 把 im.message.receive_v1 事件标准化为 InboundEvent。
// 抽成纯函数是为了能离线断言映射结果（不建连接）。
func normalizeFeishuInbound(event *larkim.P2MessageReceiveV1, appId string) (InboundEvent, bool) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return InboundEvent{}, false
	}
	message := event.Event.Message

	inbound := InboundEvent{
		EventId:  feishuStringValue(message.MessageId),
		Kind:     feishuStringValue(message.MessageType),
		AppId:    appId,
		ChatId:   feishuStringValue(message.ChatId),
		ChatType: feishuStringValue(message.ChatType),
	}
	if sender := event.Event.Sender; sender != nil {
		inbound.OrganizationId = feishuStringValue(sender.TenantKey)
		if sender.SenderId != nil {
			// 用 open_id：它是应用维度的稳定用户标识，与绑定侧（ChannelPlatformIdentity.UserId）同口径。
			inbound.UserId = feishuStringValue(sender.SenderId.OpenId)
		}
	}
	if inbound.EventId == "" {
		// 消息 ID 缺失（极少见）时退回事件 ID：去重仍能工作，只是粒度变成「事件」。
		inbound.EventId = feishuEventHeaderEventId(event)
	}
	if inbound.OrganizationId == "" {
		inbound.OrganizationId = event.TenantKey()
	}
	if inbound.Kind == InboundKindText {
		inbound.Text = feishuTextContent(message.Content)
	}
	return inbound, true
}

// feishuTextContent 解出 text 消息的正文。内容不是合法 JSON 或没有 text 字段时返回空串：
// 上层按「非文本」处理，不猜原始字符串。
func feishuTextContent(content *string) string {
	raw := feishuStringValue(content)
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return ""
	}
	return payload.Text
}

func feishuStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func feishuEventHeaderEventId(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.EventV2Base == nil || event.EventV2Base.Header == nil {
		return ""
	}
	return event.EventV2Base.Header.EventID
}

func feishuEventHeaderAppId(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.EventV2Base == nil || event.EventV2Base.Header == nil {
		return ""
	}
	return event.EventV2Base.Header.AppID
}

func feishuEventSenderType(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.Event == nil || event.Event.Sender == nil {
		return ""
	}
	return feishuStringValue(event.Event.Sender.SenderType)
}

// Send 发送一条文本消息。群聊用 chat_id，私聊用 open_id（规格 §12 的用户/群标识口径）。
//
// 正文超过 feishuMaxTextRunes 时按 rune 分段逐条发送：超长正文会被平台以 230020 拒绝，
// 而 230020 属于 permanent——不分段等于让长通知永久丢失。任一段失败立即返回该段的分类
// 错误（已发出的段不回收：平台消息无法原子回滚，重发会造成重复）。
func (a *feishuAdapter) Send(ctx context.Context, target TargetRef, msg OutboundMessage) (SendResult, error) {
	if err := validateTargetRef(target); err != nil {
		return SendResult{}, err
	}
	if strings.TrimSpace(msg.Text) == "" {
		if msg.Card != nil {
			// 卡片发送不在本轮范围（控制者裁决）：明确拒绝，不写「假装支持」的分支。
			return SendResult{}, permanentSendError("卡片消息暂不支持，请改用文本")
		}
		return SendResult{}, permanentSendError("消息内容为空，未发送")
	}
	// msg.Card 非 nil 但有正文时同样被忽略：本轮只发文本。

	// 群聊的目标标识是 chat_id，私聊是 open_id；传错类型平台会直接报参数错误。
	receiveIdType := "open_id"
	if target.Kind == TargetKindGroup {
		receiveIdType = "chat_id"
	}
	receiveId := strings.TrimSpace(target.PlatformTargetId)

	createMessage := a.createMessage
	if createMessage == nil {
		createMessage = defaultFeishuCreateMessage(normalizePlatformCredentials(a.creds))
	}

	result := SendResult{}
	for _, segment := range splitPlatformText(msg.Text, feishuMaxTextRunes) {
		resp, callErr := createMessage(ctx, feishuSendRequest{
			ReceiveIdType: receiveIdType,
			ReceiveId:     receiveId,
			Text:          segment,
		})
		if callErr != nil {
			return SendResult{}, ClassifyPlatformSendError(callErr)
		}
		if resp == nil {
			return SendResult{}, transientSendError("飞书未返回响应")
		}
		if !resp.Success() {
			status := 0
			if resp.ApiResp != nil {
				status = resp.ApiResp.StatusCode
			}
			return SendResult{}, ClassifyPlatformSendError(
				newPlatformResponseError(model.ChannelPlatformFeishu, status, resp.Code, resp.Msg),
			)
		}
		if resp.Data == nil || resp.Data.MessageId == nil {
			// 消息已发出但平台没给 ID：不重发（重发会产生重复消息），只记日志。
			log.Printf("[channel] 飞书发送成功但未返回 message_id，目标类型=%s", target.Kind)
			continue
		}
		// 多段时以最后一段的 ID 作为回执：调用方关心的是「这条通知的最终一条消息」。
		result.PlatformMessageId = *resp.Data.MessageId
	}
	return result, nil
}

// defaultFeishuCreateMessage 是生产路径：把 feishuSendRequest 组装成 SDK 请求并发出。
func defaultFeishuCreateMessage(creds PlatformCredentials) feishuCreateMessageFunc {
	return func(ctx context.Context, req feishuSendRequest) (*larkim.CreateMessageResp, error) {
		// 飞书要求 content 是「JSON 序列化后的字符串」，文本消息形如 {"text":"..."}。
		content, err := json.Marshal(map[string]string{"text": req.Text})
		if err != nil {
			// 只有字符串到 JSON 的编码，理论上不会失败；仍按永久错误处理，不重试。
			return nil, permanentSendError("序列化飞书文本消息失败：%v", err)
		}
		sdkReq := larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(req.ReceiveIdType).
			Body(larkim.NewCreateMessageReqBodyBuilder().
				ReceiveId(req.ReceiveId).
				MsgType("text").
				Content(string(content)).
				Build()).
			Build()
		return feishuClientFor(creds).Im.Message.Create(ctx, sdkReq)
	}
}

// feishuClients 复用 lark.Client：它内部带 tenant_access_token 缓存，
// 每次 Send 新建客户端会让每条消息都重新取一次 token。
var feishuClients = struct {
	mu      sync.Mutex
	byAppId map[string]*lark.Client
}{byAppId: make(map[string]*lark.Client)}

// feishuClientFor 返回绑定该凭据的客户端。缓存键包含密钥摘要：密钥轮换后自动换新客户端。
func feishuClientFor(creds PlatformCredentials) *lark.Client {
	key := creds.AppId + "\x1f" + channelDigest(creds.AppSecret)

	feishuClients.mu.Lock()
	defer feishuClients.mu.Unlock()
	if client, ok := feishuClients.byAppId[key]; ok {
		return client
	}
	client := lark.NewClient(creds.AppId, creds.AppSecret, lark.WithLogLevel(larkcore.LogLevelError))
	feishuClients.byAppId[key] = client
	return client
}

// feishuConnectFatalError 判断建连错误是否属于「重试也不会好」。
//
// 依据 ws 包源码（v3.12.2）：
//   - fetchEndpoint 对 HTTP 非 200 返回 *ws.ServerError（Code 就是 HTTP 状态码）；
//   - 响应体 code 非 0 时返回 *ws.ClientError（403 Forbidden / 514 AuthFailed /
//     1000040350 ExceedConnLimit / 1000040343 InternalError）。
//
// 鉴权失败、权限不足、连接数超限、以及 HTTP 401/403 都算致命；其余（网络、5xx、
// InternalError）按可重试处理，交给退避循环。
func feishuConnectFatalError(err error) (string, bool) {
	var serverErr *larkws.ServerError
	if errors.As(err, &serverErr) {
		switch {
		case serverErr.Code == 401 || serverErr.Code == 403:
			return "鉴权失败", true
		case serverErr.Code == 429:
			return "", false
		case serverErr.Code >= 500:
			return "", false
		default:
			return "服务端拒绝", true
		}
	}
	var clientErr *larkws.ClientError
	if errors.As(err, &clientErr) {
		switch clientErr.Code {
		case larkws.AuthFailed, larkws.Forbidden, larkws.ExceedConnLimit:
			return "鉴权失败或连接数超限", true
		case larkws.InternalError, larkws.SystemBusy:
			return "", false
		default:
			return "平台拒绝了连接请求", true
		}
	}
	return "", false
}

// feishuAuthBizCodes 是飞书鉴权/令牌族的业务码。这些错误常常以 HTTP 200 + 业务码返回，
// 必须按永久错误处理，否则会一直重试。
// ⚠ 未在真实租户逐条验证：来自开放平台错误码文档的常见取值，未知码仍走 transient。
var feishuAuthBizCodes = map[int]struct{}{
	99991661: {}, // app_id / app_secret 无效
	99991663: {}, // tenant_access_token 无效
	99991664: {}, // tenant_access_token 缺失
	99991665: {}, // tenant key 无效
	99991668: {}, // app_ticket 无效
	99991672: {}, // 应用未开通所需权限
}

// feishuPermissionBizCodes 是发送侧「重试也不会好」的业务码（目标/权限/参数问题）。
// ⚠ 同上，未经真实租户验证；落在表外的业务码按 transient 处理。
var feishuPermissionBizCodes = map[int]struct{}{
	230001: {}, // 请求参数错误
	230002: {}, // 机器人不在目标会话内
	230006: {}, // 机器人能力未开启
	230013: {}, // 目标用户不在机器人可用范围内
	230020: {}, // 消息内容超长
	230027: {}, // 无发送权限
}

// classifyPlatformBizCode 把平台业务码翻译成错误分类。只有飞书有可判定的业务码；
// 钉钉的失败信息走 HTTP 状态码，不在本表内。
func classifyPlatformBizCode(platform string, code int) (ErrorClass, bool) {
	if platform != model.ChannelPlatformFeishu {
		return "", false
	}
	if _, ok := feishuAuthBizCodes[code]; ok {
		return ErrorClassPermanent, true
	}
	if _, ok := feishuPermissionBizCodes[code]; ok {
		return ErrorClassPermanent, true
	}
	return "", false
}

func init() {
	// 让分类器认识 SDK 自己的错误类型：CodeError 是值类型（嵌入生成的响应结构体），
	// 既可能以值也可能以指针出现在 error 链里，两种都要认。
	registerPlatformErrorClassifier(func(err error) *SendError {
		var codeErr larkcore.CodeError
		if errors.As(err, &codeErr) {
			return ClassifyPlatformSendError(
				newPlatformResponseError(model.ChannelPlatformFeishu, 0, codeErr.Code, codeErr.Msg),
			)
		}
		var codeErrPtr *larkcore.CodeError
		if errors.As(err, &codeErrPtr) && codeErrPtr != nil {
			return ClassifyPlatformSendError(
				newPlatformResponseError(model.ChannelPlatformFeishu, 0, codeErrPtr.Code, codeErrPtr.Msg),
			)
		}
		return nil
	})
}
