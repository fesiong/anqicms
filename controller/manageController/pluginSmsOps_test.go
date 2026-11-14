package manageController

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"

	"kandaoni.com/anqicms/model"
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
		`Ip:\s*ctx\.RemoteAddr\(\)`,
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
// 其余错误一律不传参。统一 currentSite.Tr(err.Error(), args...)（方法值 translate）。
func s13CheckErrorDispatch(src string) error {
	if err := s13Require(src,
		`errors\.Is\(err,\s*sms\.ErrSmsSendFailed\)`,
		`errors\.As\(err,\s*&wait\)`,
		`errors\.As\(err,\s*&attempts\)`,
		`err\.Error\(\),\s*wait\.Seconds\)`,
		`err\.Error\(\),\s*attempts\.Remaining\)`,
		`(?:currentSite\.Tr|translate)\(err\.Error\(\)\)`,
		`translate\s*:=\s*currentSite\.Tr`,
		`config\.StatusFailed`,
	); err != nil {
		return err
	}
	// 反面：不得出现「对所有错误统一传固定数字参数」的翻译调用。
	if regexp.MustCompile(`(?:currentSite\.Tr|translate)\(err\.Error\(\),\s*\d+\)`).MatchString(src) {
		return errors.New("存在对所有错误传固定数字参数的翻译调用（应只对 WaitError/AttemptsError 精确传参）")
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
		re := regexp.MustCompile(`Ip:\s*ctx\.RemoteAddr\(\),`)
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
		{"status", `tx = tx.Where("status = ?", status)`},
		{"phone", `tx = tx.Where("phone = ?", phone)`},
		{"start_time", `tx = tx.Where("created_time >= ?", startTime)`},
		{"end_time", `tx = tx.Where("created_time <= ?", endTime)`},
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

// TestPluginSmsLogsNeverReturnsCode：go/ast 断言返回结构不含码字段；
// 端到端 JSON 序列化复核不出现 code_hash（SmsCode.CodeHash json:"-" 由任务 6 钉住）。
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

	// 端到端 JSON 序列化复核。
	logJSON, err := json.Marshal([]*model.SmsLog{{Phone: "19900000001", TemplateId: "LOGIN {{code}}"}})
	if err != nil {
		t.Fatalf("序列化 SmsLog 失败：%v", err)
	}
	if strings.Contains(string(logJSON), "code_hash") {
		t.Errorf("SmsLog JSON 出现 code_hash：%s", logJSON)
	}
	codeJSON, err := json.Marshal(&model.SmsCode{CodeHash: "SUPERSECRETCODEHASH"})
	if err != nil {
		t.Fatalf("序列化 SmsCode 失败：%v", err)
	}
	if strings.Contains(string(codeJSON), "code_hash") || strings.Contains(string(codeJSON), "SUPERSECRETCODEHASH") {
		t.Errorf("SmsCode.CodeHash 未被 json:\"-\" 屏蔽：%s", codeJSON)
	}
	t.Log("恢复绿 + 端到端：GetSmsLogList 返回 SmsLog 无码字段，JSON 序列化不出现 code_hash ✓")
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
