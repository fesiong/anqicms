package provider

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"kandaoni.com/anqicms/model"
)

// channelApproval.go 是渠道聊天的「持久化一次性审批」（规格 §8）。
//
// 职责边界：
//   - 本文件只负责审批行本身的状态机与落库自证，不执行工具、不调用模型、不调平台、
//     不写审计、不写进程级缓存。
//   - 「这次调用到底允不允许」由 channelPermissions.go 的 checkIntentPermission 判定；
//     审批只能证明「管理员批过同一件事」，不能扩大权限交集（规格 §7）。
//   - 执行器（T6）只按 public_id 消费审批，不自己解释审批表。
//
// 状态机（只允许这些迁移，其余一律拒绝）：
//
//	pending  → approved | rejected | expired | cancelled
//	approved → executing | cancelled
//	executing → succeeded | failed | unknown
//
// 三个「为什么」值得写下来：
//  1. 摘要 + 参数正文双写。ArgsHash 是身份，CanonicalArgs 是复核依据；只有摘要没有正文
//     就无法在执行前重算（数据库被改参数时看不出来），只有正文没有摘要就会让每次比对
//     依赖序列化实现的偶然一致。两者必须同时对得上。
//  2. 消费用条件更新（status='approved' AND expires_at > now → executing），不用
//     「先查后写」：后者在并发双击下两个请求都会读到 approved，只是很窗口小。
//     RowsAffected 必须等于 1，只有赢家能继续执行。
//  3. 收尾失败只记日志。调用已经发生，把「写结果摘要失败」报成调用失败会诱导调用方重试，
//     而重试就是真实的第二次副作用。

const (
	// channelApprovalDefaultTTL 是确认的默认有效期（规格 §8：10 分钟）。
	channelApprovalDefaultTTL = 10 * time.Minute
	// channelApprovalResultSummaryLimit 是执行结果摘要的字符上限（R60）。
	channelApprovalResultSummaryLimit = 200
	// channelApprovalPublicIdBytes 是公开确认 ID 的随机字节数（R59）。
	channelApprovalPublicIdBytes = 16
)

// CreateApproval 落一条待确认的审批（R59）。
//
// 入口先做执行主体校验（主体完整性 → 渠道可管理 → 绑定有效 + 管理员启用），
// 任一失败原样返回 channelPrincipal.go 的哨兵，调用方用 errors.Is 分流。
//
// 参数归一：空串 / 全空白 / 字面 null 一律当成 {}（零参意图），随后用
// channelCanonicalArgs 规范化，摘要用 ChannelArgsHash 计算——**不许另写序列化**：
// 执行侧（T6）与自证侧（T5）用的是这一套，换一套就会出现「确认成功、执行永远对不上」。
//
// Action 从规范化参数原样推导，解析不出就是空串，绝不回落 "*"：
// "*" 是 ChannelPolicy.Decision 内部的策略兜底语义，不是审批单的身份字段。
func (w *Website) CreateApproval(p ExecutionPrincipal, toolName, argsJson string, ttl time.Duration) (*model.ChannelApproval, error) {
	if err := w.checkChannelPrincipal(p); err != nil {
		return nil, err
	}
	// 审批与其它渠道动作同一把门：渠道必须启用且开启聊天管理，否则「仅通知」渠道
	// 也能攒审批单，等人工点击后执行管理动作（规格 §6/§7）。
	ctx := context.Background()
	channel, err := w.channelForPrincipal(ctx, p)
	if err != nil {
		return nil, err
	}
	// 绑定仍有效 + 管理员启用。放在入口而不是只信 principal：principal 由调用方构造，
	// 而审批是「稍后才被点击」的对象，期间绑定可能已被撤销。
	if err := w.checkChannelAdmin(ctx, p, nil); err != nil {
		return nil, err
	}
	// 审批必须绑定确认时的策略修订号：策略一旦变化，这张单就得重新确认。
	if int64(channel.Revision) != p.PolicyRevision {
		return nil, fmt.Errorf("%w：渠道策略修订号 %d 与执行主体 %d 不一致", ErrChannelPolicyStale, channel.Revision, p.PolicyRevision)
	}

	session, err := w.channelSessionForApproval(ctx, channel.Id, p.SessionId)
	if err != nil {
		return nil, err
	}

	tool := strings.TrimSpace(toolName)
	if tool == "" {
		return nil, fmt.Errorf("%w：缺少工具名", ErrChannelApprovalInvalid)
	}

	canonicalArgs := channelApprovalCanonicalArgs(argsJson)
	argsHash := ChannelArgsHash(canonicalArgs)
	if argsHash == "" {
		// 空摘要绝不能落库：T6 的自证把「args_hash 为空」当拒绝，但一条空摘要记录
		// 在其它读路径上就是「没有任何参数约束」的万能审批。
		return nil, fmt.Errorf("%w：审批参数无法解析", ErrChannelApprovalInvalid)
	}

	if ttl <= 0 {
		ttl = channelApprovalDefaultTTL
	}
	now := time.Now()
	row := &model.ChannelApproval{
		SiteId:    w.Id,
		BindingId: p.BindingId,
		// SessionId 是模型里的 uint：必须落 channel_sessions.id，绝不落 0
		// （0 在点击时无法反查出会话，等于把审批挂在一个不存在的上下文上）。
		SessionId:      session.Id,
		Tool:           tool,
		Action:         actionOfArgsJSON(canonicalArgs),
		CanonicalArgs:  canonicalArgs,
		ArgsHash:       argsHash,
		ExpiresAt:      now.Unix() + int64(ttl.Seconds()),
		PolicyRevision: channel.Revision,
		Status:         model.ApprovalStatusPending,
	}

	// 公开 ID 是 16 字节随机值的 base64url。唯一索引冲突最多重试一次：
	// 16 字节随机碰撞实际不可能发生，但真撞上时重试一次即可，不能进循环。
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		publicId, gerr := newChannelApprovalPublicId()
		if gerr != nil {
			return nil, fmt.Errorf("生成审批标识失败：%w", gerr)
		}
		row.PublicId = publicId
		if cerr := w.DB.WithContext(ctx).Create(row).Error; cerr != nil {
			lastErr = cerr
			if !channelApprovalPublicIdConflict(cerr) {
				return nil, fmt.Errorf("创建审批失败：%w", cerr)
			}
			continue
		}
		return row, nil
	}
	return nil, fmt.Errorf("创建审批失败：%w", lastErr)
}

