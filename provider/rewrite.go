package provider

import (
	"fmt"
	"kandaoni.com/anqicms/config"
	"strings"
	"sync"
)

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
	Archive:      "/{module}/{id}(_{page}).html",
	Category:     "/{module}/{catid}(/{page})",
	Page:         "/{id}.html",
	ArchiveIndex: "/{module}(_{page})",
	TagIndex:     "/tags(/{page})",
	Tag:          "/tag/{id}(/{page})",
}

var rewriteStringMode1Patten = RewritePatten{
	Archive:      "/{module}/{filename}(_{page}).html",
	Category:     "/{module}/{catname}(/{page})",
	Page:         "/{filename}.html",
	ArchiveIndex: "/{module}(_{page})",
	TagIndex:     "/tags(/{page})",
	Tag:          "/tag/{filename}(/{page})",
}

var rewriteStringMode2Patten = RewritePatten{
	Archive:      "/{catname}/{id}(_{page}).html",
	Category:     "/{catname}(/{page})",
	Page:         "/{filename}.html",
	ArchiveIndex: "/{module}(_{page})",
	TagIndex:     "/tags(/{page})",
	Tag:          "/tag/{id}(/{page})",
}

var rewriteStringMode3Patten = RewritePatten{
	Archive:      "/{catname}/{filename}(_{page}).html",
	Category:     "/{catname}(/{page})",
	Page:         "/{filename}.html",
	ArchiveIndex: "/{module}(_{page})",
	TagIndex:     "/tags(/{page})",
	Tag:          "/tag/{filename}(/{page})",
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
}

//var parsedPatten *RewritePatten

