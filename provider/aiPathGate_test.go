package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// ================================================================
// 路径分类回归测试 (P0-1)
//
// 这些用例的存在意义：原 SensitivePathGateMiddleware 只做原始 JSON 子串匹配，
// `../` 与符号链接逃逸能直接绕过。以下用例一旦变绿就不再可能退化回子串匹配。
// ================================================================

func TestClassifyPathInRoot(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"template/index.html",
		"public/static/a.css",
		"cache/tmp.py",
	}
	for _, c := range cases {
		if got := ClassifyPath(c, root); got != PathInRoot {
			t.Errorf("ClassifyPath(%q) = %v, want PathInRoot", c, got)
		}
	}
}

// 部署在系统保护前缀之下的站点（/var/www 等）：站内文件不得被误判为敏感。
//
// 这是实测踩到的真 bug：判定顺序曾把「系统路径」放在「站内」之前，结果
// /var/www/site 下的每一个文件都被判成敏感 —— 站内自动放行彻底失效，
// 且 t.TempDir() 恰好也在 /var/folders 下，测试环境与生产环境一起中招。
func TestClassifyPathSiteUnderSystemPrefixStillInRoot(t *testing.T) {
	roots := []string{"/var/www/site", "/usr/local/site", "/opt/site", "/home/www/site"}
	for _, root := range roots {
		if got := ClassifyPath("template/index.html", root); got != PathInRoot {
			t.Errorf("root=%s: ClassifyPath(template/index.html) = %v, want PathInRoot", root, got)
		}
	}
}

// `..` 逃逸：解析后落在站点根之外 → 必须判为站外而不是站内。
func TestClassifyPathDotDotEscapeIsOutOfRoot(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"../../etc/passwd",
		"template/../../../etc/passwd",
		"a/b/c/../../../../../etc/shadow",
	}
	for _, c := range cases {
		got := ClassifyPath(c, root)
		if got == PathInRoot {
			t.Errorf("ClassifyPath(%q) = PathInRoot，`..` 逃逸未被识别", c)
		}
		if !got.NeedsApproval() {
			t.Errorf("ClassifyPath(%q) = %v，逃逸路径必须需要审批", c, got)
		}
	}
}

// 敏感逃逸：即使逃逸出站点根，命中密钥标记仍应判为敏感（最严格）。
func TestClassifyPathSensitiveEscape(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"../../.ssh/id_rsa",
		"../../../.aws/credentials",
	}
	for _, c := range cases {
		if got := ClassifyPath(c, root); got != PathSensitive {
			t.Errorf("ClassifyPath(%q) = %v, want PathSensitive", c, got)
		}
	}
}

// 符号链接逃逸：站点内的链接指向站点外 → 必须按解析后的真实位置判定。
func TestClassifyPathSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("当前环境不支持符号链接: %v", err)
	}

	got := ClassifyPath("escape", root)
	if got == PathInRoot {
		t.Errorf("符号链接逃逸未被识别：ClassifyPath(\"escape\") = PathInRoot")
	}
	if !got.NeedsApproval() {
		t.Errorf("符号链接逃逸必须需要审批, got %v", got)
	}
}

// 站点内的敏感文件同样是敏感（含数据库口令的 config.toml 不能被自动放行）。
func TestClassifyPathSensitiveInsideRoot(t *testing.T) {
	root := t.TempDir()
	sensitive := []string{
		"config.toml",
		"config/config.json",
		".env",
		".env.local",
		".env.production",
		"cert/private.key",
		"cert/server.pem",
	}
	for _, c := range sensitive {
		if got := ClassifyPath(c, root); got != PathSensitive {
			t.Errorf("ClassifyPath(%q) = %v, want PathSensitive", c, got)
		}
	}
}

// .env 占位模板只含假值且已进版本库，不应弹审批（仿 atomcode ENV_TEMPLATE_SUFFIXES）。
func TestClassifyPathEnvTemplatesNotSensitive(t *testing.T) {
	root := t.TempDir()
	templates := []string{
		".env.example",
		".env.sample",
		".env.template",
		".env.dist",
		".env.defaults",
	}
	for _, c := range templates {
		if got := ClassifyPath(c, root); got != PathInRoot {
			t.Errorf("ClassifyPath(%q) = %v, want PathInRoot（模板不应敏感）", c, got)
		}
	}
}

