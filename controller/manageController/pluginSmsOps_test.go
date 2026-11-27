package manageController

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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

// ---------------------------------------------------------------------------
// 任务 13：后台测试发送 + 发送记录查询（TDD 源码级护栏 + 行为断言）。
//
// go test 的工作目录是 controller/manageController/，读仓库其它目录一律用 ../../。
// 复用同包 pluginSms_test.go 的 smsFuncSource（按函数名抽取函数源码）。
// 每条护栏三段式：真实源码绿 → 合成坏源码红 → 恢复绿；不写死端点总数。
// ---------------------------------------------------------------------------

const (
	s13PluginSmsFile  = "pluginSms.go"
	s13SmsServiceFile = "../../provider/smsService.go"
	s13RouteFile      = "../../route/manage.go"
)

// s13Require 逐一要求源码命中正则锚点，缺任一即报错（避免检查器空转恒绿）。
func s13Require(src string, specs ...string) error {
	for _, s := range specs {
		if !regexp.MustCompile(s).MatchString(src) {
			return fmt.Errorf("缺源码锚点 %s", s)
		}
	}
	return nil
}

// s13LoadRoute 读完整源码文件（route/manage.go 自带 package 头）。
func s13LoadRoute(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", path, err)
	}
	return string(b)
}

// s13ParseRouteFile 把完整源码字符串解析成 AST。
func s13ParseRouteFile(src string) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, "route.go", src, 0)
}

// ---------------------------------------------------------------------------
// 检查器
// ---------------------------------------------------------------------------

// s13CheckTestUsesLoginTemplate：PluginSmsTest 必须定 region、取该 region 的
// login 模板（空则明确报 ErrSmsTemplateMissing），且发码 scene 固定为 test；
// 绝不为塞 test 模板去改 currentSite.PluginSms 内存真值（§7.2 / R113）。
func s13CheckTestUsesLoginTemplate(src string) error {
	if err := s13Require(src,
		`sms\.NormalizePhone\(`,
		`\.Region\(\)`,
		`sms\.Resolve\(currentSite\.PluginSms,\s*region,\s*sms\.SceneLogin\)`,
		`sms\.ErrSmsTemplateMissing`,
		`Scene:\s*sms\.SceneTest`,
	); err != nil {
		return err
	}
	if strings.Contains(src, "PluginSms =") || strings.Contains(src, "*currentSite.PluginSms =") {
		return errors.New("handler 改了 currentSite.PluginSms 内存真值（禁止为塞 test 模板污染真值）")
	}
	return nil
}

// s13CheckTestUsesSendSmsCode：test 端点必须走同一条 SendSmsCode 生产入口
// （内含 IP 与号码日限、且第 2 步跳过 test 的 Scenes 白名单），不得旁路自己发。
func s13CheckTestUsesSendSmsCode(src string) error {
	if err := s13Require(src,
		`currentSite\.SendSmsCode\(`,
		`Scene:\s*sms\.SceneTest`,
		`Ip:\s*provider\.SmsClientIP\(ctx[^)]*\)`,
		`UserId:\s*0`,
	); err != nil {
		return err
	}
	if strings.Contains(src, "sendSmsCodeWithSender") || strings.Contains(src, "sms.NewSender") {
		return errors.New("handler 绕过 SendSmsCode 生产入口（疑似旁路 IP/号码限额）")
	}
	return nil
}

// s13CheckErrorDispatch：R114 错误文案精确分派。平台 *SendError 归约到通用键
// （不回显原始码/消息）；仅 *WaitError 传 Seconds、*AttemptsError 传 Remaining；
// 其余错误交给统一判据 provider.SmsBusinessMessage（内部 TplTr + 字典即白名单）。
func s13CheckErrorDispatch(src string) error {
	if err := s13Require(src,
		`errors\.Is\(err,\s*sms\.ErrSmsSendFailed\)`, // 平台原始码归约
		`errors\.As\(err,\s*&wait\)`,
		`errors\.As\(err,\s*&attempts\)`,
		`err\.Error\(\),\s*wait\.Seconds\)`,
		`err\.Error\(\),\s*attempts\.Remaining\)`,
		`translate\s*:=\s*currentSite\.Tr`,                   // 类型分支的翻译出口
		`provider\.SmsBusinessMessage\(currentSite,\s*err\)`, // 统一判据（TplTr，不做品牌替换）
		`translate\(sms\.ErrSmsSendFailed\.Error\(\)\)`,      // 查不到键才归约
		`log\.Println\("后台短信端点内部错误：",\s*err\)`,               // R146②：原始错误只进日志
		`config\.StatusFailed`,
	); err != nil {
		return err
	}
	// 反面：不得出现「对所有错误统一传固定数字参数」的翻译调用。
	if regexp.MustCompile(`(?:currentSite\.Tr|translate)\((?:err\.Error\(\)|raw),\s*\d+\)`).MatchString(src) {
		return errors.New("存在对所有错误传固定数字参数的翻译调用（应只对 WaitError/AttemptsError 精确传参）")
	}
	// R146①：不得再引入手抄的业务键白名单（漏了 SmsCodeLocked，且会与字典漂移）。
	if strings.Contains(src, "smsOpsKnownBusinessErrors") || strings.Contains(src, "s13IsKnownSmsError") {
		return errors.New("引入了手抄的业务键白名单：应改为「Tr(raw) != raw」的字典判定（R146）")
	}
	return nil
}

