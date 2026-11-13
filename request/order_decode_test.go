package request

import (
	"encoding/json"
	"testing"
)

// TestOrderDetailQuantityProvided 区分「没传 quantity」和「显式传 0」。
//
// 两者解码后都是 0，只能靠解码阶段记录。下单接口正是靠这个字段决定：
// 没传 → 默认 1 件；传 0 / 负数 → 直接拒绝（曾生成 -US$0.50 的订单并反向加库存）。
func TestOrderDetailQuantityProvided(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		quantity int
		provided bool
	}{
		{name: "省略 quantity", raw: `{"goods_id":7}`, quantity: 0, provided: false},
		{name: "显式 0", raw: `{"goods_id":7,"quantity":0}`, quantity: 0, provided: true},
		{name: "负数", raw: `{"goods_id":7,"quantity":-1}`, quantity: -1, provided: true},
		{name: "正常 1", raw: `{"goods_id":7,"quantity":1}`, quantity: 1, provided: true},
		{name: "null 视为已出现", raw: `{"goods_id":7,"quantity":null}`, quantity: 0, provided: true},
	}
	for _, c := range cases {
		var d OrderDetail
		if err := json.Unmarshal([]byte(c.raw), &d); err != nil {
			t.Fatalf("%s: 解码失败 %v", c.name, err)
		}
		if d.Quantity != c.quantity || d.QuantityProvided != c.provided {
			t.Fatalf("%s: got quantity=%d provided=%v, want %d/%v",
				c.name, d.Quantity, d.QuantityProvided, c.quantity, c.provided)
		}
		if d.GoodsId != 7 {
			t.Fatalf("%s: 其它字段被解码逻辑弄丢了: goods_id=%d", c.name, d.GoodsId)
		}
	}
}

// TestOrderRequestDecodeKeepsDetails 顶层自定义解码不能破坏嵌套 details 的字段。
func TestOrderRequestDecodeKeepsDetails(t *testing.T) {
	raw := `{"address_id":123,"quantity":2,"details":[{"goods_id":7,"goods_sku_id":9,"quantity":3}]}`
	var req OrderRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if req.AddressId != 123 || req.Quantity != 2 || !req.QuantityProvided {
		t.Fatalf("顶层字段解码错误: address_id=%d quantity=%d provided=%v",
			req.AddressId, req.Quantity, req.QuantityProvided)
	}
	if len(req.Details) != 1 || req.Details[0].GoodsId != 7 || req.Details[0].GoodsSkuId != 9 || req.Details[0].Quantity != 3 {
		t.Fatalf("details 解码错误: %+v", req.Details)
	}
	if !req.Details[0].QuantityProvided {
		t.Fatal("details 里的 quantity 显式出现，必须被标记")
	}
}
