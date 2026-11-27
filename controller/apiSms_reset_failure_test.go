package controller

import (
	"fmt"
	"strings"
	"testing"
)

// smsResetFailureGuard 钉住 ApiSmsPasswordReset 的两个「改密失败」出口：
// EncryptPassword 与 DB.Save 失败必须回 SaveFailed 并打服务端日志，
// 不得走 respondSmsError —— 否则会被归约成「短信发送失败」，而此时验证码
// 已被消费、密码没改成，用户会去重发验证码，排查方向完全错。
//
// ReadJSON / 密码长度 / NormalizePhone / VerifySmsCode / 查用户 这 5 个前置
// 失败分支仍应走 respondSmsError（保持原有语义）。
func smsResetFailureGuard(src string) error {
	if n := strings.Count(src, "respondSmsError("); n != 5 {
		return fmt.Errorf("respondSmsError 调用应为 5 处（前置校验），实际 %d 处（改密失败被误报成发送失败？）", n)
	}
	if n := strings.Count(src, `currentSite.Tr("SaveFailed")`); n != 2 {
		return fmt.Errorf("改密失败出口应为 2 处 SaveFailed，实际 %d 处", n)
	}
	if n := strings.Count(src, "log.Println("); n != 2 {
		return fmt.Errorf("改密失败的服务端日志应为 2 处，实际 %d 处", n)
	}
	return nil
}

func TestSmsResetFailureNotReportedAsSendFailure(t *testing.T) {
	src := readFuncSource(t, "apiSms.go", "ApiSmsPasswordReset")
	if err := smsResetFailureGuard(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}

	// 反向对照：把一处 SaveFailed 换回 respondSmsError（旧行为）→ 必红。
	bad := strings.Replace(src, `currentSite.Tr("SaveFailed")`, "respondSmsError(ctx, currentSite, err)", 1)
	if bad == src {
		t.Fatal("反向对照准备失败：源码里没有 currentSite.Tr(\"SaveFailed\")")
	}
	if err := smsResetFailureGuard(bad); err == nil {
		t.Fatal("反向对照失败：改密失败退回 respondSmsError 后仍判绿")
	}
}
