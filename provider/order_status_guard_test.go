package provider

import (
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// v3.8.4 客户复测（隔离库）暴露的 5 类订单状态问题，这里逐条钉住：
//  1. 已取消订单仍能生成 PayPal 付款链接（controller 层，见 TestCreateOrderPayment… 说明）
//  2. 已付款/已发货订单可以直接取消 → SetOrderCanceled 只会从「待支付」出发
//  3. 未付款/已取消订单可以标为已完成 → SetOrderFinished 只接受已支付/已发货
//  4. 未付款/已取消/已退款订单可以申请退款 → ApplyOrderRefund 只接受已收到钱的订单
//  5. 实物订单接受缺失或无效的收货地址 → CreateOrder 在扣库存前校验地址

func newStatusGuardOrder(t *testing.T, w *Website, status int) *model.Order {
	t.Helper()
	orderId := testOrderId("sg")
	order := &model.Order{
		OrderId: orderId,
		Amount:  100,
		Status:  status,
		// UserId 留 0：SetOrderFinished 收尾会给用户发邮件/建事件，这里不需要
	}
	if err := w.DB.Create(order).Error; err != nil {
		t.Fatalf("造订单失败: %v", err)
	}
	t.Cleanup(func() {
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.OrderRefund{})
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Order{})
	})
	return order
}

// TestSetOrderCanceledOnlyFromWaiting 已付款/已发货订单不能靠「取消」收场。
//
// 客户实测：已付款订单调用 /api/order/cancel 返回成功，订单变已取消、库存 96→97，
// 但钱没有退。取消只回补库存，所以它只能从「待支付」出发。
func TestSetOrderCanceledOnlyFromWaiting(t *testing.T) {
	w := newZeroValueTestSite(t)
	for _, status := range []int{config.OrderStatusPaid, config.OrderStatusDelivering} {
		order := newStatusGuardOrder(t, w, status)
		if err := w.SetOrderCanceled(order); err == nil {
			t.Fatalf("status=%d 的订单不该允许取消", status)
		}
		var saved model.Order
		w.DB.Where("`id` = ?", order.Id).Take(&saved)
		if saved.Status != status {
			t.Fatalf("status=%d 的订单被改成了 %d", status, saved.Status)
		}
	}
}

// TestSetOrderFinishedRequiresPaidOrDelivering 未付款/已取消订单不能标为已完成。
//
// 完成会给卖家记佣金、加余额，允许未付款订单完成等于凭空造钱。
func TestSetOrderFinishedRequiresPaidOrDelivering(t *testing.T) {
	w := newZeroValueTestSite(t)

	for _, status := range []int{config.OrderStatusWaiting, config.OrderStatusCanceled, config.OrderStatusRefunded} {
		order := newStatusGuardOrder(t, w, status)
		if err := w.SetOrderFinished(order); err == nil {
			t.Fatalf("status=%d 的订单不该允许完成", status)
		}
		var saved model.Order
		w.DB.Where("`id` = ?", order.Id).Take(&saved)
		if saved.Status != status {
			t.Fatalf("status=%d 的订单被改成了 %d", status, saved.Status)
		}
	}

	// 已付款可以完成
	order := newStatusGuardOrder(t, w, config.OrderStatusPaid)
	if err := w.SetOrderFinished(order); err != nil {
		t.Fatalf("已付款订单应可完成，实际报错: %v", err)
	}
	var saved model.Order
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if saved.Status != config.OrderStatusCompleted {
		t.Fatalf("订单应为已完成，实际 %d", saved.Status)
	}
	finishedTime := saved.FinishedTime

	// 重复「完成」必须幂等：不能再走一遍分钱，也不能刷新完成时间
	if err := w.SetOrderFinished(&model.Order{Id: order.Id, OrderId: order.OrderId, Status: config.OrderStatusPaid}); err != nil {
		t.Fatalf("重复完成应幂等成功: %v", err)
	}
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if saved.FinishedTime != finishedTime {
		t.Fatalf("重复完成不能改写完成时间（%d → %d）", finishedTime, saved.FinishedTime)
	}
}

