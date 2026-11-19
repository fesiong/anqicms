package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
)

func testTextsSite() *Website {
	return &Website{}
}

func TestParseCustomFieldTexts(t *testing.T) {
	// 1) 历史 JSON（无 type）必须原样解析出来
	legacy := `[{"key":"品牌","value":"","values":["安企","AnQi"]}]`
	items := ParseCustomFieldTexts(legacy)
	if len(items) != 1 || items[0].Key != "品牌" || len(items[0].Values) != 2 {
		t.Fatalf("legacy parse = %+v", items)
	}
	if items[0].EffectiveType() != config.CustomFieldTypeText {
		t.Fatalf("legacy type = %q", items[0].EffectiveType())
	}

	// 2) CustomField 包装体（{value: [...]}）也要能解出来
	wrapped := map[string]interface{}{
		"name":  "参数",
		"type":  "texts",
		"value": []interface{}{map[string]interface{}{"key": "k", "value": "v"}},
	}
	if got := ParseCustomFieldTexts(wrapped); len(got) != 1 || got[0].Key != "k" {
		t.Fatalf("wrapped parse = %+v", got)
	}

	// 3) []interface{}（请求体 json 解码结果）
	raw := []interface{}{map[string]interface{}{"key": "a", "value": "1", "type": "number"}}
	if got := ParseCustomFieldTexts(raw); len(got) != 1 || got[0].Type != config.CustomFieldTypeNumber {
		t.Fatalf("slice parse = %+v", got)
	}

	// 4) 空值/坏 JSON 返回 nil，不 panic
	for _, bad := range []interface{}{nil, "", "null", "not-json", 123} {
		if got := ParseCustomFieldTexts(bad); got != nil {
			t.Fatalf("ParseCustomFieldTexts(%v) = %+v, want nil", bad, got)
		}
	}
}

func TestNormalizeCustomFieldTextsKeepsLegacyShape(t *testing.T) {
	site := testTextsSite()
	items := []config.CustomFieldTexts{{Key: "k", Value: "", Values: []string{"a", "b"}}}
	got := site.NormalizeCustomFieldTexts(items, false)

	// 历史条目不能凭空长出 "type" 字段，否则老版本后台/接口会看到陌生的载荷
	buf, _ := json.Marshal(got)
	if strings.Contains(string(buf), `"type"`) {
		t.Fatalf("legacy item gained type field: %s", buf)
	}
	if got[0].Value != "a" {
		t.Fatalf("legacy value = %q, want 首个非空值 a", got[0].Value)
	}

	// 新类型条目：类型被归一化成小写
	typed := []config.CustomFieldTexts{{Key: "n", Type: " NUMBER ", Value: "01"}}
	buf, _ = json.Marshal(site.NormalizeCustomFieldTexts(typed, false))
	if !strings.Contains(string(buf), `"type":"number"`) || !strings.Contains(string(buf), `"value":"1"`) {
		t.Fatalf("typed item = %s", buf)
	}

	// 非法类型保留原值，交给校验报错（不能被悄悄洗成 text）
	bad := site.NormalizeCustomFieldTexts([]config.CustomFieldTexts{{Type: "editor"}}, false)
	if bad[0].Type != "editor" {
		t.Fatalf("invalid type = %q, want 原样保留", bad[0].Type)
	}
}

func TestFormatCustomFieldTextsReplacesImageUrl(t *testing.T) {
	site := &Website{PluginStorage: &config.PluginStorageConfig{StorageUrl: "https://cdn.example.com"}}
	items := []config.CustomFieldTexts{
		{Key: "封面", Type: config.CustomFieldTypeImage, Value: "/uploads/a.png"},
	}
	got := site.FormatCustomFieldTexts(items, true)
	if got[0].Value != "https://cdn.example.com/uploads/a.png" {
		t.Fatalf("display url = %q", got[0].Value)
	}

	// 写库方向必须还原成相对路径，否则库里会混进带域名的旧值
	back := site.NormalizeCustomFieldTexts(got, false)
	if back[0].Value != "/uploads/a.png" {
		t.Fatalf("storage url = %q", back[0].Value)
	}

	// 非图片类型不做 URL 处理
	text := site.FormatCustomFieldTexts([]config.CustomFieldTexts{{Key: "k", Value: "/uploads/a.png"}}, true)
	if text[0].Value != "/uploads/a.png" {
		t.Fatalf("text value should stay: %q", text[0].Value)
	}
}

