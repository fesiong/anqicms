package library

import (
	"bytes"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/kataras/iris/v12"
	"net"
	"reflect"
	"regexp"
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

func MarkdownToHTML(mdStr string) string {
	md := []byte(mdStr)
	// create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
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

	return string(md)
}
