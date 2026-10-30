package provider

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) GetCommunicationOrderList(ops func(tx *gorm.DB) *gorm.DB, order string, currentPage, pageSize int) ([]*model.CommunicationOrder, int64) {
	var total int64
	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize
	var list []*model.CommunicationOrder

	if order == "" {
		order = "id desc"
	}

	tx := w.DB.Model(&model.CommunicationOrder{}).Order(order)

	if ops != nil {
		tx = ops(tx)
	}
	tx.Count(&total)
	tx.Offset(offset).Limit(pageSize).Find(&list)

	return list, total
}

func (w *Website) CommunicationOrderCreate(req *request.CommunicationOrderRequest) (*model.CommunicationOrder, error) {
	order := model.CommunicationOrder{
		UserId:     req.UserId,
		ArchiveId:  req.ArchiveId,
		Content:    req.Content,
		Phone:      req.Phone,
		Title:      req.Title,
		Images:     req.Images,
		OrderExtra: req.OrderExtra,
		Status:     config.CommunicationStatusWaiting,
	}

	err := w.DB.Create(&order).Error
	if err != nil {
		return nil, err
	}

	frontUrl := w.System.BaseUrl
	if w.System.FrontUrl != "" {
		frontUrl = w.System.FrontUrl
	}
	// send email to admin
	subject := w.System.SiteName + "(" + frontUrl + ")" + w.Tr("NewCommunicationOrderCreated")
	content := w.Tr("NewCommunicationOrderCreated") + ": \n" + w.Tr("Budget") + ": " + order.Title +
		"\n" + w.Tr("EventDate") + ": " + order.EventDate +
		"\n" + w.Tr("Phone") + ": " + order.Phone +
		"\n" + w.Tr("CreatedTime") + ": " + time.Unix(order.CreatedTime, 0).Format("2006-01-02 15:04:05") +
		"\n" + w.Tr("Description") + ": " + order.Content
	go w.sendMail(subject, content, nil, nil, false, false)

	// send email to customer
	_ = w.SendCommunicationSummitedEmail(&order)

	return &order, nil
}

