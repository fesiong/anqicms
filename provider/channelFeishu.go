package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

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
}

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

	for attempt := 0; ; attempt++ {
		err := a.runOnce(ctx, creds, sink)
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
		if !waitBeforeReconnect(ctx, attempt, model.ChannelPlatformFeishu) {
			return nil
		}
	}
}

// runOnce 是一次完整的连接生命周期：建连 → 收事件 → 断开/取消后返回。
func (a *feishuAdapter) runOnce(ctx context.Context, creds PlatformCredentials, sink InboundSink) error {
	// 长连接模式下 SDK 的 Do 路径不做签名校验（signature 只用于 HTTP webhook），
	// 因此 verificationToken / encryptKey 都留空。
	dispatcher := larkdispatcher.NewEventDispatcher("", "")
	dispatcher.OnP2MessageReceiveV1(func(handlerCtx context.Context, event *larkim.P2MessageReceiveV1) error {
		// 回调跑在 SDK 的 goroutine 上：panic 会打穿到 SDK，必须自己兜住。
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("[channel] 飞书事件处理发生 panic，已忽略本次事件")
			}
		}()
		a.handleInboundEvent(handlerCtx, creds, event, sink)
		return nil
	})

	client := larkws.NewClient(creds.AppId, creds.AppSecret,
		larkws.WithEventHandler(dispatcher),
		// 只留 error 级：debug 级会打印事件体，正文与凭据都没有进日志的必要。
		larkws.WithLogLevel(larkcore.LogLevelError),
		larkws.WithAutoReconnect(false),
	)
	return client.Start(ctx)
}

// handleInboundEvent 做「标准化 → 验身份 → 调 sink」。任何失败只写日志，不回抛给 SDK：
// 回调返回错误会让 SDK 给平台回 500，而本层不掌握去重与重投语义（T4 的职责）。
func (a *feishuAdapter) handleInboundEvent(ctx context.Context, creds PlatformCredentials, event *larkim.P2MessageReceiveV1, sink InboundSink) {
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
	if err := inbound.Validate(creds.AppId, creds.OrganizationId); err != nil {
		log.Printf("[channel] 飞书入站事件未通过身份校验，已丢弃：%v", err)
		return
	}
	if err := sink(ctx, inbound); err != nil {
		// 只记事件 ID 与错误，不记正文。
		log.Printf("[channel] 飞书入站事件处理失败 eventId=%s：%s", inbound.EventId, sanitizePlatformMessage(err.Error()))
	}
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
func (a *feishuAdapter) Send(ctx context.Context, target TargetRef, msg OutboundMessage) (SendResult, error) {
	creds := normalizePlatformCredentials(a.creds)
	if creds.AppId == "" || creds.AppSecret == "" {
		// 原型适配器（注册表返回的、未绑定凭据的实例）也会走到这里。
		return SendResult{}, permanentSendError("飞书适配器未绑定应用凭据，无法发送")
	}
	if err := validateTargetRef(target); err != nil {
		return SendResult{}, err
	}
	if strings.TrimSpace(msg.Text) == "" {
		return SendResult{}, permanentSendError("消息内容为空，未发送")
	}

	// 飞书要求 content 是「JSON 序列化后的字符串」，文本消息形如 {"text":"..."}。
	content, err := json.Marshal(map[string]string{"text": msg.Text})
	if err != nil {
		// 只有字符串到 JSON 的编码，理论上不会失败；仍按永久错误处理，不重试。
		return SendResult{}, permanentSendError("序列化飞书文本消息失败：%v", err)
	}

	receiveIdType := "open_id"
	if target.Kind == TargetKindGroup {
		receiveIdType = "chat_id"
	}
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIdType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(strings.TrimSpace(target.PlatformTargetId)).
			MsgType("text").
			Content(string(content)).
			Build()).
		Build()

	resp, callErr := feishuClientFor(creds).Im.Message.Create(ctx, req)
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
		return SendResult{}, nil
	}
	return SendResult{PlatformMessageId: *resp.Data.MessageId}, nil
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
