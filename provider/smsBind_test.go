package provider

import (
	"fmt"
	"sync/atomic"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/sms"
	"kandaoni.com/anqicms/request"
)

// 编译期锚点：第一轮 RED 时 BindUserPhone 尚未定义，编译失败落到
// "undefined: BindUserPhone"（与任务 8/9/10 同款护栏）。
var _ = (*Website).BindUserPhone

var s11UserSeq atomic.Uint64

// s11SendCode 向「指定号码 + 指定场景」发一枚真库验证码，返回明文码与 sms_codes 行 id。
// 与 s9SendCode 的区别：号码由调用方指定（换绑需要旧号/新号各自固定），
// 不重新生成随机号。fakeSmsSender.lastCode 是唯一合法取码途径（R96），全程不触网（R62）。
func s11SendCode(t *testing.T, w *Website, country, local, scene string) (string, uint) {
	t.Helper()
	fake := &fakeSmsSender{res: sms.SendResult{BizId: "B-s11"}}
	if _, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: scene, Ip: s8TestIp,
	}, fake); err != nil {
		t.Fatalf("发码失败(scene=%s, phone=%s%s)：%v", scene, country, local, err)
	}
	if fake.lastCode == "" {
		t.Fatal("假 sender 未捕获到明文验证码（检查器不得空转）")
	}
	var row model.SmsCode
	if err := w.DB.Where("site_id = ? AND phone_country = ? AND phone = ? AND scene = ? AND created_time >= ?",
		w.Id, country, local, scene, s8TestStart.Load()).
		Order("id DESC").First(&row).Error; err != nil {
		t.Fatalf("读回 sms_codes 失败：%v", err)
	}
	return fake.lastCode, row.Id
}

// s11TrackUserDeletion 在 t.Cleanup 按 id 精确删除用例创建的用户，并断言残留为 0（R96）。
// t.Cleanup 为 LIFO：此处晚于 setupSmsTest 的 s8Cleanup 注册，故先执行，
// 之后 s8Cleanup 的 users 总行数核对仍能与基线一致。
func s11TrackUserDeletion(t *testing.T, w *Website, id uint) {
	t.Helper()
	t.Cleanup(func() {
		res := w.DB.Exec("DELETE FROM `users` WHERE `id` = ?", id)
		if res.Error != nil {
			t.Errorf("[R96 清理] 按 id=%d 精确删除用户失败：%v", id, res.Error)
			return
		}
		var remain int64
		if err := w.DB.Raw("SELECT COUNT(*) FROM `users` WHERE `id` = ?", id).Row().Scan(&remain); err != nil {
			t.Errorf("[R96 残留] 查询 id=%d 失败：%v", id, err)
			return
		}
		if remain != 0 {
			t.Errorf("[R96 残留] 用户 id=%d 仍有 %d 行未删除", id, remain)
		} else {
			t.Logf("[R96 残留] 用户 id=%d 已按 id 精确删除，残留 0 行", id)
		}
	})
}

// s11CreateUser 直接建一个真实用户（不走 RegisterUser，避免 ProcessNewUser 等副作用），
// 并登记按 id 精确删除 + 残留断言（R96）。
func s11CreateUser(t *testing.T, w *Website, country, phone string) uint {
	t.Helper()
	u := model.User{
		UserName:     fmt.Sprintf("s11user_%d", s11UserSeq.Add(1)),
		Phone:        phone,
		PhoneCountry: country,
		Status:       config.UserStatusActive,
	}
	if err := w.DB.Create(&u).Error; err != nil {
		t.Fatalf("创建用户失败(phone=%s%s)：%v", country, phone, err)
	}
	s11TrackUserDeletion(t, w, u.Id)
	return u.Id
}

// s11ReadUserPhone 按 id 精确读回用户的 (phone_country, phone)。
func s11ReadUserPhone(t *testing.T, w *Website, id uint) (string, string) {
	t.Helper()
	var country, phone string
	if err := w.DB.Raw("SELECT `phone_country`, `phone` FROM `users` WHERE `id` = ?", id).
		Row().Scan(&country, &phone); err != nil {
		t.Fatalf("读回用户 id=%d 手机号失败：%v", id, err)
	}
	return country, phone
}

