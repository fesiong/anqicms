package library

import (
	"fmt"
	"github.com/huichen/sego"
	"kandaoni.com/anqicms/config"
	"strings"
)

var segmenter sego.Segmenter
var dictLoaded = false
const removeWord = " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~。？！，、；：“ ” ‘ ’「」『』（）〔〕【】《》〈〉—…·～"

func WordSplit(s string, searchMode bool) []string {
	if !dictLoaded {
		initDict()
	}
	segments := segmenter.Segment([]byte(s))

	words := sego.SegmentsToSlice(segments, searchMode)
	// 移除标点、空格等
	for i := 0; i < len(words); i++ {
		if len(words[i]) == 1 && strings.ContainsAny(words[i], removeWord) {
			words = append(words[:i], words[i+1:]...)
			i--
		}
	}

	return words
}

func DictClose() {
	segmenter.Close()
}

func initDict() {
	dictFile := fmt.Sprintf("%s%s.txt", config.ExecPath, "dictionary")
	segmenter.LoadDictionary(dictFile)
	dictLoaded = true
}
