package utils

import (
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
)

// MarkdownToHTML 将markdown 转换为 html
func MarkdownToHTML(md string) string {

	unsafe := blackfriday.Run([]byte(md))
	html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
	return string(html)
}

// AvoidXSS 避免XSS
func AvoidXSS(theHTML string) string {
	return bluemonday.UGCPolicy().Sanitize(theHTML)
}
