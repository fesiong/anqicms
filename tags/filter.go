package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"strings"
)

func init() {
	pongo2.RegisterFilter("contain", filterContain)
	pongo2.RegisterFilter("trim", filterTrim)
	pongo2.RegisterFilter("trimLeft", filterTrimLeft)
	pongo2.RegisterFilter("trimRight", filterTrimRight)
	pongo2.RegisterFilter("replace", filterReplace)
	pongo2.RegisterFilter("list", filterList)
	pongo2.RegisterFilter("fields", filterFields)
	pongo2.RegisterFilter("count", filterCount)
	pongo2.RegisterFilter("index", filterIndex)
	pongo2.RegisterFilter("repeat", filterRepeat)
	pongo2.RegisterFilter("dump", filterDump)
}

func filterContain(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(in.Contains(param)), nil
}

func filterTrim(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	if param.IsNil() || len(param.String()) == 0 {
		return pongo2.AsValue(strings.TrimSpace(in.String())), nil
	}
	return pongo2.AsValue(strings.Trim(in.String(), param.String())), nil
}

func filterTrimLeft(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(strings.TrimLeft(in.String(), param.String())), nil
}

func filterTrimRight(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(strings.TrimRight(in.String(), param.String())), nil
}

func filterReplace(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	sep := strings.Split(param.String(), ",")
	from := sep[0]
	to := ""
	if len(sep) > 1 {
		to = sep[1]
	}
	return pongo2.AsValue(strings.ReplaceAll(in.String(), from, to)), nil
}

// 格式 ["aaa", "ddd", 123]
func filterList(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	income := []rune(strings.TrimSpace(strings.Trim(strings.Trim(in.String(), "'\""), "[]")))
	var result = make([]string, 0, len(income))
	if len(income) == 0 {
		return pongo2.AsValue(result), nil
	}
	start := 0
	var hasComma rune
	if income[0] == '\'' || income[0] == '"' {
		hasComma = income[0]
		start = 1
	}
	for i := 1; i < len(income); i++ {
		if hasComma > 0 && income[i] == hasComma {
			tmp := income[start:i]
			result = append(result, string(tmp))
			start = i + 1
			hasComma = 0
		} else if income[i] == ',' && hasComma == 0 {
			if start < i {
				tmp := income[start:i]
				result = append(result, string(tmp))
				start = i + 1
			} else if start == i {
				start = i + 1
			}
		} else if income[i] == ' ' && hasComma == 0 {
			if start < i {
				tmp := income[start:i]
				result = append(result, string(tmp))
				start = i + 1
			} else if start == i {
				start = i + 1
			}
		} else if i == len(income)-1 && start <= i {
			tmp := income[start:]
			result = append(result, string(tmp))
		} else if (income[i] == '\'' || income[i] == '"') && hasComma == 0 {
			hasComma = income[i]
			start = i + 1
		}
	}
	return pongo2.AsValue(result), nil
}

func filterFields(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(strings.Fields(in.String())), nil
}

func filterCount(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	if in.IsString() {
		return pongo2.AsValue(strings.Count(in.String(), param.String())), nil
	}
	total := 0
	if in.CanSlice() {
		// slice
		in.Iterate(func(idx, count int, key, value *pongo2.Value) bool {
			if value != nil {
				if value.EqualValueTo(param) {
					total++
				}
			} else if key.EqualValueTo(param) {
				total++
			}
			return true
		}, func() {})
	}
	return pongo2.AsValue(total), nil
}

func filterIndex(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	if in.IsString() {
		return pongo2.AsValue(strings.Index(in.String(), param.String())), nil
	}
	index := -1
	if in.CanSlice() {
		// slice
		in.Iterate(func(idx, count int, key, value *pongo2.Value) bool {
			if value != nil {
				if value.EqualValueTo(param) {
					index = idx
					return false
				}
			} else if key.EqualValueTo(param) {
				index = idx
				return false
			}
			return true
		}, func() {})
	}
	return pongo2.AsValue(index), nil
}

func filterRepeat(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(strings.Repeat(in.String(), param.Integer())), nil
}

func filterDump(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(fmt.Sprintf("%#v", in.Interface())), nil
}
