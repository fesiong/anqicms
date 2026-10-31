package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// TestCatalogRecognizesPayloadHelpers 保证生成器认得出 readJSONPayload 这类包装函数的绑定目标。
//
// 背景：提交 5263d86d 把 handler 里的 `ctx.ReadJSON(&req)` 换成了
// `readJSONPayload(ctx, &req)`（controller/manageController/payload_guard.go），
// 而 analyzeHandlerBody 只认 `ctx.Xxx(...)` 形式。结果 `go run ./cmd/apicatalog`
// 会把 53 个端点的 param_source 从 struct 退化成 none、params 清空——功能不报错，
// 只是 MCP 的 api_invoke 和意图层参数门禁瞬间失去字段清单。
func TestCatalogRecognizesPayloadHelpers(t *testing.T) {
	bindings, err := collectHandlerBindings("../controller/manageController")
	if err != nil {
		t.Fatalf("解析控制器源码失败：%v", err)
	}

	// CategoryDetailForm 是典型：`var req request.Category` + readJSONPayload(ctx, &req)。
	b, ok := bindings["CategoryDetailForm"]
	if !ok {
		t.Fatal("没找到 CategoryDetailForm——护栏本身失效了")
	}
	if b.source != "struct" {
		t.Fatalf("CategoryDetailForm 的取参方式应为 struct，实际 %q（包装函数没被识别）", b.source)
	}
	if !strings.Contains(b.structType, "Category") {
		t.Errorf("绑定的结构体类型不对：%q", b.structType)
	}

	// 再抽查两个不同包/不同写法，避免只对某一个形状成立。
	checked := 0
	for _, name := range []string{"PluginTagSave", "SaveModule", "AttachmentDelete", "NavSave"} {
		v, found := bindings[name]
		if !found {
			continue
		}
		checked++
		if v.source != "struct" {
			t.Errorf("%s 的取参方式应为 struct，实际 %q", name, v.source)
		}
	}
	if checked == 0 {
		t.Fatal("抽查的 handler 一个都没找到，名单需要更新")
	}
}

// TestNoUnrecognizedPayloadHelper 是一个"防复发"闸门：扫描控制器源码里所有
// 「第一个实参是 ctx 的裸函数调用」，凡是生成器不认识的，就报红。
//
// 之所以能用这个判据而不是逐个点名：这类 helper 的共同签名一定是
// `f(ctx, ...)`（要写响应、要读请求体），而 ctx.ReadJSON 这类方法调用走的是
// SelectorExpr 分支，不会落到这里。
func TestNoUnrecognizedPayloadHelper(t *testing.T) {
	// 必须把子目录也列出来：parseGoFilesInDir 不递归，只写 "../controller"
	// 会连一个 helper 都扫不到，测试照样绿——正是"护栏自己失效"的形态。
	roots := []string{"../controller", "../controller/manageController"}
	fset := token.NewFileSet()
	seen := map[string]string{}
	fileCount := 0

	for _, root := range roots {
		files, err := parseGoFilesInDir(fset, root, parser.ParseComments)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", root, err)
		}
		fileCount += len(files)
		for _, file := range files {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				ident, ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				first, ok := call.Args[0].(*ast.Ident)
				if !ok || first.Name != "ctx" {
					return true
				}
				if isJSONPayloadHelper(ident.Name) {
					return true
				}
				if _, dup := seen[ident.Name]; !dup {
					seen[ident.Name] = fset.Position(call.Pos()).String()
				}
				return true
			})
		}
	}

	if fileCount < 50 {
		t.Fatalf("只解析到 %d 个控制器文件，明显不对——护栏扫错目录了", fileCount)
	}

	if len(seen) == 0 {
		return
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	var msgs []string
	for _, name := range names {
		// iris 的构造函数与日志函数也会带 ctx 首参，但都不取请求体；只有名字里
		// 带 Payload/JSON/Body/Bind 的才可能是取参 helper。
		lower := strings.ToLower(name)
		if !strings.Contains(lower, "payload") && !strings.Contains(lower, "json") && !strings.Contains(lower, "bind") {
			continue
		}
		msgs = append(msgs, name+" @ "+seen[name])
	}
	if len(msgs) > 0 {
		t.Errorf("发现未登记的取参包装函数，生成器会把它们的端点 schema 清空：\n  %s\n修法：加进 provider/api_catalog.go 的 jsonPayloadHelpers 白名单", strings.Join(msgs, "\n  "))
	}
}
