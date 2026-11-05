package provider

import (
	"context"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"kandaoni.com/anqicms/model"
)

// 绑定消息的判定形态：32 字节随机 token 的 base64url 无填充编码恒为 43 个字符。
// 只认完整匹配，避免把正文里恰好出现的长串当成绑定口令。
var (
	bindingTokenShapePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	bindingTokenLikePattern  = regexp.MustCompile(`[A-Za-z0-9_-]{43,}`)
)

// channelWorkerState 是 worker 的退出状态。
// 只有适配器 Run 自己返回才是终态（裁决：不自动重连）；
// 「未取得租约 / 适配器不可用 / 失去租约」是可重试退出，由管理器退避后重新尝试。
type channelWorkerState int32

const (
	// channelWorkerBuilding：正在建连（构造适配器、抢租约）。
	channelWorkerBuilding channelWorkerState = iota
	// channelWorkerRunning：适配器 Run 正在运行。
	channelWorkerRunning
	// channelWorkerRetryable：可重试退出，等待下一轮扫描退避重试。
	channelWorkerRetryable
	// channelWorkerTerminal：Run 已返回，不自动重连，等配置变化或显式重建。
	channelWorkerTerminal
)

// channelWorker 负责单个渠道的连接生命周期：持有租约、跑适配器、处理入站事件。
// 入站 sink 会被 SDK 多个 goroutine 并发调用（见 InboundSink 注释），
// 这里只做「去重 + 落库」/「绑定消费」，不为 inbox 增加按会话串行队列（T6 职责）。
type channelWorker struct {
	manager   *channelManager
	channel   model.MessageChannel
	signature channelConfigSignature

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	// releaseOnStop：本 worker 退出时是否释放租约。
	// 重建时置 false（租约保持给新连接）；失去租约/停止超时时也不释放。
	releaseOnStop atomic.Bool
	// pendingRelease 记录调用方希望的释放意图：停止超时后的下一轮重试沿用它，
	// 避免重试把本该保留的租约误释放。
	pendingRelease atomic.Bool
	// abandoned：停止等待超时，旧连接仍在运行。退出前不得建立新连接。
	abandoned atomic.Bool
	leaseLost atomic.Bool
	state     atomic.Int32

	lease atomic.Pointer[model.ChannelLease]

	mu      sync.Mutex
	lastErr string
}

func (wk *channelWorker) currentState() channelWorkerState {
	return channelWorkerState(wk.state.Load())
}

func (wk *channelWorker) pendingReleaseValue() bool {
	return wk.pendingRelease.Load()
}

func (wk *channelWorker) leaseSnapshot() *model.ChannelLease {
	return wk.lease.Load()
}

// waitExit 在 timeout 内等待 worker 退出。
func (wk *channelWorker) waitExit(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-wk.done:
		return true
	case <-timer.C:
		return false
	}
}

// finishRetryable 记录可重试退出并安排下一次尝试（管理器按渠道记录退避）。
// 必须先写退避状态再发布 channelWorkerRetryable：反过来的话，扫描循环可能
// 先看到「可重试」而还没看到截止时间，把退避整个跳过一次。
func (wk *channelWorker) finishRetryable() {
	wk.manager.noteWorkerRetryable(wk.channel.Id)
	wk.state.Store(int32(channelWorkerRetryable))
}

