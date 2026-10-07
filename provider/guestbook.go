package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

func (w *Website) GetGuestbookList(ops func(tx *gorm.DB) *gorm.DB, currentPage, pageSize int) ([]*model.Guestbook, int64, error) {
	var guestbooks []*model.Guestbook
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := w.DB.Model(&model.Guestbook{}).Order("id desc")
	if ops != nil {
		builder = ops(builder)
	}

	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&guestbooks).Error
	if err != nil {
		return nil, 0, err
	}

	return guestbooks, total, nil
}

func (w *Website) GetAllGuestbooks() ([]*model.Guestbook, error) {
	var guestbooks []*model.Guestbook
	err := w.DB.Model(&model.Guestbook{}).Order("id desc").Find(&guestbooks).Error
	if err != nil {
		return nil, err
	}

	return guestbooks, nil
}

func (w *Website) GetGuestbookById(id uint) (*model.Guestbook, error) {
	var guestbook model.Guestbook

	err := w.DB.Where("`id` = ?", id).First(&guestbook).Error
	if err != nil {
		return nil, err
	}

	return &guestbook, nil
}

func (w *Website) UpdateGuestbookStatus(id uint, status int) error {
	err := w.DB.Model(&model.Guestbook{}).Where("`id` = ?", id).Update("status", status).Error

	return err
}

func (w *Website) DeleteGuestbook(guestbook *model.Guestbook) error {
	err := w.DB.Delete(guestbook).Error
	if err != nil {
		return err
	}

	return nil
}

func (w *Website) GetGuestbookFields() []*config.CustomField {
	//这里有默认的设置
	defaultFields := []*config.CustomField{
		{
			Name:      w.Tr("UserName"),
			FieldName: "user_name",
			Type:      "text",
			Required:  true,
			IsSystem:  true,
		},
		{
			Name:      w.Tr("ContactPhoneNumber"),
			FieldName: "contact",
			Type:      "text",
			Required:  false,
			IsSystem:  true,
		},
		{
			Name:      w.Tr("Email"),
			FieldName: "email",
			Type:      "text",
			Required:  false,
			IsSystem:  false,
		},
		{
			Name:      w.Tr("Qq"),
			FieldName: "qq",
			Type:      "text",
			Required:  false,
			IsSystem:  false,
		},
		{
			Name:      w.Tr("Whatsapp"),
			FieldName: "whatsapp",
			Type:      "text",
			Required:  false,
			IsSystem:  false,
		},
		{
			Name:      w.Tr("MessageContent"),
			FieldName: "content",
			Type:      "textarea",
			Required:  false,
			IsSystem:  true,
		},
	}

	exists := false
	for _, v := range w.PluginGuestbook.Fields {
		if v.IsSystem || v.FieldName == "user_name" {
			exists = true
			break
		}
	}
	var fields []*config.CustomField
	if exists {
		fields = w.PluginGuestbook.Fields
	} else {
		fields = append(defaultFields, w.PluginGuestbook.Fields...)
	}

	return fields
}

