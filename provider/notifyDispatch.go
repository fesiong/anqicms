package provider

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"kandaoni.com/anqicms/model"
)

// 投递状态机（R74）。
const (
	NotifyDeliveryStatusPending   = "pending"
	NotifyDeliveryStatusSending   = "sending"
	NotifyDeliveryStatusSent      = "sent"
	NotifyDeliveryStatusFailed    = "failed"
	NotifyDeliveryStatusRetry     = "retry"
	NotifyDeliveryStatusCancelled = "cancelled"
	NotifyDeliveryStatusUnknown   = "unknown"

	// 最多 6 次尝试；第 6 次失败直接 failed。
	notifyDeliveryMaxAttempts = 6
	// 发送租约：领取后 30 秒内有效，过期由清扫标记 unknown。
	notifyDeliveryLeaseTTL = 30 * time.Second
	// 发送超时：单次平台调用最长 15 秒。
	notifyDeliverySendTimeout = 15 * time.Second
)

// notifyDeliveryBackoff 是第 1..5 次失败后的退避：10s/30s/2m/10m/30m。
var notifyDeliveryBackoff = []time.Duration{
	10 * time.Second,
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
}

// fanOutNotificationEvent 把 ready 事件展开成逐目标投递（R73）。
//
// 渲染在扇出时完成一次并写入投递行；重试不再渲染，模板事后修改不影响历史投递。
// 规则 JSON 损坏、目标/渠道失效都只跳过或落 failed 投递，不让业务事务失败；
// 只有数据库错误返回给调用方。
func (w *Website) fanOutNotificationEvent(tx *gorm.DB, event *model.NotificationEvent) error {
	if w == nil || tx == nil || event == nil {
		return nil
	}
	if event.Status != NotifyEventStatusReady {
		return nil
	}

	var rules []model.NotificationRule
	if err := tx.Where("site_id = ? AND event_type = ? AND enabled = ?", event.SiteId, event.EventType, 1).
		Order("id asc").Find(&rules).Error; err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}

	siteName := w.System.SiteName
	siteURL := w.System.BaseUrl
	now := time.Now().Unix()

	for _, rule := range rules {
		if strings.TrimSpace(rule.TargetIds) == "" {
			continue
		}
		var targetIds []uint
		if err := json.Unmarshal([]byte(rule.TargetIds), &targetIds); err != nil {
			// 规则损坏：记规则 ID 即可，日志里不能出现事件载荷。
			log.Printf("[notify] 站点 %d 通知规则 %d 的目标配置不是合法 JSON 数组，已跳过", w.Id, rule.Id)
			continue
		}

		template := DefaultNotificationTemplate(event.EventType)
		if rule.TemplateId > 0 {
			var custom model.NotificationTemplate
			err := tx.Where("id = ? AND site_id = ?", rule.TemplateId, event.SiteId).Take(&custom).Error
			switch {
			case err == nil:
				template = &custom
			case errors.Is(err, gorm.ErrRecordNotFound):
				// 模板行不存在：回落默认模板（R72）。
			default:
				return err
			}
		}

		for _, targetId := range targetIds {
			var target model.ChannelTarget
			if err := tx.Where("id = ? AND site_id = ?", targetId, event.SiteId).Take(&target).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			if target.Approved != 1 {
				continue
			}
			var channel model.MessageChannel
			if err := tx.Where("id = ? AND site_id = ?", target.ChannelId, event.SiteId).Take(&channel).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			if channel.Enabled != 1 {
				continue
			}

			subject, content, renderErr := RenderNotificationText(
				template, channel.Platform, event.EventType, event.PayloadJson, target.Kind, siteName, siteURL,
			)
			delivery := &model.NotificationDelivery{
				SiteId:      event.SiteId,
				EventId:     event.Id,
				RuleId:      rule.Id,
				TargetId:    target.Id,
				Status:      NotifyDeliveryStatusPending,
				Attempts:    0,
				Subject:     truncateNotificationSubject(subject),
				Content:     truncateNotificationContent(content),
				UpdatedTime: now,
			}
			delivery.CreatedTime = now
			if renderErr != nil {
				// 渲染失败也要可观测：落 failed 投递，不静默丢。
				delivery.Status = NotifyDeliveryStatusFailed
				delivery.Subject = ""
				delivery.Content = ""
				delivery.LastError = "通知模板渲染失败"
			} else {
				delivery.NextAttemptAt = now
			}
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(delivery)
			if res.Error != nil {
				return res.Error
			}
		}
	}
	return nil
}

