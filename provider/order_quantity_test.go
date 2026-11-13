package provider

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// v3.8.4 客户实测：POST /api/order/create 传 quantity=0 会被改成 1 件，
// 传 -1 会生成 -US$0.50 的订单、库存 +1，并在 sold_count 上报
// Out of range value 却仍然返回成功。

func newQuantityTestGoods(t *testing.T, w *Website, stock int64) *model.Archive {
	t.Helper()
	goods := &model.Archive{
		Title:      fmt.Sprintf("数量校验-%d", time.Now().UnixNano()),
		ModuleId:   1,
		CategoryId: 1,
		Stock:      stock,
		Price:      50, // US$0.50
	}
	if err := w.DB.Create(goods).Error; err != nil {
		t.Fatalf("造测试商品失败: %v", err)
	}
	t.Cleanup(func() {
		w.DB.Unscoped().Where("`archive_id` = ?", goods.Id).Delete(&model.ArchiveProduct{})
		w.DB.Unscoped().Where("`id` = ?", goods.Id).Delete(&model.Archive{})
	})
	return goods
}

func newQuantityTestBuyer(t *testing.T, w *Website) *model.User {
	t.Helper()
	u := &model.User{
		UserName: fmt.Sprintf("qtytest%d", time.Now().UnixNano()%1e12),
		Email:    fmt.Sprintf("qty%d@example.test", time.Now().UnixNano()%1e12),
	}
	if err := w.DB.Create(u).Error; err != nil {
		t.Fatalf("造测试用户失败: %v", err)
	}
	t.Cleanup(func() { w.DB.Unscoped().Delete(&model.User{}, u.Id) })
	return u
}

func cleanupTestOrders(t *testing.T, w *Website, userId uint) {
	t.Helper()
	t.Cleanup(func() {
		var orderIds []string
		w.DB.Model(&model.Order{}).Where("`user_id` = ?", userId).Pluck("order_id", &orderIds)
		for _, oid := range orderIds {
			w.DB.Unscoped().Where("`order_id` = ?", oid).Delete(&model.OrderDetail{})
			w.DB.Unscoped().Where("`order_id` = ?", oid).Delete(&model.Order{})
		}
	})
}

// TestCreateOrderRejectsNonPositiveQuantity 显式 0 / 负数必须在算金额、扣库存之前被拒。
func TestCreateOrderRejectsNonPositiveQuantity(t *testing.T) {
	w := newZeroValueTestSite(t)
	goods := newQuantityTestGoods(t, w, 98)
	buyer := newQuantityTestBuyer(t, w)
	cleanupTestOrders(t, w, buyer.Id)

	for _, q := range []int{0, -1} {
		req := &request.OrderRequest{
			Details: []request.OrderDetail{
				{GoodsId: goods.Id, Quantity: q, QuantityProvided: true},
			},
		}
		_, err := w.CreateOrder(buyer.Id, req)
		if err == nil {
			t.Fatalf("quantity=%d 必须被拒绝，实际下单成功", q)
		}
		if !strings.Contains(err.Error(), "数量") && !strings.Contains(err.Error(), "Quantity") {
			t.Fatalf("quantity=%d 应返回数量错误，实际: %v", q, err)
		}
	}

	var orderCount int64
	w.DB.Model(&model.Order{}).Where("`user_id` = ?", buyer.Id).Count(&orderCount)
	if orderCount != 0 {
		t.Fatalf("非法数量不能生成订单，实际 %d 笔", orderCount)
	}
	var stock int64
	w.DB.Model(&model.Archive{}).Where("`id` = ?", goods.Id).Pluck("stock", &stock)
	if stock != 98 {
		t.Fatalf("非法数量不能改库存，实际 %d", stock)
	}
	var sold int64
	w.DB.Model(&model.ArchiveProduct{}).Where("`archive_id` = ?", goods.Id).Pluck("sold_count", &sold)
	if sold != 0 {
		t.Fatalf("非法数量不能改销量，实际 %d", sold)
	}
}

// TestCreateOrderMultiDetailsRejectsWholeOrder 多商品订单只要有一条非法，整单拒绝。
func TestCreateOrderMultiDetailsRejectsWholeOrder(t *testing.T) {
	w := newZeroValueTestSite(t)
	goodsA := newQuantityTestGoods(t, w, 10)
	goodsB := newQuantityTestGoods(t, w, 10)
	buyer := newQuantityTestBuyer(t, w)
	cleanupTestOrders(t, w, buyer.Id)

	req := &request.OrderRequest{
		Details: []request.OrderDetail{
			{GoodsId: goodsA.Id, Quantity: 1, QuantityProvided: true},
			{GoodsId: goodsB.Id, Quantity: -1, QuantityProvided: true},
		},
	}
	if _, err := w.CreateOrder(buyer.Id, req); err == nil {
		t.Fatal("有一行非法数量时整单必须拒绝")
	}

	var orderCount int64
	w.DB.Model(&model.Order{}).Where("`user_id` = ?", buyer.Id).Count(&orderCount)
	if orderCount != 0 {
		t.Fatalf("整单拒绝后不能留下订单，实际 %d 笔", orderCount)
	}
	var stockA, stockB int64
	w.DB.Model(&model.Archive{}).Where("`id` = ?", goodsA.Id).Pluck("stock", &stockA)
	w.DB.Model(&model.Archive{}).Where("`id` = ?", goodsB.Id).Pluck("stock", &stockB)
	if stockA != 10 || stockB != 10 {
		t.Fatalf("整单拒绝后库存不能动，实际 A=%d B=%d", stockA, stockB)
	}
}

