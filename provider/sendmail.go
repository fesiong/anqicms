package provider

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	_ "github.com/wneessen/go-mail"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

const MailLogFile = "mail.log"

const (
	SendTypeGuestbook = 1 // 新留言
	SendTypeDaily     = 2 // 网站日报
	SendTypeNewOrder  = 3 // 新订单
	SendTypePayOrder  = 4 // 新订单
)

type MailLog struct {
	CreatedTime int64  `json:"created_time"`
	Subject     string `json:"subject"`
	Status      string `json:"status"`
	Address     string `json:"address"`
}

var DefaultEmailTemplates = []config.EmailTemplate{
	{
		Key:         "guestbook",
		Type:        "system",
		Name:        "留言通知",
		Readonly:    false,
		Description: "当前网站有留言时，会通过邮件发送给设置的邮箱。",
		Subject:     `您的[{% system with name="SiteName" %}]网站有来自[{{guestbook.UserName}}]的新留言`,
		Content: `您好，您的网站[{% system with name="SiteName" %}]有一条新的留言，内容如下：

用户：{{guestbook.UserName}}

联系方式：{{guestbook.Contact}}

内容：{{guestbook.Content}}

{% for key, value in guestbook.ExtraData %}
{{key}}: {{value}}
{% endfor %}

时间：{{guestbook.CreatedTime|dateFormat:"2006-01-02 15:04:05"}}

IP: {{guestbook.Ip}}

来源页面: {{guestbook.Refer}}

此邮件为系统自动发送，请勿回复。`,
	},
	{
		Key:         "auto_reply",
		Type:        "user",
		Name:        "留言自动回复",
		Readonly:    false,
		Description: "当客户提交留言时，会自动回复一封邮件给客户。",
		Subject:     `[{% system with name="SiteName" %}]: 感谢您的留言`,
		Content: `尊敬的 {{user.UserName}}，您好！

我们已收到您的留言，将尽快回复您。

{% system with name="SiteName" %} 团队 敬上`,
	},
	{
		Key:         "report",
		Type:        "system",
		Name:        "网站日报",
		Readonly:    true,
		Description: "每日发送网站流量等数据给设置的邮箱",
	},
	{
		Key:         "register",
		Type:        "user",
		Name:        "注册验证邮件",
		Readonly:    false,
		Description: "当用户通过邮件注册账号时，会通过邮件发送验证码或验证链接给用户。",
		Subject:     `[{% system with name="SiteName" %}]: 请验证您的邮箱地址`,
		Content: `尊敬的 {{user.UserName}}，您好！

感谢您注册 {% system with name="SiteName" %}。请点击以下链接验证您的邮箱地址：

{{url|safe}}

如果您没有注册过我们的账号，请忽略此邮件。

{% system with name="SiteName" %} 团队 敬上`,
	},
	{
		Key:         "new_order",
		Type:        "system",
		Name:        "订单确认通知",
		Readonly:    false,
		Description: "当客户创建新订单时，会通过邮件发送给设置的邮箱。",
		Subject:     `[{% system with name="SiteName" %}]: 您的订单（{{order.OrderId}}）已提交`,
		Content: `尊敬的 {{user.UserName}}，您好！

感谢您在 {% system with name="SiteName" %} 下单。您的订单已提交，目前待支付。为确保商品及时处理和发货，请尽快完成支付。

订单详情：  
订单号：{{order.OrderId}}  
下单时间：{{order.CreatedTime|dateFormat:"2006-01-02 15:04:05"}}  
订单金额：{{order.Amount|priceFormat}}  

请选择合适的支付方式完成付款。如果您已经完成支付，请忽略此提醒。

支付成功且订单开始履约后，我们会再发送邮件通知您。

如有任何关于订单或支付流程的疑问，请随时与我们联系。

{% system with name="SiteName" %} 团队 敬上`,
	},
	{
		Key:         "pay_order",
		Type:        "system",
		Name:        "支付成功通知",
		Readonly:    false,
		Description: "当客户支付订单时，会通过邮件发送给设置的邮箱。",
		Subject:     `「{% system with name="SiteName" %}」: 您的订单（{{order.OrderId}}）已支付`,
		Content: `尊敬的 {{user.UserName}}，您好！

感谢您完成支付。您的订单已成功付款，目前正在处理中。订单发货后我们将再发送邮件通知您。

订单详情：  
订单号：{{order.OrderId}}  
支付时间：{{order.PaidTime|dateFormat:"2006-01-02 15:04:05"}}  
订单金额：{{order.Amount|priceFormat}}  

感谢您的惠顾，我们将确保您的订单得到及时处理。

如有任何关于订单的疑问，请随时与我们联系。

{% system with name="SiteName" %} 团队 敬上`,
	},
	{
		Key:         "recommendation",
		Type:        "user",
		Name:        "个性化推荐邮件",
		Readonly:    false,
		Description: "可以不定期根据用户浏览过的产品，向订阅用户发送个性化推荐邮件。",
		Subject:     `[{% system with name="SiteName" %}]: 为您推荐的商品`,
		Content: `尊敬的 {{user.UserName}}，您好！

根据您之前的浏览记录，我们为您推荐以下文章：

{% archiveList archives with limit=5 %}
{% for item in archives %}
**{{item.Title}}**  
  {{ item.Description }}  
链接：{{item.Link}}  
{% endfor %}
{% endarchiveList %}

快来看看这些专为您精选的推荐吧！

{% system with name="SiteName" %} 团队 敬上`,
	},
	{
		Key:         "newsletter_confirm",
		Type:        "user",
		Name:        "邮件订阅确认",
		Readonly:    false,
		Description: "当用户订阅邮件时，会通过邮件发送确认邮件给用户。",
		Subject:     `[{% system with name="SiteName" %}]: 欢迎订阅我们的邮件！`,
		Content: `您好，{{user.Email}}！

感谢您订阅我们的邮件。您已加入我们的专属社区，之后将收到：

- 抢先了解促销和优惠活动
- 新品发布通知
- 会员专属折扣
- 实用技巧和行业资讯

您很快就会收到我们发送的邮件。我们承诺只为您发送相关且有价值的内容。

如果您想退订，只需点击任意邮件底部的「退订」链接即可。

欢迎加入！

{% system with name="SiteName" %} 团队 敬上`,
	},
}

