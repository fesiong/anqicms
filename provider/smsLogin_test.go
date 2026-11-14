package provider

import (
	"errors"
	"fmt"
	"go/ast"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/sms"
	"kandaoni.com/anqicms/request"
)

// 编译期锚点：步骤 2 RED 时 LoginViaSms 尚未定义，编译失败落到 "undefined: LoginViaSms"。
var _ = (*Website).LoginViaSms

// ---------------------------------------------------------------------------
// 任务 10 测试基建：复用任务 8/9 的 setupSmsTest / s8UseConfig / fakeSmsSender /
// s9WrongCode / s8CnCountry / s8CnPhonePrefix / s8PhoneSeq / s8TestIp，
// 全程不联网、不发真实短信（R62）。
//
// 号码/配置选择说明（R85 返工后）：RegisterUser 仅在 PhoneNormalized=false 时
// 才跑历史 VerifyCellphoneFormat 老正则；LoginViaSms 置 true，故国际本地号也能
// 自动注册。自动注册用例按区域选假配置：
//   - 国际号（+1 5550100XXX，Region=international）：s8TwilioConfig 能过
//     resolveTwilio（AccountSid/AuthToken/From/Texts[login] 齐备）；
//   - 国内号（86 / 199000000XX，Region=domestic）：s10AliyunConfig 能过
//     resolveAliyun(domestic)（Domestic 槽凭据/签名/模板齐备）。
// 两套号段都在 R60 清理范围内；首轮「国内号+阿里云」绕开老正则的用例保留作回归。
// ---------------------------------------------------------------------------

// s10CnPhone 返回 R60 清理号段内的国内测试号，复用 s8PhoneSeq 保证同批次不撞号。
func s10CnPhone(t *testing.T) (string, string) {
	t.Helper()
	seq := s8PhoneSeq.Add(1)
	return s8CnCountry, fmt.Sprintf("%s%02d", s8CnPhonePrefix, seq%100)
}

// s10AliyunConfig 构造一份能通过 sms.Resolve(domestic) 的假凭据配置（不联网）。
func s10AliyunConfig(scenes ...string) *config.PluginSmsConfig {
	templates := map[string]string{}
	for _, s := range []string{"login", "register", "reset", "bind_old", "bind_new", "test"} {
		templates[s] = "SMS_FAKE_TEMPLATE_" + s
	}
	return &config.PluginSmsConfig{
		Provider: config.SmsProviderAliyun,
		Scenes:   scenes,
		Aliyun: config.SmsAliyunConfig{
			Domestic: config.SmsAliyunRegion{
				AccessKeyId:     "fake-access-key-id",
				AccessKeySecret: "fake-access-key-secret",
				SignName:        "【测试签名】",
				Templates:       templates,
			},
		},
	}
}

// s10SendCode 用测试接缝发一枚真库验证码，返回明文码（假 sender 捕获 req.Code）。
func s10SendCode(t *testing.T, w *Website, country, local, scene string) string {
	t.Helper()
	fake := &fakeSmsSender{res: sms.SendResult{BizId: "B-s10"}}
	if _, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: scene, Ip: s8TestIp,
	}, fake); err != nil {
		t.Fatalf("发码失败：%v", err)
	}
	if fake.lastCode == "" {
		t.Fatal("假 sender 未捕获明文验证码（检查器不得空转）")
	}
	return fake.lastCode
}

func s10LoginReq(country, local, code string) *request.ApiLoginRequest {
	return &request.ApiLoginRequest{
		Platform:     config.PlatformSms,
		PhoneCountry: country,
		Phone:        local,
		Code:         code,
	}
}

