package sms

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// 本文件是「15 个哨兵 ↔ 三份 locale ↔ 错误分类契约」这份跨 4 文件声明式清单的
// 入库护栏（裁决 R9）。此前只有跑完即删的临时脚本，重命名哨兵或漏配 locale 键
// 不会有任何测试变红，前台会直接看到裸键名，%d 丢失则输出 %!d(MISSING)。
// 按 AGENTS.md §6：用 go/ast 穷举校验目标存在，不写死哨兵总数。

// smsLocales 是本期承诺补齐文案的三份 locale（其余 6 份为部分翻译，缺键回落键名）。
var smsLocales = []string{"zh-CN", "zh-TW", "en-US"}

// smsParameterizedKeys 是文案必须带恰好一个 %d 的两个键（裁决 R9 ②）。
var smsParameterizedKeys = []string{"SmsResendTooSoon", "SmsCodeMismatch"}

// smsControllerOnlyKeys 是**只在 controller 层使用**、不对应 sms 包错误哨兵的
// 文案键：语义属于「入参不合法」而不是「发送链路失败」，因此不该进 sms 包。
// 目前只有 SmsParamInvalid（controller/manageController/pluginSms.go 的请求体
// 解析与筛选参数校验出口；它由任务 16A 加文案时引入）。
//
// 白名单本身也要被守卫：每个白名单键必须在三份 locale 里都真实存在，
// 否则删了 locale 文案就会变成「前台看到裸键名」而无人察觉（裁决 R151）。
var smsControllerOnlyKeys = map[string]string{
	"SmsParamInvalid": "controller/manageController/pluginSms.go 的入参校验出口",
}