func (w *Website) GetEmailTemplates() []config.EmailTemplate {
	templates := append([]config.EmailTemplate{}, DefaultEmailTemplates...)
	for i := range templates {
		for _, v := range w.PluginSendmail.Templates {
			if v.Key == templates[i].Key {
				templates[i].Open = v.Open
				if v.Subject != "" {
					templates[i].Subject = v.Subject
				}
				if v.Content != "" {
					templates[i].Content = v.Content
				}
				if v.Delay > 0 {
					templates[i].Delay = v.Delay
				}
				break
			}
		}
	}

	return templates
}

// 获取合并后的
func (w *Website) GetEmailTemplateInfo(key string) (*config.EmailTemplate, bool) {
	var exist bool
	var template config.EmailTemplate
	for _, v := range DefaultEmailTemplates {
		if v.Key == key {
			template = v
			exist = true
			break
		}
	}
	for _, v := range w.PluginSendmail.Templates {
		if v.Key == key {
			template.Open = v.Open
			if !template.Readonly {
				if v.Subject != "" {
					template.Subject = v.Subject
				}
				if v.Content != "" {
					template.Content = v.Content
				}
				if v.Delay > 0 {
					template.Delay = v.Delay
				}
			}
			break
		}
	}

	return &template, exist
}

func (w *Website) SaveEmailTemplateInfo(req *config.EmailTemplate) (*config.EmailTemplate, error) {
	template := config.EmailTemplate{
		Open:    req.Open,
		Key:     req.Key,
		Delay:   req.Delay,
		Subject: req.Subject,
		Content: req.Content,
	}
	var exist bool
	for i, v := range w.PluginSendmail.Templates {
		if v.Key == req.Key {
			w.PluginSendmail.Templates[i] = template
			exist = true
			break
		}
	}
	if !exist {
		w.PluginSendmail.Templates = append(w.PluginSendmail.Templates, template)
	}

	// 保存到数据库
	if err := w.SaveSettingValue(SendmailSettingKey, w.PluginSendmail); err != nil {
		return nil, err
	}

	return &template, nil
}

func (w *Website) RenderEmailTemplate(template *config.EmailTemplate, data map[string]interface{}) (*config.EmailTemplate, error) {
	var err error
	var err2 error
	template.Subject, err2 = pongo2.RenderTemplateString(template.Subject, data)
	if err2 != nil {
		// 渲染失败
		err = err2
		template.Subject = err.Error()
	}
	// render content
	// content先从Markdown转换为HTML
	template.Content, err2 = pongo2.RenderTemplateString(template.Content, data)
	if err2 != nil {
		// 渲染失败败
		err = err2
		template.Content = err.Error()
	}
	if !strings.HasPrefix(template.Content, "<") {
		template.Content = library.MarkdownToHTML(template.Content)
	}

	return template, err
}

