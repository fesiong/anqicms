package provider

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"kandaoni.com/anqicms/model"
)

// 入站事件与出站消息的契约。本文件只放平台无关的定义，不引用任何平台 SDK：
// 各平台的 SDK 类型与协议细节留在 channelFeishu.go / channelDingTalk.go。

// 入站事件的 Kind 取值。聊天消息的 Kind 用平台原始消息类型（如 text/image/post），
// 卡片点击没有消息类型，统一为 card_action。
const (
	InboundKindText       = "text"
	InboundKindCardAction = "card_action"
)

// 会话类型。与飞书 chat_type 取值一致；钉钉 conversationType 需要映射到这两个值
// （1 单聊 / 2 群聊），映射不出来的取值一律留空，由 InboundEvent.Validate 拒绝。
const (
	ChatTypeP2P   = "p2p"
	ChatTypeGroup = "group"
)

// 发送目标类型。与 ChannelTarget.Kind 的库内取值一致（private 对应 channelBinding.go
// 的 channelTargetKindPrivate，测试里有断言锁住两者不许漂移）。
const (
	TargetKindGroup   = "group"
	TargetKindPrivate = "private"
)

// PlatformAdapter 是一个 IM 平台的适配器。
//
// 边界（规格 §3/§5）：Run 只做凭据校验、连接维持、事件标准化与调 sink；
// 租约、去重、权限、会话调度全部在 T4/T6。Send 只做一次发送尝试与错误分类；
// 重试、退避、投递状态在 T8。适配器不落库、不读渠道配置。
type PlatformAdapter interface {
	Name() string
	Run(ctx context.Context, creds PlatformCredentials, sink InboundSink) error
	Send(ctx context.Context, target TargetRef, msg OutboundMessage) (SendResult, error)
}

// InboundSink 消费一条已标准化的入站事件。返回错误表示本次处理失败（例如入库失败），
// 适配器只记录日志，不向平台回调返回错误——是否让平台重投由 T4 的持久化语义决定。
type InboundSink func(context.Context, InboundEvent) error

// InboundEvent 是平台事件标准化后的结果。字段全部来自平台回调，不含本地渠道配置。
type InboundEvent struct {
	EventId        string
	Kind           string
	OrganizationId string
	AppId          string
	UserId         string
	ChatId         string
	ChatType       string
	Text           string
}

// Validate 校验事件身份与渠道配置一致。expectedAppId/expectedOrgId 来自渠道配置。
//
// 严格相等：expectedOrgId 为空时不放行任何事件（fail closed）。错误文案是静态中文，
// 只说明「哪一项不匹配」，绝不回显标识值本身——这些错误会进日志。
//
// 卡片点击事件（Kind == InboundKindCardAction）例外：平台卡片回调不携带会话标识，
// 由调用方按 T7 的审批流程处理，不走本方法（见 channelDingTalk.go 的说明）。
func (e InboundEvent) Validate(expectedAppId, expectedOrgId string) error {
	if strings.TrimSpace(e.EventId) == "" {
		return errors.New("事件标识不能为空")
	}
	if strings.TrimSpace(e.UserId) == "" {
		return errors.New("平台用户标识不能为空")
	}
	if strings.TrimSpace(e.ChatId) == "" {
		return errors.New("会话标识不能为空")
	}
	if strings.TrimSpace(e.AppId) != strings.TrimSpace(expectedAppId) {
		return errors.New("事件来源应用与渠道配置不一致")
	}
	if strings.TrimSpace(e.OrganizationId) != strings.TrimSpace(expectedOrgId) {
		return errors.New("事件来源组织与渠道配置不一致")
	}
	switch e.ChatType {
	case ChatTypeP2P, ChatTypeGroup:
	default:
		return errors.New("不支持的会话类型")
	}
	return nil
}

// PlatformCredentials 是一个渠道的平台应用凭据。AppSecret 是密钥：
// String() 做了脱敏，避免被 fmt 顺手打进日志。
type PlatformCredentials struct {
	AppId          string
	AppSecret      string
	OrganizationId string
}

