package provider

import (
	"encoding/json"
	"errors"
	"strings"

	"kandaoni.com/anqicms/config"
)

// texts 类型自定义字段的读写辅助。
//
// 背景：texts 字段存的是 JSON 数组（每项 key/value/values）。2026-11 起每个条目
// 可以带自己的 type（见 config.CustomFieldTextsTypes），于是「解析 → 规整 → 校验」
// 这套逻辑必须在所有读写点保持一致，不能再各页各写一遍。

// ParseCustomFieldTexts 把库里或请求体里的原始值统一成 []config.CustomFieldTexts。
//
// 兼容以下历史形态：
//   - JSON 字符串（DB 列 / 表单提交）
//   - []config.CustomFieldTexts（已经解析过）
//   - []interface{}（json 解码出来的 map 列表）
//   - map 且带 "value" 键（CustomField 包装体）
func ParseCustomFieldTexts(value interface{}) []config.CustomFieldTexts {
	switch v := value.(type) {
	case nil:
		return nil
	case []config.CustomFieldTexts:
		return v
	case *[]config.CustomFieldTexts:
		if v == nil {
			return nil
		}
		return *v
	case string:
		return parseCustomFieldTextsJSON(v)
	case []byte:
		return parseCustomFieldTextsJSON(string(v))
	case map[string]interface{}:
		if inner, ok := v["value"]; ok {
			return ParseCustomFieldTexts(inner)
		}
		// 单个条目也允许
		if _, ok := v["key"]; ok {
			if buf, err := json.Marshal(v); err == nil {
				return parseCustomFieldTextsJSON(string(buf))
			}
		}
		return nil
	}
	// 其它形态（[]interface{}、[]map[string]string 等）统一走一次 JSON 往返
	buf, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return parseCustomFieldTextsJSON(string(buf))
}

func parseCustomFieldTextsJSON(value string) []config.CustomFieldTexts {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return nil
	}
	var texts []config.CustomFieldTexts
	if err := json.Unmarshal([]byte(value), &texts); err != nil {
		return nil
	}
	return texts
}

// NormalizeCustomFieldTexts 按每个条目的 type 规整值，返回新切片（不修改入参）。
//
// replace=true 用于输出给前台/接口：把 image/file 的存储路径换成可访问 URL；
// replace=false 用于写库：只做本地化路径规整，保留相对路径。
func (w *Website) NormalizeCustomFieldTexts(texts []config.CustomFieldTexts, replace bool) []config.CustomFieldTexts {
	if len(texts) == 0 {
		return texts
	}
	result := make([]config.CustomFieldTexts, 0, len(texts))
	for _, item := range texts {
		// 历史条目没有 type：保持空，让 omitempty 不往 JSON 里塞新字段，
		// 读的时候由 EffectiveType 兜底成 text。非法类型保留原值交给校验报错。
		if strings.TrimSpace(item.Type) == "" {
			item.Type = ""
		} else if typ := config.NormalizeCustomFieldTextsType(item.Type); typ != "" {
			item.Type = typ
		}
		item.SyncValues()
		if w != nil && w.PluginStorage != nil {
			typ := item.EffectiveType()
			// images 的多张图放在 values 里，单图 image / file 放在 value 里；
			// 两边都走一遍，保证组图在接口输出时同样是完整 URL。
			if typ == config.CustomFieldTypeImage || typ == config.CustomFieldTypeImages || typ == config.CustomFieldTypeFile {
				item.Value = w.ReplaceContentUrl(item.Value, replace)
				for i := range item.Values {
					item.Values[i] = w.ReplaceContentUrl(item.Values[i], replace)
				}
			}
		}
		result = append(result, item)
	}
	return result
}

// FormatCustomFieldTexts 处理读路径上的 texts 字段值：
// 解析 + 按类型规整（replace 决定是否换成完整 URL）。解析失败时返回 nil。
func (w *Website) FormatCustomFieldTexts(value interface{}, replace bool) []config.CustomFieldTexts {
	texts := ParseCustomFieldTexts(value)
	if texts == nil {
		return nil
	}
	return w.NormalizeCustomFieldTexts(texts, replace)
}

// ValidateCustomFieldTexts 校验 texts 字段里的每个条目。
//
// 只校验「条目自己声明了类型」的约束：类型是否受支持、number/date/time/datetime/color
// 的格式、radio/select/checkbox 是否命中选项。历史的纯文本条目（type 为空）不受影响，
// 因此不会让既有数据在保存时报错。
func (w *Website) ValidateCustomFieldTexts(value interface{}, field *config.CustomField) error {
	texts := ParseCustomFieldTexts(value)
	if len(texts) == 0 {
		return nil
	}
	fieldName := ""
	if field != nil {
		fieldName = field.Name
		if fieldName == "" {
			fieldName = field.FieldName
		}
	}
	var options []string
	if field != nil {
		options = splitCustomFieldOptions(field.Content)
	}
	for _, item := range texts {
		reason := config.CustomFieldTextsValueReason(item, options)
		if reason == "" {
			continue
		}
		value := item.Value
		if value == "" && len(item.Values) > 0 {
			value = strings.Join(item.Values, ",")
		}
		switch reason {
		case "type":
			return errors.New(w.Tr("CustomFieldTextsTypeUnsupported", fieldName, item.Type))
		case "option":
			return errors.New(w.Tr("CustomFieldTextsValueNotInOptions", fieldName, value))
		default:
			return errors.New(w.Tr("CustomFieldTextsValueInvalid", fieldName, value, item.EffectiveType()))
		}
	}
	return nil
}

// StringifyCustomFieldTexts 把 texts 值序列化成写库用的 JSON 字符串。
// 先按类型规整（保留相对路径），再做一次 JSON 编码；空列表写空串。
func (w *Website) StringifyCustomFieldTexts(value interface{}) string {
	texts := ParseCustomFieldTexts(value)
	if len(texts) == 0 {
		return ""
	}
	texts = w.NormalizeCustomFieldTexts(texts, false)
	buf, err := json.Marshal(texts)
	if err != nil {
		return ""
	}
	return string(buf)
}

func splitCustomFieldOptions(content string) []string {
	var options []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			options = append(options, line)
		}
	}
	return options
}
