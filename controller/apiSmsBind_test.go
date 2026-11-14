package controller

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 源码级护栏检查器：接受源码字符串，返回第一个违规原因。
// 同一检查器同时喂「真实源码」与「合成坏源码」，实现反向对照（必红／恢复绿）。
// 复用本包既有的 readFuncSource / mustContain / mustNotContain；声明式遍历
// patterns，不写死总数（R96）。
// ---------------------------------------------------------------------------

func s11Require(src string, patterns ...string) error {
	for _, p := range patterns {
		if !regexp.MustCompile(p).MatchString(src) {
			return fmt.Errorf("缺锚点 /%s/", p)
		}
	}
	return nil
}

// s11CheckRegisterForcesSmsCode（R90 ②）：传 phone 时先归一化、回填、
// 置 PhoneNormalized=true，并强制校验 register 场景码；code == "" 不得旁路。
func s11CheckRegisterForcesSmsCode(src string) error {
	if err := s11Require(src,
		`if\s+req\.Phone\s*!=\s*""`,
		`sms\.NormalizePhone\(\s*req\.PhoneCountry\s*,\s*req\.Phone\s*\)`,
		`req\.PhoneNormalized\s*=\s*true`,
		`VerifySmsCode\(\s*p\.Country\s*,\s*p\.Local\s*,\s*sms\.SceneRegister\s*,\s*req\.Code\s*\)`,
		`respondSmsError\(`,
	); err != nil {
		return err
	}
	// 「code 为空就跳过 / code 非空才校验」都是旁路，必须没有。
	if regexp.MustCompile(`req\.Code\s*(!=|==)\s*""`).MatchString(src) {
		return errors.New(`存在按 req.Code 是否为空分叉的旁路（code == "" 不得跳过校验）`)
	}
	return nil
}

// s11CheckRegisterUnchangedWithoutPhone（R90 ③）：VerifySmsCode 必须被
// `if req.Phone != ""` 分支包住（guard 在 verify 之前、verify 在 RegisterUser 之前），
// 保证不传 phone 时不进入短信校验分支、行为与 BASE 逐字一致。
func s11CheckRegisterUnchangedWithoutPhone(src string) error {
	if err := s11Require(src,
		`if\s+req\.Phone\s*!=\s*""`,
		`VerifySmsCode\(`,
		`RegisterUser\(&req\)`,
	); err != nil {
		return err
	}
	iPhone := strings.Index(src, `req.Phone != ""`)
	iVerify := strings.Index(src, "VerifySmsCode(")
	iRegister := strings.Index(src, "RegisterUser(&req)")
	if iPhone < 0 || iVerify < 0 || iRegister < 0 {
		return errors.New("缺少 phone guard / VerifySmsCode / RegisterUser 锚点")
	}
	if !(iPhone < iVerify && iVerify < iRegister) {
		return errors.New(`VerifySmsCode 未被 if req.Phone != "" 包住（不传 phone 会误入短信分支）`)
	}
	return nil
}

// s11CheckRegisterNormalizesPhone（R90 ①⑤ / ④ 的 controller 半边）：
// ApiRegister 必先 sms.NormalizePhone 成功（回填 p.Country/p.Local）才置
// PhoneNormalized = true，且都发生在 RegisterUser 之前；guard 的区号来源
// 只能是这个已校验值。
func s11CheckRegisterNormalizesPhone(src string) error {
	if err := s11Require(src,
		`sms\.NormalizePhone\(\s*req\.PhoneCountry\s*,\s*req\.Phone\s*\)`,
		`req\.PhoneCountry\s*=\s*p\.Country`,
		`req\.Phone\s*=\s*p\.Local`,
		`req\.PhoneNormalized\s*=\s*true`,
		`respondSmsError\(`,
		`RegisterUser\(&req\)`,
	); err != nil {
		return err
	}
	iNorm := strings.Index(src, "sms.NormalizePhone(")
	iTrue := strings.Index(src, "req.PhoneNormalized = true")
	iReg := strings.Index(src, "RegisterUser(&req)")
	if !(iNorm < iTrue && iTrue < iReg) {
		return errors.New("必须先 NormalizePhone 成功才置 PhoneNormalized=true，且都在 RegisterUser 之前")
	}
	return nil
}

