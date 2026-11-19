package provider

import (
	"strings"

	"github.com/huichen/sego"
	"kandaoni.com/anqicms/provider/segmenter"
)

const removeWord = " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~。？！，、；：“ ” ‘ ’「」『』（）〔〕【】《》〈〉—…·～"

// WordSplit 用进程级共享的分词器切词。
//
// 词典由 provider/segmenter 统一加载：进程内只有一份（sync.Once），
// 与 provider/fulltext 的悟空引擎共用同一份内存。
//
// ⚠️ 三个历史坑，改这里之前先读：
//  1. 过去是 `if !dictLoaded { initDict() }`：106 个站点的初始化 goroutine
//     同时看到 false，各自 LoadDictionary 一遍（官方词典实测每份堆常驻 +124MB，
//     100 个站点就是十个 GB 级），而且 dictLoaded 是无保护的 bool，
//     本身就是 data race。
//  2. sego 的 LoadDictionary 在词典文件打不开时是 log.Fatalf（os.Exit），
//     recover 拦不住 —— 别把"加载失败"寄希望于 defer recover。
//  3. 词典是只读的、全站共享，谁都不能在运行期 Close 它（见 setting.go 的
//     DeleteCache 注释）：Close 会把 dict.trie 置 nil，下一次分词直接
//     nil 解引用 panic，而且是"别的站点"的请求在 panic。
func WordSplit(s string, contain bool) []string {
	seg := segmenter.Shared()
	if seg == nil {
		// 词典不可用：整串返回。分词失败不该让请求 500，
		// 但也不能 nil 解引用（segmenter.StatusNow().Error 里有原因）。
		return []string{s}
	}
	segments := seg.Segment([]byte(s))

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
