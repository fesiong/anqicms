package provider

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
)

// 一次性绑定令牌的固定参数。
const (
	// bindingTokenTTLSeconds 是令牌有效期：创建时间 + 3600 秒（规格 §6）。
	bindingTokenTTLSeconds = 3600
	// bindingTokenBytes 是随机明文的字节数；编码后为 43 个 URL 安全字符。
	bindingTokenBytes = 32
	// channelDigestSeparator 与 model.ComputeAppIdentityHash 保持同一拼接协议
	// （ASCII Unit Separator）。可打印分隔符会因字段自身包含分隔符而撞摘要。
	channelDigestSeparator = "\x1f"
	// channelTargetKindPrivate 是私聊目标的 kind 取值（群聊为 group，由人工配置）。
	channelTargetKindPrivate = "private"
)

// ChannelPlatformIdentity 是一次绑定请求里来自平台的已验证身份。
// IsGroup 为 true 表示来源是群聊：绑定只允许在应用私聊里完成。
type ChannelPlatformIdentity struct {
	ChannelId      uint
	OrganizationId string
	AppId          string
	UserId         string
	IsGroup        bool
}

// hashBindingToken 返回令牌明文的 SHA-256 小写十六进制。
// 不做 TrimSpace：明文是 base64url 定长串，任何空白都意味着调用方传错了内容。
func hashBindingToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// channelDigest 按「逐段 TrimSpace → \x1f 连接 → SHA-256 → 小写 hex」口径计算摘要，
// 与 model.ComputeAppIdentityHash 一致（R12）。整数段由调用方先格式化成十进制。
func channelDigest(parts ...string) string {
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, channelDigestSeparator)))
	return hex.EncodeToString(sum[:])
}

// bindingIdentityDigest 是「站点 + 渠道 + 组织 + 平台用户」的身份摘要，
// 写入 ChannelBinding.IdentityHash（唯一索引）。渠道进摘要意味着同一个平台用户
// 在不同渠道里是两条互不相干的绑定，身份不会被跨渠道复用。
func bindingIdentityDigest(siteId, channelId uint, organizationId, userId string) string {
	return channelDigest(
		strconv.FormatUint(uint64(siteId), 10),
		strconv.FormatUint(uint64(channelId), 10),
		organizationId,
		userId,
	)
}

// bindingTargetDigest 是「站点 + 渠道 + 目标类型 + 平台目标」的目标摘要，
// 写入 ChannelTarget.TargetHash（唯一索引）。
func bindingTargetDigest(siteId, channelId uint, kind, platformTargetId string) string {
	return channelDigest(
		strconv.FormatUint(uint64(siteId), 10),
		strconv.FormatUint(uint64(channelId), 10),
		kind,
		platformTargetId,
	)
}

// CreateBindingToken 为「某渠道 + 某管理员」生成一次性绑定令牌。
//
// 返回的明文只在本次响应里出现一次：库里只存它的 SHA-256，并且所有错误信息都不含明文。
// 生成前把同一管理员同一渠道的未消费令牌置为过期（R9）：用 expires_at 失效，
// 不复用 consumed_at —— 后者的语义是「被消费过」，混用会让审计与去重判断全部失真。
func (w *Website) CreateBindingToken(channelId, adminId uint) (plain string, expiresAt int64, err error) {
	if w.DB == nil {
		return "", 0, errors.New("数据库未就绪")
	}

	var channel model.MessageChannel
	if err = w.DB.Where("id = ?", channelId).Take(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", 0, errors.New("渠道不存在")
		}
		// 原始 DB 报错可能带唯一索引里的摘要或其它内部标识，一律换成静态串。
		return "", 0, errors.New("读取渠道失败")
	}
	if channel.SiteId != w.Id {
		return "", 0, errors.New("渠道不存在")
	}

	// 管理员有效状态沿用后台登录规则（Status == 1），不猜其它数值（R14/R15）。
	var admin model.Admin
	if err = w.DB.Where("id = ?", adminId).Take(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", 0, errors.New("管理员不存在或已被禁用")
		}
		return "", 0, errors.New("读取管理员失败")
	}
	if admin.Status != 1 {
		return "", 0, errors.New("管理员不存在或已被禁用")
	}

	raw := make([]byte, bindingTokenBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", 0, errors.New("生成绑定令牌失败")
	}
	plain = base64.RawURLEncoding.EncodeToString(raw)

	now := time.Now().Unix()
	expiresAt = now + bindingTokenTTLSeconds
	token := &model.ChannelBindingToken{
		SiteId:    w.Id,
		ChannelId: channelId,
		AdminId:   adminId,
		TokenHash: hashBindingToken(plain),
		ExpiresAt: expiresAt,
	}
	// 失效旧令牌与写入新令牌放在同一事务：否则插入失败会把管理员手上唯一可用的令牌也废掉。
	err = w.DB.Transaction(func(tx *gorm.DB) error {
		if txErr := tx.Model(&model.ChannelBindingToken{}).
			Where("admin_id = ? AND channel_id = ? AND consumed_at = 0", adminId, channelId).
			Update("expires_at", now).Error; txErr != nil {
			return txErr
		}
		return tx.Create(token).Error
	})
	if err != nil {
		return "", 0, errors.New("保存绑定令牌失败")
	}

	return plain, expiresAt, nil
}

