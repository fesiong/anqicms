package provider

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

// ── 优惠券零值语义护栏（2026-10-29 指针化整改第二批）───────────────────────
//
// 优惠券比会员那批更危险，因为它挂着**已发出的券码**：
//
//  1. SaveCoupon 是逐字段无条件赋值 —— 「改个标题」会把面额、有效期、
//     状态、门槛一并写成零值。面额清零的券仍在流通，已发出的券码
//     在核销时按 0 元抵扣，用户拿到的「满 100 减 30」变成什么都不减。
//  2. SaveCouponCode 用 `Money > 0` / `Status > 0` 判断要不要写 ——
//     0 元券码、把券码标记失效（status=0）都写不进去，回执却是 ok。
//
// 本文件锁住两件事：request 结构体的指针性，以及真跑 Save* 后
// 「未传字段保持原值」「显式 0 能落库」。

// TestSaveCouponKeepsUnpassedFields 复现「改一个字段清空其余」。
//
// 注意这里的调用方式：只传 Id + Title，其余字段全零 —— 这正是 AI/MCP
// 与「只改标题」的后台操作会发生的形态。
func TestSaveCouponKeepsUnpassedFields(t *testing.T) {
	w := newZeroValueTestSite(t)

	title := fmt.Sprintf("零值券-%d", time.Now().UnixNano())
	err := w.SaveCoupon(&request.CouponRequest{
		Title:        library.Ptr(title),
		Money:        library.Ptr(int64(3000)),
		MaxMoney:     library.Ptr(int64(3000)),
		MoneyType:    library.Ptr(0),
		Type:         library.Ptr(3),
		Expire:       library.Ptr(int64(30)),
		Status:       library.Ptr(1),
		NeedMoney:    library.Ptr(int64(10000)),
		NeedIntegral: library.Ptr(int64(500)),
		RangeType:    library.Ptr(1),
		StartTime:    library.Ptr(time.Now().Unix()),
		EndTime:      library.Ptr(time.Now().AddDate(0, 0, 90).Unix()),
	})
	if err != nil {
		t.Fatalf("新建优惠券失败（表不存在就跳过本用例）：%v", err)
	}

	var created model.Coupon
	if err := w.DB.Where("`title` = ?", title).Take(&created).Error; err != nil {
		t.Skipf("测试库无 coupons 表或写入未生效，跳过：%v", err)
	}
	defer func() {
		_ = w.DB.Unscoped().Delete(&model.CouponCode{}, "coupon_id = ?", created.Id).Error
		_ = w.DB.Unscoped().Delete(&model.Coupon{}, created.Id).Error
	}()

	if created.Money != 3000 || created.Status != 1 || created.Expire != 30 {
		t.Fatalf("新建优惠券字段未落库：money=%d status=%d expire=%d", created.Money, created.Status, created.Expire)
	}

	// 只改标题（PATCH：partial=true，等价于 AI/MCP 通道的默认行为）。
	err = w.SaveCoupon(&request.CouponRequest{
		Id:      created.Id,
		Title:   library.Ptr(title + "-改"),
		Partial: true,
	})
	if err != nil {
		t.Fatalf("PATCH 优惠券失败：%v", err)
	}

	var patched model.Coupon
	if err := w.DB.Where("`id` = ?", created.Id).Take(&patched).Error; err != nil {
		t.Fatalf("重查优惠券失败：%v", err)
	}
	if patched.Title != title+"-改" {
		t.Errorf("title 应为 %q，实际 %q", title+"-改", patched.Title)
	}
	if patched.Money != 3000 {
		t.Errorf("未传 money 却被改写：期望 3000，实际 %d（面额被清零，已发出的券码按 0 元抵扣）", patched.Money)
	}
	if patched.Status != 1 {
		t.Errorf("未传 status 却被改写：期望 1（启用），实际 %d（券被静默停用）", patched.Status)
	}
	if patched.Expire != 30 {
		t.Errorf("未传 expire 却被清零：期望 30，实际 %d（领取后立即过期）", patched.Expire)
	}
	if patched.NeedMoney != 10000 {
		t.Errorf("未传 need_money 却被清零：期望 10000（满 100 可用），实际 %d（变成无门槛）", patched.NeedMoney)
	}
	if patched.NeedIntegral != 500 {
		t.Errorf("未传 need_integral 却被清零：期望 500，实际 %d（积分兑换券变免费）", patched.NeedIntegral)
	}
	if patched.RangeType != 1 {
		t.Errorf("未传 range_type 却被清零：期望 1（仅指定商品可用），实际 %d（变成全站可用）", patched.RangeType)
	}
	if patched.Type != 3 {
		t.Errorf("未传 type 却被清零：期望 3（下单赠送），实际 %d", patched.Type)
	}
	// 有效期是金额券的第二条防线，被清零等于券永久有效或立即失效。
	if patched.EndTime != created.EndTime {
		t.Errorf("未传 end_time 却被改写：期望 %d，实际 %d", created.EndTime, patched.EndTime)
	}

	// 显式 0：停用必须能写进去（原来逐字段赋值反而"能"，改造后不能丢）。
	if err = w.SaveCoupon(&request.CouponRequest{Id: created.Id, Status: library.Ptr(0)}); err != nil {
		t.Fatalf("停用优惠券失败：%v", err)
	}
	var stopped model.Coupon
	if err = w.DB.Where("`id` = ?", created.Id).Take(&stopped).Error; err != nil {
		t.Fatalf("重查优惠券失败：%v", err)
	}
	if stopped.Status != 0 {
		t.Errorf("显式传 status=0 应停用，实际 %d", stopped.Status)
	}
	if stopped.Money != 3000 {
		t.Errorf("停用后 money 应保持 3000，实际 %d", stopped.Money)
	}

	// 反向对照：UpdateAll（后台表单全量提交）时，未出现的字段应当被清空。
	// 这条是刻意的语义，不是 bug —— 后台表单提交整个表单，字段没出现就是用户删了它。
	// 它同时证明上面的 PATCH 断言不是「什么都没写」导致的假绿。
	if err = w.SaveCoupon(&request.CouponRequest{
		Id:        created.Id,
		Title:     library.Ptr(title + "-全量"),
		UpdateAll: true,
	}); err != nil {
		t.Fatalf("全量保存失败：%v", err)
	}
	var replaced model.Coupon
	if err = w.DB.Where("`id` = ?", created.Id).Take(&replaced).Error; err != nil {
		t.Fatalf("重查优惠券失败：%v", err)
	}
	if replaced.Money != 0 {
		t.Errorf("UpdateAll 下未传 money 应被清零（表单全量覆盖），实际 %d", replaced.Money)
	}
	if replaced.Title != title+"-全量" {
		t.Errorf("UpdateAll 下 title 应为 %q，实际 %q", title+"-全量", replaced.Title)
	}
}

