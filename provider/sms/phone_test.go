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
