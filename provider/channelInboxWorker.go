package provider

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"kandaoni.com/anqicms/model"
)

// channelInboxWorker.go 是入站任务的消费侧（T12a）：把 channel_inbox 里的
// pending 行变成一次渠道轮次（执行 → 回复）。
//
// 设计边界：
//   - 抢占只看数据库：status + lease_until + 条件更新，多实例互斥，进程内不再维护队列；
//   - 单个 worker 内串行消费（每次只认领一条），适配器并发回调只负责落库；
//   - 通知/留言内容是**不可信数据**：只当作用户输入交给执行器，绝不按指令解释；
//   - 未绑定来源不执行、不回复（不泄露任何信息），只留下去敏原因；
//   - 群聊的 principal 仍然来自**发送者自己的绑定**，不继承群主或通知目标的权限。

const (
	// channelInboxLease 是单条入站任务的执行租约：超时后其它实例可重新认领。
	channelInboxLease = 60 * time.Second
	// channelInboxPollInterval 是队列空闲时的轮询间隔。
	channelInboxPollInterval = time.Second
	// channelInboxReasonLimit 是 last_error 的字符上限（与列宽 varchar(255) 对齐）。
	channelInboxReasonLimit = 255
	// channelInboxFinishTimeout 是终态写入的超时：写入必须脱离调用方的取消信号
	// （见 finishInbox），脱离了之后就得自己带上限，不能无限等库。
	channelInboxFinishTimeout = 5 * time.Second
)

// last_error 里固定的忽略原因。文案不含正文、身份摘要或任何平台标识原文。
const (
	channelInboxReasonChatDisabled     = "聊天管理未开启"
	channelInboxReasonCardAction       = "卡片回调未启用"
	channelInboxReasonChannelMissing   = "渠道不存在"
	channelInboxReasonUnbound          = "未绑定"
	channelInboxReasonAgentUnavailable = "渠道未配置接待智能体"
	channelInboxReasonLoadFailed       = "读取处理上下文失败"
	channelInboxReasonTurnFailed       = "渠道轮次执行失败"
)

// runChannelTurnForInbox 是消费侧的**包级可替换接缝**。
// 生产默认实现是 RunChannelTurn；测试替换为 svc.runChannelTurn + 脚本模型。
// 除测试以外不得改写这个变量（改它会绕开真实的权限校验链路）。
var runChannelTurnForInbox = func(svc *AiChatService, ctx context.Context, req ChannelTurnRequest) (ChannelTurnResult, error) {
	return svc.RunChannelTurn(ctx, req)
}

// channelInboxConsumer 串行消费单个渠道的入站任务。
type channelInboxConsumer struct {
	site      *Website
	channelId uint
	adapter   PlatformAdapter
	// pollEvery 是空闲轮询间隔；测试注入更短值，生产用 channelInboxPollInterval。
	pollEvery time.Duration
}

func newChannelInboxConsumer(site *Website, channelId uint, adapter PlatformAdapter) *channelInboxConsumer {
	return &channelInboxConsumer{
		site:      site,
		channelId: channelId,
		adapter:   adapter,
		pollEvery: channelInboxPollInterval,
	}
}

// run 在 ctx 取消前持续消费；每个 worker 生命周期内只有一个消费者 goroutine。
func (c *channelInboxConsumer) run(ctx context.Context) {
	interval := c.pollEvery
	if interval <= 0 {
		interval = channelInboxPollInterval
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		for ctx.Err() == nil && c.processNext(ctx) {
			// 有积压就连续处理，不再等一个轮询周期。
		}
		timer.Reset(interval)
	}
}

// processNext 认领并处理一条任务。返回 false 表示当前没有可处理的任务（或认领失败）。
func (c *channelInboxConsumer) processNext(ctx context.Context) bool {
	inbox, ok, err := c.claimNext(ctx)
	if err != nil {
		log.Printf("[channel] 站点 %d 渠道 %d 认领入站任务失败：%s",
			c.site.Id, c.channelId, sanitizePlatformMessage(err.Error()))
		return false
	}
	if !ok {
		return false
	}
	c.processInbox(ctx, inbox)
	return true
}

