package provider

import (
	"errors"
	"log/slog"
	"strings"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

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

func (w *Website) SaveSubscriber(req *request.SubscriberRequest) error {
	var err error
	var subscriber = &model.Subscriber{}
	if req.Id > 0 {
		subscriber, err = w.GetSubscriber(req.Id)
		if err != nil {
			return err
		}
		// 判断是否有重名
		exist, err := w.GetSubscriberByEmail(req.Email)
		if err == nil && exist.Id != req.Id {
			return errors.New(w.Tr("EmailAlreadyExists"))
		}
	} else {
		// 判断 email 是否已经存在
		_, err = w.GetSubscriberByEmail(req.Email)
		if err == nil {
			return errors.New(w.Tr("EmailAlreadyExists"))
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
		return err
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

	return nil
}

func (w *Website) SendSubscriberMail(req *request.SubscriberMailRequest) {
	template, exist := w.GetEmailTemplateInfo("recommendation")
	if !exist {
		return
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
					slog.Error("SendSubscriberMail 邮件模板渲染失败")
					return
				}
				err = w.SendMail(rendered.Subject, rendered.Content, nil, subscriber.Email)
				subscriber.SendTimes += 1
				if err != nil {
					subscriber.ErrorTimes += 1
				}
				w.DB.Save(subscriber)
			}
		}
	} else if len(req.Emails) > 0 {
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
				slog.Error("SendSubscriberMail 邮件模板渲染失败")
				return
			}
			err = w.SendMail(rendered.Subject, rendered.Content, nil, subscriber.Email)
			subscriber.SendTimes += 1
			if err != nil {
				subscriber.Status = 0
				subscriber.ErrorTimes += 1
			}
			w.DB.Save(subscriber)
		}
	}
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
