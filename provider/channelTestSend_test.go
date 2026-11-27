package provider

import (
	"strings"
	"testing"
)

// 本文件锁定 R94 试发的「诚实语义」（T12b R1）：
//
//	ChannelTestSend 在没有真正连过平台时不能说自己 valid。
//	targetId==0 只检查必填字段与适配器可构造，必须回 Valid=false/Verified=false 并给出
//	「未连接平台验证」的说明；只有真的发出一条消息（成功，或拿到平台返回的分类失败）
//	才算 Verified=true。
//
// 前端 connect.tsx 依赖 verified===false 先于 class/sent 判断，因此失败分支必须是
// Verified=true：否则「凭据被平台拒绝」会被误报成「未验证」。

// injectChannelTestAdapter 替换试发适配器工厂，用例结束恢复。
func injectChannelTestAdapter(t *testing.T, adapter PlatformAdapter) *notifyFakeAdapter {
	t.Helper()
	original := channelTestAdapterFactory
	channelTestAdapterFactory = func(string, PlatformCredentials) (PlatformAdapter, bool) {
		return adapter, true
	}
	t.Cleanup(func() { channelTestAdapterFactory = original })
	return adapter.(*notifyFakeAdapter)
}

// TestChannelTestSendOnlyChecksRequiredFields 锁住「没连平台就别说验证过」。
func TestChannelTestSendOnlyChecksRequiredFields(t *testing.T) {
	w := channelDataTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	fake := injectChannelTestAdapter(t, newNotifyFakeAdapter())

	result, err := w.ChannelTestSend(ch.Id, 0)
	if err != nil {
		t.Fatalf("只校验必填项不应报错：%v", err)
	}
	if result == nil {
		t.Fatal("只校验必填项必须返回回执")
	}
	// 反向对照：把返回值改回 Valid:true（旧行为）时这条断言必红 ——
	// 没有发起任何平台请求的回执不得声称配置有效。
	if result.Valid {
		t.Fatalf("没有连过平台的回执不得返回 valid=true：%+v", result)
	}
	if result.Verified {
		t.Fatalf("没有连过平台的回执不得返回 verified=true：%+v", result)
	}
	if result.Sent {
		t.Fatalf("只校验必填项不得标记已发送：%+v", result)
	}
	if strings.TrimSpace(result.Message) == "" {
		t.Fatal("未验证的回执必须说明原因，不能让界面显示空白")
	}
	// 文案不得出现「有效/valid」：这是本任务的核心诉求（前端不得再提示「配置校验通过」）。
	if strings.Contains(result.Message, "有效") || strings.Contains(strings.ToLower(result.Message), "valid") {
		t.Fatalf("未验证的回执文案不得暗示有效性：%q", result.Message)
	}
	if fake.callCount() != 0 {
		t.Fatalf("只校验必填项不得调用平台发送，实际 %d 次", fake.callCount())
	}
}

// TestChannelTestSendVerifiedOnlyAfterRealSend 锁住「真发了才叫验证过」。
func TestChannelTestSendVerifiedOnlyAfterRealSend(t *testing.T) {
	w := channelDataTestSite(t)
	ch := newManagerTestChannel(t, w, true)
	fake := injectChannelTestAdapter(t, newNotifyFakeAdapter())

	target := channelDataTargetRow(t, w, w.Id, ch.Id, TargetKindGroup, "test_verified_"+channelDataSuffix(), 1, 0)

	result, err := w.ChannelTestSend(ch.Id, target.Id)
	if err != nil {
		t.Fatalf("试发失败：%v", err)
	}
	if !result.Valid || !result.Verified || !result.Sent {
		t.Fatalf("发送成功的回执必须是 valid+verified+sent：%+v", result)
	}
	if fake.callCount() != 1 {
		t.Fatalf("验证只来自真实发送：期望 1 次调用，实际 %d 次", fake.callCount())
	}

	// 平台拒绝也算验证过（凭据被真实平台判过），只是 valid=false：
	// 前端靠 verified 区分「未验证」与「验证失败」。
	errorFake := newNotifyFakeAdapter(notifyFakeResponse{
		err: NewSendError(ErrorClassPermanent, "凭据被拒绝"),
	})
	channelTestAdapterFactory = func(string, PlatformCredentials) (PlatformAdapter, bool) {
		return errorFake, true
	}
	result, err = w.ChannelTestSend(ch.Id, target.Id)
	if err != nil {
		t.Fatalf("平台失败不得变成接口失败：%v", err)
	}
	if result.Valid {
		t.Fatalf("平台判定失败时不得返回 valid=true：%+v", result)
	}
	if !result.Verified {
		t.Fatalf("平台给出分类失败也是验证过：verified 必须为 true，否则界面会误报「未验证」：%+v", result)
	}
	if result.Sent || result.Class != string(ErrorClassPermanent) {
		t.Fatalf("失败回执必须保留分类且不标记已发送：%+v", result)
	}
}