// s13BadDispatchSrc 是 R114 的反面实现：对所有错误一律传固定参数 7、且不归约平台错误、
// 不做 errors.As 精确分派。喂 s13CheckErrorDispatch 必须判红。
const s13BadDispatchSrc = `package manageController
func respondSmsOpsError(ctx iris.Context, currentSite *provider.Website, err error) {
	ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": currentSite.Tr(err.Error(), 7)})
}
`

// s13BehaviorTrFallback 用真实 provider.Website.Tr 验证 R114 依赖的三条语义
// （R82：Tr 兜底是 str+fmt.Sprint 拼接，比 TplTr 的 fmt.Sprintf 更宽容）：
//  1. 对普通错误多传参会污染文案（故「对所有错误都传参」必被源码护栏判红）；
//  2. WaitError.Seconds / AttemptsError.Remaining 只有传入才会出现在文案里；
//  3. *sms.SendError 经 errors.Is 归约到 ErrSmsSendFailed，归约键不含平台原始码/消息。
func s13BehaviorTrFallback(t *testing.T) error {
	t.Helper()
	old := provider.I18n
	provider.SetI18n(nil) // w.tr 会自初始化并 load locales；locales 无短信键 → 走兜底拼接
	defer provider.SetI18n(old)
	site := &provider.Website{}
	translate := site.Tr

	// 用合成键强制走兜底，避免 locales 命中影响断言。
	const plainKey = "S13PlainKey"
	base := translate(plainKey)
	if got := translate(plainKey, 7); got == base || !strings.Contains(got, "7") {
		return fmt.Errorf("Tr 兜底语义异常：对普通错误传 7 应污染文案，got=%q base=%q", got, base)
	}

	wait := &sms.WaitError{Seconds: 42}
	if got := translate(wait.Error(), wait.Seconds); !strings.Contains(got, "42") {
		return fmt.Errorf("WaitError Seconds 未进入文案：%q", got)
	}
	if got := translate(wait.Error()); strings.Contains(got, "42") {
		return fmt.Errorf("WaitError 不传参却出现 42：%q", got)
	}

	attempts := &sms.AttemptsError{Remaining: 3}
	if got := translate(attempts.Error(), attempts.Remaining); !strings.Contains(got, "3") {
		return fmt.Errorf("AttemptsError Remaining 未进入文案：%q", got)
	}

	send := &sms.SendError{Class: sms.ClassPermanent, Code: "PLAT_SECRET_123", Message: "internal detail"}
	if !errors.Is(send, sms.ErrSmsSendFailed) {
		return errors.New("*SendError 未 Unwrap 到 ErrSmsSendFailed，无法归约平台原始码")
	}
	if reduced := translate(sms.ErrSmsSendFailed.Error()); strings.Contains(reduced, "PLAT_SECRET_123") || strings.Contains(reduced, "internal detail") {
		return fmt.Errorf("归约后文案仍回显平台原始码/消息：%q", reduced)
	}
	return nil
}

// s13CheckLogFilters：六个筛选各自有独立 Where 锚点；GetSmsLogList 恒带
// site_id = w.Id 基底、created_time DESC、分页、filter(tx) 叠加。
func s13CheckLogFilters(handlerSrc, serviceSrc string) error {
	if err := s13Require(serviceSrc,
		`site_id\s*=\s*\?`,
		`w\.Id`,
		`created_time\s+DESC`,
		`\[\]\*model\.SmsLog`,
		`filter\(tx\)`,
		`Limit\(pageSize\)`,
		`Offset\(offset\)`,
	); err != nil {
		return err
	}
	if err := s13Require(handlerSrc,
		`Where\("scene\s*=\s*\?"`,
		`Where\("provider\s*=\s*\?"`,
		`Where\("status\s*=\s*\?"`,
		`Where\("phone\s*=\s*\?"`,
		`Where\("created_time\s*>=\s*\?"`,
		`Where\("created_time\s*<=\s*\?"`,
	); err != nil {
		return err
	}
	return nil
}

// s13CheckNoCodeInLogList：返回结构必须是 []*model.SmsLog，源码不得出现
// 任何泄码痕迹（CodeHash / code_hash / model.SmsCode）。
func s13CheckNoCodeInLogList(src string) error {
	if !strings.Contains(src, "[]*model.SmsLog") {
		return errors.New("GetSmsLogList 返回类型不是 []*model.SmsLog")
	}
	for _, bad := range []string{"CodeHash", "code_hash", "model.SmsCode"} {
		if strings.Contains(src, bad) {
			return fmt.Errorf("GetSmsLogList 源码出现泄码痕迹 %q", bad)
		}
	}
	return nil
}

