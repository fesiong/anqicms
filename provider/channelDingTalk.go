package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dcard "github.com/open-dingtalk/dingtalk-stream-sdk-go/card"
	dchatbot "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	dclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	dpayload "github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	dutils "github.com/open-dingtalk/dingtalk-stream-sdk-go/utils"

	"kandaoni.com/anqicms/model"
)

// 钉钉适配器。SDK 与版本：github.com/open-dingtalk/dingtalk-stream-sdk-go v0.9.0。
//
// 长连接用 client.NewStreamClient(client.WithAppCredential(client.NewAppCredentialConfig(appId, appSecret)), ...)，
// 机器人消息回调用 stream.RegisterChatBotCallbackRouter，卡片回调用
// stream.RegisterCardCallbackRouter；Start(ctx) 建连成功后立即返回，收消息在 SDK 自己的 goroutine 里。
//
// 已知限制（v0.9.0 源码核对）：
//   - Start(ctx) 只在「取连接地址」时用到 ctx，之后完全脱离 ctx；SDK 自带的
//     autoReconnect 用固定 3 秒间隔且不可取消（client.go:225 reconnect() 用
//     context.Background()）。因此这里关掉 SDK 重连，由本适配器统一控制退避与退出。
//   - SDK 不暴露「连接是否存活」的查询接口，读失败只写 SDK 日志。因此本适配器
//     用「最近一次收到帧的时间」做兜底探活：超过阈值没有任何帧就主动重连。
//     阈值给了较大余量，避免把服务端正常的低频 ping 误判成掉线（未在真实租户校准）。
type dingtalkAdapter struct {
	creds PlatformCredentials
	// openAPIHost / httpClient 只用于发送（主动发消息不走 Stream）。留空用默认值。
	openAPIHost string
	httpClient  *http.Client
}

const (
	// dingTalkOpenAPIHost 是钉钉新版服务端 API 域名（主动发消息与取 access token）。
	dingTalkOpenAPIHost = "https://api.dingtalk.com"
	// dingTalkTokenEndpoint 取企业内部应用 accessToken。
	dingTalkTokenEndpoint = "/v1.0/oauth2/accessToken"
	// dingTalkGroupSendEndpoint 机器人发送群聊消息。
	dingTalkGroupSendEndpoint = "/v1.0/robot/groupMessages/send"
	// dingTalkPrivateSendEndpoint 机器人批量发送单聊消息。
	dingTalkPrivateSendEndpoint = "/v1.0/robot/oToMessages/batchSend"
	// dingTalkTextMsgKey 是文本消息模板键，msgParam 传 {"content":"..."}。
	dingTalkTextMsgKey = "sampleText"
	// dingTalkTokenRefreshSkew 提前这么多时间视为过期，避免边界上用到刚过期的 token。
	dingTalkTokenRefreshSkew = 60 * time.Second
	// dingTalkTokenFallbackTTL 平台没给 expireIn 时的保守有效期。
	dingTalkTokenFallbackTTL = 7200 * time.Second
	// dingTalkLivenessCheckInterval 探活检查间隔。
	dingTalkLivenessCheckInterval = 30 * time.Second
	// dingTalkSilenceReconnect 超过这个时长没有任何帧就认为连接已死。
	// ⚠ 未在真实租户校准：服务端 ping 频率未知，取值偏大以避免误判。
	dingTalkSilenceReconnect = 10 * time.Minute
)

func (a *dingtalkAdapter) Name() string { return model.ChannelPlatformDingTalk }

// Run 建立钉钉 Stream 连接并把机器人消息/卡片回调标准化后交给 sink。
// ctx 取消时返回 nil；鉴权类错误直接返回错误，不做无限快速重连。
func (a *dingtalkAdapter) Run(ctx context.Context, creds PlatformCredentials, sink InboundSink) error {
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
			return nil
		}
		if err == nil {
			return nil
		}
		if dingTalkConnectFatalError(err) {
			return fmt.Errorf("钉钉 Stream 连接无法建立：%s", sanitizePlatformMessage(err.Error()))
		}
		log.Printf("[channel] 钉钉 Stream 连接中断：%s", sanitizePlatformMessage(err.Error()))
		if !waitBeforeReconnect(ctx, attempt, model.ChannelPlatformDingTalk) {
			return nil
		}
	}
}