// DecideApproval 处理管理员的确认点击（R61）。
//
// decision 只接受 approve / reject（去空白 + 小写），其余一律 ErrChannelApprovalInvalid：
// 接受 once_allow / full_control 等于把一次点击放大成长期绕过。
//
// 两条路径的校验强度刻意不同：
//   - approve：除归属外还要重算参数摘要、比对策略修订号，并重跑完整意图门禁
//     （曝光 / 白名单 / 渠道策略其余取值 / 菜单权限 / 管理员状态）——规格 §8 要求
//     「权限仍允许且对象前置条件仍满足」。
//   - reject：只做归属校验（绑定存在且未撤销、渠道与会话存在、管理员有效）。
//     拒绝本身不产生任何执行，反而必须让管理员能撤掉一张「策略已经变了」的单；
//     若也要求修订号与参数一致，过期的单就只能等 10 分钟自然过期。
//
// 两个路径都用同一条条件更新消费 pending：绑定、public_id、未过期三者同时成立才生效，
// 并发双击只有一个赢家。
func (w *Website) DecideApproval(publicId string, bindingId uint, decision string) error {
	if w == nil || w.DB == nil {
		return fmt.Errorf("%w：站点未初始化", ErrChannelApprovalInvalid)
	}
	normalized := strings.ToLower(strings.TrimSpace(decision))
	if normalized != "approve" && normalized != "reject" {
		return fmt.Errorf("%w：未知的审批决策", ErrChannelApprovalInvalid)
	}
	id := strings.TrimSpace(publicId)
	if id == "" || bindingId == 0 {
		return fmt.Errorf("%w：缺少审批标识", ErrChannelApprovalInvalid)
	}

	ctx := context.Background()
	var approval model.ChannelApproval
	if err := w.DB.WithContext(ctx).
		Where("`site_id` = ? AND `public_id` = ? AND `binding_id` = ?", w.Id, id, bindingId).
		Take(&approval).Error; err != nil {
		// 不区分「不存在」与「不属于该绑定」：否则审批单号变成探测通道。
		return fmt.Errorf("%w：审批记录不存在或不属于该绑定", ErrChannelApprovalInvalid)
	}

	now := time.Now().Unix()
	if approval.ExpiresAt <= now {
		// 尽量把 pending 标成 expired；失败只记日志，返回值仍是「无效」。
		if uerr := w.DB.WithContext(ctx).Model(&model.ChannelApproval{}).
			Where("`id` = ? AND `site_id` = ? AND `status` = ?", approval.Id, w.Id, model.ApprovalStatusPending).
			Update("status", model.ApprovalStatusExpired).Error; uerr != nil {
			slog.Warn("标记审批过期失败", "siteId", w.Id, "approvalId", approval.Id, "error", uerr)
		}
		return fmt.Errorf("%w：审批已过期", ErrChannelApprovalInvalid)
	}

	p, channel, err := w.rebuildApprovalPrincipal(ctx, &approval)
	if err != nil {
		return err
	}

	target := model.ApprovalStatusApproved
	if normalized == "approve" {
		// 1) 策略修订号：确认必须绑定确认时的策略版本。
		if approval.PolicyRevision != channel.Revision {
			return fmt.Errorf("%w：审批策略修订号 %d 与当前 %d 不一致", ErrChannelPolicyStale, approval.PolicyRevision, channel.Revision)
		}
		// 2) 参数自证：正文与摘要必须同时对得上（数据库被改参数时摘要不再匹配）。
		argsHash := ChannelArgsHash(approval.CanonicalArgs)
		if argsHash == "" || argsHash != approval.ArgsHash {
			return fmt.Errorf("%w：审批参数与摘要不一致", ErrChannelApprovalInvalid)
		}
		// 3) 完整意图门禁（approved=true：跳过 confirm，但策略其余取值与修订号仍校验）。
		action := actionOfArgsJSON(approval.CanonicalArgs)
		if err := w.checkIntentPermission(ctx, p, approval.Tool, action, true); err != nil {
			return err
		}
	} else {
		target = model.ApprovalStatusRejected
		// 拒绝不执行任何东西，但仍必须先证明「你是这张单的主人」：
		// 否则任何登录用户按单号就能替别人拒绝。
		if err := w.checkChannelAdmin(ctx, p, nil); err != nil {
			return err
		}
	}

	res := w.DB.WithContext(ctx).Model(&model.ChannelApproval{}).
		Where("`site_id` = ? AND `public_id` = ? AND `binding_id` = ? AND `status` = ? AND `expires_at` > ?",
			w.Id, id, bindingId, model.ApprovalStatusPending, now).
		Update("status", target)
	if res.Error != nil {
		return fmt.Errorf("更新审批状态失败：%w", res.Error)
	}
	if res.RowsAffected != 1 {
		return fmt.Errorf("%w：审批已被处理或已过期", ErrChannelApprovalInvalid)
	}
	return nil
}