// TestCreateOrderOmittedQuantityDefaultsToOne 完全省略 quantity 仍按 1 件下单。
func TestCreateOrderOmittedQuantityDefaultsToOne(t *testing.T) {
	w := newZeroValueTestSite(t)
	goods := newQuantityTestGoods(t, w, 98)
	buyer := newQuantityTestBuyer(t, w)
	cleanupTestOrders(t, w, buyer.Id)

	req := &request.OrderRequest{
		Details: []request.OrderDetail{{GoodsId: goods.Id}}, // 没传 quantity
	}
	order, err := w.CreateOrder(buyer.Id, req)
	if err != nil {
		t.Fatalf("省略 quantity 应按下单 1 件处理，实际报错: %v", err)
	}
	if order.Amount != 50 {
		t.Fatalf("1 件 US$0.50 的订单金额应为 50 分，实际 %d", order.Amount)
	}
	var detail model.OrderDetail
	if err := w.DB.Where("`order_id` = ?", order.OrderId).Take(&detail).Error; err != nil {
		t.Fatalf("读回订单明细失败: %v", err)
	}
	if detail.Quantity != 1 {
		t.Fatalf("省略 quantity 时明细数量应为 1，实际 %d", detail.Quantity)
	}
	var stock int64
	w.DB.Model(&model.Archive{}).Where("`id` = ?", goods.Id).Pluck("stock", &stock)
	if stock != 97 {
		t.Fatalf("库存应只扣 1 件（98→97），实际 %d", stock)
	}
	var sold int64
	w.DB.Model(&model.ArchiveProduct{}).Where("`archive_id` = ?", goods.Id).Pluck("sold_count", &sold)
	if sold != 1 {
		t.Fatalf("销量应只加 1，实际 %d", sold)
	}
}

// TestCreateOrderRejectsOverStock 超库存仍要拒绝（不能因为改了数量校验而放松）。
func TestCreateOrderRejectsOverStock(t *testing.T) {
	w := newZeroValueTestSite(t)
	goods := newQuantityTestGoods(t, w, 1)
	buyer := newQuantityTestBuyer(t, w)
	cleanupTestOrders(t, w, buyer.Id)

	req := &request.OrderRequest{
		Details: []request.OrderDetail{{GoodsId: goods.Id, Quantity: 2, QuantityProvided: true}},
	}
	if _, err := w.CreateOrder(buyer.Id, req); err == nil {
		t.Fatal("库存只有 1 件时买 2 件必须拒绝")
	}
	var stock int64
	w.DB.Model(&model.Archive{}).Where("`id` = ?", goods.Id).Pluck("stock", &stock)
	if stock != 1 {
		t.Fatalf("拒绝后库存不能变，实际 %d", stock)
	}
}

// TestCreateOrderRejectsZeroBeforePricing 显式 0 不能被「MinQuantity 上调」掩盖成合法数量。
func TestCreateOrderRejectsZeroBeforePricing(t *testing.T) {
	w := newZeroValueTestSite(t)
	goods := newQuantityTestGoods(t, w, 98)
	// 最小起订量在侧表 archive_products：老实现会先把 0 改成 1，再被 MinQuantity 抬到 5
	if err := w.DB.Create(&model.ArchiveProduct{
		ArchiveId:   goods.Id,
		ProductType: config.ProductTypeNormal,
		MinQuantity: 5,
	}).Error; err != nil {
		t.Fatalf("设置最小起订量失败: %v", err)
	}
	buyer := newQuantityTestBuyer(t, w)
	cleanupTestOrders(t, w, buyer.Id)

	req := &request.OrderRequest{
		Details: []request.OrderDetail{{GoodsId: goods.Id, Quantity: 0, QuantityProvided: true}},
	}
	if _, err := w.CreateOrder(buyer.Id, req); err == nil {
		t.Fatal("显式 0 必须拒绝，不能被最小起订量抬成 5 件下单")
	}
	var count int64
	w.DB.Model(&model.Order{}).Where("`user_id` = ?", buyer.Id).Count(&count)
	if count != 0 {
		t.Fatalf("非法数量不能生成订单，实际 %d 笔", count)
	}
}
