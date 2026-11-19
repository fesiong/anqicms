package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider/fulltext"
	"kandaoni.com/anqicms/provider/segmenter"
)

// ============================================================
// sego 词典：进程级唯一（2026-11-16 106 站点事故的后续）
//
// 词典加载完即只读，常驻内存数百 MB。改造前有两处各自加载：
// provider.WordSplit 的包级 Segmenter，以及悟空引擎每个站点一份
// （NewWukongService 传 SegmenterDictionaries）。106 个站点同时启动
// 就是 106 份常驻 + 106 次加载，机器被吃满、端口迟迟不开。
// 现在统一走 provider/segmenter.Shared()，下面三条护栏守着它。
// ============================================================

// TestOnlyOneDictionaryLoaderInRepo 锁住「整个仓库只有一处 LoadDictionary」。
//
// 只要有人再写一处（不管是新引擎、还是把 wukong 改回
// SegmenterDictionaries），这条用例就红 —— 那意味着又会按站点复制词典。
func TestOnlyOneDictionaryLoaderInRepo(t *testing.T) {
	files := repoGoFiles(t)
	if len(files) < 100 {
		t.Fatalf("只扫到 %d 个 .go 文件，扫描范围明显不对（护栏会假绿）", len(files))
	}
	var hits []string
	for _, path := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok && strings.HasSuffix(callFullName(call), "LoadDictionary") {
				hits = append(hits, path+":"+strconv.Itoa(fset.Position(call.Pos()).Line))
			}
			return true
		})
	}

	want := "../provider/segmenter/segmenter.go:"
	if len(hits) != 1 {
		t.Fatalf("词典加载点必须唯一，实际 %d 处：%v", len(hits), hits)
	}
	if !strings.HasPrefix(hits[0], want) {
		t.Fatalf("词典只能在 %s 里加载，实际：%s", want, hits[0])
	}
}

// TestWukongUsesSharedSegmenter 锁住「悟空引擎拿外部分词器」。
//
// 两个要点：
//  1. 必须给 Segmenter（共享实例），不能再让引擎自己看 SegmenterDictionaries；
//  2. 有了 Segmenter，engine.Close() 就不会去关它（引擎只关自己加载的那个），
//     站点重载全文搜索不会把全站共用的词典关掉。
func TestWukongUsesSharedSegmenter(t *testing.T) {
	_, file := parseGoFileForGuard(t, filepath.Join("fulltext", "wukong.go"))
	body := findFuncBodyForGuard(t, file, "", "NewWukongService")

	var haveSegmenter, haveDictionaries bool
	ast.Inspect(body, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		// 只看 types.EngineInitOptions{...} 这种带字段名的字面量
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			switch key.Name {
			case "Segmenter":
				haveSegmenter = true
			case "SegmenterDictionaries":
				haveDictionaries = true
			}
		}
		return true
	})
	if !haveSegmenter {
		t.Fatal("NewWukongService 没有把共享分词器传给引擎（EngineInitOptions.Segmenter）")
	}
	if haveDictionaries {
		t.Fatal("NewWukongService 又设了 SegmenterDictionaries —— 每个站点会各加载一份词典")
	}
}

// TestNoOneClosesTheSharedDictionary 锁住「运行期没人 Close 词典」。
//
// sego 的 Close() 会把 dict.trie 置 nil，之后任何一次分词都是 nil 解引用。
// 词典是进程级共享的，一个站点"清缓存"（历史上有 DictClose）就会让
// 所有站点的下一次分词 panic —— 所以这个符号必须彻底消失。
func TestNoOneClosesTheSharedDictionary(t *testing.T) {
	files := repoGoFiles(t)
	var hits []string
	for _, path := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if ok && ident.Name == "DictClose" {
				hits = append(hits, path+":"+strconv.Itoa(fset.Position(ident.Pos()).Line))
			}
			return true
		})
	}
	if len(hits) > 0 {
		t.Fatalf("仓库里又出现了 DictClose（%v）：共享词典不允许在运行期被关闭", hits)
	}
}

// TestWordSplitIsSafeForConcurrentUse 跑一遍多站点并发分词。
//
// 词典不可用时 WordSplit 必须整串返回（而不是 nil 解引用或空切片），
// 词典可用时不能因为并发而互相踩：sego 的 Segment 只读词典。
func TestWordSplitIsSafeForConcurrentUse(t *testing.T) {
	const goroutines, rounds = 16, 20
	var wg sync.WaitGroup
	errs := make(chan string, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				words := WordSplit("安企内容管理系统 v3.8.4", false)
				if len(words) == 0 {
					errs <- "WordSplit 返回空切片"
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Fatal(msg)
	}
}

// TestMultipleWukongEnginesShareOneDictionary 走真实代码路径：
// 三个站点各建一个悟空引擎，词典只能加载一次。
//
// ⚠️ 这里**故意不调** svc.Close()：wukong 的 Close → FlushIndex 会非原子地读
// Init 里刚起的 worker 协程正在原子写的计数器（上游 engine.go:439 vs :170），
// -race 下必然报数据竞争。那是 wukong 自身的问题，与词典无关。
// "运行期不许关共享词典" 由 TestNoOneClosesTheSharedDictionary 守着；
// "用了外部分词器，engine.Close 就不会关它" 由 TestWukongUsesSharedSegmenter
// （要求 Segmenter 字段存在）+ vendor engine.go 的 usingExternalSegmenter 分支保证。
func TestMultipleWukongEnginesShareOneDictionary(t *testing.T) {
	before := segmenter.LoadCount()
	cfg := &config.PluginFulltextConfig{}
	for i := 0; i < 3; i++ {
		if _, err := fulltext.NewWukongService(cfg, "site_"+strconv.Itoa(i)); err != nil {
			t.Fatalf("第 %d 个站点创建悟空引擎失败：%v", i, err)
		}
	}
	if delta := segmenter.LoadCount() - before; delta > 1 {
		t.Fatalf("三个引擎让词典加载了 %d 次，应最多 1 次", delta)
	}
	if !segmenter.Ready() {
		t.Fatal("建完三个引擎后词典仍应可用")
	}
	// 反向对照：确实是同一份在用（分词器非 nil 且能切词），不是"都没加载"。
	if seg := segmenter.Shared(); seg == nil || seg.Dictionary() == nil {
		t.Fatal("共享分词器或其词典为 nil，三个引擎拿到的是空壳")
	}
	if words := WordSplit("安企内容管理系统", false); len(words) == 0 {
		t.Fatal("共享分词器切不出词")
	}
}

// repoGoFiles 返回仓库内所有（非 vendor / 非 node_modules / 非隐藏目录的）.go 文件。
// provider 包测试的 cwd 是 provider/，所以仓库根是 ".."。
func repoGoFiles(t *testing.T) []string {
	t.Helper()
	root := ".."
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path == root {
				return nil
			}
			if name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历仓库失败：%v", err)
	}
	return files
}
