package provider

import (
	"sync"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// TestSetOrderCanceledRestoresStockOnce 重复/并发取消只能回补一次库存。
//
// 客户实测：库存 98，下单 2 件 → 96，首次取消 → 98，再次取消同一单 → 100。
// 根因是 SetOrderCanceled 无条件改状态 + 无条件回补，没有任何闸门。
func TestSetOrderCanceledRestoresStockOnce(t *testing.T) {
	w := newZeroValueTestSite(t)

	goods := &model.Archive{Title: "取消库存护栏", ModuleId: 1, CategoryId: 1, Stock: 98}
	if err := w.DB.Create(goods).Error; err != nil {
		t.Fatalf("造测试商品失败: %v", err)
	}
	orderId := testOrderId("cn")
	order := &model.Order{OrderId: orderId, Amount: 100, Status: config.OrderStatusWaiting}
	if err := w.DB.Create(order).Error; err != nil {
		t.Fatalf("造订单失败: %v", err)
	}
	detail := &model.OrderDetail{OrderId: orderId, GoodsId: int64(goods.Id), Quantity: 2, Amount: 100}
	if err := w.DB.Create(detail).Error; err != nil {
		t.Fatalf("造订单明细失败: %v", err)
	}
	t.Cleanup(func() {
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.OrderDetail{})
		w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Order{})
		w.DB.Unscoped().Where("`id` = ?", goods.Id).Delete(&model.Archive{})
	})

	// 下单时库存已被扣成 96（这里直接设好基线，模拟扣减后的状态）
	if err := w.DB.Model(&model.Archive{}).Where("`id` = ?", goods.Id).UpdateColumn("stock", 96).Error; err != nil {
		t.Fatalf("设置基线库存失败: %v", err)
	}

	if err := w.SetOrderCanceled(order); err != nil {
		t.Fatalf("首次取消失败: %v", err)
	}
	assertStock := func(want int, step string) {
		t.Helper()
		var got int
		if err := w.DB.Model(&model.Archive{}).Where("`id` = ?", goods.Id).Pluck("stock", &got).Error; err != nil {
			t.Fatalf("%s: 读库存失败 %v", step, err)
		}
		if got != want {
			t.Fatalf("%s: 库存应为 %d，实际 %d（回补次数不对）", step, want, got)
		}
	}
	assertStock(98, "首次取消")

	// 第二次：接口重放，调用方手里可能还是「未取消」的陈旧副本
	if err := w.SetOrderCanceled(&model.Order{Id: order.Id, OrderId: orderId, Status: config.OrderStatusWaiting}); err != nil {
		t.Fatalf("重复取消应幂等成功（不能报错让前端以为失败）: %v", err)
	}
	assertStock(98, "重复取消")

	// 并发：模拟用户连点 / 多个入口同时取消
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stale := &model.Order{Id: order.Id, OrderId: orderId, Status: config.OrderStatusWaiting}
			if err := w.SetOrderCanceled(stale); err != nil {
				t.Errorf("并发取消应幂等成功: %v", err)
			}
		}()
	}
	wg.Wait()
	assertStock(98, "并发取消")

	var saved model.Order
	w.DB.Where("`id` = ?", order.Id).Take(&saved)
	if saved.Status != config.OrderStatusCanceled {
		t.Fatalf("订单应为已取消，实际 %d", saved.Status)
	}
}

// TestSetOrderCanceledRefusesFinishedOrRefunded 已完成/已退款的订单不能被「取消」掉。
//
// 反向对照：把 cancelableOrderStatuses 里加回 OrderStatusRefunded，这条立刻红 ——
// 那种情况下一次取消会把已退款订单推回「已取消」并再回补一次库存。
func TestSetOrderCanceledRefusesFinishedOrRefunded(t *testing.T) {
	w := newZeroValueTestSite(t)
	for _, status := range []int{config.OrderStatusRefunded, config.OrderStatusCompleted} {
		orderId := testOrderId("cf")
		order := &model.Order{OrderId: orderId, Amount: 100, Status: status}
		if err := w.DB.Create(order).Error; err != nil {
			t.Fatalf("造订单失败: %v", err)
		}
		t.Cleanup(func() { w.DB.Unscoped().Where("`order_id` = ?", orderId).Delete(&model.Order{}) })

		if err := w.SetOrderCanceled(order); err == nil {
			t.Fatalf("status=%d 的订单不该允许取消", status)
		}
		var saved model.Order
		w.DB.Where("`id` = ?", order.Id).Take(&saved)
		if saved.Status != status {
			t.Fatalf("订单状态被改成了 %d（原 %d）", saved.Status, status)
		}
	}
}
