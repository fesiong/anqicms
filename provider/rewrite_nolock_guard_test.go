package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"sync"
	"testing"

	"kandaoni.com/anqicms/config"
)

// TestParsePatternReadPathUnlocked 锁住「URL 重写规则读路径无全局锁」这条性能不变量。
//
// 背景：压测发现详情页 ~400 RPS 而静态文件 15,000 RPS，strace 显示 63.7% 的
// 系统调用时间耗在 futex。根因是 ParsePattern 在函数入口就 mu.Lock()，而它被
// GetUrl 在模板生成每个链接时调用——几十次/请求全部争用同一把包级锁。
//
// 断言（AST，不依赖数据库）：
//   - ParsePattern 函数体内不得出现任何 .Lock()/.Unlock() 调用；
//   - ParsePattern 读路径必须经 loadParsedPattern（原子加载已发布快照）；
//   - GetRewritePattern 的快路径（return）必须排在 rewriteMu.Lock() 之前。
//
// 反向对照：在 ParsePattern 里加回 mu.Lock()，或把快路径挪到 Lock 之后，本测试立刻红。
func TestParsePatternReadPathUnlocked(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "rewrite.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 rewrite.go 失败: %v", err)
	}

	funcs := make(map[string]*ast.FuncDecl)
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Recv != nil {
			funcs[f.Name.Name] = f
		}
	}

	// ParsePattern：不许出现任何锁调用。
	pp := funcs["ParsePattern"]
	if pp == nil {
		t.Fatal("护栏空转：没找到 (*Website).ParsePattern")
	}
	ast.Inspect(pp.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Lock" || sel.Sel.Name == "Unlock" || sel.Sel.Name == "RLock" || sel.Sel.Name == "RUnlock" {
					t.Fatalf("ParsePattern 读路径不得持有任何锁（futex 串行化根因），发现 %s()", sel.Sel.Name)
				}
			}
		}
		return true
	})

	// ParsePattern：读路径必须经原子加载 helper。
	src := renderGuardExpr(fset, pp.Body)
	if !strings.Contains(src, "loadParsedPattern") {
		t.Fatal("ParsePattern 必须通过 loadParsedPattern 原子读取已发布快照")
	}

	// GetRewritePattern：快路径 return 必须排在加权锁之前。
	grp := funcs["GetRewritePattern"]
	if grp == nil {
		t.Fatal("护栏空转：没找到 (*Website).GetRewritePattern")
	}
	lockPos := token.NoPos
	firstReturnPos := token.NoPos
	ast.Inspect(grp.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Lock" && lockPos == token.NoPos {
				lockPos = call.Pos()
			}
		}
		if r, ok := n.(*ast.ReturnStmt); ok && firstReturnPos == token.NoPos {
			firstReturnPos = r.Pos()
		}
		return true
	})
	if lockPos == token.NoPos {
		t.Fatal("护栏空转：GetRewritePattern 里没找到写路径的加锁")
	}
	if firstReturnPos == token.NoPos || firstReturnPos > lockPos {
		t.Fatal("GetRewritePattern 的读快路径 return 必须排在加锁之前，否则缓存命中也要争锁")
	}
}

// TestParsePatternConcurrentReads 在 -race 下验证：并发读同一站点的重写规则不产生
// 数据竞争，且所有读拿到的都是同一份已编译快照（Parsed=true、Rules 已就绪）。
func TestParsePatternConcurrentReads(t *testing.T) {
	w := &Website{
		Id:            987654,
		PluginRewrite: &config.PluginRewriteConfig{Mode: config.RewriteNumberMode},
	}
	// 预热：模拟站点初始化时的 GetRewritePattern(true)。
	first := w.GetRewritePattern(true)
	if !first.Parsed {
		t.Fatal("预热后快照必须已编译（Parsed=true）")
	}
	if len(first.Rules) == 0 {
		t.Fatal("预热后快照必须含编译好的 Rules")
	}

	const n = 64
	var wg sync.WaitGroup
	patterns := make([]*RewritePattern, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			patterns[idx] = w.ParsePattern(false)
		}(i)
	}
	wg.Wait()

	for i, p := range patterns {
		if p == nil {
			t.Fatalf("第 %d 个并发读返回 nil", i)
		}
		if p != first {
			t.Fatalf("第 %d 个并发读拿到了不同快照，读路径可能重建/原地改写", i)
		}
	}
}

// TestGetRewritePatternFocusReplacesSnapshot 验证 focus=true（后台改规则）时是
// 「整体替换不可变快照」而不是原地改写：新快照指针不同，且不污染旧快照。
func TestGetRewritePatternFocusReplacesSnapshot(t *testing.T) {
	w := &Website{
		Id:            987655,
		PluginRewrite: &config.PluginRewriteConfig{Mode: config.RewriteNumberMode},
	}
	old := w.ParsePattern(false)
	oldArchive := old.Patterns[PatternArchive]

	// 切到自定义模式并强制重建。
	w.PluginRewrite = &config.PluginRewriteConfig{
		Mode:   config.RewritePatternMode,
		Patten: "archive===/custom/{id}.html",
	}
	next := w.GetRewritePattern(true)

	if next == old {
		t.Fatal("focus=true 必须整体替换为新快照，而非原地改写")
	}
	if got := next.Patterns[PatternArchive]; got == oldArchive || got == "" {
		t.Fatalf("新快照未生效：archive=%q（旧 %q）", got, oldArchive)
	}
	// 旧快照必须保持不变（不可变语义，读路径安全）。
	if got := old.Patterns[PatternArchive]; got != oldArchive {
		t.Fatalf("旧快照被原地改写：%q -> %q", oldArchive, got)
	}
	if !next.Parsed {
		t.Fatal("新快照必须已编译")
	}
}