// TestBindUserPhoneBothCodes：用户已绑旧号 → 旧号新号各发一码 → 换绑成功，
// 库里 phone／phone_country 变成新号，两个码的 consumed_at 都 > 0（同一事务内消费）。
func TestBindUserPhoneBothCodes(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("bind_old", "bind_new"))
	oldCountry, oldLocal := smsTestPhone(t, w)
	newCountry, newLocal := smsTestPhone(t, w)
	userId := s11CreateUser(t, w, oldCountry, oldLocal)

	oldCode, oldId := s11SendCode(t, w, oldCountry, oldLocal, sms.SceneBindOld)
	newCode, newId := s11SendCode(t, w, newCountry, newLocal, sms.SceneBindNew)

	if err := w.BindUserPhone(userId, oldCode, newCountry, newLocal, newCode); err != nil {
		t.Fatalf("换绑应成功，得到 %v", err)
	}
	country, phone := s11ReadUserPhone(t, w, userId)
	if country != newCountry || phone != newLocal {
		t.Errorf("换绑后号码 = (%s, %s)，期望 (%s, %s)", country, phone, newCountry, newLocal)
	} else {
		t.Logf("[BothCodes] 换绑后号码 = (%s, %s)", country, phone)
	}
	if got := s9ReadConsumedAt(t, w, oldId); got <= 0 {
		t.Errorf("旧号码 consumed_at 应 > 0，实际 %d", got)
	} else {
		t.Logf("[BothCodes] 旧码 consumed_at = %d（> 0）", got)
	}
	if got := s9ReadConsumedAt(t, w, newId); got <= 0 {
		t.Errorf("新号码 consumed_at 应 > 0，实际 %d", got)
	} else {
		t.Logf("[BothCodes] 新码 consumed_at = %d（> 0）", got)
	}
}

// TestBindUserPhoneRollsBackOnNewCodeFailure：旧码正确、新码错误 → 返回 error，
// 旧号的码未被消费（consumed_at == 0），用户手机号不变。这条钉住「同一事务」。
func TestBindUserPhoneRollsBackOnNewCodeFailure(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("bind_old", "bind_new"))
	oldCountry, oldLocal := smsTestPhone(t, w)
	newCountry, newLocal := smsTestPhone(t, w)
	userId := s11CreateUser(t, w, oldCountry, oldLocal)

	oldCode, oldId := s11SendCode(t, w, oldCountry, oldLocal, sms.SceneBindOld)
	newPlain, _ := s11SendCode(t, w, newCountry, newLocal, sms.SceneBindNew)
	wrongNew := s9WrongCode(newPlain)

	if err := w.BindUserPhone(userId, oldCode, newCountry, newLocal, wrongNew); err == nil {
		t.Fatal("新号码错码应失败，得到 nil")
	}
	// 旧码在外层事务内已被消费，若嵌套调用另行提交，外层回滚撤不掉 → 此处会 > 0。
	if got := s9ReadConsumedAt(t, w, oldId); got != 0 {
		t.Errorf("整体回滚后旧号码 consumed_at 应为 0，实际 %d（内层已提交则外层回滚失效）", got)
	} else {
		t.Log("[RollsBack] 旧码 consumed_at = 0（外层回滚撤销了内层写回）")
	}
	country, phone := s11ReadUserPhone(t, w, userId)
	if country != oldCountry || phone != oldLocal {
		t.Errorf("失败后用户手机号应不变 = (%s, %s)，实际 (%s, %s)", oldCountry, oldLocal, country, phone)
	} else {
		t.Logf("[RollsBack] 失败后号码保持 (%s, %s)", country, phone)
	}
}

// TestBindUserPhoneFirstBindSkipsOld：用户 phone == "" → 只验新号即可绑定成功
// （唯一例外，必须显式覆盖）。这里连旧码都不发、oldCode 传空串。
func TestBindUserPhoneFirstBindSkipsOld(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("bind_new")) // 白名单只开 bind_new，证明旧码分支不进入
	newCountry, newLocal := smsTestPhone(t, w)
	userId := s11CreateUser(t, w, "86", "") // phone == ""：首绑

	newCode, newId := s11SendCode(t, w, newCountry, newLocal, sms.SceneBindNew)
	if err := w.BindUserPhone(userId, "", newCountry, newLocal, newCode); err != nil {
		t.Fatalf("首绑（phone==\"\"）只验新号应成功，得到 %v", err)
	}
	country, phone := s11ReadUserPhone(t, w, userId)
	if country != newCountry || phone != newLocal {
		t.Errorf("首绑后号码 = (%s, %s)，期望 (%s, %s)", country, phone, newCountry, newLocal)
	} else {
		t.Logf("[FirstBind] 首绑后号码 = (%s, %s)，旧码分支已跳过", country, phone)
	}
	if got := s9ReadConsumedAt(t, w, newId); got <= 0 {
		t.Errorf("新号码 consumed_at 应 > 0，实际 %d", got)
	}
}