// s10CountUsers 读 users 全表行数（R60 基线核对）。
func s10CountUsers(t *testing.T, w *Website) int64 {
	t.Helper()
	var n int64
	if err := w.DB.Raw("SELECT COUNT(*) FROM `users`").Row().Scan(&n); err != nil {
		t.Fatalf("读 users 行数失败：%v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// R84 源码级护栏：crypto/rand（非 math/rand）、12/32 位、撞名重试 ≤3、
// 校验在建号之前、审核门。带反向对照（合成坏源码必红、恢复绿）。
// ---------------------------------------------------------------------------

func s10RequireLogin(src string, patterns ...string) error {
	for _, p := range patterns {
		if !regexp.MustCompile(p).MatchString(src) {
			return fmt.Errorf("缺锚点 /%s/", p)
		}
	}
	return nil
}

func s10CheckLoginSource(src string) error {
	if err := s10RequireLogin(src,
		`crypto/rand`,
		`rand\.Int\(rand\.Reader`,
		`smsLoginUserNameLen\s*=\s*12`,
		`smsLoginPasswordLen\s*=\s*32`,
		`smsLoginMaxRetry\s*=\s*3`,
		`VerifySmsCode\(`,
		`RegisterUser\(`,
		`GetDefaultStatus\(\)`,
	); err != nil {
		return err
	}
	// 只匹配 import 字面量 "math/rand"（带引号），不误伤注释里「不用 math/rand」的说明。
	if strings.Contains(src, `"math/rand"`) {
		return errors.New("随机串不得使用 math/rand（R84）")
	}
	iVerify := strings.Index(src, "VerifySmsCode(")
	iReg := strings.Index(src, "RegisterUser(")
	if iVerify < 0 || iReg < 0 || iVerify >= iReg {
		return errors.New("VerifySmsCode 未在 RegisterUser 之前（错码不得建号）")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 简报 5 条真库行为测试
// ---------------------------------------------------------------------------

func TestLoginViaSmsExistingUser(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig(sms.SceneLogin))
	country, local := smsTestPhone(t, w)

	existing := &model.User{
		UserName:     "s10_existing_" + local,
		Phone:        local,
		PhoneCountry: country,
		Status:       config.UserStatusActive,
	}
	if err := w.DB.Create(existing).Error; err != nil {
		t.Fatalf("建既有号失败：%v", err)
	}
	code := s10SendCode(t, w, country, local, sms.SceneLogin)

	user, err := w.LoginViaSms(s10LoginReq(country, local, code))
	if err != nil {
		t.Fatalf("既有号登录失败：%v", err)
	}
	if user == nil || user.Id != existing.Id {
		t.Fatalf("返回 user.Id=%v，期望既有号 %d", user, existing.Id)
	}
	if user.Token == "" {
		t.Fatal("登录成功但未签发 token（应与密码登录同形）")
	}
}

func TestLoginViaSmsAutoRegisters(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s10AliyunConfig(sms.SceneLogin))

	// R84 源码护栏 + 反向对照
	src, err := os.ReadFile("smsLogin.go")
	if err != nil {
		t.Fatalf("读 smsLogin.go 失败：%v", err)
	}
	if e := s10CheckLoginSource(string(src)); e != nil {
		t.Fatalf("R84 真实源码护栏失败：%v", e)
	}
	badSrc := `package provider
import "math/rand"
func (w *Website) LoginViaSms(req *request.ApiLoginRequest) (*model.User, error) {
	_ = rand.Int
	user, _ := w.RegisterUser(&request.ApiRegisterRequest{})
	_ = user
	return nil, nil
}`
	if e := s10CheckLoginSource(badSrc); e == nil {
		t.Fatal("R84 反向对照失败：合成坏源码仍被判绿")
	}
	if e := s10CheckLoginSource(string(src)); e != nil {
		t.Fatalf("R84 恢复绿失败：%v", e)
	}

	before := s10CountUsers(t, w)
	country, local := s10CnPhone(t)
	code := s10SendCode(t, w, country, local, sms.SceneLogin)

	user, err := w.LoginViaSms(s10LoginReq(country, local, code))
	if err != nil {
		t.Fatalf("未注册号自动注册登录失败：%v", err)
	}
	if user == nil {
		t.Fatal("user 为 nil")
	}
	// UserName 随机 12 位，且不等于手机号（§6.5）
	if len(user.UserName) != 12 {
		t.Fatalf("UserName 长度 = %d，期望 12", len(user.UserName))
	}
	if user.UserName == local || user.UserName == country+local {
		t.Fatalf("UserName 不得等于手机号：%q", user.UserName)
	}
	if user.PhoneCountry != country || user.Phone != local {
		t.Fatalf("PhoneCountry/Phone = %q/%q，期望 %q/%q",
			user.PhoneCountry, user.Phone, country, local)
	}
	// 库里多一个用户，且落库字段与入参一致
	if after := s10CountUsers(t, w); after != before+1 {
		t.Fatalf("自动注册未发生：users %d → %d", before, after)
	}
	var dbUser model.User
	if e := w.DB.Where("id = ?", user.Id).First(&dbUser).Error; e != nil {
		t.Fatalf("读回新用户失败：%v", e)
	}
	if dbUser.PhoneCountry != country || dbUser.Phone != local || len(dbUser.UserName) != 12 {
		t.Fatalf("落库字段不符：phone_country=%q phone=%q user_name=%q",
			dbUser.PhoneCountry, dbUser.Phone, dbUser.UserName)
	}
}

func TestLoginViaSmsRespectsAuditPolicy(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s10AliyunConfig(sms.SceneLogin))
	if w.PluginUser == nil {
		t.Fatal("w.PluginUser 为 nil，无法设置审核策略")
	}
	old := w.PluginUser.DefaultStatus
	w.PluginUser.DefaultStatus = "pending" // GetDefaultStatus() == 0（待审核）
	t.Cleanup(func() { w.PluginUser.DefaultStatus = old })

	before := s10CountUsers(t, w)
	country, local := s10CnPhone(t)
	code := s10SendCode(t, w, country, local, sms.SceneLogin)

	user, err := w.LoginViaSms(s10LoginReq(country, local, code))
	if err == nil {
		t.Fatal("待审核策略下不应登录成功")
	}
	if user != nil {
		t.Fatalf("待审核策略下不应返回用户，得到 %+v", user)
	}
	// R81：返回可翻译键错误（与 controller/apiUser.go 既有兜底同键），
	// 不得被吞成通用 LoginFailed，也不得回显平台内部细节。
	if err.Error() != "UserNotActive" {
		t.Fatalf("错误键 = %q，期望可翻译键 UserNotActive（R81，提示未激活/待审核）", err.Error())
	}
	if strings.Contains(err.Error(), "%!") || strings.Contains(err.Error(), "PLAT_") {
		t.Fatalf("错误含格式化/平台内部泄漏：%q", err.Error())
	}
	// 不得绕过审核：自动注册确实发生（注册后拒登，而不是干脆不注册）
	if after := s10CountUsers(t, w); after != before+1 {
		t.Fatalf("自动注册未发生（审核门被绕过？）：users %d → %d", before, after)
	}
}

func TestLoginViaSmsRejectsWrongCode(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig(sms.SceneLogin))

	before := s10CountUsers(t, w)
	country, local := smsTestPhone(t, w)
	plain := s10SendCode(t, w, country, local, sms.SceneLogin)
	wrong := s9WrongCode(plain) // 与明文码等长、每位 +1，保证 != 明文码

	user, err := w.LoginViaSms(s10LoginReq(country, local, wrong))
	if err == nil {
		t.Fatal("错码不应登录成功")
	}
	if user != nil {
		t.Fatalf("错码不应返回用户，得到 %+v", user)
	}
	// 校验在建号之前：错码路径不得创建用户
	if after := s10CountUsers(t, w); after != before {
		t.Fatalf("错码触发了自动注册：users %d → %d", before, after)
	}
	var dbUser model.User
	if e := w.DB.Where("phone_country = ? AND phone = ?", country, local).
		First(&dbUser).Error; e == nil {
		t.Fatalf("错码路径创建了用户（id=%d）", dbUser.Id)
	}
}

func TestLoginViaSmsRejectsBannedUser(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig(sms.SceneLogin))
	country, local := smsTestPhone(t, w)

	banned := &model.User{
		UserName:     "s10_banned_" + local,
		Phone:        local,
		PhoneCountry: country,
		Status:       config.UserStatusBlocked,
	}
	if err := w.DB.Create(banned).Error; err != nil {
		t.Fatalf("建封禁号失败：%v", err)
	}
	code := s10SendCode(t, w, country, local, sms.SceneLogin)

	user, err := w.LoginViaSms(s10LoginReq(country, local, code))
	if err == nil {
		t.Fatal("封禁用户不应登录成功")
	}
	if user != nil {
		t.Fatalf("封禁用户不应返回用户，得到 %+v", user)
	}
	if err.Error() != "UserNotActive" {
		t.Fatalf("错误键 = %q，期望可翻译键 UserNotActive（R81）", err.Error())
	}
}

