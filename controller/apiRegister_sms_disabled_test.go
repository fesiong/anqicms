package controller

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

// R90 补充（后台未开启短信）：ApiRegister 收到 phone 时仍按 §6.2 归一化并置
// PhoneNormalized，但只有短信渠道开启（PluginSms.Provider != ""）才校验
// register 场景码；未开启时注册必须能继续往下走，不得 return、不得靠 req.Code
// 是否为空分叉。本文件用 go/ast 锁住这个结构（handler 依赖 iris/DB，跑真 handler
// 需要起库起站点，CI 里不可行），合成坏源码做反向对照保证断言不是空转。

func s11ExprText(e ast.Expr) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, token.NewFileSet(), e)
	return b.String()
}

// s11RegisterFunc 从 readFuncSource 截取的函数源码里取回 ApiRegister 的 AST。
func s11RegisterFunc(t *testing.T, src string) *ast.FuncDecl {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "apiUser.go", "package controller\n"+src, 0)
	if err != nil {
		t.Fatalf("解析 ApiRegister 源码失败：%v", err)
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "ApiRegister" {
			return fd
		}
	}
	t.Fatal("源码里找不到 ApiRegister 函数")
	return nil
}

// s11RegisterIfs 返回函数体里所有 if 语句（含嵌套），AST 遍历顺序即源码顺序。
func s11RegisterIfs(fn *ast.FuncDecl) []*ast.IfStmt {
	var out []*ast.IfStmt
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if s, ok := n.(*ast.IfStmt); ok {
			out = append(out, s)
		}
		return true
	})
	return out
}

type smsGateError string

func (e smsGateError) Error() string { return string(e) }

func errSmsGate(msg string) error { return smsGateError(msg) }

// s11CheckRegisterSkipsSmsCodeWhenDisabled 校验短信开关门禁的结构：
//  1. 存在 `if req.Phone != ""` 分支，归一化三句都在其中；
//  2. 存在以 PluginSms/Provider 为条件的 if，嵌套在 phone 分支内部、排在
//     归一化三句之后 —— 保证短信未开启时号码仍被归一化、PhoneNormalized 仍置位；
//  3. VerifySmsCode 在该门禁内部，RegisterUser 在门禁之后；
//  4. 门禁体内没有直接 return，也没有 else 分支 return —— 未开启短信不得短路注册；
//  5. 门禁条件为正向的 Provider != ""，不是 Provider == "" 的早退式写法。
func s11CheckRegisterSkipsSmsCodeWhenDisabled(t *testing.T, src string) error {
	t.Helper()
	fn := s11RegisterFunc(t, src)

	var phoneIf, gate *ast.IfStmt
	for _, s := range s11RegisterIfs(fn) {
		cond := s11ExprText(s.Cond)
		if strings.Contains(cond, `req.Phone != ""`) {
			phoneIf = s
		}
		if strings.Contains(cond, "PluginSms") && strings.Contains(cond, "Provider") {
			if gate != nil {
				return errSmsGate("出现多个以 PluginSms/Provider 为条件的 if，门禁不唯一")
			}
			gate = s
		}
	}
	if phoneIf == nil {
		return errSmsGate(`缺少 if req.Phone != "" 门禁`)
	}
	if gate == nil {
		return errSmsGate("缺少按 PluginSms.Provider 判定短信是否开启的门禁（未开启时必须跳过验码）")
	}
	if !(phoneIf.Pos() < gate.Pos() && gate.End() < phoneIf.End()) {
		return errSmsGate(`短信开关门禁必须嵌套在 if req.Phone != "" 内部`)
	}

	// 归一化三句必须都在 phone 分支里、且在门禁之前（不能被门禁一起跳过）。
	wanted := []string{
		"req.PhoneCountry = p.Country",
		"req.Phone = p.Local",
		"req.PhoneNormalized = true",
	}
	for _, w := range wanted {
		found := false
		ast.Inspect(phoneIf.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			if s11ExprText(as.Lhs[0])+" = "+s11ExprText(as.Rhs[0]) != w {
				return true
			}
			found = true
			if as.Pos() > gate.Pos() {
				return true // 位置判定在下面统一报错
			}
			return true
		})
		if !found {
			return errSmsGate("缺少归一化赋值：" + w)
		}
	}
	// 再次按位置确认三句都在门禁之前。
	for _, w := range wanted {
		latest := token.Pos(0)
		ast.Inspect(phoneIf.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			if s11ExprText(as.Lhs[0])+" = "+s11ExprText(as.Rhs[0]) == w && as.Pos() > latest {
				latest = as.Pos()
			}
			return true
		})
		if latest == 0 || latest > gate.Pos() {
			return errSmsGate("归一化 " + w + " 被短信门禁一起跳过（未开启时号码必须仍被归一化）")
		}
	}

	// VerifySmsCode 只能在门禁体内。
	verifyInGate := false
	ast.Inspect(gate.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "VerifySmsCode" {
			verifyInGate = true
		}
		return true
	})
	if !verifyInGate {
		return errSmsGate("VerifySmsCode 不在短信开关门禁内（未开启短信时会误验码）")
	}

	// 门禁不得短路注册：体内无直接 return，else 分支也不得 return。
	for _, st := range gate.Body.List {
		if _, ok := st.(*ast.ReturnStmt); ok {
			return errSmsGate("短信未开启分支直接 return，注册无法继续")
		}
	}
	if gate.Else != nil {
		hasReturn := false
		ast.Inspect(gate.Else, func(n ast.Node) bool {
			if _, ok := n.(*ast.ReturnStmt); ok {
				hasReturn = true
			}
			return true
		})
		if hasReturn {
			return errSmsGate("短信未开启的 else 分支直接 return，注册无法继续")
		}
	}

	// RegisterUser 必须在门禁之后。
	registerAfter := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "RegisterUser" && call.Pos() > gate.End() {
			registerAfter = true
		}
		return true
	})
	if !registerAfter {
		return errSmsGate("RegisterUser 不在短信门禁之后")
	}

	// 正向开启条件：Provider != ""，不能是 Provider == "" 的早退式写法。
	if !strings.Contains(s11ExprText(gate.Cond), `Provider != ""`) {
		return errSmsGate(`短信门禁必须使用正向条件 Provider != ""（反向早退写法容易把注册短路）`)
	}
	return nil
}