// s13CheckSmsParty：go/ast 精确解析 /sms party 的权限中间件必须是
// AdminPermissionRequired（不是 AdminPermission），且两条新路由挂在 sms party 上。
func s13CheckSmsParty(src string) error {
	file, err := s13ParseRouteFile(src)
	if err != nil {
		return fmt.Errorf("解析路由源码失败：%w", err)
	}
	foundParty, foundRequired, foundPlain := false, false, false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Party" || len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Value != `"/sms"` {
			return true
		}
		foundParty = true
		for _, a := range call.Args[1:] {
			s, ok := a.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			switch s.Sel.Name {
			case "AdminPermissionRequired":
				foundRequired = true
			case "AdminPermission":
				foundPlain = true
			}
		}
		return true
	})
	if !foundParty {
		return errors.New("未找到 /sms party")
	}
	if !foundRequired {
		return errors.New("/sms party 未挂 middleware.AdminPermissionRequired")
	}
	if foundPlain {
		return errors.New("/sms party 误挂 middleware.AdminPermission（应为 AdminPermissionRequired）")
	}
	if err := s13Require(src,
		`sms\.Post\("/test",\s*manageController\.PluginSmsTest\)`,
		`sms\.Get\("/logs",\s*manageController\.PluginSmsLogs\)`,
	); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// 五条测试（计划步骤 1 逐条）
// ---------------------------------------------------------------------------

// TestPluginSmsTestUsesLoginTemplate：test 场景取该 region 下 login 模板；
// login 模板为空时明确报错 ErrSmsTemplateMissing，不静默套用别的场景。
// 附带 R114 错误文案精确分派：源码级护栏 + 行为断言 + 反向对照。
func TestPluginSmsTestUsesLoginTemplate(t *testing.T) {
	real := smsFuncSource(t, s13PluginSmsFile, "PluginSmsTest")

	// (A) login 模板护栏三段式
	if err := s13CheckTestUsesLoginTemplate(real); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	t.Run("反向对照_套用test场景必红", func(t *testing.T) {
		bad := strings.Replace(real, "sms.SceneLogin", "sms.SceneTest", 1)
		if bad == real {
			t.Fatal("反向对照锚点失效：源码里没找到 sms.SceneLogin")
		}
		if err := s13CheckTestUsesLoginTemplate(bad); err == nil {
			t.Fatal("反向对照失败：把 login 场景换成 test 后护栏仍判绿")
		}
		t.Log("反向对照：静默套用 test 场景喂同一检查器 → 判违规（红）")
	})
	t.Run("反向对照_删掉明确报错必红", func(t *testing.T) {
		bad := strings.ReplaceAll(real, "sms.ErrSmsTemplateMissing", "sms.ErrSmsPhoneInvalid")
		if bad == real {
			t.Fatal("反向对照锚点失效：源码里没找到 sms.ErrSmsTemplateMissing")
		}
		if err := s13CheckTestUsesLoginTemplate(bad); err == nil {
			t.Fatal("反向对照失败：删掉 ErrSmsTemplateMissing 明确报错后护栏仍判绿")
		}
		t.Log("反向对照：无 ErrSmsTemplateMissing 明确报错喂同一检查器 → 判违规（红）")
	})
	if err := s13CheckTestUsesLoginTemplate(real); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
	t.Log("恢复：带 login 模板检查的真实源码喂同一检查器 → 判合规（绿）")

	// (B) R114 错误文案精确分派：源码级护栏 + 行为断言 + 反向对照
	t.Run("R114错误文案精确分派", func(t *testing.T) {
		errSrc := smsFuncSource(t, s13PluginSmsFile, "respondSmsOpsError")
		if err := s13CheckErrorDispatch(errSrc); err != nil {
			t.Fatalf("R114 真实源码护栏失败：%v", err)
		}
		if err := s13CheckErrorDispatch(s13BadDispatchSrc); err == nil {
			t.Fatal("R114 反向对照失败：合成『对所有错误都传参』的源码仍判绿")
		}
		t.Log("R114 反向对照：合成『对所有错误一律 Tr(err.Error(), 7)』喂同一检查器 → 判违规（红）")
		if err := s13CheckErrorDispatch(errSrc); err != nil {
			t.Fatalf("R114 恢复绿失败：%v", err)
		}
		if err := s13BehaviorTrFallback(t); err != nil {
			t.Fatalf("R114 行为断言失败：%v", err)
		}
		t.Log("R114 恢复绿 + 行为断言：精确分派源码判合规；Tr 兜底拼接 / SendError 归约均符合预期")
	})
}

