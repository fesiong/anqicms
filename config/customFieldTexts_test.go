package config

import (
	"strings"
	"testing"
)

func TestNormalizeCustomFieldTextsType(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", CustomFieldTypeText},     // 历史数据没有 type，按纯文本
		{"   ", CustomFieldTypeText},  // 空白同样按文本
		{"TEXT", CustomFieldTypeText}, // 大小写不敏感
		{" Number ", CustomFieldTypeNumber},
		{"datetime", CustomFieldTypeDatetime},
		{"images", CustomFieldTypeImages}, // 组图：条目层用 values 承载多张图
		{"IMAGES", CustomFieldTypeImages}, // 大小写不敏感
		{"editor", ""},                    // 富文本不适合放在条目层
		{"unknown", ""},                   // 非法类型返回空串，交给校验
		{"texts", ""},                     // 嵌套 texts 无意义
		{"archive", ""},                   // 关联类型不在白名单
		{"checkbox", CustomFieldTypeCheckbox},
	}
	for _, c := range cases {
		if got := NormalizeCustomFieldTextsType(c.in); got != c.want {
			t.Fatalf("NormalizeCustomFieldTextsType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCustomFieldTextsEffectiveTypeIsBackwardCompatible(t *testing.T) {
	// 零值（历史数据反序列化的结果）必须是 text，否则老数据展示会整片空掉。
	var legacy CustomFieldTexts
	if got := legacy.EffectiveType(); got != CustomFieldTypeText {
		t.Fatalf("legacy EffectiveType() = %q, want text", got)
	}
	// 非法类型也不能崩，退回 text 只是为了展示，校验另有入口。
	bad := CustomFieldTexts{Type: "whatever"}
	if got := bad.EffectiveType(); got != CustomFieldTypeText {
		t.Fatalf("invalid EffectiveType() = %q, want text", got)
	}
}

func TestCustomFieldTextsSyncValues(t *testing.T) {
	// 多值类型：values 是唯一真相，value 同步成第一个非空值。
	text := CustomFieldTexts{Type: CustomFieldTypeText, Values: []string{"", " 甲 ", "乙"}}
	text.SyncValues()
	if strings.Join(text.Values, "|") != "|甲|乙" {
		t.Fatalf("text values = %q", text.Values)
	}
	if text.Value != "甲" {
		t.Fatalf("text value = %q, want 甲", text.Value)
	}

	// 只有 value 的旧数据，要派生出 values，避免前端多行编辑器读不到内容。
	legacy := CustomFieldTexts{Value: "只有单值"}
	legacy.SyncValues()
	if len(legacy.Values) != 1 || legacy.Values[0] != "只有单值" {
		t.Fatalf("legacy values = %q", legacy.Values)
	}

	// 单值类型：value 是唯一真相，并按类型规整（01 → 1）。
	number := CustomFieldTexts{Type: CustomFieldTypeNumber, Value: "01"}
	number.SyncValues()
	if number.Value != "1" {
		t.Fatalf("number value = %q, want 1", number.Value)
	}
	if len(number.Values) != 1 || number.Values[0] != "1" {
		t.Fatalf("number values = %q", number.Values)
	}

	// checkbox 是多值，value 取首个非空。
	check := CustomFieldTexts{Type: CustomFieldTypeCheckbox, Values: []string{"A", "B"}}
	check.SyncValues()
	if check.Value != "A" {
		t.Fatalf("checkbox value = %q, want A", check.Value)
	}
	if check.DisplayText() != "A, B" {
		t.Fatalf("checkbox DisplayText() = %q", check.DisplayText())
	}

	// images 同样是多值：多张图放在 values，value 兜第一个供旧消费方读取。
	images := CustomFieldTexts{Type: CustomFieldTypeImages, Values: []string{"", "/uploads/a.png", " /uploads/b.png "}}
	images.SyncValues()
	if strings.Join(images.Values, "|") != "|/uploads/a.png|/uploads/b.png" {
		t.Fatalf("images values = %q", images.Values)
	}
	if images.Value != "/uploads/a.png" {
		t.Fatalf("images value = %q, want 首个非空图", images.Value)
	}
	if images.DisplayText() != "/uploads/a.png, /uploads/b.png" {
		t.Fatalf("images DisplayText() = %q", images.DisplayText())
	}
	// 只有一张图的旧数据（value 有值、values 为空）同样要派生出来，否则前端组图编辑器看不到图。
	imagesLegacy := CustomFieldTexts{Type: CustomFieldTypeImages, Value: "/uploads/only.png"}
	imagesLegacy.SyncValues()
	if len(imagesLegacy.Values) != 1 || imagesLegacy.Values[0] != "/uploads/only.png" {
		t.Fatalf("images legacy values = %q", imagesLegacy.Values)
	}
}

func TestCustomFieldTextsDisplayText(t *testing.T) {
	if got := (CustomFieldTexts{Values: []string{"第一行", "第二行"}}).DisplayText(); got != "第一行\n第二行" {
		t.Fatalf("text DisplayText() = %q", got)
	}
	if got := (CustomFieldTexts{Type: CustomFieldTypeTextarea, Values: []string{"a", "b"}}).DisplayText(); got != "a\nb" {
		t.Fatalf("textarea DisplayText() = %q", got)
	}
	if got := (CustomFieldTexts{Type: CustomFieldTypeSelect, Value: "A", Values: []string{"B"}}).DisplayText(); got != "A" {
		t.Fatalf("select DisplayText() = %q", got)
	}
	// 多值类型但 values 全空时退回 value，避免展示成空串。
	if got := (CustomFieldTexts{Type: CustomFieldTypeText, Value: "兜底"}).DisplayText(); got != "兜底" {
		t.Fatalf("fallback DisplayText() = %q", got)
	}
}

func TestNormalizeCustomFieldTextsValue(t *testing.T) {
	cases := []struct{ typ, in, want string }{
		{CustomFieldTypeNumber, " 0012 ", "12"},
		{CustomFieldTypeNumber, "3.50", "3.5"},
		{CustomFieldTypeNumber, "abc", "abc"}, // 非法值不改写，交给校验
		{CustomFieldTypeColor, "abc", "#aabbcc"},
		{CustomFieldTypeColor, "#AABBCC", "#aabbcc"},
		{CustomFieldTypeColor, "#zzz", "#zzz"},
		{CustomFieldTypeDate, "2026/1/2", "2026-01-02"},
		{CustomFieldTypeDate, "2026-13-40", "2026-13-40"},
		{CustomFieldTypeTime, "9:05", "09:05"},
		{CustomFieldTypeTime, "09:05:07", "09:05:07"},
		{CustomFieldTypeDatetime, "2026-01-02 03:04", "2026-01-02 03:04:00"},
		{CustomFieldTypeText, "  原样  ", "原样"},
	}
	for _, c := range cases {
		if got := NormalizeCustomFieldTextsValue(c.typ, c.in); got != c.want {
			t.Fatalf("NormalizeCustomFieldTextsValue(%q, %q) = %q, want %q", c.typ, c.in, got, c.want)
		}
	}
}

func TestCustomFieldTextsValueReason(t *testing.T) {
	options := []string{"A", "B"}

	// 历史纯文本条目：任何内容都合法（不能因为升级就把老数据判成错）。
	if got := CustomFieldTextsValueReason(CustomFieldTexts{Key: "k", Value: "任意内容"}, options); got != "" {
		t.Fatalf("legacy reason = %q", got)
	}
	if got := CustomFieldTextsValueReason(CustomFieldTexts{Type: CustomFieldTypeNumber, Value: "12"}, nil); got != "" {
		t.Fatalf("number reason = %q", got)
	}
	if got := CustomFieldTextsValueReason(CustomFieldTexts{Type: CustomFieldTypeNumber, Value: "十二"}, nil); got != "number" {
		t.Fatalf("bad number reason = %q", got)
	}
	if got := CustomFieldTextsValueReason(CustomFieldTexts{Type: CustomFieldTypeSelect, Value: "C"}, options); got != "option" {
		t.Fatalf("bad option reason = %q", got)
	}
	if got := CustomFieldTextsValueReason(CustomFieldTexts{Type: CustomFieldTypeCheckbox, Values: []string{"A", "C"}}, options); got != "option" {
		t.Fatalf("bad checkbox reason = %q", got)
	}
	if got := CustomFieldTextsValueReason(CustomFieldTexts{Type: CustomFieldTypeDate, Value: "2026-02-30"}, nil); got != "date" {
		t.Fatalf("bad date reason = %q", got)
	}
	if got := CustomFieldTextsValueReason(CustomFieldTexts{Type: CustomFieldTypeColor, Value: "#xyz"}, nil); got != "color" {
		t.Fatalf("bad color reason = %q", got)
	}
	if got := CustomFieldTextsValueReason(CustomFieldTexts{Type: "editor", Value: "a.png"}, nil); got != "type" {
		t.Fatalf("bad type reason = %q", got)
	}
	// images 已在白名单内：多张图的路径不做格式约束，只要能拿到字符串就通过。
	if got := CustomFieldTextsValueReason(CustomFieldTexts{Type: CustomFieldTypeImages, Values: []string{"/uploads/a.png", "/uploads/b.png"}}, nil); got != "" {
		t.Fatalf("images reason = %q", got)
	}
	// 空条目不算错：必填由字段级 required 判定，这里不重复拦。
	if got := CustomFieldTextsValueReason(CustomFieldTexts{Type: CustomFieldTypeNumber, Value: " "}, nil); got != "" {
		t.Fatalf("empty reason = %q", got)
	}
}
