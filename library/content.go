package library

import (
	"bytes"
	"fmt"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/kataras/iris/v12"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"
)

func StripTags(src string) string {
	//将HTML标签全转换成小写
	re, _ := regexp.Compile("\\<[\\S\\s]+?\\>")
	src = re.ReplaceAllStringFunc(src, strings.ToLower)
	//去除STYLE
	re, _ = regexp.Compile("\\<style[\\S\\s]+?\\</style\\>")
	src = re.ReplaceAllString(src, "")
	//去除SCRIPT
	re, _ = regexp.Compile("\\<script[\\S\\s]+?\\</script\\>")
	src = re.ReplaceAllString(src, "")
	//去除所有尖括号内的HTML代码，并换成换行符
	re, _ = regexp.Compile("\\<[\\S\\s]+?\\>")
	src = re.ReplaceAllString(src, "\n")
	//去除连续的换行符
	re, _ = regexp.Compile("\\s{2,}")
	src = re.ReplaceAllString(src, "\n")
	return strings.TrimSpace(src)
}

func Case2Camel(name string) string {
	name = strings.Replace(name, "_", " ", -1)
	name = strings.Title(name)
	return strings.Replace(name, " ", "", -1)
}

func ParseUrlToken(name string) string {
	if name == "" {
		return name
	}
	name = strings.ToLower(name)
	name = strings.Replace(name, " ", "-", -1)
	name = strings.Replace(name, "_", "-", -1)
	if name == "" {
		return name
	}
	names := []rune(name)
	for i := 0; i < len(names); i++ {
		if (names[i] >= 48 && names[i] <= 57) || (names[i] >= 97 && names[i] <= 122) || names[i] == 45 {
			// 这个范围是对的
		} else {
			// 需要删除
			names = append(names[:i], names[i+1:]...)
			i--
		}
	}
	//去除连续的换行符
	re, _ := regexp.Compile("-{2,}")
	name = re.ReplaceAllString(name, "-")
	if len(name) > 150 {
		name = name[:150]
	}
	return name
}

func IsNumericEnding(s string) bool {
	// 使用正则表达式判断是否以数字结尾
	re := regexp.MustCompile(`\d+$`)
	return re.MatchString(s)
}

func ReplaceSingleSpace(content string) string {
	// 将单个&nbsp;替换为空格
	re, _ := regexp.Compile(`(&nbsp;|\xA0)+`)
	content = re.ReplaceAllStringFunc(content, func(s string) string {
		if s == "&nbsp;" || s == "\xA0" {
			return " "
		}
		return s
	})

	return content
}

// BytesToString casts slice to string without copy
func BytesToString(b []byte) (s string) {
	if len(b) == 0 {
		return ""
	}

	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := reflect.StringHeader{Data: bh.Data, Len: bh.Len}

	return *(*string)(unsafe.Pointer(&sh))
}

// StringToBytes casts string to slice without copy
func StringToBytes(s string) []byte {
	if len(s) == 0 {
		return []byte{}
	}

	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{Data: sh.Data, Len: sh.Len, Cap: sh.Len}

	return *(*[]byte)(unsafe.Pointer(&bh))
}

func EscapeString(v string) string {
	var pos = 0
	if len(v) == 0 {
		return ""
	}
	buf := make([]byte, len(v[:])*2)
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch c {
		case '\x00':
			buf[pos] = '\\'
			buf[pos+1] = '0'
			pos += 2
		case '\n':
			buf[pos] = '\\'
			buf[pos+1] = 'n'
			pos += 2
		case '\r':
			buf[pos] = '\\'
			buf[pos+1] = 'r'
			pos += 2
		case '\x1a':
			buf[pos] = '\\'
			buf[pos+1] = 'Z'
			pos += 2
		case '\'':
			buf[pos] = '\\'
			buf[pos+1] = '\''
			pos += 2
		case '"':
			buf[pos] = '\\'
			buf[pos+1] = '"'
			pos += 2
		case '\\':
			buf[pos] = '\\'
			buf[pos+1] = '\\'
			pos += 2
		default:
			buf[pos] = c
			pos++
		}
	}
	return string(buf[:pos])
}

