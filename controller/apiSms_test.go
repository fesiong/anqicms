package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/provider/sms"
)

// 编译期锚点：步骤 2 RED 时 ApiSmsSend / respondSmsError 尚未定义，
// 编译失败落到 "undefined: ApiSmsSend"（与任务 8/9 同款护栏）。
var (
	_ = ApiSmsSend
	_ = respondSmsError
)

// ---------------------------------------------------------------------------
// 源码级护栏检查器：接受源码字符串，返回第一个违规原因。
// 同一检查器同时喂「真实源码」与「合成坏源码」，实现反向对照（必红／恢复绿）。
// 声明式遍历 patterns，不写死总数。
// ---------------------------------------------------------------------------

func s10Require(src string, patterns ...string) error {
	for _, p := range patterns {
		if !regexp.MustCompile(p).MatchString(src) {
			return fmt.Errorf("缺锚点 /%s/", p)
		}
	}
	return nil
}

// s10CheckCaptchaGuard：Safe.Captcha==1 时 Store.Verify，且校验在 SendSmsCode 之前。
func s10CheckCaptchaGuard(src string) error {
	if err := s10Require(src,
		`currentSite\.Safe\.Captcha\s*==\s*1`,
		`Store\.Verify\(\s*req\.CaptchaId\s*,\s*req\.Captcha\s*,\s*true\s*\)`,
	); err != nil {
		return err
	}
	iVerify := strings.Index(src, "Store.Verify(")
	iSend := strings.Index(src, "SendSmsCode(")
	if iVerify < 0 || iSend < 0 || iVerify >= iSend {
		return errors.New("captcha 校验未在 SendSmsCode 之前")
	}
	return nil
}

// s10CheckBindOldGuard：bind_old 必须同时要求「已登录」与「号码等于库内绑定号码」。
func s10CheckBindOldGuard(src string) error {
	return s10Require(src,
		`ctx\.Values\(\)\.GetUintDefault\("userId",\s*0\)`, // 登录态来源
		`userId\s*==\s*0`,       // 条件一：已登录
		`GetUserInfoById\(`,     // 取库内绑定信息，不信任请求体自证
		`NormalizePhone\(`,      // 号码归一化后比对
		`\bsms\.SceneBindOld\b`, // 只对 bind_old 场景把关
		`\.PhoneCountry`,        // 条件二：区号比对
		`\.Phone`,               // 条件二：号码比对
	)
}

// s10CheckSceneGuard：前台拒 scene == test 与 notify. 前缀。
func s10CheckSceneGuard(src string) error {
	return s10Require(src,
		`sms\.SceneTest`,
		`notify\.`,
		`HasPrefix\(`,
		`sms\.ErrSmsSceneNotEnabled`,
	)
}

// s10CheckLoginDispatch：ApiLogin 出现 PlatformSms 分派并调 LoginViaSms，
// 错误统一走 respondSmsError（R81：不得吞成 TplTr("LoginFailed")）。
func s10CheckLoginDispatch(src string) error {
	return s10Require(src,
		`req\.Platform\s*==\s*config\.PlatformSms`,
		`LoginViaSms\(&req\)`,
		`respondSmsError\(`,
	)
}