// claimNext 按 id 升序认领一条 pending（或租约已过期的 processing）任务。
//
// 条件更新以「id + 原状态 + 原 lease_until」为条件：并发下只有一个实例能把
// RowsAffected 置 1，其余重新取。不用「先更新状态再查」：那会把别人的租约抢走。
func (c *channelInboxConsumer) claimNext(ctx context.Context) (*model.ChannelInbox, bool, error) {
	if c.site == nil || c.site.DB == nil {
		return nil, false, errors.New("站点未就绪")
	}
	now := time.Now().Unix()
	var row model.ChannelInbox
	err := c.site.DB.WithContext(ctx).
		Where("`site_id` = ? AND `channel_id` = ? AND (`status` = ? OR (`status` = ? AND `lease_until` <= ?))",
			c.site.Id, c.channelId, model.InboxStatusPending, model.InboxStatusProcessing, now).
		Order("`id` asc").
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	leaseUntil := now + int64(channelInboxLease/time.Second)
	res := c.site.DB.WithContext(ctx).Model(&model.ChannelInbox{}).
		Where("`id` = ? AND `status` = ? AND `lease_until` = ?", row.Id, row.Status, row.LeaseUntil).
		Updates(map[string]any{
			"status":      model.InboxStatusProcessing,
			"lease_until": leaseUntil,
		})
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected != 1 {
		// 被别的实例抢先：本轮不处理，下一轮重新取。
		return nil, false, nil
	}
	row.Status = model.InboxStatusProcessing
	row.LeaseUntil = leaseUntil
	return &row, true, nil
}

// processInbox 处理一条已认领的任务并落最终状态。
//
// 顺序不可调换（规格 §3）：
//  0. 仅通知渠道 → done；
//  1. 卡片回调 → done（首版不发卡片，防御性忽略）；
//  2. 未配置接待智能体 / 站点服务缺失 → failed；
//  3. 未绑定 → done，不回复；
//  4. 其余 → 执行一轮并回复；
//  5. 成功/忽略 → done，处理错误 → failed。
func (c *channelInboxConsumer) processInbox(ctx context.Context, inbox *model.ChannelInbox) {
	status, reason := c.consumeInbox(ctx, inbox)
	c.finishInbox(ctx, inbox.Id, status, reason)
}

func (c *channelInboxConsumer) consumeInbox(ctx context.Context, inbox *model.ChannelInbox) (string, string) {
	if inbox == nil {
		return model.InboxStatusFailed, channelInboxReasonLoadFailed
	}
	site := c.site
	if site == nil || site.DB == nil {
		return model.InboxStatusFailed, channelInboxReasonLoadFailed
	}
	var channel model.MessageChannel
	err := site.DB.WithContext(ctx).
		Where("`id` = ? AND `site_id` = ?", c.channelId, site.Id).
		Take(&channel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.InboxStatusDone, channelInboxReasonChannelMissing
		}
		log.Printf("[channel] 站点 %d 渠道 %d 读取渠道失败：%s",
			site.Id, c.channelId, sanitizePlatformMessage(err.Error()))
		return model.InboxStatusFailed, channelInboxReasonLoadFailed
	}
	if channel.ChatEnabled != 1 {
		return model.InboxStatusDone, channelInboxReasonChatDisabled
	}
	if strings.TrimSpace(inbox.EventKind) == InboundKindCardAction {
		return model.InboxStatusDone, channelInboxReasonCardAction
	}
	if channel.AgentId == 0 || site.AiSrv == nil {
		log.Printf("[channel] 站点 %d 渠道 %d 无法执行入站任务：接待智能体或聊天服务未就绪",
			site.Id, channel.Id)
		return model.InboxStatusFailed, channelInboxReasonAgentUnavailable
	}

	binding, err := c.loadBinding(ctx, inbox.IdentityHash)
	if err != nil {
		log.Printf("[channel] 站点 %d 渠道 %d 读取绑定失败：%s",
			site.Id, channel.Id, sanitizePlatformMessage(err.Error()))
		return model.InboxStatusFailed, channelInboxReasonLoadFailed
	}
	if binding == nil {
		// 未绑定来源不进入模型、不回复：任何回执都等于向陌生人确认「机器人在这里」。
		return model.InboxStatusDone, channelInboxReasonUnbound
	}

	group := strings.TrimSpace(inbox.ChatType) == ChatTypeGroup
	req, requestErr := c.turnRequest(ctx, &channel, binding, inbox, group)
	if requestErr != nil {
		log.Printf("[channel] 站点 %d 渠道 %d 准备渠道轮次失败：%s",
			site.Id, channel.Id, sanitizePlatformMessage(requestErr.Error()))
		return model.InboxStatusFailed, channelInboxReasonLoadFailed
	}

	result, turnErr := runChannelTurnForInbox(site.AiSrv, ctx, req)
	if errors.Is(turnErr, ErrChannelTurnStale) {
		// 会话代次刚被改写：重读一次再试一次，仍失败按失败处理。
		req.Generation = c.sessionGeneration(ctx, &channel, binding, req.PlatformChatId)
		result, turnErr = runChannelTurnForInbox(site.AiSrv, ctx, req)
	}
	if turnErr != nil {
		log.Printf("[channel] 站点 %d 渠道 %d 渠道轮次未完成：%s",
			site.Id, channel.Id, channelInboxSafeErrorText(turnErr))
		reply := channelInboxErrorReply(turnErr)
		c.sendReply(ctx, inbox, binding, reply)
		if reason := channelInboxDeniedReason(turnErr); reason != "" {
			return model.InboxStatusDone, reason
		}
		if errors.Is(turnErr, ErrChannelTurnStale) {
			return model.InboxStatusFailed, ErrChannelTurnStale.Error()
		}
		return model.InboxStatusFailed, channelInboxReasonTurnFailed
	}

	c.sendReply(ctx, inbox, binding, result.Reply)
	return model.InboxStatusDone, ""
}

