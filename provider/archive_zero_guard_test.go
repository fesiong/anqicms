package provider

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// ── 零值语义护栏（2026-10-29 指针化整改）─────────────────────────────────
//
// 这批改动解决的问题是同一类静默失效：
//
//	provider 用 `if req.X > 0` / `req.X != ""` 判断「要不要写这个字段」，
//	于是「库存改 0（售罄）」「价格改 0（免费）」「状态改 0（待审核）/ -1（禁用）」
//	根本写不进库，而接口照旧回 ok=true —— 调用方以为改成功，库里其实没变；
//	反过来在 PATCH（AI/MCP 默认语义）下去传一个字段，其余字段被零值覆盖。
//
// 修法是把「未传」与「显式传零值」分开：请求体里这些字段一律用指针。
// 本文件锁住三件事，任何一件被改回去都会立刻变红：
//
//  1. request 结构体里这些字段必须是指针（改回值类型 = 语义退回原点）；
//  2. api_catalog 的 schema 生成必须剥掉指针前缀（否则参数类型退化成 object，
//     调用方无法得知该传数字还是字符串）；
//  3. 真跑一遍 Save*：显式 0 / 负数能落库，未传字段保持原值。

// TestSchemaTypeOfStripsPointer 锁住 schemaTypeOf 对指针类型的处理。
//
// 参数类型是 api_catalog.json 的一部分，AI 靠它决定传 0 还是 "0"。
// 一旦有人删掉 TrimPrefix("*")，`*int64` 会落到 default 分支变成 object，
// 生成物看起来「有类型」，实际全是 object —— 静默降级，不会报错。
func TestSchemaTypeOfStripsPointer(t *testing.T) {
	cases := []struct {
		goType string
		want   string
	}{
		{"string", "string"},
		{"*string", "string"},
		{"int", "integer"},
		{"*int", "integer"},
		{"*int64", "integer"},
		{"*uint", "integer"},
		{"*float64", "number"},
		{"*float32", "number"},
		{"*bool", "boolean"},
		{"*[]string", "array"},
		{"[]uint", "array"},
		{"*[]uint", "array"},
		{"map[string]interface{}", "object"},
		{"*map[string]interface{}", "object"},
	}
	for _, c := range cases {
		if got := schemaTypeOf(c.goType); got != c.want {
			t.Errorf("schemaTypeOf(%q) = %q，期望 %q", c.goType, got, c.want)
		}
	}
}

// TestRequestZeroValueFieldsArePointers 用反射锁住「哪些字段必须是指针」。
//
// 比 ast 扫描源码更直接：只要有人把 `Stock *int64` 改回 `Stock int64`，
// 不用等 provider 里的判断逻辑写错，这里先红。
func TestRequestZeroValueFieldsArePointers(t *testing.T) {
	mustBePointer := []struct {
		typ    any
		fields []string
	}{
		{request.Archive{}, []string{
			"ParentId", "PlaceId", "SeoTitle", "Keywords", "Description", "Template",
			"CanonicalUrl", "FixedLink", "Password", "VideoUrl", "WeightUnit", "LengthUnit",
			"Brand", "CategoryIds", "Images", "Tags", "Flags", "RelationIds", "UserId",
			"Views", "Sort", "KeywordId", "CommentCount", "FavoriteCount", "Price", "Stock",
			"OriginPrice", "CostPrice", "SoldCount", "ReviewCount", "ReadLevel", "OptionType",
			"ProductType", "HasPseudo", "OriginId", "MinQuantity", "MaxQuantity", "StepQuantity",
			"NeedLogistics", "IsFreeShipping", "AllowOversold", "Weight", "Length", "Width",
			"Height", "Volume", "AverageScore", "OrderFields",
		}},
		{request.UserRequest{}, []string{
			"RealName", "FirstName", "LastName", "Birthday", "AvatarURL", "Introduce",
			"Phone", "Email", "GroupId", "Status", "IsRetailer", "ParentId",
			"InviteCode", "ExpireTime",
		}},
		{request.UserGroupRequest{}, []string{
			"Title", "Description", "Level", "Price", "Status", "Setting",
		}},
		{request.Category{}, []string{
			"SeoTitle", "Keywords", "Description", "Content", "Template", "DetailTemplate",
			"Logo", "ParentId", "Sort", "Status", "IsInherit", "Images",
		}},
		{request.ModuleRequest{}, []string{
			"Keywords", "Description", "Status",
		}},
		{request.NavConfig{}, []string{
			"SubTitle", "Description", "ParentId", "NavType", "PageId", "TypeId",
			"Link", "Sort", "Status", "Style", "Logo",
		}},
		{request.PluginTag{}, []string{
			"SeoTitle", "Keywords", "Description", "FirstLetter", "Content", "Logo",
			"Template", "CategoryId", "Status",
		}},
	}
	for _, g := range mustBePointer {
		v := reflect.TypeOf(g.typ)
		for _, name := range g.fields {
			f, ok := v.FieldByName(name)
			if !ok {
				t.Errorf("%s 缺少字段 %s（改了名字就同步改这里）", v.Name(), name)
				continue
			}
			if f.Type.Kind() != reflect.Ptr {
				t.Errorf("%s.%s 必须是 %s 的指针类型（当前 %s）：零值写入与 PATCH 语义都依赖它",
					v.Name(), name, f.Type, f.Type)
			}
		}
	}
}

