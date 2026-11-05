package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/card"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
)

func channelStringPtr(s string) *string { return &s }

// TestInboundEventRequiresVerifiedIdentity 覆盖入站事件的身份闸门：
// 事件/用户/会话标识缺失、应用或组织与渠道配置不一致、会话类型非法都必须被拒绝，
// 且错误文案不得回显任何标识值（错误会进日志）。
func TestInboundEventRequiresVerifiedIdentity(t *testing.T) {
	const (
		expectedAppId = "cli_test_app"
		expectedOrgId = "org_test_tenant"
	)
	base := InboundEvent{
		EventId:        "om_evt_1",
		Kind:           InboundKindText,
		OrganizationId: expectedOrgId,
		AppId:          expectedAppId,
		UserId:         "ou_user_1",
		ChatId:         "oc_chat_1",
		ChatType:       ChatTypeP2P,
		Text:           "你好",
	}

	cases := []struct {
		name    string
		mutate  func(*InboundEvent)
		wantErr string
	}{
		{name: "合法私聊事件通过", mutate: func(*InboundEvent) {}, wantErr: ""},
		{
			name:    "合法群聊事件通过",
			mutate:  func(e *InboundEvent) { e.ChatType = ChatTypeGroup },
			wantErr: "",
		},
		{
			name:    "空事件标识被拒绝",
			mutate:  func(e *InboundEvent) { e.EventId = "   " },
			wantErr: "事件标识",
		},
		{
			name:    "空平台用户标识被拒绝",
			mutate:  func(e *InboundEvent) { e.UserId = "" },
			wantErr: "用户标识",
		},
		{
			name:    "空会话标识被拒绝",
			mutate:  func(e *InboundEvent) { e.ChatId = "" },
			wantErr: "会话标识",
		},
		{
			name:    "应用与渠道配置不一致被拒绝",
			mutate:  func(e *InboundEvent) { e.AppId = "cli_other_app" },
			wantErr: "应用",
		},
		{
			name:    "组织与渠道配置不一致被拒绝",
			mutate:  func(e *InboundEvent) { e.OrganizationId = "org_other_tenant" },
			wantErr: "组织",
		},
		{
			name:    "组织为非预期空串被拒绝",
			mutate:  func(e *InboundEvent) { e.OrganizationId = "" },
			wantErr: "组织",
		},
		{
			name:    "非法会话类型被拒绝",
			mutate:  func(e *InboundEvent) { e.ChatType = "topic" },
			wantErr: "会话类型",
		},
		{
			name:    "空会话类型被拒绝",
			mutate:  func(e *InboundEvent) { e.ChatType = "" },
			wantErr: "会话类型",
		},
	}

	// 反向对照：期望文案必须来自真实产出，不是凭空写的断言。
	// 先跑一遍非法用例，确认校验器真的会失败，避免所有用例共用同一空转路径。
	failed := 0
	for _, tc := range cases {
		event := base
		tc.mutate(&event)
		err := event.Validate(expectedAppId, expectedOrgId)
		if tc.wantErr == "" && err != nil {
			t.Errorf("%s：期望通过，实际报错 %v", tc.name, err)
			continue
		}
		if tc.wantErr == "" {
			continue
		}
		if err == nil {
			t.Errorf("%s：期望报错，实际通过", tc.name)
			continue
		}
		failed++
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s：错误文案缺少「%s」，实际为「%s」", tc.name, tc.wantErr, err.Error())
		}
		// 错误文案会进日志：不得回显任何身份标识值。
		for _, secret := range []string{expectedAppId, expectedOrgId, base.UserId, base.ChatId} {
			if secret != "" && strings.Contains(err.Error(), secret) {
				t.Errorf("%s：错误文案回显了标识值「%s」", tc.name, secret)
			}
		}
	}
	if failed < 5 {
		t.Fatalf("校验器实际拒绝的用例只有 %d 个，期望至少 5 个：断言可能在空转", failed)
	}
}

// TestSendErrorClassifiesAuthAsPermanent 固化错误分类规则：
// 鉴权/权限与本地校验 → permanent；限流/5xx/网络/超时/未知 → transient。
func TestSendErrorClassifiesAuthAsPermanent(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want ErrorClass
	}{
		{
			name: "飞书 HTTP 401 鉴权失败为永久错误",
			err:  newPlatformResponseError("feishu", 401, 0, "unauthorized"),
			want: ErrorClassPermanent,
		},
		{
			name: "飞书 HTTP 403 权限不足为永久错误",
			err:  newPlatformResponseError("feishu", 403, 0, "forbidden"),
			want: ErrorClassPermanent,
		},
		{
			name: "钉钉 HTTP 400 参数错误为永久错误",
			err:  newPlatformResponseError("dingtalk", 400, 0, "invalid robotCode"),
			want: ErrorClassPermanent,
		},
		{
			name: "飞书令牌业务码（HTTP 200）为永久错误",
			err:  newPlatformResponseError("feishu", 200, 99991663, "invalid access token"),
			want: ErrorClassPermanent,
		},
		{
			name: "飞书 SDK CodeError 鉴权码为永久错误",
			err:  larkcore.CodeError{Code: 99991663, Msg: "invalid access token"},
			want: ErrorClassPermanent,
		},
		{
			name: "飞书 429 限流为可重试",
			err:  newPlatformResponseError("feishu", 429, 0, "too many requests"),
			want: ErrorClassTransient,
		},
		{
			name: "飞书 500 服务端错误为可重试",
			err:  newPlatformResponseError("feishu", 500, 0, "internal error"),
			want: ErrorClassTransient,
		},
		{
			name: "钉钉 503 服务端错误为可重试",
			err:  newPlatformResponseError("dingtalk", 503, 0, "service unavailable"),
			want: ErrorClassTransient,
		},
		{
			name: "网络拨号失败为可重试",
			err:  &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
			want: ErrorClassTransient,
		},
		{
			name: "url.Error 包装的 IO 失败为可重试",
			err:  &url.Error{Op: "Post", URL: "https://open.feishu.cn", Err: errors.New("i/o timeout")},
			want: ErrorClassTransient,
		},
		{
			name: "请求超时为可重试",
			err:  context.DeadlineExceeded,
			want: ErrorClassTransient,
		},
		{
			name: "上下文取消为可重试（调用方可直接停止）",
			err:  context.Canceled,
			want: ErrorClassTransient,
		},
		{
			name: "未知错误默认可重试",
			err:  errors.New("boom"),
			want: ErrorClassTransient,
		},
		{
			name: "已是 SendError 时原样返回",
			err:  &SendError{Class: ErrorClassPermanent, Message: "本地校验失败"},
			want: ErrorClassPermanent,
		},
	}

	for _, tc := range cases {
		got := ClassifyPlatformSendError(tc.err)
		if got == nil {
			t.Errorf("%s：期望返回分类结果，实际为 nil", tc.name)
			continue
		}
		if got.Class != tc.want {
			t.Errorf("%s：期望 %s，实际 %s（%s）", tc.name, tc.want, got.Class, got.Message)
		}
		if strings.TrimSpace(got.Message) == "" {
			t.Errorf("%s：分类结果缺少可诊断信息", tc.name)
		}
	}
	if ClassifyPlatformSendError(nil) != nil {
		t.Errorf("nil 错误不应产生分类结果")
	}
	// 反向对照：未知错误必须是 transient 而不是 permanent，否则一次抖动就永久丢消息。
	if ClassifyPlatformSendError(errors.New("boom")).Class == ErrorClassPermanent {
		t.Fatalf("未知错误被分类为 permanent，规则已失效")
	}
}

