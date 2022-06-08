package library

import "github.com/mozillazg/go-pinyin"

var py = pinyin.NewArgs()

func GetPinyin(hans string) string {
	result := pinyin.Slug(hans, py)
	result = ParseUrlToken(result)

	return result
}
