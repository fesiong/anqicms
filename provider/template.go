package provider

import (
	"bytes"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"os"
	"regexp"
	"strconv"
	"sync"
)

type StoreTemplates struct {
	Templates map[string]int64
	mu        sync.Mutex
}

func (w *Website) SetTemplates(templates map[string]int64) {
	if w.Template == nil {
		return
	}
	w.Template.mu.Lock()
	defer w.Template.mu.Unlock()
	w.Template.Templates = templates
}

func (w *Website) TemplateExist(tplPaths ...string) (string, bool) {
	if len(tplPaths) == 0 {
		return "", false
	}
	if w.Template == nil {
		return tplPaths[0], false
	}
	w.Template.mu.Lock()
	defer w.Template.mu.Unlock()
	for _, tplPath := range tplPaths {
		if tplPath == "" {
			continue
		}
		if _, ok := w.Template.Templates[tplPath]; ok {
			return tplPath, true
		}
	}

	return tplPaths[0], false
}

// GetTemplate 获取 template 的具体内容
func (w *Website) GetTemplate(tplPath string) (string, bool) {
	if w.Template == nil {
		return "", false
	}
	w.Template.mu.Lock()
	defer w.Template.mu.Unlock()
	if _, ok := w.Template.Templates[tplPath]; ok {
		fullPath := w.GetTemplateDir() + "/" + tplPath
		buf, err := os.ReadFile(fullPath)
		return string(buf), err == nil
	}

	return "", false
}

// RenderTemplateMacro 渲染模板, 返回 content, showContentTitle
func (w *Website) RenderTemplateMacro(content string, ctx pongo2.Context) (string, bool) {
	// 编辑器内容区支持 @的方式来调用特定模板宏函数
	// 格式：{{ @宏函数名称(变量) "宏文件路径" }}
	// {{@toc}} 将被渲染成 目录
	showContentTitle := false
	re2, _ := regexp.Compile(`\{\{\s*@(.*?)(\(.*?\))?(\s+"(.*?)")?\s*}}`)
	content = re2.ReplaceAllStringFunc(content, func(s string) string {
		match := re2.FindStringSubmatch(s)
		if match[1] == "toc" {
			showContentTitle = true
			toc, _ := library.ParseContentTitles(content, "list")
			if match[4] != "" {
				// 有指定宏地址
				ctx["tocList"] = toc
				// 先加载模板
				tplBuf, exist := w.GetTemplate(match[4])
				if exist {
					tpl := tplBuf + "\n{{ " + match[1] + "(tocList) }}"
					// 渲染这个宏
					result, err := pongo2.RenderTemplateString(tpl, ctx)
					if err == nil {
						s = result
					} else {
						// 渲染失败
						s = err.Error()
					}
				} else {
					s = ""
				}
			} else {
				var tocHtml bytes.Buffer
				tocHtml.WriteString("<ul class=\"toc\">")
				for _, item := range toc {
					tocHtml.WriteString("<li class=\"toc-level-" + strconv.Itoa(item.Level) + "\">" +
						"<a href=\"" + item.Anchor + "\"><span class=\"toc-prefix\">" + item.Prefix + "</span><span class=\"toc-name\">" + item.Title + "</span></a>" +
						"<li>")
				}
				tocHtml.WriteString("</ul>")
				s = tocHtml.String()
			}
		} else if match[4] != "" {
			// 必须有引入文件，否则不渲染
			// 先加载模板
			tplBuf, exist := w.GetTemplate(match[4])
			if exist {
				tpl := tplBuf + "\n{{ " + match[1] + match[2] + " }}"
				// 渲染这个宏
				result, err := pongo2.RenderTemplateString(tpl, ctx)
				if err == nil {
					s = result
				} else {
					// 渲染失败
					s = err.Error()
				}
			} else {
				s = ""
			}
		}
		return s
	})

	return content, showContentTitle
}