// TestApplyOrderRefundRequiresPaidOrder 没收到钱的订单不能申请退款。
func TestApplyOrderRefundRequiresPaidOrder(t *testing.T) {
	w := newZeroValueTestSite(t)

	for _, status := range []int{config.OrderStatusWaiting, config.OrderStatusCanceled, config.OrderStatusRefunded} {
		order := newStatusGuardOrder(t, w, status)
		if err := w.ApplyOrderRefund(order); err == nil {
			t.Fatalf("status=%d 的订单不该允许申请退款", status)
		}
		var count int64
		w.DB.Model(&model.OrderRefund{}).Where("`order_id` = ?", order.OrderId).Count(&count)
		if count != 0 {
			t.Fatalf("status=%d 的订单不该落退款申请，实际 %d 条", status, count)
		}
	}

	// 已付款可以申请
	order := newStatusGuardOrder(t, w, config.OrderStatusPaid)
	if err := w.ApplyOrderRefund(order); err != nil {
		t.Fatalf("已付款订单应可申请退款，实际报错: %v", err)
	}
	var count int64
	w.DB.Model(&model.OrderRefund{}).Where("`order_id` = ?", order.OrderId).Count(&count)
	if count != 1 {
		t.Fatalf("已付款订单应落 1 条退款申请，实际 %d", count)
	}
}

// TestCreateOrderRejectsInvalidAddress 实物订单必须有可用的收货地址。
//
// 客户实测：省略地址、传 0、传不存在的 ID、传其他买家的地址，接口都会建单并扣库存；
// 内联只填姓名也能建单。
func TestCreateOrderRejectsInvalidAddress(t *testing.T) {
	w := newZeroValueTestSite(t)
	// 这家店有交易流程，订单需要发货
	w.PluginOrder.NoProcess = false
	goods := newQuantityTestGoods(t, w, 10)
	buyer := newQuantityTestBuyer(t, w)
	other := newQuantityTestBuyer(t, w)
	cleanupTestOrders(t, w, buyer.Id)

	otherAddr := &model.OrderAddress{
		UserId:      other.Id,
		Name:        "别人",
		Phone:       "13800000000",
		AddressInfo: "别人家 1 号",
		Status:      1,
	}
	if err := w.DB.Create(otherAddr).Error; err != nil {
		t.Fatalf("造他人地址失败: %v", err)
	}
	t.Cleanup(func() { w.DB.Unscoped().Delete(&model.OrderAddress{}, otherAddr.Id) })

	cases := []struct {
		name string
		req  *request.OrderRequest
	}{
		{
			name: "省略地址",
			req:  &request.OrderRequest{Details: []request.OrderDetail{{GoodsId: goods.Id, Quantity: 1, QuantityProvided: true}}},
		},
		{
			name: "address_id 传 0",
			req: &request.OrderRequest{
				Details:   []request.OrderDetail{{GoodsId: goods.Id, Quantity: 1, QuantityProvided: true}},
				AddressId: 0,
			},
		},
		{
			name: "address_id 不存在",
			req: &request.OrderRequest{
				Details:   []request.OrderDetail{{GoodsId: goods.Id, Quantity: 1, QuantityProvided: true}},
				AddressId: 99999999,
			},
		},
		{
			name: "用别人的地址",
			req: &request.OrderRequest{
				Details:   []request.OrderDetail{{GoodsId: goods.Id, Quantity: 1, QuantityProvided: true}},
				AddressId: otherAddr.Id,
			},
		},
		{
			name: "内联地址只填姓名",
			req: &request.OrderRequest{
				Details: []request.OrderDetail{{GoodsId: goods.Id, Quantity: 1, QuantityProvided: true}},
				Address: &request.OrderAddressRequest{Name: "只有名字"},
			},
		},
	}

	// 语言包随测试站点的语言走（默认英文），只断言「这是地址类错误」
	isAddressError := func(err error) bool {
		msg := strings.ToLower(err.Error())
		return strings.Contains(msg, "address") || strings.Contains(msg, "地址")
	}

	for _, c := range cases {
		if _, err := w.CreateOrder(buyer.Id, c.req); err == nil {
			t.Fatalf("%s: 必须拒绝下单", c.name)
		} else if !isAddressError(err) {
			t.Fatalf("%s: 期望地址类错误，实际: %v", c.name, err)
		}
	}

	var orderCount int64
	w.DB.Model(&model.Order{}).Where("`user_id` = ?", buyer.Id).Count(&orderCount)
	if orderCount != 0 {
		t.Fatalf("地址不合法不能留下订单，实际 %d 笔", orderCount)
	}
	var stock int64
	w.DB.Model(&model.Archive{}).Where("`id` = ?", goods.Id).Pluck("stock", &stock)
	if stock != 10 {
		t.Fatalf("地址不合法不能扣库存，实际 %d", stock)
	}

	// 完整的内联地址可以正常下单
	okReq := &request.OrderRequest{
		Details: []request.OrderDetail{{GoodsId: goods.Id, Quantity: 1, QuantityProvided: true}},
		Address: &request.OrderAddressRequest{Name: "张三", Phone: "13800000001", AddressInfo: "某路 1 号"},
	}
	if _, err := w.CreateOrder(buyer.Id, okReq); err != nil {
		t.Fatalf("完整地址应可下单，实际报错: %v", err)
	}
	w.DB.Model(&model.Order{}).Where("`user_id` = ?", buyer.Id).Count(&orderCount)
	if orderCount != 1 {
		t.Fatalf("完整地址应建出一笔订单，实际 %d", orderCount)
	}
}

