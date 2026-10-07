package provider

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// SendMailResult 是一次群发的进度与结果。
//
// 为什么需要它：SendSubscriberMail 是异步执行的（端点立即返回 job_id），
// 而 SendMail 是同步 SMTP 调用、单封可达数秒 —— 1000 封即使异步也要跑几十分钟。
// 没有这份回执，调用方既不知道发到哪了，也不知道哪些失败了，只能干等。
//
// 进度查询走 send_status action（按 job_id 取）。
//
// 并发约定：mu 保护所有字段；对外一律通过 snapshotPtr() 拿副本，
// 禁止把内部指针或值直接传出（值拷贝会触发 "copies lock value"）。
type SendMailResult struct {
	mu        sync.Mutex
	JobID     string   `json:"job_id"`
	Type      string   `json:"type"`
	Total     int      `json:"total"`     // 目标收件人数
	Sent      int      `json:"sent"`      // 已成功
	Failed    int      `json:"failed"`    // 已失败
	Processed int      `json:"processed"` // 已处理 = Sent + Failed
	Running   bool     `json:"running"`
	Errors    []string `json:"errors,omitempty"`
	StartedAt int64    `json:"started_at"`
	EndedAt   int64    `json:"ended_at,omitempty"`

	// seq 单调递增序号，仅用于淘汰最旧任务（StartedAt 只到秒，同秒创建时无法区分）。
	// 不导出，JSON 里不出现。
	seq int
}

// AddError 记录一条失败原因，最多留 50 条避免结果对象无限膨胀。
func (r *SendMailResult) AddError(email, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.Errors) >= 50 {
		return
	}
	r.Errors = append(r.Errors, email+": "+reason)
}

// inc 累加计数。
//
// 必须经方法而非在调用点直接 `r.Sent++` —— 后者是**无锁写**，
// 与 HTTP 查询 goroutine 的读并发时会触发 data race（-race 实测抓到）。
// 顺带把 Sent/Failed/Processed 三处合成一次加锁，减少锁开销。
func (r *SendMailResult) inc(sentDelta, failedDelta int) {
	r.mu.Lock()
	r.Sent += sentDelta
	r.Failed += failedDelta
	r.Processed += sentDelta + failedDelta
	r.mu.Unlock()
}

// finish 标记任务结束。
func (r *SendMailResult) finish() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Running = false
	r.EndedAt = time.Now().Unix()
}

// snapshotPtr 返回当前进度的快照副本（加锁读取后另建对象）。
//
// 返回新对象而非内部指针有两个原因：
//  1. SendMailResult 内含 sync.Mutex，值拷贝会触发 go vet 的
//     "copies lock value" 告警；
//  2. 外部拿到内部指针可绕过锁改写进度。
func (r *SendMailResult) snapshotPtr() *SendMailResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := &SendMailResult{
		JobID:     r.JobID,
		Type:      r.Type,
		Total:     r.Total,
		Sent:      r.Sent,
		Failed:    r.Failed,
		Processed: r.Processed,
		Running:   r.Running,
		StartedAt: r.StartedAt,
		EndedAt:   r.EndedAt,
	}
	c.Errors = append([]string(nil), r.Errors...)
	return c
}

// sendMailJobs 存最近若干次群发的进度。
//
// 刻意放内存而非建表：群发是低频操作（管理员手动触发），进程重启后
// 历史进度丢失可接受——真要追溯可查 sendmail_logs 与 subscriber 的
// error_times/status，那两份是持久化的。
var sendMailJobs = struct {
	mu   sync.RWMutex
	seq  int
	jobs map[string]*SendMailResult
}{jobs: map[string]*SendMailResult{}}

// 保留最近 20 条，超出淘汰最旧的。
const sendMailJobKeep = 20

func newSendMailResult(jobType string, total int) *SendMailResult {
	sendMailJobs.mu.Lock()
	sendMailJobs.seq++
	id := fmt.Sprintf("%s-%d", time.Now().Format("20060102150405"), sendMailJobs.seq)
	r := &SendMailResult{
		JobID:     id,
		Type:      jobType,
		Total:     total,
		Running:   true,
		StartedAt: time.Now().Unix(),
		seq:       sendMailJobs.seq,
	}
	sendMailJobs.jobs[id] = r
	// 淘汰最旧的：按 seq（单调递增）排序，**不能用 StartedAt** ——
	// 它只精确到秒，同一秒内连续创建多个任务时比较不出新旧，会随机淘汰掉一个。
	if len(sendMailJobs.jobs) > sendMailJobKeep {
		oldestID, oldestSeq := "", 0
		for k, v := range sendMailJobs.jobs {
			if oldestSeq == 0 || v.seq < oldestSeq {
				oldestID, oldestSeq = k, v.seq
			}
		}
		delete(sendMailJobs.jobs, oldestID)
	}
	sendMailJobs.mu.Unlock()
	return r
}

