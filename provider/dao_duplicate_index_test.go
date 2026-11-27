package provider

import (
	"errors"
	"testing"

	"gorm.io/gorm/schema"
	"kandaoni.com/anqicms/model"
)

// 这条护栏来自 2026-11-27 现场：唯一索引已存在，GORM 仍发 ADD CONSTRAINT 报 1061。
// 容错只能放名字恰好等于某个 unique 字段期望约束名的错误，其它一律不放行。
func TestDuplicateKeyName(t *testing.T) {
	got := duplicateKeyName(errors.New("Error 1061 (42000): Duplicate key name 'uni_anchors_title'"))
	if got != "uni_anchors_title" {
		t.Fatalf("解析 1061 索引名失败：got %q", got)
	}

	// 反向对照：非 1061、非该形态的错误不能被误判成"索引已存在"。
	for _, err := range []error{
		nil,
		errors.New("Error 1062 (23000): Duplicate entry 'x' for key 'uni_anchors_title'"),
		errors.New("Error 1061 (42000): Duplicate key name uni_anchors_title"),
		errors.New("migrate table error"),
	} {
		if name := duplicateKeyName(err); name != "" {
			t.Errorf("反向对照失败：错误 %v 被误判为重复索引 %q", err, name)
		}
	}
}

func TestExpectedUniqueColumnForName(t *testing.T) {
	namer := schema.NamingStrategy{} // 纯 schema 计算，不连库
	models := []interface{}{&model.Anchor{}}

	m, column, ok := expectedUniqueColumnForName(namer, models, "uni_anchors_title")
	if !ok || column != "title" || m == nil {
		t.Fatalf("uni_anchors_title 应命中 Anchor.title：ok=%v column=%q", ok, column)
	}

	// 反向对照：非 unique 列、拼错的名字、别的模型的名字都不能命中。
	for _, name := range []string{
		"uni_anchors_link", // link 是普通 index，不是 unique
		"uni_anchors_titel",
		"uni_anchor_data_title",
		"uni_anchors_title_extra",
	} {
		if _, _, ok := expectedUniqueColumnForName(namer, models, name); ok {
			t.Errorf("反向对照失败：%q 被误判为 unique 约束名", name)
		}
	}
	if _, _, ok := expectedUniqueColumnForName(namer, []interface{}{&model.AnchorData{}}, "uni_anchors_title"); ok {
		t.Error("反向对照失败：AnchorData 模型命中了 anchors 的约束名")
	}
}
