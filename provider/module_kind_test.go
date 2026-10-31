package provider

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// ── 模型类型（kind）护栏 ────────────────────────────────────────────────
//
// 「这个模型是不是产品」原本靠 `ModuleId == 2` 硬编码，散在 5 处：
// controller/api.go（浏览量归类与商品浏览事件）、provider/apiService.go
// （SKU 筛选器）、provider/jsonLd.go（Product vs Article 结构化数据）、
// provider/archive.go（Google Merchant 推送）、provider/statistics.go（新增产品数）。
//
// 模块 id 是自增的，删了重建、导入站点后未必还是 2；把 1 号模型改名成
// 「产品」也不会生效。判断错位的症状是**静默**的：价格区块不渲染、
// Product 结构化数据变成 Article、商品不推送去 Merchant——都不报错。
//
// 本文件锁住三件事：kind 列存在且历史数据已回填、判断跟着 kind 走而不是
// 跟着 id 走、保存端点能把 kind 写进去。

// TestModuleKindColumnExistsAndBackfilled 校验迁移真的补齐了列与数据。
//
// 为什么不能只靠 AutoMigrateDB：它只在 last_run_version != config.Version
// 时执行（升级才跑），新增列在老站库上不会自动出现，而 kind 是每次读
// 模块都要用的字段——缺列会让所有模块查询直接 Unknown column。
func TestModuleKindColumnExistsAndBackfilled(t *testing.T) {
	w := newZeroValueTestSite(t)

	if err := w.ensureModuleKindColumn(); err != nil {
		t.Fatalf("补齐 modules.kind 失败：%v", err)
	}
	if !w.DB.Migrator().HasColumn(&model.Module{}, "kind") {
		t.Fatalf("modules 表没有 kind 列 —— 老库升级后所有模块查询都会 Unknown column")
	}

	var article model.Module
	if err := w.DB.Where("`id` = ?", 1).Take(&article).Error; err != nil {
		t.Skipf("测试库没有默认文章模型，跳过：%v", err)
	}
	if article.Kind != model.ModuleKindArticle {
		t.Errorf("id=1 的模型应回填为 %q，实际 %q", model.ModuleKindArticle, article.Kind)
	}

	var product model.Module
	if err := w.DB.Where("`id` = ?", 2).Take(&product).Error; err != nil {
		t.Skipf("测试库没有默认产品模型，跳过：%v", err)
	}
	if product.Kind != model.ModuleKindProduct {
		t.Errorf("table_name=product 的模型应回填为 %q，实际 %q（回填不出来商品页就按文章渲染）",
			model.ModuleKindProduct, product.Kind)
	}

	// 幂等：再跑一次不能把用户改过的值刷回去。
	if err := w.ensureModuleKindColumn(); err != nil {
		t.Fatalf("重复执行应为幂等，实际报错：%v", err)
	}
}

// TestIsProductModuleFollowsKindNotId 是这一批的核心断言。
//
// 造一个 **id 不是 2** 的模型，kind=product，它必须被认成产品；
// 反过来把 kind 改成 article，就不再是产品。旧实现（id==2）两条都过不了。
func TestIsProductModuleFollowsKindNotId(t *testing.T) {
	w := newZeroValueTestSite(t)
	if err := w.ensureModuleKindColumn(); err != nil {
		t.Skipf("kind 列不可用，跳过：%v", err)
	}

	suffix := time.Now().UnixNano()
	probe := model.Module{
		TableName: fmt.Sprintf("zz_kind_probe_%d", suffix),
		Name:      fmt.Sprintf("kind探针%d", suffix),
		Title:     "kind 探针",
		UrlToken:  fmt.Sprintf("kind-probe-%d", suffix),
		Kind:      model.ModuleKindProduct,
		Status:    1,
	}
	if err := w.DB.Create(&probe).Error; err != nil {
		t.Skipf("测试库无法写入 modules，跳过：%v", err)
	}
	defer func() {
		w.DeleteCacheModules()
		_ = w.DB.Unscoped().Delete(&model.Module{}, probe.Id).Error
		w.DeleteCacheModules()
	}()
	w.DeleteCacheModules()

	if probe.Id == 2 {
		t.Fatalf("临时模型意外拿到了 id=2，用例无法区分「按 id 判断」与「按 kind 判断」")
	}
	if !w.IsProductModule(probe.Id) {
		t.Errorf("id=%d 的模型 kind=product，应被识别为产品（旧实现按 id==2 判断会漏掉它）", probe.Id)
	}

	// 反向：把 kind 改成 article，就不再是产品。
	if err := w.DB.Model(&model.Module{}).Where("`id` = ?", probe.Id).
		UpdateColumn("kind", model.ModuleKindArticle).Error; err != nil {
		t.Fatalf("改 kind 失败：%v", err)
	}
	w.DeleteCacheModules()
	if w.IsProductModule(probe.Id) {
		t.Errorf("kind 改成 article 后仍被识别为产品 —— 判断源仍是 id 或缓存没失效")
	}
}