// 系统保护路径。
func TestClassifyPathSystemProtected(t *testing.T) {
	root := t.TempDir()
	for _, c := range []string{"/etc/passwd", "/root/.bashrc", "/usr/bin/env"} {
		if got := ClassifyPath(c, root); got != PathSensitive {
			t.Errorf("ClassifyPath(%q) = %v, want PathSensitive", c, got)
		}
	}
}

// `~` 必须解析到 home（站点根之外），绝不能退化成「相对站点根」。
// 否则 `rm -rf ~` 会被误判成站内操作而自动放行。
func TestClassifyPathTildeIsOutOfRoot(t *testing.T) {
	root := t.TempDir()
	got := ClassifyPath("~", root)
	if got == PathInRoot {
		t.Errorf("ClassifyPath(\"~\") = PathInRoot，`~` 被误判为站内")
	}
	if !got.NeedsApproval() {
		t.Errorf("`~` 必须需要审批, got %v", got)
	}
}

// RootPath 未配置 → 无法锚定 → fail closed。
func TestClassifyPathWithoutRootFailsClosed(t *testing.T) {
	if got := ClassifyPath("template/index.html", ""); got == PathInRoot {
		t.Error("RootPath 为空时不得判定为站内（无法锚定即无法证明安全）")
	}
	if got := ClassifyPath("template/index.html", ""); !got.NeedsApproval() {
		t.Errorf("RootPath 为空时必须需要审批, got %v", got)
	}
}

// 空路径 → 无法判定 → 需审批（不静默放行）。
func TestClassifyPathEmpty(t *testing.T) {
	if got := ClassifyPath("", t.TempDir()); got != PathUndeterminable {
		t.Errorf("ClassifyPath(\"\") = %v, want PathUndeterminable", got)
	}
}

// ================================================================
// 目标提取
// ================================================================

func TestExtractToolTargetsFilePath(t *testing.T) {
	got := ExtractToolTargets("read_file", `{"file_path":"template/index.html","offset":1}`)
	if len(got) != 1 || got[0] != "template/index.html" {
		t.Fatalf("ExtractToolTargets = %v, want [template/index.html]", got)
	}
}

// bash：命令里的路径才是作用域，不是整条命令。
func TestExtractToolTargetsBash(t *testing.T) {
	got := ExtractToolTargets("bash", `{"command":"cat template/a.html /etc/passwd"}`)
	if len(got) != 2 {
		t.Fatalf("ExtractToolTargets(bash) = %v, want 2 个路径", got)
	}
	// 排序后 /etc/passwd 在前（绝对路径以 / 开头，字典序小于 t）
	if got[0] != "/etc/passwd" || got[1] != "template/a.html" {
		t.Fatalf("ExtractToolTargets(bash) = %v", got)
	}
}

// 非法 JSON 时回落到宽松扫描，绝不静默返回空（空会导致授权键退化）。
func TestExtractToolTargetsFallbackOnBadJSON(t *testing.T) {
	got := ExtractToolTargets("write_file", `{"file_path": "a/b.txt", broken`)
	if len(got) == 0 {
		t.Fatal("非法 JSON 的回落扫描不应返回空")
	}
	if got[0] != "a/b.txt" {
		t.Fatalf("回落扫描 = %v, want [a/b.txt]", got)
	}
}

// ================================================================
// 授权键控 (P0-2)
// ================================================================

func TestGrantKeyIsPerTarget(t *testing.T) {
	a := GrantKey("read_file", []string{"/root/.ssh/id_rsa"}, `{"file_path":"/root/.ssh/id_rsa"}`)
	b := GrantKey("read_file", []string{"/root/.aws/credentials"}, `{"file_path":"/root/.aws/credentials"}`)
	if a == b {
		t.Fatal("不同目标不得共享授权键（否则一次授权放行所有密钥）")
	}
}

