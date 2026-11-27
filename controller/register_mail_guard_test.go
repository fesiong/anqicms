package controller

import (
	"strings"
	"testing"
)

// 注册流程的验证邮件由 provider.RegisterUser 发给用户。
// ApiRegister 在未验证分支里再调用一次 SendMail(..., nil)（无收件人=管理员），
// 会让管理员额外收到一封“请验证邮箱”，与用户注册无关。
func TestApiRegisterDoesNotSendExtraAdminMail(t *testing.T) {
	src := readFuncSource(t, "apiUser.go", "ApiRegister")

	if strings.Contains(src, "SendMail(") {
		t.Error("ApiRegister 不应调用 SendMail：验证邮件已由 RegisterUser 发出，这里再发会让管理员多收一封")
	}
	if !strings.Contains(src, "PleaseVerifyEmail") {
		t.Error("ApiRegister 仍应返回 PleaseVerifyEmail 提示，不能连用户提示一起删掉")
	}
}
