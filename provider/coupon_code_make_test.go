package provider

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// 本文件只锁「批量发码的插入失败必须传出去」这一条不变式。
//
// 背景（2026-10-27）：provider.MakeCouponCode 的 userId==0 分支在循环里
// `w.DB.Create(&item)` 完全不看错误 —— 100 张全插失败也照样 return nil, nil，
// 后台拿到 code=0 的成功回执、刷新列表却是空的，没有任何线索。
// 用函数注入插入动作，是为了在没有数据库的情况下也能断言「第 N 张失败」会中止并上报。

// TestCreateCouponCodesStopsAtFirstFailure 第 N 张插入失败时必须立刻停手并返回错误。
func TestCreateCouponCodesStopsAtFirstFailure(t *testing.T) {
	coupon := &model.Coupon{
		Id:        7,
		Money:     500,
		MoneyType: config.CouponMoneyTypeFixed,
		Expire:    30,
	}
	base := model.CouponCode{CouponId: coupon.Id, Status: config.CouponStatusUnused}

	attempts := 0
	boom := errors.New("表锁等待超时")
	err := createCouponCodes(base, coupon, 3, func(code *model.CouponCode) error {
		attempts++
		if attempts == 2 {
			return boom
		}
		return nil
	})

	if err == nil {
		t.Fatal("第 2 张插入失败时必须返回错误；返回 nil 会让上层把「一张都没写进去」报成成功")
	}
	if !errors.Is(err, boom) {
		t.Errorf("错误必须能被 errors.Is 溯源到原始错误，实际：%v", err)
	}
	if !strings.Contains(err.Error(), "第 2/3 张") {
		t.Errorf("错误里要说明是第几张、共几张，方便定位，实际：%v", err)
	}
	if attempts != 2 {
		t.Errorf("失败后应立刻停止，实际继续尝试了 %d 次", attempts)
	}
}

// TestCreateCouponCodesFillsEachRow 成功路径要逐张生成券码，且不能复用 base 的 Id。
func TestCreateCouponCodesFillsEachRow(t *testing.T) {
	coupon := &model.Coupon{
		Id:        9,
		Money:     1200,
		MoneyType: config.CouponMoneyTypeFixed,
		Expire:    7,
	}
	// Id 故意给一个非零值：若 helper 直接传 base 而不归零，会覆盖库里已有的记录。
	base := model.CouponCode{CouponId: coupon.Id, Id: 99, Status: config.CouponStatusUnused}

	var got []*model.CouponCode
	if err := createCouponCodes(base, coupon, 3, func(code *model.CouponCode) error {
		got = append(got, code)
		return nil
	}); err != nil {
		t.Fatalf("插入成功时不应返回错误：%v", err)
	}

	if len(got) != 3 {
		t.Fatalf("应生成 3 张，实际 %d 张", len(got))
	}
	codes := make(map[string]bool, len(got))
	for i, code := range got {
		if code.Id != 0 {
			t.Errorf("第 %d 张的 Id 必须归零，否则会覆盖已有券码（实际 %d）", i+1, code.Id)
		}
		if code.CouponId != coupon.Id {
			t.Errorf("第 %d 张的 CouponId 应为 %d，实际 %d", i+1, coupon.Id, code.CouponId)
		}
		if code.Money != coupon.Money {
			t.Errorf("第 %d 张的金额应为券面额 %d（单位分），实际 %d", i+1, coupon.Money, code.Money)
		}
		if code.Status != config.CouponStatusUnused {
			t.Errorf("第 %d 张应为未使用状态，实际 %d", i+1, code.Status)
		}
		if code.Code == "" {
			t.Errorf("第 %d 张没有生成券码", i+1)
		}
		codes[code.Code] = true
	}
	if len(codes) != len(got) {
		t.Errorf("同一批里出现了重复券码：%v", codes)
	}
}

// TestMakeCouponCodeBatchGoesThroughHelper 锁住接线：批量分支必须走
// createCouponCodes + 事务，而不是又内联一个 `w.DB.Create` 不看错误的循环。
//
// 上一层的 createCouponCodes 测试只能证明「helper 会报错」，证明不了
// MakeCouponCode 真的用它 —— 有人把循环改回去，helper 测试照样绿。
func TestMakeCouponCodeBatchGoesThroughHelper(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "coupon.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 coupon.go 失败：%v", err)
	}

	var fn *ast.FuncDecl
	for _, decl := range parsed.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == "MakeCouponCode" {
			fn = f
			break
		}
	}
	if fn == nil {
		t.Fatal("coupon.go 里找不到 MakeCouponCode，测试前提不成立")
	}

	idents := map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			idents[id.Name] = true
		}
		return true
	})

	for _, want := range []struct{ name, why string }{
		{"createCouponCodes", "批量发码必须走统一的生成函数，才能带上错误检查"},
		{"Begin", "整批要么全部落库、要么一张不留"},
		{"Rollback", "中途失败要回滚，别留半批孤儿码"},
		{"Commit", "提交失败也要上报，否则又变成假成功"},
	} {
		if !idents[want.name] {
			t.Errorf("MakeCouponCode 里看不到 %s：%s", want.name, want.why)
		}
	}
}
