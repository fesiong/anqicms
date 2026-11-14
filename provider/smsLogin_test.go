package provider

import (
	"errors"
	"fmt"
	"os"
	"regexp"
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
// 号码选择说明（详见报告「疑虑」）：RegisterUser 的 VerifyCellphoneFormat
// （provider/user.go:1010，中国 1[3-9][0-9]{9} 正则）会拒绝国际本地号，
// 而 Twilio profile 不支持 domestic 区号。故：
//   - 不涉及自动注册的 3 条用例：国际号（smsTestPhone）+ s8TwilioConfig；
//   - 涉及自动注册的 2 条用例：国内号（86 / 199000000XX）+ 自建阿里云假配置，
//     两者都在 R60 清理号段内。
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