// TestRequestValueFieldsStayValues 反向护栏：这些字段**不能**被指针化。
//
// 它们的空值没有业务含义（或空值有专门的更强语义），指针化只会让调用方
// 多传一层。尤其 Password —— 空串的语义是「不改密码」，变成指针后
// 很容易被写成「清空密码」，那是账号安全问题。
func TestRequestValueFieldsStayValues(t *testing.T) {
	mustBeValue := []struct {
		typ    any
		fields []string
	}{
		{request.Archive{}, []string{
			"Title", "Content", "UrlToken", "ModuleId", "CategoryId", "CreatedTime",
			"OriginUrl", "OriginTitle",
		}},
		{request.UserRequest{}, []string{"UserName", "Password", "Id"}},
		{request.Category{}, []string{"Title", "ModuleId", "Type", "UrlToken"}},
	}
	for _, g := range mustBeValue {
		v := reflect.TypeOf(g.typ)
		for _, name := range g.fields {
			f, ok := v.FieldByName(name)
			if !ok {
				t.Errorf("%s 缺少字段 %s", v.Name(), name)
				continue
			}
			if f.Type.Kind() == reflect.Ptr {
				t.Errorf("%s.%s 不应是指针（当前 %s）：它的空值没有「显式清空」语义",
					v.Name(), name, f.Type)
			}
		}
	}
}

// newZeroValueTestSite 复用仓里既有的 DB 测试装配方式（见 tag_list_test.go）。
func newZeroValueTestSite(t *testing.T) *Website {
	t.Helper()
	dbSite, err := GetDBWebsiteInfo(1)
	if err != nil {
		t.Skipf("本地无可用站点库，跳过：%v", err)
	}
	InitWebsite(dbSite)
	w := GetWebsite(1)
	if w == nil || w.DB == nil {
		t.Skip("站点未初始化，跳过")
	}
	// 站点跨测试共享，Cache 为 nil 会让其它测试（以及 Agent 调度器 tick）panic。
	w.Cache = library.InitMemoryCache()
	return w
}