func (wk *channelWorker) run() {
	defer close(wk.done)
	manager := wk.manager
	channel := &wk.channel

	adapter, ok := manager.opts.newAdapter(channel.Platform, PlatformCredentials{
		AppId:          channel.AppId,
		AppSecret:      channel.AppSecret,
		OrganizationId: channel.OrgId,
	})
	if !ok {
		wk.setLastError("平台适配器不可用（平台未知或凭据不完整）")
		log.Printf("[channel] 站点 %d 渠道 %d 未建立连接：%s", manager.site.Id, channel.Id, wk.lastError())
		wk.finishRetryable()
		return
	}

	// 先抢租约再连接：同一企业应用在同一时刻只允许一个实例运行。
	// 重试路径同样必须重新取得租约，不能因为「上一轮没抢到」就跳过这一步。
	lease := manager.acquireLease(channel)
	if lease == nil {
		log.Printf("[channel] 站点 %d 渠道 %d 未取得连接租约，本实例不建立连接（退避后重试）",
			manager.site.Id, channel.Id)
		wk.finishRetryable()
		return
	}
	wk.lease.Store(lease)
	wk.state.Store(int32(channelWorkerRunning))
	manager.noteWorkerRunning(wk)

	renewCtx, renewCancel := context.WithCancel(wk.ctx)
	renewDone := make(chan struct{})
	go func() {
		defer close(renewDone)
		wk.renewLoop(renewCtx, lease)
	}()

	// 适配器内部已负责网络重连与退避；这里不再套一层重连循环。
	// Run 返回即终态：记录错误，等配置变化或显式重建才重试。
	err := adapter.Run(wk.ctx, PlatformCredentials{
		AppId:          channel.AppId,
		AppSecret:      channel.AppSecret,
		OrganizationId: channel.OrgId,
	}, wk.inboundSink(adapter))

	renewCancel()
	<-renewDone

	if err != nil {
		wk.setLastError(sanitizePlatformMessage(err.Error()))
		log.Printf("[channel] 站点 %d 渠道 %d 连接终止：%s", manager.site.Id, channel.Id, wk.lastError())
	}

	if wk.leaseLost.Load() {
		// 租约已不归本实例：条件删除也会失败，不释放。
		wk.releaseOnStop.Store(false)
		log.Printf("[channel] 站点 %d 渠道 %d 连接因失去租约停止（退避后重试接管）", manager.site.Id, channel.Id)
		wk.finishRetryable()
		return
	}

	wk.state.Store(int32(channelWorkerTerminal))
	if wk.releaseOnStop.Load() && !manager.abandoned.Load() {
		manager.releaseLease(lease)
	}
}

// renewLoop 每 renewEvery 续租一次；续租失败（RowsAffected != 1）即失去租约，
// 立即取消该渠道的 Run 上下文，停止本地接收。
func (wk *channelWorker) renewLoop(ctx context.Context, lease *model.ChannelLease) {
	manager := wk.manager
	ticker := time.NewTicker(manager.opts.renewEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !manager.renewLease(wk.channel.Id, lease) {
				wk.leaseLost.Store(true)
				log.Printf("[channel] 站点 %d 渠道 %d 失去连接租约，立即停止本地接收",
					manager.site.Id, wk.channel.Id)
				wk.cancel()
				return
			}
		}
	}
}

func (wk *channelWorker) setLastError(message string) {
	wk.mu.Lock()
	wk.lastErr = message
	wk.mu.Unlock()
}

func (wk *channelWorker) lastError() string {
	wk.mu.Lock()
	defer wk.mu.Unlock()
	return wk.lastErr
}

func (wk *channelWorker) inboundSink(adapter PlatformAdapter) InboundSink {
	return func(ctx context.Context, event InboundEvent) error {
		return wk.handleInbound(ctx, adapter, event)
	}
}