// String 实现 fmt.Stringer，只用于日志安全：密钥永不输出。
func (c PlatformCredentials) String() string {
	return fmt.Sprintf(
		"PlatformCredentials{AppId:%q, OrganizationId:%q, AppSecret:<脱敏>}",
		strings.TrimSpace(c.AppId), strings.TrimSpace(c.OrganizationId),
	)
}

// TargetRef 是一次发送的目标。Kind 取 TargetKindGroup / TargetKindPrivate。
type TargetRef struct {
	Kind             string
	PlatformTargetId string
}

// OutboundMessage 是一次发送的内容。首版只发文本；Card 预留给后续卡片能力。
type OutboundMessage struct {
	Text string
	Card map[string]any
}

// SendResult 是发送成功后的回执。PlatformMessageId 是平台消息 ID（可能为空：
// 平台吞掉回执时仍算发送成功，只是拿不到可追踪 ID）。
type SendResult struct {
	PlatformMessageId string
}

// ErrorClass 是发送错误的唯一分类维度：调用方只据此决定「重试」还是「放弃」。
type ErrorClass string

const (
	// ErrorClassPermanent 表示重试不会变好：本地校验失败、目标不存在、鉴权/权限不足。
	ErrorClassPermanent ErrorClass = "permanent"
	// ErrorClassTransient 表示可能自愈：限流、服务端 5xx、网络错误、超时、未知错误。
	ErrorClassTransient ErrorClass = "transient"
)

// SendError 是适配器返回的唯一错误形态（禁止裸错误外泄，上层不需要猜）。
type SendError struct {
	Class   ErrorClass
	Message string
}

func (e *SendError) Error() string {
	if e == nil {
		return ""
	}
	return string(e.Class) + ": " + e.Message
}

// NewSendError 供调用方构造同形态错误（例如本地缺凭据）。
func NewSendError(class ErrorClass, message string) *SendError {
	return &SendError{Class: class, Message: message}
}

func permanentSendError(format string, args ...any) *SendError {
	return &SendError{Class: ErrorClassPermanent, Message: fmt.Sprintf(format, args...)}
}

func transientSendError(format string, args ...any) *SendError {
	return &SendError{Class: ErrorClassTransient, Message: fmt.Sprintf(format, args...)}
}

// platformResponseError 保留平台返回的原始状态码与业务码。
// 两个码都要留着：飞书的鉴权错误是「HTTP 200 + 业务码 99991663」，
// 只看 HTTP 状态会把永久错误当可重试，白等一轮重试。
type platformResponseError struct {
	Platform   string
	StatusCode int
	BizCode    int
	Message    string
}

func (e *platformResponseError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s 返回错误（status=%d code=%d）：%s",
		e.Platform, e.StatusCode, e.BizCode, sanitizePlatformMessage(e.Message))
}

func newPlatformResponseError(platform string, statusCode, bizCode int, message string) *platformResponseError {
	return &platformResponseError{
		Platform:   platform,
		StatusCode: statusCode,
		BizCode:    bizCode,
		Message:    message,
	}
}

// platformErrorClassifiers 由各平台适配器在 init 里登记，用于识别平台 SDK 自己的
// 错误类型（如 larkcore.CodeError）。放在注册表里而不是直接 import，是为了让
// 本文件保持平台无关；返回 nil 表示「不认识这个错误，交给下一个分类器」。
var platformErrorClassifiers []func(error) *SendError

func registerPlatformErrorClassifier(classify func(error) *SendError) {
	platformErrorClassifiers = append(platformErrorClassifiers, classify)
}

