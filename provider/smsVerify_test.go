package provider

import (
	"errors"
	"sync"
	"testing"
	"time"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/sms"
)

// 编译期确认 VerifySmsCode 存在；步骤 2（第一轮 RED）里该方法尚未定义，
// 这里让编译失败落到 "VerifySmsCode undefined"（与任务 8 的 SendSmsCode 同款护栏）。
var _ = (*Website).VerifySmsCode

// s9SendCode 用任务 8 的测试接缝发一枚真库验证码，返回 (country, local)、明文码、
// sms_codes 行 id。fakeSmsSender 全程不触网、不发真实短信（R62）。
func s9SendCode(t *testing.T, w *Website, scene string) (country, local, plain string, codeId uint) {
	t.Helper()
	country, local = smsTestPhone(t, w)
	fake := &fakeSmsSender{res: sms.SendResult{BizId: "B-s9"}}
	if _, err := w.sendSmsCodeWithSender(SmsSendInput{
		PhoneCountry: country, Phone: local, Scene: scene, Ip: s8TestIp,
	}, fake); err != nil {
		t.Fatalf("发码失败：%v", err)
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
	return country, local, fake.lastCode, row.Id
}

// s9ReadAttempts 按 id 精确读回 attempts（R75：手工读写一律按 id）。
func s9ReadAttempts(t *testing.T, w *Website, codeId uint) int {
	t.Helper()
	var attempts int
	if err := w.DB.Raw("SELECT `attempts` FROM `sms_codes` WHERE `id` = ?", codeId).Row().Scan(&attempts); err != nil {
		t.Fatalf("读回 attempts 失败：%v", err)
	}
	return attempts
}

// s9ReadConsumedAt 按 id 精确读回 consumed_at。
func s9ReadConsumedAt(t *testing.T, w *Website, codeId uint) int64 {
	t.Helper()
	var consumed int64
	if err := w.DB.Raw("SELECT `consumed_at` FROM `sms_codes` WHERE `id` = ?", codeId).Row().Scan(&consumed); err != nil {
		t.Fatalf("读回 consumed_at 失败：%v", err)
	}
	return consumed
}

// s9WrongCode 构造与明文码等长、每位 +1 mod 10 的纯数字错码，保证 != 明文码。
func s9WrongCode(plain string) string {
	b := []byte(plain)
	for i := range b {
		b[i] = '0' + (b[i]-'0'+1)%10
	}
	return string(b)
}

// s9RunConcurrent 用起跑闸门让 n 个 goroutine 同时发起校验，共享同一个 w / w.DB、
// 同一号码、同一码（R74）。results[i] 各自写独立下标，无数据竞态。
func s9RunConcurrent(t *testing.T, w *Website, country, local, scene, code string, n int) []error {
	t.Helper()
	start := make(chan struct{})
	results := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i] = w.VerifySmsCode(country, local, scene, code)
		}(i)
	}
	close(start)
	wg.Wait()
	return results
}

// TestSmsVerifyHappyPath：正确码 → err == nil，库里 consumed_at > 0。
func TestSmsVerifyHappyPath(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	country, local, plain, codeId := s9SendCode(t, w, "login")

	if err := w.VerifySmsCode(country, local, "login", plain); err != nil {
		t.Fatalf("正确码应校验成功（err == nil），得到 %v", err)
	}
	if got := s9ReadConsumedAt(t, w, codeId); got <= 0 {
		t.Errorf("成功消费后 consumed_at 应 > 0，实际 %d", got)
	} else {
		t.Logf("[HappyPath] 正确码校验成功，consumed_at = %d（> 0）", got)
	}
}

