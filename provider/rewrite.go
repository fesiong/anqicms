package provider

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"kandaoni.com/anqicms/config"
)

// 支持的字段有：
const (
	PatternArchive      = "archive"      // 文章详情
	PatternCategory     = "category"     // 分类列表
	PatternPage         = "page"         // 单页
	PatternArchiveIndex = "archiveIndex" // 列表页
	PatternTagIndex     = "tagIndex"     // 标签列表
	PatternTag          = "tag"          // 标签详情
	PatternPeople       = "people"       // 用户
	PatternPeopleIndex  = "peopleIndex"  // 用户列表
	PatternSearch       = "search"       // 搜索
	PatternCommon       = "common"
)

type RewriteMode map[string]string

type RewriteTag map[string]map[int]string

type RewritePattern struct {
	Patterns RewriteMode
	Rules    RewriteMode
	Tags     RewriteTag

	Parsed bool
}

var rewriteNumberModePattern = RewriteMode{
	PatternArchive:      "/{module}/{id}(_{page}).html",
	PatternCategory:     "/{module}/{catid}(/{page})",
	PatternPage:         "/{id}.html",
	PatternArchiveIndex: "/{module}(_{page})",
	PatternTagIndex:     "/tags(/{page})",
	PatternTag:          "/tag/{id}(/{page})",
}

var rewriteStringMode1Pattern = RewriteMode{
	PatternArchive:      "/{module}/{filename}(_{page}).html",
	PatternCategory:     "/{module}/{catname}(/{page})",
	PatternPage:         "/{filename}.html",
	PatternArchiveIndex: "/{module}(_{page})",
	PatternTagIndex:     "/tags(/{page})",
	PatternTag:          "/tag/{filename}(/{page})",
}

var rewriteStringMode2Pattern = RewriteMode{
	PatternArchive:      "/{catname}/{id}(_{page}).html",
	PatternCategory:     "/{catname}(/{page})",
	PatternPage:         "/{filename}.html",
	PatternArchiveIndex: "/{module}(_{page})",
	PatternTagIndex:     "/tags(/{page})",
	PatternTag:          "/tag/{id}(/{page})",
}

var rewriteStringMode3Pattern = RewriteMode{
	PatternArchive:      "/{catname}/{filename}(_{page}).html",
	PatternCategory:     "/{catname}(/{page})",
	PatternPage:         "/{filename}.html",
	PatternArchiveIndex: "/{module}(_{page})",
	PatternTagIndex:     "/tags(/{page})",
	PatternTag:          "/tag/{filename}(/{page})",
}

type replaceChar struct {
	Key   string
	Value string
}

var needReplace = []replaceChar{
	{Key: "/", Value: "\\/"},
	{Key: "*", Value: "\\*"},
	{Key: "+", Value: "\\+"},
	{Key: "?", Value: "\\?"},
	{Key: ".", Value: "\\."},
	{Key: "-", Value: "\\-"},
	{Key: "[", Value: "\\["},
	{Key: "]", Value: "\\]"},
	{Key: ")", Value: ")?"}, //fix?  map无序，可能会出现?混乱
}

var replaceParams = map[string]string{
	"{id}":           "([\\d]+)",
	"{filename}":     "([^\\/]+?)",
	"{catname}":      "([^\\/]+?)",
	"{multicatname}": "(.+?)",
	"{module}":       "([^\\/]+?)",
	"{catid}":        "([\\d]+)",
	"{year}":         "([\\d]{4})",
	"{month}":        "([\\d]{2})",
	"{day}":          "([\\d]{2})",
	"{hour}":         "([\\d]{2})",
	"{minute}":       "([\\d]{2})",
	"{second}":       "([\\d]{2})",
	"{page}":         "([\\d]+)",
	"{combine}":      "([^\\/]+?)",
	// any but not slash
	"{any}": "([^\\/]+?)",
}

