package library

import (
	"strings"
	"unicode"
)

const (
	StateSearchDiv = iota
	StateInDivTag
	StateCheckClass
	StateTrackNesting
)

type DivLocator struct {
	state        int
	startIndex   int  // 目标div开始位置
	endIndex     int  // 目标div结束位置
	nestingLevel int  // 嵌套层级
	inQuote      bool // 是否在引号内
	quoteChar    byte // 当前引号类型
	classMatch   bool // 是否匹配到目标class
	findTag      string
	findClass    string
}

func NewDivLocator(findTag, findClass string) *DivLocator {
	return &DivLocator{
		state:     StateSearchDiv,
		findTag:   findTag,
		findClass: findClass,
	}
}

// 核心处理方法
func (d *DivLocator) FindDiv(html string) string {
	for i := 0; i < len(html); i++ {
		char := html[i]

		switch d.state {
		case StateSearchDiv:
			d.handleSearchDiv(html, i, char)
		case StateInDivTag:
			d.handleInDivTag(html, i, char)
		case StateCheckClass:
			d.handleCheckClass(html, i, char)
		case StateTrackNesting:
			d.handleTrackNesting(html, i, char)
		}

		// 提前终止条件
		if d.endIndex > 0 && i > d.endIndex {
			break
		}
	}

	if d.startIndex > 0 && d.endIndex > d.startIndex {
		return html[d.startIndex : d.endIndex+1]
	}
	return ""
}

// 处理状态1：寻找div开始标签
func (d *DivLocator) handleSearchDiv(html string, i int, char byte) {
	if char == '<' && i+3 < len(html) {
		if html[i:i+len(d.findTag)+1] == "<"+d.findTag {
			d.state = StateInDivTag
			d.startIndex = i // 记录潜在开始位置
		}
	}
}

// 处理状态2：在div标签内
func (d *DivLocator) handleInDivTag(html string, i int, char byte) {
	switch {
	case char == '>':
		d.state = StateSearchDiv
		if d.classMatch {
			d.state = StateTrackNesting
			d.nestingLevel = 1
		} else {
			d.startIndex = 0 // 重置非目标div
		}

	case !d.inQuote && unicode.IsSpace(rune(char)):
		d.state = StateCheckClass

	case char == '"' || char == '\'':
		if !d.inQuote {
			d.inQuote = true
			d.quoteChar = char
		} else if d.quoteChar == char {
			d.inQuote = false
		}
	}
}

// 处理状态3：检查class属性
func (d *DivLocator) handleCheckClass(html string, i int, char byte) {
	// 向后查找class属性
	if i+6 < len(html) && strings.HasPrefix(html[i:], "class") {
		valueStart := i + 5
		// 跳过等号和空格
		for valueStart < len(html) && (html[valueStart] == '=' || unicode.IsSpace(rune(html[valueStart]))) {
			valueStart++
		}

		if valueStart >= len(html) {
			return
		}

		// 检查引号
		quote := html[valueStart]
		if quote != '"' && quote != '\'' {
			return
		}

		endQuote := strings.IndexByte(html[valueStart+1:], quote)
		if endQuote == -1 {
			return
		}

		classValue := html[valueStart+1 : valueStart+1+endQuote]
		if strings.Contains(classValue, d.findClass) {
			d.classMatch = true
		}

		d.state = StateInDivTag
	}
}

// 处理状态4：追踪嵌套结构
func (d *DivLocator) handleTrackNesting(html string, i int, char byte) {
	if char == '<' {
		// 检测开始标签
		if i+len(d.findTag)+1 < len(html) && html[i:i+len(d.findTag)+1] == "<"+d.findTag {
			d.nestingLevel++
		}

		// 检测结束标签
		if i+len(d.findTag)+2 < len(html) && html[i:i+len(d.findTag)+3] == "</"+d.findTag+">" {
			d.nestingLevel--
			if d.nestingLevel == 0 {
				d.endIndex = i + len(d.findTag) + 2 // 记录结束位置
			}
		}
	}
}
