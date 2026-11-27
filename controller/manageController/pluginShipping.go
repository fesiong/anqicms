package manageController

import (
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

// PluginGetShippingSetting 获取运费设置（无参数）。
func PluginGetShippingSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	shippingSetting := currentSite.Shipping

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": shippingSetting,
	})
}

// PluginSaveShippingSetting 保存运费设置（整体覆盖）。
func PluginSaveShippingSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.ShippingConfig
	if !readJSONPayload(ctx, &req) {
		return
	}
	if shippingDeliveryPlaceMissing(&req) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseSelectTheDeliveryPlace"),
		})
		return
	}

	currentSite.Shipping.Units = req.Units
	currentSite.Shipping.VolumetricDivisor = req.VolumetricDivisor
	currentSite.Shipping.WeightUnits = req.WeightUnits
	currentSite.Shipping.Type = req.Type
	currentSite.Shipping.IsFreeShipping = req.IsFreeShipping
	currentSite.Shipping.DeliveryPlace = req.DeliveryPlace
	currentSite.Shipping.FreeShippingRule = req.FreeShippingRule
	currentSite.Shipping.DefaultRule = req.DefaultRule
	currentSite.Shipping.RegionRules = req.RegionRules

	if err := currentSite.SaveSettingValue(provider.ShippingSettingKey, currentSite.Shipping); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}

// shippingDeliveryPlaceMissing 判断自定义运费是否缺少发货地。
// 包邮时发货地不参与运费计算，允许为空；自定义运费必须选择国家/地区。
func shippingDeliveryPlaceMissing(cfg *config.ShippingConfig) bool {
	if cfg == nil || cfg.IsFreeShipping {
		return false
	}
	return strings.TrimSpace(cfg.DeliveryPlace.CountryCode) == ""
}
