package library

import (
	"bytes"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"io"
)

// CharsetMap 字符集映射
var CharsetMap = map[string]encoding.Encoding{
	"utf-8":   unicode.UTF8,
	"gbk":     simplifiedchinese.GBK,
	"gb2312":  simplifiedchinese.GB18030,
	"gb18030": simplifiedchinese.GB18030,
	"big5":    traditionalchinese.Big5,
}

// DecodeToUTF8 从输入的byte数组中按照指定的字符集解析出对应的utf8格式的内容并返回.
func DecodeToUTF8(input []byte, charset encoding.Encoding) (output []byte, err error) {
	if charset == nil || charset == unicode.UTF8 {
		output = input
		return
	}
	reader := transform.NewReader(bytes.NewReader(input), charset.NewDecoder())
	output, err = io.ReadAll(reader)
	if err != nil {
		return
	}
	return
}
