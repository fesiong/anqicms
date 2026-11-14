package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/sms"
)

// 编译期确认生产入口 SendSmsCode 存在（不调用、不触网）。
// 测试一律走测试接缝 sendSmsCodeWithSender 并传假 sender，全程不联网、不发真实短信（R62）。
// 步骤 2（第一轮 RED）里该方法尚未定义，这里让编译失败落到 "undefined: SendSmsCode"。
var _ = (*Website).SendSmsCode

// ---------------------------------------------------------------------------
// 测试基建（简报步骤 1 / 裁决 R60）
// ---------------------------------------------------------------------------

const (
	// 保留测试号段：国际 (+1) 5550100XXX、国内 (86) 199000000XX。
	// 两个号段都纳入 R60 的前置断言与清理范围；发码用例默认用国际号 + Twilio 配置，
	// TestSmsSendRegionUnsupported 用国内号触发 Twilio 区域拒发。
	s8IntlPhonePrefix = "5550100"
	s8CnPhonePrefix   = "199000000"
	s8IntlCountry     = "1"
	s8CnCountry       = "86"
	s8TestIp          = "198.51.100.7" // TEST-NET-2 保留地址，纯测试用
)

var (
	s8PhoneSeq  atomic.Uint64
	s8TestStart atomic.Int64
)

// setupSmsTest：initTestSite(t) 拿真实库站点 1，幂等 AutoMigrate 两张新表（R60 允许，
// 站点启动本就会建），记录用例开始时刻与三表基线行数，注册严格按 R60 的 t.Cleanup。
func setupSmsTest(t *testing.T) *Website {
	t.Helper()
	w := initTestSite(t)
	if w == nil || w.DB == nil {
		t.Fatal("initTestSite 未连上开发库（w.DB 为 nil）")
	}

	// 隔离全局 IP 预算单例（smsIpBudgetFor 的 sync.Map）：每个用例开始时清零所有
	// (siteId, 限额) budget 的计数。否则全量 16 条会跨用例累计，把默认 10/h 的
	// 小时限额吃满，后续用例在第 5 步被 ErrSmsIpLimited 误拒。生产语义不变。
	smsIpBudgets.Range(func(_, v interface{}) bool {
		b := v.(*smsIpBudget)
		b.hour.Reset()
		b.day.Reset()
		return true
	})

	// R60：只做幂等 AutoMigrate，绝不 DROP / TRUNCATE 任何表。
	if err := w.DB.AutoMigrate(&model.SmsCode{}, &model.SmsLog{}); err != nil {
		t.Fatalf("[R60 DDL] AutoMigrate(&model.SmsCode{}, &model.SmsLog{}) 失败：%v", err)
	}
	t.Log("[R60 DDL] AutoMigrate(&model.SmsCode{}, &model.SmsLog{}) 幂等完成（CREATE TABLE IF NOT EXISTS / 幂等 ALTER）")

	start := time.Now().Unix()
	s8TestStart.Store(start)

	// R60 前置断言：保留号段未被真实用户占用；占用则 Skip，绝不删数据。
	var occupied int64
	if err := w.DB.Raw("SELECT COUNT(*) FROM `users` WHERE `phone` LIKE ? OR `phone` LIKE ?",
		s8IntlPhonePrefix+"%", s8CnPhonePrefix+"%").Row().Scan(&occupied); err != nil {
		t.Fatalf("[R60 前置] 查询 users 号段占用失败：%v", err)
	}
	if occupied != 0 {
		t.Skipf("[R60 前置] 保留测试号段已被真实用户占用 %d 行，跳过本用例（不删数据）", occupied)
	}
	t.Logf("[R60 前置] users 中保留号段占用 = %d（期望 0）", occupied)

	// 三表基线行数（本用例开始时刻）；cleanup 用它们做实测核对。
	var baseCodes, baseLogs, baseUsers int64
	for _, c := range []struct {
		table string
		dst   *int64
	}{
		{"sms_codes", &baseCodes}, {"sms_logs", &baseLogs}, {"users", &baseUsers},
	} {
		if err := w.DB.Raw("SELECT COUNT(*) FROM `" + c.table + "`").Row().Scan(c.dst); err != nil {
			t.Fatalf("[R60 基线] SELECT COUNT(*) %s 失败：%v", c.table, err)
		}
	}
	t.Logf("[R60 基线] sms_codes=%d sms_logs=%d users=%d（控制者核实 users 基线 339）", baseCodes, baseLogs, baseUsers)

	t.Cleanup(func() { s8Cleanup(t, w, start, baseCodes, baseLogs, baseUsers) })
	return w
}

// s8CleanupWhere 是 R60 逐字口径：只删「本用例号段 且 created_time >= 用例开始时刻」的行。
// 返回完整 5 元素 args（含 start），避免调用处手工重建导致单边失配（R70）。
func s8CleanupWhere(start int64) (string, []interface{}) {
	return "((`phone_country` = ? AND `phone` LIKE ?) OR (`phone_country` = ? AND `phone` LIKE ?)) AND `created_time` >= ?",
		[]interface{}{s8IntlCountry, s8IntlPhonePrefix + "%", s8CnCountry, s8CnPhonePrefix + "%", start}
}

// s8Cleanup：三处 DELETE 全部接 .Error，失败 t.Errorf；删除/残留核对带断言而非仅 t.Logf。
func s8Cleanup(t *testing.T, w *Website, start int64, baseCodes, baseLogs, baseUsers int64) {
	t.Helper()
	where, args := s8CleanupWhere(start)

	// 每处 DELETE 必须接 .Error 并在失败时 t.Errorf（R58 minor 的教训）。
	for _, table := range []string{"sms_codes", "sms_logs", "users"} {
		res := w.DB.Exec("DELETE FROM `"+table+"` WHERE "+where, args...)
		if res.Error != nil {
			t.Errorf("[R60 清理] DELETE %s 失败：%v", table, res.Error)
		} else {
			t.Logf("[R60 清理] DELETE %s 影响 %d 行", table, res.RowsAffected)
		}
	}

	// 残留核对：号段 + created_time >= start 必须为 0 行（断言）。
	for _, table := range []string{"sms_codes", "sms_logs", "users"} {
		var remain int64
		if err := w.DB.Raw("SELECT COUNT(*) FROM `"+table+"` WHERE "+where, args...).Row().Scan(&remain); err != nil {
			t.Errorf("[R60 残留] SELECT %s 失败：%v", table, err)
			continue
		}
		if remain != 0 {
			t.Errorf("[R60 残留] %s 号段 + created_time>=start 仍有 %d 行未清理", table, remain)
		} else {
			t.Logf("[R60 残留] %s 号段残留 = 0 行", table)
		}
	}

	// 全表行数实测核对：users 必须与用例开始基线一致（339）；三表总数实测记录。
	checks := []struct {
		table string
		base  int64
	}{
		{"sms_codes", baseCodes}, {"sms_logs", baseLogs}, {"users", baseUsers},
	}
	for _, c := range checks {
		var total int64
		if err := w.DB.Raw("SELECT COUNT(*) FROM `" + c.table + "`").Row().Scan(&total); err != nil {
			t.Errorf("[R60 实测] SELECT COUNT(*) %s 失败：%v", c.table, err)
			continue
		}
		t.Logf("[R60 实测] %s 总行数 = %d（用例开始基线 = %d）", c.table, total, c.base)
		if c.table == "users" && total != c.base {
			t.Errorf("[R60 实测] users 总行数 = %d，与基线 %d 不一致（误删或残留）", total, c.base)
		}
	}
}

