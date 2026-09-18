package provider

import (
	"testing"
)

func TestSendMail(t *testing.T) {
	subject := "测试邮件"
	content := "这是一封测试邮件。收到邮件表示配置正常"

	dbSite, _ := GetDBWebsiteInfo(1)
	dbSite.Status = 1
	InitWebsite(dbSite)
	w := GetWebsite(1)

	err := w.SendMail(subject, content, nil)
	if err != nil {
		t.Fatal(err)
	}
}
