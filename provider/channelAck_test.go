package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"kandaoni.com/anqicms/model"
)

// 本文件锁定「收到即回执」：消息落库后立刻在平台侧贴一个轻量表情，
// 让用户知道消息已进队列，而不是静默等模型与工具跑完。
//
// 护栏分三层：
//  1. 闸门（channelAckGate）必须与消费侧判据一致——未绑定/仅通知/无智能体/
//     群目标未授权的消息不会被执行，回执它们等于骗人；
//  2. 两个适配器必须按平台消息 ID 打到正确的接口（飞书 message reaction / 钉钉文字表情）；
//  3. worker 接线只能在**落库成功之后**触发，重复投递不重复回执。

// 编译期护栏：两个生产适配器都必须满足可选回执接口。
var (
	_ InboundAcknowledger = (*feishuAdapter)(nil)
	_ InboundAcknowledger = (*dingtalkAdapter)(nil)
)

// ackRecordingAdapter 记录回执事件；其它平台方法都是空实现。
type ackRecordingAdapter struct {
	mu   sync.Mutex
	acks []InboundEvent
}

func (a *ackRecordingAdapter) Name() string { return "ack-recording" }

func (a *ackRecordingAdapter) Run(context.Context, PlatformCredentials, InboundSink) error {
	return nil
}

func (a *ackRecordingAdapter) Send(context.Context, TargetRef, OutboundMessage) (SendResult, error) {
	return SendResult{}, nil
}

func (a *ackRecordingAdapter) Acknowledge(_ context.Context, event InboundEvent) error {
	a.mu.Lock()
	a.acks = append(a.acks, event)
	a.mu.Unlock()
	return nil
}

func (a *ackRecordingAdapter) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.acks)
}

func (a *ackRecordingAdapter) last() (InboundEvent, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.acks) == 0 {
		return InboundEvent{}, false
	}
	return a.acks[len(a.acks)-1], true
}