// dingTalkRunState 记录最近一次收到帧的时间，用于兜底探活。
type dingTalkRunState struct {
	lastAliveNano atomic.Int64
}

func newDingTalkRunState() *dingTalkRunState {
	state := &dingTalkRunState{}
	state.touch()
	return state
}

func (s *dingTalkRunState) touch() { s.lastAliveNano.Store(time.Now().UnixNano()) }

func (s *dingTalkRunState) silentFor() time.Duration {
	return time.Since(time.Unix(0, s.lastAliveNano.Load()))
}

// runOnce 是一次完整的连接生命周期：建连 → 收回调 → 掉线/取消/探活超时后返回。
func (a *dingtalkAdapter) runOnce(ctx context.Context, creds PlatformCredentials, sink InboundSink) error {
	state := newDingTalkRunState()
	dropped := make(chan struct{}, 1)
	markDropped := func() {
		select {
		case dropped <- struct{}{}:
		default:
		}
	}

	stream := dclient.NewStreamClient(
		dclient.WithAppCredential(dclient.NewAppCredentialConfig(creds.AppId, creds.AppSecret)),
		dclient.WithAutoReconnect(false),
	)

	stream.RegisterChatBotCallbackRouter(func(callbackCtx context.Context, data *dchatbot.BotCallbackDataModel) ([]byte, error) {
		state.touch()
		// 回调跑在 SDK 的 goroutine 上：panic 会打穿到 SDK，必须自己兜住。
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("[channel] 钉钉消息回调发生 panic，已忽略本次事件")
			}
		}()
		inbound, ok := normalizeDingTalkInbound(data, creds.AppId)
		if !ok {
			log.Printf("[channel] 钉钉机器人回调缺少必要字段，已丢弃")
			return []byte(""), nil
		}
		if err := inbound.Validate(creds.AppId, creds.OrganizationId); err != nil {
			log.Printf("[channel] 钉钉入站事件未通过身份校验，已丢弃：%v", err)
			return []byte(""), nil
		}
		if err := sink(callbackCtx, inbound); err != nil {
			log.Printf("[channel] 钉钉入站事件处理失败 eventId=%s：%s", inbound.EventId, sanitizePlatformMessage(err.Error()))
		}
		return []byte(""), nil
	})

	stream.RegisterCardCallbackRouter(func(callbackCtx context.Context, request *dcard.CardRequest) (*dcard.CardResponse, error) {
		state.touch()
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("[channel] 钉钉卡片回调发生 panic，已忽略本次回调")
			}
		}()
		inbound, ok := normalizeDingTalkCardAction(request, creds.AppId)
		if !ok {
			return &dcard.CardResponse{}, nil
		}
		// 卡片回调不含会话标识：不适用 InboundEvent.Validate（它会因 ChatId 为空拒绝），
		// 由 T7 用卡片实例标识定位审批单。这里只把已验证的企业/操作人/动作交给 sink。
		if err := sink(callbackCtx, inbound); err != nil {
			log.Printf("[channel] 钉钉卡片回调处理失败 eventId=%s：%s", inbound.EventId, sanitizePlatformMessage(err.Error()))
		}
		return &dcard.CardResponse{}, nil
	})

	// 覆盖 SDK 默认的 SYSTEM/disconnect（原本只关连接、不通知上层）：现在既要关连接，
	// 也要让 runOnce 感知到掉线。返回 nil 表示这条帧不需要额外应答内容。
	stream.RegisterRouter(dutils.SubscriptionTypeKSystem, "disconnect", func(_ context.Context, _ *dpayload.DataFrame) (*dpayload.DataFrameResponse, error) {
		log.Printf("[channel] 钉钉 Stream 服务端下发断开指令")
		markDropped()
		return nil, nil
	})

	// ping 保留 SDK 原有语义（回 pong），同时作为存活信号。
	stream.RegisterRouter(dutils.SubscriptionTypeKSystem, "ping", func(_ context.Context, frame *dpayload.DataFrame) (*dpayload.DataFrameResponse, error) {
		state.touch()
		pong := dpayload.NewDataFrameAckPong(frame.GetMessageId())
		pong.Data = frame.Data
		return pong, nil
	})

	if err := stream.Start(ctx); err != nil {
		return err
	}
	// Start 返回后连接由 SDK 的 goroutine 维护；退出前必须主动关掉，否则读循环不会结束。
	defer stream.Close()

	ticker := time.NewTicker(dingTalkLivenessCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-dropped:
			return errors.New("服务端关闭了 Stream 连接")
		case <-ticker.C:
			if silent := state.silentFor(); silent >= dingTalkSilenceReconnect {
				// 读失败的 goroutine 只在 SDK 内部退出，外部无从得知，只能靠静默时长兜底。
				return fmt.Errorf("连接静默 %s，判定已掉线", silent.Round(time.Second))
			}
		}
	}
}

