package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"kandaoni.com/anqicms/model"
)

// channelModels 是本任务新增的全部模型。
// 注册断言与 SiteId 断言共用这一份清单：新增模型时漏改一处会直接编译失败，
// 不会出现「测试列了 13 个、迁移里只有 12 个」的静默偏差。
var channelModels = []interface{}{
	model.MessageChannel{},
	model.AgentReceptionPolicy{},
	model.ChannelBindingToken{},
	model.ChannelBinding{},
	model.ChannelTarget{},
	model.ChannelInbox{},
	model.ChannelSession{},
	model.ChannelApproval{},
	model.ChannelLease{},
	model.ChannelAudit{},
	model.NotificationRule{},
	model.NotificationTemplate{},
	model.NotificationEvent{},
	model.NotificationDelivery{},
}

func channelModelName(v interface{}) string {
	return reflect.TypeOf(v).Name()
}

// scanMigratedModels 解析 AutoMigrateDB 里 `var models = []interface{}{...}` 的元素类型名。
//
// 用 go/ast 而不是正则：注释、字符串、其它同名切片都不该被当成迁移清单；
// 同时返回 found，避免「没找到切片」被当成「模型全部缺失」或反过来空转通过。
func scanMigratedModels(filename string) (names map[string]bool, found bool, err error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return nil, false, err
	}
	names, found = scanMigratedModelsInFile(file)
	return names, found, nil
}

// scanMigratedModelsInFile 是真正的扫描逻辑；负向对照测试复用同一份代码，
// 否则对照测试自己写一套遍历，扫描器坏了它照样通过。
//
// dao.go 里写的是 `var models = []interface{}{...}`（DeclStmt + ValueSpec），
// 不是 `models = ...` 赋值语句；两种形式都认，免得后续改写法后扫描器静默失效。
func scanMigratedModelsInFile(file *ast.File) (names map[string]bool, found bool) {
	names = make(map[string]bool)
	collect := func(expr ast.Expr) {
		lit, ok := expr.(*ast.CompositeLit)
		if !ok {
			return
		}
		found = true
		for _, elt := range lit.Elts {
			if name := modelTypeName(elt); name != "" {
				names[name] = true
			}
		}
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "AutoMigrateDB" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.DeclStmt:
				gen, ok := node.Decl.(*ast.GenDecl)
				if !ok {
					return true
				}
				for _, spec := range gen.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range valueSpec.Names {
						if name.Name == "models" && i < len(valueSpec.Values) {
							collect(valueSpec.Values[i])
						}
					}
				}
			case *ast.AssignStmt:
				for i, lhs := range node.Lhs {
					ident, ok := lhs.(*ast.Ident)
					if !ok || ident.Name != "models" || i >= len(node.Rhs) {
						continue
					}
					collect(node.Rhs[i])
				}
			}
			return true
		})
	}
	return names, found
}

// modelTypeName 从 `&model.Foo{}` / `model.Foo{}` 中取出 Foo。
func modelTypeName(expr ast.Expr) string {
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return ""
	}
	sel, ok := lit.Type.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "model" {
		return ""
	}
	return sel.Sel.Name
}

// TestChannelTablesRegistered 断言全部新模型都进了 AutoMigrateDB 的迁移清单。
// 不写死模型总数：增量注册不该让本测试变红，漏注册才该红。
func TestChannelTablesRegistered(t *testing.T) {
	names, found, err := scanMigratedModels("dao.go")
	if err != nil {
		t.Fatalf("解析 provider/dao.go 失败：%v", err)
	}
	if !found {
		t.Fatal("在 AutoMigrateDB 里没找到 models 切片：扫描器失效会让本测试永远通过")
	}
	// 反向对照：真的解析出了迁移清单（而不是空集合）。
	if len(names) < 20 {
		t.Fatalf("只解析出 %d 个模型，明显不是真实迁移清单，扫描器可能有误", len(names))
	}

	for _, m := range channelModels {
		name := channelModelName(m)
		if !names[name] {
			t.Errorf("model.%s 没有注册到 AutoMigrateDB 的 models 切片：新表不会被创建", name)
		}
	}
}

// TestChannelMigrationScannerRejectsMissingModel 是上面那条断言的负向对照：
// 用一份「少了 ChannelInbox」的合成源码喂给扫描器，必须报告缺失。
// 没有这条对照，扫描器写错（例如恒返回全集）也不会有任何测试变红。
func TestChannelMigrationScannerRejectsMissingModel(t *testing.T) {
	const src = `package provider

func AutoMigrateDB() {
	var models = []interface{}{
		&model.Admin{},
		&model.MessageChannel{},
	}
	_ = models
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synthetic_dao.go", src, 0)
	if err != nil {
		t.Fatalf("合成源码解析失败：%v", err)
	}
	names, found := scanMigratedModelsInFile(file)
	if !found {
		t.Fatal("扫描器没在合成源码里找到 models 切片，锚点失效")
	}

	if !names["MessageChannel"] {
		t.Fatal("扫描器连合成源码里的 MessageChannel 都没解析出来，锚点失效")
	}
	if names["ChannelInbox"] {
		t.Error("合成源码里没有 ChannelInbox，扫描器却报告存在：注册断言不可信")
	}
}

// TestChannelModelsHaveSiteId 断言每个新表结构体都带 SiteId，
// 并核对 gorm column 就是 site_id —— 字段改名但列名写错同样会漏站点过滤。
func TestChannelModelsHaveSiteId(t *testing.T) {
	for _, m := range channelModels {
		typ := reflect.TypeOf(m)
		field, ok := typ.FieldByName("SiteId")
		if !ok {
			t.Errorf("model.%s 缺少 SiteId 字段：所有渠道表都必须带站点过滤", typ.Name())
			continue
		}
		if got := field.Tag.Get("gorm"); !strings.Contains(got, "column:site_id") {
			t.Errorf("model.%s.SiteId 的 gorm tag 不含 column:site_id：%q", typ.Name(), got)
		}
		if field.Type.Kind() != reflect.Uint {
			t.Errorf("model.%s.SiteId 类型为 %s，与仓库其它 SiteId 的无符号主键习惯不一致", typ.Name(), field.Type.Kind())
		}
	}

	// 反向对照：确定这条断言真的会失败。
	// model.Setting 是 key/value 表，没有 SiteId；若它也被判为「有 SiteId」，说明检查恒真。
	if _, ok := reflect.TypeOf(model.Setting{}).FieldByName("SiteId"); ok {
		t.Fatal("反向对照失效：model.Setting 竟然有 SiteId，请换一个没有 SiteId 的模型做对照")
	}
}

// TestChannelSensitiveFieldsNotSerialized 锁定「后台响应不得泄露应用密钥与 token 摘要」。
func TestChannelSensitiveFieldsNotSerialized(t *testing.T) {
	cases := []struct {
		model interface{}
		field string
	}{
		{model.MessageChannel{}, "AppSecret"},
		{model.ChannelBindingToken{}, "TokenHash"},
	}
	for _, c := range cases {
		typ := reflect.TypeOf(c.model)
		field, ok := typ.FieldByName(c.field)
		if !ok {
			t.Errorf("model.%s 缺少字段 %s", typ.Name(), c.field)
			continue
		}
		if tag := field.Tag.Get("json"); tag != "-" {
			t.Errorf("model.%s.%s 的 json tag 是 %q，必须为 \"-\"：敏感字段不得出现在后台响应里", typ.Name(), c.field, tag)
		}
	}
}