// claimNotificationDelivery 用条件 UPDATE 抢占租约，RowsAffected == 1 才是唯一赢家。
func (w *Website) claimNotificationDelivery(delivery *model.NotificationDelivery, now time.Time, leaseTTL time.Duration) (bool, error) {
	if leaseTTL <= 0 {
		leaseTTL = notifyDeliveryLeaseTTL
	}
	res := w.DB.Model(&model.NotificationDelivery{}).
		Where("id = ? AND status IN ? AND next_attempt_at <= ? AND lease_until <= ?",
			delivery.Id,
			[]string{NotifyDeliveryStatusPending, NotifyDeliveryStatusRetry},
			now.Unix(), now.Unix()).
		Updates(map[string]any{
			"status":       NotifyDeliveryStatusSending,
			"lease_until":  now.Add(leaseTTL).Unix(),
			"attempts":     gorm.Expr("attempts + 1"),
			"updated_time": now.Unix(),
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// notificationDeliveryCancelReason 在发送前逐项复核（R74）。
// 返回非空静态原因表示应取消投递；返回 error 表示数据库读取失败（本轮跳过）。
func (w *Website) notificationDeliveryCancelReason(delivery *model.NotificationDelivery) (string, error) {
	var event model.NotificationEvent
	if err := w.DB.Where("id = ? AND site_id = ?", delivery.EventId, w.Id).Take(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "事件不存在", nil
		}
		return "", err
	}
	if event.Status != NotifyEventStatusReady {
		return "事件已失效", nil
	}

	var rule model.NotificationRule
	if err := w.DB.Where("id = ? AND site_id = ?", delivery.RuleId, w.Id).Take(&rule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "规则不存在", nil
		}
		return "", err
	}
	if rule.Enabled != 1 {
		return "规则已停用", nil
	}

	var target model.ChannelTarget
	if err := w.DB.Where("id = ? AND site_id = ?", delivery.TargetId, w.Id).Take(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "目标不存在", nil
		}
		return "", err
	}
	if target.Approved != 1 {
		return "目标未批准", nil
	}
	if target.Kind == TargetKindPrivate {
		var binding model.ChannelBinding
		if err := w.DB.Where("id = ? AND site_id = ?", target.BindingId, w.Id).Take(&binding).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "绑定不存在", nil
			}
			return "", err
		}
		if binding.RevokedAt != 0 {
			return "绑定已撤销", nil
		}
		var admin model.Admin
		if err := w.DB.Where("id = ?", binding.AdminId).Take(&admin).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "管理员不存在", nil
			}
			return "", err
		}
		if admin.Status != 1 {
			return "管理员已停用", nil
		}
	}

	var channel model.MessageChannel
	if err := w.DB.Where("id = ? AND site_id = ?", target.ChannelId, w.Id).Take(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "渠道不存在", nil
		}
		return "", err
	}
	if channel.Enabled != 1 {
		return "渠道已停用", nil
	}
	if strings.TrimSpace(delivery.Content) == "" {
		return "通知内容为空", nil
	}
	return "", nil
}

