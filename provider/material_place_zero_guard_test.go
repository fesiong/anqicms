package provider

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// ── 素材 / 城市站零值语义护栏（2026-10-29 指针化整改第三批）───────────────
//
// 这两个端点是「逐字段无条件赋值」的最后两处，问题比前两批更杂：
//
//  1. SaveMaterial 里 `material.Status = 1` 硬编码 —— 请求明明带了 status
//     字段，但被忽略，素材**永远无法禁用**（`GetMaterialCategories` 只取
//     status=1，等于停用素材这件事无法表达）。
//  2. SaveMaterial 先算 `Md5(material.Content)` 再覆盖 content —— md5 存的
//     是**上一次**内容的摘要，因此「按 md5 去重 / 判断内容是否变过」全部
//     错位。这是本文件里唯一与零值无关的真 bug，但改同一段代码时一起修。
//  3. SavePlace 同样逐字段赋值 —— 改个标题就把经纬度、时区、logo、SEO、
//     排序、状态全清成零值；且 url_token 在未传时用 `req.Title` 重新生成，
//     而 PATCH 的 req.Title 是空串 → 城市站 URL 被改成空 / 拼音残片，
//     等于换了一条链接（旧链接 404）。
//  4. SavePlace 新建时先给 Status=1，紧接着被 `place.Status = req.Status`
//     覆盖成 0 —— 新建的城市站在前台**默认是停用的**，后台看得见、用户
//     打不开，且不报错。
//
// 本文件锁住 request 结构体的指针性，以及真跑 Save* 后的三条语义：
// 未传保持原值、显式 0 能落库、新建落到正确的默认值。

// TestSaveMaterialKeepsUnpassedFields 复现「改标题把正文/分类/状态清空」。
func TestSaveMaterialKeepsUnpassedFields(t *testing.T) {
	w := newZeroValueTestSite(t)
	suffix := time.Now().UnixNano()
	title := fmt.Sprintf("零值素材-%d", suffix)
	content := fmt.Sprintf("素材原始正文-%d", suffix)

	created, err := w.SaveMaterial(&request.PluginMaterial{
		Title:      library.Ptr(title),
		CategoryId: library.Ptr(uint(3)),
		Content:    library.Ptr(content),
		Status:     library.Ptr(uint(1)),
		AutoUpdate: library.Ptr(uint(1)),
	})
	if err != nil {
		t.Skipf("测试库无 materials 表或写入失败，跳过：%v", err)
	}
	defer func() {
		_ = w.DB.Unscoped().Delete(&model.Material{}, created.Id).Error
	}()

	if created.Content != content || created.CategoryId != 3 || created.Status != 1 {
		t.Fatalf("新建素材字段未落库：content=%q category_id=%d status=%d", created.Content, created.CategoryId, created.Status)
	}

	// 只改标题（PATCH：AI/MCP 通道的默认形态）。
	if _, err = w.SaveMaterial(&request.PluginMaterial{
		Id:      created.Id,
		Title:   library.Ptr(title + "-改"),
		Partial: true,
	}); err != nil {
		t.Fatalf("PATCH 素材失败：%v", err)
	}

	var patched model.Material
	if err := w.DB.Where("`id` = ?", created.Id).Take(&patched).Error; err != nil {
		t.Fatalf("重查素材失败：%v", err)
	}
	if patched.Title != title+"-改" {
		t.Errorf("title 应为 %q，实际 %q", title+"-改", patched.Title)
	}
	if patched.Content != content {
		t.Errorf("未传 content 却被清空：期望 %q，实际 %q（正文丢失，引用它的文档下次自动更新时会变成空白）", content, patched.Content)
	}
	if patched.CategoryId != 3 {
		t.Errorf("未传 category_id 却被改写：期望 3，实际 %d（素材从分类里消失）", patched.CategoryId)
	}
	if patched.Status != 1 {
		t.Errorf("未传 status 却被改写：期望 1（启用），实际 %d", patched.Status)
	}
	if patched.AutoUpdate != 1 {
		t.Errorf("未传 auto_update 却被改写：期望 1，实际 %d（自动同步静默关闭）", patched.AutoUpdate)
	}
}

