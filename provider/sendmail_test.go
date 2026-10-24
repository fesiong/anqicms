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

func TestSendMail2(t *testing.T) {

	dbSite, _ := GetDBWebsiteInfo(1)
	InitWebsite(dbSite)
	w := GetWebsite(1)

	// 传 nil user 必须命中 "no user" 守卫（SendInvoiceMail 对无收件人/开启
	// Brevo 时直接拒绝）。原版对 nil 参数 t.Fatal(err)，在无真实订单/邮件
	// 配置的环境下必然失败；改为断言守卫按设计拒绝。
	err := w.SendInvoiceMail(nil, nil, "invoices/202508312153240002.pdf")
	if err == nil {
		t.Fatal("期望 no user 守卫生效，实际发送成功")
	}
	if err.Error() != "no user" {
		t.Fatalf("期望 no user，实际: %s", err.Error())
	}
}