// ClassifyPlatformSendError 把任意错误归入 permanent / transient。
//
// 规则（T3 契约）：
//   - 已是 *SendError：原样返回；
//   - context.Canceled / DeadlineExceeded：transient（取消时调用方可直接停止）；
//   - 平台响应错误：401/403 与其余 4xx → permanent；429/408/425 与 5xx → transient；
//     HTTP 200 但业务码非 0 → 按平台业务码判定，未知业务码 → transient；
//   - 平台 SDK 自有错误类型：交给各平台登记的分类器；
//   - 网络错误（net.Error / url.Error）→ transient；
//   - 其它未知错误 → transient。
//
// 未知错误默认 transient 是刻意的：宁可多试一次，也不要因为一次未识别的抖动
// 永久丢弃一条通知；代价是平台持续故障时会有少量无效重试。
func ClassifyPlatformSendError(err error) *SendError {
	if err == nil {
		return nil
	}
	var sendErr *SendError
	if errors.As(err, &sendErr) {
		return sendErr
	}
	// 取消与超时要先判：它们常被 net/url 包包装成 url.Error，落到网络分支也还是
	// transient，但只有单独判出来才能给出「调用方可直接停止」的明确文案。
	if errors.Is(err, context.Canceled) {
		return transientSendError("请求已取消，调用方可直接停止重试：%s", sanitizePlatformMessage(err.Error()))
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return transientSendError("请求超时：%s", sanitizePlatformMessage(err.Error()))
	}
	var respErr *platformResponseError
	if errors.As(err, &respErr) {
		return classifyPlatformResponseError(respErr)
	}
	for _, classify := range platformErrorClassifiers {
		if out := classify(err); out != nil {
			return out
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return transientSendError("网络错误：%s", sanitizePlatformMessage(err.Error()))
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return transientSendError("请求失败：%s", sanitizePlatformMessage(err.Error()))
	}
	return transientSendError("未识别的平台错误：%s", sanitizePlatformMessage(err.Error()))
}

func classifyPlatformResponseError(respErr *platformResponseError) *SendError {
	detail := sanitizePlatformMessage(respErr.Message)
	switch {
	case respErr.StatusCode == 401 || respErr.StatusCode == 403:
		return permanentSendError("%s 鉴权或权限不足（HTTP %d）：%s", respErr.Platform, respErr.StatusCode, detail)
	case respErr.StatusCode == 408 || respErr.StatusCode == 425 || respErr.StatusCode == 429:
		return transientSendError("%s 限流或超时（HTTP %d）：%s", respErr.Platform, respErr.StatusCode, detail)
	case respErr.StatusCode >= 500:
		return transientSendError("%s 服务端异常（HTTP %d）：%s", respErr.Platform, respErr.StatusCode, detail)
	case respErr.StatusCode >= 400:
		return permanentSendError("%s 拒绝了请求（HTTP %d）：%s", respErr.Platform, respErr.StatusCode, detail)
	}
	// HTTP 2xx（或没有 HTTP 状态）时只能看业务码。
	if respErr.BizCode != 0 {
		if class, ok := classifyPlatformBizCode(respErr.Platform, respErr.BizCode); ok {
			return &SendError{
				Class: class,
				Message: fmt.Sprintf("%s 业务错误（code=%d）：%s",
					respErr.Platform, respErr.BizCode, detail),
			}
		}
		return transientSendError("%s 未知业务错误（code=%d）：%s", respErr.Platform, respErr.BizCode, detail)
	}
	return transientSendError("%s 未返回有效结果（status=%d）：%s", respErr.Platform, respErr.StatusCode, detail)
}

// sanitizePlatformMessage 把平台返回的内容压成一行、限长，便于写日志与错误文案。
// 只做形式处理：平台文案本身不含本应用的 secret，不要把凭据拼进来。
func sanitizePlatformMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "（平台未返回详细信息）"
	}
	message = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(message)
	if runes := []rune(message); len(runes) > 300 {
		message = string(runes[:300]) + "…"
	}
	return message
}

// validatePlatformCredentials 是所有适配器建立连接前的统一校验。
// 错误文案不含凭据内容。
func validatePlatformCredentials(creds PlatformCredentials) error {
	if strings.TrimSpace(creds.AppId) == "" {
		return errors.New("平台应用 ID 未配置")
	}
	if strings.TrimSpace(creds.AppSecret) == "" {
		return errors.New("平台应用密钥未配置")
	}
	if strings.TrimSpace(creds.OrganizationId) == "" {
		// 组织标识是入站事件的校验基准，空值等于放行一切，必须在这里拦住。
		return errors.New("平台组织标识未配置")
	}
	return nil
}

func normalizePlatformCredentials(creds PlatformCredentials) PlatformCredentials {
	return PlatformCredentials{
		AppId:          strings.TrimSpace(creds.AppId),
		AppSecret:      strings.TrimSpace(creds.AppSecret),
		OrganizationId: strings.TrimSpace(creds.OrganizationId),
	}
}