// turnRequest 按「渠道 + 绑定 + 入站任务」构造一次执行请求。
func (c *channelInboxConsumer) turnRequest(
	ctx context.Context,
	channel *model.MessageChannel,
	binding *model.ChannelBinding,
	inbox *model.ChannelInbox,
	group bool,
) (ChannelTurnRequest, error) {
	principal := ExecutionPrincipal{
		SiteId:    c.site.Id,
		AdminId:   binding.AdminId,
		BindingId: binding.Id,
		ChannelId: channel.Id,
		AgentId:   channel.AgentId,
		InGroup:   group,
	}
	request := ChannelTurnRequest{
		Principal:      principal,
		AgentId:        channel.AgentId,
		PlatformChatId: strings.TrimSpace(inbox.PlatformChatId),
		Text:           inbox.Content,
		Group:          group,
		Generation:     1,
	}
	if !group {
		request.Generation = c.sessionGeneration(ctx, channel, binding, request.PlatformChatId)
		return request, nil
	}
	// 群聊目标：只有存在且获得授权的行才把 TargetId 交给执行器；
	// 不存在就传 0，由 RunChannelTurn 统一按 ErrChannelTargetNotApproved 拒绝。
	var target model.ChannelTarget
	err := c.site.DB.WithContext(ctx).
		Where("`site_id` = ? AND `channel_id` = ? AND `kind` = ? AND `platform_target_id` = ?",
			c.site.Id, channel.Id, TargetKindGroup, strings.TrimSpace(inbox.PlatformChatId)).
		Take(&target).Error
	if err == nil {
		request.TargetId = target.Id
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ChannelTurnRequest{}, err
	}
	request.Generation = c.sessionGeneration(ctx, channel, binding, request.PlatformChatId)
	return request, nil
}

// sessionGeneration 读取当前会话代次；没有会话行时用 1（RunChannelTurn 的新会话口径）。
func (c *channelInboxConsumer) sessionGeneration(
	ctx context.Context,
	channel *model.MessageChannel,
	binding *model.ChannelBinding,
	platformChatId string,
) int64 {
	site := c.site
	if site == nil || site.DB == nil || channel == nil || binding == nil {
		return 1
	}
	userHash := PlatformIdentityHash(site.Id, channel.Id, channel.Platform, binding.OrgId, binding.PlatformUserId)
	sessionHash := channelDigest(
		strconv.FormatUint(uint64(site.Id), 10),
		strconv.FormatUint(uint64(channel.Id), 10),
		strings.TrimSpace(platformChatId),
		userHash,
	)
	var row model.ChannelSession
	if err := site.DB.WithContext(ctx).
		Where("`site_id` = ? AND `session_hash` = ?", site.Id, sessionHash).
		Take(&row).Error; err != nil {
		return 1
	}
	return row.Generation
}

