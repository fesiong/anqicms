package provider

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
)

// 本文件是飞书 / 钉钉真实平台联调的入口，默认不联网。
//
// 门禁：CHANNEL_LIVE_TEST=1 才执行，未设置（或取其它值）一律 t.Skip，普通 go test
// 永远不会访问平台。凭据全部来自环境变量，缺项时逐项打印变量名 —— 只打印名字，绝不打印值。
//
// 自动能断言的只有四项：不存在的 AppId/Secret 被平台拒绝且错误不回显密钥、主动群发、
// 主动私聊、长文本分段发送。接收文本、私聊绑定、群 @ 隔离、确认卡片、断线重连、权限撤销
// 必须在平台侧人工核对，这里只打印清单，不用睡眠或伪造回调把它们伪装成自动通过。
const (
	channelLiveGateEnv  = "CHANNEL_LIVE_TEST"
	channelLiveText     = "AnQiCMS 联调测试"
	channelLiveTimeout  = 90 * time.Second
	channelLiveLongRune = 3800
)

// TestChannelLiveFeishu 是飞书真实租户联调。默认跳过，见文件头说明。
func TestChannelLiveFeishu(t *testing.T) {
	channelLiveGuard(t)
	channelLiveManualChecklist(t, "飞书")

	env := channelLiveEnv(t,
		"FEISHU_APP_ID",
		"FEISHU_APP_SECRET",
		"FEISHU_ORG_ID",
		"FEISHU_TEST_CHAT_ID",
		"FEISHU_TEST_OPEN_ID",
	)

	creds := PlatformCredentials{
		AppId:          env["FEISHU_APP_ID"],
		AppSecret:      env["FEISHU_APP_SECRET"],
		OrganizationId: env["FEISHU_ORG_ID"],
	}
	adapter, ok := NewPlatformAdapter(model.ChannelPlatformFeishu, creds)
	if !ok {
		t.Fatalf("飞书适配器构造失败：凭据非空却返回 false")
	}

	// 1. 错误凭据：同一平台、不存在的 AppId/Secret，必须被平台拒绝。
	//    分类断言放宽为「只断言被拒绝 + 不回显密钥」：飞书 token 端点可能以 HTTP 200 +
	//    未登记的业务码返回，现有分类器会判 transient，平台行为不由本测试裁决。
	channelLiveAssertRejectedCredentials(t, model.ChannelPlatformFeishu, PlatformCredentials{
		AppId:          "cli_live_probe_invalid_app",
		AppSecret:      "live-probe-invalid-secret",
		OrganizationId: creds.OrganizationId,
	}, "", "live-probe-invalid-secret", creds.AppSecret)

	// 2. 主动群发送（chat_id）。
	group := TargetRef{Kind: TargetKindGroup, PlatformTargetId: env["FEISHU_TEST_CHAT_ID"]}
	groupResult := channelLiveSend(t, adapter, group, channelLiveText, "飞书群聊短文本", creds.AppSecret)
	t.Logf("飞书群聊短文本发送成功，平台是否返回 message_id：%t（需人工在群内核对）", groupResult.PlatformMessageId != "")

	// 3. 主动私聊（open_id）。
	private := TargetRef{Kind: TargetKindPrivate, PlatformTargetId: env["FEISHU_TEST_OPEN_ID"]}
	privateResult := channelLiveSend(t, adapter, private, channelLiveText, "飞书私聊短文本", creds.AppSecret)
	t.Logf("飞书私聊短文本发送成功，平台是否返回 message_id：%t（需人工在私聊窗口核对）", privateResult.PlatformMessageId != "")

	// 4. 长文本：3800 rune 超过 feishuMaxTextRunes（3500），必然分两段发出。
	channelLiveSend(t, adapter, group, channelLiveLongText(channelLiveLongRune), "飞书群聊长文本", creds.AppSecret)
	t.Log("飞书群聊长文本发送成功；分段顺序与平台侧截断展示需人工核对（本测试不断言）")
}