// TestInboundEventValidateIdentityIgnoresChatFields 固化卡片回调走的身份闸门：
// ValidateIdentity 只看应用与企业两项，不因 ChatId/ChatType 为空而拒绝（卡片回调本来就没有会话）。
func TestInboundEventValidateIdentityIgnoresChatFields(t *testing.T) {
	const (
		expectedAppId = "ding_test_app"
		expectedOrgId = "corp_1"
	)
	base := InboundEvent{
		EventId:        "card_instance_1:approve",
		Kind:           InboundKindCardAction,
		OrganizationId: expectedOrgId,
		AppId:          expectedAppId,
		UserId:         "staff_1",
		Text:           "approve",
	}
	if err := base.ValidateIdentity(expectedAppId, expectedOrgId); err != nil {
		t.Fatalf("合法卡片事件未通过身份校验：%v", err)
	}
	// 同一事件走完整 Validate 必然被 ChatId 拒绝：这正是卡片回调要单独走 ValidateIdentity 的原因。
	if err := base.Validate(expectedAppId, expectedOrgId); err == nil {
		t.Fatalf("缺会话标识的事件不应通过完整 Validate")
	}

	cases := []struct {
		name    string
		mutate  func(*InboundEvent)
		wantErr string
	}{
		{name: "应用为空被拒绝", mutate: func(e *InboundEvent) { e.AppId = " " }, wantErr: "应用"},
		{name: "应用不一致被拒绝", mutate: func(e *InboundEvent) { e.AppId = "ding_other" }, wantErr: "应用"},
		{name: "企业为空被拒绝", mutate: func(e *InboundEvent) { e.OrganizationId = "" }, wantErr: "组织"},
		{name: "企业不一致被拒绝", mutate: func(e *InboundEvent) { e.OrganizationId = "corp_2" }, wantErr: "组织"},
	}
	failed := 0
	for _, tc := range cases {
		event := base
		tc.mutate(&event)
		err := event.ValidateIdentity(expectedAppId, expectedOrgId)
		if err == nil {
			t.Errorf("%s：期望报错，实际通过", tc.name)
			continue
		}
		failed++
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s：错误文案缺少「%s」，实际为「%s」", tc.name, tc.wantErr, err.Error())
		}
		for _, secret := range []string{expectedAppId, expectedOrgId} {
			if strings.Contains(err.Error(), secret) {
				t.Errorf("%s：错误文案回显了标识值「%s」", tc.name, secret)
			}
		}
	}
	if failed < 4 {
		t.Fatalf("身份校验实际拒绝的用例只有 %d 个，期望 4 个：断言可能在空转", failed)
	}
}

// TestAdapterRegistryRejectsUnknownPlatform 固化平台注册表：只认飞书与钉钉，
// 且唯一的构造入口 NewPlatformAdapter 必须要求凭据——不存在「无凭据原型」这条恒失败路径。
func TestAdapterRegistryRejectsUnknownPlatform(t *testing.T) {
	validCreds := PlatformCredentials{AppId: "app_1", AppSecret: "secret_1", OrganizationId: "org_1"}
	for _, name := range []string{"feishu", "dingtalk"} {
		adapter, ok := NewPlatformAdapter(name, validCreds)
		if !ok || adapter == nil {
			t.Fatalf("%s 适配器未注册", name)
		}
		if adapter.Name() != name {
			t.Errorf("适配器 %s 的 Name() 返回 %q，与注册键不一致", name, adapter.Name())
		}
		// 工厂产出的实例必须是「已绑定凭据」的：空正文的拒绝理由应是内容为空，
		// 而不是「未绑定凭据」这类永远发不出去的原型态。
		_, err := adapter.Send(context.Background(), TargetRef{Kind: TargetKindGroup, PlatformTargetId: "target_1"}, OutboundMessage{})
		if err == nil {
			t.Errorf("%s 空正文未返回错误", name)
		} else {
			if strings.Contains(err.Error(), "未绑定") || strings.Contains(err.Error(), "unbound") {
				t.Errorf("%s 工厂返回了无凭据原型：%v", name, err)
			}
			if !strings.Contains(err.Error(), "内容为空") {
				t.Errorf("%s 空正文的错误文案异常：%v", name, err)
			}
		}
	}

	// 未知平台与空白名称都不兜底。
	for _, name := range []string{"wechat", "wecom", "lark", "", "   ", "slack"} {
		if adapter, ok := NewPlatformAdapter(name, validCreds); ok || adapter != nil {
			t.Errorf("未支持平台 %q 不应注册，实际返回 %v", name, adapter)
		}
	}

	// 构造即要求凭据：缺 AppId 或 AppSecret 一律 false，杜绝「能构造但永远发不出去」的实例。
	for _, creds := range []PlatformCredentials{
		{},
		{AppId: "app_1"},
		{AppSecret: "secret_1"},
		{AppId: "  ", AppSecret: "  "},
	} {
		for _, name := range []string{"feishu", "dingtalk"} {
			if adapter, ok := NewPlatformAdapter(name, creds); ok || adapter != nil {
				t.Errorf("%s 在凭据不完整时不应构造成功（AppId=%q AppSecret=%q）", name, creds.AppId, creds.AppSecret)
			}
		}
	}

	names := PlatformNames()
	want := []string{"dingtalk", "feishu"}
	if len(names) != len(want) {
		t.Fatalf("PlatformNames() = %v，期望 %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("PlatformNames() = %v，期望固定顺序 %v", names, want)
		}
	}

	// 目标类型常量必须与 T2 已落库的取值同值：库里的 private 与适配器的 TargetKindPrivate
	// 一旦漂移，私聊发送会全部落进「不支持的发送目标类型」。
	if TargetKindPrivate != channelTargetKindPrivate {
		t.Fatalf("TargetKindPrivate = %q，与 channelBinding.go 的 %q 不一致", TargetKindPrivate, channelTargetKindPrivate)
	}
	if TargetKindGroup != "group" {
		t.Fatalf("TargetKindGroup = %q，期望 group", TargetKindGroup)
	}
	// 会话类型与平台原始取值必须一致，否则映射层要额外翻译。
	if ChatTypeP2P != "p2p" || ChatTypeGroup != "group" {
		t.Fatalf("会话类型常量漂移：p2p=%q group=%q", ChatTypeP2P, ChatTypeGroup)
	}
}

// TestPlatformIdentityHashIsSiteScoped 证明身份摘要复用 channelDigest 且站点参与摘要。
func TestPlatformIdentityHashIsSiteScoped(t *testing.T) {
	const (
		platform  = "feishu"
		orgId     = "tenant_1"
		userId    = "ou_user_1"
		channelId = uint(7)
	)

	siteA := PlatformIdentityHash(1, channelId, platform, orgId, userId)
	siteB := PlatformIdentityHash(2, channelId, platform, orgId, userId)
	if siteA == siteB {
		t.Fatalf("不同站点的同一平台用户摘要相同：%s", siteA)
	}
	if again := PlatformIdentityHash(1, channelId, platform, orgId, userId); again != siteA {
		t.Fatalf("同一输入摘要不稳定：%s vs %s", siteA, again)
	}
	if other := PlatformIdentityHash(1, channelId+1, platform, orgId, userId); other == siteA {
		t.Fatalf("不同渠道的摘要相同：%s", siteA)
	}
	if other := PlatformIdentityHash(1, channelId, "dingtalk", orgId, userId); other == siteA {
		t.Fatalf("不同平台的摘要相同：%s", siteA)
	}

	// 与直接调用 channelDigest 的结果一致：证明复用既有摘要协议，没有另写一套。
	direct := channelDigest(
		strconv.FormatUint(1, 10),
		strconv.FormatUint(uint64(channelId), 10),
		platform,
		orgId,
		userId,
	)
	if direct != siteA {
		t.Fatalf("PlatformIdentityHash 与 channelDigest 口径不一致：%s vs %s", siteA, direct)
	}

	// 与既有绑定身份摘要同协议：两者都不允许把「站点+渠道」省略成平台内唯一。
	legacy := bindingIdentityDigest(1, channelId, orgId, userId)
	if legacy == siteA {
		t.Fatalf("平台维度没有进入摘要：%s", siteA)
	}
}

