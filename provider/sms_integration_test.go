package provider

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/sms"
)

// ---------------------------------------------------------------------------
// 任务 16A：短信链路端到端回归（真库 + 假 sender，全程不触网 R62/R143）。
//
// 复用任务 8 的 setupSmsTest / s8UseConfig / s8TwilioConfig / smsTestPhone /
// fakeSmsSender，任务 10 的 s10SendCode / s10LoginReq，任务 11 的 s11CreateUser /
// s11SendCode / s11ReadUserPhone。清理按 R60：不 DROP/TRUNCATE、号段 + created_time
// 过滤、每处 DELETE 接 .Error、残留断言 0、users 与基线一致。
// ---------------------------------------------------------------------------

// smsLogTemplateColumnLen 读 information_schema 里 sms_logs.template_id 的字符长度上限。
func smsLogTemplateColumnLen(t *testing.T, w *Website) int64 {
	t.Helper()
	var length int64
	if err := w.DB.Raw(
		"SELECT COALESCE(CHARACTER_MAXIMUM_LENGTH, 0) FROM information_schema.COLUMNS " +
			"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sms_logs' AND COLUMN_NAME = 'template_id'").
		Row().Scan(&length); err != nil {
		t.Fatalf("读取 template_id 列宽失败：%v", err)
	}
	return length
}

// TestEnsureSmsLogTemplateColumnIdempotent（R138）：真实建表后把列收窄回 64，
// 复现 1406，再调用 ensureSmsLogTemplateColumn 两次：第一次加宽到 500，
// 第二次不报错且列宽仍 500。收窄只在表内没有超长值时执行（不删任何行）。
func TestEnsureSmsLogTemplateColumnIdempotent(t *testing.T) {
	w := setupSmsTest(t)

	var longRows int64
	if err := w.DB.Raw("SELECT COUNT(*) FROM `sms_logs` WHERE CHAR_LENGTH(`template_id`) > 64").
		Row().Scan(&longRows); err != nil {
		t.Fatalf("统计超长 template_id 失败：%v", err)
	}

	narrowed := false
	if longRows == 0 {
		if err := w.DB.Exec("ALTER TABLE `sms_logs` MODIFY COLUMN `template_id` varchar(64) NOT NULL DEFAULT ''").Error; err != nil {
			t.Fatalf("临时收窄列宽失败：%v", err)
		}
		narrowed = true
		if got := smsLogTemplateColumnLen(t, w); got != 64 {
			t.Fatalf("收窄后列宽 = %d，期望 64", got)
		}
		t.Log("[R138] information_schema 列宽（ALTER 前）= 64")

		// RED 复现：65 字符在 varchar(64) 上触发 1406，事务内回滚不留残留。
		longText := strings.Repeat("X", 65)
		tx := w.DB.Begin()
		err := tx.Create(&model.SmsLog{
			SiteId: w.Id, PhoneCountry: s8IntlCountry, Phone: "5550100998",
			Scene: "login", Provider: config.SmsProviderTwilio, TemplateId: longText,
		}).Error
		tx.Rollback()
		if err == nil || !strings.Contains(err.Error(), "1406") {
			t.Fatalf("65 字符插入应触发 1406 Data too long（RED 复现），实际 %v", err)
		}
		t.Logf("[R138 RED] 65 字符插入 varchar(64)：%v", err)
	} else {
		t.Logf("[R138] 表内已有 %d 行超长 template_id，跳过收窄（不删数据），仅验证幂等", longRows)
	}

	// 第一次：加宽到 500。
	if err := w.ensureSmsLogTemplateColumn(); err != nil {
		t.Fatalf("第一次 ensureSmsLogTemplateColumn 失败：%v", err)
	}
	if got := smsLogTemplateColumnLen(t, w); got != 500 {
		t.Fatalf("加宽后列宽 = %d，期望 500", got)
	}
	t.Log("[R138] information_schema 列宽（ALTER 后）= 500")

	// 第二次：幂等，不报错、列宽仍 500。
	if err := w.ensureSmsLogTemplateColumn(); err != nil {
		t.Fatalf("第二次 ensureSmsLogTemplateColumn 应幂等成功，实际 %v", err)
	}
	if got := smsLogTemplateColumnLen(t, w); got != 500 {
		t.Fatalf("幂等调用后列宽 = %d，期望 500", got)
	}

	if narrowed {
		// 加宽后 65 字符可以正常落库（回滚，不留数据）。
		tx := w.DB.Begin()
		err := tx.Create(&model.SmsLog{
			SiteId: w.Id, PhoneCountry: s8IntlCountry, Phone: "5550100998",
			Scene: "login", Provider: config.SmsProviderTwilio, TemplateId: strings.Repeat("X", 65),
		}).Error
		tx.Rollback()
		if err != nil {
			t.Fatalf("加宽后 65 字符插入应成功（GREEN），实际 %v", err)
		}
		t.Log("[R138 GREEN] 加宽后 65 字符插入成功（事务已回滚）")
	}
}

