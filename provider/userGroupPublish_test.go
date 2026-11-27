package provider

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"kandaoni.com/anqicms/request"
)

// ── 用户组「可投稿栏目」授权的护栏 ───────────────────────────────────────
//
// 这条权限链有三个环节，任何一环单独失效都不会报错，只会「静默放行」：
//
//	① 关系表读出来是空的   → 如果判定逻辑把「空授权」当成「不限」，等于全放行；
//	② 发布入口绕过判定     → 只要有人把 ApiArchivePublish 改回直接调 SaveArchive，
//	                        授权配了也没人看；
//	③ 关系表根本没建       → 读取永远返回空，和 ① 叠加就是全放行。
//
// 下面四个测试分别钉住这三点（②③ 用 AST 断言），并全部带反向对照。

// TestDeniedPublishCategories 锁住判定本身：授权为空 = 全部拒绝（fail closed）。
func TestDeniedPublishCategories(t *testing.T) {
	cases := []struct {
		name      string
		allowed   []uint
		requested []uint
		want      []uint
	}{
		{
			name:      "未配授权时任何栏目都被拒（fail closed）",
			allowed:   nil,
			requested: []uint{3},
			want:      []uint{3},
		},
		{
			name:      "授权为空切片时同样被拒",
			allowed:   []uint{},
			requested: []uint{3, 5},
			want:      []uint{3, 5},
		},
		{
			name:      "全部命中授权",
			allowed:   []uint{1, 3, 5},
			requested: []uint{3},
			want:      nil,
		},
		{
			name:      "多分类里只要有一个没授权就整单拒绝",
			allowed:   []uint{1, 3},
			requested: []uint{3, 9},
			want:      []uint{9},
		},
		{
			name:      "子栏目不会因为父栏目被授权而放行",
			allowed:   []uint{1},
			requested: []uint{2},
			want:      []uint{2},
		},
		{
			name:      "没指定栏目时不判定（交给 SaveArchive 报请选择栏目）",
			allowed:   []uint{1},
			requested: nil,
			want:      nil,
		},
		{
			name:      "重复的未授权 ID 只报一次",
			allowed:   []uint{1},
			requested: []uint{7, 7},
			want:      []uint{7},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DeniedPublishCategories(c.allowed, c.requested)
			if len(got) != len(c.want) {
				t.Fatalf("DeniedPublishCategories(%v, %v) = %v, want %v", c.allowed, c.requested, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("DeniedPublishCategories(%v, %v) = %v, want %v", c.allowed, c.requested, got, c.want)
				}
			}
		})
	}

	// 反向对照：把「空授权」改成放行（也就是最常见的错误实现），第一、二个用例必须变红。
	if len(DeniedPublishCategories(nil, []uint{3})) == 0 {
		t.Fatal("反向对照失败：空授权被当成放行 —— 权限判定已经退化成不限栏目")
	}
}

