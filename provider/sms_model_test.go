package provider

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gorm.io/gorm/schema"
)

// smsModels 是本任务新增的两张表对应的模型清单。
// 注册护栏用 go/ast 穷举迁移清单，不写死模型总数：增量注册不该让护栏变红，漏注册才该红。
var smsModels = []string{"SmsCode", "SmsLog"}

const smsModelSourcePath = "../model/sms.go"

// smsLogForbiddenFields 是日志表不得出现的字段名：验证码明文或渲染正文。
var smsLogForbiddenFields = []string{"Code", "CodeHash", "Body", "Text", "Content"}

// loadSmsReal 读取真实 model/sms.go，同时返回源码文本（供反向对照做单点变异）
// 与解析好的 *ast.File（供正向检查器）。步骤 2 里文件尚不存在，这里直接让护栏变红。
func loadSmsReal(t *testing.T) (string, *ast.File) {
	t.Helper()
	src, err := os.ReadFile(smsModelSourcePath)
	if err != nil {
		t.Fatalf("读取 model/sms.go 失败（模型尚未创建）：%v", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, smsModelSourcePath, src, 0)
	if err != nil {
		t.Fatalf("解析 model/sms.go 失败：%v", err)
	}
	return string(src), file
}

// parseSmsSynthetic 把一份合成源码解析成 *ast.File，喂给与正向完全相同的检查器。
func parseSmsSynthetic(t *testing.T, src string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synthetic_sms.go", src, 0)
	if err != nil {
		t.Fatalf("合成源码解析失败：%v", err)
	}
	return file
}

// smsStructType 用 go/ast 找到命名结构体。扫描器（scanMigratedModels 等）一律复用
// channel_model_test.go 的既有实现，这里只补一个「按结构体取字段 tag」的小工具：
// 反向对照喂的是未编译进二进制的合成源码，reflect 拿不到它，必须走 AST。
func smsStructType(file *ast.File, structName string) *ast.StructType {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != structName {
				continue
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				return st
			}
		}
	}
	return nil
}

func smsStructTag(file *ast.File, structName, fieldName string) (reflect.StructTag, bool) {
	st := smsStructType(file, structName)
	if st == nil {
		return "", false
	}
	for _, f := range st.Fields.List {
		for _, name := range f.Names {
			if name.Name != fieldName {
				continue
			}
			if f.Tag == nil {
				return "", true
			}
			raw, err := strconv.Unquote(f.Tag.Value)
			if err != nil {
				return "", true
			}
			return reflect.StructTag(raw), true
		}
	}
	return "", false
}

func smsStructFields(file *ast.File, structName string) ([]string, bool) {
	st := smsStructType(file, structName)
	if st == nil {
		return nil, false
	}
	var fields []string
	for _, f := range st.Fields.List {
		for _, name := range f.Names {
			fields = append(fields, name.Name)
		}
	}
	return fields, true
}

// smsDeclaresTableName 判断源码是否为指定结构体声明了 TableName() 方法。
// 一旦声明，GORM 复数化命名策略就被覆盖，表名不再由结构体名推导。
func smsDeclaresTableName(file *ast.File, structName string) bool {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "TableName" || fn.Recv == nil {
			continue
		}
		for _, r := range fn.Recv.List {
			var ident string
			switch typ := r.Type.(type) {
			case *ast.Ident:
				ident = typ.Name
			case *ast.StarExpr:
				if x, ok := typ.X.(*ast.Ident); ok {
					ident = x.Name
				}
			}
			if ident == structName {
				return true
			}
		}
	}
	return false
}

// ---- 四个检查器：正向（真实源码）与反向（合成坏源码）都只喂它们 ----

func checkSmsRegistered(names map[string]bool, found bool) error {
	if !found {
		return fmt.Errorf("在 AutoMigrateDB 里没找到 models 切片：扫描器失效会让护栏永远通过")
	}
	if len(names) < 20 {
		return fmt.Errorf("只解析出 %d 个模型，明显不是真实迁移清单（下限自检，非写死总数）", len(names))
	}
	for _, m := range smsModels {
		if !names[m] {
			return fmt.Errorf("model.%s 没有注册到 AutoMigrateDB 的 models 切片：新表不会被创建", m)
		}
	}
	return nil
}

func checkSmsIndexOrders(file *ast.File) error {
	cases := []struct {
		structName, field, index string
		wantPriority             int
	}{
		{"SmsLog", "SiteId", "idx_site_created", 1},
		{"SmsLog", "CreatedTime", "idx_site_created", 2},
		{"SmsCode", "SiteId", "idx_site_phone_scene", 1},
		{"SmsCode", "PhoneCountry", "idx_site_phone_scene", 2},
		{"SmsCode", "Phone", "idx_site_phone_scene", 3},
		{"SmsCode", "Scene", "idx_site_phone_scene", 4},
	}
	for _, c := range cases {
		tag, ok := smsStructTag(file, c.structName, c.field)
		if !ok {
			return fmt.Errorf("缺少字段 %s.%s", c.structName, c.field)
		}
		gormTag := tag.Get("gorm")
		if _, declared := parseGormIndexes(gormTag)[c.index]; !declared {
			return fmt.Errorf("%s.%s 的 gorm tag 未声明索引 %s：%q", c.structName, c.field, c.index, gormTag)
		}
		if got := gormIndexPriority(gormTag, c.index); got != c.wantPriority {
			return fmt.Errorf("%s.%s 在 %s 的 priority=%d，期望 %d（索引列序被改动）", c.structName, c.field, c.index, got, c.wantPriority)
		}
	}
	return nil
}