// TestSaveMaterialStatusCanBeDisabled 复现「素材永远无法禁用」。
//
// 原实现无条件 `material.Status = 1`，请求里的 status 被完全忽略。
func TestSaveMaterialStatusCanBeDisabled(t *testing.T) {
	w := newZeroValueTestSite(t)
	suffix := time.Now().UnixNano()

	created, err := w.SaveMaterial(&request.PluginMaterial{
		Title:   library.Ptr(fmt.Sprintf("待停用素材-%d", suffix)),
		Content: library.Ptr(fmt.Sprintf("正文-%d", suffix)),
	})
	if err != nil {
		t.Skipf("测试库无 materials 表或写入失败，跳过：%v", err)
	}
	defer func() {
		_ = w.DB.Unscoped().Delete(&model.Material{}, created.Id).Error
	}()

	if _, err = w.SaveMaterial(&request.PluginMaterial{
		Id:      created.Id,
		Status:  library.Ptr(uint(0)),
		Partial: true,
	}); err != nil {
		t.Fatalf("停用素材失败：%v", err)
	}

	var patched model.Material
	if err := w.DB.Where("`id` = ?", created.Id).Take(&patched).Error; err != nil {
		t.Fatalf("重查素材失败：%v", err)
	}
	if patched.Status != 0 {
		t.Errorf("显式传 status=0 应写入 0（禁用），实际 %d —— 原实现硬编码 Status=1，素材永远无法禁用", patched.Status)
	}
}

// TestSaveMaterialMd5FollowsNewContent 复现「md5 存的是上一次内容的摘要」。
//
// 原顺序：`md5Str := library.Md5(material.Content)`（此时还是旧内容，新建时为 ""），
// 之后才 `material.Content = req.Content`。所以每条素材的 md5 都滞后一次，
// 新建素材的 md5 恒为 md5("") = d41d8cd98f00b204e9800998ecf8427e。
func TestSaveMaterialMd5FollowsNewContent(t *testing.T) {
	w := newZeroValueTestSite(t)
	suffix := time.Now().UnixNano()
	first := fmt.Sprintf("第一版正文-%d", suffix)
	second := fmt.Sprintf("第二版正文-%d", suffix)

	created, err := w.SaveMaterial(&request.PluginMaterial{
		Title:   library.Ptr(fmt.Sprintf("md5素材-%d", suffix)),
		Content: library.Ptr(first),
	})
	if err != nil {
		t.Skipf("测试库无 materials 表或写入失败，跳过：%v", err)
	}
	defer func() {
		_ = w.DB.Unscoped().Delete(&model.Material{}, created.Id).Error
	}()

	if created.Md5 != library.Md5(first) {
		t.Errorf("新建素材的 md5 应是当前内容的摘要：期望 %s（md5 of %q），实际 %s（若为 md5(\"\") 说明取的是旧值）",
			library.Md5(first), first, created.Md5)
	}

	if _, err = w.SaveMaterial(&request.PluginMaterial{
		Id:      created.Id,
		Content: library.Ptr(second),
		Partial: true,
	}); err != nil {
		t.Fatalf("PATCH 素材内容失败：%v", err)
	}

	var patched model.Material
	if err := w.DB.Where("`id` = ?", created.Id).Take(&patched).Error; err != nil {
		t.Fatalf("重查素材失败：%v", err)
	}
	if patched.Content != second {
		t.Fatalf("content 应为 %q，实际 %q", second, patched.Content)
	}
	if patched.Md5 != library.Md5(second) {
		t.Errorf("md5 应跟随新内容：期望 %s，实际 %s（滞后一次会让按 md5 的去重与「内容是否变过」全部错位）",
			library.Md5(second), patched.Md5)
	}
}