// TestChannelLiveDingTalk 是钉钉真实企业联调。默认跳过，见文件头说明。
func TestChannelLiveDingTalk(t *testing.T) {
	channelLiveGuard(t)
	channelLiveManualChecklist(t, "钉钉")

	env := channelLiveEnv(t,
		"DINGTALK_APP_KEY",
		"DINGTALK_APP_SECRET",
		"DINGTALK_CORP_ID",
		"DINGTALK_TEST_CONVERSATION_ID",
		"DINGTALK_TEST_USER_ID",
	)

	creds := PlatformCredentials{
		AppId:          env["DINGTALK_APP_KEY"],
		AppSecret:      env["DINGTALK_APP_SECRET"],
		OrganizationId: env["DINGTALK_CORP_ID"],
	}
	adapter, ok := NewPlatformAdapter(model.ChannelPlatformDingTalk, creds)
	if !ok {
		t.Fatalf("钉钉适配器构造失败：凭据非空却返回 false")
	}

	// 1. 错误凭据：钉钉取 accessToken 端点对不存在的 appKey 返回 4xx，分类必然是 permanent，
	//    这里连分类一起断言（分类器允许，就不放宽）。
	channelLiveAssertRejectedCredentials(t, model.ChannelPlatformDingTalk, PlatformCredentials{
		AppId:          "live-probe-invalid-appkey",
		AppSecret:      "live-probe-invalid-secret",
		OrganizationId: creds.OrganizationId,
	}, ErrorClassPermanent, "live-probe-invalid-secret", creds.AppSecret)

	// 2. 主动群发送（openConversationId）。
	group := TargetRef{Kind: TargetKindGroup, PlatformTargetId: env["DINGTALK_TEST_CONVERSATION_ID"]}
	groupResult := channelLiveSend(t, adapter, group, channelLiveText, "钉钉群聊短文本", creds.AppSecret)
	t.Logf("钉钉群聊短文本发送成功，平台是否返回回执：%t（需人工在群内核对）", groupResult.PlatformMessageId != "")

	// 3. 主动私聊（staffId / userId）。
	private := TargetRef{Kind: TargetKindPrivate, PlatformTargetId: env["DINGTALK_TEST_USER_ID"]}
	privateResult := channelLiveSend(t, adapter, private, channelLiveText, "钉钉私聊短文本", creds.AppSecret)
	t.Logf("钉钉私聊短文本发送成功，平台是否返回回执：%t（需人工在私聊窗口核对）", privateResult.PlatformMessageId != "")

	// 4. 长文本：3800 rune 超过 dingTalkMaxTextRunes（1800），必然分三段发出。
	channelLiveSend(t, adapter, group, channelLiveLongText(channelLiveLongRune), "钉钉群聊长文本", creds.AppSecret)
	t.Log("钉钉群聊长文本发送成功；分段顺序与平台侧截断展示需人工核对（本测试不断言）")
}

// channelLiveGuard 是顶层门禁：没有显式开启就跳过，保证普通 go test 不联网。
func channelLiveGuard(t *testing.T) {
	t.Helper()
	if os.Getenv(channelLiveGateEnv) != "1" {
		t.Skipf("设置 %s=1 才执行真实联调（默认不联网）", channelLiveGateEnv)
	}
}

// channelLiveEnv 收集必需的环境变量。缺失时逐项打印变量名后跳过；
// 打印的只有变量名，凭据值不落任何日志。
func channelLiveEnv(t *testing.T, names ...string) map[string]string {
	t.Helper()
	values := make(map[string]string, len(names))
	missing := make([]string, 0, len(names))
	for _, name := range names {
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			missing = append(missing, name)
			continue
		}
		values[name] = value
	}
	if len(missing) == 0 {
		return values
	}
	for _, name := range missing {
		t.Logf("缺少环境变量：%s", name)
	}
	t.Skipf("缺少 %d 个联调环境变量，跳过真实联调（只列出变量名，不打印值）", len(missing))
	return nil
}