func checkSmsFieldJSONTag(file *ast.File, structName, fieldName, want string) error {
	tag, ok := smsStructTag(file, structName, fieldName)
	if !ok {
		return fmt.Errorf("缺少字段 %s.%s", structName, fieldName)
	}
	if got := tag.Get("json"); got != want {
		return fmt.Errorf("%s.%s 的 json tag 是 %q，期望 %q", structName, fieldName, got, want)
	}
	return nil
}

func checkSmsLogNoPlainText(file *ast.File) error {
	fields, ok := smsStructFields(file, "SmsLog")
	if !ok {
		return fmt.Errorf("源码里找不到 SmsLog 结构体：检查器无法判定")
	}
	set := make(map[string]bool, len(fields))
	for _, f := range fields {
		set[f] = true
	}
	for _, bad := range smsLogForbiddenFields {
		if set[bad] {
			return fmt.Errorf("SmsLog 出现禁字段 %s：日志表不得存验证码明文或渲染正文", bad)
		}
	}
	return nil
}

// ---- 护栏 1：迁移注册 ----

func TestSmsTablesRegistered(t *testing.T) {
	names, found, err := scanMigratedModels("dao.go")
	if err != nil {
		t.Fatalf("解析 provider/dao.go 失败：%v", err)
	}
	if err := checkSmsRegistered(names, found); err != nil {
		t.Fatal(err)
	}

	// 反向对照：合成一份「去掉这两项」的 models 切片，喂同一个扫描器 + 同一个检查器，
	// 必须被判缺失（红）；补回两项后同一个检查器必须判合规（绿）。
	const missingSrc = `package provider

func AutoMigrateDB() {
    var models = []interface{}{
        &model.Admin{}, &model.User{}, &model.Setting{}, &model.Website{},
        &model.Archive{}, &model.Category{}, &model.Comment{}, &model.Link{},
        &model.Tag{}, &model.Nav{}, &model.Guestbook{}, &model.Keyword{},
        &model.Material{}, &model.Attachment{}, &model.Anchor{}, &model.Redirect{},
        &model.Module{}, &model.Order{}, &model.Payment{}, &model.Finance{},
        &model.NotificationDelivery{},
    }
    _ = models
}
`
	badNames, badFound := scanMigratedModelsInFile(parseSmsSynthetic(t, missingSrc))
	if !badFound || !badNames["NotificationDelivery"] {
		t.Fatal("反向对照锚点失效：扫描器没解析出合成清单里的 NotificationDelivery")
	}
	if err := checkSmsRegistered(badNames, badFound); err == nil {
		t.Fatal("反向对照失效：去掉 SmsCode/SmsLog 的合成清单竟被判为注册齐全")
	}
	restoredSrc := strings.Replace(missingSrc,
		"&model.NotificationDelivery{},",
		"&model.NotificationDelivery{},\n        &model.SmsCode{}, &model.SmsLog{},", 1)
	goodNames, goodFound := scanMigratedModelsInFile(parseSmsSynthetic(t, restoredSrc))
	if err := checkSmsRegistered(goodNames, goodFound); err != nil {
		t.Fatalf("恢复后同一个检查器仍报缺失：%v", err)
	}
}

// ---- 护栏 2：索引列序 ----

func TestSmsLogIndexOrdersSiteFirst(t *testing.T) {
	src, real := loadSmsReal(t)
	if err := checkSmsIndexOrders(real); err != nil {
		t.Fatal(err)
	}
	// 扫描器自检：字段 tag 必须真被读到，避免 smsStructTag 恒返回空让断言空转。
	if tag, ok := smsStructTag(real, "SmsLog", "SiteId"); !ok || !strings.Contains(tag.Get("gorm"), "column:site_id") {
		t.Fatal("扫描器失效：没读出 SmsLog.SiteId 的 gorm tag")
	}

	// 反向对照：把 idx_site_created 的两个 priority 对调（site_id 1→2、created_time 2→1），
	// 同一个检查器必须判违规（红）；未变异的原源码喂同一个检查器必须判合规（绿）。
	badSrc := strings.Replace(src, "index:idx_site_created,priority:1", "index:idx_site_created,priority:__SWAP__", 1)
	badSrc = strings.Replace(badSrc, "index:idx_site_created,priority:2", "index:idx_site_created,priority:1", 1)
	badSrc = strings.Replace(badSrc, "index:idx_site_created,priority:__SWAP__", "index:idx_site_created,priority:2", 1)
	if badSrc == src {
		t.Fatal("反向对照锚点失效：priority 没被替换，变异未生效")
	}
	if err := checkSmsIndexOrders(parseSmsSynthetic(t, badSrc)); err == nil {
		t.Fatal("反向对照失效：对调 priority 后检查器竟判列序合规")
	}
	if err := checkSmsIndexOrders(parseSmsSynthetic(t, src)); err != nil {
		t.Fatalf("恢复后同一个检查器仍报列序错误：%v", err)
	}
}