func GetHost(ctx iris.Context) string {
	// maybe real host in X-Host
	host := ctx.GetHeader("X-Host")
	if host == "" {
		host = ctx.Host()
	}
	// remove port from host
	if tmp, _, err := net.SplitHostPort(host); err == nil {
		host = tmp
	}

	switch host {
	// We could use the netutil.LoopbackRegex but leave it as it's for now, it's faster.
	case "localhost", "127.0.0.1", "0.0.0.0", "::1", "[::1]", "0:0:0:0:0:0:0:0", "0:0:0:0:0:0:0:1":
		// loopback.
		return "localhost"
	default:
		return host
	}
}

// ParseDescription 对于超过250字的描述，截取的时候，以标点符号为准
func ParseDescription(content string) (description string) {
	if utf8.RuneCountInString(content) > 200 {
		content = string([]rune(content)[:200])
	}
	lastIndex := strings.LastIndexAny(content, " !,.:;?~。？！，、；：…")
	if lastIndex >= 150 {
		description = content[:lastIndex]
	} else {
		description = content
	}

	return
}

// MarkdownToHTML 将markdown转换为html
// args[0] = baseUrl
// args[1] = filterOutLink
func MarkdownToHTML(mdStr string, args ...interface{}) string {
	if len(mdStr) == 0 {
		return ""
	}
	// 换行转换成p
	//mdStr = strings.ReplaceAll(mdStr, "\n", "  \n")
	md := []byte(mdStr)
	// create Markdown parser with extensions
	extensions := parser.CommonExtensions | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	// create HTML renderer with extensions
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	md = markdown.Render(doc, renderer)
	// 不转换 mermaid 的 code
	re, _ := regexp.Compile(`(?is)<pre><code class="language-mermaid">(.*?)</code></pre>`)
	md = re.ReplaceAllFunc(md, func(bs []byte) []byte {
		match := re.FindSubmatch(bs)
		if len(match) < 2 {
			return bs
		}
		buff := bytes.NewBuffer(nil)
		buff.WriteString("<pre class=\"mermaid\">")
		buff.Write(match[1])
		buff.WriteString("</pre>")
		return buff.Bytes()
	})
	if len(args) == 2 {
		baseUrl, _ := args[0].(string)
		filterOutLink, _ := args[1].(int)
		if filterOutLink == 2 {
			baseHost := ""
			urls, err := url.Parse(baseUrl)
			if err == nil {
				baseHost = urls.Host
			}
			// 添加 nofollow
			re, _ = regexp.Compile(`(?is)<a.*?href="(.+?)".*?>`)
			md = re.ReplaceAllFunc(md, func(bs []byte) []byte {
				match := re.FindSubmatch(bs)
				if len(match) < 2 {
					return bs
				}
				if bytes.HasPrefix(match[1], []byte("http")) || bytes.HasPrefix(match[1], []byte("//")) {
					aUrl, err2 := url.Parse(string(match[1]))
					if err2 == nil {
						if aUrl.Host != "" && aUrl.Host != baseHost {
							//过滤外链
							newUrl := append(match[1], []byte(`" rel="nofollow`)...)
							bs = bytes.Replace(bs, match[1], newUrl, 1)
						}
					}
				}

				return bs
			})
		}
	}

	return string(md)
}

type ContentTitle struct {
	Anchor   string          `json:"anchor"`
	Title    string          `json:"title"`
	Tag      string          `json:"tag"`
	Level    int             `json:"level"`
	Prefix   string          `json:"prefix"`
	Children []*ContentTitle `json:"children,omitempty"`
}

