package library

import (
	"github.com/mozillazg/go-pinyin"
	"strings"
)

var py pinyin.Args

func GetPinyin(hans string) string {
	var result = make([]string, 0, len(hans))
	tmpHans := []rune(hans)
	var tmp string
	for i, r := range tmpHans {
		if (r >= 65 && r <= 90) || (r >= 97 && r <= 122) {
			tmp += string(r)
			if i == len(tmpHans)-1 {
				result = append(result, tmp)
			}
		} else {
			if tmp != "" {
				result = append(result, tmp)
				tmp = ""
			}
			result = append(result, pinyin.Slug(string(r), py))
		}
	}
	str := strings.Join(result, "-")
	str = ParseUrlToken(str)
	if len(str) > 100 {
		str = str[:100]
	}

	str = strings.Trim(str, "-")

	return str
}

func init() {
	py = pinyin.NewArgs()
}
