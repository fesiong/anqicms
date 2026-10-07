package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// ================================================================
// 审批门的 action 粒度判定 + 回合级合并审批结论的消费
//
// 这两个测试存在的原因：
//   1. 合并意图（content_article 的 list/get/save/delete）按工具名判定会让纯查询也弹窗；
//   2. 旧的 Risk 集合只认 write，RiskSystem 的 shell/文件写意图完全绕过审批门；
//   3. 回合级合并审批依赖 PreviewCallApproval 与门禁判定**逐条一致**，
//      漂移会让用户被弹两次窗（预判漏了）或白批一次（预判多问了）。
// ================================================================

func TestWriteGateIsActionGranular(t *testing.T) {
	cases := []struct {
		name    string
		tool    string
		args    string
		wantAsk bool
	}{
		{"合并意图的查询动作不问", "content_article", `{"action":"list","page":1}`, false},
		{"合并意图的查询动作不问(get)", "content_article", `{"action":"get","id":1}`, false},
		{"合并意图的写动作要问", "content_article", `{"action":"save","title":"x"}`, true},
		{"合并意图的删除要问", "content_article", `{"action":"delete","id":1}`, true},
		{"只读意图不问", "traffic_statistics", `{"action":"dashboard"}`, false},
		{"通用调用的发现动作不问", "api", `{"action":"list","ns":"archive"}`, false},
		{"通用调用的只读 invoke 不问", "api", `{"action":"invoke","method":"GET","path":"/archive/list"}`, false},
		{"通用调用的写 invoke 要问", "api", `{"action":"invoke","method":"POST","path":"/archive/detail"}`, true},
		{"system 级 shell 要问（曾是绕过）", "shell_exec", `{"command":"ls"}`, true},
		{"system 级文件写要问（曾是绕过）", "fs_write", `{"path":"/etc/hosts","content":"x"}`, true},
		{"内置只读工具不问", "read_file", `{"file_path":"a.txt"}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			exec := &ToolExecContext{ToolName: c.tool, RootPath: t.TempDir()}
			out := (&WriteGateMiddleware{}).Before(context.Background(), newToolCall(c.tool, c.args), exec)
			gotAsk := out == BeforeAsk
			if gotAsk != c.wantAsk {
				t.Fatalf("%s %s → outcome=%v，期望 ask=%v", c.tool, c.args, out, c.wantAsk)
			}
		})
	}
}

func newCall(id, name, args string) *schema.ToolCall {
	return &schema.ToolCall{ID: id, Function: schema.FunctionCall{Name: name, Arguments: args}}
}

// once_allow 的授权粒度必须停在 action：批了 save 不能连带放行 delete。
func TestOnceAllowIsActionScoped(t *testing.T) {
	root := t.TempDir()
	allow := NewSessionAllowSet()
	allow.GrantTarget(ApprovalGrantKey("content_article", &ToolExecContext{RootPath: root}, `{"action":"save","title":"x"}`))

	asked := func(args string) bool {
		exec := &ToolExecContext{ToolName: "content_article", RootPath: root, AllowOnce: allow}
		return (&WriteGateMiddleware{}).Before(context.Background(), newCall("c", "content_article", args), exec) == BeforeAsk
	}
	if asked(`{"action":"save","title":"y"}`) {
		t.Error("已授权的 save 动作不应再问（参数不同也应视为同一动作）")
	}
	if !asked(`{"action":"delete","id":9}`) {
		t.Error("delete 动作不得因为 save 被授权而放行")
	}
}

// 完全控制对无路径的站点写操作同样生效（旧实现只有路径门认它）。
func TestFullControlCoversCmsWrites(t *testing.T) {
	allow := NewSessionAllowSet()
	allow.GrantFullControl()
	exec := &ToolExecContext{ToolName: "content_article", RootPath: t.TempDir(), AllowOnce: allow}
	out := (&WriteGateMiddleware{}).Before(context.Background(),
		newCall("c", "content_article", `{"action":"save","title":"x"}`), exec)
	if out != BeforeAllow {
		t.Fatalf("完全控制下站点写操作应放行, got %v", out)
	}
}

// 回合级合并审批的结论由门禁消费：预批准→放行，预拒绝→拒绝且不执行。
func TestPreApprovedAndPreDeniedAreHonored(t *testing.T) {
	root := t.TempDir()
	preApproved := map[string]bool{"call_a": true}
	preDenied := map[string]string{"call_b": "用户拒绝了此操作"}

	exec := &ToolExecContext{ToolName: "content_article", RootPath: root,
		PreApproved: preApproved, PreDenied: preDenied}
	if out := (&WriteGateMiddleware{}).Before(context.Background(),
		newCall("call_a", "content_article", `{"action":"save","title":"x"}`), exec); out != BeforeAllow {
		t.Fatalf("预批准应放行, got %v", out)
	}

	exec2 := &ToolExecContext{ToolName: "content_article", RootPath: root,
		PreApproved: preApproved, PreDenied: preDenied}
	out := (&WriteGateMiddleware{}).Before(context.Background(),
		newCall("call_b", "content_article", `{"action":"delete","id":1}`), exec2)
	if out != BeforeDeny {
		t.Fatalf("预拒绝应阻止执行, got %v", out)
	}
	if exec2.DeniedReason == "" {
		t.Error("预拒绝应带上原因")
	}
}

// 被拒项真的不执行：整条链上 handler 不能被调用。
func TestPreDeniedCallDoesNotRunHandler(t *testing.T) {
	root := t.TempDir()
	chain := NewMiddlewareChain(&PathGateMiddleware{}, &WriteGateMiddleware{})
	exec := &ToolExecContext{ToolName: "content_article", RootPath: root,
		PreDenied: map[string]string{"call_b": "用户拒绝了此操作"}}
	called := false
	res, denied, _ := chain.ExecuteTool(context.Background(),
		newCall("call_b", "content_article", `{"action":"delete","id":1}`),
		func(ctx context.Context, args string) (string, error) { called = true; return "ok", nil },
		exec)
	if called {
		t.Fatal("被拒的调用不应到达 handler")
	}
	if !denied || !res.IsError {
		t.Fatalf("被拒调用应返回 denied+IsError, denied=%v isError=%v", denied, res.IsError)
	}
}

// 意图化的文件工具也要过路径门：fs_read 读 config.toml（敏感）必须问。
// 旧实现按能力名匹配工具，意图名整体跳过路径门，等于开了个读密钥的口子。
func TestPathGateCoversIntentFileTools(t *testing.T) {
	root := t.TempDir()
	if err := writeTestFile(root, "config.toml", "secret=1"); err != nil {
		t.Fatal(err)
	}
	exec := &ToolExecContext{ToolName: "fs_read", RootPath: root}
	out := (&PathGateMiddleware{}).Before(context.Background(),
		newCall("c", "fs_read", `{"file_path":"config.toml"}`), exec)
	if out != BeforeAsk {
		t.Fatalf("fs_read 读敏感文件应弹审批, got %v (class=%v)", out, exec.PathClass)
	}
}

// PreviewCallApproval 必须与门禁判定一致（合并审批的地基）。
func TestPreviewMatchesMiddleware(t *testing.T) {
	root := t.TempDir()
	if err := writeTestFile(root, "config.toml", "secret=1"); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		tool string
		args string
	}{
		{"content_article", `{"action":"list"}`},
		{"content_article", `{"action":"save","title":"x"}`},
		{"traffic_statistics", `{"action":"dashboard"}`},
		{"api", `{"action":"invoke","method":"POST","path":"/archive/detail"}`},
		{"shell_exec", `{"command":"ls"}`},
		{"fs_write", `{"file_path":"template/a.html","content":"x"}`},
		{"fs_read", `{"file_path":"config.toml"}`},
	}
	for _, c := range cases {
		p := PreviewCallApproval("id", c.tool, c.args, root, nil)
		exec := &ToolExecContext{ToolName: c.tool, RootPath: root}
		pathOut := (&PathGateMiddleware{}).Before(context.Background(), newCall("id", c.tool, c.args), exec)
		writeOut := BeforeProceed
		if pathOut != BeforeAllow && pathOut != BeforeDeny {
			writeOut = (&WriteGateMiddleware{}).Before(context.Background(), newCall("id", c.tool, c.args), exec)
		}
		asked := pathOut == BeforeAsk || writeOut == BeforeAsk
		if asked != p.Needs {
			t.Errorf("%s %s: 门禁会问=%v，预判 Needs=%v（漂移会导致重复弹窗或漏批）", c.tool, c.args, asked, p.Needs)
		}
	}
}

func writeTestFile(dir, name, content string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
}
