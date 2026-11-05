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

// TestComputeAppIdentityHashStableAndScoped 锁定应用身份摘要的两个前提：
// 同输入必须同结果（否则唯一索引拦不住重复配置），任一维度变化必须换摘要
// （否则两个不同应用会互相顶掉对方的渠道配置）。
func TestComputeAppIdentityHashStableAndScoped(t *testing.T) {
	const (
		platform = "feishu"
		appId    = "cli_a1b2c3d4e5f6"
		orgId    = "tenant_9f8e7d"
		// 固定值由 sha256("feishu\x1fcli_a1b2c3d4e5f6\x1ftenant_9f8e7d") 独立算出。
		// 它锁死拼接顺序、分隔符与大小写：改了任何一处这条立刻红。
		wantHash = "6e97cc9760b37795b4ab6c64135c64f9d7aa4cbe1ef50a6fdc82b964f8166723"
	)
	base := model.ComputeAppIdentityHash(platform, appId, orgId)
	if base != wantHash {
		t.Fatalf("摘要与固定值不符：got %s want %s", base, wantHash)
	}
	if len(base) != 64 {
		t.Fatalf("摘要长度 %d，char(64) 列放不下（或补零读回对不上）", len(base))
	}
	if strings.Trim(base, "0123456789abcdef") != "" {
		t.Fatalf("摘要 %q 不是小写十六进制，与库里的 char(64) 存储约定不一致", base)
	}

	if again := model.ComputeAppIdentityHash(platform, appId, orgId); again != base {
		t.Errorf("同输入算出了两个摘要：%s vs %s，唯一索引会漏掉重复配置", base, again)
	}

	// TrimSpace 归一化：表单粘贴/平台回调带来的首尾空白不得改变身份。
	if padded := model.ComputeAppIdentityHash("  "+platform+"\t", appId+" ", "\n"+orgId); padded != base {
		t.Errorf("首尾空白改变了摘要：%s vs %s，同一个应用会被当成两个", padded, base)
	}

	scoped := []struct {
		name                   string
		platform, appId, orgId string
	}{
		{"平台不同", "dingtalk", appId, orgId},
		{"应用不同", platform, appId + "x", orgId},
		{"组织不同", platform, appId, orgId + "x"},
		{"组织从有到无", platform, appId, ""},
	}
	for _, c := range scoped {
		if got := model.ComputeAppIdentityHash(c.platform, c.appId, c.orgId); got == base {
			t.Errorf("%s 却算出同一摘要：唯一索引会把两个不同应用判成同一个", c.name)
		}
	}

	// 分隔符必须真的在起作用：把字符从应用ID挪到组织里要换摘要。
	// 若用空串拼接或可打印字符且未转义，这两种输入会撞成同一个摘要。
	if model.ComputeAppIdentityHash(platform, "ab", "c") == model.ComputeAppIdentityHash(platform, "a", "bc") {
		t.Error("应用ID与组织之间没有分隔：字段边界有歧义，不同应用可能撞同一摘要")
	}
	// 组织为空是合法输入（钉钉部分应用没有 corp_id 之外的归属），必须算得出确定值。
	if got := model.ComputeAppIdentityHash("dingtalk", "ding_app_01", ""); got != "d1a8e9b5cabc1b1f8d874f0fef9845d3d65e96a419a1c7b7360ef3898252a358" {
		t.Errorf("组织为空时的摘要与固定值不符：got %s", got)
	}
}

// parseGormIndexes 解析 gorm tag 里的索引声明，返回 索引名 → 是否唯一。
// 只认 index / uniqueIndex 两个键；priority:1、length:191 这类索引元数据带冒号，必须跳过，
// 否则会被当成索引名，让下面的断言看起来成立实则空转。
func parseGormIndexes(tag string) map[string]bool {
	indexes := make(map[string]bool)
	for _, part := range strings.Split(tag, ";") {
		key, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		unique := false
		switch key {
		case "uniqueIndex":
			unique = true
		case "index":
		default:
			continue
		}
		for _, name := range strings.Split(value, ",") {
			if name == "" || strings.Contains(name, ":") {
				continue
			}
			indexes[name] = indexes[name] || unique
		}
	}
	return indexes
}

// uniqueIndexNames 返回某字段参与的、带唯一性的索引名集合。
func uniqueIndexNames(t *testing.T, modelValue interface{}, fieldName string) map[string]bool {
	t.Helper()
	typ := reflect.TypeOf(modelValue)
	field, ok := typ.FieldByName(fieldName)
	if !ok {
		t.Fatalf("model.%s 缺少字段 %s", typ.Name(), fieldName)
	}
	names := make(map[string]bool)
	for name, unique := range parseGormIndexes(field.Tag.Get("gorm")) {
		if unique {
			names[name] = true
		}
	}
	return names
}