// ---- 护栏 3：摘要不序列化 ----

func TestSmsCodeHashNotSerialized(t *testing.T) {
	src, real := loadSmsReal(t)
	if err := checkSmsFieldJSONTag(real, "SmsCode", "CodeHash", "-"); err != nil {
		t.Fatal(err)
	}

	// 正向对照：同一个检查器对 SmsCode.Phone 的正常 json:"phone" 必须判合规，
	// 证明检查器不是「恒返回违规」（不恒假），也不是只对 "-" 特殊放行。
	if err := checkSmsFieldJSONTag(real, "SmsCode", "Phone", "phone"); err != nil {
		t.Fatalf("正向对照失效：正常字段 Phone 竟被判违规：%v", err)
	}

	// 反向对照：合成一份 CodeHash tag 为 json:"code_hash" 的源码，同一个检查器必须判违规（红）。
	badSrc := strings.Replace(src,
		`json:"-" gorm:"column:code_hash`, `json:"code_hash" gorm:"column:code_hash`, 1)
	if badSrc == src {
		t.Fatal("反向对照锚点失效：CodeHash 的 json tag 没被替换")
	}
	if err := checkSmsFieldJSONTag(parseSmsSynthetic(t, badSrc), "SmsCode", "CodeHash", "-"); err == nil {
		t.Fatal("反向对照失效：CodeHash 被改成 json:\"code_hash\" 后检查器竟判合规")
	}
	// 恢复：未变异源码喂同一个检查器必须判合规（绿）。
	if err := checkSmsFieldJSONTag(parseSmsSynthetic(t, src), "SmsCode", "CodeHash", "-"); err != nil {
		t.Fatalf("恢复后同一个检查器仍报错：%v", err)
	}
}

// ---- 护栏 4：日志表不存明文/正文 ----

func TestSmsLogStoresNoPlainText(t *testing.T) {
	src, real := loadSmsReal(t)
	if err := checkSmsLogNoPlainText(real); err != nil {
		t.Fatal(err)
	}
	// 扫描器自检：真实源码里 SmsLog 必须含 ErrMsg，证明 smsStructFields 真读到了字段（不是恒空）。
	if tag, ok := smsStructTag(real, "SmsLog", "ErrMsg"); !ok || !strings.Contains(tag.Get("gorm"), "column:err_msg") {
		t.Fatal("扫描器失效：没读出 SmsLog.ErrMsg 的 gorm tag")
	}

	// 反向对照：合成一份给 SmsLog 加了 Body string 的源码，同一个检查器必须判违规（红）。
	badSrc := strings.Replace(src, "type SmsLog struct {", "type SmsLog struct {\n    Body string", 1)
	if badSrc == src {
		t.Fatal("反向对照锚点失效：没找到 SmsLog 结构体声明")
	}
	if err := checkSmsLogNoPlainText(parseSmsSynthetic(t, badSrc)); err == nil {
		t.Fatal("反向对照失效：加了 Body 字段后检查器竟判合规")
	}
	// 恢复：未变异源码喂同一个检查器必须判合规（绿）。
	if err := checkSmsLogNoPlainText(parseSmsSynthetic(t, src)); err != nil {
		t.Fatalf("恢复后同一个检查器仍报错：%v", err)
	}
}

// TestSmsTableNamesFromGormNamingStrategy 是简报步骤 3「用 HasTable 实测确认表名」
// 在不连真实 DB 前提下的等价落地：用 GORM 自己的 NamingStrategy 实算表名，
// 并用 go/ast 锁定模型没有声明 TableName() 覆盖命名策略。
// 真实 w.DB.Migrator().HasTable("sms_codes") 需要连真实 MySQL，本环境按约束不连真实 DB，
// 故报告里如实标注「⚠️ 无法实测」，此处给出静态实算作为不靠推断的依据。
func TestSmsTableNamesFromGormNamingStrategy(t *testing.T) {
	_, real := loadSmsReal(t)
	ns := schema.NamingStrategy{}
	for _, tc := range []struct{ model, want string }{
		{"SmsCode", "sms_codes"},
		{"SmsLog", "sms_logs"},
	} {
		if got := ns.TableName(tc.model); got != tc.want {
			t.Errorf("GORM 复数化把 %s 映射成 %q，期望 %q", tc.model, got, tc.want)
		}
		if smsDeclaresTableName(real, tc.model) {
			t.Errorf("model.%s 声明了 TableName()：会覆盖 GORM 复数化，表名不再是 %s", tc.model, tc.want)
		}
	}
}
