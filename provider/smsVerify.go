package provider

import (
	"crypto/subtle"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/sms"
)

// VerifySmsCode 校验短信验证码（规格 §6.4 八步，判定顺序不可换）。
//
// 整段包在 w.DB.Transaction 里，取行用 clause.Locking{Strength: "UPDATE"} 加行锁，
// 全程用 tx 而非 w.DB，绝不绕过事务。一次性由 WHERE consumed_at = 0 保证：消费过的
// 行查不出来，重放落 ErrSmsCodeNotFound；行锁保证同一码不会被两个并发请求同时消费。
//
// 事务提交语义：§6.4 第 7 步「错码 → Attempts++ 写回 → ErrSmsCodeMismatch」要求
// attempts++ 落库，即使本函数要向调用方返回 error。若把该 error 作为 Transaction
// 回调的返回值，GORM 会回滚事务、attempts++ 丢失。故所有业务结果统一用外部变量
// result 带出，回调对业务结果一律 return nil 让事务提交；只有真正的 DB 错误才
// return err 触发回滚。第 8 步 ConsumedAt 与第 7 步 Attempts++ 都在同一事务内写回。
func (w *Website) VerifySmsCode(country, phone, scene, code string) error {
	// 1. 号码归一化（失败原样返回错误；发码侧落 ErrSmsPhoneInvalid）。
	p, err := sms.NormalizePhone(country, phone)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	var result error

	err = w.DB.Transaction(func(tx *gorm.DB) error {
		// 2. 事务 + 行锁取最新可用行：
		//   WHERE site_id=? AND phone_country=? AND phone=? AND scene=? AND consumed_at=0
		//   ORDER BY id DESC LIMIT 1 FOR UPDATE
		// GORM First() 会自带 ORDER BY <primaryKey> ASC；显式 .Order("id DESC").Limit(1)
		// 让 id DESC 成为主排序键（追加的升序只是同 id 无意义的次级键，不会覆盖 DESC），
		// 保证取到最新一行而不是最老一行。
		var row model.SmsCode
		res := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("site_id = ? AND phone_country = ? AND phone = ? AND scene = ? AND consumed_at = 0",
				w.Id, p.Country, p.Local, scene).
			Order("id DESC").Limit(1).
			First(&row)

		// 3. 无行 → ErrSmsCodeNotFound。
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			result = sms.ErrSmsCodeNotFound
			return nil
		}
		if res.Error != nil {
			return res.Error
		}

		// 4. ExpiresAt < now → ErrSmsCodeExpired。
		if row.ExpiresAt < now {
			result = sms.ErrSmsCodeExpired
			return nil
		}

		// 5. SendStatus != 1 → ErrSmsSendFailed。
		if row.SendStatus != smsSuccess {
			result = sms.ErrSmsSendFailed
			return nil
		}

		// 6. Attempts >= MaxAttempts → ErrSmsCodeLocked。
		// MaxAttempts 取行上的 row.MaxAttempts（发码时 snapshot.MaxVerifyAttempts 已写入
		// 该列），不读配置（规格 §6.4）。本判定在比对之前，锁定后不再 ++。
		if row.Attempts >= row.MaxAttempts {
			result = sms.ErrSmsCodeLocked
			return nil
		}

		// 7. HMAC 比对：不符 → Attempts++ 写回数据库 → ErrSmsCodeMismatch（带剩余次数）。
		if subtle.ConstantTimeCompare([]byte(w.smsCodeHash(code)), []byte(row.CodeHash)) != 1 {
			row.Attempts++
			if e := tx.Model(&model.SmsCode{}).Where("id = ?", row.Id).
				Update("attempts", row.Attempts).Error; e != nil {
				return e
			}
			remaining := row.MaxAttempts - row.Attempts
			if remaining < 0 {
				remaining = 0
			}
			// 业务错误带出事务；回调 return nil 让 attempts++ 提交落库。
			result = &sms.AttemptsError{Remaining: remaining}
			return nil
		}

		// 8. 符合 → ConsumedAt = now 写回数据库 → 成功。
		if e := tx.Model(&model.SmsCode{}).Where("id = ?", row.Id).
			Update("consumed_at", now).Error; e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		return err
	}
	return result
}
