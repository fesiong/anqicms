package sms

import (
	"errors"
	"testing"
)

// normalizePhoneCases 覆盖规格 §6.2 五条规则与任务简报审查重点 1 的全部用例。
// TestNormalizePhone 与 TestNormalizePhoneIdempotent 共用同一张表，
// 保证幂等性断言覆盖每一条成功输入。
var normalizePhoneCases = []struct {
	name        string
	country     string
	local       string
	wantCountry string
	wantLocal   string
	wantRegion  string
	wantErr     bool
}{
	{"国内基本", "86", "13900001111", "86", "13900001111", "domestic", false},
	{"加号区号", "+86", "139 0000 1111", "86", "13900001111", "domestic", false},
	{"00 前缀区号", "0086", "139-0000-1111", "86", "13900001111", "domestic", false},
	{"空区号默认 86", "", "13900001111", "86", "13900001111", "domestic", false},
	{"全角数字", "86", "１３９００００１１１１", "86", "13900001111", "domestic", false},
	{"零宽与不换行空格", "86", "139\u200b0000\u00a01111", "86", "13900001111", "domestic", false},
	{"括号与点", "1", "(415) 555.0100", "1", "4155550100", "international", false},
	{"意大利前导 0 保留", "39", "0612345678", "39", "0612345678", "international", false},
	{"港澳台算国际", "852", "61234567", "852", "61234567", "international", false},
	{"E164 恰好 15 位", "1", "12345678901234", "1", "12345678901234", "international", false},
	{"国内少一位", "86", "1390000111", "", "", "", true},
	{"国内非 1[3-9] 开头", "86", "23900001111", "", "", "", true},
	{"区号为 0", "0", "1234567", "", "", "", true},
	{"区号 5 位", "12345", "1234567", "", "", "", true},
	{"本地号码 3 位", "1", "123", "", "", "", true},
	{"E164 超 15 位", "1234", "123456789012", "", "", "", true},
	{"本地号码为空", "86", "", "", "", "", true},

	// 以下为修复第 1 轮（R8）新增：(Country, Local) 必须对 E164 单射。
	// 同一串 +8613900001111 只允许 (86, 13900001111) 一种切分，
	// 否则攻击者换切分即可乘出多份号码日限与重发冷却预算。
	{"E164 切分唯一_86 正确切分", "86", "13900001111", "86", "13900001111", "domestic", false},
	{"E164 切分唯一_8 伪区号", "8", "61390001111", "", "", "", true},
	{"E164 切分唯一_861 伪区号", "861", "3900001111", "", "", "", true},
	{"E164 切分唯一_8613 伪区号", "8613", "900001111", "", "", "", true},
	{"E164 切分唯一_39 正确切分", "39", "0612345678", "39", "0612345678", "international", false},
	{"E164 切分唯一_390 伪区号", "390", "612345678", "", "", "", true},
	// 规格 §6.2 亲自点名的回归串：VerifyCellphoneFormat 的无锚点正则会放过它。
	{"国内正则带锚点_前后缀字母", "86", "abc13900001111xyz", "", "", "", true},
	// 非地理号段（业务/卫星/试验）不可达短信，必须拒。
	{"非地理号段_800", "800", "12345678", "", "", "", true},
	{"非地理号段_882", "882", "16123456", "", "", "", true},
	// 表不能过紧：2016 年后才启用的 970 与常见地理码必须放行。
	{"巴勒斯坦 970", "970", "599123456", "970", "599123456", "international", false},
	{"英国 44", "44", "7911123456", "44", "7911123456", "international", false},
}

func TestNormalizePhone(t *testing.T) {
	for _, tc := range normalizePhoneCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizePhone(tc.country, tc.local)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizePhone(%q, %q) 应失败，实际返回 %+v", tc.country, tc.local, got)
				}
				if !errors.Is(err, ErrSmsPhoneInvalid) {
					t.Fatalf("NormalizePhone(%q, %q) 的错误应为 ErrSmsPhoneInvalid，实际 %v", tc.country, tc.local, err)
				}
				// 失败必须返回零值 Phone：调用方若忽略 err 直接拿键去查重/落库，
				// 半成品键会污染 (phone_country, phone) 唯一性。
				if got != (Phone{}) {
					t.Errorf("NormalizePhone(%q, %q) 失败时应返回零值 Phone，实际 %+v", tc.country, tc.local, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePhone(%q, %q) 意外失败: %v", tc.country, tc.local, err)
			}
			if got.Country != tc.wantCountry {
				t.Errorf("Country = %q, want %q", got.Country, tc.wantCountry)
			}
			if got.Local != tc.wantLocal {
				t.Errorf("Local = %q, want %q", got.Local, tc.wantLocal)
			}
			if got.Region() != tc.wantRegion {
				t.Errorf("Region() = %q, want %q", got.Region(), tc.wantRegion)
			}
			if want := "+" + tc.wantCountry + tc.wantLocal; got.E164() != want {
				t.Errorf("E164() = %q, want %q", got.E164(), want)
			}
		})
	}
}

// TestNormalizePhoneIdempotent 断言归一化幂等：对每条成功用例，
// 用首次结果的 Country/Local 再归一化必须逐字段相等，否则查重键会漂移。
func TestNormalizePhoneIdempotent(t *testing.T) {
	for _, tc := range normalizePhoneCases {
		if tc.wantErr {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			first, err := NormalizePhone(tc.country, tc.local)
			if err != nil {
				t.Fatalf("首次归一化失败: %v", err)
			}
			second, err := NormalizePhone(first.Country, first.Local)
			if err != nil {
				t.Fatalf("二次归一化失败（不幂等）: %v", err)
			}
			if second.Country != first.Country || second.Local != first.Local {
				t.Fatalf("归一化不幂等: first=%+v second=%+v", first, second)
			}
		})
	}
}
