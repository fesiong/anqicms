package provider

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider/sms"
)

// BindUserPhone 手机号换绑（规格 §7.1 决策 12 / 裁决 R92）：在单个事务内消费
// 旧号、新号两个验证码并更新 phone／phone_country，任一步失败整体回滚。
//
// 顺序（R92，不可换）：
//  1. 事务外 sms.NormalizePhone(newCountry, newPhone) 校验新号（§6.2 六道闸门，
//     含 ITU 成员），失败原样返回、不进事务。
//  2. 事务内 tx + FOR UPDATE 按 id 取用户，不存在返回业务错误。
//  3. user.Phone != "" 时用「库内」(PhoneCountry, Phone) 归一化旧号并
//     verifySmsCodeTx(..., SceneBindOld, oldCode)；phone == "" 首绑跳过旧码
//     （唯一例外）。不得信任请求体自证的旧号。
//  4. verifySmsCodeTx(..., SceneBindNew, newCode)。
//  5. 唯一性查重沿用 createUserWithPhoneGuard 口径：WHERE phone = ? 单列 +
//     应用层比区号 + 排除自己 + FOR UPDATE（双列 WHERE 会退化成锁全表）。
//  6. tx ... Updates 更新 phone／phone_country，接 .Error 并检查 RowsAffected
//     （硬教训 10：漏检会让并发下的静默失败变成「接口 code:0」）。
//
// 嵌套事务（R91）：verifySmsCodeTx 复用同一个 tx，且 biz／db 任一非 nil 都
// return err 让 GORM 整体回滚 —— 外层回滚才能撤销内层已写回的 consumed_at，
// 保证两个码的原子性。新号等于自己当前 (country, phone) 时允许（排除自己后
// 走完两个码的校验，再更新成同值）。
func (w *Website) BindUserPhone(userId uint, oldCode, newCountry, newPhone, newCode string) error {
	// 事务外归一化新号：含 ITU 成员校验，失败不进事务（R92）。
	newP, err := sms.NormalizePhone(newCountry, newPhone)
	if err != nil {
		return err
	}

	return w.DB.Transaction(func(tx *gorm.DB) error {
		// ① 用 tx + FOR UPDATE 按 id 取用户（不信任请求体自证）。
		var user model.User
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userId).First(&user).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return sms.ErrSmsSceneNotEnabled // 登录态下用户不存在：归约，不细分
			}
			return e
		}

		// ② 旧号：phone != "" 才验；用库内 (PhoneCountry, Phone) 归一化。
		if user.Phone != "" {
			oldP, e := sms.NormalizePhone(user.PhoneCountry, user.Phone)
			if e != nil {
				return e
			}
			biz, db := w.verifySmsCodeTx(tx, oldP, sms.SceneBindOld, oldCode)
			if db != nil {
				return db
			}
			if biz != nil {
				return biz // 任一非 nil 都整体回滚（R91）
			}
		}

		// ③ 新号。
		biz, db := w.verifySmsCodeTx(tx, newP, sms.SceneBindNew, newCode)
		if db != nil {
			return db
		}
		if biz != nil {
			return biz
		}

		// ④ 唯一性查重：单列 WHERE + 应用层比区号 + 排除自己 + FOR UPDATE。
		var existing []model.User
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("phone = ?", newP.Local).Find(&existing).Error; e != nil {
			return e
		}
		for _, u := range existing {
			if u.Id == userId {
				continue // 排除自己：允许「绑回原号」
			}
			if normalizePhoneCountry(u.PhoneCountry) == newP.Country {
				return errors.New("TheMobilePhoneNumberHasBeenRegistered")
			}
		}

		// ⑤ 更新，接 .Error 并检查 RowsAffected（硬教训 10）。
		res := tx.Model(&model.User{}).Where("id = ?", userId).
			Updates(map[string]interface{}{
				"phone":         newP.Local,
				"phone_country": newP.Country,
			})
		if res.Error != nil {
			return res.Error
		}
		// 开发库 DSN 未开 clientFoundRows（changed-rows 语义）：把号码绑回当前
		// 同值时 UPDATE 写入相同值、RowsAffected = 0，这是「新号 == 当前号允许」
		// 的合法场景（与 provider/channelManager.go extendLease 同款处理）。
		// 故 0 行时在同一 tx 内回读确认目标值已生效；>1 行在主键 WHERE 下不可能。
		if res.RowsAffected > 1 {
			return sms.ErrSmsSceneNotEnabled
		}
		if res.RowsAffected == 0 {
			var current model.User
			if e := tx.Where("id = ?", userId).First(&current).Error; e != nil {
				return e
			}
			if current.Phone != newP.Local || current.PhoneCountry != newP.Country {
				return sms.ErrSmsSceneNotEnabled
			}
		}
		return nil
	})
}
