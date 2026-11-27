package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestCategoryPlaceCacheFastPathUnlocked 锁住「分类/地点缓存命中不拿全局锁」。
//
// 背景：2026-11-17 压测发现详情页吞吐恒定在 ~360 RPS，p50 随并发线性上涨，
// strace 显示 68% 系统调用时间在 futex。根因是 GetCacheCategories/GetCachePlaces
// 把整个函数包在包级 mu 里，而模板每个页面都渲染导航/面包屑，导致所有请求串行。
// Cache.Get 自身由 RWMutex 保护，命中路径根本不需要这把全局锁。
//
// 断言（AST）：两函数内第一次 mu.Lock() 必须晚于第一次 w.Cache.Get 调用 ——
// 即缓存命中快路径完全无锁。
//
// 反向对照：把 mu.Lock() 挪回函数开头，本测试立刻红。
func TestCategoryPlaceCacheFastPathUnlocked(t *testing.T) {
	cases := []struct {
		file string
		fn   string
	}{
		{"category.go", "GetCacheCategories"},
		{"place.go", "GetCachePlaces"},
	}

	for _, c := range cases {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, c.file, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", c.file, err)
		}
		var fn *ast.FuncDecl
		for _, d := range file.Decls {
			if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == c.fn {
				fn = f
				break
			}
		}
		if fn == nil {
			t.Fatalf("护栏空转：%s 里没找到 %s", c.file, c.fn)
		}

		lockPos := token.NoPos
		cacheGetPos := token.NoPos
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "Lock" && lockPos == token.NoPos {
				lockPos = call.Pos()
			}
			// w.Cache.Get(...)
			if sel.Sel.Name == "Get" {
				if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "Cache" {
					if cacheGetPos == token.NoPos {
						cacheGetPos = call.Pos()
					}
				}
			}
			return true
		})

		if cacheGetPos == token.NoPos {
			t.Fatalf("护栏空转：%s 里没找到 Cache.Get 调用", c.fn)
		}
		if lockPos == token.NoPos {
			// 完全没有锁也算通过：快路径无锁，慢路径可能有别的保护方式。
			continue
		}
		if lockPos < cacheGetPos {
			t.Fatalf("%s 的 mu.Lock() 排在 Cache.Get 之前：缓存命中也要争全局锁，"+
				"所有渲染导航的请求会被串行化", c.fn)
		}
	}
}