// loadBinding 按发送人身份摘要查有效绑定；查不到返回 (nil, nil)。
func (c *channelInboxConsumer) loadBinding(ctx context.Context, identityHash string) (*model.ChannelBinding, error) {
	hash := strings.TrimSpace(identityHash)
	if hash == "" {
		return nil, nil
	}
	var binding model.ChannelBinding
	err := c.site.DB.WithContext(ctx).
		Where("`site_id` = ? AND `channel_id` = ? AND `identity_hash` = ? AND `revoked_at` = 0",
			c.site.Id, c.channelId, hash).
		Take(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// sendReply 把回执发到私聊（绑定管理员）或群（chat_id）。
//
// 发送失败只记脱敏日志，不改变任务的处理结果：模型/工具已经执行过，
// 因回执失败重放会产生第二次真实副作用。
func (c *channelInboxConsumer) sendReply(ctx context.Context, inbox *model.ChannelInbox, binding *model.ChannelBinding, text string) {
	text = strings.TrimSpace(text)
	if text == "" || c.adapter == nil || c.site == nil {
		return
	}
	target := TargetRef{Kind: TargetKindPrivate, PlatformTargetId: strings.TrimSpace(binding.PlatformUserId)}
	if strings.TrimSpace(inbox.ChatType) == ChatTypeGroup {
		target = TargetRef{Kind: TargetKindGroup, PlatformTargetId: strings.TrimSpace(inbox.PlatformChatId)}
	}
	if strings.TrimSpace(target.PlatformTargetId) == "" {
		log.Printf("[channel] 站点 %d 渠道 %d 缺少回复目标，回执未发送", c.site.Id, c.channelId)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, channelBindingReplyTimeout)
	defer cancel()
	if _, err := c.adapter.Send(sendCtx, target, OutboundMessage{Text: text}); err != nil {
		log.Printf("[channel] 站点 %d 渠道 %d 回执发送失败：%s",
			c.site.Id, c.channelId, sanitizePlatformMessage(err.Error()))
	}
}

// finishInbox 落最终状态与去敏原因；释放租约（lease_until = 0）。
//
// 写库必须脱离调用方的取消信号：消费者的 ctx 与连接租约同生命周期，adapter.Run
// 返回（关机/删站/失租）时会取消它。用已取消的 ctx 写库只会失败并留下
// status=processing + 未释放的租约，60 秒后 claimNext 重新认领、整轮重跑。
func (c *channelInboxConsumer) finishInbox(ctx context.Context, id uint, status, reason string) {
	if id == 0 || c.site == nil || c.site.DB == nil {
		return
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), channelInboxFinishTimeout)
	defer cancel()
	res := c.site.DB.WithContext(writeCtx).Model(&model.ChannelInbox{}).
		Where("`id` = ? AND `site_id` = ? AND `channel_id` = ?", id, c.site.Id, c.channelId).
		Updates(map[string]any{
			"status":      status,
			"last_error":  channelInboxReasonText(reason),
			"lease_until": 0,
		})
	if res.Error != nil {
		log.Printf("[channel] 站点 %d 渠道 %d 写入入站任务状态失败：%s",
			c.site.Id, c.channelId, sanitizePlatformMessage(res.Error.Error()))
	}
}

// channelInboxDeniedReason 把权限类哨兵映射成可留痕的原因文案。
// 返回空串表示不是「按规则拒绝」类错误（按处理失败处理）。
func channelInboxDeniedReason(err error) string {
	for _, sentinel := range []error{
		ErrChannelGroupRestricted,
		ErrChannelTargetNotApproved,
		ErrChannelSiteUnsupported,
		ErrChannelToolNotVisible,
		ErrChannelPrincipalInvalid,
		ErrChannelIntentUnknown,
		ErrChannelIntentNotExposed,
		ErrChannelAgentNotAllowed,
		ErrChannelAdminInvalid,
		ErrChannelDisabled,
		ErrChannelPermissionDenied,
	} {
		if errors.Is(err, sentinel) {
			return sentinel.Error()
		}
	}
	return ""
}

// channelInboxErrorReply 是给平台的安全回执：只回哨兵文案。
//
// 主体/管理员/群目标类拒绝只留日志、不回执：回执等于向未授权会话确认
// 「这里有机器人、你的请求被处理过」，而这些场景下连会话归属都还没证明。
func channelInboxErrorReply(err error) string {
	switch {
	case errors.Is(err, ErrChannelPrincipalInvalid),
		errors.Is(err, ErrChannelAdminInvalid),
		errors.Is(err, ErrChannelTargetNotApproved),
		errors.Is(err, ErrChannelTurnInvalid),
		errors.Is(err, ErrChannelTurnStale),
		errors.Is(err, ErrChannelSiteUnsupported):
		return ""
	}
	if errors.Is(err, ErrChannelTurnTruncated) {
		return ErrChannelTurnTruncated.Error()
	}
	return channelInboxDeniedReason(err)
}

// channelInboxSafeErrorText 是日志用文案：优先哨兵原文，避免把内部错误细节带进日志。
func channelInboxSafeErrorText(err error) string {
	if err == nil {
		return ""
	}
	if reason := channelInboxDeniedReason(err); reason != "" {
		return reason
	}
	if errors.Is(err, ErrChannelTurnTruncated) {
		return ErrChannelTurnTruncated.Error()
	}
	return channelInboxReasonText(err.Error())
}

// channelInboxReasonText 把原因压成单行、去敏、限长（varchar(255)）。
func channelInboxReasonText(reason string) string {
	text := strings.TrimSpace(reason)
	if text == "" {
		return ""
	}
	text = redactSecrets(sanitizePlatformMessage(text))
	if runes := []rune(text); len(runes) > channelInboxReasonLimit {
		text = string(runes[:channelInboxReasonLimit])
	}
	return text
}