// s10CheckRespondSource：respondSmsError 的 R82/R83/R144 源码护栏。
// 翻译出口经方法值 translate := site.TplTr 调用（规避 Go 1.24+ printf vet 对
// 无可变参数非常量格式串的误报），语义与 site.TplTr 直接调用一致。
func s10CheckRespondSource(src string) error {
	if err := s10Require(src,
		`errors\.As\(err,\s*&wait\)`,
		`errors\.As\(err,\s*&attempts\)`,
		`err\.Error\(\),\s*wait\.Seconds\)`,
		`err\.Error\(\),\s*attempts\.Remaining\)`,
		`translated\s*:=\s*raw`,                         // R145：含 % 时保留原串→归约
		`translate\s*:=\s*site\.TplTr`,                  // 方法值来源
		`config\.StatusFailed`,                          // code 恒 StatusFailed（R81）
		`errors\.Is\(err,\s*sms\.ErrSmsSendFailed\)`,    // 平台原始码归约（R83/§10.2）
		`raw\s*:=\s*err\.Error\(\)`,                     // R145：先取原串
		`!strings\.Contains\(raw,\s*"%"`,                // R145：含 % 短路归约
		`translate\(raw\)`,                              // R144：字典查键
		`translated\s*!=\s*raw`,                         // R144：译文!=键 即真实键
		`translate\(sms\.ErrSmsSendFailed\.Error\(\)\)`, // R144：查不到键才归约
		`log\.Println\(`,                                // R144/R145：原始错误只进日志
	); err != nil {
		return err
	}
	// 不得出现「对普通错误传固定数字参数」的翻译调用（R82 反面）。
	if regexp.MustCompile(`(?:site\.TplTr|translate)\((?:err\.Error\(\)|raw),\s*\d+\)`).MatchString(src) {
		return errors.New("存在对普通错误传固定参数的翻译调用")
	}
	// R144：不得再引入手抄的业务键白名单（与字典漂移），字典就是白名单。
	if strings.Contains(src, "smsKnownBusinessErrors") || strings.Contains(src, "s10IsKnownSmsError") {
		return errors.New("引入了手抄的业务键白名单：应改为「译文 != 键」的字典判定（R144）")
	}
	return nil
}

// ---------------------------------------------------------------------------
// R82/R83 行为断言：真 HTTP 调用错误出口，解析 {code,msg}。
// ---------------------------------------------------------------------------

type s10SmsErrorExit func(iris.Context, *provider.Website, error)

func s10Capture(t *testing.T, err error, exit s10SmsErrorExit) string {
	t.Helper()
	// 强制 I18n 为 nil，使 TplTr 走 fmt.Sprintf(str, args...) 兜底：
	// 这样「对无格式化指令的键传参」会稳定输出 %!(EXTRA int=N)，
	// 正是 R82 记录的项目硬教训，可直接用于正向/反向钉住，且不依赖 locales 加载状态。
	old := provider.I18n
	provider.SetI18n(nil)
	defer provider.SetI18n(old)

	return s10CaptureOn(t, &provider.Website{}, err, exit)
}

// s10CaptureOn 与 s10Capture 同款真实 HTTP 调用，但翻译语义取决于调用方已注入的
// provider.I18n 与 site 语言——R144 的逐键保真断言必须看到真实译文。
func s10CaptureOn(t *testing.T, site *provider.Website, err error, exit s10SmsErrorExit) string {
	t.Helper()

	app := iris.New()
	app.Get("/x", func(ctx iris.Context) { exit(ctx, site, err) })
	if e := app.Build(); e != nil {
		t.Fatalf("构建测试应用失败：%v", e)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP 状态 = %d，期望 200", rec.Code)
	}
	var body struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if e := json.Unmarshal(rec.Body.Bytes(), &body); e != nil {
		t.Fatalf("解析响应失败：%v（body=%s）", e, rec.Body.String())
	}
	if body.Code != config.StatusFailed {
		t.Fatalf("code = %d，期望 config.StatusFailed(%d)", body.Code, config.StatusFailed)
	}
	return body.Msg
}

