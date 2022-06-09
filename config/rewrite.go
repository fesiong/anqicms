package config

import (
	"fmt"
	"strings"
	"sync"
)

const (
	RewriteNumberMode = 0 //数字模式
	RewriteStringMode = 1 //命名模式
	RewriteTinyMode   = 2 //极简数字模式
	RewritePattenMode = 3 //正则模式
)

type PluginRewriteConfig struct {
	Mode   int    `json:"mode"`
	Patten string `json:"patten"`
}

type RewritePatten struct {
	Archive      string `json:"archive"`
	Category     string `json:"category"`
	ArchiveIndex string `json:"archive_index"`
	Page         string `json:"page"`
	TagIndex     string `json:"tag_index"`
	Tag          string `json:"tag"`

	ArchiveRule      string
	CategoryRule     string
	PageRule         string
	ArchiveIndexRule string
	TagIndexRule     string
	TagRule          string

	ArchiveTags      map[int]string
	CategoryTags     map[int]string
	PageTags         map[int]string
	ArchiveIndexTags map[int]string
	TagIndexTags     map[int]string
	TagTags          map[int]string

	Parsed bool
}

var rewriteNumberModePatten = RewritePatten{
	Archive:      "/{module}/{id}.html",
	Category:     "/{module}/{id}(/{page})",
	Page:         "/{id}.html",
	ArchiveIndex: "/{module}",
	TagIndex:     "/tags(/{page})",
	Tag:          "/tag/{id}(/{page})",
}

var rewriteStringModePatten = RewritePatten{
	Archive:      "/{module}/{filename}.html",
	Category:     "/{module}/{filename}(/{page})",
	Page:         "/{filename}.html",
	ArchiveIndex: "/{module}",
	TagIndex:     "/tags(/{page})",
	Tag:          "/tag/{filename}(/{page})",
}

var rewriteTinyModePatten = RewritePatten{
	Archive:      "/{module}_{id}.html",
	Category:     "/{module}_{id}(_{page})",
	Page:         "/{id}.html",
	ArchiveIndex: "/{module}",
	TagIndex:     "/tags(_{page})",
	Tag:          "/tag_{id}(_{page})",
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
	"{id}":       "([\\d]+)",
	"{filename}": "([^\\/\\.\\_]+)",
	"{catname}":  "([^\\/\\.\\_]+)",
	"{module}":  "([^\\/\\.\\_]+)",
	"{catid}":    "([\\d]+)",
	"{page}":     "([\\d]+)",
}

var parsedPatten *RewritePatten

func GetRewritePatten(focus bool) *RewritePatten {
	if parsedPatten != nil && !focus {
		return parsedPatten
	}
	if JsonData.PluginRewrite.Mode == RewriteNumberMode {
		parsedPatten = &rewriteNumberModePatten
	} else if JsonData.PluginRewrite.Mode == RewriteStringMode {
		parsedPatten = &rewriteStringModePatten
	} else if JsonData.PluginRewrite.Mode == RewriteTinyMode {
		parsedPatten = &rewriteTinyModePatten
	} else if JsonData.PluginRewrite.Mode == RewritePattenMode {
		parsedPatten = parseRewritePatten(JsonData.PluginRewrite.Patten)
	}

	return parsedPatten
}

// 只有 RewritePattenMode 模式下，才需要解析
// 一共4行,分别是文章详情、产品详情、分类、页面,===和前面部分不可修改。
// 变量由花括号包裹{},如{id}。可用的变量有:数据ID {id}、数据自定义链接名 {filename}、分类自定义链接名 {catname}、分类ID {catid},分页ID {page}，分页需要使用()处理，用来首页忽略。如：(/{page})或(_{page})
func parseRewritePatten(patten string) *RewritePatten {
	parsedPatten := &RewritePatten{}
	// 再解开
	pattenSlice := strings.Split(patten, "\n")
	for _, v := range pattenSlice {
		singlePatten := strings.Split(v, "===")
		if len(singlePatten) == 2 {
			val := strings.TrimSpace(singlePatten[1])

			switch strings.TrimSpace(singlePatten[0]) {
			case "archive":
				parsedPatten.Archive = val
			case "category":
				parsedPatten.Category = val
			case "page":
				parsedPatten.Page = val
			case "archiveIndex":
				parsedPatten.ArchiveIndex = val
			case "tagIndex":
				parsedPatten.TagIndex = val
			case "tag":
				parsedPatten.Tag = val
			}
		}
	}
	// 如果没有填写tag的规则，则给一个默认的
	if parsedPatten.TagIndex == "" {
		parsedPatten.TagIndex = "/tags(/{page})"
	}
	if parsedPatten.Tag == "" {
		parsedPatten.Tag = "/tag/{id}(/{page})"
	}

	return parsedPatten
}

var mu sync.Mutex