func TestFormatCustomFieldTextsReplacesImagesUrls(t *testing.T) {
	// ReplaceContentUrl 写库方向会读 System.BaseUrl，测试站点要补上，否则空指针。
	site := &Website{
		PluginStorage: &config.PluginStorageConfig{StorageUrl: "https://cdn.example.com"},
		System:        &config.SystemConfig{BaseUrl: "https://example.com"},
	}
	items := []config.CustomFieldTexts{
		{Key: "相册", Type: config.CustomFieldTypeImages, Values: []string{"/uploads/a.png", "/uploads/b.png"}},
	}
	got := site.FormatCustomFieldTexts(items, true)
	if len(got[0].Values) != 2 ||
		got[0].Values[0] != "https://cdn.example.com/uploads/a.png" ||
		got[0].Values[1] != "https://cdn.example.com/uploads/b.png" {
		t.Fatalf("images display = %v", got[0].Values)
	}
	if got[0].Value != "https://cdn.example.com/uploads/a.png" {
		t.Fatalf("images value = %q", got[0].Value)
	}

	// 写库方向必须还原成相对路径，否则库里会混进带域名的旧值
	back := site.NormalizeCustomFieldTexts(got, false)
	if strings.Join(back[0].Values, "|") != "/uploads/a.png|/uploads/b.png" {
		t.Fatalf("images storage = %v", back[0].Values)
	}

	// 组图经过 Stringify 再解析仍然保留 values 顺序，保证前端编辑器可继续增删
	out := site.StringifyCustomFieldTexts(items)
	roundtrip := ParseCustomFieldTexts(out)
	if len(roundtrip) != 1 || roundtrip[0].EffectiveType() != config.CustomFieldTypeImages {
		t.Fatalf("images roundtrip = %s", out)
	}
	if strings.Join(roundtrip[0].Values, "|") != "/uploads/a.png|/uploads/b.png" {
		t.Fatalf("images roundtrip values = %v", roundtrip[0].Values)
	}
}

func TestStringifyCustomFieldTexts(t *testing.T) {
	site := testTextsSite()

	// 空列表写空串（与旧实现一致，DB 列为空）
	if got := site.StringifyCustomFieldTexts([]interface{}{}); got != "" {
		t.Fatalf("empty stringify = %q", got)
	}

	raw := []interface{}{
		map[string]interface{}{"key": "价格", "type": "number", "value": " 12 "},
		map[string]interface{}{"key": "标签", "type": "checkbox", "values": []interface{}{"A", "B"}},
	}
	out := site.StringifyCustomFieldTexts(raw)
	items := ParseCustomFieldTexts(out)
	if len(items) != 2 {
		t.Fatalf("stringify roundtrip = %s", out)
	}
	if items[0].Value != "12" {
		t.Fatalf("number not normalized: %q", items[0].Value)
	}
	if items[1].DisplayText() != "A, B" {
		t.Fatalf("checkbox display = %q", items[1].DisplayText())
	}
}

func TestValidateCustomFieldTexts(t *testing.T) {
	site := testTextsSite()
	field := &config.CustomField{Name: "参数", FieldName: "params", Type: config.CustomFieldTypeTexts, Content: "A\nB"}

	// 合法：历史纯文本 + 命中选项的下拉
	ok := []interface{}{
		map[string]interface{}{"key": "任意", "value": "随便写"},
		map[string]interface{}{"key": "等级", "type": "select", "value": "A"},
	}
	if err := site.ValidateCustomFieldTexts(ok, field); err != nil {
		t.Fatalf("valid case failed: %v", err)
	}

	// 不合法：number 写成了中文
	badNumber := []interface{}{map[string]interface{}{"key": "数量", "type": "number", "value": "三"}}
	if err := site.ValidateCustomFieldTexts(badNumber, field); err == nil {
		t.Fatalf("bad number passed validation")
	} else if !strings.Contains(err.Error(), "三") {
		// 文案会被翻译，只断言「错误里点出了非法值」，具体原因由 config 层测试覆盖
		t.Fatalf("bad number error = %v", err)
	}

	// 不合法：选项外取值
	badOption := []interface{}{map[string]interface{}{"key": "等级", "type": "select", "value": "C"}}
	if err := site.ValidateCustomFieldTexts(badOption, field); err == nil {
		t.Fatalf("bad option passed validation")
	} else if !strings.Contains(err.Error(), "C") {
		t.Fatalf("bad option error = %v", err)
	}

	// 不合法：条目类型不受支持
	badType := []interface{}{map[string]interface{}{"key": "嵌套", "type": "texts", "value": "x"}}
	if err := site.ValidateCustomFieldTexts(badType, field); err == nil {
		t.Fatalf("bad type passed validation")
	} else if !strings.Contains(err.Error(), "texts") {
		t.Fatalf("bad type error = %v", err)
	}

	// 空值与空列表不该报错
	if err := site.ValidateCustomFieldTexts(nil, field); err != nil {
		t.Fatalf("nil texts: %v", err)
	}
	empty := []interface{}{map[string]interface{}{"key": "", "type": "number", "value": ""}}
	if err := site.ValidateCustomFieldTexts(empty, field); err != nil {
		t.Fatalf("empty value: %v", err)
	}
}
