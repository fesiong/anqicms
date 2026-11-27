package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestMobileCacheUsesFirstPageLink 护栏：移动端第一页必须用「第一页链接」去请求。
//
// 背景（2026-11-19 实测）：Build*Cache 生成列表页时，PC 的分页循环把局部变量 link
// 反复改写成第 2..N 页，循环结束时 link 指向**最后一页**；移动端第一页直接复用了
// link，于是请求的是 /news_3 这类末页地址。后果是 cache/mobile/ 下压根没有第一页
// （/news/index.html），手机端首页只能回落动态渲染，而末页被重复写两次。
// 在分页 bug 修好之前这条路径根本没跑到（TotalPages=0），所以问题一直没暴露。
//
// 规则（对 htmlCache.go 逐函数穷举，不写死函数清单）：分页循环（page := 2 起）之外
// 的 GetAndCacheHtmlData(x, true) 调用，x 不允许是被该分页循环改写过的变量 —— 那一定
// 是末页。分页循环内部的调用不受此限，它本来就是当前页。
func TestMobileCacheUsesFirstPageLink(t *testing.T) {
	const srcFile = "htmlCache.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, srcFile, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败: %v", srcFile, err)
	}

	type posRange struct{ start, end token.Pos }
	inside := func(spans []posRange, p token.Pos) bool {
		for _, s := range spans {
			if p >= s.start && p <= s.end {
				return true
			}
		}
		return false
	}

	checkedFuncs := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		// 1) 只收集「分页循环」的循环体区间：for page := 2; ...; page++
		var pageLoopSpans []posRange
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			s, okF := n.(*ast.ForStmt)
			if !okF {
				return true
			}
			init, okI := s.Init.(*ast.AssignStmt)
			if !okI || len(init.Lhs) != 1 || len(init.Rhs) != 1 {
				return true
			}
			lhs, okL := init.Lhs[0].(*ast.Ident)
			rhs, okR := init.Rhs[0].(*ast.BasicLit)
			if okL && okR && lhs.Name == "page" && rhs.Value == "2" {
				pageLoopSpans = append(pageLoopSpans, posRange{s.Body.Pos(), s.Body.End()})
			}
			return true
		})
		if len(pageLoopSpans) == 0 {
			continue // 没有分页，就不存在「第一页被改写」的问题
		}

		// 2) 收集被分页循环改写过的变量名（= 与 :=）
		pageLoopWritten := map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			as, okA := n.(*ast.AssignStmt)
			if !okA || !inside(pageLoopSpans, as.Pos()) {
				return true
			}
			for _, lhs := range as.Lhs {
				if id, okI := lhs.(*ast.Ident); okI && id.Name != "_" {
					pageLoopWritten[id.Name] = true
				}
			}
			return true
		})

		// 3) 检查分页循环之外的移动端缓存调用
		found := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, okC := n.(*ast.CallExpr)
			if !okC || len(call.Args) != 2 || inside(pageLoopSpans, call.Pos()) {
				return true
			}
			sel, okS := call.Fun.(*ast.SelectorExpr)
			if !okS || sel.Sel.Name != "GetAndCacheHtmlData" {
				return true
			}
			// 第二个参数是 isMobile；go 里 true 是预声明标识符而不是字面量
			mobileArg, okM := call.Args[1].(*ast.Ident)
			if !okM || mobileArg.Name != "true" {
				return true
			}
			arg, okI := call.Args[0].(*ast.Ident)
			if !okI {
				return true // 字面量 url 不可能是被改写的循环变量
			}
			found = true
			if pageLoopWritten[arg.Name] {
				t.Errorf("%s: %s 在分页循环外把移动端第一页交给 %s，但 %s 已被分页循环改写成最后一页；"+
					"请另存 firstLink 并用它请求移动端第一页",
					fset.Position(call.Pos()), fn.Name.Name, arg.Name, arg.Name)
			}
			return true
		})
		if found {
			checkedFuncs++
		}
	}

	if checkedFuncs == 0 {
		t.Fatal("没有检查到任何「分页循环 + 移动端第一页缓存」的函数，护栏已失效（方法名或循环写法变了？）")
	}
}