// TestLoginViaSmsAutoRegistersInternational（R85）：本期核心场景 —— 国际短信登录
// 自动注册。旧实现里 RegisterUser 的 VerifyCellphoneFormat 老中国正则
// 1[3-9][0-9]{9} 会拒掉国际本地号（如 +1 5550100XXX），而 LoginViaSms 把所有
// 注册失败当撞名吞掉、重试 3 次后压成 LoginFailed，故旧代码此用例必红。
// 假配置用 s8TwilioConfig：国际号 Region=international，resolveTwilio
// （provider/sms/resolve.go:224-259）逐项校验 Twilio.International 槽的
// AccountSid / AuthToken / From / Texts[login] 后放行，全程不联网、不发真实短信。
func TestLoginViaSmsAutoRegistersInternational(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig(sms.SceneLogin))

	country, local := smsTestPhone(t, w) // (1, 5550100XXX)，R60 保留号段
	code := s10SendCode(t, w, country, local, sms.SceneLogin)

	before := s10CountUsers(t, w)
	user, err := w.LoginViaSms(s10LoginReq(country, local, code))
	if err != nil {
		t.Fatalf("国际号自动注册登录失败（R85：老正则不得拒国际本地号）：%v", err)
	}
	if user == nil {
		t.Fatal("user 为 nil")
	}
	if len(user.UserName) != 12 || user.UserName == local {
		t.Fatalf("UserName 非随机 12 位：%q", user.UserName)
	}
	// R86：返回值的区号必须与登录入参一致，不得回落 86。
	if user.PhoneCountry != country || user.Phone != local {
		t.Fatalf("返回 PhoneCountry/Phone = %q/%q，期望 %q/%q",
			user.PhoneCountry, user.Phone, country, local)
	}
	if after := s10CountUsers(t, w); after != before+1 {
		t.Fatalf("自动注册未发生：users %d → %d", before, after)
	}
	var dbUser model.User
	if e := w.DB.Where("id = ?", user.Id).First(&dbUser).Error; e != nil {
		t.Fatalf("读回新用户失败：%v", e)
	}
	if dbUser.PhoneCountry != country || dbUser.Phone != local {
		t.Fatalf("落库 phone_country/phone = %q/%q，期望 %q/%q（不得回落 86）",
			dbUser.PhoneCountry, dbUser.Phone, country, local)
	}
	if len(dbUser.UserName) != 12 {
		t.Fatalf("落库 UserName 非 12 位：%q", dbUser.UserName)
	}
}

