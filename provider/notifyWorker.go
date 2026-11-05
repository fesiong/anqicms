package provider

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"kandaoni.com/anqicms/model"
)

// 通知投递工作者的运行默认值（R76）。生产每 30 秒跑一轮，单轮最多领取 20 条投递。
const (
	notifyWorkerInterval      = 30 * time.Second
	notifyWorkerLeaseTTL      = 30 * time.Second
	notifyWorkerSendTimeout   = 15 * time.Second
	notifyWorkerBatch         = 20
	notifyWorkerShutdownWait  = 5 * time.Second
	notifyRecoverWaitingLimit = 50
	notifyDeriveAuditLimit    = 50
	// notifyDeriveAuditWindow 限定 agent.failed 只从「最近失败」的审计派生。
	// 不加时间窗会每轮重扫开发库里的历史失败审计：它们早已派生过（插入是 no-op），
	// 但会把历史运行反复算进本轮派生结果，排查时无法区分「刚失败」与「旧账」。
	notifyDeriveAuditWindow = 10 * time.Minute
)

// notifyWorkerOptions 是通知工作者的全部可注入项。
// newAdapter 默认指向 NewPlatformAdapter；测试注入假适配器，只替代平台网络。
type notifyWorkerOptions struct {
	newAdapter  func(name string, creds PlatformCredentials) (PlatformAdapter, bool)
	now         func() time.Time
	interval    time.Duration
	leaseTTL    time.Duration
	sendTimeout time.Duration
	batch       int
}

func defaultNotifyWorkerOptions() notifyWorkerOptions {
	return notifyWorkerOptions{
		newAdapter:  NewPlatformAdapter,
		now:         time.Now,
		interval:    notifyWorkerInterval,
		leaseTTL:    notifyWorkerLeaseTTL,
		sendTimeout: notifyWorkerSendTimeout,
		batch:       notifyWorkerBatch,
	}
}

func (o notifyWorkerOptions) withDefaults() notifyWorkerOptions {
	defaults := defaultNotifyWorkerOptions()
	if o.newAdapter == nil {
		o.newAdapter = defaults.newAdapter
	}
	if o.now == nil {
		o.now = defaults.now
	}
	if o.interval <= 0 {
		o.interval = defaults.interval
	}
	if o.leaseTTL <= 0 {
		o.leaseTTL = defaults.leaseTTL
	}
	if o.sendTimeout <= 0 {
		o.sendTimeout = defaults.sendTimeout
	}
	if o.batch <= 0 {
		o.batch = defaults.batch
	}
	return o
}

// notifyCycleResult 是一轮工作的统计，仅用于日志/测试观察。
type notifyCycleResult struct {
	SweptUnknown int
	Recovered    int
	Derived      int
	Claimed      int
	Sent         int
	Retried      int
	Failed       int
	Cancelled    int
}

// notifyWorkerRegistry 是「每站点一个通知工作者」的登记处。
// 与 channelManagerRegistry 同理：Website 会被按值拷贝，状态放包级表才不会被复制失效。
var notifyWorkerRegistry = struct {
	sync.Mutex
	bySite map[uint]*notifyWorker
}{bySite: make(map[uint]*notifyWorker)}

// notifyWorkerOnInit 控制「站点初始化时是否自动启动通知工作者」。
// 默认关闭：测试/命令行会频繁初始化站点，不能因为库里有数据就自动跑后台投递。
var notifyWorkerOnInit atomic.Bool

// EnableNotifyWorkerOnInit 打开「站点初始化时启动通知工作者」的开关。
// 只应由进程入口（bootstrap.Serve）在 InitWebsites 之前调用。
func EnableNotifyWorkerOnInit() {
	notifyWorkerOnInit.Store(true)
}

// startNotifyWorkerOnInit 是站点初始化时启动工作者的唯一入口；测试可替换。
var startNotifyWorkerOnInit = func(w *Website) {
	w.StartNotifyWorker(context.Background())
}

// StartNotifyWorker 为当前站点启动通知投递工作者（幂等）。
// ctx 只能是进程级上下文；传入请求级 ctx 会在请求结束时取消后台投递。
func (w *Website) StartNotifyWorker(ctx context.Context) {
	w.startNotifyWorker(ctx, defaultNotifyWorkerOptions())
}

// StopNotifyWorker 取消运行上下文并等待退出（上限 5 秒）。
func (w *Website) StopNotifyWorker() {
	w.stopNotifyWorker()
}