func (w *Website) GetCommunicationOrderById(id int64) (*model.CommunicationOrder, error) {
	var order model.CommunicationOrder
	err := w.DB.Where("id = ?", id).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (w *Website) GetCommunicationOrderByOrderId(orderId string) (*model.CommunicationOrder, error) {
	var order model.CommunicationOrder
	err := w.DB.Where("order_id = ?", orderId).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (w *Website) GetCommunicationOrderByUser(userId int64, id int64) (*model.CommunicationOrder, error) {
	var order model.CommunicationOrder
	err := w.DB.Where("id = ? and user_id = ?", id, userId).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (w *Website) GetLastCommunicationOrderByUser(userId int64) (*model.CommunicationOrder, error) {
	var order model.CommunicationOrder
	err := w.DB.Where("user_id = ?", userId).Order("id desc").First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (w *Website) SaveCommunicationOrderImages(req *request.CommunicationOrderImagesRequest) error {
	order, err := w.GetCommunicationOrderById(req.Id)
	if err != nil {
		return err
	}

	if req.Field == "images" {
		order.Images = req.Images
	} else if req.Field == "sketch_images" {
		order.SketchImages = req.Images
	} else if req.Field == "finished_images" {
		order.FinishedImages = req.Images
	}

	return w.DB.Save(order).Error
}

func (w *Website) CommunicationSendQuote(req *request.CommunicationOrderRequest) error {
	order, err := w.GetCommunicationOrderById(req.Id)
	if err != nil {
		return err
	}
	order.Status = config.CommunicationStatusQuote
	order.QuotePrice = req.QuotePrice
	order.QuoteContent = req.QuoteContent
	err = w.DB.Save(order).Error
	if err != nil {
		return err
	}
	// 添加一份报价到message
	if req.QuoteContent != "" {
		msg1 := model.CommunicationMessage{
			CommOrderId: order.Id,
			SenderId:    req.SenderId,
			SenderType:  "admin",
			SenderName:  req.SenderName,
			Content:     req.QuoteContent,
			MessageType: "text",
		}
		msg1.Content = strings.TrimSpace(msg1.Content)
		if !strings.HasPrefix(msg1.Content, "<") {
			msg1.Content = library.MarkdownToHTML(msg1.Content)
		}
		w.DB.Save(&msg1)
	}
	msg2 := model.CommunicationMessage{
		CommOrderId: order.Id,
		SenderId:    req.SenderId,
		SenderType:  "admin",
		SenderName:  req.SenderName,
		Content:     fmt.Sprintf("%s%.2f", w.Currency.Symbol, float64(order.QuotePrice)/100),
		MessageType: "quote",
	}
	w.DB.Save(&msg2)
	// 发送报价
	_ = w.SendCommunicationQuoteEmail(order)

	return nil
}

func (w *Website) CommunicationAcceptQuote(id int64) error {
	communicationOrder, err := w.GetCommunicationOrderById(id)
	if err != nil {
		return err
	}
	if communicationOrder.Status == config.CommunicationStatusAccepted {
		return nil
	}

	communicationOrder.Status = config.CommunicationStatusAccepted
	err = w.DB.Save(communicationOrder).Error

	frontUrl := w.System.BaseUrl
	if w.System.FrontUrl != "" {
		frontUrl = w.System.FrontUrl
	}
	// send email to admin
	subject := w.System.SiteName + "(" + frontUrl + ")" + w.Tr("CommunicationOrderQuoteAccepted")
	content := w.Tr("CommunicationOrderQuoteAccepted") + ": \n" + w.Tr("Budget") + ": " + communicationOrder.Title +
		"\n" + w.Tr("EventDate") + ": " + communicationOrder.EventDate +
		"\n" + w.Tr("Phone") + ": " + communicationOrder.Phone +
		"\n" + w.Tr("QuotePrice") + ": " + fmt.Sprintf("%.2f", float64(communicationOrder.QuotePrice)/100) +
		"\n" + w.Tr("CreatedTime") + ": " + time.Unix(communicationOrder.CreatedTime, 0).Format("2006-01-02 15:04:05") +
		"\n" + w.Tr("Description") + ": " + communicationOrder.Content
	go w.sendMail(subject, content, nil, nil, false, false)

	return err
}

func (w *Website) CommunicationMessageSend(req *request.CommunicationMessageRequest) error {
	req.Content = strings.TrimSpace(req.Content)
	if req.MessageType == "text" && !strings.HasPrefix(req.Content, "<") {
		req.Content = library.MarkdownToHTML(req.Content)
	}

	msg1 := model.CommunicationMessage{
		CommOrderId: req.CommOrderId,
		SenderId:    req.SenderId,
		SenderType:  req.SenderType,
		SenderName:  req.SenderName,
		Content:     req.Content,
		MessageType: req.MessageType,
	}

	err := w.DB.Save(&msg1).Error
	if err != nil {
		return err
	}

	if req.SenderType == "user" {
		frontUrl := w.System.BaseUrl
		if w.System.FrontUrl != "" {
			frontUrl = w.System.FrontUrl
		}
		// send email to admin
		subject := w.System.SiteName + "(" + frontUrl + ")" + w.Tr("NewCommunicationOrderMessage")
		content := w.Tr("NewCommunicationOrderMessage") + ": \n" +
			"\n" + w.Tr("CreatedTime") + ": " + time.Unix(msg1.CreatedTime, 0).Format("2006-01-02 15:04:05") +
			"\n" + w.Tr("SenderName") + ": " + msg1.SenderName +
			"\n" + w.Tr("Content") + ": " + msg1.Content
		go w.sendMail(subject, content, nil, nil, false, false)

	} else {
		// send email to user
		_ = w.SendCommunicationNewMessageEmail(&msg1)
	}

	return nil
}

func (w *Website) DeleteCommunicationMessage(id int64) error {
	// 一行都没删掉说明这条消息不存在；原来直接返回 gorm 的 err（nil），
	// 端点就回了 "deleted"，调用方分不清"删掉了"和"本来就没有"。
	res := w.DB.Where("id = ?", id).Delete(&model.CommunicationMessage{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (w *Website) GetCommunicationMessages(commOrderId int64, q string, currentPage, pageSize int) ([]*model.CommunicationMessage, int64) {
	var total int64
	if currentPage < 1 {
		currentPage = 1
	}
	offset := (currentPage - 1) * pageSize

	var list []*model.CommunicationMessage
	tx := w.DB.Model(&model.CommunicationMessage{}).Where("comm_order_id = ?", commOrderId)
	if q != "" {
		tx = tx.Where("content like ?", "%"+q+"%")
	}
	tx.Count(&total)
	err := tx.Order("id desc").Limit(pageSize).Offset(offset).Find(&list).Error
	if err != nil {
		return nil, 0
	}

	return list, total
}

func (w *Website) CloseCommunicationOrder(orderId int64) error {
	var order model.CommunicationOrder
	err := w.DB.Where("id = ?", orderId).First(&order).Error
	if err != nil {
		return err
	}

	if order.Status == config.CommunicationStatusCancel {
		return nil
	}

	order.Status = config.CommunicationStatusCancel
	return w.DB.Save(&order).Error
}
