package sms

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

// 本文件锁死三家适配器共用的 HTTP body 读取契约（裁决 R38）：
// ① 读取上限 64 KiB——代码既然预设了「网关插 HTML／CDN 拦截」这种敌意 body，
// 就不能 io.ReadAll 把整条响应收进内存；② 日志片段 ≤200 字节且必须是合法
// UTF-8——Message 的唯一目的地是 sms_logs.err_msg（规格 §10.4「失败也留痕」），
// utf8mb4 列拒收非法字节序列时整条发送流水写入失败。
// 每条护栏都带反向对照：证明经典错法会被这里的期望值抓住，而不是空转。

func TestSMSBodySnippetStaysValidUTF8(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			// 200 mod 12 = 8：切在第 67 个汉字的第 3 字节上，必须只丢掉半个 rune。
			name: "多字节中文拦截页切在rune中间",
			body: strings.Repeat("网关拦截", 100),
			want: strings.Repeat("网关拦截", 16) + "网关",
		},
		{
			name: "四字节emoji正好落在边界",
			body: strings.Repeat("🚀", 100),
			want: strings.Repeat("🚀", 50),
		},
		{
			name: "纯ASCII超长",
			body: strings.Repeat("x", 500),
			want: strings.Repeat("x", 200),
		},
		{
			// 网关返回 GBK/Latin-1 拦截页：中段非法字节只丢非法部分，其余现场必须留住。
			name: "中段非法字节不得清空现场",
			body: "<html>" + strings.Repeat("\xff\xfe", 10) + "网关拦截</html>",
			want: "<html>网关拦截</html>",
		},
		{
			// 非法字节在开头：整段回退循环会退到空串，现场全丢。
			name: "开头非法字节不得清空现场",
			body: "\xff\xfe<html>网关</html>",
			want: "<html>网关</html>",
		},
		{
			name: "短body原样保留",
			body: "<html>502 Bad Gateway</html>",
			want: "<html>502 Bad Gateway</html>",
		},
		{name: "空body", body: "", want: ""},
		{
			name: "正好200字节不截断",
			body: strings.Repeat("y", 200),
			want: strings.Repeat("y", 200),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := smsBodySnippet([]byte(c.body))
			if !utf8.ValidString(got) {
				t.Fatalf("片段含非法 UTF-8 字节: %q", got)
			}
			if len(got) > 200 {
				t.Fatalf("片段未截断到 200 字节: %d", len(got))
			}
			if got != c.want {
				t.Fatalf("片段不符\n got: %q (%d 字节)\nwant: %q (%d 字节)", got, len(got), c.want, len(c.want))
			}
		})
	}
}

// TestSMSBodySnippetNegativeControls 反向对照：两个经典错法必须被上表的期望值
// 抓住。没有这一条，「期望值写得够不够狠」无从证明——比如把 want 写成空串，
// 上面的测试照样绿。这里逐用例跑错法实现，要求至少一个用例上错法 != want。
func TestSMSBodySnippetNegativeControls(t *testing.T) {
	// 错法①：按字节硬切后直接 string(body)——切在汉字中间会留下非法字节。
	hardCut := func(body []byte) string {
		if len(body) > 200 {
			body = body[:200]
		}
		return string(body)
	}
	// 错法②：整段 utf8.Valid 就退一字节的循环——首个非法字节之后的现场全丢，
	// 非法字节在开头时更是整段清空。
	backoffLoop := func(body []byte) string {
		if len(body) > 200 {
			body = body[:200]
		}
		for len(body) > 0 && !utf8.Valid(body) {
			body = body[:len(body)-1]
		}
		return string(body)
	}
	// 错法③：只删非法字节但不截断——超长 body 会整段进日志。
	noTruncate := func(body []byte) string {
		return strings.ToValidUTF8(string(body), "")
	}

	cases := []struct {
		name string
		body string
		want string
	}{
		{"多字节中文拦截页", strings.Repeat("网关拦截", 100), strings.Repeat("网关拦截", 16) + "网关"},
		{"中段非法字节", "<html>" + strings.Repeat("\xff\xfe", 10) + "网关拦截</html>", "<html>网关拦截</html>"},
		{"开头非法字节", "\xff\xfe<html>网关</html>", "<html>网关</html>"},
		{"纯ASCII超长", strings.Repeat("x", 500), strings.Repeat("x", 200)},
	}
	caught := map[string]int{}
	for _, c := range cases {
		for _, m := range []struct {
			name string
			fn   func([]byte) string
		}{
			{"按字节硬切", hardCut},
			{"整段utf8回退循环", backoffLoop},
			{"只删非法字节不截断", noTruncate},
		} {
			if got := m.fn([]byte(c.body)); got != c.want {
				caught[m.name]++
			}
		}
	}
	for _, name := range []string{"按字节硬切", "整段utf8回退循环", "只删非法字节不截断"} {
		if caught[name] == 0 {
			t.Fatalf("错法「%s」在所有用例上都与期望值相同：期望值抓不住它，护栏空转", name)
		}
		t.Logf("错法「%s」被 %d 个用例抓住", name, caught[name])
	}
	// 正锚：正确实现在同一组用例上必须全部命中期望值（否则上面的「抓住」
	// 可能只是期望值本身写错了）。
	for _, c := range cases {
		if got := smsBodySnippet([]byte(c.body)); got != c.want {
			t.Fatalf("正锚失败：smsBodySnippet(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestReadSMSResponseBodyLimitsTo64KiB(t *testing.T) {
	if smsMaxResponseBytes != 64<<10 {
		t.Fatalf("响应体读取上限必须是 64 KiB, got %d", smsMaxResponseBytes)
	}
	const streamSize = 200 << 10 // 200 KiB，远超上限
	stream := bytes.Repeat([]byte("z"), streamSize)

	got, err := readSMSResponseBody(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	// 反向对照就在断言里：不设上限的实现会返回 200 KiB，这里必须正好 64 KiB。
	if len(got) != smsMaxResponseBytes {
		t.Fatalf("读取了 %d 字节, want 正好 %d（上限没生效或截多了）", len(got), smsMaxResponseBytes)
	}
	if !bytes.Equal(got, stream[:smsMaxResponseBytes]) {
		t.Fatal("读到的不是流的前 64 KiB")
	}

	// 短流必须原样读完，不能被上限逻辑截短。
	short := []byte("<html>502 Bad Gateway</html>")
	got, err = readSMSResponseBody(bytes.NewReader(short))
	if err != nil {
		t.Fatalf("读取短流失败: %v", err)
	}
	if !bytes.Equal(got, short) {
		t.Fatalf("短流被改写: %q", got)
	}

	// 空流。
	got, err = readSMSResponseBody(bytes.NewReader(nil))
	if err != nil || len(got) != 0 {
		t.Fatalf("空流应返回 0 字节且无错误: %q, %v", got, err)
	}
}

// TestReadSMSResponseBodyPropagatesError 读取出错必须原样上报（不能吞成空 body，
// 那会让「HTTP 200 非 JSON」的现场彻底消失）。
func TestReadSMSResponseBodyPropagatesError(t *testing.T) {
	wantErr := io.ErrUnexpectedEOF
	_, err := readSMSResponseBody(io.MultiReader(strings.NewReader("partial"), errReader{wantErr}))
	if err == nil {
		t.Fatal("读取出错必须返回错误")
	}
	if !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("原始错误被吞: %v", err)
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }
