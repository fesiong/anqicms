package provider

import (
	"fmt"
	"net/textproto"
	"os"
	"path/filepath"
	"sync"
	"time"

	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

type TinyMailplan struct {
	Id         int64
	Identifier string
	SendTime   int64
	ErrorTimes int // 错误次数，最多重试3次
	LastError  string
}

type MailService struct {
	mu         sync.Mutex
	w          *Website
	ErrorTimes int
	MailQueue  []TinyMailplan
}

func (w *Website) NewMailService() *MailService {
	// 如果没有配置邮箱，则不需要启动服务
	ms := &MailService{
		w:          w,
		ErrorTimes: 0,
		MailQueue:  []TinyMailplan{},
	}

	go ms.init()

	return ms
}

func (ms *MailService) init() {
	// 启动的时候，把邮件计划加入队列
	var plans []model.MailPlan
	ms.w.DB.Where("status IN (0,3)").Find(&plans)
	for _, plan := range plans {
		ms.Add(&plan)
	}
	for {
		time.Sleep(time.Second * 5)
		ms.Send()
	}
}

func (ms *MailService) Add(plan *model.MailPlan) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.MailQueue = append(ms.MailQueue, TinyMailplan{
		Id:         plan.Id,
		Identifier: plan.Identifier,
		SendTime:   plan.SendTime,
		ErrorTimes: 0,
	})

	return nil
}

func (ms *MailService) Send() {
	currentTime := time.Now().Unix()
	for i := 0; i < len(ms.MailQueue); i++ {
		ms.mu.Lock()
		plan := ms.MailQueue[i]
		ms.mu.Unlock()
		if plan.SendTime > currentTime {
			// 时间不到的，跳过
			continue
		}
		// 已经过时超过1小时的，不再下发
		if plan.SendTime < currentTime-3600 {
			ms.w.CancelMailPlan(plan.Identifier, false)
			i--
			continue
		}
		err := ms.SendPlan(plan)

		ms.mu.Lock()
		if err != nil {
			// 发送失败，记录错误次数
			plan.ErrorTimes++
			if plan.ErrorTimes >= 3 {
				// 错误次数超过3次，则删除计划
				ms.MailQueue = append(ms.MailQueue[:i], ms.MailQueue[i+1:]...)
				ms.w.CancelMailPlan(plan.Identifier, false)
				i--
			} else {
				// 错误次数小于3次，则更新计划
				ms.MailQueue[i] = plan
			}
		} else {
			// 发送成功，删除计划
			ms.MailQueue = append(ms.MailQueue[:i], ms.MailQueue[i+1:]...)
			i--
		}
		ms.mu.Unlock()
	}
}

func (ms *MailService) SendPlan(tinyPlan TinyMailplan) error {
	var plan model.MailPlan
	err := ms.w.DB.Where("id = ?", tinyPlan.Id).First(&plan).Error
	if err != nil {
		return err
	}
	if plan.Status != 0 && plan.Status != 3 {
		return nil
	}
	var attach *library.Attachment
	if plan.AttachPath != "" {
		pdfBuf, err := os.ReadFile(ms.w.DataPath + plan.AttachPath)
		if err == nil {
			attach = &library.Attachment{
				Filename: filepath.Base(plan.AttachPath),
				Header: textproto.MIMEHeader{
					"Content-Disposition": []string{fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(plan.AttachPath))},
					"Content-Type":        []string{"application/pdf"},
				},
				Content: pdfBuf,
			}
		}
	}
	err = ms.w.SendMail(plan.Subject, plan.Content, attach, plan.Recipients...)
	if err != nil {
		plan.Status = 3
		plan.SendResult = err.Error()
		if len(plan.SendResult) > 1000 {
			plan.SendResult = plan.SendResult[:1000]
		}
	} else {
		plan.Status = 2
		plan.SendResult = "success"
	}
	ms.w.DB.Model(&plan).UpdateColumns(map[string]interface{}{
		"status":        plan.Status,
		"executed_time": time.Now().Unix(),
		"send_result":   plan.SendResult,
	})

	return err
}