// GetSendMailResult 导出版：供 controller 查群发进度。
func GetSendMailResult(jobID string) (*SendMailResult, bool) {
	sendMailJobs.mu.RLock()
	defer sendMailJobs.mu.RUnlock()
	r, ok := sendMailJobs.jobs[jobID]
	if !ok {
		return nil, false
	}
	// 返回快照而非内部指针：调用方在 controller 层把它塞进 JSON，
	// 直接暴露含 mutex 的结构体会触发 "copies lock value" 告警，
	// 而且外部拿到指针可能绕过锁改动进度。
	return r.snapshotPtr(), true
}


func (w *Website) GetSubscribers(categoryId int64, keyword string, currentPage, pageSize int) ([]*model.Subscriber, int64) {
	var subscribers []*model.Subscriber
	offset := (currentPage - 1) * pageSize
	var total int64
	builder := w.DB.Model(&model.Subscriber{}).Order("id desc")
	if categoryId > 0 {
		builder = builder.Where("`category_id` = ?", categoryId)
	}
	if keyword != "" {
		//模糊搜索
		builder = builder.Where("`email` like ?", keyword+"%")
	}
	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&subscribers).Error
	if err != nil {
		return nil, 0
	}

	return subscribers, total
}

func (w *Website) GetSubscriber(id int64) (*model.Subscriber, error) {
	var subscriber model.Subscriber
	err := w.DB.Where("id = ?", id).First(&subscriber).Error
	if err != nil {
		return nil, err
	}
	return &subscriber, nil
}

func (w *Website) GetSubscriberByEmail(email string) (*model.Subscriber, error) {
	var subscriber model.Subscriber
	err := w.DB.Where("email = ?", email).First(&subscriber).Error
	if err != nil {
		return nil, err
	}
	return &subscriber, nil
}

func (w *Website) DeleteSubscriber(subscriber *model.Subscriber) error {
	err := w.DB.Where("id = ?", subscriber.Id).Delete(&model.Subscriber{}).Error
	if err != nil {
		return err
	}
	// 更新 计数
	if subscriber.CategoryId > 0 {
		w.UpdateSubscriberCount(subscriber.CategoryId)
	}
	return nil
}

func (w *Website) SaveSubscriber(req *request.SubscriberRequest) (*model.Subscriber, error) {
	var err error
	var subscriber = &model.Subscriber{}
	if req.Id > 0 {
		subscriber, err = w.GetSubscriber(req.Id)
		if err != nil {
			return nil, err
		}
		// 判断是否有重名
		exist, err := w.GetSubscriberByEmail(req.Email)
		if err == nil && exist.Id != req.Id {
			return nil, errors.New(w.Tr("EmailAlreadyExists"))
		}
	} else {
		// 判断 email 是否已经存在
		_, err = w.GetSubscriberByEmail(req.Email)
		if err == nil {
			return nil, errors.New(w.Tr("EmailAlreadyExists"))
		}
	}
	oldCategoryId := subscriber.CategoryId
	subscriber.Email = req.Email
	subscriber.Remark = req.Remark
	subscriber.Status = req.Status
	subscriber.CategoryId = req.CategoryId
	// 判断是否已经注册用户
	user, err := w.GetUserInfoByEmail(req.Email)
	if err == nil {
		subscriber.UserId = int64(user.Id)
		// 同时更新用户自动
		w.DB.Model(&model.User{}).Where("email = ?", req.Email).UpdateColumn("subscribed", true)
	}

	err = w.DB.Save(subscriber).Error
	if err != nil {
		return nil, err
	}
	// 更新分类订阅者数量
	if subscriber.CategoryId > 0 {
		w.UpdateSubscriberCount(subscriber.CategoryId)
	}
	if oldCategoryId > 0 && oldCategoryId != req.CategoryId {
		w.UpdateSubscriberCount(oldCategoryId)
	}
	// 发送订阅通知
	w.SendNewSubscriberEmail(subscriber)

	return subscriber, nil
}