// TestChannelAckGate 是回执闸门的唯一判据表。
//
// 反向对照：去掉 allowed() 里任一条件（例如 ChatEnabled 或 GroupTargetReady），
// 对应用例都会变红——这些条件与 channelInboxWorker.consumeInbox / channelChat.go
// 的拒绝路径一一对应。
func TestChannelAckGate(t *testing.T) {
	channel := func(chatEnabled int, agentId uint) *model.MessageChannel {
		return &model.MessageChannel{ChatEnabled: chatEnabled, AgentId: agentId}
	}
	textEvent := func(chatType string) InboundEvent {
		return InboundEvent{EventId: "om_1", Kind: InboundKindText, ChatType: chatType, Text: "你好"}
	}

	tests := []struct {
		name string
		gate channelAckGate
		want bool
	}{
		{
			name: "已绑定私聊文本回执",
			gate: channelAckGate{SiteId: 1, Channel: channel(1, 7), Event: textEvent(ChatTypeP2P), Bound: true, AiAvailable: true},
			want: true,
		},
		{
			name: "未绑定不回执",
			gate: channelAckGate{SiteId: 1, Channel: channel(1, 7), Event: textEvent(ChatTypeP2P), Bound: false, AiAvailable: true},
		},
		{
			name: "聊天服务不可用不回执",
			gate: channelAckGate{SiteId: 1, Channel: channel(1, 7), Event: textEvent(ChatTypeP2P), Bound: true, AiAvailable: false},
		},
		{
			name: "渠道为空不回执",
			gate: channelAckGate{SiteId: 1, Channel: nil, Event: textEvent(ChatTypeP2P), Bound: true, AiAvailable: true},
		},
		{
			name: "子站点不回执",
			gate: channelAckGate{SiteId: 2, Channel: channel(1, 7), Event: textEvent(ChatTypeP2P), Bound: true, AiAvailable: true},
		},
		{
			name: "仅通知渠道不回执",
			gate: channelAckGate{SiteId: 1, Channel: channel(0, 7), Event: textEvent(ChatTypeP2P), Bound: true, AiAvailable: true},
		},
		{
			name: "未配置接待智能体不回执",
			gate: channelAckGate{SiteId: 1, Channel: channel(1, 0), Event: textEvent(ChatTypeP2P), Bound: true, AiAvailable: true},
		},
		{
			name: "非文本消息不回执",
			gate: channelAckGate{SiteId: 1, Channel: channel(1, 7), Bound: true, AiAvailable: true,
				Event: InboundEvent{EventId: "om_1", Kind: "image", ChatType: ChatTypeP2P}},
		},
		{
			name: "空正文不回执",
			gate: channelAckGate{SiteId: 1, Channel: channel(1, 7), Bound: true, AiAvailable: true,
				Event: InboundEvent{EventId: "om_1", Kind: InboundKindText, ChatType: ChatTypeP2P, Text: "   "}},
		},
		{
			name: "未知会话类型不回执",
			gate: channelAckGate{SiteId: 1, Channel: channel(1, 7), Event: textEvent(""), Bound: true, AiAvailable: true},
		},
		{
			name: "已授权群目标回执",
			gate: channelAckGate{SiteId: 1, Channel: channel(1, 7), Event: textEvent(ChatTypeGroup), Bound: true, AiAvailable: true, GroupTargetReady: true},
			want: true,
		},
		{
			name: "未批准群目标不回执",
			gate: channelAckGate{SiteId: 1, Channel: channel(1, 7), Event: textEvent(ChatTypeGroup), Bound: true, AiAvailable: true, GroupTargetReady: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gate.allowed(); got != tt.want {
				t.Fatalf("allowed() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

// TestFeishuAcknowledgeUsesMessageReaction 断言飞书回执按消息 ID 添加 OnIt 表情，
// 而不是发一条额外消息气泡。
func TestFeishuAcknowledgeUsesMessageReaction(t *testing.T) {
	var gotMessageId, gotEmoji string
	adapter := &feishuAdapter{creds: PlatformCredentials{AppId: "feishu_ack_app", AppSecret: "secret"}}
	adapter.createReaction = func(_ context.Context, messageId, emojiType string) (*larkim.CreateMessageReactionResp, error) {
		gotMessageId, gotEmoji = messageId, emojiType
		return &larkim.CreateMessageReactionResp{}, nil
	}

	if err := adapter.Acknowledge(context.Background(), InboundEvent{EventId: " om_ack_1 "}); err != nil {
		t.Fatalf("飞书回执失败：%v", err)
	}
	if gotMessageId != "om_ack_1" {
		t.Fatalf("表情必须贴在原消息上，messageId = %q", gotMessageId)
	}
	if gotEmoji != feishuAckEmojiType {
		t.Fatalf("表情类型 = %q, 期望 %q", gotEmoji, feishuAckEmojiType)
	}
	if feishuAckEmojiType != "OnIt" {
		t.Fatalf("回执语义必须是「收到，马上处理」（OnIt），实际 %q", feishuAckEmojiType)
	}
}

// TestFeishuAcknowledgeErrors 锁住两类失败：缺消息 ID 是本地 permanent，
// 平台权限不足也必须分类成 permanent（否则调用方会误以为可重试）。
func TestFeishuAcknowledgeErrors(t *testing.T) {
	adapter := &feishuAdapter{creds: PlatformCredentials{AppId: "feishu_ack_app", AppSecret: "secret"}}

	err := adapter.Acknowledge(context.Background(), InboundEvent{EventId: "   "})
	var sendErr *SendError
	if !errors.As(err, &sendErr) || sendErr.Class != ErrorClassPermanent {
		t.Fatalf("缺消息 ID 必须是 permanent，实际 %v", err)
	}

	adapter.createReaction = func(context.Context, string, string) (*larkim.CreateMessageReactionResp, error) {
		return &larkim.CreateMessageReactionResp{
			CodeError: larkcore.CodeError{Code: 99991672, Msg: "应用未开通所需权限"},
		}, nil
	}
	err = adapter.Acknowledge(context.Background(), InboundEvent{EventId: "om_denied"})
	if !errors.As(err, &sendErr) || sendErr.Class != ErrorClassPermanent {
		t.Fatalf("权限不足必须是 permanent，实际 %v", err)
	}
}

// TestDingTalkAcknowledgePostsTextEmotion 断言钉钉回执打到表情接口，
// 请求体同时带消息 ID 与会话 ID（缺任一项平台都定位不到这条消息）。
func TestDingTalkAcknowledgePostsTextEmotion(t *testing.T) {
	const appId = "ding_ack_app"
	dingTalkTokenCache.invalidate(appId)
	t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })

	emotionCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case dingTalkTokenEndpoint:
			_, _ = w.Write([]byte(`{"accessToken":"token_ack","expireIn":7200}`))
		case dingTalkEmotionReplyEndpoint:
			emotionCalls++
			if got := r.Header.Get("x-acs-dingtalk-access-token"); got != "token_ack" {
				t.Errorf("表情请求应带 access token，实际 %q", got)
			}
			var body dingTalkEmotionRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("表情请求体不是合法 JSON：%v", err)
				return
			}
			if body.RobotCode != appId {
				t.Errorf("robotCode = %q, 期望 %q", body.RobotCode, appId)
			}
			if body.OpenMsgID != "msg_ack_1" || body.OpenConversationID != "cid_ack_1" {
				t.Errorf("消息/会话标识 = %q/%q，期望 msg_ack_1/cid_ack_1", body.OpenMsgID, body.OpenConversationID)
			}
			if body.EmotionType != dingTalkEmotionTypeCustomText {
				t.Errorf("emotionType = %d, 期望 %d", body.EmotionType, dingTalkEmotionTypeCustomText)
			}
			if body.TextEmotion.Text != dingTalkAckEmotionText || body.TextEmotion.EmotionID != dingTalkCustomTextEmotionID {
				t.Errorf("文字表情 = %+v，期望收到（%s）", body.TextEmotion, dingTalkCustomTextEmotionID)
			}
			// 成功时平台可能返回空 2xx 体：不得因此判失败。
		default:
			t.Errorf("未预期的请求路径 %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: appId, AppSecret: "secret", OrganizationId: "corp_ack"},
		server.URL, server.Client(),
	)
	if err := adapter.Acknowledge(context.Background(), InboundEvent{EventId: "msg_ack_1", ChatId: "cid_ack_1"}); err != nil {
		t.Fatalf("钉钉回执失败：%v", err)
	}
	if emotionCalls != 1 {
		t.Fatalf("表情接口调用 %d 次，期望 1 次", emotionCalls)
	}
}

// TestDingTalkAcknowledgeErrors 锁住缺标识（本地 permanent）与平台拒绝（transient）。
func TestDingTalkAcknowledgeErrors(t *testing.T) {
	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: "ding_ack_err", AppSecret: "secret"},
		"http://127.0.0.1:1", nil,
	)
	err := adapter.Acknowledge(context.Background(), InboundEvent{EventId: "", ChatId: "cid"})
	var sendErr *SendError
	if !errors.As(err, &sendErr) || sendErr.Class != ErrorClassPermanent {
		t.Fatalf("缺消息标识必须是 permanent，实际 %v", err)
	}

	const appId = "ding_ack_rejected"
	dingTalkTokenCache.invalidate(appId)
	t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case dingTalkTokenEndpoint:
			_, _ = w.Write([]byte(`{"accessToken":"token_rej","expireIn":7200}`))
		case dingTalkEmotionReplyEndpoint:
			_, _ = w.Write([]byte(`{"success":false}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter = newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: appId, AppSecret: "secret"},
		server.URL, server.Client(),
	)
	err = adapter.Acknowledge(context.Background(), InboundEvent{EventId: "msg_rej", ChatId: "cid_rej"})
	if !errors.As(err, &sendErr) || sendErr.Class != ErrorClassTransient {
		t.Fatalf("平台明确未接受回执应按 transient 返回，实际 %v", err)
	}
}

// TestChannelWorkerAcknowledgesAfterPersist 锁住 worker 接线：
// 只有「已绑定 + 会被执行」的消息在落库后才回执，其余一律不贴表情。
func TestChannelWorkerAcknowledgesAfterPersist(t *testing.T) {
	env := newChannelInboxTestEnv(t, channelInboxTestConfig{
		policy:      `{"intents":{"api":{"*":"allow"}}}`,
		whitelist:   []string{"api"},
		permissions: []string{"/"},
	})
	ack := &ackRecordingAdapter{}
	wk := &channelWorker{
		manager: &channelManager{site: env.w, opts: defaultChannelManagerOptions()},
		channel: *env.channel,
		ctx:     context.Background(),
		cancel:  func() {},
	}
	deliver := func(ev InboundEvent) {
		t.Helper()
		ev.OrganizationId = env.channel.OrgId
		ev.AppId = env.channel.AppId
		if err := wk.handleInbound(context.Background(), ack, ev); err != nil {
			t.Fatalf("入站处理失败：%v", err)
		}
	}

	t.Run("已绑定私聊文本回执", func(t *testing.T) {
		ev := InboundEvent{
			EventId:  fmt.Sprintf("ack-evt-%d", time.Now().UnixNano()),
			Kind:     InboundKindText,
			UserId:   env.binding.PlatformUserId,
			ChatId:   env.binding.PlatformUserId,
			ChatType: ChatTypeP2P,
			Text:     "你好",
		}
		deliver(ev)
		got := waitAckEvent(t, ack, 1)
		if got.EventId != ev.EventId || got.ChatId != ev.ChatId {
			t.Fatalf("回执必须携带原消息与会话标识，实际 %+v", got)
		}
		// 回执只发生在落库成功之后：此时任务行必须已经存在。
		var row model.ChannelInbox
		hash := wantEventDigest(env.w.Id, env.channel.Id, env.channel.Platform, ev.EventId)
		if err := env.w.DB.Where("event_hash = ?", hash).Take(&row).Error; err != nil || row.Id == 0 {
			t.Fatalf("回执前必须已落库，查询失败：%v", err)
		}
	})

	t.Run("未绑定不回执", func(t *testing.T) {
		before := ack.count()
		deliver(InboundEvent{
			EventId:  fmt.Sprintf("ack-stranger-%d", time.Now().UnixNano()),
			Kind:     InboundKindText,
			UserId:   "ack_stranger_" + channelPermSuffix(),
			ChatId:   "ack_stranger_chat",
			ChatType: ChatTypeP2P,
			Text:     "你好",
		})
		expectNoAck(t, ack, before)
	})

	t.Run("非文本不回执", func(t *testing.T) {
		before := ack.count()
		deliver(InboundEvent{
			EventId:  fmt.Sprintf("ack-image-%d", time.Now().UnixNano()),
			Kind:     "image",
			UserId:   env.binding.PlatformUserId,
			ChatId:   env.binding.PlatformUserId,
			ChatType: ChatTypeP2P,
		})
		expectNoAck(t, ack, before)
	})

	t.Run("群目标未批准不回执", func(t *testing.T) {
		before := ack.count()
		chatId := "ack_group_pending_" + channelPermSuffix()
		env.newGroupTarget(t, chatId, 0, 0)
		deliver(InboundEvent{
			EventId:      fmt.Sprintf("ack-group-pending-%d", time.Now().UnixNano()),
			Kind:         InboundKindText,
			UserId:       env.binding.PlatformUserId,
			ChatId:       chatId,
			ChatType:     ChatTypeGroup,
			Text:         "查看站点",
			MentionedBot: true,
		})
		expectNoAck(t, ack, before)
	})

	t.Run("已授权群目标回执", func(t *testing.T) {
		before := ack.count()
		chatId := "ack_group_ok_" + channelPermSuffix()
		env.newGroupTarget(t, chatId, 1, 1)
		ev := InboundEvent{
			EventId:      fmt.Sprintf("ack-group-ok-%d", time.Now().UnixNano()),
			Kind:         InboundKindText,
			UserId:       env.binding.PlatformUserId,
			ChatId:       chatId,
			ChatType:     ChatTypeGroup,
			Text:         "查看站点",
			MentionedBot: true,
		}
		deliver(ev)
		got := waitAckEvent(t, ack, before+1)
		if got.EventId != ev.EventId {
			t.Fatalf("群回执事件 = %q, 期望 %q", got.EventId, ev.EventId)
		}
	})
}

func waitAckEvent(t *testing.T, adapter *ackRecordingAdapter, want int) InboundEvent {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if adapter.count() >= want {
			event, _ := adapter.last()
			return event
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待第 %d 次回执超时，实际 %d 次", want, adapter.count())
	return InboundEvent{}
}

// expectNoAck 给异步回执留出足够窗口后再断言没有新增：
// 用固定窗口而不是「立刻断言」，否则可能把「还没来得及发」误判成「不会发」。
func expectNoAck(t *testing.T, adapter *ackRecordingAdapter, before int) {
	t.Helper()
	time.Sleep(150 * time.Millisecond)
	if got := adapter.count(); got != before {
		t.Fatalf("这条消息不该回执，实际多出 %d 次", got-before)
	}
}

// TestDingTalkPostJSONAcceptsEmptySuccessBody 锁住「2xx + 空体 = 成功」：
// 表情接口成功时不保证返回 JSON，把空体当解析失败会让一次成功的回执被记成错误。
func TestDingTalkPostJSONAcceptsEmptySuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var out struct {
		Success *bool `json:"success"`
	}
	if _, err := dingTalkPostJSON(context.Background(), server.Client(), server.URL, "/empty", "", map[string]string{}, &out); err != nil {
		t.Fatalf("2xx 空响应体应按成功处理，实际 %v", err)
	}
	if out.Success != nil {
		t.Fatalf("空响应体不应写入结果，实际 %+v", out)
	}

	// 反向对照：非空但坏掉的 JSON 仍然必须报错。
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("{", 3)))
	}))
	defer server2.Close()
	if _, err := dingTalkPostJSON(context.Background(), server2.Client(), server2.URL, "/bad", "", map[string]string{}, &out); err == nil {
		t.Fatal("非法 JSON 响应必须报错")
	}
}
