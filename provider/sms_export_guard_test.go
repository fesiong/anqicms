package provider

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// 反向对照已实跑（2026-11-14）：把 design.go 的 NOT IN 排除清单里的 SmsSettingKey 删掉后，
// TestSmsSettingExcludedFromDesignExport 变红（"SmsSettingKey 不在排除清单里"），恢复后变绿。
// 另有 TestSmsExportGuardIsNotVacuous 在测试内永久保留同样的对照，不依赖手工改文件。

// TestSmsSettingExcludedFromDesignExport 钉住裁决 R20：模板/设计 ZIP 导出
// （BackupDesignData → writeDataToZip("settings", ...)）必须把 sms 设置键排除在外，
// 否则「导出模板」会把短信 AK/SK 明文打进 zip 分发给任意下载者。
// 清单用 go/ast 穷举提取，不写死清单长度。
func TestSmsSettingExcludedFromDesignExport(t *testing.T) {
	src, err := os.ReadFile("design.go")
	if err != nil {
		t.Fatalf("读取 design.go 失败：%v", err)
	}
	keys, err := designExportExcludedSettingKeys(src)
	if err != nil {
		t.Fatalf("提取导出排除清单失败（护栏会空转）：%v", err)
	}

	if !containsIdent(keys, "SmsSettingKey") {
		t.Errorf("SmsSettingKey 不在模板导出的 NOT IN 排除清单里，短信凭据会随 zip 泄露。当前清单：%v", keys)
	}
	// 非空转对照：既有凭据键必须仍被同一个提取器看到。
	// 若提取器失效（找错函数／找错查询），这几条会一起红，而不是静默通过。
	for _, want := range []string{"SendmailSettingKey", "StorageSettingKey", "PaySettingKey", "WechatSettingKey"} {
		if !containsIdent(keys, want) {
			t.Errorf("提取器没看到既有排除键 %s，说明它抓的不是那份清单。当前清单：%v", want, keys)
		}
	}
}

// TestSmsExportGuardIsNotVacuous 反向对照：
// ① 从真实源码里删掉 SmsSettingKey → 提取器必须报它不在清单里；
// ② 把整条 NOT IN 查询删掉 → 提取器必须报错，而不是返回空清单让上面那条断言空转。
func TestSmsExportGuardIsNotVacuous(t *testing.T) {
	src, err := os.ReadFile("design.go")
	if err != nil {
		t.Fatalf("读取 design.go 失败：%v", err)
	}
	original := string(src)

	t.Run("删掉SmsSettingKey后必须检出缺失", func(t *testing.T) {
		broken := strings.Replace(original, "SmsSettingKey, ", "", 1)
		if broken == original {
			t.Fatal("反向对照准备失败：源码里没找到可删的 SmsSettingKey（清单写法变了？）")
		}
		keys, err := designExportExcludedSettingKeys([]byte(broken))
		if err != nil {
			t.Fatalf("删掉一个键不该让提取器报错：%v", err)
		}
		if containsIdent(keys, "SmsSettingKey") {
			t.Fatal("反向对照不成立：删掉 SmsSettingKey 后提取器仍说它在清单里")
		}
		if !containsIdent(keys, "SendmailSettingKey") {
			t.Fatal("反向对照把整份清单都弄丢了，说明删错了位置")
		}
	})

	t.Run("整条排除查询消失时必须报错", func(t *testing.T) {
		broken := removeLineContaining(original, "`key` NOT IN(?)")
		if broken == original {
			t.Fatal("反向对照准备失败：没找到 NOT IN 查询行")
		}
		if _, err := designExportExcludedSettingKeys([]byte(broken)); err == nil {
			t.Fatal("反向对照不成立：排除查询没了，提取器却返回成功（护栏会空转）")
		}
	})

	t.Run("导出函数改名时必须报错", func(t *testing.T) {
		broken := strings.Replace(original, "func (w *Website) BackupDesignData(", "func (w *Website) BackupDesignDataRenamed(", 1)
		if broken == original {
			t.Fatal("反向对照准备失败：没找到 BackupDesignData 声明")
		}
		if _, err := designExportExcludedSettingKeys([]byte(broken)); err == nil {
			t.Fatal("反向对照不成立：导出函数没了，提取器却返回成功")
		}
	})
}

// designExportExcludedSettingKeys 从源码里提取 BackupDesignData 中
// `key` NOT IN(?) 排除清单的常量名，并确认该函数确实把 settings 写进了 zip。
// 任何一处对不上都返回 error——护栏宁可红，不可空转。
func designExportExcludedSettingKeys(src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "design.go", src, 0)
	if err != nil {
		return nil, err
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "BackupDesignData" {
			fn = fd
			break
		}
	}
	if fn == nil || fn.Body == nil {
		return nil, errors.New("未找到 BackupDesignData 函数体")
	}

	var keys []string
	foundQuery := false
	foundZipWrite := false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "Where":
			if len(call.Args) != 2 {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			query, err := strconv.Unquote(lit.Value)
			if err != nil || !strings.Contains(query, "`key` NOT IN") {
				return true
			}
			list, ok := call.Args[1].(*ast.CompositeLit)
			if !ok {
				return true
			}
			foundQuery = true
			for _, elt := range list.Elts {
				ident, ok := elt.(*ast.Ident)
				if !ok {
					// 清单里出现字符串字面量等非标识符写法：护栏必须重新核对，不静默跳过。
					keys = append(keys, "<非标识符:"+fset.Position(elt.Pos()).String()+">")
					continue
				}
				keys = append(keys, ident.Name)
			}
		case "writeDataToZip":
			if len(call.Args) == 0 {
				return true
			}
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if name, err := strconv.Unquote(lit.Value); err == nil && name == "settings" {
					foundZipWrite = true
				}
			}
		}
		return true
	})
	if !foundQuery {
		return nil, errors.New("BackupDesignData 里没有 `key` NOT IN(?) 排除查询")
	}
	if !foundZipWrite {
		return nil, errors.New("BackupDesignData 里没有 writeDataToZip(\"settings\", ...)，排除清单可能已不作用于导出")
	}
	if len(keys) == 0 {
		return nil, errors.New("排除清单为空")
	}
	return keys, nil
}

func containsIdent(keys []string, want string) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}

func removeLineContaining(src, needle string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, needle) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
