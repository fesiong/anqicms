package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"kandaoni.com/anqicms/model"
)

// channelModels 是本任务新增的全部模型。
// 注册断言与 SiteId 断言共用这一份清单：新增模型时漏改一处会直接编译失败，
// 不会出现「测试列了 13 个、迁移里只有 12 个」的静默偏差。
var channelModels = []interface{}{
	model.MessageChannel{},
	model.ChannelBindingToken{},
	model.ChannelBinding{},
	model.ChannelTarget{},
	model.ChannelInbox{},
	model.ChannelSession{},
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

// TestMessageChannelBeforeSaveSetsAppIdentityHash 锁定「写入口不必手工调摘要函数」。
// 摘要列是 default:” + 唯一索引：漏算时第一条空摘要能插进去、第二条才炸，
// 错误信息还会伪装成「该应用已被别的站点占用」。所以 hook 必须自己补算。
func TestMessageChannelBeforeSaveSetsAppIdentityHash(t *testing.T) {
	const (
		platform = "feishu"
		appId    = "cli_a1b2c3d4e5f6"
		orgId    = "tenant_9f8e7d"
	)
	m := &model.MessageChannel{Platform: platform, AppId: appId, OrgId: orgId}
	if err := m.BeforeSave(nil); err != nil {
		t.Fatalf("BeforeSave 返回错误：%v", err)
	}
	want := model.ComputeAppIdentityHash(platform, appId, orgId)
	if m.AppIdentityHash != want {
		t.Fatalf("hook 没写入应用身份摘要：got %q want %q", m.AppIdentityHash, want)
	}

	// 幂等：保存两次（GORM 的 Save 会先查后写，可能触发多次）不得改变摘要。
	first := m.AppIdentityHash
	if err := m.BeforeSave(nil); err != nil {
		t.Fatalf("第二次 BeforeSave 返回错误：%v", err)
	}
	if m.AppIdentityHash != first {
		t.Errorf("同一对象两次 BeforeSave 算出了不同摘要：%s vs %s", first, m.AppIdentityHash)
	}
}

// TestMessageChannelBeforeSaveSkipsEmptyAppId 是上一条的负向对照，也是部分更新的安全阀：
// app_id 为空时 hook 必须原样返回，否则只带主键的整结构体更新会把空摘要写进已有行，
// 既毁掉原摘要，又让第二条渠道插入时撞唯一索引。
func TestMessageChannelBeforeSaveSkipsEmptyAppId(t *testing.T) {
	cases := []struct {
		name    string
		channel model.MessageChannel
	}{
		{"完全零值", model.MessageChannel{}},
		{"只有主键与名称字段（部分更新的典型形态）", model.MessageChannel{Id: 7, Revision: 3, Name: "x"}},
		{"app_id 只有空白", model.MessageChannel{Platform: "feishu", AppId: "   "}},
	}
	for _, c := range cases {
		m := c.channel
		if err := m.BeforeSave(nil); err != nil {
			t.Errorf("%s：BeforeSave 返回错误：%v", c.name, err)
			continue
		}
		if m.AppIdentityHash != "" {
			t.Errorf("%s：hook 在 app_id 为空时写入了摘要 %q，会顶掉已有行的身份", c.name, m.AppIdentityHash)
		}
	}
}

// TestNotificationDeliveryBeforeSaveSetsDeliveryHash 锁定投递摘要由 hook 派生，
// 并用固定值锁死拼接协议：十进制 + \x1f 连接 + SHA-256 + 小写 hex，顺序为 (event, rule, target)。
func TestNotificationDeliveryBeforeSaveSetsDeliveryHash(t *testing.T) {
	const (
		eventId  = 11
		ruleId   = 22
		targetId = 33
		// 固定值由 sha256("11\x1f22\x1f33") 独立算出。
		wantHash = "455413770e1cbd99fcb01855d7cccff51aaaabccfaf45fa3867f9aaa0977ea25"
	)
	d := &model.NotificationDelivery{EventId: eventId, RuleId: ruleId, TargetId: targetId}
	if err := d.BeforeSave(nil); err != nil {
		t.Fatalf("BeforeSave 返回错误：%v", err)
	}
	if want := model.ComputeDeliveryHash(eventId, ruleId, targetId); d.DeliveryHash != want {
		t.Fatalf("hook 没写入投递摘要：got %q want %q", d.DeliveryHash, want)
	}
	if d.DeliveryHash != wantHash {
		t.Errorf("投递摘要与固定值不符：got %s want %s（拼接顺序、分隔符或大小写被改了）", d.DeliveryHash, wantHash)
	}

	if again := model.ComputeDeliveryHash(eventId, ruleId, targetId); again != d.DeliveryHash {
		t.Errorf("同三元组算出了两个摘要：%s vs %s", again, d.DeliveryHash)
	}

	// 三元组顺序敏感：交换 rule/target 必须换摘要，否则两个不同投递任务会互相顶掉。
	if model.ComputeDeliveryHash(eventId, ruleId, targetId) == model.ComputeDeliveryHash(eventId, targetId, ruleId) {
		t.Error("交换规则/目标后摘要不变：三元组顺序没有参与计算")
	}
	// 分隔符必须真的在起作用：(1, 23) 与 (12, 3) 直接拼接会撞成同一摘要。
	if model.ComputeDeliveryHash(1, 23, 4) == model.ComputeDeliveryHash(12, 3, 4) {
		t.Error("三元组之间没有分隔：字段边界有歧义，不同投递任务可能撞同一摘要")
	}
}

// TestNotificationDeliveryBeforeSaveSkipsZeroTriple 是上一条的负向对照：
// 三元组任一为 0 都视为「未填全」，hook 必须原样返回，不得写入半截摘要。
func TestNotificationDeliveryBeforeSaveSkipsZeroTriple(t *testing.T) {
	cases := []struct {
		name     string
		delivery model.NotificationDelivery
	}{
		{"完全零值", model.NotificationDelivery{}},
		{"缺规则", model.NotificationDelivery{EventId: 11, TargetId: 33}},
		{"缺目标", model.NotificationDelivery{EventId: 11, RuleId: 22}},
		{"缺事件", model.NotificationDelivery{RuleId: 22, TargetId: 33}},
	}
	for _, c := range cases {
		d := c.delivery
		if err := d.BeforeSave(nil); err != nil {
			t.Errorf("%s：BeforeSave 返回错误：%v", c.name, err)
			continue
		}
		if d.DeliveryHash != "" {
			t.Errorf("%s：hook 在三元组不全时写入了摘要 %q", c.name, d.DeliveryHash)
		}
	}
}

// TestNewChannelLeaseDerivesAppIdentityHash 锁定租约构造时同步派生摘要：
// ChannelLease 不存 platform/app_id，摘要只能来自渠道配置，漏算就是空摘要租约，
// 会让「同一企业应用只允许一个活动所有者」的判据失效。
func TestNewChannelLeaseDerivesAppIdentityHash(t *testing.T) {
	const (
		siteId    = 9
		owner     = "worker-a"
		expiresAt = int64(1700000000)
	)
	ch := &model.MessageChannel{Platform: model.ChannelPlatformDingTalk, AppId: "ding_app_01", OrgId: "corp_1"}
	lease := model.NewChannelLease(siteId, ch, owner, expiresAt)

	want := model.ComputeAppIdentityHash(ch.Platform, ch.AppId, ch.OrgId)
	if lease.AppIdentityHash != want {
		t.Errorf("租约的应用身份摘要与渠道不一致：got %q want %q", lease.AppIdentityHash, want)
	}
	if lease.AppIdentityHash == "" {
		t.Error("租约摘要为空：空摘要会让唯一索引把不同应用判成同一个")
	}
	if lease.SiteId != siteId {
		t.Errorf("SiteId 未填入：got %d want %d", lease.SiteId, siteId)
	}
	if lease.Owner != owner {
		t.Errorf("Owner 未填入：got %q want %q", lease.Owner, owner)
	}
	if lease.ExpiresAt != expiresAt {
		t.Errorf("ExpiresAt 未填入：got %d want %d", lease.ExpiresAt, expiresAt)
	}

	// 摘要与实际应用一一对应：换应用必须换摘要（否则租约会在两个应用间串台）。
	other := model.NewChannelLease(siteId, &model.MessageChannel{Platform: model.ChannelPlatformDingTalk, AppId: "ding_app_02"}, owner, expiresAt)
	if other.AppIdentityHash == lease.AppIdentityHash {
		t.Error("不同应用构造出了同一摘要的租约")
	}
}

// TestChannelUniqueConstraintsDeclared 把「字段 → 唯一索引名」逐条钉住。
// 这些索引是入库去重与归属约束的唯一屏障：索引名被改、tag 被删都不会让编译或
// 现有测试变红，只会让重复数据悄悄进库，所以必须显式断言。
func TestChannelUniqueConstraintsDeclared(t *testing.T) {
	cases := []struct {
		modelValue interface{}
		field      string
		indexName  string
	}{
		{model.ChannelBindingToken{}, "TokenHash", "uk_token_hash"},
		{model.ChannelBinding{}, "IdentityHash", "uk_identity_hash"},
		{model.ChannelTarget{}, "TargetHash", "uk_target_hash"},
		{model.ChannelInbox{}, "EventHash", "uk_event_hash"},
		{model.ChannelSession{}, "SessionHash", "uk_session_hash"},
		{model.NotificationEvent{}, "EventKeyHash", "uk_event_key_hash"},
		{model.NotificationDelivery{}, "DeliveryHash", "uk_delivery_hash"},
	}
	for _, c := range cases {
		typ := reflect.TypeOf(c.modelValue)
		got := uniqueIndexNames(t, c.modelValue, c.field)
		if !got[c.indexName] {
			t.Errorf("model.%s.%s 缺少唯一索引 %s：重复数据会直接落库（实际唯一索引 %v）", typ.Name(), c.field, c.indexName, got)
			continue
		}
		if len(got) != 1 {
			t.Errorf("model.%s.%s 挂了多个唯一索引 %v，列与索引应对齐到一个", typ.Name(), c.field, got)
		}
	}

	// 反向对照：解析器对没有唯一索引的字段必须返回空集合，否则上面的断言全是空转。
	if got := uniqueIndexNames(t, model.ChannelBindingToken{}, "SiteId"); len(got) != 0 {
		t.Errorf("反向对照失效：ChannelBindingToken.SiteId 只有普通索引，解析出的唯一索引却是 %v", got)
	}
}

// gormIndexPriority 取出某字段在指定 index 声明里的 priority（未声明或索引不存在返回 0）。
func gormIndexPriority(tag, indexName string) int {
	for _, part := range strings.Split(tag, ";") {
		key, value, ok := strings.Cut(part, ":")
		if !ok || key != "index" && key != "uniqueIndex" {
			continue
		}
		segments := strings.Split(value, ",")
		if segments[0] != indexName {
			continue
		}
		for _, meta := range segments[1:] {
			raw, ok := strings.CutPrefix(meta, "priority:")
			if !ok {
				continue
			}
			n, err := strconv.Atoi(raw)
			if err != nil {
				return 0
			}
			return n
		}
	}
	return 0
}

// TestChannelAuditIndexOrdersSiteFirst 锁定审计索引列序 (site_id, created_time)。
// 审计查询恒为 WHERE site_id=? ORDER BY created_time DESC：GORM 按 priority 升序建列，
// 一旦把 created_time 写成 priority:1，索引就变成 (created_time, site_id)，
// 站点过滤用不上前缀，站点一多就退化成全表扫描（功能不坏、只是慢，没人会发现）。
func TestChannelAuditIndexOrdersSiteFirst(t *testing.T) {
	typ := reflect.TypeOf(model.ChannelAudit{})
	fieldTag := func(name string) string {
		field, ok := typ.FieldByName(name)
		if !ok {
			t.Fatalf("model.ChannelAudit 缺少字段 %s", name)
		}
		return field.Tag.Get("gorm")
	}
	sitePriority := gormIndexPriority(fieldTag("SiteId"), "idx_site_created")
	createdPriority := gormIndexPriority(fieldTag("CreatedTime"), "idx_site_created")
	if sitePriority == 0 || createdPriority == 0 {
		t.Fatalf("idx_site_created 缺失或没声明 priority：site=%d created=%d", sitePriority, createdPriority)
	}
	if sitePriority >= createdPriority {
		t.Errorf("idx_site_created 的列序反了：site_id priority=%d, created_time priority=%d，建出来会先排时间列", sitePriority, createdPriority)
	}

	// 解析器自检 + 反向对照：priority 必须被真读出来，且反序能被判成反序。
	if got := gormIndexPriority("column:x;index:idx_a,priority:2", "idx_a"); got != 2 {
		t.Errorf("解析器没读出 priority:2，实际 %d", got)
	}
	if got := gormIndexPriority("column:x;index:idx_a,priority:2", "idx_other"); got != 0 {
		t.Errorf("解析器把别的索引的 priority 算到了 idx_other 上：%d", got)
	}
	if !(gormIndexPriority("column:x;index:idx_a,priority:2", "idx_a") > gormIndexPriority("column:y;index:idx_a,priority:1", "idx_a")) {
		t.Error("反向对照失效：priority 2 vs 1 应判为反序")
	}
}