// TestNormalizePublishCategoryIds 锁住授权列表归一化（去 0、去重、升序）。
// 不归一化会让唯一键 uk_group_category 在重复提交时报错，保存直接失败。
func TestNormalizePublishCategoryIds(t *testing.T) {
	cases := []struct {
		in   []uint
		want []uint
	}{
		{nil, []uint{}},
		{[]uint{5, 3, 5, 0, 3, 1}, []uint{1, 3, 5}},
		{[]uint{2}, []uint{2}},
	}
	for _, c := range cases {
		if got := NormalizePublishCategoryIds(c.in); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("NormalizePublishCategoryIds(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestCollectPublishCategoryIds 锁住「请求到底要落到哪些栏目」的取法。
// 必须与 SaveArchive 的多分类归一保持一致，否则校验的是 A 集合、写库的是 B 集合。
func TestCollectPublishCategoryIds(t *testing.T) {
	// category_ids 优先于 category_id（与 SaveArchive 一致）
	multi := []uint{2, 4}
	req := &request.Archive{CategoryId: 2, CategoryIds: &multi}
	if got := CollectPublishCategoryIds(req); !reflect.DeepEqual(got, []uint{2, 4}) {
		t.Fatalf("多分类取值错误：%v", got)
	}

	// 只传 category_id
	req = &request.Archive{CategoryId: 7}
	if got := CollectPublishCategoryIds(req); !reflect.DeepEqual(got, []uint{7}) {
		t.Fatalf("单分类取值错误：%v", got)
	}

	// 显式空数组不能回落到 category_id：SaveArchive 里这两者的优先级也是如此
	empty := []uint{}
	req = &request.Archive{CategoryId: 7, CategoryIds: &empty}
	if got := CollectPublishCategoryIds(req); len(got) != 0 {
		t.Fatalf("显式空数组被回落成了 %v", got)
	}
}

// TestPublishEndpointGoesThroughPermissionGate 锁住「发布接口必须走统一入口」。
//
// controller.ApiArchivePublish 一旦改回直接调 currentSite.SaveArchive，
// 栏目授权就再也拦不住任何人 —— 而接口照样返回「发布成功」，
// 运营侧只会以为权限配置不生效。这个断言让那次改动立刻变红。
func TestPublishEndpointGoesThroughPermissionGate(t *testing.T) {
	fn := mustFindFunc(t, "../controller/apiTag.go", "ApiArchivePublish")

	calls := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		calls[selector.Sel.Name] = true
		return true
	})

	// 反向对照：连 SaveArchive 都没解析到，说明取调用点的方式失效了（而不是真的漏了校验）。
	if !calls["SaveArchive"] && !calls["PublishArchiveAsUser"] {
		t.Fatal("反向对照失败：ApiArchivePublish 里没解析到任何发布调用，断言已空转")
	}
	if !calls["PublishArchiveAsUser"] {
		t.Fatal("ApiArchivePublish 必须走 PublishArchiveAsUser（内含可投稿栏目校验），不得直接 SaveArchive")
	}
	if calls["SaveArchive"] {
		t.Fatal("ApiArchivePublish 不得直接调 SaveArchive —— 那会绕过栏目授权")
	}
}

// TestInitModelDataEnsuresUserGroupCategoryTable 锁住关系表的幂等建表。
//
// AutoMigrateDB 只在 last_run_version != Version 时跑，漏掉这行的话
// 老站升级后表根本不存在，读授权永远返回空 —— 表现为「配了权限也没用」。
func TestInitModelDataEnsuresUserGroupCategoryTable(t *testing.T) {
	fn := mustFindFunc(t, "dao.go", "InitModelData")

	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
			if selector.Sel.Name == "ensureUserGroupCategoryTable" {
				found = true
			}
		}
		return true
	})
	if !found {
		t.Fatal("InitModelData 必须调用 ensureUserGroupCategoryTable，否则老站缺表且无报错")
	}

	// 反向对照：同一个函数体里必须还能解析到别的调用，否则是遍历逻辑坏了。
	hasOtherCall := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name != "ensureUserGroupCategoryTable" {
				hasOtherCall = true
			}
		}
		return true
	})
	if !hasOtherCall {
		t.Fatal("反向对照失败：InitModelData 里除了目标调用什么都没解析到，断言已空转")
	}
}

// TestUserGroupCategoryRegisteredForMigration 保证关系表进自动迁移清单
// （新装机时由 AutoMigrateDB 建表，与 ensureUserGroupCategoryTable 互为兜底）。
func TestUserGroupCategoryRegisteredForMigration(t *testing.T) {
	models := autoMigrateModels(t)
	// 反向对照：一直都在的 UserGroup 都解析不到，说明提取逻辑失效。
	if !models["UserGroup"] {
		t.Fatal("反向对照失败：UserGroup 未在解析结果中，清单提取逻辑失效")
	}
	if !models["UserGroupCategory"] {
		t.Fatal("UserGroupCategory 未注册到 AutoMigrateDB 的 models 清单")
	}
}

// mustFindFunc 在指定源文件里取顶层函数声明。
func mustFindFunc(t *testing.T, file, name string) *ast.FuncDecl {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", file, err)
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if ok && fd.Name.Name == name {
			if fd.Body == nil {
				t.Fatalf("%s 中 %s 没有函数体", file, name)
			}
			return fd
		}
	}
	t.Fatalf("%s 中找不到函数 %s", file, name)

	return nil
}