// errorsNewKey 判断表达式是否 errors.New("<字面量>")，是则返回键名。
func errorsNewKey(expr ast.Expr) (string, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "New" {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "errors" {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	key, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return key, true
}

// sentinelKeys 用 go/ast 从 errors.go 穷举 errors.New("...") 的键集合。
// 同时校验：每个键都挂在包级 var 上，且变量名恒为 "Err" + 键名。
func sentinelKeys(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "errors.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 errors.go 失败: %v", err)
	}

	all := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if key, ok := errorsNewKey(call); ok {
				all[key] = true
			}
		}
		return true
	})
	// 反空转：解析不到任何哨兵时，后面的差集断言会两侧皆空而假绿。
	if len(all) == 0 {
		t.Fatal("errors.go 中未解析到任何 errors.New 哨兵：护栏会空转")
	}

	named := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if i >= len(spec.Values) {
				continue
			}
			key, ok := errorsNewKey(spec.Values[i])
			if !ok {
				continue
			}
			named[key] = true
			if want := "Err" + key; name.Name != want {
				t.Errorf("哨兵变量名 %q 与翻译键 %q 不一致，应为 %q", name.Name, key, want)
			}
		}
		return true
	})
	for key := range all {
		if !named[key] {
			t.Errorf("翻译键 %q 没有挂在包级 var 上（调用方无法 errors.Is）", key)
		}
	}

	keys := make([]string, 0, len(all))
	for key := range all {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// smsLocaleMessages 读取一份 locale 的全部 Sms* 键值。
// yaml.v3 对重复键会直接报错，顺带锁住「同一键写两遍」。
func smsLocaleMessages(t *testing.T, locale string) map[string]string {
	t.Helper()
	path := filepath.Join("..", "..", "locales", locale, "default.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	var all map[string]string
	if err := yaml.Unmarshal(data, &all); err != nil {
		t.Fatalf("解析 %s 失败: %v", path, err)
	}
	msgs := make(map[string]string)
	for k, v := range all {
		if strings.HasPrefix(k, "Sms") {
			msgs[k] = v
		}
	}
	if len(msgs) == 0 {
		t.Fatalf("%s 中没有任何 Sms* 键：差集断言会空转", path)
	}
	return msgs
}

// TestSmsSentinelKeysMatchLocales 双向差集：哨兵键 ⊆ locale 键，且 locale 的
// Sms* 键 ⊆ 哨兵键。任一方向漏配都会让前台看到裸键名或留下孤儿文案。
func TestSmsSentinelKeysMatchLocales(t *testing.T) {
	keys := sentinelKeys(t)
	inSource := make(map[string]bool, len(keys))
	for _, k := range keys {
		inSource[k] = true
	}
	for _, locale := range smsLocales {
		t.Run(locale, func(t *testing.T) {
			msgs := smsLocaleMessages(t, locale)
			for _, k := range keys {
				if _, ok := msgs[k]; !ok {
					t.Errorf("locale %s 缺少哨兵键 %q：前台会看到裸键名", locale, k)
				}
			}
			orphans := make([]string, 0)
			for k := range msgs {
				if !inSource[k] {
					// controller 专用键没有 sms 哨兵是**预期**的（见 smsControllerOnlyKeys），
					// 但白名单本身由下面的断言守住（必须在 locale 里存在）。
					if _, ok := smsControllerOnlyKeys[k]; ok {
						continue
					}
					orphans = append(orphans, k)
				}
			}
			sort.Strings(orphans)
			for _, k := range orphans {
				t.Errorf("locale %s 的键 %q 没有对应哨兵（孤儿文案）", locale, k)
			}
			// 白名单键必须在 locale 里真实存在（白名单不得腐化）。
			for k := range smsControllerOnlyKeys {
				if _, ok := msgs[k]; !ok {
					t.Errorf("locale %s 缺少 controller 专用键 %q（白名单里登记过）", locale, k)
				}
			}
		})
	}
}

// TestSmsControllerOnlyKeysAreReallyUsed 白名单不得退化成「永久豁免口袋」：
// 每个 controller 专用键都必须在 controller 源码里被真实引用，且引用处是
// `Tr("键名")` 形态。若该键将来被挪进 sms 包（变成哨兵），本测试会红，
// 提醒把白名单条目删掉。
func TestSmsControllerOnlyKeysAreReallyUsed(t *testing.T) {
	controllerSrc := filepath.Join("..", "..", "controller", "manageController", "pluginSms.go")
	data, err := os.ReadFile(controllerSrc)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", controllerSrc, err)
	}
	src := string(data)
	for k := range smsControllerOnlyKeys {
		if !strings.Contains(src, `Tr("`+k+`")`) {
			t.Errorf("controller 专用键 %q 在 %s 里没有真实引用（白名单会退化成永久豁免）", k, controllerSrc)
		}
	}
}

// TestSmsLocalePlaceholders 断言 %d 恰好落在两个参数化键上，且文案可被
// fmt.Sprintf 正常消费（丢失 %d 会输出 %!d(MISSING)）。
func TestSmsLocalePlaceholders(t *testing.T) {
	want := make(map[string]bool, len(smsParameterizedKeys))
	for _, k := range smsParameterizedKeys {
		want[k] = true
	}
	for _, locale := range smsLocales {
		t.Run(locale, func(t *testing.T) {
			msgs := smsLocaleMessages(t, locale)
			for _, k := range smsParameterizedKeys {
				v, ok := msgs[k]
				if !ok {
					t.Fatalf("locale %s 缺少参数化键 %q", locale, k)
				}
				if n := strings.Count(v, "%d"); n != 1 {
					t.Errorf("locale %s 键 %q 应恰好带 1 个 %%d，实际 %d 个: %q", locale, k, n, v)
				}
				if n := strings.Count(v, "%"); n != 1 {
					t.Errorf("locale %s 键 %q 只允许一个 %%d 占位符，实际 %d 个 %%: %q", locale, k, n, v)
				}
				// 真实消费一次：Tr 的可变参数走的就是 fmt.Sprintf。
				got := fmt.Sprintf(v, 60)
				if strings.Contains(got, "%!") || !strings.Contains(got, "60") {
					t.Errorf("locale %s 键 %q 格式化异常: %q", locale, k, got)
				}
			}
			for k, v := range msgs {
				if !want[k] && strings.Contains(v, "%") {
					t.Errorf("locale %s 键 %q 不该带占位符: %q", locale, k, v)
				}
			}
		})
	}
}

// TestClassifyHTTPStatus 锁住规格 §10.1 的分类边界。
func TestClassifyHTTPStatus(t *testing.T) {
	cases := []struct {
		status int
		want   ErrorClass
	}{
		{0, ClassTransient},   // 未知
		{-1, ClassTransient},  // 未知
		{100, ClassTransient}, // 1xx
		{200, ClassTransient}, // 2xx：业务码另判
		{204, ClassTransient},
		{302, ClassTransient}, // 3xx
		{399, ClassTransient},
		{400, ClassPermanent},
		{401, ClassPermanent},
		{403, ClassPermanent},
		{404, ClassPermanent},
		{408, ClassTransient}, // 例外：超时
		{425, ClassTransient}, // 例外：Too Early
		{429, ClassTransient}, // 例外：限流
		{451, ClassPermanent},
		{499, ClassPermanent}, // 4xx 上界
		{500, ClassTransient}, // 5xx 下界
		{501, ClassTransient},
		{503, ClassTransient},
		{599, ClassTransient},
		{600, ClassTransient}, // ≥500 一律 transient
	}
	for _, tc := range cases {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			if got := ClassifyHTTPStatus(tc.status); got != tc.want {
				t.Errorf("ClassifyHTTPStatus(%d) = %q, want %q", tc.status, got, tc.want)
			}
		})
	}
}