// TestBindUserPhoneRejectsDuplicate：新号已被别的用户占用 → 失败；
// 新号就是自己当前号 → 允许（排除自己）。三个子场景号码互不相同，避开
// 回滚后仍活跃码的重发间隔（smsSend.go 第 7b 步按 (phone, scene) 判定）。
func TestBindUserPhoneRejectsDuplicate(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("bind_old", "bind_new"))

	// ① 新号被别的用户占用 → 失败，且失败整体回滚、用户号码不变。
	old1C, old1L := smsTestPhone(t, w)
	occC, occL := smsTestPhone(t, w)
	user1 := s11CreateUser(t, w, old1C, old1L)
	_ = s11CreateUser(t, w, occC, occL) // 别的已占用 occL
	oldCode1, _ := s11SendCode(t, w, old1C, old1L, sms.SceneBindOld)
	newCode1, _ := s11SendCode(t, w, occC, occL, sms.SceneBindNew)
	if err := w.BindUserPhone(user1, oldCode1, occC, occL, newCode1); err == nil {
		t.Error("新号已被别的用户占用，换绑应失败，得到 nil")
	}
	c1, p1 := s11ReadUserPhone(t, w, user1)
	if c1 != old1C || p1 != old1L {
		t.Errorf("查重失败后 user1 号码应保持 (%s,%s)，实际 (%s,%s)", old1C, old1L, c1, p1)
	}

	// ② 新号就是自己当前号 → 允许（排除自己），走完两个码的校验后更新成同值。
	// 用全新号码，避免与 ① 回滚后仍活跃的码撞上重发间隔。
	selfC, selfL := smsTestPhone(t, w)
	user2 := s11CreateUser(t, w, selfC, selfL)
	oldCode2, _ := s11SendCode(t, w, selfC, selfL, sms.SceneBindOld)
	newCode2, newId2 := s11SendCode(t, w, selfC, selfL, sms.SceneBindNew)
	if err := w.BindUserPhone(user2, oldCode2, selfC, selfL, newCode2); err != nil {
		t.Fatalf("新号等于自己当前号应允许（排除自己），得到 %v", err)
	}
	c2, p2 := s11ReadUserPhone(t, w, user2)
	if c2 != selfC || p2 != selfL {
		t.Errorf("绑回原号后号码应保持 (%s,%s)，实际 (%s,%s)", selfC, selfL, c2, p2)
	} else {
		t.Logf("[RejectsDuplicate] 绑回原号成功，号码保持 (%s,%s)", c2, p2)
	}
	if got := s9ReadConsumedAt(t, w, newId2); got <= 0 {
		t.Errorf("绑回原号的新码 consumed_at 应 > 0，实际 %d", got)
	}
}

// TestRegisterRejectsDuplicateWithValidatedCountry（R90 ④ 的 provider 半边）：
// 证明「伪造区号」在 §6.2 六道闸门的 ITU 成员校验处被拒，且 guard 用「已校验区号」
// 查重拒绝同 (区号,号码) 重复注册；不写死总数，按号码精确计数。
func TestRegisterRejectsDuplicateWithValidatedCountry(t *testing.T) {
	w := setupSmsTest(t)
	country, local := smsTestPhone(t, w)
	// 模拟 ApiRegister 的 R90 ①：先用 sms.NormalizePhone 校验，伪造区号（非 ITU 成员）必须被拒。
	p, err := sms.NormalizePhone(country, local)
	if err != nil {
		t.Fatalf("合法测试号归一化失败：%v", err)
	}
	if _, err := sms.NormalizePhone("999", local); err == nil {
		t.Error("伪造区号 999 应被 sms.NormalizePhone 的 ITU 成员闸门拒绝")
	} else {
		t.Log("[R90④] 伪造区号 999 被 §6.2 六道闸门拒绝")
	}
	mkReq := func(name string) *request.ApiRegisterRequest {
		return &request.ApiRegisterRequest{
			UserName:        name,
			Password:        "s11pass123456",
			Phone:           p.Local,
			PhoneCountry:    p.Country,
			PhoneNormalized: true, // 仅由已校验路径置 true，前台 json:"-" 无法绑定
		}
	}
	first, err := w.RegisterUser(mkReq("s11_dup_a_" + local))
	if err != nil {
		t.Fatalf("首次注册（已校验区号）失败：%v", err)
	}
	s11TrackUserDeletion(t, w, first.Id)
	if _, err := w.RegisterUser(mkReq("s11_dup_b_" + local)); err == nil {
		t.Error("同区号同号重复注册应被 guard 拒绝（R90 ④ / §4.4）")
	} else {
		t.Logf("[R90④] guard 用已校验区号查重，拒绝重复注册：%v", err)
	}
	var n int64
	if err := w.DB.Model(&model.User{}).
		Where("phone_country = ? AND phone = ?", p.Country, p.Local).
		Count(&n).Error; err != nil {
		t.Fatalf("按号码计数失败：%v", err)
	}
	if n != 1 {
		t.Errorf("该 (区号,号码) 应恰好 1 行，实际 %d 行", n)
	} else {
		t.Logf("[R90④] 该 (区号,号码) 库中恰好 %d 行，无重复行", n)
	}
}