// validateTargetRef 校验发送目标。本地校验失败是 permanent：重试也一样。
func validateTargetRef(target TargetRef) *SendError {
	if strings.TrimSpace(target.PlatformTargetId) == "" {
		return permanentSendError("发送目标标识为空")
	}
	switch target.Kind {
	case TargetKindGroup, TargetKindPrivate:
		return nil
	default:
		return permanentSendError("不支持的发送目标类型")
	}
}

// PlatformIdentityHash 是「站点 + 渠道 + 平台 + 组织 + 平台用户」的身份摘要。
//
// 复用 channelDigest（逐段 TrimSpace → \x1f 连接 → SHA-256 → 小写 hex），
// 与绑定身份的摘要协议完全一致；站点参与摘要，因此同一个平台用户在不同站点
// 是两条互不相干的身份。
func PlatformIdentityHash(siteId uint, channelId uint, platform, orgId, userId string) string {
	return channelDigest(
		strconv.FormatUint(uint64(siteId), 10),
		strconv.FormatUint(uint64(channelId), 10),
		platform,
		orgId,
		userId,
	)
}

// platformAdapterRegistry 是平台名到适配器原型的映射。
// 原型不带凭据：Run 由调用方逐次传 creds，Send 必须用 NewPlatformAdapter 绑定凭据。
var platformAdapterRegistry = map[string]PlatformAdapter{
	model.ChannelPlatformFeishu:   &feishuAdapter{},
	model.ChannelPlatformDingTalk: &dingtalkAdapter{},
}

// GetPlatformAdapter 返回未绑定凭据的适配器原型；未知平台返回 false（不兜底）。
func GetPlatformAdapter(name string) (PlatformAdapter, bool) {
	adapter, ok := platformAdapterRegistry[strings.ToLower(strings.TrimSpace(name))]
	return adapter, ok
}

// NewPlatformAdapter 返回已绑定凭据的适配器实例。Send 需要凭据（PlatformAdapter.Send
// 的签名里没有 creds），所以投递侧必须用它构造，不能拿注册表里的原型发消息。
func NewPlatformAdapter(name string, creds PlatformCredentials) (PlatformAdapter, bool) {
	creds = normalizePlatformCredentials(creds)
	switch strings.ToLower(strings.TrimSpace(name)) {
	case model.ChannelPlatformFeishu:
		return &feishuAdapter{creds: creds}, true
	case model.ChannelPlatformDingTalk:
		return &dingtalkAdapter{creds: creds}, true
	default:
		return nil, false
	}
}

// PlatformNames 返回已实现平台名，按字典序固定排序（便于比对与展示）。
func PlatformNames() []string {
	names := make([]string, 0, len(platformAdapterRegistry))
	for name := range platformAdapterRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// 长连接重连的退避参数（规格 §5：1 秒起、60 秒封顶、带抖动）。
const (
	platformReconnectMinDelay = time.Second
	platformReconnectMaxDelay = 60 * time.Second
)

// platformReconnectDelay 返回第 attempt 次（从 0 开始）重连的等待基准：1s 起翻倍、60s 封顶。
// 抖动在 waitBeforeReconnect 里叠加，两者分开是为了让基准可被直接断言。
func platformReconnectDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	// 防止移位溢出：60s 封顶后 attempt 再大也没有意义。
	if attempt > 16 {
		attempt = 16
	}
	delay := platformReconnectMinDelay << uint(attempt)
	if delay <= 0 || delay > platformReconnectMaxDelay {
		return platformReconnectMaxDelay
	}
	return delay
}

// waitBeforeReconnect 等待一次带抖动的退避。返回 false 表示 ctx 已取消（应立即停止）。
//
// 抖动取 [base/2, base]：只向下抖动，既保证「至少等一会儿」不打爆平台，
// 又让多个渠道实例不会在同一毫秒一起重连。
func waitBeforeReconnect(ctx context.Context, attempt int, platform string) bool {
	base := platformReconnectDelay(attempt)
	half := base / 2
	delay := half + rand.N(half+1)
	log.Printf("[channel] %s 长连接准备重连：第 %d 次，等待 %s", platform, attempt+1, delay)
	select {
	case <-ctx.Done():
		return false
	case <-time.After(delay):
		return true
	}
}
