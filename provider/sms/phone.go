package sms

import (
	"regexp"
	"strings"

	"kandaoni.com/anqicms/config"
)

// Phone 是归一化后的手机号：Country 为纯数字区号，Local 为纯数字本地号码。
// 本地号码不去前导 0——意大利等国的号码前导 0 是有效组成部分，去掉会发错人。
type Phone struct {
	Country string
	Local   string
}

// E164 返回 "+" + 区号 + 本地号码，如 +8613900001111。
func (p Phone) E164() string {
	return "+" + p.Country + p.Local
}

// Region 按区号判区域：86 → domestic，其余（含 852／853／886）→ international。
func (p Phone) Region() string {
	if p.Country == "86" {
		return config.SmsRegionDomestic
	}
	return config.SmsRegionInternational
}

// 校验正则逐字来自规格 §6.2。
var (
	rePhoneCountry  = regexp.MustCompile(`^[1-9]\d{0,3}$`)
	rePhoneLocal    = regexp.MustCompile(`^\d{4,14}$`)
	rePhoneDomestic = regexp.MustCompile(`^1[3-9]\d{9}$`)
)

// NormalizePhone 归一化并校验手机号（规格 §6.2）：
// 先折全角数字 → 再去格式字符 → 再校验。所有失败路径返回 ErrSmsPhoneInvalid。
// 归一化是幂等的：NormalizePhone(p.Country, p.Local) 与首次结果逐字段相等，
// 否则查重键会漂移。
func NormalizePhone(country, local string) (Phone, error) {
	country = cleanPhone(country)
	local = cleanPhone(local)

	// 区号去前导 + 与 00，空则默认 86。
	country = strings.TrimPrefix(country, "+")
	country = strings.TrimPrefix(country, "00")
	if country == "" {
		country = "86"
	}
	if !rePhoneCountry.MatchString(country) {
		return Phone{}, ErrSmsPhoneInvalid
	}
	if !rePhoneLocal.MatchString(local) {
		return Phone{}, ErrSmsPhoneInvalid
	}
	// 组合后 E.164 数字总长 ≤ 15。
	if len(country)+len(local) > 15 {
		return Phone{}, ErrSmsPhoneInvalid
	}
	// 国内号码额外校验（带锚点，不复用 VerifyCellphoneFormat）。
	if country == "86" && !rePhoneDomestic.MatchString(local) {
		return Phone{}, ErrSmsPhoneInvalid
	}
	return Phone{Country: country, Local: local}, nil
}

// cleanPhone 先把全角数字（U+FF10..U+FF19）折半角，再去格式字符：
// 半角空格、\t、-、(、)、.、不换行空格、零宽空格、零宽连接符、BOM。
func cleanPhone(s string) string {
	s = foldFullWidthDigits(s)
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '-', '(', ')', '.', '\u00a0', '\u200b', '\u200d', '\ufeff':
			return -1
		}
		return r
	}, s)
}

// foldFullWidthDigits 把全角数字 ０-９ 折成半角 0-9，其余字符原样保留。
func foldFullWidthDigits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return r - '０' + '0'
		}
		return r
	}, s)
}
