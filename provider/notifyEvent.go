package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"kandaoni.com/anqicms/model"
)

// 通知事件类型与状态。常量放在 provider，不塞进 model（T1 的模型只描述存储）。
const (
	NotifyEventGuestbookCreated = "guestbook.created"
	NotifyEventOrderCreated     = "order.created"
	NotifyEventOrderPaid        = "order.paid"
	NotifyEventAgentFailed      = "agent.failed"

	// 留言必须先过反垃圾分类：waiting_classification 期间绝不外发。
	NotifyEventStatusWaitingClassification = "waiting_classification"
	NotifyEventStatusReady                 = "ready"
	NotifyEventStatusCancelled             = "cancelled"
)

// notifyEventKeyHash 是 EventKey 的 SHA-256 小写十六进制摘要，与 ComputeDeliveryHash 同风格。
func notifyEventKeyHash(eventKey string) string {
	sum := sha256.Sum256([]byte(eventKey))
	return hex.EncodeToString(sum[:])
}

// notificationTableExists 探测通知表是否已迁移。
//
// 包级变量是为了可测：老库升级窗口里通知表可能不存在，用真实开发库 DROP 表来复现
// 既危险又不可并行，测试改为注入这个探测。
var notificationTableExists = func(db *gorm.DB, model any) bool {
	return db.Migrator().HasTable(model)
}

// notificationTableNoted 记录本进程已提示过的缺失通知表名。
// 升级窗口里每个下单/留言请求都会走到探测，不节流会把日志刷爆。
var notificationTableNoted sync.Map

// ensureNotificationTable 确认通知表存在；缺失时每进程每表只记一次日志。
//
// 缺表绝不能升级成业务失败：通知是旁路，表没迁移好不应该让下单/留言回滚
// （旧实现里 INSERT 报错会触发调用方 tx.Rollback，等于通知把业务带下水）。
// 日志只写表名，不含事件载荷或用户内容。
func ensureNotificationTable(db *gorm.DB, table any, name string) bool {
	if notificationTableExists(db, table) {
		return true
	}
	if _, noted := notificationTableNoted.LoadOrStore(name, struct{}{}); !noted {
		log.Printf("[notify] 通知表 %s 尚未迁移，已跳过通知写入（业务主线不受影响），请到后台执行「修复数据库」", name)
	}
	return false
}

// emitNotificationEvent 是通知事件的唯一写入口。
//
// 事务边界（R68）：事件与业务写同一个 tx；数据库错误必须返回给调用方，让业务事务回滚。
// 通知侧的非数据库问题（目标未批准、模板渲染失败、规则 JSON 损坏）在扇出内部消化，
// 只跳过或落 failed 投递，不影响业务结果。
//
// ⚠ 唯一例外是「通知表还没迁移」：这时返回 (nil, nil)，让业务事务照常提交。
// 缺表不是数据错误，而是部署时序问题；把它升级成业务失败会让升级窗口里的下单/留言全部失败。
//
// status == ready 时在同一 tx 内立即扇出；waiting_classification 由
// ProcessGuestbookNotification / worker 恢复后再扇出。
func (w *Website) emitNotificationEvent(tx *gorm.DB, eventType, businessKey string, occurredAt int64, payload map[string]any, status string) (*model.NotificationEvent, error) {
	if w == nil || tx == nil {
		return nil, errors.New("通知事件缺少数据库事务")
	}
	if !ensureNotificationTable(tx, &model.NotificationEvent{}, "notification_events") {
		return nil, nil
	}
	if occurredAt <= 0 {
		occurredAt = time.Now().Unix()
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化通知事件载荷失败: %w", err)
	}
	eventKey := eventType + ":" + businessKey
	event := &model.NotificationEvent{
		SiteId:       w.Id,
		EventType:    eventType,
		EventKey:     eventKey,
		OccurredAt:   occurredAt,
		PayloadJson:  string(payloadJSON),
		Status:       status,
		EventKeyHash: notifyEventKeyHash(eventKey),
	}
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(event)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		// 唯一键命中：读回已有行返回，重复回调不算错误。
		if err := tx.Where("site_id = ? AND event_key_hash = ?", w.Id, event.EventKeyHash).Take(event).Error; err != nil {
			return nil, err
		}
		return event, nil
	}
	if status == NotifyEventStatusReady {
		if err := w.fanOutNotificationEvent(tx, event); err != nil {
			return nil, err
		}
	}
	return event, nil
}