// TestSaveMaterialCategoryStatusCanBeDisabled 复现「素材分类永远无法停用」。
func TestSaveMaterialCategoryStatusCanBeDisabled(t *testing.T) {
	w := newZeroValueTestSite(t)
	suffix := time.Now().UnixNano()

	category, err := w.SaveMaterialCategory(&request.PluginMaterialCategory{
		Title: library.Ptr(fmt.Sprintf("待停用素材分类-%d", suffix)),
	})
	if err != nil {
		t.Skipf("测试库无 material_categories 表或写入失败，跳过：%v", err)
	}
	defer func() {
		_ = w.DB.Unscoped().Delete(&model.MaterialCategory{}, category.Id).Error
	}()

	if category.Status != 1 {
		t.Errorf("新建素材分类默认应启用（status=1），实际 %d", category.Status)
	}

	if _, err = w.SaveMaterialCategory(&request.PluginMaterialCategory{
		Id:      category.Id,
		Status:  library.Ptr(uint(0)),
		Partial: true,
	}); err != nil {
		t.Fatalf("停用素材分类失败：%v", err)
	}

	var patched model.MaterialCategory
	if err := w.DB.Where("`id` = ?", category.Id).Take(&patched).Error; err != nil {
		t.Fatalf("重查素材分类失败：%v", err)
	}
	if patched.Status != 0 {
		t.Errorf("显式传 status=0 应写入 0，实际 %d —— 原实现硬编码 Status=1，分类永远无法停用", patched.Status)
	}
	if patched.Title != category.Title {
		t.Errorf("未传 title 却被清空：期望 %q，实际 %q", category.Title, patched.Title)
	}
}

// TestSavePlaceKeepsUnpassedFields 复现「改城市站标题清空其余全部字段」。
//
// 这里最刺眼的是 url_token：PATCH 未传 title 时它是空串，而 SavePlace 会
// 用 `GetPinyin(req.Title)` 重新生成 —— 城市站 URL 被改掉，旧链接 404。
func TestSavePlaceKeepsUnpassedFields(t *testing.T) {
	w := newZeroValueTestSite(t)
	suffix := time.Now().UnixNano()
	title := fmt.Sprintf("零值城市-%d", suffix)
	content := fmt.Sprintf("城市正文-%d", suffix)

	created, err := w.SavePlace(&request.PlaceRequest{
		Title:       library.Ptr(title),
		SeoTitle:    library.Ptr(title + "SEO"),
		Keywords:    library.Ptr("关键词A,关键词B"),
		Description: library.Ptr("这是城市描述"),
		Content:     library.Ptr(content),
		Sort:        library.Ptr(uint(5)),
		Latitude:    library.Ptr(31.230416),
		Longitude:   library.Ptr(121.473701),
		Timezone:    library.Ptr("Asia/Shanghai"),
		Logo:        library.Ptr("/uploads/place-logo.png"),
		Status:      library.Ptr(uint(1)),
	})
	if err != nil {
		t.Skipf("测试库无 places 表或写入失败，跳过：%v", err)
	}
	defer func() {
		_ = w.DB.Unscoped().Delete(&model.Place{}, created.Id).Error
	}()

	if created.Status != 1 || created.Sort != 5 {
		t.Fatalf("新建城市站字段未落库：status=%d sort=%d", created.Status, created.Sort)
	}
	originToken := created.UrlToken
	if originToken == "" {
		t.Fatalf("新建城市站的 url_token 不应为空")
	}

	// 只改标题。
	if _, err = w.SavePlace(&request.PlaceRequest{
		Id:      created.Id,
		Title:   library.Ptr(title + "-改"),
		Partial: true,
	}); err != nil {
		t.Fatalf("PATCH 城市站失败：%v", err)
	}

	var patched model.Place
	if err := w.DB.Where("`id` = ?", created.Id).Take(&patched).Error; err != nil {
		t.Fatalf("重查城市站失败：%v", err)
	}
	if patched.Title != title+"-改" {
		t.Errorf("title 应为 %q，实际 %q", title+"-改", patched.Title)
	}
	checks := []struct {
		name string
		got  any
		want any
		why  string
	}{
		{"seo_title", patched.SeoTitle, title + "SEO", "SEO 标题被清空"},
		{"keywords", patched.Keywords, "关键词A,关键词B", "关键词被清空"},
		{"description", patched.Description, "这是城市描述", "描述被清空"},
		{"content", patched.Content, content, "正文被清空"},
		{"sort", patched.Sort, uint(5), "排序被归零"},
		{"latitude", patched.Latitude, 31.230416, "纬度被清空（地图定位失效）"},
		{"longitude", patched.Longitude, 121.473701, "经度被清空（地图定位失效）"},
		{"timezone", patched.Timezone, "Asia/Shanghai", "时区被清空"},
		{"logo", patched.Logo, "/uploads/place-logo.png", "logo 被清空"},
		{"status", patched.Status, uint(1), "状态被停用（城市站在前台直接不显示）"},
		{"url_token", patched.UrlToken, originToken, "URL 别名被重新生成（旧链接 404）"},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("未传 %s 却被改写：期望 %v，实际 %v —— %s", c.name, c.want, c.got, c.why)
		}
	}
	if strings.TrimSpace(patched.UrlToken) == "" {
		t.Errorf("url_token 变成空串：城市站的链接直接失效")
	}
}

