package provider

import (
	"strings"
	"testing"

	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// TestUserUpdateInfoRejectsPhoneChange 钉住「编辑资料不能改手机号」：
// 手机号在短信分支已是登录身份，(phone_country, phone) 相同即同一账号。
// 若允许 POST /api/user/detail 直写，登录用户可以把手机号改成他人尚未注册的
// 号码——号码真实主人日后短信登录会进入攻击者账号，且该号码注册被永久占用。
//
// 四条子用例覆盖规格允许/拒绝的全部组合：
//
//	① 改成未注册号码 → 报错且库内值未变；
//	② 改成已注册号码 → 报错；
//	③ 原样回传（值不变）→ 成功；
//	④ 传空串解绑 → 成功。
//
// 反向对照（人工）：注释掉 UpdateUserInfo 里的拒绝分支后重跑本用例，① 必须变红。
func TestUserUpdateInfoRejectsPhoneChange(t *testing.T) {
	w := newZeroValueTestSite(t)

	ownerPhone := smsUserRandomPhone()
	otherPhone := smsUserRandomPhone()
	owner := &model.User{
		UserName:     "phone_change_owner_" + ownerPhone,
		Phone:        ownerPhone,
		PhoneCountry: "86",
	}
	other := &model.User{
		UserName:     "phone_change_other_" + otherPhone,
		Phone:        otherPhone,
		PhoneCountry: "86",
	}
	for _, u := range []*model.User{owner, other} {
		if err := w.DB.Create(u).Error; err != nil {
			t.Fatalf("造测试用户失败：%v", err)
		}
		id := u.Id
		t.Cleanup(func() { w.DB.Unscoped().Delete(&model.User{}, id) })
	}

	reloadOwner := func() *model.User {
		var got model.User
		if err := w.DB.Where("id = ?", owner.Id).Take(&got).Error; err != nil {
			t.Fatalf("读回 owner 失败：%v", err)
		}
		return &got
	}

	t.Run("① 改成未注册号码必须报错且库内值不变", func(t *testing.T) {
		target := smsUserRandomPhone()
		err := w.UpdateUserInfo(owner.Id, &request.UserRequest{Phone: &target})
		if err == nil {
			t.Fatal("改成未注册号码应报错，实际成功（账号接管风险）")
		}
		if got := reloadOwner(); got.Phone != ownerPhone {
			t.Fatalf("库内手机号被改写：期望 %q，实际 %q", ownerPhone, got.Phone)
		}
	})

	t.Run("② 改成已注册号码必须报错", func(t *testing.T) {
		target := otherPhone
		err := w.UpdateUserInfo(owner.Id, &request.UserRequest{Phone: &target})
		if err == nil {
			t.Fatal("改成已注册号码应报错，实际成功")
		}
		if !strings.Contains(err.Error(), "SaveFailed") && !strings.Contains(err.Error(), "保存失败") {
			// 语言键语义借用 SaveFailed（无合适既有键），只断言确为错误，不锁译文。
			t.Logf("拒绝错误文案 = %q", err.Error())
		}
		if got := reloadOwner(); got.Phone != ownerPhone {
			t.Fatalf("库内手机号被改写：期望 %q，实际 %q", ownerPhone, got.Phone)
		}
	})

	t.Run("③ 原样回传成功", func(t *testing.T) {
		samePhone := ownerPhone
		sameCountry := "86"
		if err := w.UpdateUserInfo(owner.Id, &request.UserRequest{
			Phone:        &samePhone,
			PhoneCountry: &sameCountry,
		}); err != nil {
			t.Fatalf("原样回传应成功，实际报错：%v", err)
		}
		if got := reloadOwner(); got.Phone != ownerPhone || got.PhoneCountry != "86" {
			t.Fatalf("原样回传后库内值被改动：phone=%q country=%q", got.Phone, got.PhoneCountry)
		}
	})

	t.Run("④ 传空串解绑成功", func(t *testing.T) {
		empty := ""
		if err := w.UpdateUserInfo(owner.Id, &request.UserRequest{Phone: &empty}); err != nil {
			t.Fatalf("解绑应成功，实际报错：%v", err)
		}
		if got := reloadOwner(); got.Phone != "" {
			t.Fatalf("解绑后手机号应为空，实际 %q", got.Phone)
		}
	})
}