// TestWaitErrorIsAndAs 锁住重发等待错误的翻译键与 Is/As 链。
func TestWaitErrorIsAndAs(t *testing.T) {
	var err error = &WaitError{Seconds: 42}
	if got := err.Error(); got != "SmsResendTooSoon" {
		t.Errorf("WaitError.Error() = %q, want %q", got, "SmsResendTooSoon")
	}
	if !errors.Is(err, ErrSmsResendTooSoon) {
		t.Error("errors.Is(WaitError, ErrSmsResendTooSoon) 应为 true")
	}
	if errors.Is(err, ErrSmsCodeMismatch) {
		t.Error("WaitError 不该命中 ErrSmsCodeMismatch")
	}
	var wait *WaitError
	if !errors.As(err, &wait) {
		t.Fatal("errors.As 取不出 *WaitError")
	}
	if wait.Seconds != 42 {
		t.Errorf("WaitError.Seconds = %d, want 42", wait.Seconds)
	}
	// 被 fmt.Errorf("%w") 包一层后链条仍要通。
	wrapped := fmt.Errorf("发送前置检查: %w", err)
	if !errors.Is(wrapped, ErrSmsResendTooSoon) {
		t.Error("包装后 errors.Is 应仍命中 ErrSmsResendTooSoon")
	}
	if !errors.As(wrapped, &wait) || wait.Seconds != 42 {
		t.Error("包装后 errors.As 应仍取出 Seconds=42")
	}
}

// TestAttemptsErrorIsAndAs 锁住剩余次数错误的翻译键与 Is/As 链。
func TestAttemptsErrorIsAndAs(t *testing.T) {
	var err error = &AttemptsError{Remaining: 3}
	if got := err.Error(); got != "SmsCodeMismatch" {
		t.Errorf("AttemptsError.Error() = %q, want %q", got, "SmsCodeMismatch")
	}
	if !errors.Is(err, ErrSmsCodeMismatch) {
		t.Error("errors.Is(AttemptsError, ErrSmsCodeMismatch) 应为 true")
	}
	if errors.Is(err, ErrSmsResendTooSoon) {
		t.Error("AttemptsError 不该命中 ErrSmsResendTooSoon")
	}
	var attempts *AttemptsError
	if !errors.As(err, &attempts) {
		t.Fatal("errors.As 取不出 *AttemptsError")
	}
	if attempts.Remaining != 3 {
		t.Errorf("AttemptsError.Remaining = %d, want 3", attempts.Remaining)
	}
	wrapped := fmt.Errorf("校验: %w", err)
	if !errors.Is(wrapped, ErrSmsCodeMismatch) {
		t.Error("包装后 errors.Is 应仍命中 ErrSmsCodeMismatch")
	}
	if !errors.As(wrapped, &attempts) || attempts.Remaining != 3 {
		t.Error("包装后 errors.As 应仍取出 Remaining=3")
	}
}

// TestSendErrorUnwrap 锁住裁决 R10：下游只用 errors.Is 就能落到通用翻译文案，
// 不必依赖每个 controller 记得做类型分派；原始码仍由 errors.As 取出落日志。
func TestSendErrorUnwrap(t *testing.T) {
	// 一律用假凭据/假平台码。
	var err error = &SendError{Class: ClassPermanent, Code: "isv.FAKE_CODE", Message: "fake raw message"}
	if !errors.Is(err, ErrSmsSendFailed) {
		t.Error("errors.Is(SendError, ErrSmsSendFailed) 应为 true（否则前台拿不到通用文案）")
	}
	if errors.Is(err, ErrSmsPhoneInvalid) {
		t.Error("SendError 不该命中 ErrSmsPhoneInvalid")
	}
	var sendErr *SendError
	if !errors.As(err, &sendErr) {
		t.Fatal("errors.As 取不出 *SendError")
	}
	if sendErr.Class != ClassPermanent || sendErr.Code != "isv.FAKE_CODE" || sendErr.Message != "fake raw message" {
		t.Errorf("errors.As 取出的字段被改写: %+v", sendErr)
	}
	// Error() 的现有格式不变：后台日志需要平台原始码。
	if got, want := err.Error(), "[permanent] isv.FAKE_CODE: fake raw message"; got != want {
		t.Errorf("SendError.Error() = %q, want %q", got, want)
	}
	wrapped := fmt.Errorf("平台发送: %w", err)
	if !errors.Is(wrapped, ErrSmsSendFailed) {
		t.Error("包装后 errors.Is 应仍命中 ErrSmsSendFailed")
	}
	if !errors.As(wrapped, &sendErr) || sendErr.Code != "isv.FAKE_CODE" {
		t.Error("包装后 errors.As 应仍取出原始码")
	}
}