// orderNotificationPayload 生成订单事件的载荷白名单（R69/R70）。
// amount 单位为分（int64）；不写 items/IP/Refer，减少群聊泄漏面。
func orderNotificationPayload(order *model.Order, address *model.OrderAddress, status string) map[string]any {
	payload := map[string]any{
		"order_id":     order.OrderId,
		"amount":       order.Amount,
		"order_type":   order.Type,
		"status":       status,
		"created_time": order.CreatedTime,
	}
	if status == "paid" {
		payload["paid_time"] = order.PaidTime
	}
	if address != nil {
		payload["buyer_name"] = strings.TrimSpace(address.Name + " " + address.LastName)
		payload["buyer_phone"] = address.Phone
		payload["buyer_email"] = address.Email
		payload["buyer_address"] = singleLineOrderAddress(address)
	}
	return payload
}

// singleLineOrderAddress 把地址压成单行摘要（换行/制表符压成空格）。
func singleLineOrderAddress(address *model.OrderAddress) string {
	parts := []string{
		address.Province, address.City, address.Town,
		address.AddressInfo, address.Company, address.Postcode,
	}
	var kept []string
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	joined := strings.Join(kept, " ")
	return strings.Join(strings.Fields(joined), " ")
}

// guestbookNotificationPayload 生成留言事件载荷（R71）。
//
// 只放 5 个白名单字段：guestbook_id / user_name / contact / content / created_time。
// 刻意不写 IP、Refer 与 ExtraData（自定义字段可能有任意内容与结构性标识）：
// 事件载荷会随投递进群，网络元数据与未知结构字段一律不进事件表。
func guestbookNotificationPayload(guestbook *model.Guestbook) map[string]any {
	return map[string]any{
		"guestbook_id": guestbook.Id,
		"user_name":    guestbook.UserName,
		"contact":      guestbook.Contact,
		"content":      guestbook.Content,
		"created_time": guestbook.CreatedTime,
	}
}

