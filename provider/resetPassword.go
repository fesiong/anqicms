package provider

import "kandaoni.com/anqicms/library"

// ResetPasswordToken 生成邮件验证 / 密码重置链接里的校验凭据。
//
// ⚠️ 必须混入站点 TokenSecret。历史实现是 Md5(email+password)，而第三方登录
// （Google / 公众号 / 小程序）以及邮箱验证码快速注册的账号 password 是空串，
// 凭据直接退化成 Md5(email)：任何人知道受害者邮箱就能调用
// POST /api/password/reset 改密并登录（2026-10-28 修复的未授权重置漏洞）。
//
// 发信端 provider.SendVerifyEmail 与校验端 controller.ApiResetUserPassword
// 必须都走本函数；两端算法不一致会让正常用户点邮件链接后永远重置失败。
func ResetPasswordToken(secret, email, passwordHash string) string {
	return library.Md5(secret + email + passwordHash)
}
