package provider

import (
	"fmt"
	"github.com/huichen/sego"
	"kandaoni.com/anqicms/config"
	"log"
	"strings"
)

var segmenter sego.Segmenter
var dictLoaded = false

const removeWord = " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~。？！，、；：“ ” ‘ ’「」『』（）〔〕【】《》〈〉—…·～"

func WordSplit(s string, contain bool) []string {
	if !dictLoaded {
		initDict()
	}
	if !dictLoaded {
		return []string{s}
	}
	segments := segmenter.Segment([]byte(s))

	words := sego.SegmentsToSlice(segments, false)
	// 移除标点、空格等
	x := 0
	// 对大小写字母进行还原
	ss := []rune(s)
	l := len(ss)
	if contain {
		for i := 0; i < len(words); i++ {
			ws := []rune(words[i])
			change := false
			for j := range ws {
				if x < l && ss[x] != ws[j] {
					change = true
					ws[j] = ss[x]
				}
				x++
			}
			if change {
				words[i] = string(ws)
			}
		}
	} else {
		for i := 0; i < len(words); i++ {
			if len(words[i]) == 1 && strings.ContainsAny(words[i], removeWord) {
				words = append(words[:i], words[i+1:]...)
				i--
			}
		}
	}

	return words
}

func DictClose() {
	segmenter.Close()
}

func initDict() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("初始化 sego 失败:", err)
		}
	}()
	dictFile := fmt.Sprintf("%s%s.txt", config.ExecPath, "dictionary")
	segmenter.LoadDictionary(dictFile)
	dictLoaded = true
}