// ---------------------------------------------------------------------------
// R85/R86/R88 返工护栏（源码级，带反向对照）
// ---------------------------------------------------------------------------

// s10CheckPhoneNormalizedTag 断言 ApiRegisterRequest.PhoneNormalized 带 json:"-"：
// 内部标记绝不能从前台 JSON 绑定，否则可绕过号码校验（R85）。
func s10CheckPhoneNormalizedTag(file *ast.File) error {
	st := smsStructType(file, "ApiRegisterRequest")
	if st == nil {
		return fmt.Errorf("源码里找不到 ApiRegisterRequest 结构体：检查器无法判定")
	}
	var tag string
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name != "PhoneNormalized" {
				continue
			}
			if f.Tag == nil {
				return fmt.Errorf("PhoneNormalized 字段无 struct tag：必须忽略 JSON 绑定（R85）")
			}
			v, err := strconv.Unquote(f.Tag.Value)
			if err != nil {
				return fmt.Errorf("解析 PhoneNormalized tag 失败：%v", err)
			}
			tag = v
		}
	}
	if tag == "" {
		return fmt.Errorf("ApiRegisterRequest 找不到带 tag 的 PhoneNormalized 字段（R85）")
	}
	if !strings.Contains(tag, `json:"-"`) {
		return fmt.Errorf("PhoneNormalized tag = %q，必须含 json:\"-\"（不得由前台绑定，R85）", tag)
	}
	return nil
}