// ConsumeBindingToken 消费一个绑定令牌，创建平台身份到管理员的绑定与私聊目标。
//
// 事务内顺序（R16）：条件更新令牌（消费即失效）→ 校验站点/渠道/应用/组织 →
// 冲突判定（R13）→ 创建写入。任何一步失败都回滚，令牌不会被白白吃掉。
func (w *Website) ConsumeBindingToken(plain string, ident ChannelPlatformIdentity) (*model.ChannelBinding, error) {
	if w.DB == nil {
		return nil, errors.New("数据库未就绪")
	}
	if ident.IsGroup {
		return nil, errors.New("仅支持在私聊中完成绑定")
	}
	plain = strings.TrimSpace(plain)
	if plain == "" {
		return nil, errors.New("绑定令牌不能为空")
	}
	userId := strings.TrimSpace(ident.UserId)
	if userId == "" {
		return nil, errors.New("平台用户标识不能为空")
	}
	// 平台回调与后台表单常带首尾空白；摘要口径按 TrimSpace 归一化（R12），
	// 比对也必须用同一份归一化后的值，否则同一个应用会被判成两个。
	orgId := strings.TrimSpace(ident.OrganizationId)
	appId := strings.TrimSpace(ident.AppId)
	tokenHash := hashBindingToken(plain)

	var binding *model.ChannelBinding
	// 闭包内的失败要带「给调用方看的静态文案」，而 Transaction 还会原样返回
	// Begin/Commit/回滚层的驱动原始错误（如「Deadlock found…」、连接重置），
	// 两者在 error 值上无法区分。闭包内一律走 fail() 登记，出口据此分流：
	// 只有经 fail() 产生的错误允许直出，其余一律换成静态串。
	var txFailure error
	fail := func(msg string) error {
		txFailure = errors.New(msg)
		return txFailure
	}
	err := w.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		// 条件更新是并发消费的唯一闸门：只有一个事务能让 RowsAffected == 1。
		res := tx.Model(&model.ChannelBindingToken{}).
			Where("token_hash = ? AND consumed_at = 0 AND expires_at > ?", tokenHash, now).
			Update("consumed_at", now)
		if res.Error != nil {
			return fail("消费绑定令牌失败")
		}
		if res.RowsAffected != 1 {
			return fail("绑定令牌无效或已过期")
		}

		var token model.ChannelBindingToken
		if txErr := tx.Where("token_hash = ?", tokenHash).Take(&token).Error; txErr != nil {
			return fail("绑定令牌无效或已过期")
		}
		if token.SiteId != w.Id {
			return fail("绑定令牌不属于当前站点")
		}
		if token.ChannelId != ident.ChannelId {
			return fail("绑定令牌与当前渠道不匹配")
		}

		// 令牌有效不等于生成者现在仍可用：TTL 有一小时，期间管理员可能已被停用。
		// 生成时（CreateBindingToken）与撤销时都只认 Status == 1，消费侧必须同口径，
		// 否则已停用管理员的私聊目标会以 approved=1 落库，T8 之后仍会继续投递。
		// 放在事务内是为了让失败连带回滚令牌消费，管理员恢复后同一令牌仍可用。
		// 用 tx 读 admins 表而不是另开连接：读的是与已锁 token 行无交集的另一张表，
		// 不会自锁，但不必为一个校验再占一条连接（并发消费时少一轮连接争用）。
		var tokenAdmin model.Admin
		if txErr := tx.Where("id = ?", token.AdminId).Take(&tokenAdmin).Error; txErr != nil {
			return fail("管理员不存在或已被禁用")
		}
		if tokenAdmin.Status != 1 {
			return fail("管理员不存在或已被禁用")
		}

		var channel model.MessageChannel
		if txErr := tx.Where("id = ?", token.ChannelId).Take(&channel).Error; txErr != nil {
			return fail("渠道不存在")
		}
		if channel.SiteId != w.Id {
			return fail("渠道不存在")
		}
		if strings.TrimSpace(channel.AppId) != appId {
			return fail("绑定令牌与当前平台应用不匹配")
		}
		if strings.TrimSpace(channel.OrgId) != orgId {
			return fail("绑定令牌与当前组织不匹配")
		}

		identityHash := bindingIdentityDigest(w.Id, channel.Id, orgId, userId)

		// R13：同一平台身份已有有效绑定 → 拒绝；绝不覆盖。
		var sameIdentity int64
		if txErr := tx.Model(&model.ChannelBinding{}).
			Where("identity_hash = ? AND revoked_at = 0", identityHash).
			Count(&sameIdentity).Error; txErr != nil {
			return fail("读取已有绑定失败")
		}
		if sameIdentity > 0 {
			return fail("该平台身份已绑定，请先解绑后再试")
		}

		// R13：同一管理员在同一渠道已有有效绑定 → 拒绝并提示先解绑；绝不覆盖。
		var sameAdmin int64
		if txErr := tx.Model(&model.ChannelBinding{}).
			Where("site_id = ? AND channel_id = ? AND admin_id = ? AND revoked_at = 0", w.Id, channel.Id, token.AdminId).
			Count(&sameAdmin).Error; txErr != nil {
			return fail("读取已有绑定失败")
		}
		if sameAdmin > 0 {
			return fail("该管理员在当前渠道已有绑定，请先解绑后再试")
		}

		// uk_identity_hash 覆盖已撤销的历史行，所以「撤销后换绑」不能直接插新行，
		// 只能复活原行。仅当原行已撤销时才走到这里，有效绑定在上面的判定里已被拒绝。
		var existing model.ChannelBinding
		findErr := tx.Where("identity_hash = ?", identityHash).Take(&existing).Error
		switch {
		case findErr == nil:
			res := tx.Model(&model.ChannelBinding{}).
				Where("id = ? AND revoked_at != 0", existing.Id).
				Updates(map[string]interface{}{
					"admin_id":         token.AdminId,
					"org_id":           orgId,
					"platform_user_id": userId,
					"revoked_at":       0,
					"revision":         gorm.Expr("revision + 1"),
				})
			if res.Error != nil {
				return fail("保存绑定失败")
			}
			if res.RowsAffected != 1 {
				// 并发下被别人抢先恢复成了有效绑定：按冲突处理，绝不覆盖。
				return fail("该平台身份已绑定，请先解绑后再试")
			}
			existing.AdminId = token.AdminId
			existing.OrgId = orgId
			existing.PlatformUserId = userId
			existing.RevokedAt = 0
			// revision 由 SQL 自增，结构体不会自动跟随；不补这一步调用方拿到的是旧值，
			// 按 revision 做乐观锁的后续写入会立刻撞失败。
			existing.Revision++
			binding = &existing
		case errors.Is(findErr, gorm.ErrRecordNotFound):
			binding = &model.ChannelBinding{
				SiteId:         w.Id,
				ChannelId:      channel.Id,
				OrgId:          orgId,
				PlatformUserId: userId,
				AdminId:        token.AdminId,
				IdentityHash:   identityHash,
				Revision:       1,
			}
			if txErr := tx.Create(binding).Error; txErr != nil {
				return fail("保存绑定失败")
			}
		default:
			return fail("读取已有绑定失败")
		}

		// 私聊目标同样有唯一摘要：撤销后重新绑定同一个人会撞上旧行，按摘要复用并重新批准。
		targetHash := bindingTargetDigest(w.Id, channel.Id, channelTargetKindPrivate, userId)
		var target model.ChannelTarget
		targetErr := tx.Where("target_hash = ?", targetHash).Take(&target).Error
		switch {
		case targetErr == nil:
			if txErr := tx.Model(&model.ChannelTarget{}).
				Where("id = ?", target.Id).
				Updates(map[string]interface{}{
					"binding_id": binding.Id,
					"approved":   1,
				}).Error; txErr != nil {
				return fail("保存私聊目标失败")
			}
		case errors.Is(targetErr, gorm.ErrRecordNotFound):
			target = model.ChannelTarget{
				SiteId:           w.Id,
				ChannelId:        channel.Id,
				Kind:             channelTargetKindPrivate,
				PlatformTargetId: userId,
				BindingId:        binding.Id,
				Approved:         1,
				TargetHash:       targetHash,
			}
			if txErr := tx.Create(&target).Error; txErr != nil {
				return fail("保存私聊目标失败")
			}
		default:
			return fail("读取已有私聊目标失败")
		}

		return nil
	})
	if err != nil {
		// 闭包经 fail() 产生的错误是给调用方看的静态串，原样返回；
		// 其余（Begin/Commit/回滚层的驱动错误）一律换成静态串，不透出原文。
		if txFailure != nil {
			return nil, txFailure
		}
		return nil, errors.New("消费绑定令牌失败")
	}

	return binding, nil
}