// TestCreateOrderAcceptsAddressIdWithInlineAddressObject 结账页同时提交
// address_id 和地址对象时必须能下单。
//
// v3.8.4 实测回归：地址先保存成功，结账页把 address_id 和保存接口返回的地址对象
// 一起提交，下单报「收件地址不正确」；同一个地址只传 address_id 却能下单。
// 原因是地址校验先看内联对象、要求它自带姓名/电话/详细地址，内联对象只带 id 或
// 只带省市区时就被判成不完整。地址解析必须以「已有地址优先」。
func TestCreateOrderAcceptsAddressIdWithInlineAddressObject(t *testing.T) {
	w := newZeroValueTestSite(t)
	w.PluginOrder.NoProcess = false
	goods := newQuantityTestGoods(t, w, 10)
	buyer := newQuantityTestBuyer(t, w)
	cleanupTestOrders(t, w, buyer.Id)

	saved := &model.OrderAddress{
		UserId:      buyer.Id,
		Name:        "张三",
		Phone:       "13800000001",
		Province:    "北京",
		City:        "北京",
		Country:     "中国",
		AddressInfo: "某路 1 号",
		Status:      1,
	}
	if err := w.DB.Create(saved).Error; err != nil {
		t.Fatalf("造测试地址失败: %v", err)
	}
	t.Cleanup(func() { w.DB.Unscoped().Delete(&model.OrderAddress{}, saved.Id) })

	cases := []struct {
		name string
		req  *request.OrderRequest
	}{
		{
			name: "address_id + 完整地址对象",
			req: &request.OrderRequest{
				AddressId: saved.Id,
				Address: &request.OrderAddressRequest{
					Id: saved.Id, Name: "张三", Phone: "13800000001",
					Province: "北京", City: "北京", AddressInfo: "某路 1 号",
				},
			},
		},
		{
			name: "address_id + 只带 id 的地址对象",
			req: &request.OrderRequest{
				AddressId: saved.Id,
				Address:   &request.OrderAddressRequest{Id: saved.Id},
			},
		},
		{
			name: "address_id + 只带省市区的地址对象",
			req: &request.OrderRequest{
				AddressId: saved.Id,
				Address:   &request.OrderAddressRequest{Province: "北京", City: "北京", Town: "朝阳"},
			},
		},
		{
			name: "只带 id 的地址对象，不传 address_id",
			req: &request.OrderRequest{
				Address: &request.OrderAddressRequest{Id: saved.Id},
			},
		},
	}

	for _, c := range cases {
		c.req.Details = []request.OrderDetail{{GoodsId: goods.Id, Quantity: 1, QuantityProvided: true}}
		order, err := w.CreateOrder(buyer.Id, c.req)
		if err != nil {
			t.Fatalf("%s: 应可下单，实际报错: %v", c.name, err)
		}
		if order.AddressId != saved.Id {
			t.Fatalf("%s: 订单应挂在地址 %d 上，实际 %d", c.name, saved.Id, order.AddressId)
		}
		// 不完整的内联对象不能反向覆盖已保存的地址
		var after model.OrderAddress
		w.DB.Where("`id` = ?", saved.Id).Take(&after)
		if after.Name != "张三" || after.Phone != "13800000001" || after.AddressInfo != "某路 1 号" {
			t.Fatalf("%s: 已保存地址被内联对象清空了: %+v", c.name, after)
		}
	}

	var orderCount int64
	w.DB.Model(&model.Order{}).Where("`user_id` = ?", buyer.Id).Count(&orderCount)
	if orderCount != int64(len(cases)) {
		t.Fatalf("应建出 %d 笔订单，实际 %d", len(cases), orderCount)
	}
}