// TestFeishuNormalizeInbound 用构造的 SDK 事件对象断言飞书事件标准化结果（离线）。
func TestFeishuNormalizeInbound(t *testing.T) {
	const appId = "cli_test_app"

	textEvent := &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{
			Header: &larkevent.EventHeader{
				EventID:   "evt_header_1",
				EventType: "im.message.receive_v1",
				AppID:     appId,
				TenantKey: "tenant_header_1",
			},
		},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:   &larkim.UserId{OpenId: channelStringPtr("ou_sender_1")},
				SenderType: channelStringPtr("user"),
				TenantKey:  channelStringPtr("tenant_header_1"),
			},
			Message: &larkim.EventMessage{
				MessageId:   channelStringPtr("om_msg_1"),
				ChatId:      channelStringPtr("oc_chat_1"),
				ChatType:    channelStringPtr("group"),
				MessageType: channelStringPtr("text"),
				Content:     channelStringPtr(`{"text":"你好，帮我查下订单"}`),
			},
		},
	}

	event, ok := normalizeFeishuInbound(textEvent, appId)
	if !ok {
		t.Fatalf("文本事件未被识别")
	}
	if event.EventId != "om_msg_1" {
		t.Errorf("EventId = %q，期望 om_msg_1", event.EventId)
	}
	if event.Kind != InboundKindText {
		t.Errorf("Kind = %q，期望 %q", event.Kind, InboundKindText)
	}
	if event.OrganizationId != "tenant_header_1" {
		t.Errorf("OrganizationId = %q，期望 tenant_header_1", event.OrganizationId)
	}
	if event.AppId != appId {
		t.Errorf("AppId = %q，期望凭据里的 %q", event.AppId, appId)
	}
	if event.UserId != "ou_sender_1" {
		t.Errorf("UserId = %q，期望 ou_sender_1（open_id）", event.UserId)
	}
	if event.ChatId != "oc_chat_1" {
		t.Errorf("ChatId = %q，期望 oc_chat_1", event.ChatId)
	}
	if event.ChatType != ChatTypeGroup {
		t.Errorf("ChatType = %q，期望 %q", event.ChatType, ChatTypeGroup)
	}
	if event.Text != "你好，帮我查下订单" {
		t.Errorf("Text = %q，与消息正文不一致", event.Text)
	}
	if err := event.Validate(appId, "tenant_header_1"); err != nil {
		t.Errorf("标准化后的事件未通过身份校验：%v", err)
	}

	// 非文本消息：保留 kind，正文置空。
	imageEvent := &larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{Header: &larkevent.EventHeader{EventID: "evt_header_2"}},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId:  &larkim.UserId{OpenId: channelStringPtr("ou_sender_2")},
				TenantKey: channelStringPtr("tenant_2"),
			},
			Message: &larkim.EventMessage{
				MessageId:   channelStringPtr("om_msg_2"),
				ChatId:      channelStringPtr("oc_chat_2"),
				ChatType:    channelStringPtr("p2p"),
				MessageType: channelStringPtr("image"),
				Content:     channelStringPtr(`{"image_key":"img_v2_xxx"}`),
			},
		},
	}
	image, ok := normalizeFeishuInbound(imageEvent, appId)
	if !ok {
		t.Fatalf("图片事件未被识别")
	}
	if image.Kind != "image" {
		t.Errorf("图片事件 Kind = %q，期望 image", image.Kind)
	}
	if image.Text != "" {
		t.Errorf("图片事件 Text = %q，期望空串", image.Text)
	}
	if image.ChatType != ChatTypeP2P {
		t.Errorf("图片事件 ChatType = %q，期望 %q", image.ChatType, ChatTypeP2P)
	}

	// 正文不是合法 JSON：不得 panic，正文置空。
	broken := *textEvent
	brokenEvent := *textEvent.Event
	brokenMessage := *textEvent.Event.Message
	brokenMessage.Content = channelStringPtr(`not-json`)
	brokenEvent.Message = &brokenMessage
	broken.Event = &brokenEvent
	if event, ok := normalizeFeishuInbound(&broken, appId); !ok || event.Text != "" {
		t.Errorf("非法正文应保留事件但正文为空，实际 ok=%v text=%q", ok, event.Text)
	}

	// 消息 ID 缺失时退回事件头里的 event_id。
	noMsgId := *textEvent.Event
	noMsgIdMessage := *textEvent.Event.Message
	noMsgIdMessage.MessageId = nil
	noMsgId.Message = &noMsgIdMessage
	fallback := *textEvent
	fallback.Event = &noMsgId
	if event, ok := normalizeFeishuInbound(&fallback, appId); !ok || event.EventId != "evt_header_1" {
		t.Errorf("消息 ID 缺失时 EventId = %q，期望退回 evt_header_1", event.EventId)
	}

	// 结构不全的负载不 panic，直接判为不可用。
	if _, ok := normalizeFeishuInbound(nil, appId); ok {
		t.Errorf("nil 事件不应被识别")
	}
	if _, ok := normalizeFeishuInbound(&larkim.P2MessageReceiveV1{}, appId); ok {
		t.Errorf("缺少 event 的事件不应被识别")
	}
}

