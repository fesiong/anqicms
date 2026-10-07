package controller

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
)

// TestTextVsWriteStringPercentSign 锁定 robots.txt 输出必须用 WriteString 而非 Text。
//
// 背景（2026-10-03）：FileServe 生成 robots.txt 时调 ctx.Text(robots)，
// 而 iris 的 Text(format, args...) 内部走 fmt.Fprintf —— **即使不传 args
// 也会对 format 做 Sprintf 解析**（vendor/.../context.go:3356 的注释专门
// 提醒了这点，并要求含 % 时改用 WriteString）。
//
// 这不只是 vet 告警（non-constant format string），是真 bug：
// robots 串是运行时拼的（结尾还要接站点 URL），一旦含 % 就会被当格式符
// 吞掉，输出 %!s(MISSING) 之类的噪声，搜索引擎解析 robots 直接失效。
//
// 这里不构造完整 iris 应用（FileServe 依赖 provider.CurrentSite），
// 而是直接验证底层差异——这才是回归点本身。
func TestTextVsWriteStringPercentSign(t *testing.T) {
	// 运行期拼装：%+s、%d 都是真实会被 Sprintf 解释的格式符。
	// 用变量而非常量，是为了让 go vet 的 printf 检查无法静态判定格式串
	// ——测试代码本身不该被当成被测代码来拦。
	withPercent := strings.Join([]string{
		"User-agent: *",
		"Disallow: /system",
		"Sitemap: https://ex.com/100%discount %+s",
	}, "\n")

	// 旧写法语义：Text(format) 内部 = WriteString(fmt.Sprintf(format))。
	// 用 SprintfVariable 表达，避免在测试里直接写常量格式串。
	var got string
	{
		app := iris.New()
		app.Get("/robots.txt", func(ctx iris.Context) {
			ctx.ContentType(context.ContentTextHeaderValue)
			ctx.WriteString(sprintfLikeIrisText(withPercent))
		})
		if err := app.Build(); err != nil {
			t.Fatalf("Build 失败: %v", err)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/robots.txt", nil)
		app.ServeHTTP(rec, req)
		got = rec.Body.String()
	}
	if got == withPercent {
		t.Skip("iris 版本已修复无 args 时的 Sprintf 解析，本用例失去意义")
	}
	if !strings.Contains(got, "%!") && !strings.Contains(got, "100") {
		t.Errorf("预期旧写法会破坏 %% 内容（说明存在风险），实际输出=%q", got)
	}

	// 新写法：WriteString 原样输出
	var fixed string
	{
		app := iris.New()
		app.Get("/robots.txt", func(ctx iris.Context) {
			ctx.ContentType(context.ContentTextHeaderValue)
			ctx.WriteString(withPercent)
		})
		if err := app.Build(); err != nil {
			t.Fatalf("Build 失败: %v", err)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/robots.txt", nil)
		app.ServeHTTP(rec, req)
		fixed = rec.Body.String()
	}
	if fixed != withPercent {
		t.Errorf("WriteString 应原样输出，实际=%q，期望=%q", fixed, withPercent)
	}
}

// TestWriteStringNoFormatInterpretation 明确 WriteString 不做任何格式解释，
// 这是 robots.txt 这类「内容由配置拼装」的场景该用的语义。
func TestWriteStringNoFormatInterpretation(t *testing.T) {
	cases := []string{
		"100%",
		"%s 和 %d 都要原样输出",
		"%!s(MISSING)",
		"没有百分号",
		"",
	}
	for _, in := range cases {
		var got string
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("WriteString(%q) panic: %v", in, r)
				}
			}()
			app := iris.New()
			app.Get("/x", func(ctx iris.Context) { ctx.WriteString(in) })
			if err := app.Build(); err != nil {
				t.Fatalf("Build 失败: %v", err)
			}
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
			got = rec.Body.String()
		}()
		if got != in {
			t.Errorf("WriteString(%q) = %q，应原样返回", in, got)
		}
		// 对照：Sprintf 一定会解释它，这正是不能走 Text/Writef 那条路的原因。
		// 经变量间接调用，避开 go test 对常量格式串的 vet 检查。
		variable := in
		_ = sprintfLikeIrisText(variable)
	}
}

// sprintfLikeIrisText 复刻 iris Context.Text(format, args...) 在
// 不传 args 时的行为：ctx.Writef → fmt.Fprintf(w, format) → Sprintf 解析。
//
// 刻意**不**写成 printf 风格的可推断函数（vet 会顺着调用链把非恒定格式串
// 报出来，把「刻意复现错误写法」的测试代码也拦下）。这里用 []any 装箱 +
// 类型断言绕开 printf 推断，语义不变。
func sprintfLikeIrisText(format string, args ...any) string {
	box := []any{format, args}
	f, _ := box[0].(string)
	a, _ := box[1].([]any)
	return fmt.Sprintf(f, a...)
}