// normalizeDingTalkInbound 把机器人消息回调标准化为 InboundEvent。抽成纯函数以便离线断言。
//
// 字段口径（以 v0.9.0 chatbot.BotCallbackDataModel 为准）：
//   - EventId   ← msgId
//   - Kind      ← msgtype
//   - 组织标识   ← chatbotCorpId（机器人所属企业）；缺失时退回 senderCorpId
//   - UserId    ← senderStaffId（企业内 staffId，与绑定/目标配置同口径）
//   - ChatId    ← conversationId（群聊时即 openConversationId）
//   - ChatType  ← conversationType：1 单聊 → p2p，2 群聊 → group，其它留空
//   - Text      ← text.content（仅 msgtype == text），原文保留不裁剪
func normalizeDingTalkInbound(data *dchatbot.BotCallbackDataModel, appId string) (InboundEvent, bool) {
	if data == nil {
		return InboundEvent{}, false
	}
	inbound := InboundEvent{
		EventId:        strings.TrimSpace(data.MsgId),
		Kind:           strings.TrimSpace(data.Msgtype),
		OrganizationId: strings.TrimSpace(data.ChatbotCorpId),
		AppId:          appId,
		UserId:         strings.TrimSpace(data.SenderStaffId),
		ChatId:         strings.TrimSpace(data.ConversationId),
		ChatType:       normalizeDingTalkConversationType(data.ConversationType),
	}
	if inbound.OrganizationId == "" {
		inbound.OrganizationId = strings.TrimSpace(data.SenderCorpId)
	}
	if inbound.Kind == InboundKindText {
		inbound.Text = data.Text.Content
	}
	return inbound, true
}

// normalizeDingTalkConversationType 只映射文档明确的两种取值；其它取值留空，
// 交给 InboundEvent.Validate 拒绝——不猜「大概是群聊」。
func normalizeDingTalkConversationType(conversationType string) string {
	switch strings.TrimSpace(conversationType) {
	case "1":
		return ChatTypeP2P
	case "2":
		return ChatTypeGroup
	default:
		return ""
	}
}

// normalizeDingTalkCardAction 把互动卡片点击回调标准化为 InboundEvent。
//
// 卡片回调只带企业、操作人、卡片实例与动作参数，没有会话标识，
// 因此 ChatId / ChatType 留空（不编造），Text 放动作标识。
func normalizeDingTalkCardAction(request *dcard.CardRequest, appId string) (InboundEvent, bool) {
	if request == nil {
		return InboundEvent{}, false
	}
	outTrackId := strings.TrimSpace(request.OutTrackId)
	actionId := ""
	if actions := request.CardActionData.CardPrivateData.ActionIdList; len(actions) > 0 {
		actionId = strings.TrimSpace(actions[0])
	}
	inbound := InboundEvent{
		// 同一张卡片的同一次操作得到同一个 ID：重复点击由 T7 的条件更新消费兜住。
		EventId:        outTrackId + ":" + actionId,
		Kind:           InboundKindCardAction,
		OrganizationId: strings.TrimSpace(request.CorpId),
		AppId:          appId,
		UserId:         strings.TrimSpace(request.UserId),
		Text:           actionId,
	}
	if strings.TrimSpace(inbound.EventId) == ":" {
		// 卡片实例标识与动作标识都缺失：这条回调什么也定位不到。
		return InboundEvent{}, false
	}
	return inbound, true
}