// TestDingTalkNormalizeInbound 用构造的 SDK 回调对象断言钉钉消息与卡片标准化结果（离线）。
func TestDingTalkNormalizeInbound(t *testing.T) {
	const appId = "ding_test_app"

	groupMessage := &chatbot.BotCallbackDataModel{
		ConversationId:   "cid_group_1",
		ConversationType: "2",
		MsgId:            "msg_group_1",
		ChatbotCorpId:    "corp_1",
		SenderStaffId:    "staff_1",
		SenderId:         "sender_1",
		Msgtype:          "text",
		Text:             chatbot.BotCallbackDataTextModel{Content: " 帮我统计今天的订单 "},
	}
	event, ok := normalizeDingTalkInbound(groupMessage, appId)
	if !ok {
		t.Fatalf("群聊文本消息未被识别")
	}
	if event.EventId != "msg_group_1" {
		t.Errorf("EventId = %q，期望 msg_group_1", event.EventId)
	}
	if event.Kind != InboundKindText {
		t.Errorf("Kind = %q，期望 %q", event.Kind, InboundKindText)
	}
	if event.OrganizationId != "corp_1" {
		t.Errorf("OrganizationId = %q，期望 corp_1", event.OrganizationId)
	}
	if event.AppId != appId {
		t.Errorf("AppId = %q，期望凭据里的 %q", event.AppId, appId)
	}
	if event.UserId != "staff_1" {
		t.Errorf("UserId = %q，期望 staff_1（staffId）", event.UserId)
	}
	if event.ChatId != "cid_group_1" {
		t.Errorf("ChatId = %q，期望 cid_group_1", event.ChatId)
	}
	if event.ChatType != ChatTypeGroup {
		t.Errorf("ChatType = %q，期望 %q（conversationType=2）", event.ChatType, ChatTypeGroup)
	}
	if event.Text != " 帮我统计今天的订单 " {
		t.Errorf("Text = %q，正文应原样保留", event.Text)
	}
	if event.MentionedBot {
		t.Errorf("IsInAtList 缺省时 MentionedBot 应为 false")
	}
	if err := event.Validate(appId, "corp_1"); err != nil {
		t.Errorf("标准化后的事件未通过身份校验：%v", err)
	}

	// 群聊里 @ 了应用：isInAtList 是平台给的判定，直接映射。
	atMessage := *groupMessage
	atMessage.IsInAtList = true
	atEvent, ok := normalizeDingTalkInbound(&atMessage, appId)
	if !ok {
		t.Fatalf("@ 应用的消息未被识别")
	}
	if !atEvent.MentionedBot {
		t.Errorf("IsInAtList=true 时 MentionedBot 应为 true")
	}

	// 单聊映射。
	p2pMessage := *groupMessage
	p2pMessage.ConversationType = "1"
	p2p, ok := normalizeDingTalkInbound(&p2pMessage, appId)
	if !ok {
		t.Fatalf("单聊消息未被识别")
	}
	if p2p.ChatType != ChatTypeP2P {
		t.Errorf("ChatType = %q，期望 %q（conversationType=1）", p2p.ChatType, ChatTypeP2P)
	}
	if err := p2p.Validate(appId, "corp_1"); err != nil {
		t.Errorf("单聊事件未通过身份校验：%v", err)
	}

	// 未识别的会话类型不猜测：留空，交给 Validate 拒绝。
	unknownType := *groupMessage
	unknownType.ConversationType = "9"
	unknown, ok := normalizeDingTalkInbound(&unknownType, appId)
	if !ok {
		t.Fatalf("未知会话类型的消息不应导致标准化失败")
	}
	if unknown.ChatType != "" {
		t.Errorf("未知会话类型 ChatType = %q，期望空串", unknown.ChatType)
	}
	if err := unknown.Validate(appId, "corp_1"); err == nil {
		t.Errorf("未知会话类型应被 Validate 拒绝")
	}

	// 非文本消息：保留 kind，正文置空。
	imageMessage := *groupMessage
	imageMessage.Msgtype = "picture"
	image, ok := normalizeDingTalkInbound(&imageMessage, appId)
	if !ok {
		t.Fatalf("图片消息未被识别")
	}
	if image.Kind != "picture" {
		t.Errorf("Kind = %q，期望 picture", image.Kind)
	}
	if image.Text != "" {
		t.Errorf("图片消息 Text = %q，期望空串", image.Text)
	}

	// 组织标识缺失时退回发送者企业，仍不许凭空造值。
	noCorp := *groupMessage
	noCorp.ChatbotCorpId = ""
	noCorp.SenderCorpId = "corp_sender"
	fallbackCorp, _ := normalizeDingTalkInbound(&noCorp, appId)
	if fallbackCorp.OrganizationId != "corp_sender" {
		t.Errorf("OrganizationId = %q，期望退回 senderCorpId", fallbackCorp.OrganizationId)
	}

	if _, ok := normalizeDingTalkInbound(nil, appId); ok {
		t.Errorf("nil 回调不应被识别")
	}

	// 卡片回调：不含会话标识，只带企业/操作人/卡片实例。
	cardEvent, ok := normalizeDingTalkCardAction(&card.CardRequest{
		CorpId:     "corp_1",
		UserId:     "staff_9",
		OutTrackId: "card_instance_1",
		CardActionData: card.PrivateCardActionData{
			CardPrivateData: card.CardPrivateData{ActionIdList: []string{"approve"}},
		},
	}, appId)
	if !ok {
		t.Fatalf("卡片回调未被识别")
	}
	if cardEvent.Kind != InboundKindCardAction {
		t.Errorf("Kind = %q，期望 %q", cardEvent.Kind, InboundKindCardAction)
	}
	if cardEvent.EventId == "" || !strings.Contains(cardEvent.EventId, "card_instance_1") {
		t.Errorf("EventId = %q，应包含卡片实例标识", cardEvent.EventId)
	}
	if cardEvent.UserId != "staff_9" || cardEvent.OrganizationId != "corp_1" {
		t.Errorf("卡片回调身份映射错误：user=%q org=%q", cardEvent.UserId, cardEvent.OrganizationId)
	}
	if cardEvent.ChatId != "" {
		t.Errorf("卡片回调不应编造 ChatId，实际 %q", cardEvent.ChatId)
	}
	if cardEvent.Text != "approve" {
		t.Errorf("卡片回调 Text = %q，期望动作标识 approve", cardEvent.Text)
	}
	if cardEvent.MentionedBot {
		t.Errorf("卡片回调不应被标成「@ 了应用」")
	}
	// 卡片实例标识缺失：无法定位审批单，直接拒绝，不能产出 ":approve" 这类 EventId。
	if _, ok := normalizeDingTalkCardAction(&card.CardRequest{
		CorpId: "corp_1",
		UserId: "staff_9",
		CardActionData: card.PrivateCardActionData{
			CardPrivateData: card.CardPrivateData{ActionIdList: []string{"approve"}},
		},
	}, appId); ok {
		t.Errorf("缺少 outTrackId 的卡片回调应被拒绝")
	}
	if _, ok := normalizeDingTalkCardAction(nil, appId); ok {
		t.Errorf("nil 卡片回调不应被识别")
	}
}

// TestSplitPlatformText 固化正文分段契约：按 rune 切、不裂 UTF-8、优先在换行/句读处断、
// 拼接回原文、空串返回 nil。
func TestSplitPlatformText(t *testing.T) {
	if segments := splitPlatformText("", 10); segments != nil {
		t.Errorf("空串应返回 nil，实际 %v", segments)
	}
	// 单段边界：刚好等于上限不应被切开。
	exact := strings.Repeat("a", 10)
	if segments := splitPlatformText(exact, 10); len(segments) != 1 || segments[0] != exact {
		t.Errorf("恰好等于上限的正文应原样一段，实际 %v", segments)
	}
	// maxRunes <= 0 视为不分段（防御性分支，避免调用方传错就把正文切成空段）。
	if segments := splitPlatformText("abc", 0); len(segments) != 1 || segments[0] != "abc" {
		t.Errorf("maxRunes<=0 应不分段，实际 %v", segments)
	}

	cases := []struct {
		name     string
		text     string
		maxRunes int
		wantLen  int
	}{
		{name: "超一个字符切两段", text: strings.Repeat("a", 11), maxRunes: 10, wantLen: 2},
		{name: "长中文多段", text: strings.Repeat("中文测试", 50), maxRunes: 100, wantLen: 2},
		{name: "无断点硬切", text: strings.Repeat("x", 250), maxRunes: 100, wantLen: 3},
		{
			name:     "换行处断开",
			text:     strings.Repeat("a", 90) + "\n" + strings.Repeat("b", 90),
			maxRunes: 100,
			wantLen:  2,
		},
		{
			name:     "句号处断开",
			text:     strings.Repeat("中", 99) + "。" + strings.Repeat("文", 99),
			maxRunes: 100,
			wantLen:  2,
		},
	}
	for _, tc := range cases {
		segments := splitPlatformText(tc.text, tc.maxRunes)
		if len(segments) != tc.wantLen {
			t.Errorf("%s：段数 = %d，期望 %d", tc.name, len(segments), tc.wantLen)
		}
		if joined := strings.Join(segments, ""); joined != tc.text {
			t.Errorf("%s：拼接后与原文不一致（len %d vs %d）", tc.name, len(joined), len(tc.text))
		}
		for i, segment := range segments {
			if segment == "" {
				t.Errorf("%s：第 %d 段为空", tc.name, i)
			}
			if count := len([]rune(segment)); count > tc.maxRunes {
				t.Errorf("%s：第 %d 段 %d rune 超过上限 %d", tc.name, i, count, tc.maxRunes)
			}
			if !utf8.ValidString(segment) {
				t.Errorf("%s：第 %d 段不是合法 UTF-8", tc.name, i)
			}
		}
	}

	// 断点必须落在换行之后，不能在上限处硬切。
	lineText := strings.Repeat("a", 90) + "\n" + strings.Repeat("b", 90)
	if first := splitPlatformText(lineText, 100)[0]; !strings.HasSuffix(first, "\n") {
		t.Errorf("换行处断开的期望落空，第一段结尾是 %q", first[len(first)-3:])
	}
	// 句号同理。
	dotText := strings.Repeat("中", 99) + "。" + strings.Repeat("文", 99)
	if first := splitPlatformText(dotText, 100)[0]; !strings.HasSuffix(first, "。") {
		t.Errorf("句号处断开的期望落空，第一段结尾 3 字符是 %q", []rune(first)[len([]rune(first))-3:])
	}

	// 中文按 rune 切：max=3 时每段最多 3 个汉字，不会出现半个字符。
	chinese := strings.Repeat("你好世界", 10)
	for i, segment := range splitPlatformText(chinese, 3) {
		if !utf8.ValidString(segment) || len([]rune(segment)) > 3 {
			t.Errorf("中文第 %d 段不合法：%q", i, segment)
		}
	}
}