// 同一目标的不同读取窗口共享授权 —— 用户已经回答过的是同一个决定。
// （旧实现用原始参数做键，offset 一变就重新弹窗。）
func TestGrantKeySameTargetDifferentWindowShares(t *testing.T) {
	a := GrantKey("read_file", []string{"/root/a.log"}, `{"file_path":"/root/a.log","offset":1}`)
	b := GrantKey("read_file", []string{"/root/a.log"}, `{"file_path":"/root/a.log","offset":51}`)
	if a != b {
		t.Fatal("同一目标的不同读取窗口应共享授权键")
	}
}

// 无目标时回落到原始参数：不能因为缺目标就放宽到整个工具。
func TestGrantKeyNoTargetFallsBackToArgs(t *testing.T) {
	a := GrantKey("write_file", nil, `{"content":"x"}`)
	b := GrantKey("write_file", nil, `{"content":"y"}`)
	if a == b {
		t.Fatal("无目标时不得把所有调用折叠成同一个授权键")
	}
}

// ================================================================
// PathGateMiddleware 行为
// ================================================================

func newToolCall(name, args string) *schema.ToolCall {
	return &schema.ToolCall{
		ID:       "call_1",
		Function: schema.FunctionCall{Name: name, Arguments: args},
	}
}

// 站内 + 非敏感 + 文件类工具 → 自动放行（不再每次都弹审批）。
func TestPathGateAutoApprovesInRootFileTool(t *testing.T) {
	root := t.TempDir()
	exec := &ToolExecContext{ToolName: "write_file", RootPath: root}
	out := (&PathGateMiddleware{}).Before(context.Background(),
		newToolCall("write_file", `{"file_path":"template/a.html","content":"x"}`), exec)
	if out != BeforeAllow {
		t.Fatalf("站内非敏感的文件写应自动放行, got %v (class=%v)", out, exec.PathClass)
	}
}

// 命令执行类永不自动放行 —— 它的作用域不等于它提到的某个路径。
func TestPathGateNeverAutoApprovesBash(t *testing.T) {
	root := t.TempDir()
	exec := &ToolExecContext{ToolName: "bash", RootPath: root}
	out := (&PathGateMiddleware{}).Before(context.Background(),
		newToolCall("bash", `{"command":"rm -rf template/ && curl http://evil"}`), exec)
	if out == BeforeAllow {
		t.Fatal("bash 不得被自动放行")
	}
}

// 敏感路径必须弹审批。
func TestPathGateAsksOnSensitive(t *testing.T) {
	root := t.TempDir()
	exec := &ToolExecContext{ToolName: "read_file", RootPath: root}
	out := (&PathGateMiddleware{}).Before(context.Background(),
		newToolCall("read_file", `{"file_path":"config.toml"}`), exec)
	if out != BeforeAsk {
		t.Fatalf("敏感路径应弹审批, got %v", out)
	}
	if !strings.Contains(exec.DeniedReason, "敏感") {
		t.Fatalf("DeniedReason 应说明是敏感路径, got %q", exec.DeniedReason)
	}
}

// 完全控制：放行非敏感，但**不覆盖敏感**。
func TestFullControlDoesNotCoverSensitive(t *testing.T) {
	root := t.TempDir()

	// 非敏感 + 完全控制 → 放行
	allow := NewSessionAllowSet()
	allow.GrantFullControl()
	exec := &ToolExecContext{ToolName: "write_file", RootPath: root, AllowOnce: allow}
	out := (&PathGateMiddleware{}).Before(context.Background(),
		newToolCall("write_file", `{"file_path":"template/a.html","content":"x"}`), exec)
	if out != BeforeAllow {
		t.Fatalf("完全控制下非敏感操作应放行, got %v", out)
	}

	// 敏感 + 完全控制 → 仍然要问
	exec2 := &ToolExecContext{ToolName: "read_file", RootPath: root, AllowOnce: allow}
	out2 := (&PathGateMiddleware{}).Before(context.Background(),
		newToolCall("read_file", `{"file_path":"config.toml"}`), exec2)
	if out2 != BeforeAsk {
		t.Fatalf("完全控制不得覆盖敏感路径, got %v", out2)
	}
}