func ParsePatten(focus bool) *RewritePatten {
	mu.Lock()
	GetRewritePatten(focus)
	if parsedPatten.Parsed {
		mu.Unlock()
		return parsedPatten
	}

	parsedPatten.ArchiveTags = map[int]string{}
	parsedPatten.CategoryTags = map[int]string{}
	parsedPatten.PageTags = map[int]string{}
	parsedPatten.ArchiveIndexTags = map[int]string{}
	parsedPatten.TagIndexTags = map[int]string{}
	parsedPatten.TagTags = map[int]string{}

	pattens := map[string]string{
		"archive":      parsedPatten.Archive,
		"category":     parsedPatten.Category,
		"page":         parsedPatten.Page,
		"archiveIndex": parsedPatten.ArchiveIndex,
		"tagIndex":     parsedPatten.TagIndex,
		"tag":          parsedPatten.Tag,
	}

	for key, item := range pattens {
		n := 0
		str := ""
		for _, v := range item {
			if v == '{' {
				n++
				str += string(v)
			} else if v == '}' {
				str = strings.TrimLeft(str, "{")
				if str == "page" {
					//page+1
					n++
				}
				switch key {
				case "archive":
					parsedPatten.ArchiveTags[n] = str
				case "category":
					parsedPatten.CategoryTags[n] = str
				case "page":
					parsedPatten.PageTags[n] = str
				case "archiveIndex":
					parsedPatten.ArchiveIndexTags[n] = str
				case "tagIndex":
					parsedPatten.TagIndexTags[n] = str
				case "tag":
					parsedPatten.TagTags[n] = str
				}
				//重置
				str = ""
			} else if str != "" {
				str += string(v)
			}
		}
	}

	//移除首个 /
	parsedPatten.ArchiveRule = strings.TrimLeft(parsedPatten.Archive, "/")
	parsedPatten.CategoryRule = strings.TrimLeft(parsedPatten.Category, "/")
	parsedPatten.PageRule = strings.TrimLeft(parsedPatten.Page, "/")
	parsedPatten.ArchiveIndexRule = strings.TrimLeft(parsedPatten.ArchiveIndex, "/")
	parsedPatten.TagIndexRule = strings.TrimLeft(parsedPatten.TagIndex, "/")
	parsedPatten.TagRule = strings.TrimLeft(parsedPatten.Tag, "/")

	for _, r := range needReplace {
		if strings.Contains(parsedPatten.ArchiveRule, r.Key) {
			parsedPatten.ArchiveRule = strings.ReplaceAll(parsedPatten.ArchiveRule, r.Key, r.Value)
		}
		if strings.Contains(parsedPatten.CategoryRule, r.Key) {
			parsedPatten.CategoryRule = strings.ReplaceAll(parsedPatten.CategoryRule, r.Key, r.Value)
		}
		if strings.Contains(parsedPatten.PageRule, r.Key) {
			parsedPatten.PageRule = strings.ReplaceAll(parsedPatten.PageRule, r.Key, r.Value)
		}
		if strings.Contains(parsedPatten.ArchiveIndexRule, r.Key) {
			parsedPatten.ArchiveIndexRule = strings.ReplaceAll(parsedPatten.ArchiveIndexRule, r.Key, r.Value)
		}
		if strings.Contains(parsedPatten.TagIndexRule, r.Key) {
			parsedPatten.TagIndexRule = strings.ReplaceAll(parsedPatten.TagIndexRule, r.Key, r.Value)
		}
		if strings.Contains(parsedPatten.TagRule, r.Key) {
			parsedPatten.TagRule = strings.ReplaceAll(parsedPatten.TagRule, r.Key, r.Value)
		}
	}

	for s, r := range replaceParams {
		if strings.Contains(parsedPatten.ArchiveRule, s) {
			parsedPatten.ArchiveRule = strings.ReplaceAll(parsedPatten.ArchiveRule, s, r)
		}
		if strings.Contains(parsedPatten.CategoryRule, s) {
			parsedPatten.CategoryRule = strings.ReplaceAll(parsedPatten.CategoryRule, s, r)
		}
		if strings.Contains(parsedPatten.PageRule, s) {
			parsedPatten.PageRule = strings.ReplaceAll(parsedPatten.PageRule, s, r)
		}
		if strings.Contains(parsedPatten.ArchiveIndexRule, s) {
			parsedPatten.ArchiveIndexRule = strings.ReplaceAll(parsedPatten.ArchiveIndexRule, s, r)
		}
		if strings.Contains(parsedPatten.TagIndexRule, s) {
			parsedPatten.TagIndexRule = strings.ReplaceAll(parsedPatten.TagIndexRule, s, r)
		}
		if strings.Contains(parsedPatten.TagRule, s) {
			parsedPatten.TagRule = strings.ReplaceAll(parsedPatten.TagRule, s, r)
		}
	}
	//修改为强制包裹
	parsedPatten.ArchiveRule = fmt.Sprintf("^%s$", parsedPatten.ArchiveRule)
	parsedPatten.CategoryRule = fmt.Sprintf("^%s$", parsedPatten.CategoryRule)
	parsedPatten.PageRule = fmt.Sprintf("^%s$", parsedPatten.PageRule)
	parsedPatten.ArchiveIndexRule = fmt.Sprintf("^%s$", parsedPatten.ArchiveIndexRule)
	parsedPatten.TagIndexRule = fmt.Sprintf("^%s$", parsedPatten.TagIndexRule)
	parsedPatten.TagRule = fmt.Sprintf("^%s$", parsedPatten.TagRule)

	//标记替换过
	parsedPatten.Parsed = true
	mu.Unlock()

	return parsedPatten
}