// TestFeishuMentionedBot 固化飞书群聊 @ 判定：只有 mentions 里出现本应用机器人的 open_id
// 才算「明确 @ 应用」；私聊与取不到机器人身份时一律 false（fail closed）。
func TestFeishuMentionedBot(t *testing.T) {
	const botOpenId = "ou_bot_1"
	message := &larkim.EventMessage{
		Mentions: []*larkim.MentionEvent{
			{Id: &larkim.UserId{OpenId: channelStringPtr("ou_someone_else")}},
			{Id: &larkim.UserId{OpenId: channelStringPtr(botOpenId)}},
		},
	}
	if !feishuMentionedBot(message, ChatTypeGroup, botOpenId) {
		t.Errorf("群聊 mentions 含机器人 open_id 时应判为已 @")
	}
	if feishuMentionedBot(message, ChatTypeGroup, "ou_other_bot") {
		t.Errorf("mentions 不含机器人 open_id 时不应判为已 @")
	}
	if feishuMentionedBot(message, ChatTypeP2P, botOpenId) {
		t.Errorf("私聊事件的 MentionedBot 必须保持 false")
	}
	if feishuMentionedBot(message, ChatTypeGroup, "") {
		t.Errorf("取不到机器人 open_id 时必须 fail closed")
	}
	if feishuMentionedBot(nil, ChatTypeGroup, botOpenId) {
		t.Errorf("nil 消息不应判为已 @")
	}
	// mentions 里个别项结构不全（Id 为 nil）不应 panic，也不应误判。
	partial := &larkim.EventMessage{Mentions: []*larkim.MentionEvent{nil, {}}}
	if feishuMentionedBot(partial, ChatTypeGroup, botOpenId) {
		t.Errorf("结构不全的 mentions 不应判为已 @")
	}
}

// captureFeishuSends 把飞书的发送调用记录下来，返回一个假的成功响应。
func captureFeishuSends(t *testing.T, ids ...string) (*feishuAdapter, *[]feishuSendRequest) {
	t.Helper()
	requests := make([]feishuSendRequest, 0, len(ids))
	call := 0
	adapter := &feishuAdapter{
		creds: PlatformCredentials{AppId: "cli_app", AppSecret: "secret", OrganizationId: "tenant_1"},
		createMessage: func(_ context.Context, req feishuSendRequest) (*larkim.CreateMessageResp, error) {
			requests = append(requests, req)
			messageId := ""
			if call < len(ids) {
				messageId = ids[call]
			}
			call++
			return &larkim.CreateMessageResp{
				CodeError: larkcore.CodeError{Code: 0},
				Data:      &larkim.CreateMessageRespData{MessageId: channelStringPtr(messageId)},
			}, nil
		},
	}
	return adapter, &requests
}

// TestFeishuSendUsesChatIdForGroup 断言群聊用 chat_id 且回执取最后一段的消息 ID。
func TestFeishuSendUsesChatIdForGroup(t *testing.T) {
	adapter, requests := captureFeishuSends(t, "om_seg_1", "om_seg_2")
	longText := strings.Repeat("通知正文", 1000) // 4000 rune > 3500，必须分段
	result, err := adapter.Send(context.Background(), TargetRef{
		Kind:             TargetKindGroup,
		PlatformTargetId: "oc_chat_1",
	}, OutboundMessage{Text: longText})
	if err != nil {
		t.Fatalf("发送失败：%v", err)
	}
	if len(*requests) < 2 {
		t.Fatalf("超长正文应分段发送，实际只发了 %d 段", len(*requests))
	}
	for i, req := range *requests {
		if req.ReceiveIdType != "chat_id" {
			t.Errorf("第 %d 段 receive_id_type = %q，群聊期望 chat_id", i, req.ReceiveIdType)
		}
		if req.ReceiveId != "oc_chat_1" {
			t.Errorf("第 %d 段 receive_id = %q，期望 oc_chat_1", i, req.ReceiveId)
		}
		if count := len([]rune(req.Text)); count > feishuMaxTextRunes {
			t.Errorf("第 %d 段 %d rune 超过上限 %d", i, count, feishuMaxTextRunes)
		}
	}
	if joined := joinFeishuTexts(*requests); joined != longText {
		t.Errorf("分段后拼接与原文不一致（%d vs %d 字符）", len(joined), len(longText))
	}
	if result.PlatformMessageId != "om_seg_2" {
		t.Errorf("回执 message_id = %q，期望最后一段的 om_seg_2", result.PlatformMessageId)
	}
}