// TestSaveCouponCodeZeroValue 复现券码的两个零值写不进。
func TestSaveCouponCodeZeroValue(t *testing.T) {
	w := newZeroValueTestSite(t)

	// 直接落一张券与一张券码，避开 MakeCouponCode 的随机性。
	coupon := model.Coupon{
		Title:     fmt.Sprintf("券码零值券-%d", time.Now().UnixNano()),
		Money:     2000,
		Status:    1,
		Type:      3,
		Expire:    30,
		MoneyType: 0,
	}
	if err := w.DB.Create(&coupon).Error; err != nil {
		t.Skipf("测试库无 coupons 表，跳过：%v", err)
	}
	defer func() { _ = w.DB.Unscoped().Delete(&model.Coupon{}, coupon.Id).Error }()

	couponCode := model.CouponCode{
		CouponId: coupon.Id,
		Code:     fmt.Sprintf("zv%d", time.Now().UnixNano()),
		Money:    2000,
		Status:   1,
	}
	if err := w.DB.Create(&couponCode).Error; err != nil {
		t.Fatalf("创建券码失败：%v", err)
	}
	defer func() { _ = w.DB.Unscoped().Delete(&model.CouponCode{}, couponCode.Id).Error }()

	// 显式 0 元券码（全额抵扣或补差价场景），必须写进去。
	if err := w.SaveCouponCode(&request.CouponCodeRequest{
		Id:    couponCode.Id,
		Money: library.Ptr(int64(0)),
	}); err != nil {
		t.Fatalf("写入 money=0 失败：%v", err)
	}
	var zeroed model.CouponCode
	if err := w.DB.Where("`id` = ?", couponCode.Id).Take(&zeroed).Error; err != nil {
		t.Fatalf("重查券码失败：%v", err)
	}
	if zeroed.Money != 0 {
		t.Errorf("显式传 money=0 应写入 0，实际 %d（`Money > 0` 把 0 挡在门外）", zeroed.Money)
	}

	// 显式 status=0（标记失效），必须写进去。
	if err := w.SaveCouponCode(&request.CouponCodeRequest{
		Id:     couponCode.Id,
		Status: library.Ptr(0),
	}); err != nil {
		t.Fatalf("标记券码失效失败：%v", err)
	}
	var invalid model.CouponCode
	if err := w.DB.Where("`id` = ?", couponCode.Id).Take(&invalid).Error; err != nil {
		t.Fatalf("重查券码失败：%v", err)
	}
	if invalid.Status != 0 {
		t.Errorf("显式传 status=0 应写入 0（失效），实际 %d（`Status > 0` 把 0 挡在门外）", invalid.Status)
	}
	if invalid.Money != 0 {
		t.Errorf("只改 status 不应动 money：期望 0，实际 %d", invalid.Money)
	}
	// 未传 code 时不能把已有券码改成空串 —— 那会让券码彻底查不到。
	if invalid.Code == "" {
		t.Errorf("未传 code 不应清空已有券码（实际被清空）")
	}
}