// 按目标授权：授权 A 文件后，B 文件仍需审批（旧实现会一并放行）。
func TestPerTargetGrantDoesNotLeakToOtherFiles(t *testing.T) {
	root := t.TempDir()
	allow := NewSessionAllowSet()
	// 模拟用户对 template/a.html 选择了 once_allow
	allow.GrantTarget(GrantKey("write_file", []string{"template/a.html"},
		`{"file_path":"template/a.html","content":"x"}`))

	exec := &ToolExecContext{ToolName: "write_file", RootPath: root, AllowOnce: allow}
	out := (&PathGateMiddleware{}).Before(context.Background(),
		newToolCall("write_file", `{"file_path":"template/a.html","content":"y"}`), exec)
	if out != BeforeAllow {
		t.Fatalf("已授权的目标应放行, got %v", out)
	}

	// 另一个文件不应被同一授权覆盖（这里站内的另一个文件本身也会自动放行，
	// 所以改用「敏感目标」来验证授权不外溢）
	exec2 := &ToolExecContext{ToolName: "read_file", RootPath: root, AllowOnce: allow}
	out2 := (&PathGateMiddleware{}).Before(context.Background(),
		newToolCall("read_file", `{"file_path":".env"}`), exec2)
	if out2 != BeforeAsk {
		t.Fatalf("敏感目标不得被其他授权覆盖, got %v", out2)
	}
}

// Agent 会话不审批（保持既有行为）。
func TestPathGateSkipsAgent(t *testing.T) {
	root := t.TempDir()
	exec := &ToolExecContext{ToolName: "read_file", RootPath: root, IsAgent: true}
	out := (&PathGateMiddleware{}).Before(context.Background(),
		newToolCall("read_file", `{"file_path":"config.toml"}`), exec)
	if out != BeforeProceed {
		t.Fatalf("Agent 会话应跳过审批门, got %v", out)
	}
}

// 无路径语义的工具不拦截。
func TestPathGateSkipsNonPathTool(t *testing.T) {
	root := t.TempDir()
	exec := &ToolExecContext{ToolName: "archive_create", RootPath: root}
	out := (&PathGateMiddleware{}).Before(context.Background(),
		newToolCall("archive_create", `{"title":"x"}`), exec)
	if out != BeforeProceed {
		t.Fatalf("非路径工具不应被 path_gate 拦截, got %v", out)
	}
}

// ================================================================
// 链：新决策与 P0-3 枚举
// ================================================================

// once_allow 记住的是「工具+目标」，且敏感目标不记住。
func TestChainOnceAllowIsPerTargetAndSkipsSensitive(t *testing.T) {
	root := t.TempDir()

	// 非敏感：once_allow 应写入按目标的授权
	allow := NewSessionAllowSet()
	exec := &ToolExecContext{SessionID: "s1", ToolName: "write_file", RootPath: root, AllowOnce: allow,
		ApprovalFn: func(ctx context.Context, c *schema.ToolCall, e *ToolExecContext) (string, string) {
			return "once_allow", ""
		}}
	chain := NewMiddlewareChain(&PathGateMiddleware{})
	// 用一个站外路径强制走审批
	outside := filepath.Join(filepath.Dir(root), "outside-"+filepath.Base(root), "a.txt")
	chain.ExecuteTool(context.Background(), newToolCall("write_file",
		`{"file_path":"`+outside+`","content":"x"}`), func(ctx context.Context, args string) (string, error) {
		return "ok", nil
	}, exec)
	if len(allow.grants) == 0 {
		t.Fatal("once_allow 应写入一条按目标的授权")
	}
	for k := range allow.grants {
		if !strings.HasPrefix(k, "write_file::") {
			t.Fatalf("授权键应以工具名开头, got %q", k)
		}
	}

	// 敏感：once_allow 不得写入任何授权（退化为单次）
	allow2 := NewSessionAllowSet()
	exec2 := &ToolExecContext{SessionID: "s2", ToolName: "read_file", RootPath: root, AllowOnce: allow2,
		ApprovalFn: func(ctx context.Context, c *schema.ToolCall, e *ToolExecContext) (string, string) {
			return "once_allow", ""
		}}
	chain.ExecuteTool(context.Background(), newToolCall("read_file", `{"file_path":"config.toml"}`),
		func(ctx context.Context, args string) (string, error) { return "ok", nil }, exec2)
	if len(allow2.grants) != 0 {
		t.Fatalf("敏感目标的 once_allow 不得被记住, grants=%v", allow2.grants)
	}
}

