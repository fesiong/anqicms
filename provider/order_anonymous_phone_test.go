package provider

import (
	"fmt"
	"testing"
	"time"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// TestOrderAnonymousPhoneNormalized 钉住匿名下单的手机号处理：
//
// 旧实现把地址里的原始字符串直接写进 user.Phone，且从不写 PhoneCountry：
//   - 带分隔符的号码与库里已归一化的号码查不上 → 建出重复账号；
//   - 新号码建号后 phone_country=” → 该账号既不能短信登录，也进不了换绑流程。
//
// 新实现先用 sms.NormalizePhone 得到 (country, local)，查重与建号都用这对值；
// 非法号码不拒绝整单，但也不落库（Phone 留空，区号取默认值，避免脏行）。
//
// 反向对照（人工）：去掉归一化、改回 `user.Phone = req.Address.Phone` 后，
// ① 必须变红（会新建重复账号）。
func TestOrderAnonymousPhoneNormalized(t *testing.T) {
	w := newZeroValueTestSite(t)

	oldNeedLogin, oldNoProcess := w.PluginOrder.NoNeedLogin, w.PluginOrder.NoProcess
	w.PluginOrder.NoNeedLogin = true
	w.PluginOrder.NoProcess = false
	t.Cleanup(func() {
		w.PluginOrder.NoNeedLogin = oldNeedLogin
		w.PluginOrder.NoProcess = oldNoProcess
	})

	newAddress := func(phone string) *request.OrderAddressRequest {
		return &request.OrderAddressRequest{
			Name:        "测试收货人",
			Phone:       phone,
			AddressInfo: "测试路 1 号",
		}
	}
	newReq := func(phone string) *request.OrderRequest {
		goods := newQuantityTestGoods(t, w, 100)
		return &request.OrderRequest{
			Type:    config.OrderTypeGoods,
			Address: newAddress(phone),
			Details: []request.OrderDetail{{GoodsId: goods.Id, Quantity: 1, QuantityProvided: true}},
		}
	}

	t.Run("① 已有86用户用带分隔符号码下单不新增用户且订单挂老用户", func(t *testing.T) {
		local := fmt.Sprintf("139%08d", time.Now().UnixNano()%100000000)
		existing := &model.User{
			UserName:     "anon_owner_" + local,
			Phone:        local,
			PhoneCountry: "86",
		}
		if err := w.DB.Create(existing).Error; err != nil {
			t.Fatalf("造已有用户失败：%v", err)
		}
		t.Cleanup(func() {
			w.DB.Unscoped().Where("user_id = ?", existing.Id).Delete(&model.OrderAddress{})
			w.DB.Unscoped().Delete(&model.User{}, existing.Id)
		})
		cleanupTestOrders(t, w, existing.Id)

		// 地址里的号码带分隔符，模拟前端/用户原样填写；归一化后应命中老用户。
		formatted := local[:3] + "-" + local[3:7] + "-" + local[7:]
		order, err := w.CreateOrder(0, newReq(formatted))
		if err != nil {
			t.Fatalf("匿名下单失败：%v", err)
		}
		if order.UserId != existing.Id {
			t.Fatalf("订单应挂到已有用户 %d，实际 %d（说明新建了重复账号）", existing.Id, order.UserId)
		}
		var count int64
		if err := w.DB.Model(&model.User{}).Where("phone IN ?", []string{local, formatted}).Count(&count).Error; err != nil {
			t.Fatalf("统计用户失败：%v", err)
		}
		if count != 1 {
			t.Fatalf("该号码应只有 1 个用户，实际 %d 个（重复账号）", count)
		}
	})

	t.Run("② 新号码建号后phone_country非空", func(t *testing.T) {
		local := fmt.Sprintf("137%08d", time.Now().UnixNano()%100000000)
		order, err := w.CreateOrder(0, newReq(local))
		if err != nil {
			t.Fatalf("匿名下单失败：%v", err)
		}
		if order.UserId == 0 {
			t.Fatal("匿名下单应建出用户，实际 UserId=0")
		}
		created := order.UserId
		t.Cleanup(func() {
			w.DB.Unscoped().Where("user_id = ?", created).Delete(&model.OrderAddress{})
			w.DB.Unscoped().Delete(&model.User{}, created)
		})
		cleanupTestOrders(t, w, created)

		var got model.User
		if err := w.DB.Where("id = ?", created).Take(&got).Error; err != nil {
			t.Fatalf("读回新用户失败：%v", err)
		}
		if got.PhoneCountry == "" {
			t.Fatalf("新用户 phone_country 不能为空（否则无法短信登录）：%+v", got)
		}
		if got.Phone != local {
			t.Fatalf("新用户 phone 应为归一化后的 %q，实际 %q", local, got.Phone)
		}
	})

	t.Run("③ 非法号码不panic且不产生phone_country为空的脏行", func(t *testing.T) {
		order, err := w.CreateOrder(0, newReq("not-a-phone"))
		if err != nil {
			t.Fatalf("非法号码不应拒绝整单，实际报错：%v", err)
		}
		if order.UserId == 0 {
			t.Fatal("匿名下单应建出用户，实际 UserId=0")
		}
		created := order.UserId
		t.Cleanup(func() {
			w.DB.Unscoped().Where("user_id = ?", created).Delete(&model.OrderAddress{})
			w.DB.Unscoped().Delete(&model.User{}, created)
		})
		cleanupTestOrders(t, w, created)

		var got model.User
		if err := w.DB.Where("id = ?", created).Take(&got).Error; err != nil {
			t.Fatalf("读回新用户失败：%v", err)
		}
		if got.Phone != "" {
			t.Fatalf("非法号码不得写进 users.phone，实际 %q", got.Phone)
		}
		if got.PhoneCountry == "" {
			t.Fatalf("users.phone_country 不得为空（脏行），实际空串")
		}
		var dirty int64
		if err := w.DB.Model(&model.User{}).Where("phone = ?", "not-a-phone").Count(&dirty).Error; err != nil {
			t.Fatalf("统计非法号码失败：%v", err)
		}
		if dirty != 0 {
			t.Fatalf("users 表出现了非法号码 %d 行", dirty)
		}
	})
}