// TestSaveActivityKeepsUnpassedFieldsAndSkus 复现活动的两个坑：
//
//  1. 逐字段无条件赋值 —— 「改个标题」把活动价/库存/状态/时间清零；
//  2. `len(req.Skus)==0` 就删光规格 —— 「没传 skus」与「传空数组」在值类型下
//     是同一个零值，于是改标题顺手删掉了全部规格，活动价与库存随之失效。
func TestSaveActivityKeepsUnpassedFieldsAndSkus(t *testing.T) {
	w := newZeroValueTestSite(t)

	title := fmt.Sprintf("零值活动-%d", time.Now().UnixNano())
	skus := []request.ActivitySku{
		{SkuId: 11, GoodsId: 101, Price: 1999, Stock: 5},
		{SkuId: 12, GoodsId: 101, Price: 2999, Stock: 7},
	}
	err := w.SaveActivity(&request.ActivityRequest{
		Title:        title,
		Price:        library.Ptr(int64(1999)),
		Stock:        library.Ptr(int64(12)),
		UserLimit:    library.Ptr(int64(2)),
		Status:       library.Ptr(uint(1)),
		StartTime:    library.Ptr(time.Now().Unix()),
		EndTime:      library.Ptr(time.Now().AddDate(0, 1, 0).Unix()),
		CanUseCoupon: library.Ptr(true),
		GoodsId:      library.Ptr(int64(101)),
		Skus:         &skus,
	})
	if err != nil {
		t.Fatalf("新建活动失败（表不存在就跳过）：%v", err)
	}

	var created model.Activity
	if err := w.DB.Where("`title` = ?", title).Take(&created).Error; err != nil {
		t.Skipf("测试库无 activities 表，跳过：%v", err)
	}
	defer func() {
		_ = w.DB.Unscoped().Delete(&model.ActivitySku{}, "activity_id = ?", created.Id).Error
		_ = w.DB.Unscoped().Delete(&model.Activity{}, created.Id).Error
	}()

	var createdSkuCount int64
	w.DB.Model(&model.ActivitySku{}).Where("activity_id = ?", created.Id).Count(&createdSkuCount)
	if createdSkuCount != 2 {
		t.Fatalf("新建活动应写入 2 条规格，实际 %d", createdSkuCount)
	}
	if created.Price != 1999 || created.UserLimit != 2 {
		t.Fatalf("新建活动字段未落库：price=%d user_limit=%d", created.Price, created.UserLimit)
	}

	// 只改标题：其余字段与规格都必须原样保留。
	if err = w.SaveActivity(&request.ActivityRequest{
		Id:      created.Id,
		Title:   title + "-改",
		Partial: true,
	}); err != nil {
		t.Fatalf("PATCH 活动失败：%v", err)
	}

	var patched model.Activity
	if err = w.DB.Where("`id` = ?", created.Id).Take(&patched).Error; err != nil {
		t.Fatalf("重查活动失败：%v", err)
	}
	if patched.Title != title+"-改" {
		t.Errorf("title 应为 %q，实际 %q", title+"-改", patched.Title)
	}
	if patched.Price != 1999 {
		t.Errorf("未传 price 却被清零：期望 1999，实际 %d（活动变成免费）", patched.Price)
	}
	if patched.Stock != 12 {
		t.Errorf("未传 stock 却被清零：期望 12，实际 %d（活动变成售罄）", patched.Stock)
	}
	if patched.UserLimit != 2 {
		t.Errorf("未传 user_limit 却被清零：期望 2（每人限购 2），实际 %d（变成不限购）", patched.UserLimit)
	}
	if patched.Status != 1 {
		t.Errorf("未传 status 却被改写：期望 1（生效），实际 %d（活动被静默下线）", patched.Status)
	}
	if !patched.CanUseCoupon {
		t.Errorf("未传 can_use_coupon 却被改写：期望 true，实际 false")
	}
	if patched.GoodsId != 101 {
		t.Errorf("未传 goods_id 却被清零：期望 101，实际 %d（活动与商品脱钩）", patched.GoodsId)
	}
	if patched.EndTime != created.EndTime {
		t.Errorf("未传 end_time 却被改写：期望 %d，实际 %d", created.EndTime, patched.EndTime)
	}

	var skuCount int64
	w.DB.Model(&model.ActivitySku{}).Where("activity_id = ?", created.Id).Count(&skuCount)
	if skuCount != 2 {
		t.Errorf("PATCH 未传 skus 时规格应保持 2 条，实际 %d（规格被整批删除，活动价与库存失效）", skuCount)
	}

	// 显式 0 / false：必须落库（免费、售罄、不限购、下线、不许叠加优惠）。
	if err = w.SaveActivity(&request.ActivityRequest{
		Id:           created.Id,
		Price:        library.Ptr(int64(0)),
		Stock:        library.Ptr(int64(0)),
		UserLimit:    library.Ptr(int64(0)),
		Status:       library.Ptr(uint(0)),
		CanUseCoupon: library.Ptr(false),
		Partial:      true,
	}); err != nil {
		t.Fatalf("写入零值失败：%v", err)
	}
	var zeroed model.Activity
	if err = w.DB.Where("`id` = ?", created.Id).Take(&zeroed).Error; err != nil {
		t.Fatalf("重查活动失败：%v", err)
	}
	if zeroed.Price != 0 {
		t.Errorf("显式 price=0 应写入（免费活动），实际 %d", zeroed.Price)
	}
	if zeroed.Stock != 0 {
		t.Errorf("显式 stock=0 应写入（售罄），实际 %d", zeroed.Stock)
	}
	if zeroed.UserLimit != 0 {
		t.Errorf("显式 user_limit=0 应写入（不限购），实际 %d", zeroed.UserLimit)
	}
	if zeroed.Status != 0 {
		t.Errorf("显式 status=0 应写入（失效），实际 %d", zeroed.Status)
	}
	if zeroed.CanUseCoupon {
		t.Errorf("显式 can_use_coupon=false 应写入，实际 true（还能叠加优惠券）")
	}

	// 显式空数组：这才是「清空规格」的入口。
	empty := []request.ActivitySku{}
	if err = w.SaveActivity(&request.ActivityRequest{
		Id:      created.Id,
		Skus:    &empty,
		Partial: true,
	}); err != nil {
		t.Fatalf("清空规格失败：%v", err)
	}
	w.DB.Model(&model.ActivitySku{}).Where("activity_id = ?", created.Id).Count(&skuCount)
	if skuCount != 0 {
		t.Errorf("显式传空数组应清空全部规格，实际还剩 %d 条", skuCount)
	}
}