func TestApiRegisterSkipsSmsCodeWhenSmsDisabled(t *testing.T) {
	src := readFuncSource(t, "apiUser.go", "ApiRegister")
	if err := s11CheckRegisterSkipsSmsCodeWhenDisabled(t, src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}

	// 反向对照①：把整个 phone 块塞进短信门禁 —— 未开启时连归一化都不做。
	badWholeBlock := `func ApiRegister(ctx iris.Context) {
		if currentSite.PluginSms != nil && currentSite.PluginSms.Provider != "" {
			if req.Phone != "" {
				p, _ := sms.NormalizePhone(req.PhoneCountry, req.Phone)
				req.PhoneCountry = p.Country
				req.Phone = p.Local
				req.PhoneNormalized = true
				if e := currentSite.VerifySmsCode(p.Country, p.Local, sms.SceneRegister, req.Code); e != nil {
					respondSmsError(ctx, currentSite, e)
					return
				}
			}
		}
		user, err := currentSite.RegisterUser(&req)
		_, _ = user, err
	}`
	if err := s11CheckRegisterSkipsSmsCodeWhenDisabled(t, badWholeBlock); err == nil {
		t.Fatal("反向对照①失败：整块 phone 处理被门禁包住的源码仍判绿")
	}

	// 反向对照②：短信未开启时直接 return —— 注册走不下去。
	badEarlyReturn := `func ApiRegister(ctx iris.Context) {
		if req.Phone != "" {
			p, _ := sms.NormalizePhone(req.PhoneCountry, req.Phone)
			req.PhoneCountry = p.Country
			req.Phone = p.Local
			req.PhoneNormalized = true
			if currentSite.PluginSms == nil || currentSite.PluginSms.Provider == "" {
				return
			}
			if e := currentSite.VerifySmsCode(p.Country, p.Local, sms.SceneRegister, req.Code); e != nil {
				respondSmsError(ctx, currentSite, e)
				return
			}
		}
		user, err := currentSite.RegisterUser(&req)
		_, _ = user, err
	}`
	if err := s11CheckRegisterSkipsSmsCodeWhenDisabled(t, badEarlyReturn); err == nil {
		t.Fatal("反向对照②失败：短信未开启直接 return 的源码仍判绿")
	}

	// 反向对照③：按 req.Code 是否为空旁路（R90 原有禁令，短信开启时也必须验码）。
	badCodeGate := `func ApiRegister(ctx iris.Context) {
		if req.Phone != "" {
			p, _ := sms.NormalizePhone(req.PhoneCountry, req.Phone)
			req.PhoneCountry = p.Country
			req.Phone = p.Local
			req.PhoneNormalized = true
			if currentSite.PluginSms != nil && currentSite.PluginSms.Provider != "" && req.Code != "" {
				if e := currentSite.VerifySmsCode(p.Country, p.Local, sms.SceneRegister, req.Code); e != nil {
					respondSmsError(ctx, currentSite, e)
					return
				}
			}
		}
		user, err := currentSite.RegisterUser(&req)
		_, _ = user, err
	}`
	if err := s11CheckRegisterForcesSmsCode(badCodeGate); err == nil {
		t.Fatal("反向对照③失败：按 req.Code 空值旁路的源码仍判绿")
	}

	if err := s11CheckRegisterSkipsSmsCodeWhenDisabled(t, src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}