// channelLiveManualChecklist 打印只能在平台侧人工核对的清单。
// 这些项没有可自动化的判据，绝不用睡眠或伪造回调冒充通过。
func channelLiveManualChecklist(t *testing.T, platform string) {
	t.Helper()
	t.Logf("%s 联调人工核对清单（需人工在平台侧核对，本测试不断言通过）：", platform)
	for _, item := range []string{
		"接收文本：在平台侧给机器人发文本，后台运行记录应出现正确正文与发送人。",
		"私聊绑定：用一次性绑定 token 完成绑定，之后发消息应命中已绑定身份。",
		"群 @ 隔离：未 @ 机器人的群消息不得触发回复，@ 机器人后应触发。",
		"确认卡片：高危操作弹出确认卡片，通过 / 拒绝 / 重复点击各一次，结果与去重须正确。",
		"断线重连：断网或重启 Stream 服务后，连接应按退避重连并在恢复后继续收消息。",
		"权限撤销：在平台撤回应用权限后，渠道应转入错误状态并停止处理消息。",
	} {
		t.Logf("  - %s", item)
	}
}

// channelLiveAssertRejectedCredentials 用同一平台但不存在的 AppId/Secret 发送，
// 断言被拒绝、错误形态统一（*SendError）且错误文本不含任何密钥原文。
// wantClass 为空的平台只断言「被拒绝 + 不回显密钥」，非空时连同分类一起断言。
func channelLiveAssertRejectedCredentials(t *testing.T, platform string, creds PlatformCredentials, wantClass ErrorClass, secrets ...string) {
	t.Helper()

	adapter, ok := NewPlatformAdapter(platform, creds)
	if !ok {
		t.Fatalf("%s 探针适配器构造失败：探针凭据必须非空", platform)
	}
	ctx, cancel := context.WithTimeout(context.Background(), channelLiveTimeout)
	defer cancel()

	_, err := adapter.Send(ctx, TargetRef{
		Kind:             TargetKindPrivate,
		PlatformTargetId: "live_probe_target_invalid",
	}, OutboundMessage{Text: channelLiveText})
	if err == nil {
		t.Fatalf("%s 使用不存在的 AppId/Secret 发送竟然成功", platform)
	}
	for _, secret := range secrets {
		if secret != "" && strings.Contains(err.Error(), secret) {
			// 连错误文本都不打印：它已经含密钥原文。
			t.Fatalf("%s 发送错误里出现了密钥原文，已中止", platform)
		}
	}
	var sendErr *SendError
	if !errors.As(err, &sendErr) {
		t.Fatalf("%s 错误形态不是 *SendError，分类无法判定：%T", platform, err)
	}
	if strings.TrimSpace(sendErr.Message) == "" {
		t.Fatalf("%s 拒绝错误的文案为空", platform)
	}
	if wantClass != "" && sendErr.Class != wantClass {
		t.Fatalf("%s 拒绝错误的分类为 %s，期望 %s（文案：%s）",
			platform, sendErr.Class, wantClass, channelLiveRedact(sendErr.Message, secrets...))
	}
	t.Logf("%s 不存在的 AppId/Secret 被拒绝，错误分类=%s", platform, sendErr.Class)
}

// channelLiveSend 发一条文本并断言成功。失败文案过一遍脱敏，绝不输出密钥。
func channelLiveSend(t *testing.T, adapter PlatformAdapter, target TargetRef, text, label string, secrets ...string) SendResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), channelLiveTimeout)
	defer cancel()
	result, err := adapter.Send(ctx, target, OutboundMessage{Text: text})
	if err == nil {
		return result
	}
	var sendErr *SendError
	if errors.As(err, &sendErr) {
		t.Fatalf("%s 发送失败（分类=%s）：%s", label, sendErr.Class, channelLiveRedact(sendErr.Message, secrets...))
	}
	t.Fatalf("%s 发送失败：%s", label, channelLiveRedact(err.Error(), secrets...))
	return SendResult{}
}

// channelLiveRedact 把已知密钥替换成占位符，供测试失败输出使用。
func channelLiveRedact(text string, secrets ...string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		text = strings.ReplaceAll(text, secret, "<脱敏>")
	}
	return text
}

// channelLiveLongText 生成指定 rune 数的长文本（按句重复，保持可读）。
func channelLiveLongText(runes int) string {
	seed := []rune("AnQiCMS 长文本联调测试：分段顺序与平台截断须在平台侧人工核对。")
	out := make([]rune, 0, runes)
	for len(out) < runes {
		out = append(out, seed...)
	}
	return string(out[:runes])
}
