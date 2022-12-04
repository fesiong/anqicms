package library

import (
	"reflect"
	"regexp"
	"strings"
	"unsafe"
)

func StripTags(src string) string {
	//将HTML标签全转换成小写
	re, _ := regexp.Compile("\\<[\\S\\s]+?\\>")
	src = re.ReplaceAllStringFunc(src, strings.ToLower)
	//去除STYLE
	re, _ = regexp.Compile("\\<style[\\S\\s]+?\\</style\\>")
	src = re.ReplaceAllString(src, "")
	//去除SCRIPT
	re, _ = regexp.Compile("\\<script[\\S\\s]+?\\</script\\>")
	src = re.ReplaceAllString(src, "")
	//去除所有尖括号内的HTML代码，并换成换行符
	re, _ = regexp.Compile("\\<[\\S\\s]+?\\>")
	src = re.ReplaceAllString(src, "\n")
	//去除连续的换行符
	re, _ = regexp.Compile("\\s{2,}")
	src = re.ReplaceAllString(src, "\n")
	return strings.TrimSpace(src)
}

func Case2Camel(name string) string {
	name = strings.Replace(name, "_", " ", -1)
	name = strings.Title(name)
	return strings.Replace(name, " ", "", -1)
}

func ParseUrlToken(name string) string {
	if name == "" {
		return name
	}
	name = strings.ToLower(name)
	name = strings.Replace(name, " ", "-", -1)
	name = strings.Replace(name, "_", "-", -1)
	if name == "" {
		return name
	}
	names := []rune(name)
	for i := 0; i < len(names); i++ {
		if (names[i] >= 48 && names[i] <= 57) || (names[i] >= 97 && names[i] <= 122) || names[i] == 45 {
			// 这个范围是对的
		} else {
			// 需要删除
			names = append(names[:i], names[i+1:]...)
			i--
		}
	}
	//去除连续的换行符
	re, _ := regexp.Compile("-{2,}")
	name = re.ReplaceAllString(name, "-")
	return name
}

func ReplaceSingleSpace(content string) string {
	// 将单个&nbsp;替换为空格
	re, _ := regexp.Compile(`(&nbsp;|\xA0)+`)
	content = re.ReplaceAllStringFunc(content, func(s string) string {
		if s == "&nbsp;" || s == "\xA0" {
			return " "
		}
		return s
	})

	return content
}

// BytesToString casts slice to string without copy
func BytesToString(b []byte) (s string) {
	if len(b) == 0 {
		return ""
	}

	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := reflect.StringHeader{Data: bh.Data, Len: bh.Len}

	return *(*string)(unsafe.Pointer(&sh))
}

// StringToBytes casts string to slice without copy
func StringToBytes(s string) []byte {
	if len(s) == 0 {
		return []byte{}
	}

	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{Data: sh.Data, Len: sh.Len, Cap: sh.Len}

	return *(*[]byte)(unsafe.Pointer(&bh))
}

func EscapeString(v string) string {
	var pos = 0
	if len(v) == 0 {
		return ""
	}
	buf := make([]byte, len(v[:])*2)
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch c {
		case '\x00':
			buf[pos] = '\\'
			buf[pos+1] = '0'
			pos += 2
		case '\n':
			buf[pos] = '\\'
			buf[pos+1] = 'n'
			pos += 2
		case '\r':
			buf[pos] = '\\'
			buf[pos+1] = 'r'
			pos += 2
		case '\x1a':
			buf[pos] = '\\'
			buf[pos+1] = 'Z'
			pos += 2
		case '\'':
			buf[pos] = '\\'
			buf[pos+1] = '\''
			pos += 2
		case '"':
			buf[pos] = '\\'
			buf[pos+1] = '"'
			pos += 2
		case '\\':
			buf[pos] = '\\'
			buf[pos+1] = '\\'
			pos += 2
		default:
			buf[pos] = c
			pos++
		}
	}
	return string(buf[:pos])
}