// TestSetOrderFinishedRequiresDeliveringForPhysicalOrder 实物订单未发货不能确认收货。
//
// v3.8.4 实测：买家对「已付款、未发货」的实物订单调用 POST /api/order/finish 直接变
// 已完成，后台把待发货订单显示成已完成，运营漏发货，还会提前给卖家结算。
// 虚拟/服务/订阅商品没有物流环节，已付款即可完成（支付回调也这么走）。
func TestSetOrderFinishedRequiresDeliveringForPhysicalOrder(t *testing.T) {
	w := newZeroValueTestSite(t)
	w.PluginOrder.NoProcess = false
	buyer := newQuantityTestBuyer(t, w)
	cleanupTestOrders(t, w, buyer.Id)
	addr := &request.OrderAddressRequest{Name: "李四", Phone: "13800000002", AddressInfo: "某路 2 号"}

	physical := newQuantityTestGoods(t, w, 10)
	order, err := w.CreateOrder(buyer.Id, &request.OrderRequest{
		Details: []request.OrderDetail{{GoodsId: physical.Id, Quantity: 1, QuantityProvided: true}},
		Address: addr,
	})
	if err != nil {
		t.Fatalf("实物订单创建失败: %v", err)
	}
	w.DB.Model(&model.Order{}).Where("`id` = ?", order.Id).UpdateColumn("status", config.OrderStatusPaid)

	// 已付款 ≠ 已发货：确认收货必须被拒
	if err := w.SetOrderFinished(order); err == nil {
		t.Fatal("已付款、未发货的实物订单不该允许确认收货")
	}
	var saved model.Order
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if saved.Status != config.OrderStatusPaid {
		t.Fatalf("订单状态不该被改动，实际 %d", saved.Status)
	}

	// 发货后就可以确认收货
	w.DB.Model(&model.Order{}).Where("`id` = ?", order.Id).UpdateColumn("status", config.OrderStatusDelivering)
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if err := w.SetOrderFinished(&saved); err != nil {
		t.Fatalf("已发货订单应可完成，实际报错: %v", err)
	}
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if saved.Status != config.OrderStatusCompleted {
		t.Fatalf("已发货订单应变为已完成，实际 %d", saved.Status)
	}

	// 虚拟商品没有物流环节：已付款直接完成
	virtual := newQuantityTestGoods(t, w, 10)
	if err := w.DB.Create(&model.ArchiveProduct{
		ArchiveId:   virtual.Id,
		ProductType: config.ProductTypeVirtual,
	}).Error; err != nil {
		t.Fatalf("造虚拟商品侧表失败: %v", err)
	}
	virtualOrder, err := w.CreateOrder(buyer.Id, &request.OrderRequest{
		Details: []request.OrderDetail{{GoodsId: virtual.Id, Quantity: 1, QuantityProvided: true}},
	})
	if err != nil {
		t.Fatalf("虚拟订单创建失败（不应要求地址）: %v", err)
	}
	w.DB.Model(&model.Order{}).Where("`id` = ?", virtualOrder.Id).UpdateColumn("status", config.OrderStatusPaid)
	if err := w.SetOrderFinished(virtualOrder); err != nil {
		t.Fatalf("虚拟商品已付款应可完成，实际报错: %v", err)
	}
}
