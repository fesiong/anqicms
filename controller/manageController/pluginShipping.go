package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func PluginGetShippingSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	shippingSetting := currentSite.Shipping

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": shippingSetting,
	})
}

func PluginSaveShippingSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.ShippingConfig

	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
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