// handleInbound 是入站处理顺序（规格 §6/§7）：
//  1. 身份校验失败 → 丢弃；
//  2. EventId 为空 → 丢弃并记脱敏日志（R22，优先于绑定判定，避免无标识事件消费令牌）；
//  3. 群聊未 @ 机器人 → 直接丢弃，不落 Inbox（§7 第一层门禁；第二层由 T6 做）；
//  4. 私聊 + token 形态 → 绑定路径（绝不写入 ChannelInbox、绝不写日志、绝不回显 token）；
//  5. 其它 → 按 EventHash 去重后写入 ChannelInbox，并应答平台。
//
// T4 不执行模型或管理动作（T6 职责）。
func (wk *channelWorker) handleInbound(ctx context.Context, adapter PlatformAdapter, event InboundEvent) error {
	manager := wk.manager
	channel := &wk.channel

	if err := event.ValidateIdentity(channel.AppId, channel.OrgId); err != nil {
		log.Printf("[channel] 站点 %d 渠道 %d 入站事件身份校验失败，已丢弃：%s",
			manager.site.Id, channel.Id, sanitizePlatformMessage(err.Error()))
		return nil
	}

	eventId := strings.TrimSpace(event.EventId)
	if eventId == "" {
		// 空摘要会互相撞唯一键，把所有空事件去重成一条，所以直接丢弃。
		log.Printf("[channel] 站点 %d 渠道 %d 入站事件缺少事件标识，已丢弃（类型=%s）",
			manager.site.Id, channel.Id, sanitizePlatformMessage(event.Kind))
		return nil
	}

	if event.ChatType == ChatTypeGroup && !event.MentionedBot {
		// 规格 §7 第一层门禁：群聊里没 @ 机器人的消息不进入处理链路，也不落 Inbox。
		// 只记类型与事件标识，不记正文（可能是业务敏感内容）。
		log.Printf("[channel] 站点 %d 渠道 %d 群聊消息未 @ 机器人，已丢弃（类型=%s 事件=%s）",
			manager.site.Id, channel.Id, sanitizePlatformMessage(event.Kind), sanitizePlatformMessage(eventId))
		return nil
	}

	text := strings.TrimSpace(event.Text)
	if event.ChatType == ChatTypeP2P && isBindingTokenShape(text) {
		wk.handleBindingAttempt(ctx, adapter, event, text)
		return nil
	}

	inbox := &model.ChannelInbox{
		SiteId:    manager.site.Id,
		ChannelId: channel.Id,
		EventKind: event.Kind,
		Content:   sanitizeInboundContent(event.Text),
		Status:    model.InboxStatusPending,
		EventHash: channelDigest(
			strconv.FormatUint(uint64(manager.site.Id), 10),
			strconv.FormatUint(uint64(channel.Id), 10),
			channel.Platform,
			eventId,
		),
	}
	if err := manager.site.DB.WithContext(ctx).Create(inbox).Error; err != nil {
		if isDuplicateKeyError(err) {
			// 平台重投 / 并发重复：唯一键已存在，视为成功。
			return nil
		}
		log.Printf("[channel] 站点 %d 渠道 %d 入站事件落库失败：%s",
			manager.site.Id, channel.Id, sanitizePlatformMessage(err.Error()))
		return err
	}
	return nil
}

// handleBindingAttempt 处理绑定消息。无论成功失败都回一句固定中文；
// 发送失败只记日志，不影响已完成的绑定。
func (wk *channelWorker) handleBindingAttempt(ctx context.Context, adapter PlatformAdapter, event InboundEvent, token string) {
	manager := wk.manager
	channel := &wk.channel

	userId := strings.TrimSpace(event.UserId)
	if userId == "" {
		log.Printf("[channel] 站点 %d 渠道 %d 绑定消息缺少用户标识，已丢弃", manager.site.Id, channel.Id)
		return
	}

	key := channelBindingAttemptKey(manager.site.Id, channel.Id, userId)
	var reply string
	if !manager.allowBindingAttempt(key) {
		// R21：超限后不再调用 ConsumeBindingToken。
		reply = channelBindingRateLimitReply
	} else {
		_, err := manager.site.ConsumeBindingToken(token, ChannelPlatformIdentity{
			ChannelId:      channel.Id,
			OrganizationId: event.OrganizationId,
			AppId:          event.AppId,
			UserId:         userId,
			IsGroup:        event.ChatType == ChatTypeGroup,
		})
		if err != nil {
			manager.recordBindingFailure(key)
			// 失败文案由 ConsumeBindingToken 保证是静态中文、不含 token。
			reply = err.Error()
		} else {
			manager.clearBindingFailures(key)
			reply = channelBindingSuccessReply
		}
	}

	sendCtx, cancel := context.WithTimeout(ctx, channelBindingReplyTimeout)
	defer cancel()
	if _, err := adapter.Send(sendCtx, TargetRef{
		Kind:             TargetKindPrivate,
		PlatformTargetId: userId,
	}, OutboundMessage{Text: reply}); err != nil {
		log.Printf("[channel] 站点 %d 渠道 %d 绑定回执发送失败：%s",
			manager.site.Id, channel.Id, sanitizePlatformMessage(err.Error()))
	}
}

// isBindingTokenShape 判断整段文本是否是 32 字节 base64url token 的形态（43 个字符）。
func isBindingTokenShape(text string) bool {
	return bindingTokenShapePattern.MatchString(strings.TrimSpace(text))
}

// sanitizeInboundContent 只保留脱敏后的正文：
// 先抹掉 token 形态的串（防御性：用户可能把绑定口令粘贴到普通消息里），
// 再走通用密钥脱敏，最后限长。
func sanitizeInboundContent(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = bindingTokenLikePattern.ReplaceAllString(text, "[REDACTED]")
	text = redactSecrets(text)
	if runes := []rune(text); len(runes) > channelInboxContentMaxRunes {
		text = string(runes[:channelInboxContentMaxRunes]) + "…"
	}
	return text
}
