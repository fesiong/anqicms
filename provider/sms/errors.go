package sms

import (
	"errors"
	"fmt"
)

// ErrorClass 是平台发送失败的分类（规格 §10.1）。发码接口不自动重试：
// 短信按条计费，transient 只在 sms_logs.retryable 打标，由站长决定是否重发。
type ErrorClass string

const (
	ClassPermanent ErrorClass = "permanent"
	ClassTransient ErrorClass = "transient"
)

// SendError 包装平台发送失败，携带分类与平台原始错误码／消息。
// 平台原始错误码与消息只进 sms_logs，不回给前台用户（规格 §10.2）。
type SendError struct {
	Class   ErrorClass
	Code    string
	Message string
}

func (e *SendError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Class, e.Code, e.Message)
}

// Unwrap 让下游只用 errors.Is(err, ErrSmsSendFailed) 就能落到通用翻译文案，
// 不必依赖每个 controller 记得做类型分派（裁决 R10）。
// 需要平台原始码落日志时仍走 errors.As 取 *SendError——Error() 的格式因此不变。
func (e *SendError) Unwrap() error { return ErrSmsSendFailed }

// ClassifyHTTPStatus 按 HTTP 状态码分类（规格 §10.1）：
// 429／408／425 与 ≥500 → transient；其余 4xx → permanent；
// 2xx／3xx 及未知一律 transient。
func ClassifyHTTPStatus(status int) ErrorClass {
	switch status {
	case 408, 425, 429:
		return ClassTransient
	}
	if status >= 500 {
		return ClassTransient
	}
	if status >= 400 && status < 500 {
		return ClassPermanent
	}
	return ClassTransient
}

// 业务错误哨兵。Error() 字符串就是多语言翻译键本身：provider/sms 拿不到
// *Website，翻译在 controller 层用 w.Tr(key, args...) 做，缺键时回落返回键名。
// 键名与规格 §10.2 表格逐行对应，文案在 locales/{zh-CN,zh-TW,en-US}/default.yml。
var (
	ErrSmsServiceNotEnabled = errors.New("SmsServiceNotEnabled")
	ErrSmsSceneNotEnabled   = errors.New("SmsSceneNotEnabled")
	ErrSmsPhoneInvalid      = errors.New("SmsPhoneInvalid")
	ErrSmsRegionUnsupported = errors.New("SmsRegionUnsupported")
	ErrSmsCredentialMissing = errors.New("SmsCredentialMissing")
	ErrSmsSignMissing       = errors.New("SmsSignMissing")
	ErrSmsTemplateMissing   = errors.New("SmsTemplateMissing")
	ErrSmsIpLimited         = errors.New("SmsIpLimited")
	ErrSmsPhoneLimited      = errors.New("SmsPhoneLimited")
	ErrSmsResendTooSoon     = errors.New("SmsResendTooSoon")
	ErrSmsCodeNotFound      = errors.New("SmsCodeNotFound")
	ErrSmsCodeExpired       = errors.New("SmsCodeExpired")
	ErrSmsCodeMismatch      = errors.New("SmsCodeMismatch")
	ErrSmsCodeLocked        = errors.New("SmsCodeLocked")
	ErrSmsSendFailed        = errors.New("SmsSendFailed")
)

// WaitError 携带重发等待秒数。Error() 仍返回翻译键，
// 数值由调用方 errors.As 取出后作为 Tr 的 %d 参数。
type WaitError struct {
	Seconds int
}

func (e *WaitError) Error() string { return ErrSmsResendTooSoon.Error() }
func (e *WaitError) Unwrap() error { return ErrSmsResendTooSoon }

// AttemptsError 携带剩余尝试次数。Error() 仍返回翻译键，
// 数值由调用方 errors.As 取出后作为 Tr 的 %d 参数。
type AttemptsError struct {
	Remaining int
}

func (e *AttemptsError) Error() string { return ErrSmsCodeMismatch.Error() }
func (e *AttemptsError) Unwrap() error { return ErrSmsCodeMismatch }