// TestSaveArchiveZeroValueRoundTrip 直接验证 P0 的三条语义：
//
//	① 新建时 nil 要落成数据库默认值（不是零值）；
//	② PATCH 未传的字段保持库里的原值（不被零值覆盖）；
//	③ 显式传的 0 必须真的写进去（售罄/免费/清空 SEO 标题）。
func TestSaveArchiveZeroValueRoundTrip(t *testing.T) {
	w := newZeroValueTestSite(t)

	var category model.Category
	if err := w.DB.Where("`module_id` > 0").Order("id asc").Take(&category).Error; err != nil {
		t.Skipf("测试库没有可用分类，跳过：%v", err)
	}

	title := fmt.Sprintf("零值护栏-%d", time.Now().UnixNano())
	created, err := w.SaveArchive(&request.Archive{
		Title:       title,
		CategoryId:  category.Id,
		CategoryIds: library.Ptr([]uint{category.Id}),
		Draft:       true,
	})
	if err != nil {
		t.Fatalf("新建文档失败：%v", err)
	}
	defer func() {
		_ = w.DB.Unscoped().Delete(&model.ArchiveDraft{}, created.Id).Error
		_ = w.DB.Unscoped().Delete(&model.Archive{}, created.Id).Error
	}()

	// ① 新建：nil 必须落成默认值。GORM 的 Save 写全列，DB 的 DEFAULT 不生效，
	//    默认值只能在新增分支里显式给，漏了就是「新商品库存 0 = 直接售罄」。
	if created.Stock != 9999999 {
		t.Errorf("新建商品 stock 默认值应为 9999999，实际 %d", created.Stock)
	}
	if created.ProductType != config.ProductTypeNormal {
		t.Errorf("新建商品 product_type 默认值应为 %d，实际 %d", config.ProductTypeNormal, created.ProductType)
	}

	// ② PATCH 只改标题：其余字段原样保留（含刚补的默认值与被清空的 price）。
	patched, err := w.SaveArchive(&request.Archive{
		Id:    created.Id,
		Title: title + "-改",
	})
	if err != nil {
		t.Fatalf("PATCH 标题失败：%v", err)
	}
	if patched.Title != title+"-改" {
		t.Errorf("PATCH 后标题应为 %q，实际 %q", title+"-改", patched.Title)
	}
	if patched.Stock != created.Stock {
		t.Errorf("PATCH 未传 stock 却变了：%d → %d（零值覆盖）", created.Stock, patched.Stock)
	}
	if patched.ProductType != created.ProductType {
		t.Errorf("PATCH 未传 product_type 却变了：%d → %d", created.ProductType, patched.ProductType)
	}

	// ③ 显式传 0：必须落库。这是「售罄 / 免费 / 清空 SEO 标题」的唯一入口。
	zeroed, err := w.SaveArchive(&request.Archive{
		Id:       created.Id,
		Stock:    library.Ptr(int64(0)),
		Price:    library.Ptr(int64(0)),
		SeoTitle: library.Ptr(""),
	})
	if err != nil {
		t.Fatalf("写入零值失败：%v", err)
	}
	if zeroed.Stock != 0 {
		t.Errorf("显式传 stock=0 应写入 0（售罄），实际 %d", zeroed.Stock)
	}
	if zeroed.Price != 0 {
		t.Errorf("显式传 price=0 应写入 0（免费），实际 %d", zeroed.Price)
	}

	// 再 PATCH 一次，确认 0 不会被下一次「没传」冲掉。
	again, err := w.SaveArchive(&request.Archive{Id: created.Id, Title: title + "-再改"})
	if err != nil {
		t.Fatalf("二次 PATCH 失败：%v", err)
	}
	if again.Stock != 0 {
		t.Errorf("PATCH 未传 stock 时不应改动已有的 0，实际 %d", again.Stock)
	}
}

// TestSaveUserInfoZeroValueRoundTrip 验证会员状态能写 0 与 -1。
//
// 原实现是 `if req.UpdateAll || req.Status > 0`，于是 status=0（待审核）和
// status=-1（禁用）永远写不进去；更糟的是 PATCH 只改用户名时，未传的 status
// 被写成 0，等于「改个名字账号就被踢回待审核」。实测在 2026-10-03 发生过。
func TestSaveUserInfoZeroValueRoundTrip(t *testing.T) {
	w := newZeroValueTestSite(t)

	userName := fmt.Sprintf("zv_user_%d", time.Now().UnixNano())
	phone := fmt.Sprintf("139%08d", time.Now().UnixNano()%100000000)
	created, err := w.SaveUserInfo(&request.UserRequest{
		UserName: userName,
		Phone:    library.Ptr(phone),
		Status:   library.Ptr(1),
	})
	if err != nil {
		t.Fatalf("新建用户失败：%v", err)
	}
	defer func() { _ = w.DB.Unscoped().Delete(&model.User{}, created.Id).Error }()

	if created.Status != 1 {
		t.Fatalf("新建用户 status 应为 1，实际 %d", created.Status)
	}

	// PATCH 只改真实姓名：status / phone 必须原样保留。
	patched, err := w.SaveUserInfo(&request.UserRequest{
		Id:       created.Id,
		RealName: library.Ptr("零值测试"),
	})
	if err != nil {
		t.Fatalf("PATCH 用户失败：%v", err)
	}
	if patched.RealName != "零值测试" {
		t.Errorf("PATCH 后 real_name 应为「零值测试」，实际 %q", patched.RealName)
	}
	if patched.Status != 1 {
		t.Errorf("PATCH 未传 status 应保持 1（正常），实际 %d（账号被静默禁用/待审核）", patched.Status)
	}
	if patched.Phone != phone {
		t.Errorf("PATCH 未传 phone 应保持 %q，实际 %q（手机号被清空）", phone, patched.Phone)
	}

	// 显式 -1（禁用）：tinyint 有符号，原来被 `> 0` 挡在门外。
	disabled, err := w.SaveUserInfo(&request.UserRequest{Id: created.Id, Status: library.Ptr(-1)})
	if err != nil {
		t.Fatalf("写入 status=-1 失败：%v", err)
	}
	if disabled.Status != -1 {
		t.Errorf("显式传 status=-1 应写入 -1（禁用），实际 %d", disabled.Status)
	}

	// 显式 0（待审核）。
	review, err := w.SaveUserInfo(&request.UserRequest{Id: created.Id, Status: library.Ptr(0)})
	if err != nil {
		t.Fatalf("写入 status=0 失败：%v", err)
	}
	if review.Status != 0 {
		t.Errorf("显式传 status=0 应写入 0（待审核），实际 %d", review.Status)
	}
}

