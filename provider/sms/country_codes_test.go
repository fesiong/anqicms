package sms

import (
	"sort"
	"strings"
	"testing"
)

// nonGeographicCodes 是裁决 R8 明确排除的非地理号段：国际免费电话／共享成本／
// 卫星／国际网络／灾难救济／试验号段。短信是人对人投递，这些号段不可达。
var nonGeographicCodes = []string{
	"388", "800", "808", "870", "878", "879",
	"881", "882", "883", "888", "979", "991",
}

// TestCountryCodesPrefixFree 穷举断言区号表是前缀无关集（prefix-free）：
// 没有任何一个码是另一个码的真前缀。这条不变量正是「同一 E164 只有一种
// (Country, Local) 切分」的数学依据——表一旦失去它，号码日限与重发冷却键
// 就能被换切分乘法放大（审查重要 1）。
func TestCountryCodesPrefixFree(t *testing.T) {
	// 反空转：空表会让下面的双重循环一条都不执行。
	if len(countryCodes) == 0 {
		t.Fatal("countryCodes 是空表：前缀无关断言会空转，且 NormalizePhone 会拒绝所有号码")
	}
	codes := make([]string, 0, len(countryCodes))
	for code := range countryCodes {
		codes = append(codes, code)
	}
	// map 迭代不确定 → 排序构造，保证失败信息可复现。
	sort.Strings(codes)

	for _, a := range codes {
		// 表里的码必须都能通过规格 §6.2 的区号正则，否则成员校验永远命中不了。
		if !rePhoneCountry.MatchString(a) {
			t.Errorf("区号 %q 不满足规格 §6.2 的 ^[1-9]\\d{0,3}$", a)
		}
		for _, b := range codes {
			if a != b && strings.HasPrefix(b, a) {
				t.Errorf("区号 %q 是 %q 的真前缀：同一 E164 会出现多种切分，限额键可被放大", a, b)
			}
		}
	}

	// 抽查必须收录的地理码（含港澳台与 2016 年后才启用的 970），
	// 防止表被删到只剩几条也能通过上面的循环。
	for _, want := range []string{"1", "7", "39", "44", "81", "86", "852", "853", "886", "970"} {
		if _, ok := countryCodes[want]; !ok {
			t.Errorf("区号表缺少地理码 %q", want)
		}
	}
}

// TestCountryCodesExcludeNonGeographic 锁住裁决 R8 的排除清单：
// 业务/卫星/试验号段不得进入区号表。
func TestCountryCodesExcludeNonGeographic(t *testing.T) {
	for _, code := range nonGeographicCodes {
		if _, ok := countryCodes[code]; ok {
			t.Errorf("非地理号段 %q 不得进入区号表（短信不可达）", code)
		}
	}
}