// TestSmsFullLoginJourney（计划步骤 1）：未注册号 → 发 login 码 → LoginViaSms →
// 自动建号 → 号码与区号与入参一致、token 非空。
func TestSmsFullLoginJourney(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig(sms.SceneLogin))
	country, local := smsTestPhone(t, w)

	if _, err := w.GetUserInfoByPhoneCountry(country, local); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("前置：号码应未注册，实际 %v", err)
	}

	code := s10SendCode(t, w, country, local, sms.SceneLogin)
	user, err := w.LoginViaSms(s10LoginReq(country, local, code))
	if err != nil {
		t.Fatalf("未注册号短信登录失败：%v", err)
	}
	if user == nil || user.Id == 0 {
		t.Fatal("自动注册后 user 为空")
	}
	if user.PhoneCountry != country || user.Phone != local {
		t.Errorf("自动注册号码 = (%s,%s)，期望 (%s,%s)", user.PhoneCountry, user.Phone, country, local)
	}
	if user.Token == "" {
		t.Error("登录成功但未签发 token")
	}

	var dbUser model.User
	if err := w.DB.Where("id = ?", user.Id).First(&dbUser).Error; err != nil {
		t.Fatalf("读回新用户失败：%v", err)
	}
	if dbUser.PhoneCountry != country || dbUser.Phone != local {
		t.Errorf("落库号码 = (%s,%s)，期望 (%s,%s)", dbUser.PhoneCountry, dbUser.Phone, country, local)
	}
}

// TestSmsFullBindJourney（计划步骤 1）：建号并绑旧号 → 旧号新号各发码 →
// BindUserPhone → 新号可登录（同一 user id）、旧号不再关联该账号。
//
// 说明：旧号「登录失败」在实现上等价于「旧号已不关联该账号」——LoginViaSms 对
// 未注册号会自动建号，直接断言「登录报错」会与 §6.5 自相矛盾，故这里断言旧号
// 查询为 NotFound、新号命中同一 user id。
func TestSmsFullBindJourney(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig(sms.SceneBindOld, sms.SceneBindNew, sms.SceneLogin))
	oldCountry, oldLocal := smsTestPhone(t, w)
	newCountry, newLocal := smsTestPhone(t, w)
	userId := s11CreateUser(t, w, oldCountry, oldLocal)

	oldCode, _ := s11SendCode(t, w, oldCountry, oldLocal, sms.SceneBindOld)
	newCode, _ := s11SendCode(t, w, newCountry, newLocal, sms.SceneBindNew)
	if err := w.BindUserPhone(userId, oldCode, newCountry, newLocal, newCode); err != nil {
		t.Fatalf("换绑失败：%v", err)
	}

	if got, err := w.GetUserInfoByPhoneCountry(newCountry, newLocal); err != nil || got.Id != userId {
		t.Fatalf("新号应关联 user id=%d，实际 user=%v err=%v", userId, got, err)
	}
	if _, err := w.GetUserInfoByPhoneCountry(oldCountry, oldLocal); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("旧号应已不关联任何账号（NotFound），实际 %v", err)
	}

	// 新号可登录：发 login 码 → LoginViaSms → 命中同一个 user id。
	loginCode := s10SendCode(t, w, newCountry, newLocal, sms.SceneLogin)
	user, err := w.LoginViaSms(s10LoginReq(newCountry, newLocal, loginCode))
	if err != nil {
		t.Fatalf("换绑后新号登录失败：%v", err)
	}
	if user.Id != userId {
		t.Errorf("新号登录命中 user id=%d，期望换绑前的 %d", user.Id, userId)
	}
}