// TestPluginSmsTestBypassesSceneWhitelistOnly：Scenes 为空时 test 仍可发
// （生产入口第 2 步跳过 test 白名单），但号码日限与 IP 限额仍生效——
// 源码级断言走的是同一条 SendSmsCode 链路，不是旁路。
func TestPluginSmsTestBypassesSceneWhitelistOnly(t *testing.T) {
	real := smsFuncSource(t, s13PluginSmsFile, "PluginSmsTest")
	if err := s13CheckTestUsesSendSmsCode(real); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	t.Run("反向对照_改成测试接缝旁路必红", func(t *testing.T) {
		bad := strings.Replace(real, "currentSite.SendSmsCode(", "currentSite.sendSmsCodeWithSender(", 1)
		if bad == real {
			t.Fatal("反向对照锚点失效：源码里没找到 SendSmsCode 调用")
		}
		if err := s13CheckTestUsesSendSmsCode(bad); err == nil {
			t.Fatal("反向对照失败：换成 sendSmsCodeWithSender 旁路后护栏仍判绿")
		}
		t.Log("反向对照：旁路 SendSmsCode 生产入口喂同一检查器 → 判违规（红）")
	})
	t.Run("反向对照_去掉Ip限额入参必红", func(t *testing.T) {
		re := regexp.MustCompile(`Ip:\s*provider\.SmsClientIP\(ctx[^)]*\),`)
		bad := re.ReplaceAllString(real, "")
		if bad == real {
			t.Fatal("反向对照锚点失效：源码里没找到 Ip 入参")
		}
		if err := s13CheckTestUsesSendSmsCode(bad); err == nil {
			t.Fatal("反向对照失败：去掉 Ip 入参后护栏仍判绿")
		}
		t.Log("反向对照：去掉 Ip（IP 限额依赖）入参喂同一检查器 → 判违规（红）")
	})
	if err := s13CheckTestUsesSendSmsCode(real); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
	t.Log("恢复：走 SendSmsCode 生产入口且带 Ip/UserId 的真实源码 → 判合规（绿）")
}

// TestPluginSmsLogsFilters：表驱动断言六个筛选各自生效，且恒带 site_id 基底。
func TestPluginSmsLogsFilters(t *testing.T) {
	handler := smsFuncSource(t, s13PluginSmsFile, "PluginSmsLogs")
	service := smsFuncSource(t, s13SmsServiceFile, "GetSmsLogList")
	if err := s13CheckLogFilters(handler, service); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}

	cases := []struct {
		name  string
		where string
	}{
		{"scene", `tx = tx.Where("scene = ?", scene)`},
		{"provider", `tx = tx.Where("provider = ?", providerName)`},
		{"status", `tx = tx.Where("status = ?", filters.status)`},
		{"phone", `tx = tx.Where("phone = ?", phone)`},
		{"start_time", `tx = tx.Where("created_time >= ?", filters.startTS)`},
		{"end_time", `tx = tx.Where("created_time <= ?", filters.endTS)`},
	}
	for _, c := range cases {
		t.Run("筛选_"+c.name, func(t *testing.T) {
			bad := strings.Replace(handler, c.where, "", 1)
			if bad == handler {
				t.Fatalf("反向对照锚点失效：源码里没找到 %q", c.where)
			}
			if err := s13CheckLogFilters(bad, service); err == nil {
				t.Fatal("反向对照失败：删掉该筛选后护栏仍判绿")
			}
			if err := s13CheckLogFilters(handler, service); err != nil {
				t.Fatalf("恢复绿失败：%v", err)
			}
			t.Logf("筛选 %s：删掉该 Where 判红 / 恢复源码判绿 ✓", c.name)
		})
	}

	// 恒带 site_id = w.Id 基底（多站点隔离）：合成删掉必红、恢复绿。
	t.Run("恒带site_id", func(t *testing.T) {
		bad := strings.Replace(service, `Where("site_id = ?", w.Id)`, "", 1)
		if bad == service {
			t.Fatal("反向对照锚点失效：源码里没找到 site_id 基底")
		}
		if err := s13CheckLogFilters(handler, bad); err == nil {
			t.Fatal("反向对照失败：删掉 site_id = w.Id 基底后护栏仍判绿")
		}
		if err := s13CheckLogFilters(handler, service); err != nil {
			t.Fatalf("恢复绿失败：%v", err)
		}
		t.Log("恒带 site_id：删掉基底判红 / 恢复源码判绿 ✓")
	})
}

// TestPluginSmsLogsNeverReturnsCode：go/ast 断言 GetSmsLogList 返回结构不含码字段。
//
// R120②：这里曾经序列化手工构造的 []*model.SmsLog 字面量（恒绿且证明不了真实
// 返回值）。真库 + 假 sender 的端到端 JSON 复核需要 DB 与未导出的测试接缝，
// 而本包测试进程不加载 config.json、拿不到 DB，故真实序列化复核已迁到
// provider/sms_integration_test.go 的 TestSmsLogListJSONHasNoCodeOrPlaintext。
func TestPluginSmsLogsNeverReturnsCode(t *testing.T) {
	service := smsFuncSource(t, s13SmsServiceFile, "GetSmsLogList")
	if err := s13CheckNoCodeInLogList(service); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	t.Run("反向对照_合成带码字段必红", func(t *testing.T) {
		bad := strings.Replace(service, "[]*model.SmsLog", "[]*model.SmsCode /* CodeHash code_hash */", 1)
		if bad == service {
			t.Fatal("反向对照锚点失效：源码里没找到 []*model.SmsLog")
		}
		if err := s13CheckNoCodeInLogList(bad); err == nil {
			t.Fatal("反向对照失败：合成含 CodeHash / model.SmsCode 的源码仍判绿")
		}
		t.Log("反向对照：返回 SmsCode / 含 CodeHash 喂同一检查器 → 判违规（红）")
	})
	if err := s13CheckNoCodeInLogList(service); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
	t.Log("恢复绿：GetSmsLogList 返回 SmsLog、源码无码字段 ✓" +
		"（真实 JSON 序列化在 provider 包 TestSmsLogsJSONNeverExposesCodeHash）")
}