func (w *Website) GetLastSendmailList(currentPage, pageSize int) ([]*MailLog, int64) {
	offset := (currentPage - 1) * pageSize
	var total int64

	var mailLogs []*MailLog
	//获取20条数据
	filePath := w.CachePath + MailLogFile
	logFile, err := os.Open(filePath)
	if nil != err {
		//打开失败
		return mailLogs, 0
	}
	defer logFile.Close()
	var fileSize int64
	// 获取文件大小
	if fileInfo, err := logFile.Stat(); err == nil {
		fileSize = fileInfo.Size()
	} else {
		return nil, 0
	}

	reader := bufio.NewReader(logFile)
	// 每次读取8K
	buffer := make([]byte, 8192)
	lineBuffer := ""

	// 按limit行读取
	var lines = make([]string, 0, pageSize)
	var curPos = fileSize
	var curLine = 0

	// 倒序读取文件内容
	for curPos > 0 {
		// 定位到需要读取的位置
		bytesToRead := int64(len(buffer))
		if curPos-bytesToRead < 0 {
			bytesToRead = curPos
		}

		// 调整文件指针到合适位置
		curPos -= bytesToRead
		logFile.Seek(curPos, io.SeekStart)

		// 读取文件数据到缓冲区
		n, err := reader.Read(buffer[:bytesToRead])
		if err != nil && err != io.EOF {
			return nil, 0
		}

		// 将读到的数据加入行缓冲
		lineBuffer = string(buffer[:n]) + lineBuffer

		// 处理行
		for {
			newLineIdx := len(lineBuffer) - 1
			for newLineIdx >= 0 && lineBuffer[newLineIdx] != '\n' {
				newLineIdx--
			}

			// 找到完整的行
			if newLineIdx == -1 {
				break
			}

			line := lineBuffer[newLineIdx+1:]
			lineBuffer = lineBuffer[:newLineIdx]
			if line != "" {
				curLine++
				if curLine <= offset {
					continue
				}
				lines = append(lines, line)

				// 如果已经获取到需要的行数，跳出循环
				if len(lines) >= pageSize {
					break
				}
			}
		}

		// 处理剩余内容
		if curPos == 0 && lineBuffer != "" && len(lines) < pageSize {
			lines = append(lines, lineBuffer)
		}
		// 如果已经获取到需要的行数，跳出循环
		if len(lines) >= pageSize {
			break
		}
	}
	for _, line := range lines {
		var mailLog MailLog
		err := json.Unmarshal([]byte(line), &mailLog)
		if err == nil {
			mailLogs = append(mailLogs, &mailLog)
		}
	}

	// 如果文件计数缓存，则重新计数
	err = w.Cache.Get("send-mail-total", &total)
	if err != nil {
		logFile.Seek(0, io.SeekStart)
		sc := bufio.NewScanner(reader)
		for sc.Scan() {
			total++
		}
		// 如果是当天的文件，则不缓存
		w.Cache.Set("send-mail-total", total, 300)
	}
	if pageSize > len(mailLogs) {
		total = int64(offset + len(mailLogs))
	}
	if total < int64(len(mailLogs)) {
		total = int64(len(mailLogs))
	}

	return mailLogs, total
}

func (w *Website) SendMail(subject, content string, attach *library.Attachment, recipients ...string) error {
	setting := w.PluginSendmail
	if setting.Account == "" {
		//成功配置，则跳过
		return errors.New(w.Tr("PleaseConfigureSender"))
	}
	userHtml := false
	if strings.HasPrefix(content, "<") {
		userHtml = true
	}
	err := w.sendMail(subject, content, nil, recipients, userHtml, true)

	return err
}

func (w *Website) sendMail(subject, content string, attachments []*library.Attachment, recipients []string, useHtml bool, setLog bool) error {
	setting := w.PluginSendmail
	port := setting.Port
	if port == 0 {
		//默认使用25端口
		port = 25
	}
	if setting.UseSSL == 1 && port == 25 {
		//如果使用ssl，设置了25端口，则使用465
		port = 465
	}

	if setting.Account == "" {
		//成功配置，则跳过
		return errors.New(w.Tr("PleaseConfigureSender"))
	}

	//开始发送
	email := library.NewEMail(`{"port":25}`)
	email.From = setting.Account
	email.Host = setting.Server
	email.Port = setting.Port
	email.Username = setting.Account
	if setting.UseSSL == 1 {
		email.Secure = "SSL"
	}
	email.Password = setting.Password
	for _, attach := range attachments {
		email.Attach(bytes.NewReader(attach.Content), attach.Filename, attach.Header.Get("Content-Type"))
	}

	if len(recipients) == 0 {
		if setting.Recipient != "" {
			tmp := strings.Split(setting.Recipient, ",")
			for _, v := range tmp {
				v = strings.TrimSpace(v)
				if v != "" {
					recipients = append(recipients, v)
				}
			}
		}
		if len(recipients) == 0 {
			recipients = append(recipients, setting.Account)
		}
	}
	// 多个收件地址的时候，分开发送
	var err error
	for _, to := range recipients {
		email.To = []string{to}
		email.Subject = subject
		if useHtml {
			email.HTML = content
		} else {
			email.Text = content
		}

		if err = email.Send(); err != nil {
			if setLog {
				w.logMailError(to, subject, err.Error())
			}
			continue
		}
		if setLog {
			w.logMailError(to, subject, w.Tr("SentSuccessfully"))
		}
	}
	return err
}