// TestFeishuSendUsesOpenIdForPrivate 断言私聊用 open_id。
func TestFeishuSendUsesOpenIdForPrivate(t *testing.T) {
	adapter, requests := captureFeishuSends(t, "om_p2p_1")
	result, err := adapter.Send(context.Background(), TargetRef{
		Kind:             TargetKindPrivate,
		PlatformTargetId: "ou_user_1",
	}, OutboundMessage{Text: "你好"})
	if err != nil {
		t.Fatalf("发送失败：%v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("短文本应只发一段，实际 %d 段", len(*requests))
	}
	req := (*requests)[0]
	if req.ReceiveIdType != "open_id" {
		t.Errorf("私聊 receive_id_type = %q，期望 open_id", req.ReceiveIdType)
	}
	if req.ReceiveId != "ou_user_1" {
		t.Errorf("私聊 receive_id = %q，期望 ou_user_1", req.ReceiveId)
	}
	if req.Text != "你好" {
		t.Errorf("正文 = %q，期望原样透传", req.Text)
	}
	if result.PlatformMessageId != "om_p2p_1" {
		t.Errorf("回执 message_id = %q，期望 om_p2p_1", result.PlatformMessageId)
	}
}

func joinFeishuTexts(requests []feishuSendRequest) string {
	var builder strings.Builder
	for _, req := range requests {
		builder.WriteString(req.Text)
	}
	return builder.String()
}

// TestSendRejectsCardOnlyMessage 断言卡片没有被「假装支持」：只有 Card 没有 Text 时
// 两个适配器都必须拒绝，且拒绝是 permanent（重试不会变好）。
func TestSendRejectsCardOnlyMessage(t *testing.T) {
	creds := PlatformCredentials{AppId: "app_1", AppSecret: "secret_1", OrganizationId: "org_1"}
	feishu := &feishuAdapter{creds: creds, createMessage: func(context.Context, feishuSendRequest) (*larkim.CreateMessageResp, error) {
		t.Errorf("卡片消息不应触达发送 SDK")
		return nil, nil
	}}
	dingtalk := newDingTalkAdapterWithEndpoint(creds, "http://127.0.0.1:1", nil)
	target := TargetRef{Kind: TargetKindGroup, PlatformTargetId: "target_1"}
	card := map[string]any{"type": "template", "data": map[string]any{}}

	for name, adapter := range map[string]PlatformAdapter{"feishu": feishu, "dingtalk": dingtalk} {
		_, err := adapter.Send(context.Background(), target, OutboundMessage{Card: card})
		if err == nil {
			t.Fatalf("%s 只发卡片时应拒绝", name)
		}
		var sendErr *SendError
		if !errors.As(err, &sendErr) {
			t.Fatalf("%s 返回了非 SendError：%v", name, err)
		}
		if sendErr.Class != ErrorClassPermanent {
			t.Errorf("%s 卡片消息的错误分类 = %s，期望 permanent", name, sendErr.Class)
		}
		if !strings.Contains(sendErr.Message, "卡片消息暂不支持") {
			t.Errorf("%s 错误文案应说明卡片不支持，实际 %q", name, sendErr.Message)
		}
	}

	// 有正文时 Card 被显式忽略：不能因为带了 Card 就改变文本发送行为。
	adapter, requests := captureFeishuSends(t, "om_text_1")
	if _, err := adapter.Send(context.Background(), target, OutboundMessage{Text: "正文", Card: card}); err != nil {
		t.Fatalf("带正文的消息应正常发送：%v", err)
	}
	if len(*requests) != 1 || (*requests)[0].Text != "正文" {
		t.Errorf("带 Card 时正文未被正确发送：%+v", *requests)
	}
}

// TestDingTalkSendRejectsEmptyText 断言空正文在本地就被拒绝（permanent），不发任何请求。
func TestDingTalkSendRejectsEmptyText(t *testing.T) {
	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: "ding_empty_text", AppSecret: "secret"},
		"http://127.0.0.1:1", nil,
	)
	_, err := adapter.Send(context.Background(), TargetRef{Kind: TargetKindPrivate, PlatformTargetId: "staff_1"}, OutboundMessage{Text: "   "})
	if err == nil {
		t.Fatalf("空正文应被拒绝")
	}
	var sendErr *SendError
	if !errors.As(err, &sendErr) || sendErr.Class != ErrorClassPermanent {
		t.Fatalf("空正文的期望 permanent，实际 %v", err)
	}
	if !strings.Contains(sendErr.Message, "内容为空") {
		t.Errorf("错误文案 = %q，期望说明内容为空", sendErr.Message)
	}
}

// TestDingTalkSendFetchesTokenOnce 断言连续两次发送只取一次 accessToken，且分段发送复用同一 token。
func TestDingTalkSendFetchesTokenOnce(t *testing.T) {
	const appId = "ding_token_once"
	dingTalkTokenCache.invalidate(appId)
	t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })

	var tokenCalls, sendCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case dingTalkTokenEndpoint:
			tokenCalls.Add(1)
			if got := r.Header.Get("x-acs-dingtalk-access-token"); got != "" {
				t.Errorf("取 token 的请求不该带 access token 头，实际 %q", got)
			}
			_, _ = w.Write([]byte(`{"accessToken":"token_1","expireIn":7200}`))
		case dingTalkGroupSendEndpoint:
			sendCalls.Add(1)
			if got := r.Header.Get("x-acs-dingtalk-access-token"); got != "token_1" {
				t.Errorf("发送请求应带缓存 token，实际 %q", got)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("发送请求体不是合法 JSON：%v", err)
			}
			if body["openConversationId"] != "cid_group_1" {
				t.Errorf("群聊发送的 openConversationId = %v，期望 cid_group_1", body["openConversationId"])
			}
			if body["robotCode"] != appId {
				t.Errorf("群聊发送的 robotCode = %v，期望 %s", body["robotCode"], appId)
			}
			if _, ok := body["userIds"]; ok {
				t.Errorf("群聊发送不应带 userIds")
			}
			_, _ = w.Write([]byte(`{"processQueryKey":"pqk_1"}`))
		default:
			t.Errorf("未预期的请求路径 %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: appId, AppSecret: "secret", OrganizationId: "corp_1"},
		server.URL, server.Client(),
	)
	target := TargetRef{Kind: TargetKindGroup, PlatformTargetId: "cid_group_1"}
	for i := 0; i < 2; i++ {
		result, err := adapter.Send(context.Background(), target, OutboundMessage{Text: "你好"})
		if err != nil {
			t.Fatalf("第 %d 次发送失败：%v", i+1, err)
		}
		if result.PlatformMessageId != "pqk_1" {
			t.Errorf("第 %d 次回执 = %q，期望 pqk_1", i+1, result.PlatformMessageId)
		}
	}
	if got := tokenCalls.Load(); got != 1 {
		t.Fatalf("两次发送取了 %d 次 token，期望 1 次", got)
	}
	if got := sendCalls.Load(); got != 2 {
		t.Fatalf("两次发送实际发了 %d 次，期望 2 次", got)
	}
}

// TestDingTalkSendSplitsLongText 断言超长正文被分段，而不是整条丢给平台（4xx → permanent → 丢消息）。
func TestDingTalkSendSplitsLongText(t *testing.T) {
	const appId = "ding_split_long"
	dingTalkTokenCache.invalidate(appId)
	t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })

	var sendCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case dingTalkTokenEndpoint:
			_, _ = w.Write([]byte(`{"accessToken":"token_2","expireIn":7200}`))
		case dingTalkPrivateSendEndpoint:
			sendCalls.Add(1)
			var body struct {
				UserIds  []string `json:"userIds"`
				MsgParam string   `json:"msgParam"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("请求体不是合法 JSON：%v", err)
			}
			if len(body.UserIds) != 1 || body.UserIds[0] != "staff_1" {
				t.Errorf("单聊发送的 userIds = %v，期望 [staff_1]", body.UserIds)
			}
			var param struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(body.MsgParam), &param); err != nil {
				t.Errorf("msgParam 不是合法 JSON：%v", err)
			}
			if count := len([]rune(param.Content)); count > dingTalkMaxTextRunes {
				t.Errorf("单段 %d rune 超过上限 %d", count, dingTalkMaxTextRunes)
			}
			_, _ = w.Write([]byte(`{"processQueryKey":"pqk_seg"}`))
		default:
			t.Errorf("未预期的请求路径 %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: appId, AppSecret: "secret"},
		server.URL, server.Client(),
	)
	text := strings.Repeat("长通知正文", 500) // 2500 rune > 1800
	if _, err := adapter.Send(context.Background(), TargetRef{Kind: TargetKindPrivate, PlatformTargetId: "staff_1"}, OutboundMessage{Text: text}); err != nil {
		t.Fatalf("分段发送失败：%v", err)
	}
	if got := sendCalls.Load(); got < 2 {
		t.Fatalf("超长正文只发了 %d 段，期望至少 2 段", got)
	}
}

// TestDingTalkSendUnauthorizedIsTransient 断言发送端 401 不是 permanent：
// 先清 token 缓存再按 transient 返回，重试时重新取 token（这里用取 token 次数证明缓存确实被清）。
func TestDingTalkSendUnauthorizedIsTransient(t *testing.T) {
	const appId = "ding_unauthorized"
	dingTalkTokenCache.invalidate(appId)
	t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })

	var tokenCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case dingTalkTokenEndpoint:
			tokenCalls.Add(1)
			_, _ = w.Write([]byte(`{"accessToken":"token_expired","expireIn":7200}`))
		case dingTalkPrivateSendEndpoint:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"InvalidAuthentication","message":"access token expired"}`))
		default:
			t.Errorf("未预期的请求路径 %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: appId, AppSecret: "secret"},
		server.URL, server.Client(),
	)
	target := TargetRef{Kind: TargetKindPrivate, PlatformTargetId: "staff_1"}
	_, err := adapter.Send(context.Background(), target, OutboundMessage{Text: "你好"})
	if err == nil {
		t.Fatalf("401 应返回错误")
	}
	var sendErr *SendError
	if !errors.As(err, &sendErr) {
		t.Fatalf("返回了非 SendError：%v", err)
	}
	if sendErr.Class != ErrorClassTransient {
		t.Fatalf("发送端 401 应为 transient（重取 token 即可成功），实际 %s：%s", sendErr.Class, sendErr.Message)
	}
	if !strings.Contains(sendErr.Message, "已清除缓存") {
		t.Errorf("错误文案应说明已清除缓存，实际 %q", sendErr.Message)
	}

	// 缓存被清的直接证据：再次发送必须重新取 token。
	if _, err := adapter.Send(context.Background(), target, OutboundMessage{Text: "你好"}); err == nil {
		t.Fatalf("第二次 401 也应返回错误")
	}
	if got := tokenCalls.Load(); got != 2 {
		t.Fatalf("401 后 token 缓存未清除：两次发送共取 %d 次 token，期望 2 次", got)
	}
}