// SendSubscriberMail 执行群发，返回本次任务的 job_id（供 send_status 查进度）。
//
// 保持异步：端点 `go` 调用本方法后立即返回，而 SendMail 是同步 SMTP 调用，
// 单封可达数秒 —— 同步等待必然导致 API 超时。调用方用 job_id 轮询进度。
func (w *Website) SendSubscriberMail(req *request.SubscriberMailRequest) string {
	template, exist := w.GetEmailTemplateInfo("recommendation")
	if !exist {
		slog.Error("SendSubscriberMail 缺少 recommendation 邮件模板", "job_type", req.Type)
		return ""
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content != "" {
		template.Content = req.Content
	}
	req.Subject = strings.TrimSpace(req.Subject)
	if req.Subject != "" {
		template.Subject = req.Subject
	}
	// 提示正在验证, 并且不登录
	data := map[string]interface{}{
		"website": w,
		"user":    nil,
	}
	// 检查是否需要发送全部
	if req.Type == "all" || req.Type == "category" {
		// 先数出目标总数，让调用方一开始就知道这批要发多少封
		// （否则 progress 无从判断，也看不出是否有漏发）。
		var total int64
		cntTx := w.DB.Model(&model.Subscriber{}).Where("status = ?", 1)
		if req.CategoryId > 0 {
			cntTx = cntTx.Where("category_id = ?", req.CategoryId)
		}
		cntTx.Count(&total)
		result := newSendMailResult(req.Type, int(total))

		// 获取所有订阅者
		var lastId int64 = 0
		var limit = 1000
		for {
			var subscribers []*model.Subscriber
			// 1 = 有效的订阅者
			tx := w.DB.Where("status = ? and id > ?", 1, lastId).Omit("remark")
			if req.CategoryId > 0 {
				tx.Where("category_id = ?", req.CategoryId)
			}
			tx.Limit(limit).Find(&subscribers)
			if len(subscribers) == 0 {
				break
			}
			lastId = subscribers[len(subscribers)-1].Id
			for _, subscriber := range subscribers {
				template2 := *template
				user, err := w.GetUserInfoById(uint(subscriber.UserId))
				if err != nil {
					user = &model.User{Email: subscriber.Email, UserName: strings.SplitN(subscriber.Email, "@", 2)[0]}
				}
				data["user"] = user
				rendered, err := w.RenderEmailTemplate(&template2, data)
				if err != nil {
					// 渲染失败只跳过这一封，不能中断整批 ——
					// 原先是 return，一封模板出错会导致其后所有收件人静默漏发，
					// 而接口早已返回「操作成功」（2026-10-03 修）。
					slog.Error("SendSubscriberMail 邮件模板渲染失败", "email", subscriber.Email, "error", err)
					result.inc(0, 1)
					result.AddError(subscriber.Email, "渲染失败: "+err.Error())
					continue
				}
				err = w.SendMail(rendered.Subject, rendered.Content, nil, subscriber.Email)
				subscriber.SendTimes += 1
				if err != nil {
					subscriber.ErrorTimes += 1
				}
				w.DB.Save(subscriber)
				if err != nil {
					result.inc(0, 1)
					result.AddError(subscriber.Email, "发送失败: "+err.Error())
				} else {
					result.inc(1, 0)
				}
			}
		}
		result.finish()
		return result.JobID
	} else if len(req.Emails) > 0 {
		result := newSendMailResult("email", len(req.Emails))
		for _, email := range req.Emails {
			subscriber, _ := w.GetSubscriberByEmail(email)
			if subscriber == nil {
				subscriber = &model.Subscriber{
					Email:  email,
					Status: 1,
				}
			}
			user, err := w.GetUserInfoById(uint(subscriber.UserId))
			if err != nil {
				user = &model.User{Email: subscriber.Email, UserName: strings.SplitN(subscriber.Email, "@", 2)[0]}
			}
			data["user"] = user
			template2 := *template
			rendered, err := w.RenderEmailTemplate(&template2, data)
			if err != nil {
				// 同上：跳过这一封而不是中断整批
				slog.Error("SendSubscriberMail 邮件模板渲染失败", "email", subscriber.Email, "error", err)
				result.inc(0, 1)
				result.AddError(subscriber.Email, "渲染失败: "+err.Error())
				continue
			}
			err = w.SendMail(rendered.Subject, rendered.Content, nil, subscriber.Email)
			subscriber.SendTimes += 1
			if err != nil {
				subscriber.Status = 0
				subscriber.ErrorTimes += 1
			}
			w.DB.Save(subscriber)
			if err != nil {
				result.inc(0, 1)
				result.AddError(subscriber.Email, "发送失败: "+err.Error())
			} else {
				result.inc(1, 0)
			}
		}
		result.finish()
		return result.JobID
	}
	// type 非法（既非 all/category，也没有任何 emails）：连 job 都没建，
	// 返回空串让端点报错——原先这里静默返回「操作成功」却什么都没发。
	return ""
}

func (w *Website) GetSubscriberCategories() []*model.SubscriberCategory {
	var categories []*model.SubscriberCategory
	w.DB.Model(&model.SubscriberCategory{}).Find(&categories)
	return categories
}

func (w *Website) GetSubscriberCategory(id int64) (*model.SubscriberCategory, error) {
	var category model.SubscriberCategory
	err := w.DB.Model(&model.SubscriberCategory{}).Where("id = ?", id).First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (w *Website) GetSubscriberCategoryByTitle(title string) (*model.SubscriberCategory, error) {
	var category model.SubscriberCategory
	err := w.DB.Model(&model.SubscriberCategory{}).Where("title = ?", title).First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (w *Website) DeleteSubscriberCategory(id int64) {
	w.DB.Model(&model.SubscriberCategory{}).Where("id = ?", id).Delete(&model.SubscriberCategory{})
	// 更新关联的订阅者
	w.DB.Model(&model.Subscriber{}).Where("category_id = ?", id).Update("category_id", 0)
}

func (w *Website) UpdateSubscriberCount(categoryId int64) {
	var subscriberCount int64
	w.DB.Model(&model.Subscriber{}).Where("`category_id` = ?", categoryId).Count(&subscriberCount)
	w.DB.Model(&model.SubscriberCategory{}).Where("`id` = ?", categoryId).Update("subscriber_count", subscriberCount)
}
