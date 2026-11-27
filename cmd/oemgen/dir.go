package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"kandaoni.com/anqicms/config"
)

// 只改文本类文件。图片、字体、压缩包里没有可读文案，硬读只会浪费时间。
var textExts = map[string]bool{
	".html": true, ".htm": true, ".txt": true, ".js": true, ".css": true,
	".json": true, ".xml": true, ".svg": true, ".yml": true, ".yaml": true,
	".sh": true, ".bat": true,
}

// 模板里的翻译标签 `{% tr 'anqicms' %}`：单引号里是**键**不是文案，
// 一律不许动。键被改了而语言包里没跟着改，页面上会直接显示键名。
var trTagRe = regexp.MustCompile(`\{%\s*tr\s+['"][^'"]*['"]\s*%\}`)

// yml 的 `键: 值` 结构：只替换值部分，键同样不能动。
var ymlLineRe = regexp.MustCompile(`^(\s*(?:"[^"]*"|'[^']*'|[^:#\n]+):\s*)(.*)$`)

// brandifyDir 对打包副本做品牌替换。
//
// 为什么不改源码里的 template/default：官方版需要保留自己的官网反链，
// 而 OEM 版必须去掉。改源码就得让工作区在两种品牌之间来回脏，且很容易
// 把某个品牌的模板误提交；改 release 目录下的副本则天然是一次性的。
//
// 官方版构建时本函数是空操作（BrandizeWeb 对官方品牌直接返回原串），
// 所以不会给主站带来任何 diff。
func brandifyDir(dir string, brand config.OEMBrand) (int, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("%s 不是目录", dir)
	}
	touched := 0
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if d.IsDir() || !textExts[ext] {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var next string
		if ext == ".yml" || ext == ".yaml" {
			next = brandifyYAML(string(raw), brand)
		} else {
			next = brandifyMarkup(string(raw), brand)
		}
		if next == string(raw) {
			return nil
		}
		if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
			return err
		}
		touched++
		return nil
	})
	return touched, err
}

// brandifyMarkup 先保护翻译标签，再替换其余文案。
func brandifyMarkup(s string, brand config.OEMBrand) string {
	var tags []string
	masked := trTagRe.ReplaceAllStringFunc(s, func(tag string) string {
		tags = append(tags, tag)
		return "\x00" + strconv.Itoa(len(tags)-1) + "\x00"
	})
	masked = config.BrandizeWith(brand, masked)
	for i, tag := range tags {
		masked = strings.Replace(masked, "\x00"+strconv.Itoa(i)+"\x00", tag, 1)
	}
	return masked
}

// brandifyYAML 只替换值，保留键。
func brandifyYAML(s string, brand config.OEMBrand) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		m := ymlLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lines[i] = m[1] + config.BrandizeWith(brand, m[2])
	}
	return strings.Join(lines, "\n")
}