// dingTalkConnectFatalError 判断建连错误是否属于「重试也不会好」。
//
// 依据 SDK 源码（v0.9.0 client.go:GetConnectionEndpoint）：取连接地址失败时
// 只返回 utils.ErrorFromHttpResponseBody 的裸文本（没有结构化状态码），
// 因此只能按关键字判定鉴权失败。判定偏保守：认不出来就当可重试，
// 退避本身已把重试间隔拉到 60 秒，不会形成快速重连风暴。
func dingTalkConnectFatalError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	for _, keyword := range []string{
		"invalid authentication",
		"invalidauthentication",
		"authentication failed",
		"invalid appkey",
		"invalid appsecret",
		"appkey or appsecret",
		"unauthorized",
	} {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// Send 发送一条文本消息：群聊走 groupMessages/send，单聊走 oToMessages/batchSend。
//
// 发送只需要一次尝试：重试、投递状态与下一步动作属于 T8 的 worker。
func (a *dingtalkAdapter) Send(ctx context.Context, target TargetRef, msg OutboundMessage) (SendResult, error) {
	creds := normalizePlatformCredentials(a.creds)
	if creds.AppId == "" || creds.AppSecret == "" {
		return SendResult{}, permanentSendError("钉钉适配器未绑定应用凭据，无法发送")
	}
	if err := validateTargetRef(target); err != nil {
		return SendResult{}, err
	}
	if strings.TrimSpace(msg.Text) == "" {
		return SendResult{}, permanentSendError("消息内容为空，未发送")
	}

	msgParam, err := json.Marshal(map[string]string{"content": msg.Text})
	if err != nil {
		return SendResult{}, permanentSendError("序列化钉钉文本消息失败：%v", err)
	}

	accessToken, tokenErr := a.accessToken(ctx, creds)
	if tokenErr != nil {
		return SendResult{}, tokenErr
	}

	path := dingTalkPrivateSendEndpoint
	body := map[string]any{
		"robotCode": creds.AppId,
		// robotCode 即自建应用的 appKey，与 stream 用的 clientId 是同一个值。
		"userIds":  []string{strings.TrimSpace(target.PlatformTargetId)},
		"msgKey":   dingTalkTextMsgKey,
		"msgParam": string(msgParam),
	}
	if target.Kind == TargetKindGroup {
		path = dingTalkGroupSendEndpoint
		body = map[string]any{
			"robotCode":          creds.AppId,
			"openConversationId": strings.TrimSpace(target.PlatformTargetId),
			"msgKey":             dingTalkTextMsgKey,
			"msgParam":           string(msgParam),
		}
	}

	var result struct {
		ProcessQueryKey           string   `json:"processQueryKey"`
		InvalidStaffIdList        []string `json:"invalidStaffIdList"`
		FlowControlledStaffIdList []string `json:"flowControlledStaffIdList"`
	}
	sendErr := dingTalkPostJSON(ctx, a.httpClientOrDefault(), a.openAPIHostOrDefault(), path, accessToken, body, &result)
	if sendErr != nil {
		var respErr *platformResponseError
		if errors.As(sendErr, &respErr) && respErr.StatusCode == http.StatusUnauthorized {
			// 缓存里的 token 失效：清掉它，让下一次发送重新取；本次仍按契约记为 permanent。
			dingTalkTokenCache.invalidate(creds.AppId)
		}
		return SendResult{}, ClassifyPlatformSendError(sendErr)
	}
	if len(result.InvalidStaffIdList) > 0 && result.ProcessQueryKey == "" {
		// 单聊目标不存在/已离职：重试也不会好。只要有回执说明已送达一部分，就不算失败。
		return SendResult{}, permanentSendError("钉钉单聊目标无效：%d 个目标不在可用范围", len(result.InvalidStaffIdList))
	}
	if len(result.FlowControlledStaffIdList) > 0 && result.ProcessQueryKey == "" {
		return SendResult{}, transientSendError("钉钉发送被限流，需稍后重试")
	}
	return SendResult{PlatformMessageId: result.ProcessQueryKey}, nil
}

// accessToken 返回可用的企业内部应用 accessToken（进程内缓存，带提前过期）。
func (a *dingtalkAdapter) accessToken(ctx context.Context, creds PlatformCredentials) (string, *SendError) {
	return dingTalkTokenCache.get(ctx, a.httpClientOrDefault(), a.openAPIHostOrDefault(), creds)
}

func (a *dingtalkAdapter) openAPIHostOrDefault() string {
	if host := strings.TrimSpace(a.openAPIHost); host != "" {
		return strings.TrimRight(host, "/")
	}
	return dingTalkOpenAPIHost
}

func (a *dingtalkAdapter) httpClientOrDefault() *http.Client {
	if a.httpClient != nil {
		return a.httpClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// dingTalkPostJSON 发一个 JSON 请求并反序列化 2xx 响应体。
// 非 2xx 转成带状态码的 platformResponseError；网络层失败原样返回，
// 让 ClassifyPlatformSendError 的网络分支把它判成 transient。
func dingTalkPostJSON(ctx context.Context, httpClient *http.Client, host, path, accessToken string, body any, out any) error {
	requestBody, err := json.Marshal(body)
	if err != nil {
		return newPlatformResponseError(model.ChannelPlatformDingTalk, 0, 0, "请求体序列化失败")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+path, bytes.NewReader(requestBody))
	if err != nil {
		return newPlatformResponseError(model.ChannelPlatformDingTalk, 0, 0, "构造请求失败")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if accessToken != "" {
		req.Header.Set("x-acs-dingtalk-access-token", accessToken)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return newPlatformResponseError(model.ChannelPlatformDingTalk, resp.StatusCode, 0, "读取响应失败")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newPlatformResponseError(model.ChannelPlatformDingTalk, resp.StatusCode, 0, dingTalkErrorText(raw))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return newPlatformResponseError(model.ChannelPlatformDingTalk, resp.StatusCode, 0, "响应解析失败")
		}
	}
	return nil
}

// dingTalkErrorText 从钉钉错误响应里取出可读文案（{code,message,requestid}）。
func dingTalkErrorText(raw []byte) string {
	var errBody struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &errBody); err == nil && (errBody.Code != "" || errBody.Message != "") {
		return strings.TrimSpace(errBody.Code + " " + errBody.Message)
	}
	if text := strings.TrimSpace(string(raw)); text != "" {
		return text
	}
	return "钉钉返回了空的错误响应"
}

// dingTalkTokenCache 是进程内 accessToken 缓存。
// 钉钉 token 有效期约 7200 秒，每次 Send 都重新取会让每条通知多一次网络往返，
// 也更容易触发平台限流。
type dingTalkTokenStore struct {
	mu        sync.Mutex
	appId     string
	token     string
	expiresAt time.Time
}

var dingTalkTokenCache = &dingTalkTokenStore{}

// get 返回可用 token。整个取 token 过程持锁：并发发送时只允许一个请求打平台，
// 代价是最坏情况下其它发送要等一次 token 请求（有 10 秒超时兜底）。
func (c *dingTalkTokenStore) get(ctx context.Context, httpClient *http.Client, host string, creds PlatformCredentials) (string, *SendError) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if c.token != "" && c.appId == creds.AppId && now.Add(dingTalkTokenRefreshSkew).Before(c.expiresAt) {
		return c.token, nil
	}

	adapterHost := host
	if strings.TrimSpace(adapterHost) == "" {
		adapterHost = dingTalkOpenAPIHost
	}
	var result struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int64  `json:"expireIn"`
	}
	if err := dingTalkPostJSON(ctx, httpClient, strings.TrimRight(adapterHost, "/"), dingTalkTokenEndpoint, "", map[string]string{
		"appKey":    creds.AppId,
		"appSecret": creds.AppSecret,
	}, &result); err != nil {
		return "", ClassifyPlatformSendError(err)
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		return "", transientSendError("钉钉未返回 accessToken")
	}

	ttl := dingTalkTokenFallbackTTL
	if result.ExpireIn > 0 {
		ttl = time.Duration(result.ExpireIn) * time.Second
	}
	c.appId = creds.AppId
	c.token = result.AccessToken
	c.expiresAt = now.Add(ttl)
	return c.token, nil
}

// invalidate 丢弃指定应用的缓存 token。只清同应用，避免串掉其它渠道的缓存。
func (c *dingTalkTokenStore) invalidate(appId string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.appId == appId {
		c.token = ""
		c.expiresAt = time.Time{}
	}
}