// s11CheckResetHidesState（R93）：reset 与发码两侧都不得出现 UserDoesNotExist／
// 按注册状态查用户的分叉；reset 查不到用户返回 &sms.AttemptsError{Remaining: 0}，
// 绝不裸返回 sms.ErrSmsCodeMismatch（该键文案含 %d，无参会输出 %!(NOVERB)，R82）。
func s11CheckResetHidesState(resetSrc, sendSrc string) error {
	for _, src := range []string{resetSrc, sendSrc} {
		if strings.Contains(src, "UserDoesNotExist") {
			return errors.New("出现 UserDoesNotExist：reset 发码/重置侧不得区分注册状态")
		}
	}
	if err := s11Require(resetSrc,
		`GetUserInfoByPhoneCountry\(`,
		`&sms\.AttemptsError\{Remaining:\s*0\}`,
		`respondSmsError\(`,
		`config\.StatusOK`,
		`"msg":\s*""`,
	); err != nil {
		return err
	}
	if regexp.MustCompile(`return\s+sms\.ErrSmsCodeMismatch`).MatchString(resetSrc) {
		return errors.New("reset 不得裸返回 sms.ErrSmsCodeMismatch（R82/R93）")
	}
	// 发码侧现状已不查注册状态：不得新增按注册状态查用户的分叉。
	if regexp.MustCompile(`GetUserInfoByPhoneCountry|GetUserInfoByPhone\b`).MatchString(sendSrc) {
		return errors.New("发码侧新增了按注册状态查用户的分叉（R93）")
	}
	return nil
}

// s11CheckPhoneBindRoute（R94）：路由挂 middleware.UserAuth；reset 端点存在、
// 且不挂 AccountProbeGuard（该中间件预算语义围绕三个邮箱探针，R94 明令不加挂）。
func s11CheckPhoneBindRoute(src string) error {
	if err := s11Require(src,
		`"/sms/phone/bind"`,
		`middleware\.UserAuth\s*,\s*controller\.ApiSmsPhoneBind`,
		`"/sms/password/reset"`,
		`controller\.ApiSmsPasswordReset`,
	); err != nil {
		return err
	}
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, `"/sms/password/reset"`) && strings.Contains(line, "AccountProbeGuard") {
			return errors.New("/sms/password/reset 不得挂 AccountProbeGuard（R94）")
		}
	}
	return nil
}

// s11CheckBindHandlerUsesLogin（R94）：userId 取 ctx.Values().GetUintDefault，
// 为 0 直接拒；错误走 respondSmsError；成功返回 {code: StatusOK, msg: ""}。
func s11CheckBindHandlerUsesLogin(src string) error {
	return s11Require(src,
		`ctx\.Values\(\)\.GetUintDefault\("userId",\s*0\)`,
		`userId\s*==\s*0`,
		`respondSmsError\(`,
		`BindUserPhone\(\s*userId\s*,`,
		`config\.StatusOK`,
		`"msg":\s*""`,
	)
}

