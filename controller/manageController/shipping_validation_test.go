package manageController

import (
	"testing"

	"kandaoni.com/anqicms/config"
)

// 自定义运费必须选发货地；包邮时发货地不参与计算，允许为空。
// 过去前端空选择返回 {country_code:”,country:”} 这个真值对象，
// 绕过 required 规则，后端也不校验，于是能保存缺少发货地的规则。
func TestShippingDeliveryPlaceMissing(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.ShippingConfig
		want bool
	}{
		{
			name: "包邮时忽略空发货地",
			cfg:  config.ShippingConfig{IsFreeShipping: true},
			want: false,
		},
		{
			name: "自定义运费但未选发货地",
			cfg:  config.ShippingConfig{IsFreeShipping: false},
			want: true,
		},
		{
			name: "自定义运费且已选发货地",
			cfg: config.ShippingConfig{
				IsFreeShipping: false,
				DeliveryPlace:  config.PlaceItem{CountryCode: "CN"},
			},
			want: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := shippingDeliveryPlaceMissing(&tt.cfg); got != tt.want {
				t.Errorf("shippingDeliveryPlaceMissing() = %v，期望 %v", got, tt.want)
			}
		})
	}
}
