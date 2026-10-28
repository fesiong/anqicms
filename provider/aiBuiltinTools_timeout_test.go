package provider

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestWithinRoot 路径归属判定要覆盖三种边界。
//
// 2026-10-27 实测：projectRoot 带尾斜杠时，strings.HasPrefix 会把**根目录自身**
// 判成越界（fs_list_dir path="." 报"超出项目目录范围"），而纯前缀比较又会
// 放过 ".../anqicms-backup" 这种同前缀兄弟目录。
func TestWithinRoot(t *testing.T) {
	root := "/srv/site/anqicms" // 不带尾斜杠的规范形态
	cases := []struct {
		p    string
		want bool
	}{
		{"/srv/site/anqicms", true},
		{"/srv/site/anqicms/", true},
		{"/srv/site/anqicms/cache/a.txt", true},
		{"/srv/site/anqicms-backup/x", false},
		{"/srv/site/anqicmsX", false},
		{"/etc/passwd", false},
	}
	for _, c := range cases {
		if got := withinRoot(c.p, root); got != c.want {
			t.Errorf("withinRoot(%q, %q) = %v，期望 %v", c.p, root, got, c.want)
		}
	}
	// 带尾斜杠的 projectRoot（配置里常见写法）结果必须一致。
	for _, c := range cases {
		if got := withinRoot(c.p, root+"/"); got != c.want {
			t.Errorf("withinRoot(%q, %q) = %v，期望 %v（尾斜杠不该改变判定）", c.p, root+"/", got, c.want)
		}
	}
}

// TestBashTimeoutActuallyKills 超时必须真的终止进程，而不是等到命令自己结束。
//
// 2026-10-27 实测：此前用 exec.Command + cmd.Run，ctx 到期对 Run 毫无影响——
// timeout=1 的 `sleep 5` 仍然等满 5 秒才返回，等于没有超时。
func TestBashTimeoutActuallyKills(t *testing.T) {
	svc := tempFileService(t)
	handler, ok := svc.Handlers["bash"]
	if !ok {
		t.Skip("bash 工具未注册")
	}
	start := time.Now()
	out, err := handler(context.Background(), `{"command":"sleep 5","timeout":1}`)
	if err != nil {
		t.Fatalf("超时应以文本回执返回而不是 Go error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 3*time.Second {
		t.Errorf("超时未生效：耗时 %v（应在 1s 左右返回）", elapsed)
	}
	if !strings.HasPrefix(out, "错误：") || !strings.Contains(out, "超时") {
		t.Errorf("超时回执应带文本失败前缀与原因，实际: %q", out)
	}
}

// TestBashNonZeroExitIsError 非零退出码必须走文本失败前缀（触发 MCP isError）。
func TestBashNonZeroExitIsError(t *testing.T) {
	svc := tempFileService(t)
	handler, ok := svc.Handlers["bash"]
	if !ok {
		t.Skip("bash 工具未注册")
	}
	out, err := handler(context.Background(), `{"command":"exit 3","timeout":10}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "错误：") {
		t.Errorf("非零退出码应以「错误：」开头，实际: %q", out)
	}
}
