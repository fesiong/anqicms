package provider

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"testing"

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

// TestAdapterRegistryRejectsUnknownPlatform 固化平台注册表：只认飞书与钉钉。
func TestAdapterRegistryRejectsUnknownPlatform(t *testing.T) {
	for _, name := range []string{"feishu", "dingtalk"} {
		adapter, ok := GetPlatformAdapter(name)
		if !ok || adapter == nil {
			t.Fatalf("%s 适配器未注册", name)
		}
		if adapter.Name() != name {
			t.Errorf("适配器 %s 的 Name() 返回 %q，与注册键不一致", name, adapter.Name())
		}
	}

	for _, name := range []string{"wechat", "wecom", "lark", "", "   ", "slack"} {
		if adapter, ok := GetPlatformAdapter(name); ok || adapter != nil {
			t.Errorf("未支持平台 %q 不应注册，实际返回 %v", name, adapter)
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
	if err := event.Validate(appId, "corp_1"); err != nil {
		t.Errorf("标准化后的事件未通过身份校验：%v", err)
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
	if _, ok := normalizeDingTalkCardAction(nil, appId); ok {
		t.Errorf("nil 卡片回调不应被识别")
	}
}