func (w *Website) startNotifyWorker(parent context.Context, opts notifyWorkerOptions) {
	if w == nil || w.DB == nil {
		return
	}
	notifyWorkerRegistry.Lock()
	if existing, exists := notifyWorkerRegistry.bySite[w.Id]; exists {
		if existing.servesSite(w) {
			notifyWorkerRegistry.Unlock()
			return
		}
		delete(notifyWorkerRegistry.bySite, w.Id)
		notifyWorkerRegistry.Unlock()
		log.Printf("[notify] 站点 %d 的通知工作者绑定了旧的站点/数据库句柄，先停止旧实例再重建", w.Id)
		existing.stop()
		notifyWorkerRegistry.Lock()
	}
	worker := newNotifyWorker(w, opts)
	notifyWorkerRegistry.bySite[w.Id] = worker
	notifyWorkerRegistry.Unlock()
	worker.start(parent)
}

func (w *Website) stopNotifyWorker() {
	notifyWorkerRegistry.Lock()
	worker := notifyWorkerRegistry.bySite[w.Id]
	delete(notifyWorkerRegistry.bySite, w.Id)
	notifyWorkerRegistry.Unlock()
	if worker != nil {
		worker.stop()
	}
}

type notifyWorker struct {
	site *Website
	opts notifyWorkerOptions

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newNotifyWorker(w *Website, opts notifyWorkerOptions) *notifyWorker {
	return &notifyWorker{site: w, opts: opts.withDefaults()}
}

func (worker *notifyWorker) servesSite(w *Website) bool {
	return worker.site == w && worker.site.DB == w.DB
}

func (worker *notifyWorker) start(parent context.Context) {
	if parent == nil {
		parent = context.Background()
	}
	worker.ctx, worker.cancel = context.WithCancel(parent)
	worker.wg.Add(1)
	go func() {
		defer worker.wg.Done()
		ticker := time.NewTicker(worker.opts.interval)
		defer ticker.Stop()
		worker.site.runNotifyCycle(worker.ctx, worker.opts)
		for {
			select {
			case <-worker.ctx.Done():
				return
			case <-ticker.C:
				worker.site.runNotifyCycle(worker.ctx, worker.opts)
			}
		}
	}()
}

func (worker *notifyWorker) stop() {
	if worker.cancel == nil {
		return
	}
	worker.cancel()
	done := make(chan struct{})
	go func() {
		worker.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(notifyWorkerShutdownWait):
		log.Printf("[notify] 站点 %d 通知工作者停止超时（%s）", worker.site.Id, notifyWorkerShutdownWait)
	}
}

// runNotifyCycle 执行一轮：清扫过期租约 → 恢复待分类留言 → 派生 agent.failed → 领取并发送。
// 单条失败只记日志，不中断整轮。
func (w *Website) runNotifyCycle(ctx context.Context, opts notifyWorkerOptions) notifyCycleResult {
	result := notifyCycleResult{}
	if w == nil || w.DB == nil {
		return result
	}
	opts = opts.withDefaults()
	if ctx == nil {
		ctx = context.Background()
	}
	// 老库可能还没迁移通知表：不阻断站点，轮次直接跳过。
	if !w.DB.Migrator().HasTable(&model.NotificationEvent{}) || !w.DB.Migrator().HasTable(&model.NotificationDelivery{}) {
		return result
	}

	result.SweptUnknown = w.sweepStaleNotificationDeliveriesAt(opts.now().Unix())
	result.Recovered = w.recoverWaitingNotificationEvents()
	result.Derived = w.deriveAgentFailedNotificationEvents()

	var deliveries []model.NotificationDelivery
	err := w.DB.Where(
		"site_id = ? AND status IN ? AND next_attempt_at <= ? AND lease_until <= ?",
		w.Id,
		[]string{NotifyDeliveryStatusPending, NotifyDeliveryStatusRetry},
		opts.now().Unix(), opts.now().Unix(),
	).Order("id asc").Limit(opts.batch).Find(&deliveries).Error
	if err != nil {
		log.Printf("[notify] 站点 %d 读取待投递任务失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return result
	}
	for _, delivery := range deliveries {
		if ctx.Err() != nil {
			return result
		}
		before := w.notificationDeliveryStatus(delivery.Id)
		w.processNotificationDelivery(ctx, delivery.Id, opts)
		after := w.notificationDeliveryStatus(delivery.Id)
		if after == before && after != NotifyDeliveryStatusSending {
			continue
		}
		result.Claimed++
		switch after {
		case NotifyDeliveryStatusSent:
			result.Sent++
		case NotifyDeliveryStatusRetry:
			result.Retried++
		case NotifyDeliveryStatusFailed:
			result.Failed++
		case NotifyDeliveryStatusCancelled:
			result.Cancelled++
		}
	}
	return result
}

func (w *Website) notificationDeliveryStatus(deliveryId uint) string {
	var status string
	if err := w.DB.Model(&model.NotificationDelivery{}).Where("id = ?", deliveryId).Pluck("status", &status).Error; err != nil {
		return ""
	}
	return status
}

// sweepStaleNotificationDeliveriesAt 是 sweepStaleNotificationDeliveries 的计数版本。
func (w *Website) sweepStaleNotificationDeliveriesAt(nowUnix int64) int {
	res := w.DB.Model(&model.NotificationDelivery{}).
		Where("site_id = ? AND status = ? AND lease_until < ?", w.Id, NotifyDeliveryStatusSending, nowUnix).
		Updates(map[string]any{
			"status":      NotifyDeliveryStatusUnknown,
			"lease_until": 0,
			"last_error":  "发送结果未知（租约过期），人工重试可能重复",
		})
	if res.Error != nil {
		log.Printf("[notify] 站点 %d 清扫过期发送租约失败：%s", w.Id, sanitizePlatformMessage(res.Error.Error()))
		return 0
	}
	return int(res.RowsAffected)
}

// recoverWaitingNotificationEvents 恢复 goroutine 丢失/进程重启后留下的待分类留言事件（R71）。
// 不重发网络请求、不复用请求对象：只按 guestbook_id 读留言当前状态再收敛。
func (w *Website) recoverWaitingNotificationEvents() int {
	var events []model.NotificationEvent
	if err := w.DB.Where("site_id = ? AND status = ?", w.Id, NotifyEventStatusWaitingClassification).
		Order("id asc").Limit(notifyRecoverWaitingLimit).Find(&events).Error; err != nil {
		log.Printf("[notify] 站点 %d 读取待分类事件失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return 0
	}
	recovered := 0
	for _, event := range events {
		guestbookId := notificationEventUintField(&event, "guestbook_id")
		if guestbookId == 0 {
			continue
		}
		var guestbook model.Guestbook
		if err := w.DB.Where("id = ?", guestbookId).Take(&guestbook).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("[notify] 站点 %d 读取留言 %d 失败：%s", w.Id, guestbookId, sanitizePlatformMessage(err.Error()))
			}
			continue
		}
		switch {
		case guestbook.Status == 1:
			if w.markNotificationEventReady(&event) {
				recovered++
			}
		case guestbook.Status == 2:
			w.cancelNotificationEvent(&event, "垃圾留言")
		case !w.AkismetCheckEnabled(CheckTypeGuestbook):
			if w.markNotificationEventReady(&event) {
				recovered++
			}
		default:
			if event.LastError == "" {
				w.DB.Model(&model.NotificationEvent{}).Where("id = ?", event.Id).
					Update("last_error", "垃圾内容检测未完成，暂不外发")
			}
		}
	}
	return recovered
}

// markNotificationEventReady 把 waiting_classification 事件转 ready 并在同事务内扇出。
// 条件更新保证多实例/goroutine 并发时只有一个赢家。
func (w *Website) markNotificationEventReady(event *model.NotificationEvent) bool {
	tx := w.DB.Begin()
	if tx.Error != nil {
		return false
	}
	res := tx.Model(&model.NotificationEvent{}).
		Where("id = ? AND status = ?", event.Id, NotifyEventStatusWaitingClassification).
		Updates(map[string]any{"status": NotifyEventStatusReady, "last_error": ""})
	if res.Error != nil || res.RowsAffected != 1 {
		tx.Rollback()
		return false
	}
	event.Status = NotifyEventStatusReady
	event.LastError = ""
	if err := w.fanOutNotificationEvent(tx, event); err != nil {
		tx.Rollback()
		log.Printf("[notify] 站点 %d 恢复事件 %d 时扇出失败：%s", w.Id, event.Id, sanitizePlatformMessage(err.Error()))
		return false
	}
	if err := tx.Commit().Error; err != nil {
		log.Printf("[notify] 站点 %d 恢复事件 %d 提交失败：%s", w.Id, event.Id, sanitizePlatformMessage(err.Error()))
		return false
	}
	return true
}

func (w *Website) cancelNotificationEvent(event *model.NotificationEvent, reason string) {
	if err := w.DB.Model(&model.NotificationEvent{}).
		Where("id = ? AND status = ?", event.Id, NotifyEventStatusWaitingClassification).
		Updates(map[string]any{"status": NotifyEventStatusCancelled, "last_error": reason}).Error; err != nil {
		log.Printf("[notify] 站点 %d 取消事件 %d 失败：%s", w.Id, event.Id, sanitizePlatformMessage(err.Error()))
	}
}

func notificationEventUintField(event *model.NotificationEvent, key string) uint {
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.PayloadJson), &payload); err != nil {
		return 0
	}
	text := notificationString(payload[key])
	parsed, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0
	}
	return uint(parsed)
}

