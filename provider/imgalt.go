package provider

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// 图片 Alt 优化：为正文中没有 alt（或 alt 为空）的 <img> 标签自动填充 alt。
// alt 是插入图片时固化到正文 HTML 里的内联属性，系统没有独立的 alt 库，
// 因此在内容保存管线里做一次固化补全：纯字符串处理，不访问数据库，
// 渲染路径零额外开销（与 ReplaceContentUrl / ReplaceSingleSpace 同一处理链）。
//
// 规则：
//   - 第 1 张图片 alt = 标题；后续图片追加序号（标题-2、标题-3），避免 SEO 重复
//   - 已有非空 alt 的图片不覆盖（用户显式设置的值优先）
//   - 标题做 HTML 转义，防止破坏属性引号

var (
	imgTagRe     = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	imgAltAttrRe = regexp.MustCompile(`(?i)\balt\s*=\s*(["'])([^"']*)(["'])`)
)

// FillContentImgAlt 为 content 中没有 alt 或 alt 为空的 <img> 填充 alt。
// title 为空或 content 为空时原样返回。
func FillContentImgAlt(content, title string) string {
	if content == "" || strings.TrimSpace(title) == "" {
		return content
	}
	title = html.EscapeString(strings.TrimSpace(title))
	index := 0
	return imgTagRe.ReplaceAllStringFunc(content, func(tag string) string {
		index++
		alt := title
		if index > 1 {
			alt = fmt.Sprintf("%s-%d", title, index)
		}
		m := imgAltAttrRe.FindStringSubmatchIndex(tag)
		if m != nil {
			// alt 属性存在：m[4]:m[5] 为值（无引号分支时 m[6]:m[7]，两者必有其一）
			valStart, valEnd := m[4], m[5]
			if valStart < 0 {
				valStart, valEnd = m[6], m[7]
			}
			if strings.TrimSpace(tag[valStart:valEnd]) != "" {
				return tag // 已有非空 alt，保留用户显式设置
			}
			// 空 alt：把值替换为填充值，保留原引号
			quote := tag[m[2]:m[3]]
			attrStart, attrEnd := m[0], m[1]
			return tag[:attrStart] + `alt=` + quote + alt + quote + tag[attrEnd:]
		}
		// 无 alt 属性：紧跟 <img 之后插入，兼容大小写
		lower := strings.ToLower(tag)
		insertAt := strings.Index(lower, "<img") + len("<img")
		return tag[:insertAt] + ` alt="` + alt + `"` + tag[insertAt:]
	})
}