// full_control 写入完全控制标记。
func TestChainFullControlGrantsSession(t *testing.T) {
	root := t.TempDir()
	allow := NewSessionAllowSet()
	exec := &ToolExecContext{SessionID: "s1", ToolName: "write_file", RootPath: root, AllowOnce: allow,
		ApprovalFn: func(ctx context.Context, c *schema.ToolCall, e *ToolExecContext) (string, string) {
			return "full_control", ""
		}}
	outside := filepath.Join(filepath.Dir(root), "outside2-"+filepath.Base(root), "b.txt")
	chain := NewMiddlewareChain(&PathGateMiddleware{})
	chain.ExecuteTool(context.Background(), newToolCall("write_file",
		`{"file_path":"`+outside+`","content":"x"}`), func(ctx context.Context, args string) (string, error) {
		return "ok", nil
	}, exec)
	if !allow.HasFullControl() {
		t.Fatal("full_control 应授予本会话完全控制")
	}
}

// P0-3: BeforeDenyTurn → 结果标记 TurnTerminated。
type denyTurnMW struct{}

func (denyTurnMW) Name() string { return "deny_turn" }
func (denyTurnMW) Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome {
	exec.DeniedReason = "检测到死循环，终止本轮"
	return BeforeDenyTurn
}
func (denyTurnMW) After(ctx context.Context, r *ToolExecResult, exec *ToolExecContext) AfterOutcome {
	return AfterProceed
}

func TestChainDenyTurnTerminatesRound(t *testing.T) {
	exec := &ToolExecContext{SessionID: "s1", ToolName: "bash", RootPath: t.TempDir()}
	chain := NewMiddlewareChain(denyTurnMW{})
	res, denied, reason := chain.ExecuteTool(context.Background(), newToolCall("bash", `{"command":"ls"}`),
		func(ctx context.Context, args string) (string, error) { return "should-not-run", nil }, exec)
	if !denied {
		t.Fatal("DenyTurn 应视为被拒绝")
	}
	if !res.TurnTerminated {
		t.Fatal("DenyTurn 应设置 TurnTerminated，供调用方跳出本轮")
	}
	if reason == "" {
		t.Fatal("DenyTurn 应带原因")
	}
}

// P0-3: AfterBlock → 原因回灌给模型（覆盖原始输出）。
type blockMW struct{}

func (blockMW) Name() string { return "block" }
func (blockMW) Before(ctx context.Context, call *schema.ToolCall, exec *ToolExecContext) BeforeOutcome {
	return BeforeProceed
}
func (blockMW) After(ctx context.Context, r *ToolExecResult, exec *ToolExecContext) AfterOutcome {
	exec.DeniedReason = "工具输出包含未脱敏的密钥，已拦截"
	return AfterBlock
}

func TestChainAfterBlockFeedsReasonBackToModel(t *testing.T) {
	exec := &ToolExecContext{SessionID: "s1", ToolName: "read_file", RootPath: t.TempDir()}
	chain := NewMiddlewareChain(blockMW{})
	res, _, _ := chain.ExecuteTool(context.Background(), newToolCall("read_file", `{"file_path":"a.txt"}`),
		func(ctx context.Context, args string) (string, error) { return "原始输出 secret=xxx", nil }, exec)
	if !res.Blocked {
		t.Fatal("AfterBlock 应设置 Blocked")
	}
	if !strings.Contains(res.Content, "已拦截") || strings.Contains(res.Content, "原始输出") {
		t.Fatalf("AfterBlock 应把阻断原因回灌给模型并覆盖原始输出, got %q", res.Content)
	}
}