// RevokeBinding 撤销一条绑定：标记绑定、失效其未执行审批与排队任务、收回私聊目标授权。
//
// 幂等：已经撤销过的绑定直接返回 nil。绑定不存在或不属于本站点、操作者不存在
// 或已停用都报错（R14）——「已经撤销」与「根本不存在」必须是两种结果，
// 否则调用方无法区分误操作与重复点击。
func (w *Website) RevokeBinding(bindingId, operatorAdminId uint) error {
	if w.DB == nil {
		return errors.New("数据库未就绪")
	}

	// 操作者有效状态沿用后台登录规则（Status == 1）。
	var operator model.Admin
	if err := w.DB.Where("id = ?", operatorAdminId).Take(&operator).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("操作者不存在或已被禁用")
		}
		return errors.New("读取操作者失败")
	}
	if operator.Status != 1 {
		return errors.New("操作者不存在或已被禁用")
	}

	var binding model.ChannelBinding
	if err := w.DB.Where("id = ?", bindingId).Take(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("绑定不存在")
		}
		return errors.New("读取绑定失败")
	}
	if binding.SiteId != w.Id {
		return errors.New("绑定不存在")
	}
	if binding.RevokedAt != 0 {
		return nil
	}

	return w.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()
		// 条件更新兜住并发撤销：另一个事务先撤销时 RowsAffected == 0，按幂等成功处理。
		res := tx.Model(&model.ChannelBinding{}).
			Where("id = ? AND site_id = ? AND revoked_at = 0", binding.Id, w.Id).
			Updates(map[string]interface{}{
				"revoked_at": now,
				"revision":   gorm.Expr("revision + 1"),
			})
		if res.Error != nil {
			return errors.New("撤销绑定失败")
		}
		if res.RowsAffected == 0 {
			return nil
		}

		// 未执行审批失效：pending 尚未确认，approved 已确认但还没开始执行，
		// 绑定身份变化后都必须重新确认（model.ChannelApproval 注释）。
		if err := tx.Model(&model.ChannelApproval{}).
			Where("binding_id = ? AND status IN ?", binding.Id, []string{model.ApprovalStatusPending, model.ApprovalStatusApproved}).
			Update("status", model.ApprovalStatusCancelled).Error; err != nil {
			return errors.New("失效未执行审批失败")
		}

		// 排队中的入站任务失效。executing 等已开跑的状态不在这里处理，由 T4 的租约兜底。
		if err := tx.Model(&model.ChannelInbox{}).
			Where("binding_id = ? AND status = ?", binding.Id, model.InboxStatusPending).
			Update("status", model.InboxStatusCancelled).Error; err != nil {
			return errors.New("失效排队任务失败")
		}

		// 规格 §4：私聊目标来自有效绑定。绑定撤销后它不得再被当作已批准目标投递；
		// 重新绑定时 ConsumeBindingToken 会把 approved 置回 1。
		if err := tx.Model(&model.ChannelTarget{}).
			Where("binding_id = ? AND kind = ?", binding.Id, channelTargetKindPrivate).
			Update("approved", 0).Error; err != nil {
			return errors.New("收回私聊目标失败")
		}

		return nil
	})
}