// s11CheckVerifyTxUsesOnlyTx（R91）：verifySmsCodeTx 双返回值区分业务/DB 错误，
// 全程只用传入的 tx，不得出现 w.DB 或再开 .Transaction（否则外层回滚撤不掉内层写回）。
func s11CheckVerifyTxUsesOnlyTx(src string) error {
	if err := s11Require(src,
		`func \(w \*Website\) verifySmsCodeTx\(tx \*gorm\.DB, p sms\.Phone, scene, code string\) \(bizErr, dbErr error\)`,
		`tx\.Clauses\(clause\.Locking`,
		`tx\.Model\(&model\.SmsCode\{\}\)`,
	); err != nil {
		return err
	}
	if regexp.MustCompile(`w\.DB\b`).MatchString(src) {
		return errors.New("verifySmsCodeTx 内不得出现 w.DB（必须只用传入的 tx，R91）")
	}
	if regexp.MustCompile(`\.Transaction\(`).MatchString(src) {
		return errors.New("verifySmsCodeTx 内不得再开事务（R91）")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 测试（每条都带反向对照：合成坏源码必红、恢复绿）
// ---------------------------------------------------------------------------

func TestApiRegisterForcesSmsCodeWhenPhonePresent(t *testing.T) {
	src := readFuncSource(t, "apiUser.go", "ApiRegister")
	if err := s11CheckRegisterForcesSmsCode(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	mustContain(t, src, `VerifySmsCode\(\s*p\.Country\s*,\s*p\.Local\s*,\s*sms\.SceneRegister\s*,\s*req\.Code\s*\)`,
		"传 phone 时必须强制校验 register 场景短信码")

	// 反向对照：合成「if req.Code != \"\" 才校验」的旁路源码，同一检查器必红。
	badSrc := `func ApiRegister(ctx iris.Context) {
		if req.Phone != "" {
			p, _ := sms.NormalizePhone(req.PhoneCountry, req.Phone)
			req.PhoneNormalized = true
			if req.Code != "" {
				_ = currentSite.VerifySmsCode(p.Country, p.Local, sms.SceneRegister, req.Code)
			}
			respondSmsError(ctx, currentSite, nil)
		}
	}`
	if err := s11CheckRegisterForcesSmsCode(badSrc); err == nil {
		t.Fatal("反向对照失败：存在 code 非空才校验的旁路，护栏仍判绿")
	}
	if err := s11CheckRegisterForcesSmsCode(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

func TestApiRegisterUnchangedWithoutPhone(t *testing.T) {
	src := readFuncSource(t, "apiUser.go", "ApiRegister")
	if err := s11CheckRegisterUnchangedWithoutPhone(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}

	// 反向对照：合成「无条件先校验短信码」的源码（不传 phone 也会进入短信分支）。
	badSrc := `func ApiRegister(ctx iris.Context) {
		p, _ := sms.NormalizePhone(req.PhoneCountry, req.Phone)
		_ = currentSite.VerifySmsCode(p.Country, p.Local, sms.SceneRegister, req.Code)
		user, err := currentSite.RegisterUser(&req)
		_, _ = user, err
	}`
	if err := s11CheckRegisterUnchangedWithoutPhone(badSrc); err == nil {
		t.Fatal("反向对照失败：无 phone guard 的源码仍判绿")
	}
	if err := s11CheckRegisterUnchangedWithoutPhone(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

func TestApiRegisterNormalizesPhoneBeforeRegisterUser(t *testing.T) {
	src := readFuncSource(t, "apiUser.go", "ApiRegister")
	if err := s11CheckRegisterNormalizesPhone(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	mustContain(t, src, `req\.PhoneNormalized\s*=\s*true`,
		"R90：归一化成功后必须置内部标记，且该标记 json:\"-\" 不受影响")

	// 反向对照（R90）：合成「无归一化就置 true」的源码必红。
	badSrc := `func ApiRegister(ctx iris.Context) {
		req.PhoneNormalized = true
		user, err := currentSite.RegisterUser(&req)
		_, _ = user, err
	}`
	if err := s11CheckRegisterNormalizesPhone(badSrc); err == nil {
		t.Fatal("反向对照失败：无 sms.NormalizePhone 就置 PhoneNormalized=true 的源码仍判绿")
	}
	if err := s11CheckRegisterNormalizesPhone(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

func TestApiSmsResetHidesRegistrationState(t *testing.T) {
	resetSrc := readFuncSource(t, "apiSms.go", "ApiSmsPasswordReset")
	sendSrc := readFuncSource(t, "apiSms.go", "ApiSmsSend")
	if err := s11CheckResetHidesState(resetSrc, sendSrc); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}
	mustNotContain(t, resetSrc, `UserDoesNotExist`, "reset 不得区分注册状态")
	mustNotContain(t, sendSrc, `UserDoesNotExist`, "发码侧不得区分注册状态")

	// 反向对照①：合成含 UserDoesNotExist 的 reset 源码，同一检查器必红。
	badReset := `func ApiSmsPasswordReset(ctx iris.Context) {
		user, err := currentSite.GetUserInfoByPhoneCountry(p.Country, p.Local)
		if err != nil {
			respondSmsError(ctx, currentSite, errors.New("UserDoesNotExist"))
			return
		}
		_ = user
	}`
	if err := s11CheckResetHidesState(badReset, sendSrc); err == nil {
		t.Fatal("反向对照失败：含 UserDoesNotExist 的 reset 源码仍判绿")
	}
	// 反向对照②：发码侧新增按注册状态查用户的分叉，必红。
	badSend := `func ApiSmsSend(ctx iris.Context) {
		_, _ = currentSite.GetUserInfoByPhoneCountry(req.PhoneCountry, req.Phone)
	}`
	if err := s11CheckResetHidesState(resetSrc, badSend); err == nil {
		t.Fatal("反向对照失败：发码侧按注册状态分叉的源码仍判绿")
	}
	if err := s11CheckResetHidesState(resetSrc, sendSrc); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

func TestApiSmsPhoneBindRequiresLogin(t *testing.T) {
	src := readFuncSource(t, "../route/base.go", "Register")
	if err := s11CheckPhoneBindRoute(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}

	// 反向对照：/sms/phone/bind 没挂 UserAuth 的源码必红。
	badSrc := `func Register(app *iris.Application, systemFiles embed.FS) {
		api.Post("/sms/phone/bind", controller.ApiSmsPhoneBind)
		api.Post("/sms/password/reset", controller.ApiSmsPasswordReset)
	}`
	if err := s11CheckPhoneBindRoute(badSrc); err == nil {
		t.Fatal("反向对照失败：/sms/phone/bind 未挂 middleware.UserAuth 的源码仍判绿")
	}
	if err := s11CheckPhoneBindRoute(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

func TestApiSmsPhoneBindUsesLoginUserId(t *testing.T) {
	src := readFuncSource(t, "apiSms.go", "ApiSmsPhoneBind")
	if err := s11CheckBindHandlerUsesLogin(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}

	// 反向对照：userId 写死 / 不取登录态的源码必红。
	badSrc := `func ApiSmsPhoneBind(ctx iris.Context) {
		userId := uint(1)
		_ = currentSite.BindUserPhone(userId, req.OldCode, req.NewPhoneCountry, req.NewPhone, req.NewCode)
	}`
	if err := s11CheckBindHandlerUsesLogin(badSrc); err == nil {
		t.Fatal("反向对照失败：不取 ctx.Values().GetUintDefault(\"userId\") 的源码仍判绿")
	}
	if err := s11CheckBindHandlerUsesLogin(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}

func TestVerifySmsCodeTxUsesOnlyPassedTx(t *testing.T) {
	src := readFuncSource(t, "../provider/smsVerify.go", "verifySmsCodeTx")
	if err := s11CheckVerifyTxUsesOnlyTx(src); err != nil {
		t.Fatalf("真实源码护栏失败：%v", err)
	}

	// 反向对照：内层再开 w.DB.Transaction 的源码必红。
	badSrc := `func (w *Website) verifySmsCodeTx(tx *gorm.DB, p sms.Phone, scene, code string) (bizErr, dbErr error) {
		tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row)
		tx.Model(&model.SmsCode{}).Update("attempts", 1)
		_ = w.DB.Transaction(func(tx2 *gorm.DB) error { return nil })
		return
	}`
	if err := s11CheckVerifyTxUsesOnlyTx(badSrc); err == nil {
		t.Fatal("反向对照失败：内层开事务 / 用 w.DB 的源码仍判绿")
	}
	if err := s11CheckVerifyTxUsesOnlyTx(src); err != nil {
		t.Fatalf("恢复绿失败：%v", err)
	}
}