// smsTestPhone 返回一个保留号段内的国际测试号；号段级前置占用断言在 setupSmsTest，
// 这里用原子序号保证同一批次内每次调用得到不同号（换号码绕开号码限额/重发间隔）。
func smsTestPhone(t *testing.T, w *Website) (string, string) {
	t.Helper()
	seq := s8PhoneSeq.Add(1)
	return s8IntlCountry, fmt.Sprintf("%s%03d", s8IntlPhonePrefix, seq%1000)
}

// s8UseConfig 替换站点内存配置并在用例结束恢复，避免污染其它依赖全局单例的测试。
func s8UseConfig(t *testing.T, w *Website, cfg *config.PluginSmsConfig) {
	t.Helper()
	old := w.PluginSms
	w.PluginSms = cfg
	t.Cleanup(func() { w.PluginSms = old })
}

// s8TwilioConfig 构造一份能真正过 sms.Resolve 的假凭据配置（不联网、不发真实短信）。
func s8TwilioConfig(scenes ...string) *config.PluginSmsConfig {
	texts := map[string]string{}
	for _, s := range []string{"login", "register", "reset", "bind_old", "bind_new", "test"} {
		texts[s] = "【测试】您的验证码是 {{code}}，请勿转发。"
	}
	return &config.PluginSmsConfig{
		Provider: config.SmsProviderTwilio,
		Scenes:   scenes,
		Twilio: config.SmsTwilioConfig{
			International: config.SmsTwilioRegion{
				AccountSid: "AC_fake000000000000000000000000",
				AuthToken:  "fake-auth-token",
				From:       "+15550000000",
				Texts:      texts,
			},
		},
	}
}

// s8DayStart 与生产代码同口径：当天零点（本地时区）Unix 秒。
func s8DayStart() int64 {
	n := time.Now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, n.Location()).Unix()
}

// s8ResetIpBudget 重置给定限额对应的 IP 预算，隔离测试间的全局单例计数。
func s8ResetIpBudget(w *Website, hourly, daily int) {
	b := smsIpBudgetFor(w.Id, hourly, daily)
	b.hour.Reset()
	b.day.Reset()
}

// fakeSmsSender 是测试接缝用的假 sender：全程不触网、不发真实短信（R62）。
type fakeSmsSender struct {
	res        sms.SendResult
	err        error
	calls      int
	beforeSend func()
	lastCode   string
	lastReq    sms.SendRequest
}

func (f *fakeSmsSender) Name() string { return "fake" }

func (f *fakeSmsSender) Send(_ context.Context, _ sms.Profile, req sms.SendRequest) (sms.SendResult, error) {
	f.calls++
	f.lastCode = req.Code
	f.lastReq = req
	if f.beforeSend != nil {
		f.beforeSend()
	}
	return f.res, f.err
}

// ---------------------------------------------------------------------------
// IP 限流器白盒工具（仅测试内；生产代码零改动）
// ---------------------------------------------------------------------------

// s8InjectLimiterClock 用可控时钟替换 library.RateLimiter 私有的 now 字段，
// 让「窗口到期时刻不因被拒调用而后移」可在毫秒级测试内精确验证。
// 仅测试文件内使用 unsafe，恢复闭包会把原 now 写回。
func s8InjectLimiterClock(l *library.RateLimiter, clock *time.Time) func() {
	rv := reflect.ValueOf(l).Elem().FieldByName("now")
	p := unsafe.Pointer(rv.UnsafeAddr())
	old := *(*func() time.Time)(p)
	*(*func() time.Time)(p) = func() time.Time { return *clock }
	return func() { *(*func() time.Time)(p) = old }
}

// s8LimiterHits 只读 library.RateLimiter 私有的 hits map（白盒证据用），返回 ip 的拷贝。
// 测试为单线程、且只在 Allow 返回之后读取，不与生产写并发。
func s8LimiterHits(l *library.RateLimiter, ip string) []time.Time {
	rv := reflect.ValueOf(l).Elem().FieldByName("hits")
	m := *(*map[string][]time.Time)(unsafe.Pointer(rv.UnsafeAddr()))
	return append([]time.Time(nil), m[ip]...)
}

func s8TimesEqual(a, b []time.Time) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// go/ast 通用工具（每条护栏都带「合成坏源码红 + 恢复绿」反向对照）
// ---------------------------------------------------------------------------

func s8LoadSource(t *testing.T, path string) (string, *ast.File) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", path, err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatalf("解析 %s 失败：%v", path, err)
	}
	return string(src), file
}

func s8ParseSynthetic(t *testing.T, src string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synthetic_sms8.go", src, 0)
	if err != nil {
		t.Fatalf("合成源码解析失败：%v", err)
	}
	return file
}

// s8CheckDailyCountNoScene：日限计数 Where（含 created_time >=）不得带 scene 条件。
// 找不到该 Where 时报「扫描器失效」，避免检查器空转恒绿。
func s8CheckDailyCountNoScene(file *ast.File) error {
	var dailyWhere string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Where" || len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		val, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if strings.Contains(val, "created_time >=") {
			dailyWhere = val
		}
		return true
	})
	if dailyWhere == "" {
		return fmt.Errorf("未找到含 created_time >= 的日限计数 Where（扫描器失效，护栏不得空转）")
	}
	if strings.Contains(dailyWhere, "scene") {
		return fmt.Errorf("日限计数 Where 带了 scene 条件，跨场景合计被破坏：%q", dailyWhere)
	}
	return nil
}

// s8CheckNoMiddleware：smsSend.go / smsService.go 不得 import middleware、不得出现
// accountProbeLimiter（provider → middleware 本就会成环；共用预算会污染邮箱枚举探测）。
func s8CheckNoMiddleware(file *ast.File) error {
	for _, imp := range file.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if strings.Contains(p, "middleware") {
			return fmt.Errorf("import 了 middleware 包：%s", p)
		}
	}
	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "accountProbeLimiter" {
			found = true
		}
		return true
	})
	if found {
		return fmt.Errorf("出现 accountProbeLimiter：短信 IP 预算不得与邮箱枚举探测共用")
	}
	return nil
}

// s8CheckNoCodeCache：sms*.go 生产文件不得出现 library.CodeCache 选择器（R64）。
// 返回「已检查的文件数」，供自检确认没有因文件不存在而恒绿。
func s8CheckNoCodeCache(files []*ast.File) (int, error) {
	for i, f := range files {
		var bad bool
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if ok && x.Name == "library" && sel.Sel.Name == "CodeCache" {
				bad = true
			}
			return true
		})
		if bad {
			return i + 1, fmt.Errorf("第 %d 个 sms*.go 生产文件出现 library.CodeCache（CodeCache 有七项已知缺陷，禁止复用）", i+1)
		}
	}
	return len(files), nil
}