// TestSmsVerifyRejectsWrongCode：错码 → ErrSmsCodeMismatch + *sms.AttemptsError
// (Remaining == 行上 MaxAttempts-1)；重开一次独立连接再查库 attempts == 1。
func TestSmsVerifyRejectsWrongCode(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	country, local, plain, codeId := s9SendCode(t, w, "login")
	wrong := s9WrongCode(plain)

	var maxA int
	if err := w.DB.Raw("SELECT `max_attempts` FROM `sms_codes` WHERE `id` = ?", codeId).Row().Scan(&maxA); err != nil {
		t.Fatalf("读回 max_attempts 失败：%v", err)
	}

	err := w.VerifySmsCode(country, local, "login", wrong)
	if !errors.Is(err, sms.ErrSmsCodeMismatch) {
		t.Fatalf("错码应落 ErrSmsCodeMismatch，得到 %v", err)
	}
	var ae *sms.AttemptsError
	if !errors.As(err, &ae) {
		t.Fatalf("应能 errors.As 取到 *sms.AttemptsError，得到 %T", err)
	}
	if want := maxA - 1; ae.Remaining != want {
		t.Errorf("Remaining = %d，期望 %d（行上 max_attempts=%d）", ae.Remaining, want, maxA)
	} else {
		t.Logf("[RejectsWrongCode] errors.Is(ErrSmsCodeMismatch)=true，AttemptsError.Remaining = %d", ae.Remaining)
	}

	// 重开一次独立连接（InitDB 新连接池）再查库：证明 attempts++ 已提交落库，
	// 不是只在内存改、也不是未提交事务在同一连接上的可见性假象。
	fresh, err := InitDB(w.Mysql)
	if err != nil {
		t.Fatalf("重开独立数据库连接失败：%v", err)
	}
	sqlDB, err := fresh.DB()
	if err != nil {
		t.Fatalf("取得新连接池底层句柄失败：%v", err)
	}
	defer sqlDB.Close()
	var attempts int
	if e := fresh.Raw("SELECT `attempts` FROM `sms_codes` WHERE `id` = ?", codeId).Row().Scan(&attempts); e != nil {
		t.Fatalf("新连接读回 attempts 失败：%v", e)
	}
	if attempts != 1 {
		t.Errorf("重开连接查库 attempts 应为 1（写回 DB，不是只在内存加），实际 %d", attempts)
	} else {
		t.Logf("[写回证据] 独立新连接 SELECT attempts = %d", attempts)
	}
}

// TestSmsVerifyLocksAfterMaxAttempts（R72）：第 1..MaxAttempts 次错码返回
// ErrSmsCodeMismatch（Remaining 依次递减到 0），第 MaxAttempts+1 次用正确码
// 才返回 ErrSmsCodeLocked（第 6 步在比对之前），并断言 attempts 停在上限。
func TestSmsVerifyLocksAfterMaxAttempts(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	country, local, plain, codeId := s9SendCode(t, w, "login")
	wrong := s9WrongCode(plain)

	var maxA int
	if err := w.DB.Raw("SELECT `max_attempts` FROM `sms_codes` WHERE `id` = ?", codeId).Row().Scan(&maxA); err != nil {
		t.Fatalf("读回 max_attempts 失败：%v", err)
	}
	t.Logf("[R72] 行上 max_attempts = %d（发码写入 snapshot.MaxVerifyAttempts，默认 5）", maxA)

	for i := 1; i <= maxA; i++ {
		err := w.VerifySmsCode(country, local, "login", wrong)
		if !errors.Is(err, sms.ErrSmsCodeMismatch) {
			t.Fatalf("第 %d 次错码应落 ErrSmsCodeMismatch，得到 %v", i, err)
		}
		var ae *sms.AttemptsError
		if !errors.As(err, &ae) {
			t.Fatalf("第 %d 次应能 errors.As 取到 *sms.AttemptsError，得到 %T", i, err)
		}
		if want := maxA - i; ae.Remaining != want {
			t.Errorf("第 %d 次 Remaining = %d，期望 %d", i, ae.Remaining, want)
		} else {
			t.Logf("[R72] 第 %d 次错码 → ErrSmsCodeMismatch，Remaining = %d", i, ae.Remaining)
		}
	}
	if got := s9ReadAttempts(t, w, codeId); got != maxA {
		t.Errorf("%d 次错码后 attempts 应为 %d，实际 %d", maxA, maxA, got)
	}
	// 第 maxA+1 次用正确码：第 6 步（Attempts >= MaxAttempts）在比对之前，仍落 Locked。
	if err := w.VerifySmsCode(country, local, "login", plain); !errors.Is(err, sms.ErrSmsCodeLocked) {
		t.Errorf("第 %d 次（正确码）应返回 ErrSmsCodeLocked，得到 %v", maxA+1, err)
	} else {
		t.Logf("[R72] 第 %d 次用正确码 → ErrSmsCodeLocked（正确码也打不开）", maxA+1)
	}
	if got := s9ReadAttempts(t, w, codeId); got != maxA {
		t.Errorf("锁定后 attempts 应停在 %d，实际 %d", maxA, got)
	}
}