// TestActivityRequestFieldsArePointers 锁住活动请求结构体的指针性。
func TestActivityRequestFieldsArePointers(t *testing.T) {
	v := reflect.TypeOf(request.ActivityRequest{})
	for _, name := range []string{
		"GoodsType", "StartTime", "EndTime", "UserLimit", "Description", "Images",
		"GoodsId", "Price", "Stock", "CanUseCoupon", "Status", "Skus",
	} {
		f, ok := v.FieldByName(name)
		if !ok {
			t.Errorf("ActivityRequest 缺少字段 %s（改了名字就同步改这里）", name)
			continue
		}
		if f.Type.Kind() != reflect.Ptr {
			t.Errorf("ActivityRequest.%s 必须是 %s 的指针类型（当前 %s）：0/false/空数组都是它的合法业务值",
				name, f.Type, f.Type)
		}
	}
	// Title 反过来必须保持值类型：空串的语义是「不改」，指针化只会引向「清空标题」。
	if f, _ := v.FieldByName("Title"); f.Type.Kind() == reflect.Ptr {
		t.Errorf("ActivityRequest.Title 不应是指针：空串语义是「不改」")
	}
}

// TestCouponRequestFieldsArePointers 锁住优惠券请求结构体的指针性。

func TestCouponRequestFieldsArePointers(t *testing.T) {
	mustBePointer := []struct {
		typ    any
		fields []string
	}{
		{request.CouponRequest{}, []string{
			"Money", "StartTime", "EndTime", "Title", "Description", "Expire",
			"Status", "Type", "MaxMoney", "MoneyType", "NeedIntegral", "NeedMoney",
			"RangeType",
		}},
		{request.CouponCodeRequest{}, []string{
			"UserId", "StartTime", "EndTime", "Status", "Money",
		}},
	}
	for _, g := range mustBePointer {
		v := reflect.TypeOf(g.typ)
		for _, name := range g.fields {
			f, ok := v.FieldByName(name)
			if !ok {
				t.Errorf("%s 缺少字段 %s（改了名字就同步改这里）", v.Name(), name)
				continue
			}
			if f.Type.Kind() != reflect.Ptr {
				t.Errorf("%s.%s 必须是 %s 的指针类型（当前 %s）：`0` 是它的合法业务值",
					v.Name(), name, f.Type, f.Type)
			}
		}
	}
}
