package manageController

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// 本文件的职责：用 go/ast 静态锁住「handler 必须给出响应」这一族不变式。
//
// 起因是两个已上线的静默失效（2026-10-27）：
//   - MakeCouponCode 成功路径没有任何 ctx.JSON，前端拿到空 body，MCP 侧
//     endpointFailure("") 直接返回空串 → 失败被当成成功，成功也没有任何回执；
//   - SettingCacheForm 的 `if req.Update` 分支同样不写响应，切换缓存类型时
//     只有走 else（手动清缓存）才有回执；
//   - CommunicationOrderSaveImages 的 ReadJSON 失败分支漏了 return，弹出
//     Invalid Params 之后仍继续用零值 req 调 SaveCommunicationOrderImages。
//
// 这三处都是「跑起来不报错、只是行为不对」的类型，常规单元测试抓不到
// （要起库起站点），所以用 AST 规则兜住整个包，而不是逐个函数写断言。

// noJSONResponseHandlers 是本来就不写 JSON 包络的 handler。
// 每一项都必须写明它用什么方式响应 —— 名单本身就是文档，新增条目要能过审。
var noJSONResponseHandlers = map[string]string{
	"AiChat":                      "SSE 流式响应：ctx.ContentType + ctx.ResponseWriter 持续写流",
	"AiAgentChat":                 "SSE 流式响应：同上",
	"GetQuickImportExcelTemplate": "直接回写 xlsx 文件流（ctx.ResponseWriter）",
	"AdminFileServ":               "静态文件服务：ctx.ServeFile 回写文件，失败时才用 JSON 报错",
	"DownloadDesignInfo":          "直接回写模板压缩包（ctx.ResponseWriter）",
}

// ctxResponseMethods 是「写出响应」的判定集合。只认 ctx.<Method>() 形式，
// 这样 `currentSite.GetXxx()` 之类不会被误判。
var ctxResponseMethods = map[string]bool{
	"JSON": true, "XML": true, "Text": true, "HTML": true,
	"WriteString": true, "Writef": true, "Write": true,
	"Redirect": true, "ServeFile": true, "SendFile": true,
	"StreamWriter": true, "NotFound": true, "StatusCode": true,
}

// TestHandlersMustWriteResponse 要求 handler 正常执行到函数末尾时一定写响应。
//
// 判定方式：从最后一条顶层语句往前看出口 —— 语句是 ctx 的响应调用或 return 即通过；
// 是 if/switch 则要求每个分支都同样通过（缺 else 意味着「不进 if」的路径会落空，判失败）。
// 这条规则能抓住「只在错误分支里写过 ctx.JSON」的函数，那正是 MakeCouponCode 的形态。
func TestHandlersMustWriteResponse(t *testing.T) {
	handlers := parsePackageHandlers(t)
	if len(handlers) < 300 {
		t.Fatalf("只扫描到 %d 个 handler，明显不是整包扫描 —— 断言已失去意义", len(handlers))
	}

	scanned := 0
	for _, h := range handlers {
		if _, ok := noJSONResponseHandlers[h.name]; ok {
			continue
		}
		scanned++
		if !endsWithResponse(h.decl.Body.List) {
			t.Errorf("%s（%s:%d）正常执行到末尾时不写任何响应：\n"+
				"  成功路径必须 ctx.JSON 回执，否则前端拿到空 body、MCP 侧 endpointFailure(\"\") "+
				"判不出失败，失败会被当成成功。\n"+
				"  若该 handler 本来就不写 JSON（流式/文件下载），请在 noJSONResponseHandlers 里登记并写明理由。",
				h.name, h.file, h.line)
		}
	}

	// 反向对照：证明豁免名单没有把整个包吃光。
	if scanned < len(handlers)*3/4 {
		t.Fatalf("豁免了 %d/%d 个 handler，规则形同空转", len(handlers)-scanned, len(handlers))
	}
	// 名单会随代码漂移：函数改名/删掉后必须同步清理，否则名单变成垃圾。
	for name := range noJSONResponseHandlers {
		if _, ok := handlers[name]; !ok {
			t.Errorf("noJSONResponseHandlers 里的 %s 已不存在，请删除该条目", name)
		}
	}
}