func (w *Website) GetRewritePattern(focus bool) *RewritePattern {
	if w.parsedPattern != nil && !focus {
		return w.parsedPattern
	}
	if w.PluginRewrite.Mode == config.RewriteNumberMode {
		w.parsedPattern = &RewritePattern{
			Patterns: rewriteNumberModePattern,
			Rules:    RewriteMode{},
			Tags:     RewriteTag{},
		}
	} else if w.PluginRewrite.Mode == config.RewriteStringMode1 {
		w.parsedPattern = &RewritePattern{
			Patterns: rewriteStringMode1Pattern,
			Rules:    RewriteMode{},
			Tags:     RewriteTag{},
		}
	} else if w.PluginRewrite.Mode == config.RewriteStringMode2 {
		w.parsedPattern = &RewritePattern{
			Patterns: rewriteStringMode2Pattern,
			Rules:    RewriteMode{},
			Tags:     RewriteTag{},
		}
	} else if w.PluginRewrite.Mode == config.RewriteStringMode3 {
		w.parsedPattern = &RewritePattern{
			Patterns: rewriteStringMode3Pattern,
			Rules:    RewriteMode{},
			Tags:     RewriteTag{},
		}
	} else if w.PluginRewrite.Mode == config.RewritePatternMode {
		w.parsedPattern = parseRewritePattern(w.PluginRewrite.Patten) // 原来拼写错误的单词，不改
	}
	// 如果没有填写tag的规则，则给一个默认的
	if w.parsedPattern.Patterns[PatternTagIndex] == "" {
		w.parsedPattern.Patterns[PatternTagIndex] = "/tags(/{page})"
	}
	if w.parsedPattern.Patterns[PatternTag] == "" {
		w.parsedPattern.Patterns[PatternTag] = "/tag/{id}(/{page})"
	}
	if w.parsedPattern.Patterns[PatternPeople] == "" {
		w.parsedPattern.Patterns[PatternPeople] = "/people/{id}.html"
	}
	if w.parsedPattern.Patterns[PatternPeopleIndex] == "" {
		w.parsedPattern.Patterns[PatternPeopleIndex] = "/peoples(/{page})"
	}
	if w.parsedPattern.Patterns[PatternSearch] == "" {
		w.parsedPattern.Patterns[PatternSearch] = "/search(/{module})"
	}
	// 强制加page
	if !strings.Contains(w.parsedPattern.Patterns[PatternArchive], "{page}") {
		if strings.HasSuffix(w.parsedPattern.Patterns[PatternArchive], ".html") {
			strings.ReplaceAll(w.parsedPattern.Patterns[PatternArchive], ".html", "(_{page}).html")
		} else if strings.HasSuffix(w.parsedPattern.Patterns[PatternArchive], "/") {
			w.parsedPattern.Patterns[PatternArchive] = strings.TrimRight(w.parsedPattern.Patterns[PatternArchive], "/") + "(_{page})/"
		} else {
			w.parsedPattern.Patterns[PatternArchive] += "(/{page})"
		}
	}
	w2 := GetWebsite(w.Id)
	w2.parsedPattern = w.parsedPattern
	return w.parsedPattern
}

// 只有 RewritePatternMode 模式下，才需要解析
// 一共4行,分别是文章详情、产品详情、分类、页面,===和前面部分不可修改。
// 变量由花括号包裹{},如{id}。可用的变量有:数据ID {id}、数据自定义链接名 {filename}、分类自定义链接名 {catname}、分类ID {catid},分页ID {page}，分页需要使用()处理，用来首页忽略。如：(/{page})或(_{page})
func parseRewritePattern(patten string) *RewritePattern {
	parsedPattern := &RewritePattern{
		Patterns: RewriteMode{},
		Rules:    RewriteMode{},
		Tags:     RewriteTag{},
	}
	// 再解开
	pattenSlice := strings.Split(patten, "\n")
	for _, v := range pattenSlice {
		singlePattern := strings.Split(v, "===")
		if len(singlePattern) == 2 {
			val := strings.TrimSpace(singlePattern[1])
			key := strings.TrimSpace(singlePattern[0])
			parsedPattern.Patterns[key] = val
		}
	}

	return parsedPattern
}

var mu sync.Mutex

func (w *Website) ParsePattern(focus bool) *RewritePattern {
	mu.Lock()
	defer mu.Unlock()
	w.GetRewritePattern(focus)
	if w.parsedPattern.Parsed {
		return w.parsedPattern
	}

	// archive 支持combine
	if strings.Contains(w.parsedPattern.Patterns["archive"], "{id}") {
		w.parsedPattern.Patterns["archive"] = strings.Replace(w.parsedPattern.Patterns["archive"], "{id}", "{id}(/c-{combine})", 1)
	} else if strings.Contains(w.parsedPattern.Patterns["archive"], "{filename}") {
		w.parsedPattern.Patterns["archive"] = strings.Replace(w.parsedPattern.Patterns["archive"], "{filename}", "{filename}(/c-{combine})", 1)
	}

	for key, item := range w.parsedPattern.Patterns {
		n := 0
		str := ""
		for _, v := range item {
			if v == '(' {
				n++
				continue
			}
			if v == '{' {
				n++
				str += string(v)
			} else if v == '}' {
				str = strings.TrimLeft(str, "{")
				if w.parsedPattern.Tags[key] == nil {
					w.parsedPattern.Tags[key] = make(map[int]string)
				}
				w.parsedPattern.Tags[key][n] = str
				//重置
				str = ""
			} else if str != "" {
				str += string(v)
			}
		}
	}

	//移除首个 /
	rep, _ := regexp.Compile(`\{.+?}`)
	for k, v := range w.parsedPattern.Patterns {
		rule := strings.TrimLeft(v, "/")
		for _, r := range needReplace {
			if strings.Contains(rule, r.Key) {
				rule = strings.ReplaceAll(rule, r.Key, r.Value)
			}
		}
		rule = rep.ReplaceAllStringFunc(rule, func(s string) string {
			if replaceParams[s] != "" {
				return replaceParams[s]
			} else {
				// any param
				return replaceParams["{any}"]
			}
		})
		//修改为强制包裹
		w.parsedPattern.Rules[k] = fmt.Sprintf("^%s$", rule)
	}

	//标记替换过
	w.parsedPattern.Parsed = true

	return w.parsedPattern
}