// ParseContentTitles 获取toc，showType = tree|children
func ParseContentTitles(content, showType string) ([]*ContentTitle, string) {
	re, _ := regexp.Compile(`(?is)<(h\d)([^>]*)>(.*?)</h\d>`)
	re2, _ := regexp.Compile(`<(h\d)([^>]*)>`)
	var titles []*ContentTitle
	var rootTitles []*ContentTitle
	var levelStack []int
	var prefix []int64
	content = re.ReplaceAllStringFunc(content, func(s string) string {
		match := re.FindStringSubmatch(s)
		tag := strings.ToLower(match[1])
		currentLevel, _ := strconv.Atoi(strings.TrimLeft(tag, "h"))

		// 计算标题层级关系
		if len(levelStack) == 0 {
			levelStack = append(levelStack, currentLevel)
		} else {
			// 如果当前标题等级更高（数字更小），移除栈中比它高的等级
			for len(levelStack) > 0 && currentLevel <= levelStack[len(levelStack)-1] {
				levelStack = levelStack[:len(levelStack)-1]
			}
			levelStack = append(levelStack, currentLevel)
		}

		// 构建前缀编号
		if len(levelStack) <= len(prefix) {
			prefix = prefix[:len(levelStack)]
			if len(prefix) > 0 {
				prefix[len(prefix)-1]++
			} else {
				prefix = append(prefix, 1)
			}
		} else {
			prefix = append(prefix, 1)
		}

		title := strings.TrimSpace(strings.ReplaceAll(StripTags(match[3]), "\n", " "))
		itemPrefix := JoinInt(prefix, ".")
		anchor := strings.ReplaceAll(itemPrefix, ".", "-") + "-" + GetPinyin(title, true)
		cTitle := &ContentTitle{
			Anchor:   "#" + anchor,
			Title:    title,
			Tag:      tag,
			Level:    len(levelStack),
			Prefix:   itemPrefix,
			Children: make([]*ContentTitle, 0),
		}

		// 根据展示类型构建结构
		if showType == "tree" {
			titles = append(titles, cTitle)
		} else { // children模式
			if len(levelStack) == 1 {
				// 顶级标题作为根节点
				rootTitles = append(rootTitles, cTitle)
				titles = append(titles, rootTitles[len(rootTitles)-1])
			} else {
				// 找到最近的父级标题并添加为子节点
				for i := len(titles) - 1; i >= 0; i-- {
					if titles[i].Level == len(levelStack)-1 {
						titles[i].Children = append(titles[i].Children, cTitle)
						break
					}
				}
				// 将当前标题也添加到扁平列表中，以便后续查找
				titles = append(titles, cTitle)
			}
		}

		// 替换原始HTML中的标题标签，添加锚点ID
		s = re2.ReplaceAllStringFunc(s, func(s2 string) string {
			if strings.Contains(s2, "id=\"") {
				// 先删除原始的id属性
				re3, _ := regexp.Compile(`(?is)id=".*?"`)
				s2 = re3.ReplaceAllString(s2, "")
			}

			return strings.TrimSuffix(s2, ">") + fmt.Sprintf(` id="%s">`, anchor)
		})

		return s
	})

	if showType != "tree" {
		return rootTitles, content
	}
	return titles, content
}

type NameVal struct {
	Name string
	Val  string
}

var SpiderNames = []NameVal{
	{Name: "googlebot", Val: "google"},
	{Name: "bingbot", Val: "bing"},
	{Name: "baiduspider", Val: "baidu"},
	{Name: "360spider", Val: "360"},
	{Name: "yahoo!", Val: "yahoo"},
	{Name: "sogou", Val: "sogou"},
	{Name: "bytespider", Val: "byte"},
	{Name: "yisouspider", Val: "yisou"},
	{Name: "yandexbot", Val: "yandex"},
	{Name: "spider", Val: "other"},
	{Name: "bot", Val: "other"},
}

var DeviceNames = []NameVal{
	{Name: "android", Val: "android"},
	{Name: "iphone", Val: "iphone"},
	{Name: "windows", Val: "windows"},
	{Name: "macintosh", Val: "mac"},
	{Name: "linux", Val: "linux"},
	{Name: "mobile", Val: "mobile"},
	{Name: "curl", Val: "curl"},
	{Name: "python", Val: "python"},
	{Name: "client", Val: "client"},
	{Name: "spider", Val: "spider"},
	{Name: "bot", Val: "spider"},
}

func GetSpider(ua string) string {
	ua = strings.ToLower(ua)
	//获取蜘蛛
	for _, v := range SpiderNames {
		if strings.Contains(ua, v.Name) {
			return v.Val
		}
	}

	return ""
}

func GetDevice(ua string) string {
	ua = strings.ToLower(ua)

	for _, v := range DeviceNames {
		if strings.Contains(ua, v.Name) {
			return v.Val
		}
	}

	return "proxy"
}