func (w *Website) logMailError(address, subject, status string) {
	mailLog := MailLog{
		CreatedTime: time.Now().Unix(),
		Subject:     subject,
		Status:      status,
		Address:     address,
	}

	content, err := json.Marshal(mailLog)

	if err == nil {
		library.DebugLog(w.CachePath, MailLogFile, string(content))
	}
}

func (w *Website) SendVerifyEmail(user *model.User, state string) error {
	// 是否需要邮箱验证
	template, exist := w.GetEmailTemplateInfo("register")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	token := library.Md5(user.Email + user.Password)
	verifyCode := library.CodeCache.Generate(user.Email)
	verifyUrl := w.System.BaseUrl + "/api/verify/email?token=" + token + "&code=" + verifyCode + "&state=" + state + "&email=" + user.Email
	data := map[string]interface{}{
		"website": w,
		"user":    user,
		"code":    verifyCode,
		"url":     verifyUrl,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	return w.SendMail(template.Subject, template.Content, nil, user.Email)
}

func (w *Website) SendWelcomeEmail(user *model.User) error {
	if user.Email == "" {
		return errors.New("no email")
	}
	template, exist := w.GetEmailTemplateInfo("welcome")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	return w.SendMail(template.Subject, template.Content, nil, user.Email)
}

func (w *Website) SendNewSubscriberEmail(subscriber *model.Subscriber) error {
	if subscriber.Email == "" {
		return errors.New("no email")
	}
	template, exist := w.GetEmailTemplateInfo("newsletter_confirm")
	if !exist || !template.Open {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"user":    subscriber,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	return w.SendMail(template.Subject, template.Content, nil, subscriber.Email)
}

func (w *Website) SendUserGroupChangeEmail(user *model.User, oldGroup *model.UserGroup) error {
	if user.Email == "" {
		return errors.New("no email")
	}
	template, exist := w.GetEmailTemplateInfo("group_change")
	if !exist || !template.Open {
		return errors.New("no template")
	}
	data := map[string]interface{}{
		"website":  w,
		"oldGroup": oldGroup,
		"user":     user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	w.SendMail(template.Subject, template.Content, nil, user.Email)

	return nil
}

func (w *Website) SendNewOrderEmail(user *model.User, order *model.Order) error {
	if user == nil {
		user = &model.User{}
	}
	template, exist := w.GetEmailTemplateInfo("new_order")
	if !exist {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"order":   order,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	if user.Email != "" && template.Open {
		// 发给用户
		w.SendMail(template.Subject, template.Content, nil, user.Email)
	}
	// 发给网站管理员
	_ = w.sendMail(template.Subject, template.Content, nil, nil, false, false)

	return nil
}

func (w *Website) SendPayOrderEmail(user *model.User, order *model.Order) error {
	if user == nil {
		user = &model.User{}
	}
	template, exist := w.GetEmailTemplateInfo("pay_order")
	if !exist {
		return errors.New("no template")
	}

	data := map[string]interface{}{
		"website": w,
		"order":   order,
		"user":    user,
	}
	var err error
	template, err = w.RenderEmailTemplate(template, data)
	if err != nil {
		return err
	}

	if user.Email != "" && template.Open {
		// 发给用户
		w.SendMail(template.Subject, template.Content, nil, user.Email)
	}
	// 发给网站管理员
	_ = w.sendMail(template.Subject, template.Content, nil, nil, false, false)

	return nil
}

func (w *Website) AddMailPlan(plan *model.MailPlan) error {
	// 如果没开启邮件服务，则不添加计划
	if w.PluginSendmail.Account == "" {
		//成功配置，则跳过
		return errors.New(w.Tr("PleaseConfigureSender"))
	}
	err := w.DB.Create(plan).Error
	if err == nil {
		// 添加到计划
		if w.MailService != nil {
			w.MailService.Add(plan)
		}
	}
	return err
}

func (w *Website) CancelMailPlan(identifier string, isDel bool) error {
	// 删除计划
	var plan model.MailPlan
	err := w.DB.Where("`identifier` = ?", identifier).First(&plan).Error
	if err == nil && plan.Status != 2 {
		if isDel {
			return w.DB.Where("`identifier` = ?", identifier).Delete(&model.MailPlan{}).Error
		} else {
			w.DB.Model(&model.MailPlan{}).Where("`identifier` = ?", identifier).UpdateColumn("status", -1)
		}
	}
	return nil
}
