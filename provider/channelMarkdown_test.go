package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Markdown 分段：切点优先空行，其次换行；围栏跨段时必须闭合并在下一段重开。
func TestSplitMarkdownReopensFence(t *testing.T) {
	text := "```go\n" + strings.Repeat("fmt.Println(\"x\")\n", 40) + "```\n"
	segments := splitMarkdownText(text, 60)
	if len(segments) < 2 {
		t.Fatalf("应被分段的围栏代码只得到 %d 段", len(segments))
	}
	for i, seg := range segments {
		if strings.Count(seg, "```")%2 != 0 {
			t.Fatalf("第 %d 段围栏未配对：%q", i, seg)
		}
		if i > 0 && !strings.HasPrefix(seg, "```go\n") {
			t.Fatalf("第 %d 段没有重开 go 围栏：%q", i, seg)
		}
	}
}

func TestSplitMarkdownPrefersBlankLine(t *testing.T) {
	text := strings.Repeat("a", 30) + "\n\n" + strings.Repeat("b", 50)
	segments := splitMarkdownText(text, 50)
	if len(segments) < 2 {
		t.Fatalf("应被分段，实际 %d 段", len(segments))
	}
	if !strings.HasSuffix(segments[0], "\n\n") {
		t.Fatalf("应优先在空行处切，第一段结尾为 %q", segments[0])
	}
}

func TestSplitMarkdownRespectsLimit(t *testing.T) {
	text := "```go\n" + strings.Repeat("fmt.Println(\"长行\")\n", 60) + "```\n"
	for _, max := range []int{40, 60, 100} {
		for i, seg := range splitMarkdownText(text, max) {
			if count := len([]rune(seg)); count > max {
				t.Fatalf("上限 %d：第 %d 段 %d rune 超限", max, i, count)
			}
		}
	}
}

// 围栏行不能被切点劈开：任何一段的末行都不允许是半截反引号。
func TestSplitMarkdownKeepsFenceLineWhole(t *testing.T) {
	text := strings.Repeat("正文", 20) + "\n```go\nfmt.Println(1)\n```\n"
	for _, max := range []int{40, 41, 42, 43, 44, 45} {
		for i, seg := range splitMarkdownText(text, max) {
			last := strings.TrimSpace(seg[strings.LastIndex(seg, "\n")+1:])
			if strings.HasPrefix(last, "`") && last != "```" {
				t.Fatalf("上限 %d：第 %d 段把围栏行切成了 %q", max, i, last)
			}
		}
	}
}