// TestMessageChannelHasUniqueAppIdentity 锁定规格第 4 节「同一企业应用只允许归属一个站点连接」：
// 配置层必须有自己的唯一约束，不能只靠 ChannelLease 的运行期兜底
// （两个站点各写一份配置、再争租约，表现为一边拿不到连接）。
func TestMessageChannelHasUniqueAppIdentity(t *testing.T) {
	typ := reflect.TypeOf(model.MessageChannel{})
	field, ok := typ.FieldByName("AppIdentityHash")
	if !ok {
		t.Fatal("model.MessageChannel 缺少 AppIdentityHash：同一企业应用可以被两个站点各写一份渠道配置")
	}
	if tag := field.Tag.Get("json"); tag != "-" {
		t.Errorf("AppIdentityHash 的 json tag 是 %q，必须为 \"-\"（摘要不对外输出）", tag)
	}
	gorm := field.Tag.Get("gorm")
	if !strings.Contains(gorm, "column:app_identity_hash") {
		t.Errorf("AppIdentityHash 的 gorm tag 不含 column:app_identity_hash：%q", gorm)
	}
	if !strings.Contains(gorm, "char(64)") {
		t.Errorf("AppIdentityHash 的列类型不是 char(64)：%q", gorm)
	}
	if got := uniqueIndexNames(t, model.MessageChannel{}, "AppIdentityHash"); !got["uk_app_identity_hash"] {
		t.Errorf("AppIdentityHash 没有 uk_app_identity_hash 唯一索引：应用归属约束缺失（实际唯一索引 %v）", got)
	} else if len(got) != 1 {
		t.Errorf("AppIdentityHash 挂了多个唯一索引 %v，列与索引应对齐到一个", got)
	}

	// 解析器自检：普通 index 不得被当成唯一索引，priority 元数据不得被当成索引名。
	self := parseGormIndexes("column:x;index:idx_a,priority:1;uniqueIndex:uk_b,priority:2")
	if unique, ok := self["idx_a"]; !ok {
		t.Error("解析器没解析出普通索引 idx_a")
	} else if unique {
		t.Error("解析器把普通索引 idx_a 当成了唯一索引")
	}
	if unique, ok := self["uk_b"]; !ok || !unique {
		t.Errorf("解析器没把 uk_b 标记为唯一索引：%v", self)
	}
	if self["priority"] {
		t.Error("解析器把 priority 元数据当成了索引名")
	}

	// 反向对照：同结构体里没有唯一索引的字段必须解析为空集合，
	// 否则 parseGormIndexes 恒返回真，上面的断言全是空转。
	if got := uniqueIndexNames(t, model.MessageChannel{}, "Name"); len(got) != 0 {
		t.Errorf("反向对照失效：Name 无唯一索引，解析结果却有 %v", got)
	}
	// 反向对照 2：普通 index（idx_site_id）不得被算进唯一索引集合。
	if got := uniqueIndexNames(t, model.MessageChannel{}, "SiteId"); len(got) != 0 {
		t.Errorf("反向对照失效：SiteId 只有普通索引，解析出的唯一索引却是 %v", got)
	}
}

// TestAgentReceptionPolicyUniquePerAgent 锁定「每站点每智能体只有一条策略」。
// 两列必须挂同一个索引名：各挂各的单列唯一索引会变成「一个智能体全站只能出现一次」，
// 那是比缺失约束更糟的错误语义。
func TestAgentReceptionPolicyUniquePerAgent(t *testing.T) {
	for _, fieldName := range []string{"SiteId", "AgentId"} {
		got := uniqueIndexNames(t, model.AgentReceptionPolicy{}, fieldName)
		if !got["uk_agent_policy"] {
			t.Errorf("AgentReceptionPolicy.%s 没有 uk_agent_policy 唯一索引：同一智能体可以有互相矛盾的策略（实际唯一索引 %v）", fieldName, got)
			continue
		}
		if len(got) != 1 {
			t.Errorf("AgentReceptionPolicy.%s 挂了多个唯一索引 %v，复合唯一约束只能是一个索引名", fieldName, got)
		}
	}

	// 反向对照：别的模型同名字段不带 uk_agent_policy，证明断言不是恒真。
	if got := uniqueIndexNames(t, model.MessageChannel{}, "AgentId"); len(got) != 0 {
		t.Errorf("反向对照失效：MessageChannel.AgentId 没有 uk_agent_policy，解析结果却有 %v", got)
	}
}