// TestSmsVerifySceneIsolation：login 的码拿去校验 reset → ErrSmsCodeNotFound
// （第 2 步 WHERE 带 scene；VerifySmsCode 不做场景白名单检查）。
func TestSmsVerifySceneIsolation(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	country, local, plain, _ := s9SendCode(t, w, "login")

	if err := w.VerifySmsCode(country, local, "reset", plain); !errors.Is(err, sms.ErrSmsCodeNotFound) {
		t.Errorf("跨场景校验应落 ErrSmsCodeNotFound，得到 %v", err)
	} else {
		t.Log("[SceneIsolation] login 码验 reset → ErrSmsCodeNotFound")
	}
}

// TestSmsVerifyExpired：按 id 精确把 expires_at 改到过去 → ErrSmsCodeExpired
// （第 4 步在比对之前；用正确码也落 Expired）。
func TestSmsVerifyExpired(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	country, local, plain, codeId := s9SendCode(t, w, "login")

	past := time.Now().Add(-time.Hour).Unix()
	if err := w.DB.Exec("UPDATE `sms_codes` SET `expires_at` = ? WHERE `id` = ?", past, codeId).Error; err != nil {
		t.Fatalf("按 id 精确改 expires_at 失败：%v", err)
	}
	if err := w.VerifySmsCode(country, local, "login", plain); !errors.Is(err, sms.ErrSmsCodeExpired) {
		t.Errorf("过期码应落 ErrSmsCodeExpired，得到 %v", err)
	} else {
		t.Log("[Expired] 手工改 expires_at 到过去 → ErrSmsCodeExpired")
	}
}

// TestSmsVerifySendFailed：按 id 精确把 send_status 改成 -1 → ErrSmsSendFailed。
func TestSmsVerifySendFailed(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	country, local, plain, codeId := s9SendCode(t, w, "login")

	if err := w.DB.Exec("UPDATE `sms_codes` SET `send_status` = -1 WHERE `id` = ?", codeId).Error; err != nil {
		t.Fatalf("按 id 精确改 send_status 失败：%v", err)
	}
	if err := w.VerifySmsCode(country, local, "login", plain); !errors.Is(err, sms.ErrSmsSendFailed) {
		t.Errorf("发送失败的码应落 ErrSmsSendFailed，得到 %v", err)
	} else {
		t.Log("[SendFailed] 手工改 send_status=-1 → ErrSmsSendFailed")
	}
}

// TestSmsVerifyReplay：成功消费后再校验 → ErrSmsCodeNotFound
// （第 2 步 WHERE consumed_at=0 把已消费行过滤掉）。
func TestSmsVerifyReplay(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	country, local, plain, codeId := s9SendCode(t, w, "login")

	if err := w.VerifySmsCode(country, local, "login", plain); err != nil {
		t.Fatalf("第一次用正确码应成功：%v", err)
	}
	if got := s9ReadConsumedAt(t, w, codeId); got <= 0 {
		t.Fatalf("第一次成功后 consumed_at 应 > 0，实际 %d", got)
	}
	if err := w.VerifySmsCode(country, local, "login", plain); !errors.Is(err, sms.ErrSmsCodeNotFound) {
		t.Errorf("重放应落 ErrSmsCodeNotFound，得到 %v", err)
	} else {
		t.Log("[Replay] 成功后重放 → ErrSmsCodeNotFound（consumed_at=0 过滤）")
	}
}

