package library

import (
	"container/list"
	"unicode/utf8"
)

// AhoCorasick 自动机
type AhoCorasick struct {
	root *acNode
}

type acNode struct {
	children map[rune]*acNode
	fail     *acNode
	pattern  []byte // 如果是模式串的结尾，记录该模式串
	data     interface{}
}

func NewAhoCorasick(patterns []string) *AhoCorasick {
	return NewAhoCorasickWithData(patterns, nil)
}

func NewAhoCorasickWithData(patterns []string, data []interface{}) *AhoCorasick {
	ac := &AhoCorasick{
		root: &acNode{
			children: make(map[rune]*acNode),
		},
	}
	for i, p := range patterns {
		var d interface{}
		if data != nil && i < len(data) {
			d = data[i]
		}
		ac.insert(p, d)
	}
	ac.build()
	return ac
}

func (ac *AhoCorasick) insert(pattern string, data interface{}) {
	node := ac.root
	for _, r := range pattern {
		if node.children[r] == nil {
			node.children[r] = &acNode{
				children: make(map[rune]*acNode),
			}
		}
		node = node.children[r]
	}
	node.pattern = []byte(pattern)
	node.data = data
}

func (ac *AhoCorasick) build() {
	queue := list.New()
	for _, node := range ac.root.children {
		node.fail = ac.root
		queue.PushBack(node)
	}

	for queue.Len() > 0 {
		element := queue.Front()
		queue.Remove(element)
		parent := element.Value.(*acNode)

		for r, child := range parent.children {
			fail := parent.fail
			for fail != nil && fail.children[r] == nil {
				fail = fail.fail
			}
			if fail == nil {
				child.fail = ac.root
			} else {
				child.fail = fail.children[r]
			}
			queue.PushBack(child)
		}
	}
}

// ACMatch 查找所有匹配的模式串及其位置
type ACMatch struct {
	Start   int
	End     int
	Pattern []byte
	Data    interface{}
}

func (ac *AhoCorasick) MultiMatch(content []byte) []ACMatch {
	var matches []ACMatch
	node := ac.root

	offset := 0
	for offset < len(content) {
		r, size := utf8.DecodeRune(content[offset:])
		for node != ac.root && node.children[r] == nil {
			node = node.fail
		}
		if node.children[r] != nil {
			node = node.children[r]
		}

		// 检查当前节点及其 fail 链上的所有匹配
		temp := node
		for temp != ac.root {
			if len(temp.pattern) > 0 {
				matches = append(matches, ACMatch{
					Start:   offset + size - len(temp.pattern),
					End:     offset + size,
					Pattern: temp.pattern,
					Data:    temp.data,
				})
			}
			temp = temp.fail
		}
		offset += size
	}
	return matches
}

// ReplaceAll 替换所有匹配的模式串
func (ac *AhoCorasick) ReplaceAll(content []byte, replaceFn func(match ACMatch) []byte) []byte {
	if len(content) == 0 {
		return content
	}

	matches := ac.MultiMatch(content)
	if len(matches) == 0 {
		return content
	}

	// 排序并处理重叠匹配
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].Start > matches[j].Start || (matches[i].Start == matches[j].Start && len(matches[i].Pattern) < len(matches[j].Pattern)) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	result := make([]byte, 0, len(content))
	lastEnd := 0

	for _, match := range matches {
		if match.Start < lastEnd {
			continue
		}
		result = append(result, content[lastEnd:match.Start]...)
		result = append(result, replaceFn(match)...)
		lastEnd = match.End
	}
	result = append(result, content[lastEnd:]...)

	return result
}
