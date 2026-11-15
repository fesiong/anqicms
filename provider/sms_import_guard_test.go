package provider

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// 裁决 R25（2026-11-15 整分支审查补做）：模板包**导入**路径必须跳过短信配置键。
//
// 导出侧（R20）早就把 SmsSettingKey 排除在 zip 之外，但导入侧
// （design.go 的 restoreSingleData → settings 分支）没堵：它对 settings 行做
// OnConflict{UpdateAll:true} 写入，第三方模板包只要携带一条 sms 行，就会把本站
// 的 AK/SK/签名/场景模板整体覆盖 —— 把站长的 OTP 引到攻击者自己的 Twilio 账号。
//
// 本护栏三段式：真实源码绿 / 合成「删掉 SmsSettingKey」的源码红 / 恢复绿。

// TestSmsSettingExcludedFromDesignImport 钉住导入侧的跳过清单。
func TestSmsSettingExcludedFromDesignImport(t *testing.T) {
	src, err := os.ReadFile("design.go")
	if err != nil {
		t.Fatalf("读取 design.go 失败：%v", err)
	}
	keys, err := designImportSkippedSettingKeys(src)
	if err != nil {
		t.Fatalf("提取导入跳过清单失败（护栏会空转）：%v", err)
	}
	if !containsIdent(keys, "SmsSettingKey") {
		t.Errorf("SmsSettingKey 不在模板导入的跳过清单里，导入第三方模板包会覆盖本站短信凭据。当前清单：%v", keys)
	}
	// 非空转对照：既有三个跳过键必须仍被同一个提取器看到。
	for _, want := range []string{"StorageSettingKey", "ImportApiSettingKey", "AnqiSettingKey"} {
		if !containsIdent(keys, want) {
			t.Errorf("提取器没看到既有跳过键 %s，说明它抓的不是那份条件。当前清单：%v", want, keys)
		}
	}
}

// TestSmsImportGuardIsNotVacuous 反向对照：删掉 SmsSettingKey 后提取器必须看不到它；
// 把整个条件删掉后提取器必须报错而不是返回空清单。
func TestSmsImportGuardIsNotVacuous(t *testing.T) {
	src, err := os.ReadFile("design.go")
	if err != nil {
		t.Fatalf("读取 design.go 失败：%v", err)
	}
	original := string(src)

	t.Run("删掉SmsSettingKey后必须检出缺失", func(t *testing.T) {
		broken := strings.Replace(original, "v.Key == SmsSettingKey", "v.Key == \"SmsSettingKey\"", 1)
		if broken == original {
			t.Fatal("反向对照准备失败：源码里没找到 v.Key == SmsSettingKey")
		}
		keys, err := designImportSkippedSettingKeys([]byte(broken))
		if err != nil {
			t.Fatalf("删掉一个键不该让提取器报错：%v", err)
		}
		if containsIdent(keys, "SmsSettingKey") {
			t.Fatal("反向对照不成立：删掉 SmsSettingKey 后提取器仍说它在清单里")
		}
		if !containsIdent(keys, "StorageSettingKey") {
			t.Fatal("反向对照把整份条件都弄丢了，说明删错了位置")
		}
	})

	t.Run("整条跳过条件消失时必须报错", func(t *testing.T) {
		broken := removeLineContaining(original, "v.Key == SmsSettingKey")
		broken = removeLineContaining(broken, "v.Key == StorageSettingKey")
		broken = removeLineContaining(broken, "v.Key == ImportApiSettingKey")
		broken = removeLineContaining(broken, "v.Key == AnqiSettingKey")
		if broken == original {
			t.Fatal("反向对照准备失败：没找到跳过条件所在行")
		}
		if _, err := designImportSkippedSettingKeys([]byte(broken)); err == nil {
			t.Fatal("反向对照不成立：跳过条件没了，提取器却返回成功（护栏会空转）")
		}
	})
}

// designImportSkippedSettingKeys 从 restoreSingleData 的源码里提取
// 「settings 分支按 v.Key == X 跳过」的键名清单。
// 找不到函数 / 找不到这样的条件 / 条件里没有 continue → 一律返回 error，护栏宁可红不可空转。
func designImportSkippedSettingKeys(src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "design.go", src, 0)
	if err != nil {
		return nil, err
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "restoreSingleData" {
			fn = fd
			break
		}
	}
	if fn == nil || fn.Body == nil {
		return nil, errors.New("未找到 restoreSingleData 函数体")
	}

	var keys []string
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ifs, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		cond, ok := ifs.Cond.(*ast.BinaryExpr)
		if !ok || cond.Op != token.LOR {
			return true
		}
		idents := settingKeyConditionIdents(cond)
		if len(idents) == 0 {
			return true
		}
		hasContinue := false
		ast.Inspect(ifs.Body, func(b ast.Node) bool {
			if br, ok := b.(*ast.BranchStmt); ok && br.Tok == token.CONTINUE {
				hasContinue = true
			}
			return true
		})
		if !hasContinue {
			return true
		}
		found = true
		keys = append(keys, idents...)
		return true
	})
	if !found {
		return nil, errors.New("restoreSingleData 里没有「按设置键跳过」的 continue 分支")
	}
	if len(keys) == 0 {
		return nil, errors.New("跳过清单为空")
	}
	return keys, nil
}

// settingKeyConditionIdents 从 `v.Key == X || v.Key == Y` 形式的条件里取出右侧
// 标识符（沿 || 链递归）。只认 `v.Key == Ident` 这种形状；右侧是字符串字面量等
// 写法时不收（那不是我们要找的清单）。
func settingKeyConditionIdents(cond ast.Expr) []string {
	var out []string
	collectSettingKeyIdents(cond, &out)
	return out
}

func collectSettingKeyIdents(expr ast.Expr, out *[]string) {
	bin, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return
	}
	if bin.Op == token.LOR {
		collectSettingKeyIdents(bin.X, out)
		collectSettingKeyIdents(bin.Y, out)
		return
	}
	if bin.Op != token.EQL {
		return
	}
	sel, ok := bin.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Key" {
		return
	}
	if ident, ok := bin.Y.(*ast.Ident); ok {
		*out = append(*out, ident.Name)
	}
}