// ---------------------------------------------------------------------------
// 第一轮测试：限额与状态机（简报步骤 1）
// ---------------------------------------------------------------------------

func TestSmsSendDisabled(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, &config.PluginSmsConfig{}) // Provider = ""
	country, local := smsTestPhone(t, w)
	fake := &fakeSmsSender{}

	_, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
	}, fake)
	if !errors.Is(err, sms.ErrSmsServiceNotEnabled) {
		t.Fatalf("Provider 为空应返回 ErrSmsServiceNotEnabled，得到 %v", err)
	}
	if fake.calls != 0 {
		t.Errorf("服务未启用时不应调用 sender.Send，calls=%d", fake.calls)
	}
	// sms_logs 无新行（本用例号段 + created_time >= 用例开始时刻）。
	var n int64
	if e := w.DB.Raw("SELECT COUNT(*) FROM `sms_logs` WHERE `phone` = ? AND `created_time` >= ?",
		local, s8TestStart.Load()).Row().Scan(&n); e != nil {
		t.Fatalf("查询 sms_logs 失败：%v", e)
	}
	if n != 0 {
		t.Errorf("服务未启用时不应写 sms_logs，实际 %d 行", n)
	}
}

func TestSmsSendSceneWhitelist(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login")) // Scenes = ["login"]
	country, local := smsTestPhone(t, w)

	// register 不在白名单 → ErrSmsSceneNotEnabled
	_, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "register", Ip: s8TestIp,
	}, &fakeSmsSender{})
	if !errors.Is(err, sms.ErrSmsSceneNotEnabled) {
		t.Errorf("register 不在白名单应落 ErrSmsSceneNotEnabled，得到 %v", err)
	}
	// notify.order 不在白名单 → ErrSmsSceneNotEnabled
	_, err = w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "notify.order", Ip: s8TestIp,
	}, &fakeSmsSender{})
	if !errors.Is(err, sms.ErrSmsSceneNotEnabled) {
		t.Errorf("notify.order 不在白名单应落 ErrSmsSceneNotEnabled，得到 %v", err)
	}
	// 后台 test 场景不受白名单限制 → 允许（不落 ErrSmsSceneNotEnabled）
	_, err = w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: sms.SceneTest, Ip: s8TestIp,
	}, &fakeSmsSender{})
	if errors.Is(err, sms.ErrSmsSceneNotEnabled) {
		t.Errorf("后台 test 场景不应被白名单拒绝，却得到 ErrSmsSceneNotEnabled")
	}
	if err != nil {
		t.Errorf("后台 test 场景应允许发码，得到 error：%v", err)
	}
}

func TestSmsSendDailyLimitAcrossScenes(t *testing.T) {
	w := setupSmsTest(t)
	cfg := s8TwilioConfig("login", "register", "reset")
	cfg.DailyLimitPerPhone = 3
	s8UseConfig(t, w, cfg)
	country, local := smsTestPhone(t, w)

	// 跨场景合计：login / register / reset 各一次全部成功。
	for _, scene := range []string{"login", "register", "reset"} {
		_, err := w.sendSmsCodeWithSender(SmsSendInput{
			PhoneCountry: country, Phone: local, Scene: scene, Ip: s8TestIp,
		}, &fakeSmsSender{})
		if err != nil {
			t.Fatalf("scene=%s 第 N 次发码应成功（日限 3 未到），却报 %v", scene, err)
		}
	}
	// 第四次（任意场景）→ ErrSmsPhoneLimited。
	_, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
	}, &fakeSmsSender{})
	if !errors.Is(err, sms.ErrSmsPhoneLimited) {
		t.Fatalf("第四次（跨场景累计达 3）应落 ErrSmsPhoneLimited，得到 %v", err)
	}
	// sms_logs 该号当日行数 == 3（不带 scene）。
	var n int64
	if e := w.DB.Raw("SELECT COUNT(*) FROM `sms_logs` WHERE `site_id` = ? AND `phone_country` = ? AND `phone` = ? AND `created_time` >= ?",
		w.Id, country, local, s8DayStart()).Row().Scan(&n); e != nil {
		t.Fatalf("查询当日 sms_logs 失败：%v", e)
	}
	if n != 3 {
		t.Errorf("该号当日 sms_logs 行数 = %d，期望 3（跨场景合计）", n)
	}

	// go/ast 护栏：计数查询不得带 scene；反向对照「加 scene 必须变红」+「恢复绿」。
	src, real := s8LoadSource(t, "smsSend.go")
	if err := s8CheckDailyCountNoScene(real); err != nil {
		t.Errorf("日限计数查询违规或扫描器失效：%v", err)
	}
	t.Logf("[检查器] s8CheckDailyCountNoScene；变异锚点 = 日限计数 Where 里的 `created_time >= ?` → 追加 `AND scene = ?`")
	t.Run("反向对照_计数加scene必须变红", func(t *testing.T) {
		bad := strings.Replace(src, "created_time >= ?", "created_time >= ? AND scene = ?", 1)
		if bad == src {
			t.Fatal("反向对照锚点失效：源码里没找到 `created_time >= ?`")
		}
		if err := s8CheckDailyCountNoScene(s8ParseSynthetic(t, bad)); err == nil {
			t.Fatal("反向对照失效：给计数查询加 scene 条件后检查器竟判合规")
		}
		t.Log("反向对照：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_未变异源码必须变绿", func(t *testing.T) {
		if err := s8CheckDailyCountNoScene(s8ParseSynthetic(t, src)); err != nil {
			t.Fatalf("恢复后同一检查器仍报错：%v", err)
		}
		t.Log("恢复：未变异源码喂同一检查器 → 判合规（绿）")
	})
}

func TestSmsSendResendIntervalPerScene(t *testing.T) {
	w := setupSmsTest(t)
	cfg := s8TwilioConfig("login", "register")
	cfg.ResendInterval = 60
	s8UseConfig(t, w, cfg)
	country, local := smsTestPhone(t, w)

	// login 连发两次：第二次 ErrSmsResendTooSoon + *WaitError(Seconds 1..60)。
	_, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
	}, &fakeSmsSender{})
	if err != nil {
		t.Fatalf("login 第一次应成功：%v", err)
	}
	_, err = w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
	}, &fakeSmsSender{})
	if !errors.Is(err, sms.ErrSmsResendTooSoon) {
		t.Fatalf("login 第二次应落 ErrSmsResendTooSoon，得到 %v", err)
	}
	var we *sms.WaitError
	if !errors.As(err, &we) {
		t.Fatalf("应能 errors.As 取到 *sms.WaitError，得到 %T", err)
	}
	if we.Seconds < 1 || we.Seconds > 60 {
		t.Errorf("WaitError.Seconds = %d，期望 1..60", we.Seconds)
	}
	// 紧接着发 register → 成功（重发间隔按场景隔离）。
	_, err = w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "register", Ip: s8TestIp,
	}, &fakeSmsSender{})
	if err != nil {
		t.Errorf("换场景 register 不应受 login 冷却影响，却报 %v", err)
	}
}