// SaveGuestbookWithNotification 是两处留言保存入口（controller/guestbook.go 与
// controller/apiTag.go）的共享实现：留言行与 waiting_classification 事件同事务写入。
// 数据库故障会回滚留言并返回错误，与原 Save 行为一致。
func (w *Website) SaveGuestbookWithNotification(guestbook *model.Guestbook) error {
	if w == nil || w.DB == nil {
		return errors.New("数据库未就绪")
	}
	if guestbook == nil {
		return errors.New("留言不能为空")
	}
	tx := w.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := tx.Save(guestbook).Error; err != nil {
		tx.Rollback()
		return err
	}
	businessKey := strconv.FormatUint(uint64(guestbook.Id), 10)
	if _, err := w.emitNotificationEvent(tx, NotifyEventGuestbookCreated, businessKey, guestbook.CreatedTime, guestbookNotificationPayload(guestbook), NotifyEventStatusWaitingClassification); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// ProcessGuestbookNotification 根据反垃圾三态推进留言事件（R71）。
// outcome == Failed 时保持 waiting_classification 并记录静态原因，绝不外发；
// spamStatus == 2 取消；其余转 ready 并在同事务内扇出。重复调用幂等。
func (w *Website) ProcessGuestbookNotification(gb *model.Guestbook, spamStatus int, outcome AkismetOutcome) {
	if w == nil || w.DB == nil || gb == nil || gb.Id == 0 {
		return
	}
	businessKey := strconv.FormatUint(uint64(gb.Id), 10)
	eventKey := NotifyEventGuestbookCreated + ":" + businessKey
	tx := w.DB.Begin()
	if tx.Error != nil {
		log.Printf("[notify] 站点 %d 留言 %d 推进事件失败：数据库事务不可用", w.Id, gb.Id)
		return
	}
	var event model.NotificationEvent
	if err := tx.Where("site_id = ? AND event_key_hash = ?", w.Id, notifyEventKeyHash(eventKey)).Take(&event).Error; err != nil {
		tx.Rollback()
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[notify] 站点 %d 留言 %d 读取事件失败：%s", w.Id, gb.Id, sanitizePlatformMessage(err.Error()))
		}
		return
	}

	switch {
	case outcome == AkismetOutcomeFailed:
		// 检测没完成：保持待分类；只补空值，不覆盖已有原因。
		if event.LastError == "" {
			if err := tx.Model(&model.NotificationEvent{}).Where("id = ?", event.Id).
				Update("last_error", "垃圾内容检测未完成，暂不外发").Error; err != nil {
				tx.Rollback()
				log.Printf("[notify] 站点 %d 留言事件 %d 记录分类失败原因失败：%s", w.Id, event.Id, sanitizePlatformMessage(err.Error()))
				return
			}
		}
		if err := tx.Commit().Error; err != nil {
			log.Printf("[notify] 站点 %d 留言事件 %d 提交失败：%s", w.Id, event.Id, sanitizePlatformMessage(err.Error()))
		}
		return
	case spamStatus == 2:
		if err := tx.Model(&model.NotificationEvent{}).Where("id = ?", event.Id).Updates(map[string]any{
			"status":     NotifyEventStatusCancelled,
			"last_error": "垃圾留言",
		}).Error; err != nil {
			tx.Rollback()
			log.Printf("[notify] 站点 %d 留言事件 %d 取消失败：%s", w.Id, event.Id, sanitizePlatformMessage(err.Error()))
			return
		}
		if err := tx.Commit().Error; err != nil {
			log.Printf("[notify] 站点 %d 留言事件 %d 提交失败：%s", w.Id, event.Id, sanitizePlatformMessage(err.Error()))
		}
		return
	}

	if event.Status == NotifyEventStatusReady {
		// 已就绪（goroutine 重复执行/worker 已恢复）：不重复扇出。
		tx.Rollback()
		return
	}
	if event.Status == NotifyEventStatusCancelled {
		tx.Rollback()
		return
	}
	// 转 ready 必须是条件更新（镜像 notifyWorker.markNotificationEventReady）：
	// 从上面的读到这里的写之间，事件可能已被 controller 的另一个 goroutine 或 worker 恢复推进。
	// 只按 id 更新会让双方都继续扇出——uk_delivery_hash 只是兜住了重复行，
	// 「谁是赢家」变得不可观测，最后一个提交的事务还会覆盖对方的 last_error。
	res := tx.Model(&model.NotificationEvent{}).
		Where("id = ? AND status = ?", event.Id, NotifyEventStatusWaitingClassification).
		Updates(map[string]any{"status": NotifyEventStatusReady, "last_error": ""})
	if res.Error != nil {
		tx.Rollback()
		log.Printf("[notify] 站点 %d 留言事件 %d 转 ready 失败：%s", w.Id, event.Id, sanitizePlatformMessage(res.Error.Error()))
		return
	}
	if res.RowsAffected != 1 {
		// 已有赢家完成迁移：直接退出，不重复扇出。
		tx.Rollback()
		return
	}
	event.Status = NotifyEventStatusReady
	event.LastError = ""
	if err := w.fanOutNotificationEvent(tx, &event); err != nil {
		tx.Rollback()
		log.Printf("[notify] 站点 %d 留言事件 %d 扇出失败：%s", w.Id, event.Id, sanitizePlatformMessage(err.Error()))
		return
	}
	if err := tx.Commit().Error; err != nil {
		log.Printf("[notify] 站点 %d 留言事件 %d 提交失败：%s", w.Id, event.Id, sanitizePlatformMessage(err.Error()))
	}
}