// s10CheckBehavior 对同一「错误出口」跑一组行为断言：
// WaitError 传 Seconds、AttemptsError 传 Remaining、其余不传参、SendError 不回显平台码。
func s10CheckBehavior(t *testing.T, exit s10SmsErrorExit) error {
	t.Helper()
	msgWait := s10Capture(t, &sms.WaitError{Seconds: 42}, exit)
	if !strings.Contains(msgWait, `%!(EXTRA int=42`) {
		return fmt.Errorf("WaitError 未把 Seconds=42 作为 TplTr 参数传入（msg=%q）", msgWait)
	}
	msgAttempts := s10Capture(t, &sms.AttemptsError{Remaining: 3}, exit)
	if !strings.Contains(msgAttempts, `%!(EXTRA int=3`) {
		return fmt.Errorf("AttemptsError 未把 Remaining=3 作为 TplTr 参数传入（msg=%q）", msgAttempts)
	}
	msgPlain := s10Capture(t, errors.New("SmsCodeNotFound"), exit)
	if strings.Contains(msgPlain, `%!(EXTRA`) {
		return fmt.Errorf("普通错误被多传了参数（msg=%q）", msgPlain)
	}
	msgSend := s10Capture(t, &sms.SendError{
		Class: sms.ClassPermanent, Code: "PLAT_SECRET_123", Message: "internal detail",
	}, exit)
	if strings.Contains(msgSend, "PLAT_SECRET_123") || strings.Contains(msgSend, "internal detail") {
		return fmt.Errorf("回显了平台原始错误码/消息（msg=%q）", msgSend)
	}
	return nil
}

// s10BadExit 是 R82/R83 的反面实现：对所有错误一律传固定参数 7，且不归约平台错误。
func s10BadExit(ctx iris.Context, site *provider.Website, err error) {
	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  site.TplTr(err.Error(), 7),
	})
}

// ---------------------------------------------------------------------------
// 简报 4 条源码级护栏测试（每条都带反向对照：合成坏源码必红、恢复绿）
// ---------------------------------------------------------------------------