func TestApiRegisterRequestPhoneNormalizedTag(t *testing.T) {
	src, real := loadSmsUserSource(t, "../request/user.go")
	if err := s10CheckPhoneNormalizedTag(real); err != nil {
		t.Fatal(err)
	}
	t.Log(`[检查器] s10CheckPhoneNormalizedTag；变异锚点 = json:"-" → json:"phone_normalized"`)
	t.Run("反向对照_可JSON绑定必须变红", func(t *testing.T) {
		badSrc := strings.Replace(src, `json:"-"`, `json:"phone_normalized"`, 1)
		if badSrc == src {
			t.Fatal(`反向对照锚点失效：json:"-" 没被替换`)
		}
		if err := s10CheckPhoneNormalizedTag(parseSmsSynthetic(t, badSrc)); err == nil {
			t.Fatal("反向对照失效：改成可从前台 JSON 绑定后检查器竟判合规")
		}
		t.Log("反向对照：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_未变异源码必须变绿", func(t *testing.T) {
		if err := s10CheckPhoneNormalizedTag(parseSmsSynthetic(t, src)); err != nil {
			t.Fatalf("恢复后同一个检查器仍报错：%v", err)
		}
		t.Log("恢复：未变异源码喂同一检查器 → 判合规（绿）")
	})
}

// s10CheckPhoneCountryWiring 断言 R86 的两条源码事实：
// RegisterUser 构造 model.User 时写入 req.PhoneCountry；smsLogin.go 不再事后回写区号。
func s10CheckPhoneCountryWiring(userSrc, loginSrc string) error {
	if !regexp.MustCompile(`PhoneCountry:\s*req\.PhoneCountry`).MatchString(userSrc) {
		return errors.New("RegisterUser 未把 req.PhoneCountry 写进 model.User（guard 会回落 86，R86）")
	}
	if strings.Contains(loginSrc, `Update("phone_country"`) {
		return errors.New(`smsLogin.go 仍存在事后 Update("phone_country")（R86 要求删除）`)
	}
	return nil
}

// TestRegisterUserPhoneCountryConsistency（R86）：直接调 RegisterUser，证明
// createUserWithPhoneGuard 拿到的是正确区号，而非把空区号规范化成 86。
// 期望按 §4.4/§6.6/R55 推导：(区号,号码) 为逻辑唯一键 —— 同区号同号拒绝、
// 不同区号同号放行；两种情形落库区号都不得是 86。
func TestRegisterUserPhoneCountryConsistency(t *testing.T) {
	userSrc, e := os.ReadFile("user.go")
	if e != nil {
		t.Fatalf("读 user.go 失败：%v", e)
	}
	loginSrc, e := os.ReadFile("smsLogin.go")
	if e != nil {
		t.Fatalf("读 smsLogin.go 失败：%v", e)
	}
	if e := s10CheckPhoneCountryWiring(string(userSrc), string(loginSrc)); e != nil {
		t.Fatalf("R86 真实源码护栏失败：%v", e)
	}
	t.Run("反向对照_区号不直连必须变红", func(t *testing.T) {
		re := regexp.MustCompile(`PhoneCountry:\s*req\.PhoneCountry`)
		bad := re.ReplaceAllString(string(userSrc), `PhoneCountry: ""`)
		if bad == string(userSrc) {
			t.Fatal("反向对照锚点失效：PhoneCountry: req.PhoneCountry 没被替换")
		}
		if e := s10CheckPhoneCountryWiring(bad, string(loginSrc)); e == nil {
			t.Fatal("反向对照失效：区号不直连后检查器竟判合规")
		}
		t.Log("反向对照①：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("反向对照_恢复事后回写必须变红", func(t *testing.T) {
		bad := string(loginSrc) + "\nfunc (w *Website) s10BadUpdate() { _ = w.DB.Model(&model.User{}).Update(\"phone_country\", \"1\") }\n"
		if e := s10CheckPhoneCountryWiring(string(userSrc), bad); e == nil {
			t.Fatal("反向对照失效：恢复事后回写后检查器竟判合规")
		}
		t.Log("反向对照②：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_未变异源码必须变绿", func(t *testing.T) {
		if e := s10CheckPhoneCountryWiring(string(userSrc), string(loginSrc)); e != nil {
			t.Fatalf("恢复后同一个检查器仍报错：%v", e)
		}
		t.Log("恢复：未变异源码喂同一检查器 → 判合规（绿）")
	})

	w := setupSmsTest(t)
	// 「同号不同区号」会落到 (1, X) 与 (44, X)；44 不在 s8CleanupWhere 的两个
	// (country,prefix) 组合内，故追加一个按 phone = X AND created_time >= start
	// 的精确清理（DELETE 接 .Error + 残留断言），与 R60 号段清理叠加，保证本用例
	// 自建的所有行都被回收、users 回到基线 339。
	seq := s8PhoneSeq.Add(1)
	local := fmt.Sprintf("%s%03d", s8IntlPhonePrefix, seq%1000)
	start := s8TestStart.Load()
	t.Cleanup(func() {
		res := w.DB.Exec("DELETE FROM `users` WHERE `phone` = ? AND `created_time` >= ?", local, start)
		if res.Error != nil {
			t.Errorf("[R60 补充清理] DELETE users(phone=?) 失败：%v", res.Error)
		}
		var remain int64
		if err := w.DB.Raw("SELECT COUNT(*) FROM `users` WHERE `phone` = ? AND `created_time` >= ?", local, start).Row().Scan(&remain); err != nil {
			t.Errorf("[R60 补充清理] 残留查询失败：%v", err)
		} else if remain != 0 {
			t.Errorf("[R60 补充清理] phone=%s 仍有 %d 行残留", local, remain)
		}
	})

	// ① (1, X) 直接调 RegisterUser：PhoneNormalized=true 跳过老正则；落库区号必须为 1。
	u1, err := w.RegisterUser(&request.ApiRegisterRequest{
		UserName:        "s10_r86_a_" + local,
		Password:        "s10-r86-pass-" + local,
		Phone:           local,
		PhoneCountry:    "1",
		PhoneNormalized: true,
	})
	if err != nil {
		t.Fatalf("① 以 (1, %s) 注册失败：%v", local, err)
	}
	var db1 model.User
	if e := w.DB.Where("id = ?", u1.Id).First(&db1).Error; e != nil {
		t.Fatalf("① 读回用户失败：%v", e)
	}
	if db1.PhoneCountry != "1" || db1.Phone != local {
		t.Fatalf("① 落库 phone_country/phone = %q/%q，期望 1/%s（guard 不得回落 86，R86）",
			db1.PhoneCountry, db1.Phone, local)
	}
	t.Logf("① 落库实测 phone_country=%q phone=%q（正确区号，非 86）", db1.PhoneCountry, db1.Phone)

	// ② 同本地号、同区号 (1, X) 再来一次：§4.4 应用层比对命中，应拒绝、不产生重复行。
	_, err = w.RegisterUser(&request.ApiRegisterRequest{
		UserName:        "s10_r86_b_" + local,
		Password:        "s10-r86-pass-" + local,
		Phone:           local,
		PhoneCountry:    "1",
		PhoneNormalized: true,
	})
	if err == nil {
		t.Fatal("② 同 (区号=1, 号码) 第二次注册应被 guard 拒绝，却成功（会产生重复行）")
	}
	t.Logf("② 同区号同号第二次被拒：%q", err.Error())

	// ③ 同本地号、不同区号 (44, X)：§4.4/§6.6 不同键，应放行。
	u3, err := w.RegisterUser(&request.ApiRegisterRequest{
		UserName:        "s10_r86_c_" + local,
		Password:        "s10-r86-pass-" + local,
		Phone:           local,
		PhoneCountry:    "44",
		PhoneNormalized: true,
	})
	if err != nil {
		t.Fatalf("③ 同号不同区号 (44, %s) 应被放行，却报错：%v", local, err)
	}
	_ = u3
	t.Log("③ 同号不同区号 (44, X) 放行成功")

	// ④ 汇总：phone=X 恰好 2 行，区号集合 {1,44}，绝无 86。
	var rows []model.User
	if e := w.DB.Where("phone = ?", local).Find(&rows).Error; e != nil {
		t.Fatalf("④ 查询 phone=%s 失败：%v", local, e)
	}
	if len(rows) != 2 {
		t.Fatalf("④ phone=%s 共 %d 行，期望 2 行（(1,X) 与 (44,X)，无重复、无 86 回落）", local, len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.PhoneCountry] = true
		if r.PhoneCountry == "86" {
			t.Fatalf("④ 出现 phone_country=86 的回落行：guard 没用正确区号（R86 未修）")
		}
	}
	if !seen["1"] || !seen["44"] {
		t.Fatalf("④ 区号集合 = %v，期望 {1,44}", seen)
	}
	t.Logf("④ 实测：phone=%s 共 2 行，区号集合 {1,44}，无 86 回落", local)
}

// s10CheckR88Source 断言 R88：RegisterUser 失败分支带 GetUserInfoByUserName 撞名
// 判据 + 原样 return nil, e，且不得回到无条件 if e != nil { continue } 旧写法。
func s10CheckR88Source(loginSrc string) error {
	if !strings.Contains(loginSrc, "w.GetUserInfoByUserName(") {
		return errors.New("缺少 GetUserInfoByUserName 撞名判据（R88）")
	}
	if !regexp.MustCompile(`return nil, e\b`).MatchString(loginSrc) {
		return errors.New("RegisterUser 失败分支未原样返回 e（R88：不得吞成 LoginFailed）")
	}
	if regexp.MustCompile(`if e != nil \{\s*continue`).MatchString(loginSrc) {
		return errors.New("仍存在无条件吞错误的 if e != nil { continue }（R88）")
	}
	return nil
}

// TestLoginViaSmsNonCollisionErrorNotSwallowed（R88）：证明非撞名错误不被吞。
func TestLoginViaSmsNonCollisionErrorNotSwallowed(t *testing.T) {
	w := setupSmsTest(t)

	// (1) 分类函数行为：不存在的 UserName → 非撞名（false，失败须原样返回）；
	//     预置一个真实用户占用该 UserName 后 → 撞名（true，才换名重试）。
	probe := "s10_r88_probe"
	if w.smsLoginIsNameCollision(probe) {
		t.Fatal("探针用户名尚未创建却被判为撞名（检查器不得空转）")
	}
	seq := s8PhoneSeq.Add(1)
	local := fmt.Sprintf("%s%03d", s8IntlPhonePrefix, seq%1000)
	existing := &model.User{
		UserName:     probe,
		Phone:        local,
		PhoneCountry: "1",
		Status:       config.UserStatusActive,
	}
	if e := w.DB.Create(existing).Error; e != nil {
		t.Fatalf("预置撞名用户失败：%v", e)
	}
	if !w.smsLoginIsNameCollision(probe) {
		t.Fatal("已存在的 UserName 未被判为撞名（分类函数失效）")
	}
	t.Log("(1) 分类判据：不存在→false（非撞名，原样返回 e）；已存在→true（撞名，换名重试）")

	// (2) 源码护栏 + 反向对照。
	loginSrc, err := os.ReadFile("smsLogin.go")
	if err != nil {
		t.Fatalf("读 smsLogin.go 失败：%v", err)
	}
	if e := s10CheckR88Source(string(loginSrc)); e != nil {
		t.Fatalf("R88 真实源码护栏失败：%v", e)
	}
	t.Run("反向对照_删撞名判据必须变红", func(t *testing.T) {
		bad := strings.Replace(string(loginSrc), "w.GetUserInfoByUserName(", "w.GetUserInfoByEmail(", 1)
		if bad == string(loginSrc) {
			t.Fatal("反向对照锚点失效：w.GetUserInfoByUserName( 没被替换")
		}
		if e := s10CheckR88Source(bad); e == nil {
			t.Fatal("反向对照失效：删掉撞名判据后检查器竟判合规")
		}
		t.Log("反向对照①：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("反向对照_失败不原样返回必须变红", func(t *testing.T) {
		re := regexp.MustCompile(`return nil, e\b`)
		bad := re.ReplaceAllString(string(loginSrc), "continue")
		if bad == string(loginSrc) {
			t.Fatal("反向对照锚点失效：return nil, e 没被替换")
		}
		if e := s10CheckR88Source(bad); e == nil {
			t.Fatal("反向对照失效：失败不原样返回后检查器竟判合规")
		}
		t.Log("反向对照②：合成坏源码喂同一检查器 → 判违规（红）")
	})
	t.Run("恢复_未变异源码必须变绿", func(t *testing.T) {
		if e := s10CheckR88Source(string(loginSrc)); e != nil {
			t.Fatalf("恢复后同一个检查器仍报错：%v", e)
		}
		t.Log("恢复：未变异源码喂同一检查器 → 判合规（绿）")
	})
}