// MarkApprovalExecuting 消费一次已批准的审批（R60），是「重复点击不重复执行」的最后一道闸。
//
// 两步都必须做：
//  1. 读行并重算参数摘要——参数在批准之后被改过（脏行 / 迁移脚本）就不得执行；
//  2. 条件更新 approved + 未过期 → executing，RowsAffected != 1 即「已被消费 / 已过期 /
//     根本没批准」。只有赢家返回 nil，调用方据此决定是否真正执行。
func (w *Website) MarkApprovalExecuting(publicId string) error {
	id := strings.TrimSpace(publicId)
	if w == nil || w.DB == nil || id == "" {
		return fmt.Errorf("%w：缺少审批标识", ErrChannelApprovalInvalid)
	}
	ctx := context.Background()
	var approval model.ChannelApproval
	if err := w.DB.WithContext(ctx).
		Where("`site_id` = ? AND `public_id` = ?", w.Id, id).
		Take(&approval).Error; err != nil {
		return fmt.Errorf("%w：审批记录不存在", ErrChannelApprovalInvalid)
	}
	argsHash := ChannelArgsHash(approval.CanonicalArgs)
	if approval.ArgsHash == "" || argsHash == "" || argsHash != approval.ArgsHash {
		return fmt.Errorf("%w：审批参数与摘要不一致", ErrChannelApprovalInvalid)
	}
	res := w.DB.WithContext(ctx).Model(&model.ChannelApproval{}).
		Where("`site_id` = ? AND `public_id` = ? AND `status` = ? AND `expires_at` > ?",
			w.Id, id, model.ApprovalStatusApproved, time.Now().Unix()).
		Update("status", model.ApprovalStatusExecuting)
	if res.Error != nil {
		return fmt.Errorf("更新审批状态失败：%w", res.Error)
	}
	if res.RowsAffected != 1 {
		return fmt.Errorf("%w：审批已消费、已过期或未经批准", ErrChannelApprovalInvalid)
	}
	return nil
}