// deriveAgentFailedNotificationEvents 从 channel_audits 派生 agent.failed 事件（R75）。
//
// ⚠ 这是对规格「运行 ID」的近似：T6 没有持久化运行 ID，channel_audits.result='failed'
// 的审计 ID 是当前唯一稳定的失败身份。一次运行里多个工具失败会产生多条通知（可接受）。
// 不扫描 denied：那是权限拒绝，不是任务失败。
// 只取最近 notifyDeriveAuditWindow 内失败的审计：历史失败早已派生过，重放没有意义。
// 只取 binding_id > 0：管理员试发（channel.test）与后台操作失败的审计没有绑定身份，
// 不属于「智能体任务失败」，派生成 agent.failed 会往群里发无关噪音。
// payload 只放标识与动作名，绝不含参数正文、错误原文或用户消息。
func (w *Website) deriveAgentFailedNotificationEvents() int {
	if !w.DB.Migrator().HasTable(&model.ChannelAudit{}) {
		return 0
	}
	cutoff := time.Now().Unix() - int64(notifyDeriveAuditWindow/time.Second)
	var audits []model.ChannelAudit
	if err := w.DB.Where("site_id = ? AND result = ? AND binding_id > 0 AND created_time >= ?", w.Id, "failed", cutoff).
		Order("id desc").Limit(notifyDeriveAuditLimit).Find(&audits).Error; err != nil {
		log.Printf("[notify] 站点 %d 读取失败审计失败：%s", w.Id, sanitizePlatformMessage(err.Error()))
		return 0
	}
	derived := 0
	for _, audit := range audits {
		payload := map[string]any{
			"audit_id":     audit.Id,
			"action":       audit.Action,
			"channel_id":   audit.ChannelId,
			"target_hash":  audit.TargetHash,
			"created_time": audit.CreatedTime,
		}
		businessKey := "audit:" + strconv.FormatUint(uint64(audit.Id), 10)
		// 计数只认「真的插入了新行」。不能用插入语句的 RowsAffected 判断：
		// GORM 把 OnConflict DoNothing 编译成 `ON DUPLICATE KEY UPDATE id=id RETURNING id`，
		// MariaDB 支持 RETURNING，命中已存在行时同样返回 1 行，RowsAffected 恒为 1
		// （MariaDB 11.7 实测）。所以先读后写：已有同键事件直接跳过，不计入本轮派生。
		var existing int64
		if err := w.DB.Model(&model.NotificationEvent{}).
			Where("site_id = ? AND event_key_hash = ?", w.Id, notifyEventKeyHash(NotifyEventAgentFailed+":"+businessKey)).
			Count(&existing).Error; err != nil {
			log.Printf("[notify] 站点 %d 查询审计 %d 的既有事件失败：%s", w.Id, audit.Id, sanitizePlatformMessage(err.Error()))
			continue
		}
		if existing > 0 {
			continue
		}
		tx := w.DB.Begin()
		if tx.Error != nil {
			continue
		}
		if _, err := w.emitNotificationEvent(tx, NotifyEventAgentFailed, businessKey, audit.CreatedTime, payload, NotifyEventStatusReady); err != nil {
			tx.Rollback()
			log.Printf("[notify] 站点 %d 派生审计 %d 的失败事件失败：%s", w.Id, audit.Id, sanitizePlatformMessage(err.Error()))
			continue
		}
		if err := tx.Commit().Error; err != nil {
			log.Printf("[notify] 站点 %d 提交审计 %d 的失败事件失败：%s", w.Id, audit.Id, sanitizePlatformMessage(err.Error()))
			continue
		}
		derived++
	}
	return derived
}
