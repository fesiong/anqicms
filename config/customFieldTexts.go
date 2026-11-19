package config

import (
	"strconv"
	"strings"
	"time"
)

// CustomFieldTextsTypes 是 texts 条目支持的字段类型，与 CustomField.Type 对齐。
//
// 注意这里只包含「单个条目」能表达的类型：
// editor / texts / archive / category / timeline 这类富文本、嵌套或关联类型
// 在条目层面无法表达，故意不在白名单内。
// images（组图）是例外：一个条目用 values 承载多张图，与字段级 images 语义一致。
var CustomFieldTextsTypes = []string{
	CustomFieldTypeText,
	CustomFieldTypeNumber,
	CustomFieldTypeTextarea,
	CustomFieldTypeRadio,
	CustomFieldTypeCheckbox,
	CustomFieldTypeSelect,
	CustomFieldTypeImage,
	CustomFieldTypeImages,
	CustomFieldTypeFile,
	CustomFieldTypeDate,
	CustomFieldTypeTime,
	CustomFieldTypeDatetime,
	CustomFieldTypeColor,
}

// IsCustomFieldTextsType 判断类型是否可用于 texts 条目。
func IsCustomFieldTextsType(typ string) bool {
	typ = strings.ToLower(strings.TrimSpace(typ))
	if typ == "" {
		return false
	}
	for _, v := range CustomFieldTextsTypes {
		if v == typ {
			return true
		}
	}
	return false
}

// NormalizeCustomFieldTextsType 归一化条目类型：
//   - 空值 → text（历史数据只有纯文本，必须继续可读可写）
//   - 合法类型 → 小写去空格后的值
//   - 非法类型 → 空串（交由校验报错，处理时按 text 兜底）
func NormalizeCustomFieldTextsType(typ string) string {
	typ = strings.ToLower(strings.TrimSpace(typ))
	if typ == "" {
		return CustomFieldTypeText
	}
	if IsCustomFieldTextsType(typ) {
		return typ
	}
	return ""
}

// EffectiveType 返回条目实际生效的类型。
// 历史数据没有 type 字段，按 text 处理；写了非法类型时同样退回 text，保证展示不崩。
func (c CustomFieldTexts) EffectiveType() string {
	if typ := NormalizeCustomFieldTextsType(c.Type); typ != "" {
		return typ
	}
	return CustomFieldTypeText
}

// IsMultiValue 判断该类型是否可以有多个值。
// text / textarea 沿用历史的「多行 values」；checkbox 是选项多选；images 是组图。
// 其余类型（number/radio/select/image/file/date/time/datetime/color）都是单值。
func (c CustomFieldTexts) IsMultiValue() bool {
	switch c.EffectiveType() {
	case CustomFieldTypeText, CustomFieldTypeTextarea, CustomFieldTypeCheckbox, CustomFieldTypeImages:
		return true
	}
	return false
}

// SyncValues 把 value / values 两个字段整理成与类型一致的样子：
//   - 多值类型：values 为唯一真相，value 同步成 values 里第一个非空值；
//   - 单值类型：value 为唯一真相，values 若为空则由 value 派生，避免旧数据丢失。
//
// 不做任何裁剪之外的事（不删字段），保证写回 JSON 后旧前端仍能读到原结构。
func (c *CustomFieldTexts) SyncValues() {
	if c == nil {
		return
	}
	if c.IsMultiValue() {
		if len(c.Values) == 0 && c.Value != "" {
			c.Values = []string{c.Value}
		}
		for i := range c.Values {
			c.Values[i] = NormalizeCustomFieldTextsValue(c.EffectiveType(), c.Values[i])
		}
		c.Value = ""
		for _, v := range c.Values {
			if v != "" {
				c.Value = v
				break
			}
		}
		return
	}
	c.Value = NormalizeCustomFieldTextsValue(c.EffectiveType(), c.Value)
	if len(c.Values) == 0 && c.Value != "" {
		c.Values = []string{c.Value}
	}
}