func TestSmsSendRegionUnsupported(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login")) // Twilio 只有国际 profile
	fake := &fakeSmsSender{}

	// 国内号 (86) + Twilio → ErrSmsRegionUnsupported，且假 sender 不被调用。
	_, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: s8CnCountry, Phone: "19900000001", Scene: "login", Ip: s8TestIp,
	}, fake)
	if !errors.Is(err, sms.ErrSmsRegionUnsupported) {
		t.Fatalf("Twilio + 国内号应落 ErrSmsRegionUnsupported，得到 %v", err)
	}
	if fake.calls != 0 {
		t.Errorf("区域不支持时不应调用 sender.Send，calls=%d", fake.calls)
	}
}

func TestSmsSendPrunesStaleCodes(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	country, local := smsTestPhone(t, w)
	now := time.Now().Unix()

	// 手工插同键的「已过期」行与「已消费」行。
	expired := &model.SmsCode{
		SiteId: w.Id, PhoneCountry: country, Phone: local, Scene: "login",
		CodeHash: "stale-expired-hash", ExpiresAt: now - 10, CreatedTime: now,
	}
	consumed := &model.SmsCode{
		SiteId: w.Id, PhoneCountry: country, Phone: local, Scene: "login",
		CodeHash: "stale-consumed-hash", ExpiresAt: now + 300, ConsumedAt: now - 5, CreatedTime: now,
	}
	for _, row := range []*model.SmsCode{expired, consumed} {
		if err := w.DB.Create(row).Error; err != nil {
			t.Fatalf("插入同键过期/已消费行失败：%v", err)
		}
	}

	_, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
	}, &fakeSmsSender{})
	if err != nil {
		t.Fatalf("发码应成功：%v", err)
	}

	// 两行被删。
	var stale int64
	if e := w.DB.Raw("SELECT COUNT(*) FROM `sms_codes` WHERE `id` IN (?,?)", expired.Id, consumed.Id).
		Row().Scan(&stale); e != nil {
		t.Fatalf("查询 stale 行失败：%v", e)
	}
	if stale != 0 {
		t.Errorf("过期/已消费行应被事务第 7c 步删除，实际仍有 %d 行", stale)
	}
	// 新行存在且只有 1 行。
	var fresh int64
	if e := w.DB.Raw("SELECT COUNT(*) FROM `sms_codes` WHERE `site_id` = ? AND `phone_country` = ? AND `phone` = ? AND `scene` = ? AND `id` NOT IN (?,?)",
		w.Id, country, local, "login", expired.Id, consumed.Id).Row().Scan(&fresh); e != nil {
		t.Fatalf("查询新行失败：%v", e)
	}
	if fresh != 1 {
		t.Errorf("发码后应仅存在 1 行新码，实际 %d 行", fresh)
	}
}