// TestReadJSONErrorBranchMustTerminate 要求 ctx.ReadJSON 的 err != nil 分支以
// return/continue/break 结束。
//
// 漏 return 时，函数会带着未解析成功（零值）的 req 继续往下跑：轻则写进零值，
// 重则用 id=0 命中别的记录。这类错误不会 panic，只会静默写错数据。
func TestReadJSONErrorBranchMustTerminate(t *testing.T) {
	files := packageGoFiles(t)
	found := 0
	for _, file := range files {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", file, err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			ifs, ok := n.(*ast.IfStmt)
			if !ok || ifs.Init == nil {
				return true
			}
			if !isReadJSONErrCheck(ifs) {
				return true
			}
			found++
			if len(ifs.Body.List) == 0 {
				return true
			}
			switch ifs.Body.List[len(ifs.Body.List)-1].(type) {
			case *ast.ReturnStmt, *ast.BranchStmt:
			default:
				pos := fset.Position(ifs.Pos())
				t.Errorf("%s:%d ctx.ReadJSON 失败分支没有 return：报完错还会继续执行，"+
					"用的是零值 req", filepath.Base(file), pos.Line)
			}
			return true
		})
	}
	if found == 0 {
		t.Fatal("没扫到任何 ReadJSON 错误分支 —— 断言已失去意义")
	}
}

// isReadJSONErrCheck 判断 if 语句是否是 `if err := ctx.ReadJSON(&x); err != nil {` 形态。
// 只认 `!= nil`，不认 `err2 == nil` 这类「可选解析」写法。
func isReadJSONErrCheck(ifs *ast.IfStmt) bool {
	as, ok := ifs.Init.(*ast.AssignStmt)
	if !ok || len(as.Rhs) != 1 {
		return false
	}
	call, ok := as.Rhs[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ReadJSON" {
		return false
	}
	bin, ok := ifs.Cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	id, ok := bin.Y.(*ast.Ident)
	return ok && id.Name == "nil"
}

// endsWithResponse 判断「执行到这段语句末尾」时是否一定写出了响应。
func endsWithResponse(stmts []ast.Stmt) bool {
	if len(stmts) == 0 {
		return false
	}
	last := stmts[len(stmts)-1]

	switch s := last.(type) {
	case *ast.ReturnStmt:
		return true // 交给别处收尾，交由调用方负责
	case *ast.ExprStmt:
		return isCtxResponseCall(s)
	case *ast.IfStmt:
		if !endsWithResponse(s.Body.List) {
			return false
		}
		// 没有 else 就表示「条件不成立」时直接落到函数末尾，等于没写响应。
		if s.Else == nil {
			return false
		}
		switch e := s.Else.(type) {
		case *ast.BlockStmt:
			return endsWithResponse(e.List)
		case *ast.IfStmt:
			return endsWithResponse([]ast.Stmt{e})
		}
	case *ast.SwitchStmt:
		if s.Body == nil || len(s.Body.List) == 0 {
			return false
		}
		for _, c := range s.Body.List {
			cc, ok := c.(*ast.CaseClause)
			if !ok || !endsWithResponse(cc.Body) {
				return false
			}
		}
		return true
	}
	return false
}

func isCtxResponseCall(s *ast.ExprStmt) bool {
	call, ok := s.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "ctx" && ctxResponseMethods[sel.Sel.Name]
}

type handlerFunc struct {
	name string
	file string
	line int
	decl *ast.FuncDecl
}

// parsePackageHandlers 扫描本包全部非测试源文件，取出形如
// `func X(ctx iris.Context)` 的 handler（带 receiver 的方法不算）。
func parsePackageHandlers(t *testing.T) map[string]handlerFunc {
	t.Helper()
	out := map[string]handlerFunc{}
	for _, file := range packageGoFiles(t) {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", file, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil || !hasCtxParam(fn) {
				continue
			}
			out[fn.Name.Name] = handlerFunc{
				name: fn.Name.Name,
				file: filepath.Base(file),
				line: fset.Position(fn.Pos()).Line,
				decl: fn,
			}
		}
	}
	return out
}

func hasCtxParam(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}
	for _, p := range fn.Type.Params.List {
		for _, n := range p.Names {
			if n.Name == "ctx" {
				return true
			}
		}
	}
	return false
}

// packageGoFiles 返回本包的非测试源文件（按文件名排序，保证报错顺序稳定）。
func packageGoFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("列出本包源文件失败：%v", err)
	}
	out := files[:0]
	for _, f := range files {
		if !strings.HasSuffix(f, "_test.go") {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}