// TestSaveUserGroupInfoKeepsUnpassedFields 验证用户组的「改一个字段不清空其余」。
//
// 原实现是「用请求字段拼一个新对象再 Save」，Save 写全列，所以改个名字就会把
// Level/Price/Setting 清成零值，Status 还被硬编码成 1 —— 停用的用户组
// 只要保存一次就会被强制启用。
func TestSaveUserGroupInfoKeepsUnpassedFields(t *testing.T) {
	w := newZeroValueTestSite(t)

	title := fmt.Sprintf("零值用户组-%d", time.Now().UnixNano())
	created, err := w.SaveUserGroupInfo(&request.UserGroupRequest{
		Title:  library.Ptr(title),
		Level:  library.Ptr(3),
		Price:  library.Ptr(int64(9900)),
		Status: library.Ptr(1),
	})
	if err != nil {
		t.Fatalf("新建用户组失败：%v", err)
	}
	defer func() { _ = w.DB.Unscoped().Delete(&model.UserGroup{}, created.Id).Error }()

	if created.Level != 3 || created.Price != 9900 {
		t.Fatalf("新建用户组字段未落库：level=%d price=%d", created.Level, created.Price)
	}

	// 只改名字：level / price / status 必须原样保留。
	renamed, err := w.SaveUserGroupInfo(&request.UserGroupRequest{
		Id:    created.Id,
		Title: library.Ptr(title + "-改"),
	})
	if err != nil {
		t.Fatalf("PATCH 用户组失败：%v", err)
	}
	if renamed.Title != title+"-改" {
		t.Errorf("title 应为 %q，实际 %q", title+"-改", renamed.Title)
	}
	if renamed.Level != 3 {
		t.Errorf("未传 level 却被清空：期望 3，实际 %d", renamed.Level)
	}
	if renamed.Price != 9900 {
		t.Errorf("未传 price 却被清空：期望 9900，实际 %d", renamed.Price)
	}
	if renamed.Status != 1 {
		t.Errorf("未传 status 却被改写：期望 1，实际 %d", renamed.Status)
	}

	// 显式 0：停用必须能写进去（原来恒为 1）。
	stopped, err := w.SaveUserGroupInfo(&request.UserGroupRequest{Id: created.Id, Status: library.Ptr(0)})
	if err != nil {
		t.Fatalf("停用用户组失败：%v", err)
	}
	if stopped.Status != 0 {
		t.Errorf("显式传 status=0 应停用（0），实际 %d", stopped.Status)
	}
}

// TestZeroValueGuardFilesMentioned 防「注释承诺了护栏但文件被删」。
//
// provider/archive.go 与 request/archive.go 的注释都指名引用了本文件；
// 文件名改了或文件被删，说明护栏体系被拆了一角，这里主动报错。
func TestZeroValueGuardFilesMentioned(t *testing.T) {
	for _, name := range []string{"archive.go", "../request/archive.go"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读取 %s 失败：%v", name, err)
		}
		if !strings.Contains(string(body), "archive_zero_guard_test.go") {
			t.Errorf("%s 应保留指向 archive_zero_guard_test.go 的护栏注释", name)
		}
	}
}