func TestSmsIpBudgetIndependent(t *testing.T) {
	w := setupSmsTest(t)

	// 小时限额：同一 IP 连发三次（换号码绕开号码限额）→ 第三次 ErrSmsIpLimited。
	t.Run("小时限额", func(t *testing.T) {
		cfg := s8TwilioConfig("login")
		cfg.HourlyLimitPerIp = 2
		cfg.DailyLimitPerIp = 1000
		s8UseConfig(t, w, cfg)
		s8ResetIpBudget(w, 2, 1000)
		defer s8ResetIpBudget(w, 2, 1000)

		for i := 0; i < 3; i++ {
			country, local := smsTestPhone(t, w)
			_, err := w.sendSmsCodeWithSender(SmsSendInput{
				PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
			}, &fakeSmsSender{})
			if i < 2 && err != nil {
				t.Fatalf("小时限额内第 %d 次应成功：%v", i+1, err)
			}
			if i == 2 && !errors.Is(err, sms.ErrSmsIpLimited) {
				t.Fatalf("小时限额第 3 次应落 ErrSmsIpLimited，得到 %v", err)
			}
		}
	})

	// 日限额同理（不同限额 → 不同 budget key，天然隔离）。
	t.Run("日限额", func(t *testing.T) {
		cfg := s8TwilioConfig("login")
		cfg.HourlyLimitPerIp = 1000
		cfg.DailyLimitPerIp = 2
		s8UseConfig(t, w, cfg)
		s8ResetIpBudget(w, 1000, 2)
		defer s8ResetIpBudget(w, 1000, 2)

		for i := 0; i < 3; i++ {
			country, local := smsTestPhone(t, w)
			_, err := w.sendSmsCodeWithSender(SmsSendInput{
				PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
			}, &fakeSmsSender{})
			if i < 2 && err != nil {
				t.Fatalf("日限额内第 %d 次应成功：%v", i+1, err)
			}
			if i == 2 && !errors.Is(err, sms.ErrSmsIpLimited) {
				t.Fatalf("日限额第 3 次应落 ErrSmsIpLimited，得到 %v", err)
			}
		}
	})

	// 源码级独立性断言：smsSend.go / smsService.go 不得 import middleware、不得出现 accountProbeLimiter。
	for _, path := range []string{"smsSend.go", "smsService.go"} {
		_, file := s8LoadSource(t, path)
		if err := s8CheckNoMiddleware(file); err != nil {
			t.Errorf("%s 违反 IP 预算独立性：%v", path, err)
		}
	}
	t.Logf("[检查器] s8CheckNoMiddleware；变异锚点 = 合成源码 import middleware / 出现 accountProbeLimiter")
	t.Run("反向对照_合成共用预算源码必须变红", func(t *testing.T) {
		bad := `package provider
import "kandaoni.com/anqicms/middleware"
func s8SyntheticBad() { _ = middleware.X; _ = accountProbeLimiter }
`
		if err := s8CheckNoMiddleware(s8ParseSynthetic(t, bad)); err == nil {
			t.Fatal("反向对照失效：import middleware + accountProbeLimiter 的合成源码竟判合规")
		}
		t.Log("反向对照：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_未变异源码必须变绿", func(t *testing.T) {
		for _, path := range []string{"smsSend.go", "smsService.go"} {
			_, file := s8LoadSource(t, path)
			if err := s8CheckNoMiddleware(file); err != nil {
				t.Fatalf("恢复后 %s 仍报错：%v", path, err)
			}
		}
		t.Log("恢复：未变异源码喂同一检查器 → 判合规（绿）")
	})
}

func TestSmsIpLimitRejectedDoesNotExtendWindow(t *testing.T) {
	w := setupSmsTest(t)
	cfg := s8TwilioConfig("login")
	cfg.HourlyLimitPerIp = 2
	cfg.DailyLimitPerIp = 1000
	s8UseConfig(t, w, cfg)

	budget := smsIpBudgetFor(w.Id, 2, 1000)
	budget.hour.Reset()
	budget.day.Reset()

	// 可控时钟（仅测试内 unsafe 注入 library.RateLimiter.now），退出恢复原时钟。
	var clock time.Time
	restore := s8InjectLimiterClock(budget.hour, &clock)
	defer func() {
		restore()
		budget.hour.Reset()
		budget.day.Reset()
	}()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock = base

	sendOnce := func() error {
		country, local := smsTestPhone(t, w)
		_, err := w.sendSmsCodeWithSender(SmsSendInput{
			PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
		}, &fakeSmsSender{})
		return err
	}

	// 两次成功（不同号），窗口内记 2 个时间戳。
	for i := 0; i < 2; i++ {
		if err := sendOnce(); err != nil {
			t.Fatalf("第 %d 次应成功：%v", i+1, err)
		}
	}
	before := s8LimiterHits(budget.hour, s8TestIp)
	if len(before) != 2 {
		t.Fatalf("白盒自检：两次成功后 hits[%s] 应有 2 个时间戳，实际 %d（检查器不得空转）", s8TestIp, len(before))
	}

	// 超限后再调一次仍被拒。
	clock = base.Add(10 * time.Minute)
	if err := sendOnce(); !errors.Is(err, sms.ErrSmsIpLimited) {
		t.Fatalf("超限后第 3 次应落 ErrSmsIpLimited，得到 %v", err)
	}
	// 再调一次（仍窗口内）仍被拒。
	clock = base.Add(20 * time.Minute)
	if err := sendOnce(); !errors.Is(err, sms.ErrSmsIpLimited) {
		t.Fatalf("超限后第 4 次仍应落 ErrSmsIpLimited，得到 %v", err)
	}

	// 白盒硬证据：被拒调用前后 hits 时间戳集合完全不变 → 窗口到期时刻不后移。
	after := s8LimiterHits(budget.hour, s8TestIp)
	if !s8TimesEqual(before, after) {
		t.Errorf("被拒调用改变了 hits（窗口被延长）：before=%v after=%v", before, after)
	} else {
		t.Logf("白盒：被拒调用前后 hits 均为 %v（窗口到期时刻不后移）", after)
	}

	// 行为硬证据：推进到首次成功 + window 之后 → 应放行（证明被拒没顺延窗口）。
	clock = base.Add(61 * time.Minute)
	if err := sendOnce(); err != nil {
		t.Errorf("窗口滑过后应重新放行（被拒调用不该顺延窗口），却报 %v", err)
	}
}

// TestSmsIpBudgetNoCrossDimensionAccounting（R69）：小时 / 日两个窗口不得互相记账。
// 旧实现第 5 步无条件各调一次 Allow，被一个窗口拒绝的请求仍会消耗另一个窗口名额
// （过度拒绝）；新实现先双 Check、都通过才双 Allow，任一拒绝则两个窗口都不记账。
// 两个方向限额都取小值（hour=2/day=3 与 hour=3/day=2），不许用 1000 回避交叉。
func TestSmsIpBudgetNoCrossDimensionAccounting(t *testing.T) {
	w := setupSmsTest(t)

	// 方向一：hour 先满（hour=2 < day=3）→ 被 hour 拒后 day 命中数不得增长。
	t.Run("被小时拒后日窗口不记账", func(t *testing.T) {
		cfg := s8TwilioConfig("login")
		cfg.HourlyLimitPerIp = 2
		cfg.DailyLimitPerIp = 3
		s8UseConfig(t, w, cfg)
		s8ResetIpBudget(w, 2, 3)
		defer s8ResetIpBudget(w, 2, 3)
		budget := smsIpBudgetFor(w.Id, 2, 3)

		sendOnce := func() error {
			country, local := smsTestPhone(t, w)
			_, err := w.sendSmsCodeWithSender(SmsSendInput{
				PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
			}, &fakeSmsSender{})
			return err
		}

		// 前 2 次成功（hour=2 放行，day=3 也放行）。
		for i := 0; i < 2; i++ {
			if err := sendOnce(); err != nil {
				t.Fatalf("小时限额内第 %d 次应成功：%v", i+1, err)
			}
		}
		dayBefore := s8LimiterHits(budget.day, s8TestIp)
		if len(dayBefore) != 2 {
			t.Fatalf("白盒自检：2 次成功后 day hits 应为 2，实际 %d（检查器不得空转）", len(dayBefore))
		}

		// 第 3 次被 hour 拒（hour 已满，day 仍有 1 个名额）。
		if err := sendOnce(); !errors.Is(err, sms.ErrSmsIpLimited) {
			t.Fatalf("hour 满后第 3 次应落 ErrSmsIpLimited，得到 %v", err)
		}
		// 关键断言：被 hour 拒的请求不得消耗 day 名额。
		dayAfter := s8LimiterHits(budget.day, s8TestIp)
		if !s8TimesEqual(dayBefore, dayAfter) {
			t.Errorf("被 hour 拒后 day 命中数增长（跨维度记账）：before=%d after=%d", len(dayBefore), len(dayAfter))
		} else {
			t.Logf("[R69 GREEN 方向一] 被 hour 拒后 day hits 保持 %d 个不变（旧无条件双 Allow 会变成 3）", len(dayAfter))
		}
	})

	// 方向二：day 先满（hour=3 > day=2）→ 被 day 拒后 hour 命中数不得增长。
	t.Run("被日拒后小时窗口不记账", func(t *testing.T) {
		cfg := s8TwilioConfig("login")
		cfg.HourlyLimitPerIp = 3
		cfg.DailyLimitPerIp = 2
		s8UseConfig(t, w, cfg)
		s8ResetIpBudget(w, 3, 2)
		defer s8ResetIpBudget(w, 3, 2)
		budget := smsIpBudgetFor(w.Id, 3, 2)

		sendOnce := func() error {
			country, local := smsTestPhone(t, w)
			_, err := w.sendSmsCodeWithSender(SmsSendInput{
				PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
			}, &fakeSmsSender{})
			return err
		}

		// 前 2 次成功（day=2 放行，hour=3 也放行）。
		for i := 0; i < 2; i++ {
			if err := sendOnce(); err != nil {
				t.Fatalf("日限额内第 %d 次应成功：%v", i+1, err)
			}
		}
		hourBefore := s8LimiterHits(budget.hour, s8TestIp)
		if len(hourBefore) != 2 {
			t.Fatalf("白盒自检：2 次成功后 hour hits 应为 2，实际 %d（检查器不得空转）", len(hourBefore))
		}

		// 第 3 次被 day 拒（day 已满，hour 仍有 1 个名额）。
		if err := sendOnce(); !errors.Is(err, sms.ErrSmsIpLimited) {
			t.Fatalf("day 满后第 3 次应落 ErrSmsIpLimited，得到 %v", err)
		}
		// 关键断言：被 day 拒的请求不得消耗 hour 名额。
		hourAfter := s8LimiterHits(budget.hour, s8TestIp)
		if !s8TimesEqual(hourBefore, hourAfter) {
			t.Errorf("被 day 拒后 hour 命中数增长（跨维度记账）：before=%d after=%d", len(hourBefore), len(hourAfter))
		} else {
			t.Logf("[R69 GREEN 方向二] 被 day 拒后 hour hits 保持 %d 个不变（旧无条件双 Allow 会变成 3）", len(hourAfter))
		}
	})
}

func TestSmsDoesNotReuseCodeCache(t *testing.T) {
	// R64：目录级 glob provider/sms*.go（排除 _test.go），使任务 9 创建 smsVerify.go 后自动纳入。
	paths, err := filepath.Glob("sms*.go")
	if err != nil {
		t.Fatalf("glob sms*.go 失败：%v", err)
	}
	var prodFiles []*ast.File
	var prodNames []string
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue // 护栏只查生产源码；测试文件本身含合成坏源码字符串，必须排除
		}
		_, file := s8LoadSource(t, p)
		prodFiles = append(prodFiles, file)
		prodNames = append(prodNames, p)
	}

	// 自检：至少检查了 N 个已存在的生产文件，不得因文件不存在而恒绿。
	const minChecked = 2 // 本任务创建 smsService.go + smsSend.go；任务 9 后自动 +1（smsVerify.go）
	if len(prodFiles) < minChecked {
		t.Errorf("[R64 自检] 只检查了 %d 个 sms*.go 生产文件（%v），期望 >= %d：不得因文件不存在而恒绿",
			len(prodFiles), prodNames, minChecked)
	}
	checked, err := s8CheckNoCodeCache(prodFiles)
	if err != nil {
		t.Errorf("短信路径复用了 library.CodeCache：%v", err)
	}
	if checked != len(prodFiles) {
		t.Errorf("检查器报告检查了 %d 个文件，实际传入 %d 个", checked, len(prodFiles))
	}
	t.Logf("[R64] 已检查 %d 个生产文件：%v；smsVerify.go 属任务 9、当前不存在故跳过该路径，目录级 glob 会在其创建后自动纳入",
		checked, prodNames)

	t.Run("反向对照_合成含CodeCache源码必须变红", func(t *testing.T) {
		bad := `package provider
import "kandaoni.com/anqicms/library"
func s8SyntheticBad() { _ = library.CodeCache.Generate("x") }
`
		if _, err := s8CheckNoCodeCache([]*ast.File{s8ParseSynthetic(t, bad)}); err == nil {
			t.Fatal("反向对照失效：含 library.CodeCache 的合成源码竟判合规")
		}
		t.Log("反向对照：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_真实源码必须变绿", func(t *testing.T) {
		if _, err := s8CheckNoCodeCache(prodFiles); err != nil {
			t.Fatalf("恢复后同一检查器仍报错：%v", err)
		}
		t.Log("恢复：真实源码喂同一检查器 → 判合规（绿）")
	})
}

// ---------------------------------------------------------------------------
// 第二轮 go/ast 检查器（每条护栏都带「合成坏源码红 + 恢复绿」反向对照）
// ---------------------------------------------------------------------------

func s8FindStruct(file *ast.File, name string) *ast.StructType {
	for _, d := range file.Decls {
		gen, ok := d.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name {
				continue
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				return st
			}
		}
	}
	return nil
}

// s8CheckSendOutsideTransaction：sendSmsCodeWithSender 内必须存在 sender.Send 调用，
// 且它不得落在任何 w.DB.Transaction(...) 回调的 FuncLit 区间内（R62）。
// 找不到 Send 时报错（护栏不得空转；第一轮未接线时正是靠这条变红）。
func s8CheckSendOutsideTransaction(file *ast.File) error {
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "sendSmsCodeWithSender" {
			fn = fd
			break
		}
	}
	if fn == nil || fn.Body == nil {
		return fmt.Errorf("找不到 sendSmsCodeWithSender 函数体（扫描器失效）")
	}
	var txLits []*ast.FuncLit
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Transaction" {
			return true
		}
		for _, a := range call.Args {
			if fl, ok := a.(*ast.FuncLit); ok {
				txLits = append(txLits, fl)
			}
		}
		return true
	})
	sendCount := 0
	var insideErr error
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Send" {
			return true
		}
		sendCount++
		for _, fl := range txLits {
			if fl.Body == nil {
				continue
			}
			if call.Pos() >= fl.Body.Lbrace && call.End() <= fl.Body.Rbrace {
				insideErr = fmt.Errorf("sender.Send 落在 w.DB.Transaction 回调内（持锁事务做网络 IO）")
			}
		}
		return true
	})
	if sendCount == 0 {
		return fmt.Errorf("sendSmsCodeWithSender 内未找到任何 .Send 调用（第 8 步尚未接线；护栏不得空转）")
	}
	return insideErr
}