// CompleteApproval 把 executing 收尾为 succeeded 并写结果摘要（R60）。
func (w *Website) CompleteApproval(publicId, result string) error {
	return w.finishApproval(publicId, model.ApprovalStatusSucceeded, result)
}

// FailApproval 把 executing 收尾为 failed。
//
// 为什么计划里的四个方法表达不了失败态：CompleteApproval 的语义是「调用成功」，
// 拿它承载失败会把 failed 状态吞掉，后台与重启恢复都无法区分「执行成功」与
// 「执行报错」；而规格 §8 的状态机明确要求 executing → succeeded/failed/unknown 三态。
// 两个方法共用同一套条件更新与摘要口径，只有目标状态不同。
func (w *Website) FailApproval(publicId, result string) error {
	return w.finishApproval(publicId, model.ApprovalStatusFailed, result)
}

func (w *Website) finishApproval(publicId, status, result string) error {
	id := strings.TrimSpace(publicId)
	if w == nil || w.DB == nil || id == "" {
		return fmt.Errorf("%w：缺少审批标识", ErrChannelApprovalInvalid)
	}
	summary := channelApprovalResultSummary(result)
	res := w.DB.Model(&model.ChannelApproval{}).
		Where("`site_id` = ? AND `public_id` = ? AND `status` = ?", w.Id, id, model.ApprovalStatusExecuting).
		Updates(map[string]any{
			"status":         status,
			"result_summary": summary,
		})
	if res.Error != nil {
		return fmt.Errorf("更新审批状态失败：%w", res.Error)
	}
	if res.RowsAffected != 1 {
		return fmt.Errorf("%w：审批不在执行中", ErrChannelApprovalInvalid)
	}
	return nil
}

// recoverChannelApprovals 是启动恢复（R62）：只归档，不重放。
//
//   - pending / approved → cancelled：这两个状态都还没有产生任何副作用，
//     重启后保留它们只会在管理员不知情时被点击执行。
//   - executing → unknown：调用可能已经发出去了，结果不确定。绝不能自动重放写操作，
//     也不能假装成功；标记 unknown 提示到后台核查（规格 §8）。
//
// 表不存在直接返回 nil：老库与测试库可能还没迁移到渠道表，
// 启动缺表不能阻断站点初始化，也不能顺手对业务库做破坏性修复。
func (w *Website) recoverChannelApprovals() error {
	if w == nil || w.DB == nil {
		return nil
	}
	if !w.DB.Migrator().HasTable(&model.ChannelApproval{}) {
		return nil
	}
	// 两条条件更新天然幂等：重复执行时第二次的 RowsAffected 为 0，结果不变。
	if err := w.DB.Model(&model.ChannelApproval{}).
		Where("`site_id` = ? AND `status` IN ?", w.Id, []string{model.ApprovalStatusPending, model.ApprovalStatusApproved}).
		Update("status", model.ApprovalStatusCancelled).Error; err != nil {
		return fmt.Errorf("取消未执行审批失败：%w", err)
	}
	if err := w.DB.Model(&model.ChannelApproval{}).
		Where("`site_id` = ? AND `status` = ?", w.Id, model.ApprovalStatusExecuting).
		Update("status", model.ApprovalStatusUnknown).Error; err != nil {
		return fmt.Errorf("归档执行中审批失败：%w", err)
	}
	return nil
}

