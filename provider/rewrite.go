package provider

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

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
	PatternPlace        = "place"        // 地点
	PatternPlaceIndex   = "placeIndex"   // 地点列表
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
	"{place}":        "([^\\/]+?)",
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

// cloneRewriteMode 复制一份规则表。
// 不能共享 rewrite*ModePattern 那些全局 map：补默认值、combine 展开、编译都会
// 原地写 Patterns，多个站点共用同一个 map 会互相污染（combine 展开还会重复插入）。
func cloneRewriteMode(m RewriteMode) RewriteMode {
	out := make(RewriteMode, len(m)+8)
	for k, v := range m {
		out[k] = v
	}
	return out
}

// buildRewritePattern 按站点当前配置构建一份全新的规则表（尚未编译）。
// 只写局部变量，不触碰已发布状态；由 rewriteMu 保护调用。
func (w *Website) buildRewritePattern() *RewritePattern {
	var p *RewritePattern
	switch w.PluginRewrite.Mode {
	case config.RewriteNumberMode:
		p = &RewritePattern{Patterns: cloneRewriteMode(rewriteNumberModePattern), Rules: RewriteMode{}, Tags: RewriteTag{}}
	case config.RewriteStringMode1:
		p = &RewritePattern{Patterns: cloneRewriteMode(rewriteStringMode1Pattern), Rules: RewriteMode{}, Tags: RewriteTag{}}
	case config.RewriteStringMode2:
		p = &RewritePattern{Patterns: cloneRewriteMode(rewriteStringMode2Pattern), Rules: RewriteMode{}, Tags: RewriteTag{}}
	case config.RewriteStringMode3:
		p = &RewritePattern{Patterns: cloneRewriteMode(rewriteStringMode3Pattern), Rules: RewriteMode{}, Tags: RewriteTag{}}
	case config.RewritePatternMode:
		p = parseRewritePattern(w.PluginRewrite.Patten) // 原来拼写错误的单词，不改
	default:
		// 未知模式不能返回 nil，否则下面补默认值会 nil 解引用。
		p = &RewritePattern{Patterns: RewriteMode{}, Rules: RewriteMode{}, Tags: RewriteTag{}}
	}
	// 如果没有填写tag的规则，则给一个默认的
	if p.Patterns[PatternTagIndex] == "" {
		p.Patterns[PatternTagIndex] = "/tags(/{page})"
	}
	if p.Patterns[PatternTag] == "" {
		p.Patterns[PatternTag] = "/tag/{id}(/{page})"
	}
	if p.Patterns[PatternPeople] == "" {
		p.Patterns[PatternPeople] = "/people/{id}.html"
	}
	if p.Patterns[PatternPeopleIndex] == "" {
		p.Patterns[PatternPeopleIndex] = "/peoples(/{page})"
	}
	if p.Patterns[PatternSearch] == "" {
		p.Patterns[PatternSearch] = "/search(/{module})"
	}
	if p.Patterns[PatternPlaceIndex] == "" {
		p.Patterns[PatternPlaceIndex] = "/cities(/{page})"
	}
	if p.Patterns[PatternPlace] == "" {
		p.Patterns[PatternPlace] = "/{filename}(/{page})"
	}
	// 强制加page
	if !strings.Contains(p.Patterns[PatternArchive], "{page}") {
		if strings.HasSuffix(p.Patterns[PatternArchive], ".html") {
			strings.ReplaceAll(p.Patterns[PatternArchive], ".html", "(_{page}).html")
		} else if strings.HasSuffix(p.Patterns[PatternArchive], "/") {
			p.Patterns[PatternArchive] = strings.TrimRight(p.Patterns[PatternArchive], "/") + "(_{page})/"
		} else {
			p.Patterns[PatternArchive] += "(/{page})"
		}
	}
	return p
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

// compileRewritePattern 把 Patterns 编译成 Tags/Rules，并在最后标记 Parsed。
// 所有写入都在发布之前完成；发布后的 *RewritePattern 一律只读。
func compileRewritePattern(p *RewritePattern) {
	if p.Parsed {
		return
	}

	// archive 支持combine
	if strings.Contains(p.Patterns["archive"], "{id}") {
		p.Patterns["archive"] = strings.Replace(p.Patterns["archive"], "{id}", "{id}(/c-{combine})", 1)
	} else if strings.Contains(p.Patterns["archive"], "{filename}") {
		p.Patterns["archive"] = strings.Replace(p.Patterns["archive"], "{filename}", "{filename}(/c-{combine})", 1)
	}

	for key, item := range p.Patterns {
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
				if p.Tags[key] == nil {
					p.Tags[key] = make(map[int]string)
				}
				p.Tags[key][n] = str
				//重置
				str = ""
			} else if str != "" {
				str += string(v)
			}
		}
	}

	//移除首个 /
	rep, _ := regexp.Compile(`\{.+?}`)
	for k, v := range p.Patterns {
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
		p.Rules[k] = fmt.Sprintf("^%s$", rule)
	}

	//标记替换过（必须在所有字段写完之后，作为发布前的最后一步）
	p.Parsed = true
}

// loadParsedPattern 原子读取已发布的重写规则快照。
//
// 这里用 atomic.LoadPointer 而不是 atomic.Pointer 字段，是因为 Website 会被
// cloneWithContext 整体值拷贝（cloned := *w）：atomic.Pointer 带 noCopy，会触发
// copylocks；普通指针字段配原子访问既保留拷贝语义，也让读热路径彻底无锁。
func (w *Website) loadParsedPattern() *RewritePattern {
	return (*RewritePattern)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&w.parsedPattern))))
}

// storeParsedPattern 原子发布一份完整的快照。快照一经发布即不可变。
func (w *Website) storeParsedPattern(p *RewritePattern) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&w.parsedPattern)), unsafe.Pointer(p))
}

// 包级通用锁，仍被 GetCacheCategories/GetCachePlaces/翻译缓存等使用；
// 重写规则已不再用它，改由下面只服务慢路径的 rewriteMu 承担。
var mu sync.Mutex

// rewriteMu 只保护「构建/热更新」这条慢路径，读路径完全不碰它。
var rewriteMu sync.Mutex

// GetRewritePattern 返回已编译、不可变的重写规则快照。
// focus=false 且快照已就绪时只做一次原子加载，不拿锁；只有首次构建或
// focus=true（后台改了重写规则）才进慢路径重建并整体替换。
func (w *Website) GetRewritePattern(focus bool) *RewritePattern {
	if p := w.loadParsedPattern(); p != nil && !focus {
		return p
	}

	rewriteMu.Lock()
	defer rewriteMu.Unlock()
	// 双重检查：等锁期间可能已有别的请求把快照发布了。
	if p := w.loadParsedPattern(); p != nil && !focus {
		return p
	}

	p := w.buildRewritePattern()
	compileRewritePattern(p)
	w.storeParsedPattern(p)
	// 请求里拿到的是 cloneWithContext 副本，解析结果要回写到注册表对象，
	// 后续请求的副本才会一开始就带上这份快照。
	if reg := GetWebsite(w.Id); reg != nil && reg != w {
		reg.storeParsedPattern(p)
	}
	return p
}

// ParsePattern 返回已编译、不可变的重写规则快照。
// 读路径（focus=false 且快照已就绪）只做一次原子加载，不持有任何锁。
func (w *Website) ParsePattern(focus bool) *RewritePattern {
	if !focus {
		if p := w.loadParsedPattern(); p != nil && p.Parsed {
			return p
		}
	}
	return w.GetRewritePattern(focus)
}