// s8CheckResultHasNoCode：SmsSendResult 字段集合必须恰好 {ExpiresIn, ResendAfter}。
func s8CheckResultHasNoCode(file *ast.File) error {
	st := s8FindStruct(file, "SmsSendResult")
	if st == nil {
		return fmt.Errorf("找不到 SmsSendResult 结构体（扫描器失效）")
	}
	var fields []string
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			fields = append(fields, n.Name)
		}
	}
	if len(fields) != 2 {
		return fmt.Errorf("SmsSendResult 有 %d 个字段 %v，期望恰好 2 个（绝不返回验证码）", len(fields), fields)
	}
	want := map[string]bool{"ExpiresIn": true, "ResendAfter": true}
	for _, f := range fields {
		if !want[f] {
			return fmt.Errorf("SmsSendResult 出现非预期字段 %q（绝不返回验证码）", f)
		}
		delete(want, f)
	}
	if len(want) != 0 {
		return fmt.Errorf("SmsSendResult 缺字段 %v", want)
	}
	return nil
}

// s8CheckPruneHasLimit1000：惰性清理语句必须带 Limit(1000)（单次限量，避免长事务）。
func s8CheckPruneHasLimit1000(file *ast.File) error {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Limit" || len(call.Args) != 1 {
			return true
		}
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.INT && lit.Value == "1000" {
			found = true
		}
		return true
	})
	if !found {
		return fmt.Errorf("未找到惰性清理的 Limit(1000)（第一轮未接线时这里会 FAIL；护栏不得空转）")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 第二轮测试：平台调用、可观测、清理（简报步骤 5）
// ---------------------------------------------------------------------------

func TestSmsSendPendingLogBeforeCall(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	country, local := smsTestPhone(t, w)
	start := s8TestStart.Load()

	pendingSeen := false
	fake := &fakeSmsSender{
		res: sms.SendResult{BizId: "B-pending"},
		beforeSend: func() {
			// 调平台前查库：此刻应已存在 status=0 的 pending 日志（先落 pending 再调平台）。
			var n int64
			if e := w.DB.Raw("SELECT COUNT(*) FROM `sms_logs` WHERE `phone_country` = ? AND `phone` = ? AND `status` = 0 AND `created_time` >= ?",
				country, local, start).Row().Scan(&n); e != nil {
				t.Errorf("beforeSend 查库失败：%v", e)
				return
			}
			if n >= 1 {
				pendingSeen = true
			} else {
				t.Errorf("调平台前应先存在 status=0 的 pending 日志，实际 0 行")
			}
		},
	}
	_, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
	}, fake)
	if err != nil {
		t.Fatalf("发码应成功：%v", err)
	}
	if fake.calls != 1 {
		t.Errorf("第 8 步应调用 sender.Send 恰好 1 次，实际 calls=%d（第一轮未接线时这里会 FAIL）", fake.calls)
	}
	if !pendingSeen {
		t.Errorf("beforeSend 未观测到 pending 日志（先落 pending 再调平台）")
	}
}