// s13CheckTestReadJSONExit（R120①）：PluginSmsTest 的 ReadJSON 失败必须走
// respondSmsOpsError，不得直接 ctx.JSON 回显 err.Error()。
func s13CheckTestReadJSONExit(src string) error {
	if err := s13Require(src,
		`if err := ctx\.ReadJSON\(&req\); err != nil \{`,
	); err != nil {
		return err
	}
	// 只截取 ReadJSON 失败分支（到该分支的 return 为止）：全函数里还有其它
	// respondSmsOpsError 调用，不截断会让合成坏源码被后面的分支“救活”。
	start := strings.Index(src, "if err := ctx.ReadJSON(&req); err != nil {")
	if start < 0 {
		return errors.New("未找到 ReadJSON 失败分支")
	}
	rest := src[start:]
	end := strings.Index(rest, "return")
	if end < 0 {
		return errors.New("ReadJSON 失败分支结构异常（未找到 return）")
	}
	branch := rest[:end]
	// R146③：请求体不合法的语义是「参数错误」，必须直接回 SmsParamInvalid，
	// 不能再经 ops 归约（那样会变成「短信发送失败」），也不得回显解析器文本。
	if !strings.Contains(branch, `currentSite.Tr("SmsParamInvalid")`) {
		return errors.New("ReadJSON 失败未返回 SmsParamInvalid（R146③：参数错误语义）")
	}
	if strings.Contains(branch, "respondSmsOpsError") {
		return errors.New("ReadJSON 失败仍走 ops 归约（会被误报成「短信发送失败」）")
	}
	if strings.Contains(branch, "err.Error()") {
		return errors.New("ReadJSON 失败分支仍把 err.Error() 直接写进响应")
	}
	return nil
}

