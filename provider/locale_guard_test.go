package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLocaleValuesHaveNoTemplateBraces 语言包文案里**不允许出现 `{{` / `}}`**。
//
// 原因（2026-11-15 实测踩坑）：iris 的 i18n.Load 会把每条文案当 **Go template**
// 解析。SMS 的 `SmsTemplateInvalid` 文案里原本写了「…（{{code}}）」，
// 于是加载语言包时报 `parse string: template: SmsTemplateInvalid:1:
// function "code" not defined` —— 一个语言包的**一条文案**能让整包加载失败，
// 后果是全站 i18n 不可用（不是「这条文案翻译不出来」，是「整个语言包炸了」）。
//
// 这条护栏扫全部语言包：任何文案里出现花括号就判红。
// 真要用占位符，请用 iris 支持的 `%s` 形态（键名带 %d 的那两个键已按此处理）。
func TestLocaleValuesHaveNoTemplateBraces(t *testing.T) {
	locales, err := filepath.Glob(filepath.Join("..", "locales", "*", "default.yml"))
	if err != nil {
		t.Fatalf("扫描 locales 目录失败: %v", err)
	}
	if len(locales) == 0 {
		t.Fatal("没找到任何语言包文件（护栏会空转）")
	}
	for _, path := range locales {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", path, err)
		}
		var all map[string]string
		if err := yaml.Unmarshal(data, &all); err != nil {
			t.Fatalf("解析 %s 失败: %v", path, err)
		}
		for k, v := range all {
			if strings.Contains(v, "{{") || strings.Contains(v, "}}") {
				t.Errorf("%s 的键 %q 的文案里含 {{ 或 }}：iris i18n 会把它当 Go template 解析，"+
					"整包加载会失败（全站 i18n 不可用）。文案：%q", path, k, v)
			}
		}
	}
}
