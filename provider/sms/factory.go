package sms

import (
	"context"
	"fmt"

	"kandaoni.com/anqicms/config"
)

// NewSender 是发码流程拿 Sender 的唯一入口：任务 8 只通过它分派，不再直接调
// 各厂商构造函数。按 config.SmsProvider* 常量精确匹配，不做大小写／空白归一化。
//
// 未知值 fail closed：返回 error，且**不返回 nil sender**——调用方漏判 err 时
// 不会 nil panic，而是拿到一条 permanent 失败记录（unknownSender 永不出网）。
// 也绝不兜底成某一家：那会拿空凭据真的出网，按条计费。
// ⚠️ 新增服务商常量必须同步这里的 switch（factory_test.go 用 go/ast 双向穷举）。
func NewSender(provider string) (Sender, error) {
	switch provider {
	case config.SmsProviderAliyun:
		return NewAliyunSender(), nil
	case config.SmsProviderTencent:
		return NewTencentSender(), nil
	case config.SmsProviderTwilio:
		return NewTwilioSender(), nil
	default:
		return &unknownSender{provider: provider}, fmt.Errorf("未知短信服务商: %q", provider)
	}
}

// unknownSender 是未知 provider 的兜底发送器：不持有 http.Client，永不出网，
// Send 一律 permanent 失败并留下可读原因。
type unknownSender struct{ provider string }

func (s *unknownSender) Name() string { return s.provider }

func (s *unknownSender) Send(context.Context, Profile, SendRequest) (SendResult, error) {
	return SendResult{}, &SendError{
		Class:   ClassPermanent,
		Code:    "UnknownProvider",
		Message: "未知短信服务商: " + s.provider,
	}
}