// DisplayText 返回条目用于展示的文本：
// 多值类型用换行拼接；checkbox 与 images 这类「单个值本身就是短串」的用逗号。
func (c CustomFieldTexts) DisplayText() string {
	if c.IsMultiValue() {
		sep := "\n"
		if typ := c.EffectiveType(); typ == CustomFieldTypeCheckbox || typ == CustomFieldTypeImages {
			sep = ", "
		}
		parts := make([]string, 0, len(c.Values))
		for _, v := range c.Values {
			if v != "" {
				parts = append(parts, v)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, sep)
		}
		return c.Value
	}
	return c.Value
}

// NormalizeCustomFieldTextsValue 按类型规整单个值。
// 无法识别的值原样返回（由校验负责报错），避免把用户数据改坏。
func NormalizeCustomFieldTextsValue(typ, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	switch NormalizeCustomFieldTextsType(typ) {
	case CustomFieldTypeNumber:
		// 数字统一成规范化写法（01 → 1，1.50 → 1.5），非法值保持原样。
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
	case CustomFieldTypeColor:
		// #RGB / #RRGGBB / 不带 # 的写法统一成小写 #rrggbb（三位则补成六位）。
		hex := strings.TrimPrefix(value, "#")
		if len(hex) == 3 && isHexString(hex) {
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		}
		if (len(hex) == 6 || len(hex) == 8) && isHexString(hex) {
			return "#" + strings.ToLower(hex)
		}
	case CustomFieldTypeDate:
		if t, ok := parseCustomFieldTextsDate(value); ok {
			return t.Format("2006-01-02")
		}
	case CustomFieldTypeTime:
		if t, ok := parseCustomFieldTextsTime(value); ok {
			if strings.Count(value, ":") == 1 {
				return t.Format("15:04")
			}
			return t.Format("15:04:05")
		}
	case CustomFieldTypeDatetime:
		if t, ok := parseCustomFieldTextsDatetime(value); ok {
			return t.Format("2006-01-02 15:04:05")
		}
	}
	return value
}

// CustomFieldTextsValueReason 校验单个条目，返回不通过的原因标识（"" = 通过）。
// 原因标识由上层负责翻译，避免 config 包依赖语言包。
func CustomFieldTextsValueReason(c CustomFieldTexts, options []string) string {
	if IsCustomFieldTextsType(c.Type) || strings.TrimSpace(c.Type) == "" {
		// 空类型按 text 处理，合法
	} else {
		return "type"
	}
	typ := c.EffectiveType()
	values := c.Values
	if !c.IsMultiValue() {
		values = []string{c.Value}
	} else if len(values) == 0 && c.Value != "" {
		values = []string{c.Value}
	}
	// 过滤空值：空条目本身不算填了内容，必填交给字段级 required 判定。
	filled := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			filled = append(filled, v)
		}
	}
	if len(filled) == 0 {
		return ""
	}
	switch typ {
	case CustomFieldTypeNumber:
		for _, v := range filled {
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return "number"
			}
		}
	case CustomFieldTypeColor:
		hex := strings.TrimPrefix(filled[0], "#")
		if !(len(hex) == 3 || len(hex) == 6 || len(hex) == 8) || !isHexString(hex) {
			return "color"
		}
	case CustomFieldTypeDate:
		if _, ok := parseCustomFieldTextsDate(filled[0]); !ok {
			return "date"
		}
	case CustomFieldTypeTime:
		if _, ok := parseCustomFieldTextsTime(filled[0]); !ok {
			return "time"
		}
	case CustomFieldTypeDatetime:
		if _, ok := parseCustomFieldTextsDatetime(filled[0]); !ok {
			return "datetime"
		}
	case CustomFieldTypeRadio, CustomFieldTypeSelect:
		if len(options) > 0 && !containsString(options, filled[0]) {
			return "option"
		}
	case CustomFieldTypeCheckbox:
		for _, v := range filled {
			if len(options) > 0 && !containsString(options, v) {
				return "option"
			}
		}
	}
	return ""
}

func isHexString(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func containsString(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

func parseCustomFieldTextsDate(value string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", "2006/01/02", "2006-1-2", "2006/1/2", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseCustomFieldTextsTime(value string) (time.Time, bool) {
	for _, layout := range []string{"15:04:05", "15:04", "3:04 PM"} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseCustomFieldTextsDatetime(value string) (time.Time, bool) {
	v := strings.TrimSpace(value)
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05Z07:00",
		"2006/01/02 15:04:05",
		time.RFC3339,
	} {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
