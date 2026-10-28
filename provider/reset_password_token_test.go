package provider

import (
	"testing"

	"kandaoni.com/anqicms/library"
)

// 2026-10-28 修复的未授权重置漏洞：/api/password/reset 曾用 Md5(email+password)
// 当唯一校验凭据。空密码账号（Google / 公众号 / 邮箱验证码注册）password 为 ""，
// 凭据退化成 Md5(email)，任何人知道邮箱即可改密并登录。
// 因此凭据必须混入站点侧密钥（websites.token_secret，建站时随机生成且不下发）。
func TestResetPasswordTokenIsNotComputableFromEmailAlone(t *testing.T) {
	const email = "victim.google@gmail.com"
	const emptyHash = ""
	const secret = "fiynse8sl2kd9fj3kd93jf8sl2kd9fj3" // 32 位，与 token_secret 同形

	token := ResetPasswordToken(secret, email, emptyHash)

	if token == library.Md5(email) {
		t.Fatalf("空密码账号的凭据退化成 Md5(email)，知道邮箱即可重置密码")
	}
	if token == library.Md5(email+emptyHash) {
		t.Fatalf("凭据仍是 Md5(email+password)，没混入站点密钥")
	}
}

// 反向对照：本站算出的凭据不能在其他站点通用（密钥必须真的参与运算）。
func TestResetPasswordTokenDependsOnSiteSecret(t *testing.T) {
	a := ResetPasswordToken("secret-a", "u@example.com", "")
	b := ResetPasswordToken("secret-b", "u@example.com", "")

	if a == b {
		t.Fatalf("不同站点密钥产出同一凭据 %s，说明密钥没参与计算", a)
	}
}

// 发信端（provider.SendVerifyEmail）与校验端（ApiResetUserPassword）必须算出同一凭据，
// 否则正常用户点邮件链接后永远重置失败。任何引入时间戳/随机数的实现都会在这里变红。
func TestResetPasswordTokenIsStableForSameSiteAndUser(t *testing.T) {
	const (
		secret = "s"
		email  = "u@example.com"
		hash   = "$2a$12$abcdefghijklmnopqrstuv"
	)

	a := ResetPasswordToken(secret, email, hash)
	b := ResetPasswordToken(secret, email, hash)

	if a != b {
		t.Fatalf("同一用户两次生成凭据不一致：%s vs %s", a, b)
	}
}