func (w *Website) ProcessGuestbook(guestbook *model.Guestbook, spamStatus int) {
	if w.PluginGuestbook.PushWay == config.GuestbookPushWaySite {
		// 推送站点
		if w.PluginGuestbook.SiteId > 0 {
			pushSite := GetWebsite(w.PluginGuestbook.SiteId)
			if pushSite != nil {
				parentGuestbook := *guestbook
				parentGuestbook.Id = 0
				parentGuestbook.Status = spamStatus
				parentGuestbook.SiteId = w.Id
				_ = w.DB.Save(&parentGuestbook)
			}
		}
	} else if w.PluginGuestbook.PushWay == config.GuestbookPushWayApi {
		// 推送到API
		if w.PluginGuestbook.ApiURL == "" {
			return
		}
		// 构建请求参数
		params := url.Values{}
		params.Set("user_name", guestbook.UserName)
		params.Set("contact", guestbook.Contact)
		params.Set("content", guestbook.Content)
		params.Set("ip", guestbook.Ip)
		params.Set("refer", guestbook.Refer)
		params.Set("created_time", time.Unix(guestbook.CreatedTime, 0).Format("2006-01-02 15:04:05"))
		for key, value := range guestbook.ExtraData {
			params.Set("extra["+key+"]", fmt.Sprint(value))
		}

		apiURL := w.PluginGuestbook.ApiURL
		var req *http.Request
		var err error

		if w.PluginGuestbook.ApiMethod == "query" {
			// GET 方式，参数拼在 URL 上
			if strings.Contains(apiURL, "?") {
				apiURL += "&" + params.Encode()
			} else {
				apiURL += "?" + params.Encode()
			}
			req, err = http.NewRequest(http.MethodGet, apiURL, nil)
		} else if w.PluginGuestbook.ApiMethod == "formdata" {
			// POST form-urlencoded
			body := strings.NewReader(params.Encode())
			req, err = http.NewRequest(http.MethodPost, apiURL, body)
			if err == nil {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
		} else {
			// 默认 json 方式 POST
			jsonBody := make(map[string]interface{})
			jsonBody["user_name"] = guestbook.UserName
			jsonBody["contact"] = guestbook.Contact
			jsonBody["content"] = guestbook.Content
			jsonBody["ip"] = guestbook.Ip
			jsonBody["refer"] = guestbook.Refer
			jsonBody["created_time"] = time.Unix(guestbook.CreatedTime, 0).Format("2006-01-02 15:04:05")
			if len(guestbook.ExtraData) > 0 {
				jsonBody["extra"] = guestbook.ExtraData
			}
			buf, marshalErr := json.Marshal(jsonBody)
			if marshalErr != nil {
				return
			}
			req, err = http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(buf))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
			}
		}
		if err != nil {
			return
		}
		// 设置自定义 Header
		if w.PluginGuestbook.HeaderKey != "" {
			req.Header.Set(w.PluginGuestbook.HeaderKey, w.PluginGuestbook.HeaderValue)
		}
		// 发送请求，非阻塞，忽略响应
		client := &http.Client{Timeout: 10 * time.Second}
		resp, respErr := client.Do(req)
		if respErr == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	} else {
		if spamStatus == 1 {
			// 1 是正常，可以发邮件
			w.SendGuestbookToMail(guestbook)
			if w.ParentId > 0 {
				mainSite := w.GetMainWebsite()
				parentGuestbook := *guestbook
				parentGuestbook.Id = 0
				parentGuestbook.Status = spamStatus
				parentGuestbook.SiteId = w.Id
				_ = mainSite.DB.Save(&parentGuestbook)
				mainSite.SendGuestbookToMail(&parentGuestbook)
			}
		}
	}
}

func (w *Website) SendGuestbookToMail(guestbook *model.Guestbook) {
	recipient := guestbook.Contact
	if !w.VerifyEmailFormat(recipient) {
		recipient = ""
		for _, v := range guestbook.ExtraData {
			vv, _ := v.(string)
			if w.VerifyEmailFormat(vv) {
				recipient = vv
				break
			}
		}
	}
	data := map[string]interface{}{
		"website":   w,
		"guestbook": guestbook,
	}
	//发送邮件
	template, exist := w.GetEmailTemplateInfo("guestbook")
	if exist && template.Open {
		var err error
		template, err = w.RenderEmailTemplate(template, data)
		if err == nil {
			w.SendMail(template.Subject, template.Content, nil)
		}
	}

	// 回复邮件
	template, exist = w.GetEmailTemplateInfo("auto_reply")
	if exist && template.Open {
		var err error
		template, err = w.RenderEmailTemplate(template, data)
		if err == nil {
			w.SendMail(template.Subject, template.Content, nil, recipient)
		}
	}
}