// rebuildApprovalPrincipal 由审批行反查执行主体（R61）。
//
// 为什么不把 principal 存进审批行：审批可能在很久之后才被点击，期间绑定、渠道配置、
// 会话代次都可能变化。重建一次才能让「权限仍允许」这句话有真实依据。
// 任一行缺失都返回 ErrChannelApprovalInvalid——不区分哪一行缺，避免把审批单号变成探测通道。
func (w *Website) rebuildApprovalPrincipal(ctx context.Context, approval *model.ChannelApproval) (ExecutionPrincipal, *model.MessageChannel, error) {
	invalid := fmt.Errorf("%w：审批关联的绑定、渠道或会话已失效", ErrChannelApprovalInvalid)

	var binding model.ChannelBinding
	if err := w.DB.WithContext(ctx).
		Where("`id` = ? AND `site_id` = ? AND `revoked_at` = 0", approval.BindingId, w.Id).
		Take(&binding).Error; err != nil {
		return ExecutionPrincipal{}, nil, invalid
	}
	var channel model.MessageChannel
	if err := w.DB.WithContext(ctx).
		Where("`id` = ? AND `site_id` = ?", binding.ChannelId, w.Id).
		Take(&channel).Error; err != nil {
		return ExecutionPrincipal{}, nil, invalid
	}
	var session model.ChannelSession
	if err := w.DB.WithContext(ctx).
		Where("`id` = ? AND `site_id` = ? AND `channel_id` = ?", approval.SessionId, w.Id, channel.Id).
		Take(&session).Error; err != nil {
		return ExecutionPrincipal{}, nil, invalid
	}

	p := ExecutionPrincipal{
		SiteId:         w.Id,
		AdminId:        binding.AdminId,
		BindingId:      binding.Id,
		ChannelId:      channel.Id,
		AgentId:        channel.AgentId,
		SessionId:      session.ChatSessionId,
		PolicyRevision: int64(channel.Revision),
	}
	return p, &channel, nil
}

// channelSessionForApproval 按「站点 + 渠道 + 内部 chat 会话字符串」取会话行。
//
// 查不到直接拒绝，绝不把 SessionId 落成 0：0 会让审批在点击时无法反查会话，
// 而「无法反查」在状态机里表现为一条永远点不动、也永远不进 unknown 的僵尸单。
func (w *Website) channelSessionForApproval(ctx context.Context, channelId uint, chatSessionId string) (*model.ChannelSession, error) {
	chatId := strings.TrimSpace(chatSessionId)
	if chatId == "" {
		return nil, fmt.Errorf("%w：缺少渠道会话标识", ErrChannelApprovalInvalid)
	}
	var session model.ChannelSession
	if err := w.DB.WithContext(ctx).
		Where("`site_id` = ? AND `channel_id` = ? AND `chat_session_id` = ?", w.Id, channelId, chatId).
		Take(&session).Error; err != nil {
		return nil, fmt.Errorf("%w：渠道会话不存在", ErrChannelApprovalInvalid)
	}
	return &session, nil
}

// channelApprovalCanonicalArgs 归一审批参数：空串 / 全空白 / 字面 null 一律当成 {}
// （零参意图的合法调用），其余交给 channelCanonicalArgs。
//
// 为什么 null 要特判：json.Unmarshal("null") 得到 nil，再 Marshal 回 "null"，
// 摘要算的是字符串 "null" 而不是"没有参数"。零参调用在 T6 侧规范化结果是 "{}"，
// 两边不一致就会表现为「确认了但执行时摘要对不上」。
func channelApprovalCanonicalArgs(argsJson string) string {
	raw := strings.TrimSpace(argsJson)
	if raw == "" || raw == "null" {
		return "{}"
	}
	return channelCanonicalArgs(raw)
}

// channelApprovalResultSummary 把执行结果压成单行摘要，最多 200 个字符（R60）。
//
// 用 rune 截断而不是字节：中文结果按字节截会把最后一个字劈成半个非法 UTF-8。
func channelApprovalResultSummary(result string) string {
	fields := strings.Fields(result)
	text := strings.Join(fields, " ")
	runes := []rune(text)
	if len(runes) > channelApprovalResultSummaryLimit {
		text = string(runes[:channelApprovalResultSummaryLimit])
	}
	return text
}

// newChannelApprovalPublicId 生成公开确认 ID：16 字节随机值，base64url 无填充，不加前缀。
func newChannelApprovalPublicId() (string, error) {
	buf := make([]byte, channelApprovalPublicIdBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// channelApprovalPublicIdConflict 判断 Create 的错误是否是公开 ID 唯一索引冲突。
//
// 不只看 gorm.ErrDuplicatedKey：该哨兵依赖驱动开启 TranslateError，
// 本仓库的 MySQL 连接没有开启，实测只能靠错误文本识别（"Duplicate entry"）。
// 只认 uk_public_id 相关的冲突，避免把别的唯一约束错误也当成「换个 ID 再试」。
func channelApprovalPublicIdConflict(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "duplicate entry") && strings.Contains(text, "public_id")
}