// processNotificationDelivery 领取并发送一条投递。单条失败只记日志，不向调用方扩散。
func (w *Website) processNotificationDelivery(ctx context.Context, deliveryId uint, opts notifyWorkerOptions) {
	var delivery model.NotificationDelivery
	if err := w.DB.Where("id = ? AND site_id = ?", deliveryId, w.Id).Take(&delivery).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[notify] 站点 %d 读取投递 %d 失败：%s", w.Id, deliveryId, sanitizePlatformMessage(err.Error()))
		}
		return
	}
	now := opts.now()
	claimed, err := w.claimNotificationDelivery(&delivery, now, opts.leaseTTL)
	if err != nil {
		log.Printf("[notify] 站点 %d 领取投递 %d 失败：%s", w.Id, delivery.Id, sanitizePlatformMessage(err.Error()))
		return
	}
	if !claimed {
		return
	}
	delivery.Status = NotifyDeliveryStatusSending
	delivery.Attempts++

	reason, checkErr := w.notificationDeliveryCancelReason(&delivery)
	if checkErr != nil {
		log.Printf("[notify] 站点 %d 复核投递 %d 失败：%s", w.Id, delivery.Id, sanitizePlatformMessage(checkErr.Error()))
		// 复核失败（数据库抖动）：放回 retry 交给下一轮，不消耗额外尝试。
		w.updateNotificationDelivery(delivery.Id, map[string]any{
			"status":          NotifyDeliveryStatusRetry,
			"lease_until":     0,
			"next_attempt_at": now.Unix(),
			"attempts":        gorm.Expr("attempts - 1"),
		})
		return
	}
	if reason != "" {
		w.updateNotificationDelivery(delivery.Id, map[string]any{
			"status":      NotifyDeliveryStatusCancelled,
			"lease_until": 0,
			"last_error":  reason,
		})
		return
	}

	var target model.ChannelTarget
	if err := w.DB.Where("id = ? AND site_id = ?", delivery.TargetId, w.Id).Take(&target).Error; err != nil {
		w.markNotificationDeliveryFailed(delivery.Id, NewSendError(ErrorClassPermanent, "目标不存在"))
		return
	}
	var channel model.MessageChannel
	if err := w.DB.Where("id = ? AND site_id = ?", target.ChannelId, w.Id).Take(&channel).Error; err != nil {
		w.markNotificationDeliveryFailed(delivery.Id, NewSendError(ErrorClassPermanent, "渠道不存在"))
		return
	}

	creds := PlatformCredentials{
		AppId:          channel.AppId,
		AppSecret:      channel.AppSecret,
		OrganizationId: channel.OrgId,
	}
	adapter, supported := opts.newAdapter(channel.Platform, creds)
	if !supported {
		w.markNotificationDeliveryFailed(delivery.Id, NewSendError(ErrorClassPermanent, "不支持的平台类型"))
		return
	}

	sendTimeout := opts.sendTimeout
	if sendTimeout <= 0 {
		sendTimeout = notifyDeliverySendTimeout
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	result, sendErr := adapter.Send(sendCtx, TargetRef{
		Kind:             target.Kind,
		PlatformTargetId: target.PlatformTargetId,
	}, OutboundMessage{Text: delivery.Content})
	if sendErr == nil {
		w.updateNotificationDelivery(delivery.Id, map[string]any{
			"status":              NotifyDeliveryStatusSent,
			"lease_until":         0,
			"platform_message_id": truncatePlatformMessageId(result.PlatformMessageId),
			"last_error":          "",
		})
		return
	}

	classified := ClassifyPlatformSendError(sendErr)
	if classified == nil {
		classified = NewSendError(ErrorClassTransient, "发送失败")
	}
	lastError := string(classified.Class) + ": " + sanitizePlatformMessage(classified.Message)
	if classified.Class == ErrorClassPermanent {
		w.updateNotificationDelivery(delivery.Id, map[string]any{
			"status":      NotifyDeliveryStatusFailed,
			"lease_until": 0,
			"last_error":  lastError,
		})
		return
	}
	if delivery.Attempts >= notifyDeliveryMaxAttempts {
		w.updateNotificationDelivery(delivery.Id, map[string]any{
			"status":      NotifyDeliveryStatusFailed,
			"lease_until": 0,
			"last_error":  lastError,
		})
		return
	}
	delay := notifyDeliveryBackoff[delivery.Attempts-1]
	if retryAfter := time.Duration(result.RetryAfterSeconds) * time.Second; retryAfter > delay {
		delay = retryAfter
	}
	w.updateNotificationDelivery(delivery.Id, map[string]any{
		"status":          NotifyDeliveryStatusRetry,
		"lease_until":     0,
		"next_attempt_at": now.Add(delay).Unix(),
		"last_error":      lastError,
	})
}

func (w *Website) markNotificationDeliveryFailed(deliveryId uint, sendErr *SendError) {
	lastError := string(sendErr.Class) + ": " + sanitizePlatformMessage(sendErr.Message)
	w.updateNotificationDelivery(deliveryId, map[string]any{
		"status":      NotifyDeliveryStatusFailed,
		"lease_until": 0,
		"last_error":  lastError,
	})
}

func (w *Website) updateNotificationDelivery(deliveryId uint, values map[string]any) {
	if err := w.DB.Model(&model.NotificationDelivery{}).Where("id = ?", deliveryId).Updates(values).Error; err != nil {
		log.Printf("[notify] 站点 %d 更新投递 %d 失败：%s", w.Id, deliveryId, sanitizePlatformMessage(err.Error()))
	}
}

// truncatePlatformMessageId 防止超长回执写爆 varchar(128)。
func truncatePlatformMessageId(messageId string) string {
	runes := []rune(messageId)
	if len(runes) <= 128 {
		return messageId
	}
	return string(runes[:128])
}