// 飞书：Markdown 用 post + md 标签；纯文本仍是 text。
func TestFeishuOutboundPayloadMarkdown(t *testing.T) {
	msgType, content, err := feishuOutboundPayload(feishuSendRequest{Text: "**粗体**", Markdown: true})
	if err != nil {
		t.Fatalf("构造飞书 Markdown 载荷失败：%v", err)
	}
	if msgType != "post" {
		t.Fatalf("Markdown 的 msg_type = %q，期望 post", msgType)
	}
	var body struct {
		ZhCN struct {
			Content [][]struct {
				Tag  string `json:"tag"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"zh_cn"`
	}
	if err := json.Unmarshal([]byte(content), &body); err != nil {
		t.Fatalf("post 载荷不是合法 JSON：%v", err)
	}
	if len(body.ZhCN.Content) != 1 || len(body.ZhCN.Content[0]) != 1 {
		t.Fatalf("post 载荷结构不对：%s", content)
	}
	block := body.ZhCN.Content[0][0]
	if block.Tag != "md" || block.Text != "**粗体**" {
		t.Fatalf("md 块 = %+v，期望 tag=md、text=**粗体**", block)
	}
}

func TestFeishuTextPathStaysText(t *testing.T) {
	msgType, content, err := feishuOutboundPayload(feishuSendRequest{Text: "**粗体**"})
	if err != nil {
		t.Fatalf("构造飞书文本载荷失败：%v", err)
	}
	if msgType != "text" {
		t.Fatalf("纯文本的 msg_type = %q，期望 text", msgType)
	}
	var body map[string]string
	if err := json.Unmarshal([]byte(content), &body); err != nil {
		t.Fatalf("text 载荷不是合法 JSON：%v", err)
	}
	if body["text"] != "**粗体**" {
		t.Fatalf("纯文本正文 = %q，期望原样透传", body["text"])
	}
}

func TestMarkdownTitleExtraction(t *testing.T) {
	cases := map[string]string{
		"\n## 今日摘要\n正文":         "今日摘要",
		"1. 处理订单\n详情":           "处理订单",
		"> 引用开头\n正文":            "引用开头",
		"":                      "通知",
		strings.Repeat("长", 40): strings.Repeat("长", markdownTitleMaxRunes),
	}
	for input, want := range cases {
		if got := markdownTitle(input); got != want {
			t.Errorf("markdownTitle(%q) = %q，期望 %q", input, got, want)
		}
	}
}

// 钉钉：Markdown 用 sampleMarkdown + {title,text}；纯文本仍是 sampleText + {content}。
func TestDingTalkSendMarkdownUsesSampleMarkdown(t *testing.T) {
	const appId = "ding_markdown"
	dingTalkTokenCache.invalidate(appId)
	t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })

	var mu sync.Mutex
	var gotKey string
	var gotParam struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case dingTalkTokenEndpoint:
			_, _ = w.Write([]byte(`{"accessToken":"token_md","expireIn":7200}`))
		case dingTalkPrivateSendEndpoint:
			var body struct {
				MsgKey   string `json:"msgKey"`
				MsgParam string `json:"msgParam"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			gotKey = body.MsgKey
			_ = json.Unmarshal([]byte(body.MsgParam), &gotParam)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"processQueryKey":"pqk_md"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: appId, AppSecret: "secret"}, server.URL, server.Client(),
	)
	body := "## 今日摘要\n- 订单 1\n- 订单 2"
	if _, err := adapter.Send(context.Background(),
		TargetRef{Kind: TargetKindPrivate, PlatformTargetId: "staff_1"},
		OutboundMessage{Text: body, Markdown: true}); err != nil {
		t.Fatalf("发送 Markdown 失败：%v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotKey != "sampleMarkdown" {
		t.Fatalf("msgKey = %q，期望 sampleMarkdown", gotKey)
	}
	if gotParam.Title != "今日摘要" || gotParam.Text != body {
		t.Fatalf("msgParam = %+v，期望标题「今日摘要」、正文原样", gotParam)
	}
}

func TestDingTalkTextPathStaysSampleText(t *testing.T) {
	const appId = "ding_plain"
	dingTalkTokenCache.invalidate(appId)
	t.Cleanup(func() { dingTalkTokenCache.invalidate(appId) })

	var mu sync.Mutex
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case dingTalkTokenEndpoint:
			_, _ = w.Write([]byte(`{"accessToken":"token_plain","expireIn":7200}`))
		case dingTalkPrivateSendEndpoint:
			var body struct {
				MsgKey string `json:"msgKey"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			gotKey = body.MsgKey
			mu.Unlock()
			_, _ = w.Write([]byte(`{"processQueryKey":"pqk_plain"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter := newDingTalkAdapterWithEndpoint(
		PlatformCredentials{AppId: appId, AppSecret: "secret"}, server.URL, server.Client(),
	)
	if _, err := adapter.Send(context.Background(),
		TargetRef{Kind: TargetKindPrivate, PlatformTargetId: "staff_1"},
		OutboundMessage{Text: "## 标题"}); err != nil {
		t.Fatalf("发送纯文本失败：%v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotKey != "sampleText" {
		t.Fatalf("纯文本 msgKey = %q，期望 sampleText", gotKey)
	}
}