// TestSmsVerifyConcurrent（审查重点 3 / R71 / R74）：三个子用例共享父用例的
// w / w.DB；每个 20-goroutine 子用例只调一次 smsTestPhone（固定同一号码、
// 同一 fakeSmsSender.lastCode），逐 goroutine 不生成新号。
func TestSmsVerifyConcurrent(t *testing.T) {
	w := setupSmsTest(t)
	s8UseConfig(t, w, s8TwilioConfig("login"))
	const n = 20

	// ① 20 并发正确码 → 恰好 1 成功，其余全 ErrSmsCodeNotFound。
	// 反向对照锚点：去掉 clause.Locking 的 FOR UPDATE 后本子用例必须变红。
	t.Run("S1_correct_code_exactly_one_winner", func(t *testing.T) {
		country, local, plain, _ := s9SendCode(t, w, "login")
		results := s9RunConcurrent(t, w, country, local, "login", plain, n)
		success, notFound := 0, 0
		for _, err := range results {
			switch {
			case err == nil:
				success++
			case errors.Is(err, sms.ErrSmsCodeNotFound):
				notFound++
			default:
				t.Errorf("并发①出现非预期错误：%v", err)
			}
		}
		if success != 1 {
			t.Errorf("并发①恰好 1 个成功，实际 %d（去掉 FOR UPDATE 后会 > 1 而变红）", success)
		}
		if notFound != n-success {
			t.Errorf("并发①其余应全为 ErrSmsCodeNotFound，实际 %d，期望 %d", notFound, n-success)
		}
		t.Logf("[并发①] 成功 = %d，ErrSmsCodeNotFound = %d（共 %d）", success, notFound, n)
	})

	// ②(a) R71：保持默认 5，20 并发错码 → attempts 停在 5（锁定闸门），成功 0；
	// 错误允许 ErrSmsCodeMismatch / ErrSmsCodeLocked 两种，不要求精确分布。
	t.Run("S2a_default5_attempts_capped_at_5", func(t *testing.T) {
		country, local, plain, codeId := s9SendCode(t, w, "login")
		wrong := s9WrongCode(plain)
		results := s9RunConcurrent(t, w, country, local, "login", wrong, n)
		success := 0
		for _, err := range results {
			if err == nil {
				success++
				continue
			}
			if !errors.Is(err, sms.ErrSmsCodeMismatch) && !errors.Is(err, sms.ErrSmsCodeLocked) {
				t.Errorf("并发②(a) 出现非 Mismatch/Locked 的错误：%v", err)
			}
		}
		if success != 0 {
			t.Errorf("并发②(a) 成功次数应为 0，实际 %d", success)
		}
		got := s9ReadAttempts(t, w, codeId)
		if got != 5 {
			t.Errorf("并发②(a) attempts 应停在 5（锁定闸门 + 无丢失更新），实际 %d", got)
		}
		t.Logf("[R71(a)] 默认 MaxAttempts=5，20 并发错码 → attempts=%d，成功=%d", got, success)
	})

	// ②(b) R71：测试内 SQL 按 id 精确把 max_attempts 改成 25，20 并发错码 →
	// attempts == 20，成功 0，纯测「每次失败都落库、不被并发覆盖」。
	t.Run("S2b_max25_attempts_all_persisted", func(t *testing.T) {
		country, local, plain, codeId := s9SendCode(t, w, "login")
		if err := w.DB.Exec("UPDATE `sms_codes` SET `max_attempts` = 25 WHERE `id` = ?", codeId).Error; err != nil {
			t.Fatalf("按 id 精确改 max_attempts=25 失败：%v", err)
		}
		wrong := s9WrongCode(plain)
		results := s9RunConcurrent(t, w, country, local, "login", wrong, n)
		success := 0
		for _, err := range results {
			if err == nil {
				success++
			}
		}
		if success != 0 {
			t.Errorf("并发②(b) 成功次数应为 0，实际 %d", success)
		}
		got := s9ReadAttempts(t, w, codeId)
		if got != n {
			t.Errorf("并发②(b) 每次失败都应落库，attempts 期望 %d，实际 %d", n, got)
		}
		t.Logf("[R71(b)] max_attempts 放大到 25，20 并发错码 → attempts=%d，成功=%d（纯测每次失败都落库）", got, success)
	})
}