// TestIsProductModuleIgnoresDisabledModule 模型被停用不影响类型判断。
//
// GetModuleFromCache 只返回 status=1 的模型；若 IsProductModule 直接复用它，
// 产品模块一停用就会退化成「文章」，前台价格区块与 Product 结构化数据一起消失
// ——这正是旧硬编码没有的退化路径，不能靠改类型判断重新引入。
func TestIsProductModuleIgnoresDisabledModule(t *testing.T) {
	w := newZeroValueTestSite(t)
	if err := w.ensureModuleKindColumn(); err != nil {
		t.Skipf("kind 列不可用，跳过：%v", err)
	}

	suffix := time.Now().UnixNano()
	probe := model.Module{
		TableName: fmt.Sprintf("zz_kind_off_%d", suffix),
		Name:      fmt.Sprintf("kind停用探针%d", suffix),
		Title:     "kind 停用探针",
		UrlToken:  fmt.Sprintf("kind-off-%d", suffix),
		Kind:      model.ModuleKindProduct,
		Status:    0, // 停用
	}
	if err := w.DB.Create(&probe).Error; err != nil {
		t.Skipf("测试库无法写入 modules，跳过：%v", err)
	}
	defer func() {
		w.DeleteCacheModules()
		_ = w.DB.Unscoped().Delete(&model.Module{}, probe.Id).Error
		w.DeleteCacheModules()
	}()
	w.DeleteCacheModules()

	if !w.IsProductModule(probe.Id) {
		t.Errorf("停用的产品模型（id=%d）仍应被识别为产品：判断类型不该受 status 影响", probe.Id)
	}
}

// TestSaveModuleKindPersists 模型保存能把 kind 写进库。
//
// 前端模型编辑页新增了「模型类型」选择，存不进去等于白做——而且失败是静默的
// （回执 ok，列表里还是旧类型）。
func TestSaveModuleKindPersists(t *testing.T) {
	w := newZeroValueTestSite(t)
	if err := w.ensureModuleKindColumn(); err != nil {
		t.Skipf("kind 列不可用，跳过：%v", err)
	}

	suffix := time.Now().UnixNano()
	tableName := fmt.Sprintf("zz_kind_save_%d", suffix)
	defer func() {
		w.DeleteCacheModules()
		_ = w.DB.Exec("DROP TABLE IF EXISTS `" + tableName + "`").Error
		_ = os.RemoveAll(w.GetTemplateDir() + "/" + tableName)
		w.DeleteCacheModules()
	}()

	created, err := w.SaveModule(&request.ModuleRequest{
		TableName: tableName,
		UrlToken:  fmt.Sprintf("kind-save-%d", suffix),
		Title:     "kind 保存探针",
		Name:      fmt.Sprintf("kindSave%d", suffix),
		Kind:      library.Ptr(model.ModuleKindProduct),
	})
	if err != nil {
		t.Skipf("测试库无法创建模型（会建扩展表），跳过：%v", err)
	}
	defer func() {
		_ = w.DB.Unscoped().Delete(&model.Module{}, created.Id).Error
	}()

	var saved model.Module
	if err := w.DB.Where("`id` = ?", created.Id).Take(&saved).Error; err != nil {
		t.Fatalf("重查模型失败：%v", err)
	}
	if saved.Kind != model.ModuleKindProduct {
		t.Errorf("新建模型的 kind 应为 %q，实际 %q（前端选了类型却没存下来）", model.ModuleKindProduct, saved.Kind)
	}

	// PATCH 不传 kind 时保持原值，传 article 才改。
	if _, err := w.SaveModule(&request.ModuleRequest{
		Id: created.Id, Title: "改名", Partial: true,
	}); err != nil {
		t.Fatalf("PATCH 模型失败：%v", err)
	}
	var patched model.Module
	if err := w.DB.Where("`id` = ?", created.Id).Take(&patched).Error; err != nil {
		t.Fatalf("重查模型失败：%v", err)
	}
	if patched.Kind != model.ModuleKindProduct {
		t.Errorf("PATCH 未传 kind 却把类型改成 %q（模型会从产品退化成文章）", patched.Kind)
	}
}

// TestModuleRequestKindIsPointer 锁住 Kind 的指针性：值类型分不清
// 「没传」与「传空串」，而空串是非法的模型类型。
func TestModuleRequestKindIsPointer(t *testing.T) {
	f, ok := reflect.TypeOf(request.ModuleRequest{}).FieldByName("Kind")
	if !ok {
		t.Fatal("ModuleRequest 缺 Kind 字段")
	}
	if f.Type.Kind() != reflect.Ptr {
		t.Errorf("ModuleRequest.Kind 必须是指针（nil=不改），实际 %s", f.Type)
	}
}