// TestSmsFullResetJourney（计划步骤 1）：建号 → 发 reset 码 → 走重置密码的真实
// 步骤（VerifySmsCode(reset) → GetUserInfoByPhoneCountry → EncryptPassword → Save）
// → 新密码可通过既有密码校验（user.CheckPassword，即 LoginViaPassword 的同一判据）。
//
// 为什么不在 controller 包走 ApiSmsPasswordReset 的 HTTP 路由：controller/
// manageController 的测试进程不加载仓库根 config.json（config.initPath 只对
// provider 目录回退到仓库根），拿不到 DB；而 provider 包不能 import controller
// （循环依赖）。handler 本身只是顺序编排，顺序护栏见 controller/apiSms_test.go。
func TestSmsFullResetJourney(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig(sms.SceneReset))
	country, local := smsTestPhone(t, w)
	userId := s11CreateUser(t, w, country, local)

	const oldPwd = "old-pass-123456"
	const newPwd = "new-pass-654321"
	var user model.User
	if err := w.DB.Where("id = ?", userId).First(&user).Error; err != nil {
		t.Fatalf("读回用户失败：%v", err)
	}
	if err := user.EncryptPassword(oldPwd); err != nil {
		t.Fatalf("设置旧密码失败：%v", err)
	}
	if err := w.DB.Save(&user).Error; err != nil {
		t.Fatalf("保存旧密码失败：%v", err)
	}

	code := s11SendCodeOnly(t, w, country, local, sms.SceneReset)
	// 以下四步与 controller/apiSms.go::ApiSmsPasswordReset 逐字对应。
	if err := w.VerifySmsCode(country, local, sms.SceneReset, code); err != nil {
		t.Fatalf("重置码校验失败：%v", err)
	}
	target, err := w.GetUserInfoByPhoneCountry(country, local)
	if err != nil {
		t.Fatalf("按号码查用户失败：%v", err)
	}
	if err := target.EncryptPassword(newPwd); err != nil {
		t.Fatalf("加密新密码失败：%v", err)
	}
	if err := w.DB.Save(target).Error; err != nil {
		t.Fatalf("保存新密码失败：%v", err)
	}

	var reloaded model.User
	if err := w.DB.Where("id = ?", userId).First(&reloaded).Error; err != nil {
		t.Fatalf("重新读回用户失败：%v", err)
	}
	if !reloaded.CheckPassword(newPwd) {
		t.Error("新密码无法通过既有密码校验（CheckPassword）")
	}
	if reloaded.CheckPassword(oldPwd) {
		t.Error("旧密码仍然有效：重置未生效")
	}
}

// s11SendCodeOnly 只发码并返回明文码（不需要 sms_codes 行 id）。
func s11SendCodeOnly(t *testing.T, w *Website, country, local, scene string) string {
	t.Helper()
	code, _ := s11SendCode(t, w, country, local, scene)
	return code
}

// TestSmsSceneCrossReplayRejected（计划步骤 1）：login 码拿去 reset、
// reset 码拿去 bind_new，均必须落 ErrSmsCodeNotFound（跨场景不可重放）。
func TestSmsSceneCrossReplayRejected(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig(sms.SceneLogin, sms.SceneReset, sms.SceneBindNew))
	country, local := smsTestPhone(t, w)

	loginCode := s11SendCodeOnly(t, w, country, local, sms.SceneLogin)
	if err := w.VerifySmsCode(country, local, sms.SceneReset, loginCode); !errors.Is(err, sms.ErrSmsCodeNotFound) {
		t.Errorf("login 码拿去 reset 应返回 ErrSmsCodeNotFound，实际 %v", err)
	}

	resetCountry, resetLocal := smsTestPhone(t, w)
	resetCode := s11SendCodeOnly(t, w, resetCountry, resetLocal, sms.SceneReset)
	if err := w.VerifySmsCode(resetCountry, resetLocal, sms.SceneBindNew, resetCode); !errors.Is(err, sms.ErrSmsCodeNotFound) {
		t.Errorf("reset 码拿去 bind_new 应返回 ErrSmsCodeNotFound，实际 %v", err)
	}
}