func TestApiSmsSendVerifiesCaptcha(t *testing.T) {
	src := readFuncSource(t, "apiSms.go", "ApiSmsSend")
	if err := s10CheckCaptchaGuard(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	// 复用本包既有 mustContain 再钉一次关键锚点
	mustContain(t, src, `Store\.Verify\(\s*req\.CaptchaId\s*,\s*req\.Captcha\s*,\s*true\s*\)`,
		"Safe.Captcha==1 分支必须真正调用 Store.Verify")

	// 反向对照：合成一份「删掉 captcha 分支」的源码，喂同一检查器必红
	badSrc := `func ApiSmsSend(ctx iris.Context) {
		currentSite := provider.CurrentSite(ctx)
		var req request.ApiSmsSendRequest
		_ = ctx.ReadJSON(&req)
		_, err := currentSite.SendSmsCode(provider.SmsSendInput{})
		_ = err
	}`
	if err := s10CheckCaptchaGuard(badSrc); err == nil {
		t.Fatal("反向对照失败：删掉 captcha 分支后护栏仍判绿")
	}
	// 恢复绿
	if err := s10CheckCaptchaGuard(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

func TestApiSmsSendBindOldRequiresLogin(t *testing.T) {
	src := readFuncSource(t, "apiSms.go", "ApiSmsSend")
	if err := s10CheckBindOldGuard(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	mustContain(t, src, `GetUserInfoById\(`,
		"bind_old 必须取库内绑定号码，不得信任请求体自证")

	// 反向对照：只判登录、不判绑定号码（免费短信轰炸跳板）
	badSrc := `func ApiSmsSend(ctx iris.Context) {
		userId := ctx.Values().GetUintDefault("userId", 0)
		if userId == 0 {
			respondSmsError(ctx, currentSite, errors.New("Denied"))
			return
		}
		_, err := currentSite.SendSmsCode(provider.SmsSendInput{})
		_ = err
	}`
	if err := s10CheckBindOldGuard(badSrc); err == nil {
		t.Fatal("反向对照失败：只判登录不判号码的源码仍被判绿（免费短信轰炸跳板）")
	}
	if err := s10CheckBindOldGuard(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

func TestApiSmsSendRejectsTestScene(t *testing.T) {
	src := readFuncSource(t, "apiSms.go", "ApiSmsSend")
	if err := s10CheckSceneGuard(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	mustContain(t, src, `sms\.ErrSmsSceneNotEnabled`,
		"拒 test / notify. 前缀必须落到 ErrSmsSceneNotEnabled 出口")

	// 反向对照：只拒 test、漏掉 notify. 前缀
	badSrc := `func ApiSmsSend(ctx iris.Context) {
		if req.Scene == sms.SceneTest {
			respondSmsError(ctx, currentSite, sms.ErrSmsSceneNotEnabled)
			return
		}
	}`
	if err := s10CheckSceneGuard(badSrc); err == nil {
		t.Fatal("反向对照失败：漏掉 notify. 前缀的源码仍被判绿")
	}
	if err := s10CheckSceneGuard(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

func TestApiLoginDispatchesSms(t *testing.T) {
	src := readFuncSource(t, "apiUser.go", "ApiLogin")
	if err := s10CheckLoginDispatch(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}

	// 反向对照：没有 PlatformSms 分派、把错误吞成 LoginFailed 的源码
	badSrc := `func ApiLogin(ctx iris.Context) {
		user, err := currentSite.LoginViaPassword(&req)
		if err != nil {
			ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": currentSite.TplTr("LoginFailed")})
			return
		}
		_ = user
	}`
	if err := s10CheckLoginDispatch(badSrc); err == nil {
		t.Fatal("反向对照失败：无 PlatformSms 分派/吞掉错误的源码仍被判绿")
	}
	if err := s10CheckLoginDispatch(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

// ---------------------------------------------------------------------------
// R82 裁决追加：respondSmsError 的源码护栏 + 行为断言（各带反向对照）
// ---------------------------------------------------------------------------

func TestApiSmsSendRespondErrorTplTrArgs(t *testing.T) {
	src := readFuncSource(t, "apiSms.go", "respondSmsError")
	if err := s10CheckRespondSource(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	mustNotContain(t, src, `TplTr\("LoginFailed"\)`,
		"R81：短信错误出口不得把错误吞成通用 LoginFailed")

	// 源码级反向对照：合成坏源码（无 errors.As 分派、对所有错误传固定参、不归约平台错误）
	badSrc := `func respondSmsError(ctx iris.Context, site *provider.Website, err error) {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": site.TplTr(err.Error(), 7)})
	}`
	if err := s10CheckRespondSource(badSrc); err == nil {
		t.Fatal("反向对照失败：合成坏源码仍被护栏判绿")
	}
	// R144 反向对照：删掉字典查键（把译文直接取成原串），等于回到 R139 的一刀切，必红。
	noDict := strings.Replace(src, "translated = translate(raw)", "translated = raw", 1)
	if noDict == src {
		t.Fatal("反向对照准备失败：真实源码里没有 R144 的字典查键")
	}
	if err := s10CheckRespondSource(noDict); err == nil {
		t.Fatal("反向对照失败：删掉字典查键后的源码仍被判绿")
	}
	// R145 反向对照：把「含 % 短路」改成恒真（等于不短路），必红。
	noPct := strings.Replace(src, `if !strings.Contains(raw, "%") {`, "if true {", 1)
	if noPct == src {
		t.Fatal("反向对照准备失败：真实源码里没有 R145 的 % 短路")
	}
	if err := s10CheckRespondSource(noPct); err == nil {
		t.Fatal("反向对照失败：删掉 % 短路后的源码仍被判绿")
	}
	if err := s10CheckRespondSource(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}

	// 行为断言：真实实现绿、合成坏实现红、恢复绿
	if err := s10CheckBehavior(t, respondSmsError); err != nil {
		t.Fatalf("R82/R83 行为断言失败：%v", err)
	}
	if err := s10CheckBehavior(t, s10BadExit); err == nil {
		t.Fatal("行为断言反向对照失败：合成坏实现仍被判绿")
	}
	if err := s10CheckBehavior(t, respondSmsError); err != nil {
		t.Fatalf("行为断言恢复绿失败：%v", err)
	}
}

// ---------------------------------------------------------------------------
// R144：出口改为「翻译字典即白名单」——逐键保真 + 泄露关闭
// ---------------------------------------------------------------------------

// s10LoadTestI18n 注入真实语言包（controller 测试进程的 ExecPath 不含 locales，
// 故用相对路径 glob 加载）。逐键断言必须看到真实译文，不能靠 nil 兜底。
func s10LoadTestI18n(t *testing.T) {
	t.Helper()
	app := iris.New()
	if err := app.I18n.Load("../locales/*/*.yml", config.LoadLocales()...); err != nil {
		t.Fatalf("加载 locales 失败：%v", err)
	}
	old := provider.I18n
	provider.SetI18n(app.I18n)
	t.Cleanup(func() { provider.SetI18n(old) })
}

// s10TestSite 返回一个可安全调用 TplTr 的最小站点。Website.System 是
// *config.SystemConfig，空指针会在 TplTr 里被解引用（I18n 非空时读 w.System.Language）。
func s10TestSite() *provider.Website {
	return &provider.Website{System: &config.SystemConfig{}}
}

// TestSmsRespondErrorPreservesBusinessKeys 是 R144 的核心断言：
// 业务键必须原样输出译文，不得被归约成「短信发送失败」；只有查不到键的才归约。
func TestSmsRespondErrorPreservesBusinessKeys(t *testing.T) {
	s10LoadTestI18n(t)
	site := s10TestSite()
	// 方法值：规避 Go 1.24+ printf vet 对「无可变参数非常量格式串」的误报。
	translate := site.TplTr
	generic := translate(sms.ErrSmsSendFailed.Error())
	if generic == sms.ErrSmsSendFailed.Error() {
		t.Fatalf("locales 缺 SmsSendFailed 译文，逐键断言会失去意义（generic=%q）", generic)
	}

	cases := []struct {
		key string
		err error
	}{
		// 不带 %d 的 Sms* 键（SmsResendTooSoon / SmsCodeMismatch 带 %d，见下方类型分支）。
		{"SmsServiceNotEnabled", sms.ErrSmsServiceNotEnabled},
		{"SmsSceneNotEnabled", sms.ErrSmsSceneNotEnabled},
		{"SmsPhoneInvalid", sms.ErrSmsPhoneInvalid},
		{"SmsRegionUnsupported", sms.ErrSmsRegionUnsupported},
		{"SmsCredentialMissing", sms.ErrSmsCredentialMissing},
		{"SmsSignMissing", sms.ErrSmsSignMissing},
		{"SmsTemplateMissing", sms.ErrSmsTemplateMissing},
		{"SmsIpLimited", sms.ErrSmsIpLimited},
		{"SmsPhoneLimited", sms.ErrSmsPhoneLimited},
		{"SmsCodeNotFound", sms.ErrSmsCodeNotFound},
		{"SmsCodeExpired", sms.ErrSmsCodeExpired},
		{"SmsCodeLocked", sms.ErrSmsCodeLocked},
		{"SmsSendFailed", sms.ErrSmsSendFailed},
		// 非 Sms* 前缀的业务键：R139 的硬编码归约把它们全吞了，R144 必须放行。
		{"GraphicCodeIncorrect", errors.New("GraphicCodeIncorrect")},
		{"PleaseFillInAPasswordOfMoreThan6Digits", errors.New("PleaseFillInAPasswordOfMoreThan6Digits")},
		// R145：三语补了 UserNotActive，这里从「字典注入」改为真键断言。
		{"UserNotActive", errors.New("UserNotActive")},
	}

	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			want := translate(c.key)
			if want == c.key {
				t.Fatalf("locales 缺 %q 的译文，断言会失去意义", c.key)
			}
			if c.key != sms.ErrSmsSendFailed.Error() && want == generic {
				t.Fatalf("%q 的译文与 SmsSendFailed 相同（%q），断言会失去意义", c.key, want)
			}
			got := s10CaptureOn(t, site, c.err, respondSmsError)
			if got != want {
				t.Fatalf("出口 msg = %q，期望 %q（业务键被吞或被改写）", got, want)
			}
		})
	}

	// 带 %d 的两条键走类型分支，同样必须输出译文而不是键名或通用文案。
	t.Run("SmsResendTooSoon 带秒数", func(t *testing.T) {
		got := s10CaptureOn(t, site, &sms.WaitError{Seconds: 42}, respondSmsError)
		if want := translate(sms.ErrSmsResendTooSoon.Error(), 42); got != want {
			t.Fatalf("出口 msg = %q，期望 %q", got, want)
		}
	})
	t.Run("SmsCodeMismatch 带剩余次数", func(t *testing.T) {
		got := s10CaptureOn(t, site, &sms.AttemptsError{Remaining: 3}, respondSmsError)
		if want := translate(sms.ErrSmsCodeMismatch.Error(), 3); got != want {
			t.Fatalf("出口 msg = %q，期望 %q", got, want)
		}
	})
}

// TestApiSmsRespondHasNoHandWrittenWhitelist 守 R144 的「字典即白名单」：
// 不得再引入 smsKnownBusinessErrors / s10IsKnownSmsError 这类手抄表（会与字典漂移）。
func TestApiSmsRespondHasNoHandWrittenWhitelist(t *testing.T) {
	raw, err := os.ReadFile("apiSms.go")
	if err != nil {
		t.Fatalf("读取 apiSms.go 失败：%v", err)
	}
	for _, banned := range []string{"smsKnownBusinessErrors", "s10IsKnownSmsError"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("apiSms.go 仍含手抄白名单 %q：应改为「译文 != 键」的字典判定（R144）", banned)
		}
	}
}

// TestSmsRespondErrorReducesUnknownErrors 是 R139/R144 的泄露关闭断言。
func TestSmsRespondErrorReducesUnknownErrors(t *testing.T) {
	t.Run("非法JSON不回显解析器措辞", func(t *testing.T) {
		app := iris.New()
		// CurrentSite(ctx) 里的 ctx.GetLocale() 依赖 app.I18n；不加载 locales 会空指针 panic
		// （与 middleware/accountProbe_test.go 的 newProbeTestApp 同款）。
		if err := app.I18n.Load("../locales/*/*.yml", config.LoadLocales()...); err != nil {
			t.Fatalf("加载 locales 失败：%v", err)
		}
		app.I18n.SetDefault("zh-CN")
		// 生产启动时 bootstrap 会把 app.I18n 注入 provider；测试同款注入，
		// 才能断言「归约后输出的是 SmsSendFailed 的译文」而不是键名。
		old := provider.I18n
		provider.SetI18n(app.I18n)
		defer provider.SetI18n(old)

		app.Post("/sms/send", ApiSmsSend)
		if err := app.Build(); err != nil {
			t.Fatalf("构建测试应用失败：%v", err)
		}
		const rawBody = `{"scene": not-json`
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/sms/send", strings.NewReader(rawBody))
		req.Header.Set("Content-Type", "application/json")
		app.ServeHTTP(rec, req)

		body := rec.Body.String()
		for _, leak := range []string{"invalid character", "cannot unmarshal", "unexpected end of JSON", "looking for beginning"} {
			if strings.Contains(body, leak) {
				t.Errorf("响应回显了 JSON 解析器措辞 %q：%s", leak, body)
			}
		}
		if strings.Contains(body, "not-json") {
			t.Errorf("响应回显了原始请求文本：%s", body)
		}
		var resp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应失败：%v（body=%s）", err, body)
		}
		if resp.Code != config.StatusFailed {
			t.Errorf("code = %d，期望 %d", resp.Code, config.StatusFailed)
		}
		site := s10TestSite()
		translate := site.TplTr
		want := translate(sms.ErrSmsSendFailed.Error())
		if want == sms.ErrSmsSendFailed.Error() {
			t.Fatalf("locales 缺 SmsSendFailed 译文，断言会失去意义")
		}
		if resp.Msg != want {
			t.Errorf("msg = %q，期望归约文案 %q", resp.Msg, want)
		}
	})

	t.Run("DB错误归约为SmsSendFailed", func(t *testing.T) {
		s10LoadTestI18n(t)
		site := s10TestSite()
		translate := site.TplTr
		want := translate(sms.ErrSmsSendFailed.Error())
		if want == sms.ErrSmsSendFailed.Error() {
			t.Fatalf("locales 缺 SmsSendFailed 译文，断言会失去意义")
		}
		synthetic := errors.New("Error 1406: Data too long for column 'template_id' at row 1")
		got := s10CaptureOn(t, site, synthetic, respondSmsError)
		if got != want {
			t.Fatalf("合成 DB 错误应归约为 %q，实际 %q", want, got)
		}
		for _, leak := range []string{"1406", "Data too long", "template_id", "Error "} {
			if strings.Contains(got, leak) {
				t.Errorf("响应回显了 DB 内部措辞 %q：%q", leak, got)
			}
		}
	})

	t.Run("含百分号的错误串归约为SmsSendFailed", func(t *testing.T) {
		// R145：TplTr 兜底是 fmt.Sprintf，含 % 的文本会被改写成 %!c(MISSING)，
		// 改写后必然 != 原串 —— 若不加短路，这里会被误判成语言键、原文照旧出门。
		s10LoadTestI18n(t)
		site := s10TestSite()
		translate := site.TplTr
		want := translate(sms.ErrSmsSendFailed.Error())
		if want == sms.ErrSmsSendFailed.Error() {
			t.Fatalf("locales 缺 SmsSendFailed 译文，断言会失去意义")
		}
		got := s10CaptureOn(t, site, errors.New("invalid character '%' looking for beginning of value"), respondSmsError)
		if got != want {
			t.Fatalf("含 %% 的错误应归约为 %q，实际 %q", want, got)
		}
		// 响应体只有 code/msg 两个字段，msg 即唯一文本出口，检查它就等于检查 body。
		for _, leak := range []string{"invalid character", "%!"} {
			if strings.Contains(got, leak) {
				t.Errorf("响应回显了内部措辞 %q：%q", leak, got)
			}
		}
	})
}

// s10CheckResetStepOrder 钉住 ApiSmsPasswordReset 的步骤顺序：
// ReadJSON → 密码长度 → NormalizePhone → VerifySmsCode(reset) → 按号码查用户
// → EncryptPassword → DB.Save。provider 侧的 TestSmsFullResetJourney 按同一
// 顺序跑真库；本护栏保证 handler 没有偏离那条路径。
func s10CheckResetStepOrder(src string) error {
	steps := []string{
		"ctx.ReadJSON(&req)",
		"sms.NormalizePhone(",
		"sms.SceneReset",
		"GetUserInfoByPhoneCountry(",
		"EncryptPassword(",
		"currentSite.DB.Save(user)",
	}
	prev := -1
	for _, step := range steps {
		i := strings.Index(src, step)
		if i < 0 {
			return fmt.Errorf("缺步骤 %q", step)
		}
		if i <= prev {
			return fmt.Errorf("步骤顺序错误：%q 出现在前一阶段之前", step)
		}
		prev = i
	}
	return nil
}

// TestSmsResetHandlerStepOrder：真实源码绿 / 合成「删掉 EncryptPassword」源码红 / 恢复绿。
func TestSmsResetHandlerStepOrder(t *testing.T) {
	src := readFuncSource(t, "apiSms.go", "ApiSmsPasswordReset")
	if err := s10CheckResetStepOrder(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	bad := strings.Replace(src, "if err = user.EncryptPassword(req.Password); err != nil {", "", 1)
	if bad == src {
		t.Fatal("反向对照准备失败：真实源码里没有 EncryptPassword 调用")
	}
	if err := s10CheckResetStepOrder(bad); err == nil {
		t.Fatal("反向对照失败：删掉 EncryptPassword 的源码仍判绿")
	}
	if err := s10CheckResetStepOrder(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}