func TestSmsSendWritesBackSuccess(t *testing.T) {
	w := setupSmsTest(t)
	cfg := s8TwilioConfig("login")
	s8UseConfig(t, w, cfg)
	country, local := smsTestPhone(t, w)
	start := s8TestStart.Load()

	want := *cfg
	sms.ApplyDefaults(&want)

	fake := &fakeSmsSender{res: sms.SendResult{BizId: "B-1"}}
	result, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
	}, fake)
	if err != nil {
		t.Fatalf("发码应成功：%v", err)
	}
	if fake.calls != 1 {
		t.Errorf("sender.Send 应被调用 1 次，实际 %d", fake.calls)
	}
	if result == nil || result.ExpiresIn != want.CodeTTL || result.ResendAfter != want.ResendInterval {
		t.Errorf("SmsSendResult = %+v，期望 ExpiresIn=%d ResendAfter=%d", result, want.CodeTTL, want.ResendInterval)
	}

	var logRow model.SmsLog
	if e := w.DB.Where("phone_country = ? AND phone = ? AND created_time >= ?", country, local, start).
		Order("id DESC").First(&logRow).Error; e != nil {
		t.Fatalf("读回 sms_logs 失败：%v", e)
	}
	if logRow.Status != 1 {
		t.Errorf("sms_logs.status = %d，期望 1（success）", logRow.Status)
	}
	if logRow.BizId != "B-1" {
		t.Errorf("sms_logs.biz_id = %q，期望 B-1", logRow.BizId)
	}
	if logRow.CostMs <= 0 {
		t.Errorf("sms_logs.cost_ms = %d，期望 >0", logRow.CostMs)
	}

	var codeRow model.SmsCode
	if e := w.DB.Where("phone_country = ? AND phone = ? AND scene = ? AND created_time >= ?", country, local, "login", start).
		Order("id DESC").First(&codeRow).Error; e != nil {
		t.Fatalf("读回 sms_codes 失败：%v", e)
	}
	if codeRow.SendStatus != 1 {
		t.Errorf("sms_codes.send_status = %d，期望 1（success）", codeRow.SendStatus)
	}
	if codeRow.BizId != "B-1" {
		t.Errorf("sms_codes.biz_id = %q，期望 B-1", codeRow.BizId)
	}
}

func TestSmsSendPlatformFailureInvalidatesCode(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))

	type rows struct {
		log   model.SmsLog
		code  model.SmsCode
		plain string
		text  string
		calls int
	}
	run := func(t *testing.T, sendErr error) rows {
		t.Helper()
		country, local := smsTestPhone(t, w)
		start := s8TestStart.Load()
		fake := &fakeSmsSender{err: sendErr}
		_, err := w.sendSmsCodeWithSender(SmsSendInput{
			PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
		}, fake)
		if err == nil {
			t.Errorf("平台失败应返回 error")
		}
		var out rows
		out.plain, out.text, out.calls = fake.lastCode, fake.lastReq.Text, fake.calls
		if e := w.DB.Where("phone_country = ? AND phone = ? AND created_time >= ?", country, local, start).
			Order("id DESC").First(&out.log).Error; e != nil {
			t.Errorf("读回 sms_logs 失败：%v", e)
		}
		if e := w.DB.Where("phone_country = ? AND phone = ? AND created_time >= ?", country, local, start).
			Order("id DESC").First(&out.code).Error; e != nil {
			t.Errorf("读回 sms_codes 失败：%v", e)
		}
		return out
	}

	t.Run("permanent作废码且retryable=0", func(t *testing.T) {
		r := run(t, &sms.SendError{Class: sms.ClassPermanent, Code: "isv.X", Message: "m"})
		if r.calls != 1 {
			t.Errorf("sender.Send 应被调用 1 次，实际 %d", r.calls)
		}
		if r.code.SendStatus != -1 {
			t.Errorf("平台失败应置 sms_codes.send_status=-1（该码作废），实际 %d", r.code.SendStatus)
		}
		if r.log.Status != -1 {
			t.Errorf("sms_logs.status 应为 -1，实际 %d", r.log.Status)
		}
		if r.log.ErrCode != "isv.X" {
			t.Errorf("err_code = %q，期望 isv.X", r.log.ErrCode)
		}
		if r.log.ErrMsg == "" {
			t.Errorf("err_msg 应非空")
		}
		if r.log.Retryable != 0 {
			t.Errorf("permanent 应 retryable=0，实际 %d", r.log.Retryable)
		}
		// R61：err_msg 不含验证码明文 / 渲染后的正文。
		if r.plain != "" && strings.Contains(r.log.ErrMsg, r.plain) {
			t.Errorf("err_msg 含验证码明文（最严重隐私事故）")
		}
		if r.text != "" && strings.Contains(r.log.ErrMsg, r.text) {
			t.Errorf("err_msg 含渲染后的正文")
		}
	})

	t.Run("transient打标retryable=1", func(t *testing.T) {
		r := run(t, &sms.SendError{Class: sms.ClassTransient, Code: "isv.T", Message: "t"})
		if r.log.Retryable != 1 {
			t.Errorf("transient 应 retryable=1，实际 %d", r.log.Retryable)
		}
		if r.code.SendStatus != -1 {
			t.Errorf("平台失败应置 sms_codes.send_status=-1，实际 %d", r.code.SendStatus)
		}
	})

	// R61 硬要求：err_msg 用 errors.As 取 *SendError 的单行 Code/Message，
	// 不得用 err.Error()（errors.Join 的 Error() 是两行）。
	t.Run("errorsJoin单行err_msg不用errError", func(t *testing.T) {
		joined := errors.Join(sms.ErrSmsCredentialMissing, &sms.SendError{
			Class: sms.ClassPermanent, Code: "isv.J", Message: "platform said no",
		})
		r := run(t, joined)
		if r.log.ErrCode != "isv.J" {
			t.Errorf("err_code = %q，期望用 errors.As 取 *SendError.Code=isv.J", r.log.ErrCode)
		}
		if strings.ContainsAny(r.log.ErrMsg, "\n\r") {
			t.Errorf("err_msg = %q 含换行：疑似用了 errors.Join 的 err.Error()（两行）", r.log.ErrMsg)
		}
		if strings.Contains(r.log.ErrMsg, "SmsCredentialMissing") {
			t.Errorf("err_msg = %q 混入哨兵键名：说明用了 err.Error() 而非 se.Message", r.log.ErrMsg)
		}
		if r.plain != "" && strings.Contains(r.log.ErrMsg, r.plain) {
			t.Errorf("err_msg 含验证码明文")
		}
	})
}