// TestSmsLogListJSONHasNoCodeOrPlaintext（R120②）：用假 sender 造真实 sms_logs /
// sms_codes 行 → 调 GetSmsLogList 拿真实返回值 → json.Marshal 断言：
// 不含 code_hash、不含验证码明文、不含渲染后的正文，且含 template_id。
func TestSmsLogListJSONHasNoCodeOrPlaintext(t *testing.T) {
	w := setupSmsTest(t)
	cfg := s8TwilioConfig(sms.SceneLogin)
	const template = "Your verification code is {{code}}. Do not share it with anyone."
	cfg.Twilio.International.Texts[sms.SceneLogin] = template
	s8UseConfig(t, w, cfg)

	country, local := smsTestPhone(t, w)
	start := s8TestStart.Load()
	fake := &fakeSmsSender{res: sms.SendResult{BizId: "B-json"}}
	if _, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: sms.SceneLogin, Ip: s8TestIp,
	}, fake); err != nil {
		t.Fatalf("发码失败：%v", err)
	}
	plain := fake.lastCode
	if plain == "" {
		t.Fatal("假 sender 未捕获明文验证码")
	}
	rendered := strings.ReplaceAll(template, "{{code}}", plain)

	list, total := w.GetSmsLogList(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("phone_country = ? AND phone = ? AND created_time >= ?", country, local, start)
	}, 1, 20)
	if total < 1 || len(list) < 1 {
		t.Fatalf("GetSmsLogList 未返回真实行：total=%d len=%d", total, len(list))
	}

	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("序列化真实日志失败：%v", err)
	}
	body := string(raw)
	if strings.Contains(body, "code_hash") || strings.Contains(body, "CodeHash") {
		t.Errorf("日志 JSON 出现 code_hash：%s", body)
	}
	if !strings.Contains(body, `"template_id"`) {
		t.Errorf("日志 JSON 缺 template_id：%s", body)
	}
	if !strings.Contains(body, template) {
		t.Errorf("日志 JSON 未包含真实未渲染模板：%s", body)
	}
	if strings.Contains(body, rendered) {
		t.Errorf("日志 JSON 出现渲染后的正文（含验证码）：%s", body)
	}
	if containsJSONValue(t, raw, plain) {
		t.Errorf("日志 JSON 出现验证码明文：%s", body)
	}

	// sms_codes 的真实行同样不得带出摘要或明文（CodeHash json:"-"）。
	var codeRow model.SmsCode
	if err := w.DB.Where("site_id = ? AND phone_country = ? AND phone = ? AND scene = ? AND created_time >= ?",
		w.Id, country, local, sms.SceneLogin, start).Order("id DESC").First(&codeRow).Error; err != nil {
		t.Fatalf("读回 sms_codes 失败：%v", err)
	}
	codeRaw, err := json.Marshal(&codeRow)
	if err != nil {
		t.Fatalf("序列化 sms_codes 行失败：%v", err)
	}
	if strings.Contains(string(codeRaw), "code_hash") || strings.Contains(string(codeRaw), codeRow.CodeHash) {
		t.Errorf("sms_codes JSON 泄露了摘要：%s", codeRaw)
	}
	if containsJSONValue(t, codeRaw, plain) {
		t.Errorf("sms_codes JSON 出现验证码明文：%s", codeRaw)
	}
	t.Logf("真实日志 JSON：%s", body)
}

// containsJSONValue 判断 JSON 中是否有任一标量值与 target 逐字相等
// （精确相等，避免子串命中手机号等无关字段造成偶发误报）。
func containsJSONValue(t *testing.T, raw []byte, target string) bool {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("解析 JSON 失败：%v", err)
	}
	var walk func(interface{}) bool
	walk = func(n interface{}) bool {
		switch x := n.(type) {
		case map[string]interface{}:
			for _, item := range x {
				if walk(item) {
					return true
				}
			}
		case []interface{}:
			for _, item := range x {
				if walk(item) {
					return true
				}
			}
		case string:
			return x == target
		}
		return false
	}
	return walk(v)
}