// TestSavePlaceNewDefaultsEnabled 复现「新建城市站默认停用」。
//
// 原实现先给 `place = &model.Place{Status: 1}`，紧接着
// `place.Status = req.Status`（未传时为 0）把它覆盖掉。
func TestSavePlaceNewDefaultsEnabled(t *testing.T) {
	w := newZeroValueTestSite(t)
	suffix := time.Now().UnixNano()

	created, err := w.SavePlace(&request.PlaceRequest{
		Title:   library.Ptr(fmt.Sprintf("默认状态城市-%d", suffix)),
		Content: library.Ptr("正文"),
	})
	if err != nil {
		t.Skipf("测试库无 places 表或写入失败，跳过：%v", err)
	}
	defer func() {
		_ = w.DB.Unscoped().Delete(&model.Place{}, created.Id).Error
	}()

	if created.Status != 1 {
		t.Errorf("新建城市站未传 status 时应默认启用（1），实际 %d —— 后台建好了、前台打不开，且不报错", created.Status)
	}
}

// TestMaterialAndPlaceRequestFieldsArePointers 锁住请求结构体的指针性。
//
// 这几个字段必须是指针，否则「未传」与「传零值」无法区分；
// Id 必须保持值类型（0 = 新建）。
func TestMaterialAndPlaceRequestFieldsArePointers(t *testing.T) {
	cases := []struct {
		value    any
		mustPtr  []string
		mustKeep []string
	}{
		{
			request.PluginMaterial{},
			[]string{"Title", "CategoryId", "Content", "Status", "AutoUpdate"},
			[]string{"Id"},
		},
		{
			request.PluginMaterialCategory{},
			[]string{"Title", "Status"},
			[]string{"Id"},
		},
		{
			request.PlaceRequest{},
			[]string{
				"Title", "SeoTitle", "Keywords", "Description", "Content", "UrlToken",
				"ParentId", "Sort", "Template", "IsInherit", "Images", "Logo",
				"Latitude", "Longitude", "Timezone", "Status",
			},
			[]string{"Id", "Extra"},
		},
	}
	for _, c := range cases {
		typ := reflect.TypeOf(c.value)
		for _, name := range c.mustPtr {
			f, ok := typ.FieldByName(name)
			if !ok {
				t.Errorf("%s 缺少字段 %s", typ.Name(), name)
				continue
			}
			if f.Type.Kind() != reflect.Ptr {
				t.Errorf("%s.%s 必须是指针（nil=未传），实际 %s", typ.Name(), name, f.Type)
			}
		}
		for _, name := range c.mustKeep {
			f, ok := typ.FieldByName(name)
			if !ok {
				t.Errorf("%s 缺少字段 %s", typ.Name(), name)
				continue
			}
			if f.Type.Kind() == reflect.Ptr {
				t.Errorf("%s.%s 不应是指针，实际 %s", typ.Name(), name, f.Type)
			}
		}
	}
}
