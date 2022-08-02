package library

import (
	"math/rand"
	"regexp"
	"strings"
	"time"
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

func GenerateRandString(length int) string {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		b := r.Intn(26) + 65
		bytes[i] = byte(b)
	}
	return strings.ToLower(string(bytes))
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
