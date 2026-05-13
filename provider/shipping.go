package provider

import (
	"fmt"
	"math"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

// CalculateVolumeFromDimensions 根据用户填写的长宽高和单位，计算体积（m³）
func CalculateVolumeFromDimensions(length, width, height float64, ratio float64) float64 {
	if length <= 0 || width <= 0 || height <= 0 {
		return 0
	}

	// 转换为米
	lM, wM, hM := length*ratio, width*ratio, height*ratio

	// 计算体积（m³）
	volume := lM * wM * hM

	// 保留 6 位小数
	return math.Round(volume*1000000) / 1000000
}

// ==================== 商品运费属性计算 ====================

// CalculateShippingAttrs 计算商品的运费相关属性（体积、体积重）
// 在检查订单的时候调用
func (w *Website) CalculateShippingAttrs(product *model.Archive, quantity int) (volume, volWeight float64) {
	if product.Weight > 0 {
		weightUnit := w.Shipping.GetWeightUnit(product.WeightUnit)
		volWeight = product.Weight * weightUnit.Ratio
	}
	volume = product.Volume
	// 1. 计算体积（m³）
	if product.Length > 0 && product.Width > 0 && product.Height > 0 {
		// 获取转换系数
		unit := w.Shipping.GetLengthUnit(product.LengthUnit)
		volume = CalculateVolumeFromDimensions(product.Length, product.Width, product.Height, unit.Ratio)
		// 计算体积重
		if w.Shipping.VolumetricDivisor > 0 {
			// 转换成立方里面计算
			volWeight = volume * 1000000 / float64(w.Shipping.VolumetricDivisor)
		}
	} else if volume > 0 && w.Shipping.VolumetricDivisor > 0 {
		// 转换成立方里面计算
		volWeight = volume * 1000000 / float64(w.Shipping.VolumetricDivisor)
	}

	// 计算总重量和体积
	volume = volume * float64(quantity)
	volWeight = volWeight * float64(quantity)

	return volume, volWeight
}

// CalculateSkuShippingAttrs 计算 SKU 的运费相关属性
// 在检查订单的时候调用
func (w *Website) CalculateSkuShippingAttrs(sku *model.ArchiveSku, quantity int) (volume, volWeight float64) {
	if sku.Weight > 0 {
		weightUnit := w.Shipping.GetWeightUnit(sku.WeightUnit)
		volWeight = sku.Weight * weightUnit.Ratio
	}
	volume = sku.Volume
	// 1. 计算体积（m³）
	if sku.Length > 0 && sku.Width > 0 && sku.Height > 0 {
		// 获取转换系数
		unit := w.Shipping.GetLengthUnit(sku.LengthUnit)
		volume = CalculateVolumeFromDimensions(sku.Length, sku.Width, sku.Height, unit.Ratio)
		// 计算体积重
		if w.Shipping.VolumetricDivisor > 0 {
			// 转换成立方里面计算
			volWeight = volume * 1000000 / float64(w.Shipping.VolumetricDivisor)
		}
	} else if volume > 0 && w.Shipping.VolumetricDivisor > 0 {
		// 转换成立方里面计算
		volWeight = volume * 1000000 / float64(w.Shipping.VolumetricDivisor)
	}

	// 计算总重量和体积
	volume = volume * float64(quantity)
	volWeight = volWeight * float64(quantity)

	return volume, volWeight
}

// CalculateShippingFee 计算运费（基于规则配置）
// 需要返回单位为分的运费
func (w *Website) CalculateShippingFee(address *model.OrderAddress, orderAmount int64, quantity int, weight float64, volume float64) int64 {
	// 如果是免运费，则直接返回0
	if w.Shipping.IsFreeShipping {
		return 0
	}
	// 如果启用了满额包邮，则先判断
	if w.Shipping.FreeShippingRule.Type > 0 {
		if w.Shipping.FreeShippingRule.Type == 1 {
			// 金额满n包邮
			if float64(orderAmount) >= w.Shipping.FreeShippingRule.Threshold*100 {
				return 0
			}
		} else if w.Shipping.FreeShippingRule.Type == 2 {
			// 重量满n包邮
			if weight >= w.Shipping.FreeShippingRule.Threshold {
				return 0
			}
		} else if w.Shipping.FreeShippingRule.Type == 3 {
			// 体积满n包邮
			if volume >= w.Shipping.FreeShippingRule.Threshold {
				return 0
			}
		}
	}
	// 继续匹配规则
	if address == nil {
		address = &model.OrderAddress{}
	}

	// 匹配运费规则
	rule := w.Shipping.DefaultRule
	if len(w.Shipping.RegionRules) > 0 {
		var matched = false
		for _, tmpRule := range w.Shipping.RegionRules {
			for _, region := range tmpRule.Regions {
				if (address.CountryCode != "" && region.CountryCode == address.CountryCode) || (address.Country != "" && region.Country == address.Country) {
					// 匹配到了国家，接着匹配省份
					if (address.ProvinceCode != "" && region.ProvinceCode == address.ProvinceCode) || (address.Province != "" && region.Province == address.Province) {
						if region.City == address.City {
							matched = true
							rule = tmpRule
							break
						}
						if !matched {
							rule = tmpRule
						}
					}
					if !matched {
						rule = tmpRule
					}
				}
			}
		}
	}

	switch w.Shipping.Type {
	case config.ShippingTypeWeight:
		// 按重量计算
		if weight <= rule.FirstUnit {
			return rule.FirstFee
		}
		// 续重费用
		continueWeight := weight - rule.FirstUnit
		continueCount := int64(math.Ceil(continueWeight / rule.ContinueUnit))
		return rule.FirstFee + continueCount*rule.ContinueFee

	case config.ShippingTypeVolume:
		// 按体积计算
		if volume <= rule.FirstUnit {
			return rule.FirstFee
		}
		continueVolume := volume - rule.FirstUnit
		continueCount := int64(math.Ceil(continueVolume / rule.ContinueUnit))
		return rule.FirstFee + continueCount*rule.ContinueFee
	case config.ShippingTypeAmount:
		// 按体积计算
		amount := float64(orderAmount) / 100
		if amount <= rule.FirstUnit {
			return rule.FirstFee
		}
		continueAmount := amount - rule.FirstUnit
		continueCount := int64(math.Ceil(continueAmount / rule.ContinueUnit))
		return rule.FirstFee + continueCount*rule.ContinueFee

	default:
		return rule.FirstFee
	}
}

// ==================== 便捷函数 ====================

// GetVolumeUnitFromLengthUnit 根据长度单位推导体积单位
func GetVolumeUnitFromLengthUnit(lengthUnit string) string {
	switch lengthUnit {
	case "cm":
		return "cm³"
	case "inch", "in":
		return "in³"
	default: // m 或空
		return "m³"
	}
}

// FormatShippingInfo 格式化运费信息用于展示
func FormatShippingInfo(weight float64, volume float64, volWeight float64, unit string) string {
	var parts []string
	if weight > 0 {
		parts = append(parts, fmt.Sprintf("%.2f%s", weight, unit))
	}
	if volume > 0 {
		parts = append(parts, fmt.Sprintf("%.6fm³", volume))
	}
	if volWeight > 0 && volWeight != weight {
		parts = append(parts, fmt.Sprintf("体积重:%.2f%s", volWeight, unit))
	}

	if len(parts) == 0 {
		return "0.00"
	}

	return fmt.Sprintf("[%s]", parts[0])
}
