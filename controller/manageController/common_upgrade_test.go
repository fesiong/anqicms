package manageController

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kandaoni.com/anqicms/provider"
)

// 升级换主程序这一段的历史形态是「界面报成功、跑的仍是旧版本」：
// execPath 取自 os.Executable()，而 Linux 在内核层会把正在运行的程序路径跟着 rename 改动，
// 于是升级后没重启再升一次，就会把 xxx.old 当主程序备份成 xxx.old.old，
// 真正的 xxx 从头到尾没被替换，返回值却还是 StatusOK。
// 下面把「路径归一化」「替换语义」「两道闸门是否还在」逐条钉死。

func TestNormalizeExecPath(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		want   string
		wasOld bool
	}{
		{"正常路径", "/opt/app/xlspider", "/opt/app/xlspider", false},
		{"进程跑在备份上", "/opt/app/xlspider.old", "/opt/app/xlspider", true},
		{"已套娃两层", "/opt/app/xlspider.old.old", "/opt/app/xlspider", true},
		{"已套娃三层", "/opt/app/xlspider.old.old.old", "/opt/app/xlspider", true},
		// 升级把旧备份删掉后，Linux 会在 /proc/self/exe 后面追加这个标记，它不是文件名。
		{"inode 已删除", "/opt/app/xlspider (deleted)", "/opt/app/xlspider", false},
		{"删除 + 备份", "/opt/app/xlspider.old (deleted)", "/opt/app/xlspider", true},
		{"windows 备份", `C:\app\xlspider.exe.old`, `C:\app\xlspider.exe`, true},
		{"空值", "", "", false},
	}

	for _, c := range cases {
		got, wasOld := provider.NormalizeExecPath(c.raw)
		if got != c.want || wasOld != c.wasOld {
			t.Errorf("%s: NormalizeExecPath(%q) = (%q, %v), want (%q, %v)",
				c.name, c.raw, got, wasOld, c.want, c.wasOld)
		}
	}
}

func TestSwapExecutableReplacesBinaryAndKeepsBackup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "xlspider")
	next := target + ".new"

	writeTestFile(t, target, "old-binary")
	writeTestFile(t, next, "new-binary")

	if err := swapExecutable(next, target); err != nil {
		t.Fatalf("替换应当成功，却返回：%v", err)
	}
	if got := readTestFile(t, target); got != "new-binary" {
		t.Errorf("主程序内容未替换：%q", got)
	}
	// 备份必须留着：新程序起不来时这是唯一的回滚手段。
	if got := readTestFile(t, target+".old"); got != "old-binary" {
		t.Errorf("旧程序备份内容不对：%q", got)
	}
	if _, err := os.Stat(next); !os.IsNotExist(err) {
		t.Errorf("临时文件 %s 应已被移走", next)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("取主程序权限失败：%v", err)
		}
		if info.Mode().Perm()&0o100 == 0 {
			t.Errorf("主程序丢了可执行位：%v", info.Mode().Perm())
		}
	}
}

// 备份路径必须由目标文件自身推出。旧实现把它拼在 config.ExecPath 下，
// 程序不在工作目录时会算出另一条路径，rename 全落空却仍返回成功。
func TestSwapExecutableBackupStaysNextToTarget(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "bin")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(sub, "xlspider")
	next := target + ".new"
	writeTestFile(t, target, "old-binary")
	writeTestFile(t, next, "new-binary")

	if err := swapExecutable(next, target); err != nil {
		t.Fatalf("替换应当成功，却返回：%v", err)
	}
	if _, err := os.Stat(filepath.Join(sub, "xlspider.old")); err != nil {
		t.Errorf("备份没有落在主程序同目录：%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "xlspider.old")); err == nil {
		t.Errorf("备份跑到了上一级目录")
	}
}

// 反向对照：主程序不存在时必须报错并指出缺失路径。
// 旧实现在这一步只会打一行日志，然后照样回「升级成功」。
func TestSwapExecutableReportsMissingTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "not-here")
	next := filepath.Join(dir, "new")
	writeTestFile(t, next, "new-binary")

	err := swapExecutable(next, target)
	if err == nil {
		t.Fatal("主程序不存在时不应返回成功")
	}
	if !strings.Contains(err.Error(), target) {
		t.Errorf("错误信息没有带出缺失路径 %s：%v", target, err)
	}
}

func TestSwapExecutableCleansNestedBackups(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "xlspider")
	writeTestFile(t, target, "old-binary")
	// 模拟已经套娃的机器：.old.old / .old.old.old 都是"升级后未重启"的产物。
	writeTestFile(t, target+".old.old", "nested-2")
	writeTestFile(t, target+".old.old.old", "nested-3")
	next := target + ".new"
	writeTestFile(t, next, "new-binary")

	if err := swapExecutable(next, target); err != nil {
		t.Fatalf("替换应当成功，却返回：%v", err)
	}
	for _, p := range []string{target + ".old.old", target + ".old.old.old"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("嵌套备份 %s 未被清理", filepath.Base(p))
		}
	}
	if got := readTestFile(t, target+".old"); got != "old-binary" {
		t.Errorf("单层备份被误删/写错：%q", got)
	}
}

// VersionUpgrade 的两道闸门必须是真调用，不能只在注释里说。
// 用 go/ast 读源码，避免"改回原样也测得过"的空转断言。
func TestVersionUpgradeKeepsExecutableGuards(t *testing.T) {
	src, err := os.ReadFile("common.go")
	if err != nil {
		t.Fatalf("读不到 common.go：%v", err)
	}
	calls := collectCallees(t, src)
	for _, want := range []string{"provider.NormalizeExecPath", "swapExecutable"} {
		if calls[want] == 0 {
			t.Errorf("VersionUpgrade 里少了 %s：入口守卫或替换逻辑被绕过", want)
		}
	}

	// 反向对照：旧实现（直接 execPath + ".old" 后 rename、失败只打日志）必须被判为不合格。
	const mutated = `package manageController

import "os"

func VersionUpgrade() {
	path, _ := os.Executable()
	execPath := path
	oldPath := execPath + ".old"
	_ = os.Rename(execPath, oldPath)
	_ = os.Rename(execPath+".new", execPath)
	_ = os.Remove(oldPath)
}`
	got := collectCallees(t, []byte(mutated))
	if got["swapExecutable"] != 0 {
		t.Errorf("反向对照失效：检查器把旧实现的裸 rename 当成了 swapExecutable")
	}
	if got["provider.NormalizeExecPath"] != 0 {
		t.Errorf("反向对照失效：旧实现不应含路径归一化")
	}
	if got["os.Rename"] == 0 {
		t.Errorf("反向对照失效：检查器连 os.Rename 都没数到，说明收集逻辑本身是空转的")
	}
}

// collectCallees 统计 VersionUpgrade 函数体内出现的被调方名字
// （Ident 记 "rename"，SelectorExpr 记 "os.Rename"）。
func collectCallees(t *testing.T, src []byte) map[string]int {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "common.go", src, 0)
	if err != nil {
		t.Fatalf("解析源码失败：%v", err)
	}
	calls := make(map[string]int)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "VersionUpgrade" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch f := call.Fun.(type) {
			case *ast.Ident:
				calls[f.Name]++
			case *ast.SelectorExpr:
				if x, ok := f.X.(*ast.Ident); ok {
					calls[x.Name+"."+f.Sel.Name]++
				}
			}
			return true
		})
	}
	return calls
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("写测试文件 %s 失败：%v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读测试文件 %s 失败：%v", path, err)
	}
	return string(raw)
}
