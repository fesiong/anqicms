package provider

import (
	"testing"
)

func TestSendMail(t *testing.T) {
	subject := "测试邮件"
	content := "这是一封测试邮件。收到邮件表示配置正常"

	err := SendMail(subject, content)
	if err != nil {
		t.Fatal(err)
	}
}