// TestDingTalkSendRetryAfterFromHeader 断言 429 的 Retry-After 被带进 SendResult，供 T8 排重试。
func TestDingTalkSendRetryAfterFromHeader(t *testing.T) {
	const appId = "ding_retry_after"
	dingTalkTokenCache.invalidate(appId)
	t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case dingTalkTokenEndpoint:
			_, _ = w.Write([]byte(`{"accessToken":"token_429","expireIn":7200}`))
		case dingTalkPrivateSendEndpoint:
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":"Throttling","message":"too many requests"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: appId, AppSecret: "secret"},
		server.URL, server.Client(),
	)
	result, err := adapter.Send(context.Background(), TargetRef{Kind: TargetKindPrivate, PlatformTargetId: "staff_1"}, OutboundMessage{Text: "你好"})
	if err == nil {
		t.Fatalf("429 应返回错误")
	}
	var sendErr *SendError
	if !errors.As(err, &sendErr) || sendErr.Class != ErrorClassTransient {
		t.Fatalf("429 期望 transient，实际 %v", err)
	}
	if result.RetryAfterSeconds != 7 {
		t.Errorf("RetryAfterSeconds = %d，期望 7", result.RetryAfterSeconds)
	}
}

// TestDingTalkSendForbiddenIsPermanent 固化「发送端 403 = 权限问题 = 永久错误」：
// 机器人被移出群 / 没有发送权限，重试不会变好；按 transient 重试只会让每条通知多打一次
// token 请求。必须与 401 分开：只有 401 才可能是缓存 token 被平台提前作废。
func TestDingTalkSendForbiddenIsPermanent(t *testing.T) {
	const appId = "ding_forbidden"
	dingTalkTokenCache.invalidate(appId)
	t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case dingTalkTokenEndpoint:
			_, _ = w.Write([]byte(`{"accessToken":"token_403","expireIn":7200}`))
		case dingTalkGroupSendEndpoint:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"code":"Forbidden.AccessDenied","message":"robot has been removed from the conversation"}`))
		default:
			t.Errorf("未预期的请求路径 %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: appId, AppSecret: "secret"},
		server.URL, server.Client(),
	)
	result, err := adapter.Send(context.Background(), TargetRef{
		Kind:             TargetKindGroup,
		PlatformTargetId: "cid_group_1",
	}, OutboundMessage{Text: "你好"})
	if err == nil {
		t.Fatalf("发送端 403 应返回错误")
	}
	var sendErr *SendError
	if !errors.As(err, &sendErr) {
		t.Fatalf("返回了非 SendError：%v", err)
	}
	if sendErr.Class != ErrorClassPermanent {
		t.Fatalf("发送端 403 应为 permanent（权限问题重试不会变好），实际 %s：%s", sendErr.Class, sendErr.Message)
	}
	if strings.Contains(sendErr.Message, "已清除缓存") {
		t.Errorf("403 不应走「清缓存 + 按 transient 重试」分支，实际文案 %q", sendErr.Message)
	}
	if result.PlatformMessageId != "" {
		t.Errorf("失败时不应返回成功回执，实际 %q", result.PlatformMessageId)
	}
}

// TestDingTalkSendTokenStageAuthFailuresArePermanent 固化取 token 阶段的鉴权失败：
// appKey/appSecret 无效（HTTP 401/403）重试不会变好，且必须停在 token 阶段、不发消息。
func TestDingTalkSendTokenStageAuthFailuresArePermanent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{name: "取 token 返回 401", status: http.StatusUnauthorized},
		{name: "取 token 返回 403", status: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			appId := "ding_token_auth_" + strconv.Itoa(tc.status)
			dingTalkTokenCache.invalidate(appId)
			t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })

			var sendCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case dingTalkTokenEndpoint:
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(`{"code":"InvalidAuthentication","message":"invalid appKey or appSecret"}`))
				case dingTalkPrivateSendEndpoint:
					sendCalls.Add(1)
					_, _ = w.Write([]byte(`{"processQueryKey":"pqk_should_not_happen"}`))
				default:
					t.Errorf("未预期的请求路径 %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			adapter := newDingTalkAdapterWithEndpoint(
				PlatformCredentials{AppId: appId, AppSecret: "secret"},
				server.URL, server.Client(),
			)
			_, err := adapter.Send(context.Background(), TargetRef{
				Kind:             TargetKindPrivate,
				PlatformTargetId: "staff_1",
			}, OutboundMessage{Text: "你好"})
			if err == nil {
				t.Fatalf("取 token 失败应返回错误")
			}
			var sendErr *SendError
			if !errors.As(err, &sendErr) || sendErr.Class != ErrorClassPermanent {
				t.Fatalf("取 token 阶段 HTTP %d 期望 permanent，实际 %v", tc.status, err)
			}
			if got := sendCalls.Load(); got != 0 {
				t.Errorf("取 token 失败后不应继续发消息，实际发了 %d 次", got)
			}
		})
	}
}

// TestDingTalkInboundP2PNeverMentionsBot 固化「私聊事件 MentionedBot 恒为 false」：
// 平台在单聊里即使把本条消息算进 atList，也不得让 MentionedBot 为 true。
func TestDingTalkInboundP2PNeverMentionsBot(t *testing.T) {
	const appId = "ding_p2p_mention"
	base := chatbot.BotCallbackDataModel{
		ConversationId:   "cid_p2p_1",
		ConversationType: "1",
		MsgId:            "msg_p2p_1",
		ChatbotCorpId:    "corp_1",
		SenderStaffId:    "staff_1",
		Msgtype:          "text",
		Text:             chatbot.BotCallbackDataTextModel{Content: "你好"},
		IsInAtList:       true,
	}
	p2p, ok := normalizeDingTalkInbound(&base, appId)
	if !ok {
		t.Fatalf("单聊消息未被识别")
	}
	if p2p.ChatType != ChatTypeP2P {
		t.Fatalf("ChatType = %q，期望 %q", p2p.ChatType, ChatTypeP2P)
	}
	if p2p.MentionedBot {
		t.Errorf("单聊事件即使 IsInAtList=true 也必须 MentionedBot=false")
	}

	// 正向对照：同样的 atList 在群聊里必须映射成 true，证明上面的 false 不是恒 false。
	group := base
	group.ConversationType = "2"
	groupEvent, _ := normalizeDingTalkInbound(&group, appId)
	if !groupEvent.MentionedBot {
		t.Errorf("群聊 IsInAtList=true 时 MentionedBot 应为 true：私聊断言可能在空转")
	}

	// 未知会话类型同样不采信 atList（它随后会被 InboundEvent.Validate 拒绝）。
	unknown := base
	unknown.ConversationType = "9"
	unknownEvent, _ := normalizeDingTalkInbound(&unknown, appId)
	if unknownEvent.MentionedBot {
		t.Errorf("未知会话类型不应采信 atList")
	}
}

// TestFeishuBotIdentityTimeoutFailsClosed 固化建连前拉取机器人身份的超时：
// lark 客户端默认没有响应超时，半开连接会让整条重连循环永久卡住。
// 超时必须按「拉取失败」处理（open_id 为空 → MentionedBot 恒 false），而不是无限等待。
func TestFeishuBotIdentityTimeoutFailsClosed(t *testing.T) {
	creds := PlatformCredentials{AppId: "cli_bot_timeout", AppSecret: "secret", OrganizationId: "tenant_1"}
	blocking := func(ctx context.Context, _ PlatformCredentials) string {
		<-ctx.Done()
		// 超时之后即使还返回了值也不得采信：这条路径等价于「请求被取消」。
		return "ou_bot_stale"
	}
	started := time.Now()
	openId := fetchFeishuBotIdentity(context.Background(), blocking, creds, 50*time.Millisecond)
	elapsed := time.Since(started)
	if openId != "" {
		t.Errorf("拉取超时后必须 fail closed（open_id 为空），实际 %q", openId)
	}
	if elapsed > 2*time.Second {
		t.Errorf("拉取身份的超时没有生效：耗时 %s", elapsed)
	}

	// 正常路径不受影响：及时返回的 open_id 原样使用（只做 TrimSpace）。
	fast := func(context.Context, PlatformCredentials) string { return "  ou_bot_fast " }
	if got := fetchFeishuBotIdentity(context.Background(), fast, creds, time.Second); got != "ou_bot_fast" {
		t.Errorf("正常路径 open_id = %q，期望 ou_bot_fast", got)
	}
}

// TestDingTalkSendReportsSecondSegmentFailure 固化「多段发送中途失败」：
// 错误分类与 Retry-After 必须取自失败的那一段，且不得返回成功回执。
func TestDingTalkSendReportsSecondSegmentFailure(t *testing.T) {
	const appId = "ding_seg2_fail"
	dingTalkTokenCache.invalidate(appId)
	t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })

	var sendCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case dingTalkTokenEndpoint:
			_, _ = w.Write([]byte(`{"accessToken":"token_seg2","expireIn":7200}`))
		case dingTalkPrivateSendEndpoint:
			if sendCalls.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"processQueryKey":"pqk_seg_1"}`))
				return
			}
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":"Throttling","message":"too many requests"}`))
		default:
			t.Errorf("未预期的请求路径 %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: appId, AppSecret: "secret"},
		server.URL, server.Client(),
	)
	text := strings.Repeat("长通知正文", 500) // 2500 rune > 1800 → 至少两段
	result, err := adapter.Send(context.Background(), TargetRef{
		Kind:             TargetKindPrivate,
		PlatformTargetId: "staff_1",
	}, OutboundMessage{Text: text})
	if err == nil {
		t.Fatalf("第 2 段 429 应返回错误")
	}
	var sendErr *SendError
	if !errors.As(err, &sendErr) {
		t.Fatalf("返回了非 SendError：%v", err)
	}
	if sendErr.Class != ErrorClassTransient {
		t.Errorf("第 2 段 429 应为 transient，实际 %s：%s", sendErr.Class, sendErr.Message)
	}
	if result.RetryAfterSeconds != 5 {
		t.Errorf("RetryAfterSeconds = %d，期望取第 2 段 429 的 5", result.RetryAfterSeconds)
	}
	if result.PlatformMessageId != "" {
		t.Errorf("失败时不应返回成功回执（第 1 段已送达也不回执），实际 %q", result.PlatformMessageId)
	}
	if got := sendCalls.Load(); got != 2 {
		t.Errorf("第 2 段失败后应立即停止，实际发了 %d 段", got)
	}
}

