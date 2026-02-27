package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	_ "github.com/wneessen/go-mail"
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

func (w *Website) GetLastSendmailList() ([]*MailLog, error) {
	var mailLogs []*MailLog
	//获取20条数据
	filePath := w.CachePath + MailLogFile
	logFile, err := os.Open(filePath)
	if nil != err {
		//打开失败
		return mailLogs, nil
	}
	defer logFile.Close()

	line := int64(1)
	cursor := int64(0)
	stat, err := logFile.Stat()
	fileSize := stat.Size()
	tmp := ""
	for {
		cursor -= 1
		logFile.Seek(cursor, io.SeekEnd)

		char := make([]byte, 1)
		logFile.Read(char)

		if cursor != -1 && (char[0] == 10 || char[0] == 13) {
			//跳到一个新行，清空
			line++
			//解析
			if tmp != "" {
				var mailLog MailLog
				err := json.Unmarshal([]byte(tmp), &mailLog)
				if err == nil {
					mailLogs = append(mailLogs, &mailLog)
				}
			}
			tmp = ""
		}

		tmp = fmt.Sprintf("%s%s", string(char), tmp)

		if cursor == -fileSize {
			// stop if we are at the beginning
			break
		}
		if line == 100 {
			break
		}
	}
	//解析最后一条
	if tmp != "" {
		var mailLog MailLog
		err := json.Unmarshal([]byte(tmp), &mailLog)
		if err == nil {
			mailLogs = append(mailLogs, &mailLog)
		}
	}

	return mailLogs, nil
}

func (w *Website) SendMail(subject, content string, recipients ...string) error {
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

// SendTypeValid 检查发送类型是否可发送
func (w *Website) SendTypeValid(sendType int) bool {
	// 默认支持新留言发送
	if len(w.PluginSendmail.SendType) == 0 && sendType == SendTypeGuestbook {
		return true
	}
	for _, v := range w.PluginSendmail.SendType {
		if v == sendType {
			return true
		}
	}

	return false
}

// ReplyMail 如果设置了回复邮件，则尝试回复给用户
func (w *Website) ReplyMail(recipient string) error {
	if !strings.Contains(recipient, "@") {
		return errors.New(w.Tr("IncorrectRecipientAddress"))
	}
	if w.PluginSendmail.AutoReply && w.PluginSendmail.ReplySubject != "" && w.PluginSendmail.ReplyMessage != "" {
		return w.SendMail(w.PluginSendmail.ReplySubject, w.PluginSendmail.ReplyMessage, recipient)
	}

	return nil
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
	if w.PluginSendmail.SignupVerify {
		// 发送验证邮件
		// 需要替换的内容：{siteName}/{userName}/{email}/{url}/{verifyCode}/{verifyUrl}
		token := library.Md5(user.Email + user.Password)
		verifyCode := library.CodeCache.Generate(token)
		verifyUrl := w.System.BaseUrl + "/api/verify/email?token=" + token + "&code=" + verifyCode + "&state=" + state + "&email=" + user.Email
		subject := w.PluginSendmail.VerifySubject
		if subject == "" {
			subject = "[{siteName}]: Please verify your email address"
		}
		subject = strings.ReplaceAll(subject, "{siteName}", w.System.SiteName)
		subject = strings.ReplaceAll(subject, "{userName}", user.UserName)
		subject = strings.ReplaceAll(subject, "{email}", user.Email)
		subject = strings.ReplaceAll(subject, "{url}", w.System.BaseUrl)
		subject = strings.ReplaceAll(subject, "{verifyUrl}", verifyUrl)
		subject = strings.ReplaceAll(subject, "{verifyCode}", verifyCode)
		mailBody := w.PluginSendmail.VerifyMessage
		if mailBody == "" {
			mailBody = "<div>Dear {userName},<br><br>Please click the link below to verify your email address:<br><br><a href=\"{verifyUrl}\">{verifyUrl}</a><br><br>If you did not register for this site, please ignore this email.<br><br>Regards,<br>{siteName}</div>"
		}
		mailBody = strings.ReplaceAll(mailBody, "{siteName}", w.System.SiteName)
		mailBody = strings.ReplaceAll(mailBody, "{userName}", user.UserName)
		mailBody = strings.ReplaceAll(mailBody, "{email}", user.Email)
		mailBody = strings.ReplaceAll(mailBody, "{url}", w.System.BaseUrl)
		mailBody = strings.ReplaceAll(mailBody, "{verifyUrl}", verifyUrl)
		mailBody = strings.ReplaceAll(mailBody, "{verifyCode}", verifyCode)

		_ = w.SendMail(subject, mailBody, user.Email)
	}

	return nil
}