// TestPluginSmsTestReadJSONReturnsParamInvalid 三段式：真实源码绿 / 合成回显
// err.Error() 的源码红 / 恢复绿（R146③：参数错误不回「短信发送失败」）。
func TestPluginSmsTestReadJSONReturnsParamInvalid(t *testing.T) {
	real := smsFuncSource(t, s13PluginSmsFile, "PluginSmsTest")
	if err := s13CheckTestReadJSONExit(real); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	bad := strings.Replace(real, `currentSite.Tr("SmsParamInvalid")`, "err.Error()", 1)
	if bad == real {
		t.Fatal("反向对照锚点失效：源码里没找到 currentSite.Tr(\"SmsParamInvalid\")")
	}
	if err := s13CheckTestReadJSONExit(bad); err == nil {
		t.Fatal("反向对照失败：ReadJSON 失败直接把 err.Error() 写进响应的源码仍判绿")
	}
	// 第二个反面对照：退回旧行为（走 ops 归约 → 被误报成发送失败）也必须判红。
	oldBehavior := strings.Replace(real, `currentSite.Tr("SmsParamInvalid")`,
		"currentSite.Tr(\"SmsSendFailed\")", 1)
	if err := s13CheckTestReadJSONExit(oldBehavior); err == nil {
		t.Fatal("反向对照失败：ReadJSON 失败退回通用文案的源码仍判绿")
	}
	if err := s13CheckTestReadJSONExit(real); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

// s13CheckLogParamValidation（R119②/R120③）：logs handler 必须先经
// smsParseLogFilters 校验，非法时返回 SmsParamInvalid，且校验在查询之前。
func s13CheckLogParamValidation(src string) error {
	if err := s13Require(src,
		`status := ctx\.URLParam\("status"\)`,
		`startTime := ctx\.URLParam\("start_time"\)`,
		`endTime := ctx\.URLParam\("end_time"\)`,
		`smsParseLogFilters\(status, startTime, endTime\)`,
		`currentSite\.Tr\("SmsParamInvalid"\)`,
		`config\.StatusFailed`,
	); err != nil {
		return err
	}
	iValidate := strings.Index(src, "smsParseLogFilters(")
	iList := strings.Index(src, "GetSmsLogList(")
	if iValidate < 0 || iList < 0 || iValidate >= iList {
		return errors.New("参数校验未在查询之前（非法参数会静默退化成全量）")
	}
	return nil
}

// TestPluginSmsLogsParamValidation：status / start_time / end_time 的解析口径
// 逐值表驱动（每个非法值一条 + 合法值不误伤），并钉住 handler 真的用了它。
func TestPluginSmsLogsParamValidation(t *testing.T) {
	cases := []struct {
		name               string
		status, start, end string
		ok                 bool
	}{
		{"不传", "", "", "", true},
		{"status0", "0", "", "", true},
		{"status1", "1", "", "", true},
		{"status-1", "-1", "", "", true},
		{"status超出枚举", "2", "", "", false},
		{"status只有空格", " ", "", "", false},
		{"status非数字", "abc", "", "", false},
		{"status小数", "1.0", "", "", false},
		{"start合法", "", "1700000000", "", true},
		{"start为0非法", "", "0", "", false},
		{"start负数非法", "", "-1", "", false},
		{"start非数字非法", "", "abc", "", false},
		{"start小数非法", "", "1.5", "", false},
		{"end合法", "", "", "1700000100", true},
		{"end为0非法", "", "", "0", false},
		{"end非数字非法", "", "", "abc", false},
		{"区间合法", "", "1700000000", "1700000100", true},
		{"区间相等合法", "", "1700000000", "1700000000", true},
		{"区间反向非法", "", "1700000100", "1700000000", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := smsParseLogFilters(c.status, c.start, c.end); ok != c.ok {
				t.Errorf("smsParseLogFilters(%q,%q,%q) ok=%v，期望 %v",
					c.status, c.start, c.end, ok, c.ok)
			}
		})
	}

	// 合法值不误伤：解析结果逐字段核对（status=0 必须区分「传了 0」与「没传」）。
	p, ok := smsParseLogFilters("0", "1700000000", "1700000100")
	if !ok {
		t.Fatal("合法参数被判非法")
	}
	if !p.hasStatus || p.status != 0 {
		t.Errorf("status=0 应作为合法筛选值：hasStatus=%v status=%d", p.hasStatus, p.status)
	}
	if p.startTS != 1700000000 || p.endTS != 1700000100 {
		t.Errorf("时间戳解析错误：start=%d end=%d", p.startTS, p.endTS)
	}
	if p, ok := smsParseLogFilters("", "", ""); !ok || p.hasStatus || p.startTS != 0 || p.endTS != 0 {
		t.Errorf("未传筛选时不应产生任何条件：%+v ok=%v", p, ok)
	}

	// handler 真的调用校验器（三段式）。
	real := smsFuncSource(t, s13PluginSmsFile, "PluginSmsLogs")
	if err := s13CheckLogParamValidation(real); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	bad := strings.Replace(real, `"msg":  currentSite.Tr("SmsParamInvalid")`, `"msg":  ""`, 1)
	if bad == real {
		t.Fatal("反向对照锚点失效：源码里没找到 SmsParamInvalid 文案")
	}
	if err := s13CheckLogParamValidation(bad); err == nil {
		t.Fatal("反向对照失败：非法参数不报 SmsParamInvalid 的源码仍判绿")
	}
	if err := s13CheckLogParamValidation(real); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

// TestPluginSmsOpsRequireAdminPermission：go/ast 解析 route/manage.go，
// 断言 /sms party 挂 AdminPermissionRequired（不是 AdminPermission），
// 且两条新路由挂在 sms party 上；反向对照换成 AdminPermission 必红。
func TestPluginSmsOpsRequireAdminPermission(t *testing.T) {
	real := s13LoadRoute(t, s13RouteFile)
	if err := s13CheckSmsParty(real); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	t.Run("反向对照_换成AdminPermission必红", func(t *testing.T) {
		bad := strings.Replace(real,
			`sms := plugin.Party("/sms", middleware.AdminPermissionRequired)`,
			`sms := plugin.Party("/sms", middleware.AdminPermission)`, 1)
		if bad == real {
			t.Fatal("反向对照锚点失效：源码里没找到 /sms party 声明")
		}
		if err := s13CheckSmsParty(bad); err == nil {
			t.Fatal("反向对照失败：换成 AdminPermission 后护栏仍判绿")
		}
		t.Log("反向对照：/sms party 挂 AdminPermission 喂同一检查器 → 判违规（红）")
	})
	t.Run("反向对照_删掉新路由必红", func(t *testing.T) {
		re := regexp.MustCompile(`sms\.Post\("/test",\s*manageController\.PluginSmsTest\)`)
		bad := re.ReplaceAllString(real, "")
		if bad == real {
			t.Fatal("反向对照锚点失效：源码里没找到 /test 路由")
		}
		if err := s13CheckSmsParty(bad); err == nil {
			t.Fatal("反向对照失败：删掉 /test 路由后护栏仍判绿")
		}
		t.Log("反向对照：删掉 /test 路由喂同一检查器 → 判违规（红）")
	})
	if err := s13CheckSmsParty(real); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
	t.Log("恢复：挂 AdminPermissionRequired 且含两条新路由的真实源码 → 判合规（绿）")
}

// ---------------------------------------------------------------------------
// R146①：后台出口改用「翻译字典即白名单」——漏掉的 SmsCodeLocked 必须自动覆盖
// ---------------------------------------------------------------------------

// s13LoadTestI18n 注入真实语言包（本包测试进程的 config.ExecPath 不含 locales，
// 故用相对路径 glob 加载）；行为断言必须看到真实译文，不能靠兜底原样返回。
func s13LoadTestI18n(t *testing.T) {
	t.Helper()
	app := iris.New()
	if err := app.I18n.Load("../../locales/*/*.yml", config.LoadLocales()...); err != nil {
		t.Fatalf("加载 locales 失败：%v", err)
	}
	old := provider.I18n
	provider.SetI18n(app.I18n)
	t.Cleanup(func() { provider.SetI18n(old) })
}

// s13CaptureOpsMsg 经真实 HTTP 调 respondSmsOpsError 并解析 {code,msg}。
func s13CaptureOpsMsg(t *testing.T, site *provider.Website, err error) string {
	t.Helper()
	app := iris.New()
	app.Get("/x", func(ctx iris.Context) { respondSmsOpsError(ctx, site, err) })
	if e := app.Build(); e != nil {
		t.Fatalf("构建测试应用失败：%v", e)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
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

// TestPluginSmsOpsErrorKeepsBusinessKeys 钉住 R146①：
// 手抄白名单漏掉的 SmsCodeLocked 与其它业务键一样原样输出译文；
// 查不到键的内部错误（含 %）仍归约为 SmsSendFailed，并只进服务端日志。
func TestPluginSmsOpsErrorKeepsBusinessKeys(t *testing.T) {
	s13LoadTestI18n(t)
	// TplTr 在 I18n 非空时会读 w.System.Language，必须给 System，否则空指针 panic。
	site := &provider.Website{System: &config.SystemConfig{}}
	translate := site.Tr
	generic := translate(sms.ErrSmsSendFailed.Error())
	if generic == sms.ErrSmsSendFailed.Error() {
		t.Fatalf("locales 缺 SmsSendFailed 译文，断言会失去意义（generic=%q）", generic)
	}

	t.Run("SmsCodeLocked 不被吞", func(t *testing.T) {
		want := translate(sms.ErrSmsCodeLocked.Error())
		if want == sms.ErrSmsCodeLocked.Error() {
			t.Fatalf("locales 缺 SmsCodeLocked 译文，断言会失去意义")
		}
		got := s13CaptureOpsMsg(t, site, sms.ErrSmsCodeLocked)
		if got != want {
			t.Fatalf("出口 msg = %q，期望 %q（R146①：旧手抄表漏了 ErrSmsCodeLocked）", got, want)
		}
		if got == generic {
			t.Fatalf("SmsCodeLocked 被归约成了通用文案 %q", generic)
		}
	})

	t.Run("DB错误归约为SmsSendFailed", func(t *testing.T) {
		got := s13CaptureOpsMsg(t, site, errors.New("Error 1406: Data too long for column 'template_id' at row 1"))
		if got != generic {
			t.Fatalf("出口 msg = %q，期望归约文案 %q", got, generic)
		}
		for _, leak := range []string{"1406", "Data too long", "template_id"} {
			if strings.Contains(got, leak) {
				t.Errorf("响应回显了 DB 内部措辞 %q：%q", leak, got)
			}
		}
	})

	t.Run("含百分号的错误串归约为SmsSendFailed", func(t *testing.T) {
		got := s13CaptureOpsMsg(t, site, errors.New("invalid character '%' looking for beginning of value"))
		if got != generic {
			t.Fatalf("出口 msg = %q，期望归约文案 %q", got, generic)
		}
		for _, leak := range []string{"invalid character", "%!"} {
			if strings.Contains(got, leak) {
				t.Errorf("响应回显了内部措辞 %q：%q", leak, got)
			}
		}
	})

	t.Run("类型分支仍带数字", func(t *testing.T) {
		if got := s13CaptureOpsMsg(t, site, &sms.WaitError{Seconds: 42}); !strings.Contains(got, "42") {
			t.Errorf("WaitError 秒数未进入文案：%q", got)
		}
		if got := s13CaptureOpsMsg(t, site, &sms.AttemptsError{Remaining: 3}); !strings.Contains(got, "3") {
			t.Errorf("AttemptsError 剩余次数未进入文案：%q", got)
		}
	})

	// 源码级反向对照：把统一判据换回 currentSite.Tr（OEM 下 Brandize 会污染
	// 「译文 != 原文」判据）后，同一护栏必红 —— 这正是本缺陷的回退路径。
	errSrc := smsFuncSource(t, s13PluginSmsFile, "respondSmsOpsError")
	if err := s13CheckErrorDispatch(errSrc); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	noUnified := strings.Replace(errSrc, "provider.SmsBusinessMessage(currentSite, err)",
		"currentSite.Tr(err.Error())", 1)
	if noUnified == errSrc {
		t.Fatal("反向对照准备失败：源码里没有 provider.SmsBusinessMessage 调用")
	}
	if err := s13CheckErrorDispatch(noUnified); err == nil {
		t.Fatal("反向对照失败：把统一判据换回 Tr 后的源码仍被判绿")
	}
	if err := s13CheckErrorDispatch(errSrc); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

// ---------------------------------------------------------------------------
// 任务 16B-fix1：配置保存端点的失败分支不得回显解析器原文（R114/R144/R146③）
// ---------------------------------------------------------------------------

// s13CheckSettingFormErrorExit：PluginSmsSettingForm 的 5 个失败分支一律不把
// Go/JSON 解析器原文写进响应 —— 4 个解析分支回 SmsParamInvalid，保存失败回
// SaveFailed（DB 失败不是「参数格式不正确」，回「保存失败」才是对的语义），
// 原始错误只进服务端日志。同时禁止套用 respondSmsOpsError：那是发送链路出口，
// 会把保存失败说成「短信发送失败」，比原文更误导。
func s13CheckSettingFormErrorExit(src string) error {
	if err := s13Require(src,
		`ctx\.ReadJSON\(&req\)`,                // 具名且恰好一次（apicatalog 字段级 params）
		`currentSite\.Tr\("SmsParamInvalid"\)`, // 解析失败语义
		`currentSite\.Tr\("SaveFailed"\)`,      // 落库失败语义
		`log\.Println\(`,                       // 原始错误只进服务端日志
	); err != nil {
		return err
	}
	// 反面①：任何形式的解析器原文回显（含 msg 字段）都判违规。
	if strings.Contains(src, "err.Error()") {
		return errors.New("PluginSmsSettingForm 仍把 err.Error() 写进响应（原始错误只进服务端日志）")
	}
	// 反面②：不得套发送链路出口，否则保存失败会被回成「短信发送失败」。
	if strings.Contains(src, "respondSmsOpsError") {
		return errors.New("PluginSmsSettingForm 套用了 respondSmsOpsError（发送链路出口，会把保存失败误报成发送失败）")
	}
	// 5 个失败分支各自都要留日志：少于 5 处说明有分支静默吞掉了原始错误，
	// 线上出问题时服务端将完全没有线索。
	if n := strings.Count(src, "log.Println("); n < 5 {
		return fmt.Errorf("失败分支的服务端日志只有 %d 处，少于 5 个失败分支", n)
	}
	return nil
}

// TestPluginSmsSettingFormErrorExit 三段式：真实源码绿 / 换回 err.Error() 红 /
// 套用 respondSmsOpsError 红 / 删掉服务端日志红 / 恢复绿。
//
// 本包测试不加载 config.json、拿不到 DB，行为级验证在本包不可行（构造 iris ctx
// 真跑 handler 会一路撞到 provider.CurrentSubSite），故只钉源码结构。
func TestPluginSmsSettingFormErrorExit(t *testing.T) {
	real := smsFuncSource(t, s13PluginSmsFile, "PluginSmsSettingForm")
	if err := s13CheckSettingFormErrorExit(real); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	t.Log("真实源码：4 个解析分支回 SmsParamInvalid、保存失败回 SaveFailed，原始错误只进日志 → 判合规（绿）")

	t.Run("反向对照_换回err.Error必红", func(t *testing.T) {
		bad := strings.Replace(real, `currentSite.Tr("SmsParamInvalid")`, "err.Error()", 1)
		if bad == real {
			t.Fatal("反向对照锚点失效：源码里没找到 currentSite.Tr(\"SmsParamInvalid\")")
		}
		err := s13CheckSettingFormErrorExit(bad)
		if err == nil {
			t.Fatal("反向对照失败：把一处 SmsParamInvalid 换回 err.Error() 后仍判绿")
		}
		t.Logf("反向对照：一处解析分支换回 err.Error() → 判违规（红：%v）", err)
	})

	t.Run("反向对照_套用respondSmsOpsError必红", func(t *testing.T) {
		bad := strings.Replace(real, `currentSite.Tr("SmsParamInvalid")`,
			"respondSmsOpsError(ctx, currentSite, err)", 1)
		if bad == real {
			t.Fatal("反向对照锚点失效：源码里没找到 currentSite.Tr(\"SmsParamInvalid\")")
		}
		err := s13CheckSettingFormErrorExit(bad)
		if err == nil {
			t.Fatal("反向对照失败：套用发送链路出口的源码仍判绿")
		}
		t.Logf("反向对照：一处解析分支换成 respondSmsOpsError → 判违规（红：%v）", err)
	})

	t.Run("反向对照_删掉服务端日志必红", func(t *testing.T) {
		// 只删 4 处、留 1 处：确保锚点仍在，让「少于 5 处」的计数断言真正生效
		// （全部删光会先被锚点拦住，测不到计数规则）。
		bad := strings.Replace(real, `log.Println("后台短信配置保存失败：", err)`, "", 4)
		if bad == real {
			t.Fatal("反向对照锚点失效：源码里没找到 log.Println(\"后台短信配置保存失败：\", err)")
		}
		err := s13CheckSettingFormErrorExit(bad)
		if err == nil {
			t.Fatal("反向对照失败：只剩 1 处服务端日志（5 个失败分支）仍判绿")
		}
		t.Logf("反向对照：5 处服务端日志只留 1 处 → 判违规（红：%v）", err)
	})

	if err := s13CheckSettingFormErrorExit(real); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}