func TestSmsSendNoNetworkInTransaction(t *testing.T) {
	_, real := s8LoadSource(t, "smsSend.go")
	// 用 t.Errorf（不用 Fatal）：第一轮未接线时正向报「未找到 .Send」变红，
	// 反向对照子测试仍能跑，RED 输出信息完整。
	if err := s8CheckSendOutsideTransaction(real); err != nil {
		t.Errorf("真实源码违规（第 8 步尚未接线时这里会 FAIL）：%v", err)
	}
	t.Logf("[检查器] s8CheckSendOutsideTransaction；变异锚点 = 合成源码把 sender.Send 挪进 w.DB.Transaction 回调")
	t.Run("反向对照_合成Send在事务内必须变红", func(t *testing.T) {
		bad := `package provider
func (w *Website) sendSmsCodeWithSender(in SmsSendInput, sender sms.Sender) (*SmsSendResult, error) {
    w.DB.Transaction(func(tx *gorm.DB) error {
        sender.Send(ctx, p, req)
        return nil
    })
    return nil, nil
}
`
		if err := s8CheckSendOutsideTransaction(s8ParseSynthetic(t, bad)); err == nil {
			t.Fatal("反向对照失效：把 Send 挪进事务后检查器竟判合规")
		}
		t.Log("反向对照：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_合成Send在事务外必须变绿", func(t *testing.T) {
		good := `package provider
func (w *Website) sendSmsCodeWithSender(in SmsSendInput, sender sms.Sender) (*SmsSendResult, error) {
    w.DB.Transaction(func(tx *gorm.DB) error { return nil })
    sender.Send(ctx, p, req)
    return nil, nil
}
`
		if err := s8CheckSendOutsideTransaction(s8ParseSynthetic(t, good)); err != nil {
			t.Fatalf("恢复后同一检查器仍报错：%v", err)
		}
		t.Log("恢复：Send 置于事务外的源码喂同一检查器 → 判合规（绿）")
	})
}

func TestSmsSendResultHasNoCode(t *testing.T) {
	src, real := s8LoadSource(t, "smsSend.go")
	if err := s8CheckResultHasNoCode(real); err != nil {
		t.Errorf("SmsSendResult 结构违规：%v", err)
	}
	t.Logf("[检查器] s8CheckResultHasNoCode；变异锚点 = 合成源码给 SmsSendResult 加 Code string 字段")
	t.Run("反向对照_合成加Code字段必须变红", func(t *testing.T) {
		bad := `package provider
type SmsSendResult struct {
    ExpiresIn int
    ResendAfter int
    Code string
}
`
		if err := s8CheckResultHasNoCode(s8ParseSynthetic(t, bad)); err == nil {
			t.Fatal("反向对照失效：加了 Code 字段后检查器竟判合规")
		}
		t.Log("反向对照：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_真实源码必须变绿", func(t *testing.T) {
		if err := s8CheckResultHasNoCode(s8ParseSynthetic(t, src)); err != nil {
			t.Fatalf("恢复后同一检查器仍报错：%v", err)
		}
		t.Log("恢复：真实源码喂同一检查器 → 判合规（绿）")
	})
}

func TestSmsCodeHashIsKeyedHMAC(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	country, local := smsTestPhone(t, w)
	start := s8TestStart.Load()

	fake := &fakeSmsSender{res: sms.SendResult{BizId: "B-hash"}}
	_, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
	}, fake)
	if err != nil {
		t.Fatalf("发码应成功：%v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("需要 sender.Send 被调用以捕获明文验证码（第一轮未接线时这里会 FAIL），calls=%d", fake.calls)
	}
	plain := fake.lastCode
	if plain == "" {
		t.Fatal("未捕获到明文验证码（检查器不得空转）")
	}

	var codeRow model.SmsCode
	if e := w.DB.Where("phone_country = ? AND phone = ? AND created_time >= ?", country, local, start).
		Order("id DESC").First(&codeRow).Error; e != nil {
		t.Fatalf("读回 sms_codes 失败：%v", e)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(codeRow.CodeHash) {
		t.Errorf("code_hash = %q，不是 64 位 hex", codeRow.CodeHash)
	}
	sum := sha256.Sum256([]byte(plain))
	if codeRow.CodeHash == hex.EncodeToString(sum[:]) {
		t.Errorf("code_hash 等于裸 sha256(明文码)：未用 TokenSecret 做 HMAC")
	}
	mac := hmac.New(sha256.New, []byte(w.TokenSecret))
	_, _ = mac.Write([]byte(plain))
	if codeRow.CodeHash != hex.EncodeToString(mac.Sum(nil)) {
		t.Errorf("code_hash 不等于 HMAC-SHA256(TokenSecret, 明文码)")
	}
}

func TestSmsLogRetentionPrune(t *testing.T) {
	w := setupSmsTest(t)
	cfg := s8TwilioConfig("login")
	cfg.LogRetentionDays = 90
	s8UseConfig(t, w, cfg)
	country, local := smsTestPhone(t, w)
	now := time.Now().Unix()

	// 插入一条 created_time 早于 now - LogRetentionDays*86400 的超期日志（本用例创建、保留号段）。
	// 说明：超期日志的 created_time 必然早于用例开始时刻，故不落在 setupSmsTest 的
	// R60 主 cleanup（只删 created_time >= start）范围内；它由被验证的第 11 步惰性清理删除，
	// 本用例末尾用「该 id 不存在」做残留断言。
	stale := &model.SmsLog{
		SiteId: w.Id, PhoneCountry: country, Phone: local, Scene: "login",
		CreatedTime: now - int64(cfg.LogRetentionDays+1)*86400,
	}
	if err := w.DB.Create(stale).Error; err != nil {
		t.Fatalf("插入超期日志失败：%v", err)
	}

	_, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: "login", Ip: s8TestIp,
	}, &fakeSmsSender{res: sms.SendResult{BizId: "B-prune"}})
	if err != nil {
		t.Fatalf("发码应成功：%v", err)
	}

	var n int64
	if e := w.DB.Raw("SELECT COUNT(*) FROM `sms_logs` WHERE `id` = ?", stale.Id).Row().Scan(&n); e != nil {
		t.Fatalf("查询超期日志失败：%v", e)
	}
	if n != 0 {
		t.Errorf("超期日志应被惰性清理删除，id=%d 仍存在（第一轮未接线时这里会 FAIL）", stale.Id)
	}

	// go/ast：惰性清理语句带 Limit(1000)（单次限量，避免长事务）。
	_, real := s8LoadSource(t, "smsSend.go")
	if err := s8CheckPruneHasLimit1000(real); err != nil {
		t.Errorf("惰性清理语句违规（第一轮未接线时这里会 FAIL）：%v", err)
	}
	t.Logf("[检查器] s8CheckPruneHasLimit1000；变异锚点 = 合成源码把 Limit(1000) 改成 Limit(999)")
	t.Run("反向对照_合成无Limit1000必须变红", func(t *testing.T) {
		bad := `package provider
func s8Bad() { w.DB.Where("created_time < ?", cutoff).Limit(999).Delete(&model.SmsLog{}) }
`
		if err := s8CheckPruneHasLimit1000(s8ParseSynthetic(t, bad)); err == nil {
			t.Fatal("反向对照失效：无 Limit(1000) 的合成源码竟判合规")
		}
		t.Log("反向对照：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_合成带Limit1000必须变绿", func(t *testing.T) {
		good := `package provider
func s8Good() { w.DB.Where("created_time < ?", cutoff).Limit(1000).Delete(&model.SmsLog{}) }
`
		if err := s8CheckPruneHasLimit1000(s8ParseSynthetic(t, good)); err != nil {
			t.Fatalf("恢复后同一检查器仍报错：%v", err)
		}
		t.Log("恢复：带 Limit(1000) 的源码喂同一检查器 → 判合规（绿）")
	})
}