// TestFeishuSendReportsSecondSegmentFailure 固化飞书多段发送中途失败：
// 第 2 段建消息失败时错误分类来自第 2 段，且不返回成功回执。
func TestFeishuSendReportsSecondSegmentFailure(t *testing.T) {
	call := 0
	adapter := &feishuAdapter{
		creds: PlatformCredentials{AppId: "cli_seg2", AppSecret: "secret", OrganizationId: "tenant_1"},
		createMessage: func(_ context.Context, _ feishuSendRequest) (*larkim.CreateMessageResp, error) {
			call++
			if call == 1 {
				return &larkim.CreateMessageResp{
					CodeError: larkcore.CodeError{Code: 0},
					Data:      &larkim.CreateMessageRespData{MessageId: channelStringPtr("om_seg_1")},
				}, nil
			}
			return &larkim.CreateMessageResp{
				CodeError: larkcore.CodeError{Code: 230002, Msg: "bot is not in the chat"},
			}, nil
		},
	}
	result, err := adapter.Send(context.Background(), TargetRef{
		Kind:             TargetKindGroup,
		PlatformTargetId: "oc_chat_1",
	}, OutboundMessage{Text: strings.Repeat("通知正文", 1000)}) // 4000 rune > 3500 → 两段
	if err == nil {
		t.Fatalf("第 2 段失败应返回错误")
	}
	var sendErr *SendError
	if !errors.As(err, &sendErr) || sendErr.Class != ErrorClassPermanent {
		t.Fatalf("第 2 段业务码 230002 期望 permanent，实际 %v", err)
	}
	if !strings.Contains(sendErr.Message, "230002") {
		t.Errorf("错误文案应带上第 2 段的业务码，实际 %q", sendErr.Message)
	}
	if result.PlatformMessageId != "" {
		t.Errorf("失败时不应返回成功回执，实际 %q", result.PlatformMessageId)
	}
	if call != 2 {
		t.Errorf("第 2 段失败后应立即停止，实际调用 %d 次", call)
	}
}

// TestParseRetryAfterSeconds 固化 Retry-After 解析：秒数与 HTTP 日期两种形态都支持，
// 解析不出来或日期已经过去一律返回 0（绝不返回负数）。
func TestParseRetryAfterSeconds(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  int
	}{
		{name: "秒数", value: "7", want: 7},
		{name: "带空白的秒数", value: " 12 ", want: 12},
		{name: "空值", value: "", want: 0},
		{name: "零秒", value: "0", want: 0},
		{name: "负秒数不得返回负数", value: "-30", want: 0},
		{name: "无法解析", value: "soon", want: 0},
	}
	for _, tc := range cases {
		if got := parseRetryAfterSeconds(tc.value); got != tc.want {
			t.Errorf("%s：parseRetryAfterSeconds(%q) = %d，期望 %d", tc.name, tc.value, got, tc.want)
		}
	}

	// HTTP-date 分支：未来 30 秒的日期应解析出接近 30 的秒数（秒级截断会少 1 秒以内）。
	future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	if got := parseRetryAfterSeconds(future); got < 27 || got > 30 {
		t.Errorf("HTTP-date 未来 30 秒解析为 %d 秒，期望 27~30", got)
	}
	// 已经过去的日期：等待时间已过，返回 0 而不是负数。
	past := time.Now().Add(-2 * time.Minute).UTC().Format(http.TimeFormat)
	if got := parseRetryAfterSeconds(past); got != 0 {
		t.Errorf("HTTP-date 已过期解析为 %d，期望 0", got)
	}
}