func (w *Website) GetRewritePatten(focus bool) *RewritePatten {
	if w.parsedPatten != nil && !focus {
		return w.parsedPatten
	}
	if w.PluginRewrite.Mode == config.RewriteNumberMode {
		w.parsedPatten = &rewriteNumberModePatten
	} else if w.PluginRewrite.Mode == config.RewriteStringMode1 {
		w.parsedPatten = &rewriteStringMode1Patten
	} else if w.PluginRewrite.Mode == config.RewriteStringMode2 {
		w.parsedPatten = &rewriteStringMode2Patten
	} else if w.PluginRewrite.Mode == config.RewriteStringMode3 {
		w.parsedPatten = &rewriteStringMode3Patten
	} else if w.PluginRewrite.Mode == config.RewritePattenMode {
		w.parsedPatten = parseRewritePatten(w.PluginRewrite.Patten)
	}
	// 强制加page
	if !strings.Contains(w.parsedPatten.Archive, "{page}") {
		if strings.HasSuffix(w.parsedPatten.ArchiveIndex, ".html") {
			strings.ReplaceAll(w.parsedPatten.ArchiveIndex, ".html", "(_{page}).html")
		} else if strings.HasSuffix(w.parsedPatten.ArchiveIndex, "/") {
			w.parsedPatten.ArchiveIndex = strings.TrimRight(w.parsedPatten.ArchiveIndex, "/") + "(_{page})/"
		} else {
			w.parsedPatten.ArchiveIndex += "(_{page})"
		}
	}
	w2 := GetWebsite(w.Id)
	w2.parsedPatten = w.parsedPatten
	return w.parsedPatten
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

func (w *Website) ParsePatten(focus bool) *RewritePatten {
	mu.Lock()
	defer mu.Unlock()
	w.GetRewritePatten(focus)
	if w.parsedPatten.Parsed {
		return w.parsedPatten
	}

	// archive 支持combine
	if strings.Contains(w.parsedPatten.Archive, "{id}") {
		w.parsedPatten.Archive = strings.Replace(w.parsedPatten.Archive, "{id}", "{id}(/c-{combine})", 1)
	} else if strings.Contains(w.parsedPatten.Archive, "{filename}") {
		w.parsedPatten.Archive = strings.Replace(w.parsedPatten.Archive, "{filename}", "{filename}(/c-{combine})", 1)
	}

	w.parsedPatten.ArchiveTags = map[int]string{}
	w.parsedPatten.CategoryTags = map[int]string{}
	w.parsedPatten.PageTags = map[int]string{}
	w.parsedPatten.ArchiveIndexTags = map[int]string{}
	w.parsedPatten.TagIndexTags = map[int]string{}
	w.parsedPatten.TagTags = map[int]string{}

	pattens := map[string]string{
		"archive":      w.parsedPatten.Archive,
		"category":     w.parsedPatten.Category,
		"page":         w.parsedPatten.Page,
		"archiveIndex": w.parsedPatten.ArchiveIndex,
		"tagIndex":     w.parsedPatten.TagIndex,
		"tag":          w.parsedPatten.Tag,
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
				if str == "page" || str == "combine" {
					//page+1
					n++
				}
				switch key {
				case "archive":
					w.parsedPatten.ArchiveTags[n] = str
				case "category":
					w.parsedPatten.CategoryTags[n] = str
				case "page":
					w.parsedPatten.PageTags[n] = str
				case "archiveIndex":
					w.parsedPatten.ArchiveIndexTags[n] = str
				case "tagIndex":
					w.parsedPatten.TagIndexTags[n] = str
				case "tag":
					w.parsedPatten.TagTags[n] = str
				}
				//重置
				str = ""
			} else if str != "" {
				str += string(v)
			}
		}
	}

	//移除首个 /
	w.parsedPatten.ArchiveRule = strings.TrimLeft(w.parsedPatten.Archive, "/")
	w.parsedPatten.CategoryRule = strings.TrimLeft(w.parsedPatten.Category, "/")
	w.parsedPatten.PageRule = strings.TrimLeft(w.parsedPatten.Page, "/")
	w.parsedPatten.ArchiveIndexRule = strings.TrimLeft(w.parsedPatten.ArchiveIndex, "/")
	w.parsedPatten.TagIndexRule = strings.TrimLeft(w.parsedPatten.TagIndex, "/")
	w.parsedPatten.TagRule = strings.TrimLeft(w.parsedPatten.Tag, "/")

	for _, r := range needReplace {
		if strings.Contains(w.parsedPatten.ArchiveRule, r.Key) {
			w.parsedPatten.ArchiveRule = strings.ReplaceAll(w.parsedPatten.ArchiveRule, r.Key, r.Value)
		}
		if strings.Contains(w.parsedPatten.CategoryRule, r.Key) {
			w.parsedPatten.CategoryRule = strings.ReplaceAll(w.parsedPatten.CategoryRule, r.Key, r.Value)
		}
		if strings.Contains(w.parsedPatten.PageRule, r.Key) {
			w.parsedPatten.PageRule = strings.ReplaceAll(w.parsedPatten.PageRule, r.Key, r.Value)
		}
		if strings.Contains(w.parsedPatten.ArchiveIndexRule, r.Key) {
			w.parsedPatten.ArchiveIndexRule = strings.ReplaceAll(w.parsedPatten.ArchiveIndexRule, r.Key, r.Value)
		}
		if strings.Contains(w.parsedPatten.TagIndexRule, r.Key) {
			w.parsedPatten.TagIndexRule = strings.ReplaceAll(w.parsedPatten.TagIndexRule, r.Key, r.Value)
		}
		if strings.Contains(w.parsedPatten.TagRule, r.Key) {
			w.parsedPatten.TagRule = strings.ReplaceAll(w.parsedPatten.TagRule, r.Key, r.Value)
		}
	}

	for s, r := range replaceParams {
		if strings.Contains(w.parsedPatten.ArchiveRule, s) {
			w.parsedPatten.ArchiveRule = strings.ReplaceAll(w.parsedPatten.ArchiveRule, s, r)
		}
		if strings.Contains(w.parsedPatten.CategoryRule, s) {
			w.parsedPatten.CategoryRule = strings.ReplaceAll(w.parsedPatten.CategoryRule, s, r)
		}
		if strings.Contains(w.parsedPatten.PageRule, s) {
			w.parsedPatten.PageRule = strings.ReplaceAll(w.parsedPatten.PageRule, s, r)
		}
		if strings.Contains(w.parsedPatten.ArchiveIndexRule, s) {
			w.parsedPatten.ArchiveIndexRule = strings.ReplaceAll(w.parsedPatten.ArchiveIndexRule, s, r)
		}
		if strings.Contains(w.parsedPatten.TagIndexRule, s) {
			w.parsedPatten.TagIndexRule = strings.ReplaceAll(w.parsedPatten.TagIndexRule, s, r)
		}
		if strings.Contains(w.parsedPatten.TagRule, s) {
			w.parsedPatten.TagRule = strings.ReplaceAll(w.parsedPatten.TagRule, s, r)
		}
	}
	//修改为强制包裹
	w.parsedPatten.ArchiveRule = fmt.Sprintf("^%s$", w.parsedPatten.ArchiveRule)
	w.parsedPatten.CategoryRule = fmt.Sprintf("^%s$", w.parsedPatten.CategoryRule)
	w.parsedPatten.PageRule = fmt.Sprintf("^%s$", w.parsedPatten.PageRule)
	w.parsedPatten.ArchiveIndexRule = fmt.Sprintf("^%s$", w.parsedPatten.ArchiveIndexRule)
	w.parsedPatten.TagIndexRule = fmt.Sprintf("^%s$", w.parsedPatten.TagIndexRule)
	w.parsedPatten.TagRule = fmt.Sprintf("^%s$", w.parsedPatten.TagRule)

	//标记替换过
	w.parsedPatten.Parsed = true

	return w.parsedPatten
}
