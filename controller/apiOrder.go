package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	"github.com/go-pay/gopay/paypal"
	"github.com/go-pay/gopay/wechat"
	"github.com/kataras/iris/v12"
	"github.com/skip2/go-qrcode"
	"github.com/stripe/stripe-go/v82"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func ApiGetOrders(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	status := ctx.URLParam("status")

	userId := ctx.Values().GetUintDefault("userId", 0)

	orders, total := currentSite.GetOrderList(userId, "", "", status, currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  orders,
	})
}

func ApiGetOrdersStatistics(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	// 统计数据，各个数量
	statistic := response.OrderStatistic{
		Total:      0,
		Pending:    0,
		Processing: 0,
		Completed:  0,
		Canceled:   0,
		Refunding:  0,
		Refunded:   0,
		Wishlist:   0,
	}

	userId := ctx.Values().GetUintDefault("userId", 0)

	// 统计当前用户的订单
	currentSite.DB.Model(&model.Order{}).Where("user_id = ?", userId).Count(&statistic.Total)
	currentSite.DB.Model(&model.Order{}).Where("user_id = ? AND status = ?", userId, config.OrderStatusWaiting).Count(&statistic.Pending)
	currentSite.DB.Model(&model.Order{}).Where("user_id = ? AND status (?)", userId, []int{config.OrderStatusPaid, config.OrderStatusDelivering}).Count(&statistic.Processing)
	currentSite.DB.Model(&model.Order{}).Where("user_id = ? AND status = ?", userId, config.OrderStatusCompleted).Count(&statistic.Completed)
	currentSite.DB.Model(&model.Order{}).Where("user_id = ? AND status = ?", userId, config.OrderStatusCanceled).Count(&statistic.Canceled)
	currentSite.DB.Model(&model.Order{}).Where("user_id = ? AND status = ?", userId, config.OrderStatusRefunding).Count(&statistic.Refunding)
	currentSite.DB.Model(&model.Order{}).Where("user_id = ? AND status = ?", userId, config.OrderStatusRefunded).Count(&statistic.Refunded)
	currentSite.DB.Model(&model.Wishlist{}).Where("user_id = ?", userId).Count(&statistic.Wishlist)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": statistic,
	})
}

func ApiGetOrderChartData(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)
	showType := ctx.URLParam("type")
	var tmpResult []response.ChartData
	// 最近30天
	startTime := time.Now().AddDate(0, 0, -30).Unix()
	if showType == "" { // total
		currentSite.DB.Model(&model.Order{}).Where("user_id = ? and created_time >= ? and status not in(?)", userId, startTime, []int{config.OrderStatusCanceled, config.OrderStatusWaiting}).Select("DATE(FROM_UNIXTIME(created_time)) AS `date`, sum(amount) as `value`, 'total' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult)
	} else if showType == "pending" {
		currentSite.DB.Model(&model.Order{}).Where("user_id = ? and created_time >= ? and status = ?", userId, startTime, config.OrderStatusWaiting).Select("DATE(FROM_UNIXTIME(created_time)) AS `date`, sum(amount) as `value`, 'pending' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult)
	} else if showType == "processing" {
		currentSite.DB.Model(&model.Order{}).Where("user_id = ? and created_time >= ? and status (?)", userId, startTime, []int{config.OrderStatusPaid, config.OrderStatusDelivering}).Select("DATE(FROM_UNIXTIME(created_time)) AS `date`, sum(amount) as `value`, 'processing' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult)
	} else if showType == "completed" {
		currentSite.DB.Model(&model.Order{}).Where("user_id = ? and created_time >= ? and status = ?", userId, startTime, config.OrderStatusCompleted).Select("DATE(FROM_UNIXTIME(created_time)) AS `date`, sum(amount) as `value`, 'completed' as `label`").Group("`date`").Order("`date`").Scan(&tmpResult)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": tmpResult,
	})
}

func ApiGetOrderServices(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	services, total := currentSite.GetOrderServices(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("enabled = ?", true)
	}, 1, 100)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  services,
	})
}

func ApiGetOrderDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	orderId := ctx.URLParam("order_id")
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := currentSite.GetOrderInfoByOrderId(orderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.UserId != userId && currentSite.PluginOrder.NoNeedLogin == false {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("InsufficientPermissions"),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": order,
	})
}

func ApiGetOrderCheckout(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	cartToken := ctx.GetCookie("cart")
	if cartToken == "" {
		cartToken = ctx.GetHeader("cart")
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	user, err := currentSite.GetUserInfoById(userId)
	if err != nil {
		user = nil
	}
	if userId > 0 && cartToken != "" {
		// 更新 购物车
		_ = currentSite.DB.Model(&model.Cart{}).Where("token = ?", cartToken).Update("user_id", req.UserId)
	}

	// order id
	if req.OrderId != "" {
		order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		if order.Status != 0 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("OrderHasBeenProcessed"),
			})
			return
		}
		// if order.UserId != userId {
		// 	ctx.JSON(iris.Map{
		// 		"code": config.StatusFailed,
		// 	})
		// }
		details := []response.OrderDetail{}
		for _, detail := range order.Details {
			item := response.OrderDetail{
				GoodsId:     detail.GoodsId,
				GoodsSkuId:  detail.GoodsSkuId,
				Price:       detail.Price,
				OriginPrice: detail.OriginPrice,
				Amount:      detail.OriginAmount,
				RealAmount:  detail.Amount,
				Quantity:    detail.Quantity,
				OrderExtra:  detail.OrderExtra,
				OrderFields: detail.OrderFields,
				SkuValues:   detail.SkuValues,
			}
			if detail.Goods != nil {
				item.Link = detail.Goods.Link
				item.Title = detail.Goods.Title
				item.Thumb = detail.Goods.Thumb
				item.Logo = detail.Goods.Logo
			}
			if detail.GoodsSku != nil {
				item.SkuTitle = detail.GoodsSku.Title
			}
			details = append(details, item)
		}
		orderResp := response.OrderConfirmResponse{
			CurrencyCode:   currentSite.Currency.Currency,
			CurrencySymbol: currentSite.Currency.Symbol,
			PayWays:        currentSite.GetPaymentWays(),
			OrderId:        order.OrderId,
			Amount:         order.Amount,
			OriginAmount:   order.OriginAmount,
			DiscountAmount: order.DiscountAmount,
			CouponCode:     order.CouponCodeId,
			AddressId:      order.AddressId,
			ServiceFee:     order.ServiceFee,
			Details:        details,
		}

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": orderResp,
		})
		return
	}

	var couponCode *model.CouponCode
	// 使用优惠券
	if req.CouponCode != "" {
		couponCode, err = currentSite.GetCouponCodeByCode(req.CouponCode)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else if req.CouponCodeId > 0 {
		couponCode, err = currentSite.GetCouponCodeById(req.CouponCodeId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else if req.CouponId > 0 {
		// 优惠券
		coupon, err := currentSite.GetCouponById(req.CouponId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		// 生成一个优惠券码
		couponCode, err = currentSite.MakeCouponCode(coupon.Id, int64(userId), 1)
		if couponCode == nil {
			if err == nil {
				err = errors.New("fail to get coupon")
			}
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		couponCode.Coupon = coupon
	}

	var amount int64
	var originAmount int64
	var orderResp response.OrderConfirmResponse
	orderResp.CurrencyCode = currentSite.Currency.Currency
	orderResp.CurrencySymbol = currentSite.Currency.Symbol
	orderResp.PayWays = currentSite.GetPaymentWays()

	if len(req.Details) == 0 && req.GoodsId == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseSelectTheProduct"),
		})
		return
	}
	if len(req.Details) == 0 {
		req.Details = []request.OrderDetail{{GoodsId: req.GoodsId, GoodsSkuId: req.GoodsSkuId, Quantity: req.Quantity}}
	}
	var activity *model.Activity
	if req.ActivityId > 0 {
		// 参加活动，只能购买单个商品
		activity, err = currentSite.GetActivityById(req.ActivityId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		for _, detail := range req.Details {
			if detail.GoodsId != activity.GoodsId {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("PleaseSelectTheProduct"),
				})
				return
			}
		}
		if couponCode != nil && activity.CanUseCoupon == false {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("CouponCodeCanNotUse"),
			})
			return
		}
		orderResp.ActivityId = req.ActivityId
	}

	// 如果商品标记了包邮，则不参与计算运费
	var calcQuantity int = 0
	var calcWeight float64 = 0
	var calcVolume float64 = 0

	// VIP优惠
	userDiscount, shippingDiscount := currentSite.GetUserDiscount(userId, user)
	userDiscountAmount := int64(0)

	if req.Type == config.OrderTypeVip {
		group, err := currentSite.GetUserGroupInfo(uint(req.Details[0].GoodsId))
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		//计算价格
		price := group.Price
		originPrice := price
		if activity != nil {
			price = activity.Price
		}
		if userDiscount > 0 {
			price = price * userDiscount / 100
		}
		detailAmount := price * int64(req.Details[0].Quantity)
		originDetailAmount := originPrice * int64(req.Details[0].Quantity)
		userDiscountAmount += originDetailAmount - detailAmount
		amount += detailAmount
		originAmount += originDetailAmount
		orderResp.Details = append(orderResp.Details, response.OrderDetail{
			GoodsId:     req.Details[0].GoodsId,
			GoodsSkuId:  req.Details[0].GoodsSkuId,
			Price:       price,
			OriginPrice: originPrice,
			Amount:      originDetailAmount,
			RealAmount:  detailAmount,
			Quantity:    req.Details[0].Quantity,
			SkuValues:   req.Details[0].SkuValues,
			Title:       group.Title,
			Link:        "",
			Thumb:       "",
			SkuTitle:    "",
		})
	} else if req.Type == config.OrderTypeCommunication {
		communicationOrder, err := currentSite.GetCommunicationOrderById(req.Details[0].GoodsId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		communicationOrder.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		//计算价格
		price := communicationOrder.QuotePrice
		originPrice := price
		if activity != nil {
			price = activity.Price
		}
		if userDiscount > 0 {
			price = price * userDiscount / 100
		}
		detailAmount := price * int64(req.Details[0].Quantity)
		originDetailAmount := originPrice * int64(req.Details[0].Quantity)
		userDiscountAmount += originDetailAmount - detailAmount
		amount += detailAmount
		originAmount += originDetailAmount
		orderResp.Details = append(orderResp.Details, response.OrderDetail{
			GoodsId:     req.Details[0].GoodsId,
			GoodsSkuId:  req.Details[0].GoodsSkuId,
			Price:       price,
			OriginPrice: originPrice,
			Amount:      originDetailAmount,
			RealAmount:  detailAmount,
			SkuValues:   req.Details[0].SkuValues,
			Quantity:    req.Details[0].Quantity,
			Title:       communicationOrder.Title,
			Link:        "",
			Logo:        communicationOrder.Logo,
			Thumb:       communicationOrder.Thumb,
			SkuTitle:    "",
		})
	} else {
		for _, v := range req.Details {
			archive, err := currentSite.GetArchiveById(v.GoodsId)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
			archive.Link = currentSite.GetUrl(provider.PatternArchive, archive, 0)
			archive.GetThumb(currentSite.PluginHtmlCache.StorageUrl, currentSite.Content.DefaultThumb)
			if archive.MinQuantity > 0 && v.Quantity < archive.MinQuantity {
				v.Quantity = archive.MinQuantity
			}
			if archive.MaxQuantity > 0 && v.Quantity > archive.MaxQuantity {
				v.Quantity = archive.MaxQuantity
			}
			//计算价格
			price := archive.Price
			originPrice := price
			if activity != nil {
				price = activity.Price
			}
			skuTitle := ""
			// sku_id
			var archiveSku *model.ArchiveSku
			if v.GoodsSkuId > 0 {
				archiveSku, err = currentSite.GetArchiveSkuById(v.GoodsId, v.GoodsSkuId)
				if err != nil {
					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  err.Error(),
					})
					return
				}
				// 如果数量超了，自动修改为剩余库存
				if archive.AllowOversold == false && int(archiveSku.Stock) < v.Quantity {
					v.Quantity = int(archiveSku.Stock)
				}
				skuTitle = archiveSku.Title
				price = archiveSku.Price
				originPrice = archiveSku.Price
				if activity != nil {
					activitySku, err := currentSite.GetActivitySkuBySkuId(activity.Id, v.GoodsSkuId)
					if err == nil {
						price = activitySku.Price
					}
				}
			} else {
				// 如果数量超了，自动修改为剩余库存
				if archive.AllowOversold == false && int(archive.Stock) < v.Quantity {
					v.Quantity = int(archive.Stock)
				}
			}
			if archive.OptionType == config.OptionTypePrice && len(v.SkuValues) > 0 {
				// 加价属性处理
				var optionPrice int64 = 0
				var originOptionPrice int64 = 0
				for _, sv := range v.SkuValues {
					// 如果option是属性，不需要处理库存
					option, err := currentSite.GetArchiveOption(archive.Id, sv.OptionId)
					if err != nil {
						// 不存在属性
						continue
					}
					// 获取optionValue
					optionValue, err := currentSite.GetArchiveOptionValue(archive.Id, sv.Id)
					if err != nil {
						// 不存在属性值
						continue
					}
					if !option.IsAttribute && archive.AllowOversold == false {
						// 需要处理库存
						if int(optionValue.Stock) < v.Quantity {
							v.Quantity = int(optionValue.Stock)
						}
					}
					if optionValue.Price > 0 {
						optionPrice += optionValue.Price
						originOptionPrice += optionValue.Price
					}
				}
				// 更新价格
				price += optionPrice
				originPrice += originOptionPrice
			}
			if v.Quantity == 0 {
				// 跳过无库存的商品
				continue
			}
			// 批发价格设置
			if archive.ProductType == config.ProductTypeWholesale {
				wholesaleRules, _ := currentSite.GetArchiveWholesaleRules(archive.Id)
				var percentage int64 = 100 // 默认没折扣
				for _, rule := range wholesaleRules {
					if v.Quantity >= rule.MinQuantity {
						percentage = int64(rule.DiscountRate)
						archive.WholesaleRule = rule
					}
				}
				if percentage > 0 {
					if userDiscount == 0 {
						userDiscount = percentage
					} else {
						userDiscount = percentage * userDiscount / 100
					}
				}
			}
			if userDiscount > 0 {
				price = price * userDiscount / 100
			}
			detailAmount := price * int64(v.Quantity)
			originDetailAmount := originPrice * int64(v.Quantity)
			userDiscountAmount += originDetailAmount - detailAmount

			// 计算运费，非包邮商品才计算运费
			weight, volume := 0.0, 0.0
			if archive.NeedLogistics && !archive.IsFreeShipping {
				// 计算重量和体积
				tmpVolume, tmpWeight := currentSite.CalculateShippingAttrs(archive, v.Quantity)
				if archiveSku != nil {
					tmpVolume2, tmpWeight2 := currentSite.CalculateSkuShippingAttrs(archiveSku, v.Quantity)
					if tmpVolume2 > 0 {
						tmpVolume = tmpVolume2
					}
					if tmpWeight2 > 0 {
						tmpWeight = tmpWeight2
					}
				}
				weight = tmpWeight
				volume = tmpVolume
				calcWeight += weight
				calcQuantity += v.Quantity
				calcVolume += volume
			}

			amount += detailAmount
			originAmount += originDetailAmount
			orderResp.Details = append(orderResp.Details, response.OrderDetail{
				GoodsId:       v.GoodsId,
				GoodsSkuId:    v.GoodsSkuId,
				Price:         price,
				OriginPrice:   originPrice,
				Amount:        originDetailAmount,
				RealAmount:    detailAmount,
				Quantity:      v.Quantity,
				Weight:        weight,
				Volume:        volume,
				Title:         archive.Title,
				Logo:          archive.Logo,
				Thumb:         archive.Thumb,
				SkuValues:     v.SkuValues,
				SkuTitle:      skuTitle,
				Link:          archive.Link,
				WholesaleRule: archive.WholesaleRule,
				OrderExtra:    v.OrderExtra,
				OrderFields:   archive.OrderFields,
			})
		}
	}
	orderResp.Amount = amount
	orderResp.OriginAmount = originAmount
	// 用户优惠金额
	orderResp.UserDiscount = userDiscountAmount
	if couponCode != nil {
		if couponCode.Status != 0 {
			//ctx.JSON(iris.Map{
			//	"code": config.StatusFailed,
			//	"msg":  ctx.Tr("CouponCodeUsed"),
			//})
			//return
			orderResp.CouponCode = ""
			orderResp.CouponCodeId = 0
			orderResp.CouponId = 0
		} else {
			couponMoney := couponCode.GetMoney(orderResp.Amount)
			orderResp.CouponDiscount = couponMoney
			orderResp.CouponCode = couponCode.Code
			orderResp.CouponCodeId = couponCode.Id
			orderResp.CouponId = couponCode.CouponId
			orderResp.Amount = orderResp.Amount - couponMoney
		}
	}

	if calcQuantity > 0 {
		orderResp.ShippingQuantity = calcQuantity
		orderResp.ShippingWeight = calcWeight
		orderResp.ShippingVolume = calcVolume
		// 计算运费
		var orderAddress *model.OrderAddress
		if req.Address != nil {
			orderAddress = &model.OrderAddress{
				CountryCode:  req.Address.CountryCode,
				Country:      req.Address.Country,
				ProvinceCode: req.Address.ProvinceCode,
				Province:     req.Address.Province,
				City:         req.Address.City,
				Postcode:     req.Address.Postcode,
			}
		} else if req.AddressId > 0 {
			orderAddress, _ = currentSite.GetOrderAddressById(req.AddressId)
		}
		orderResp.ShippingFee = currentSite.CalculateShippingFee(orderAddress, orderResp.Amount, calcQuantity, calcWeight, calcVolume)
		if shippingDiscount > 0 {
			originShippingFee := orderResp.ShippingFee
			orderResp.ShippingFee = int64(float64(originShippingFee) * (float64(shippingDiscount) / 100))
			orderResp.ShippingDiscount = originShippingFee - orderResp.ShippingFee
		}
		// 累计优惠金额
		orderResp.DiscountAmount = orderResp.UserDiscount + orderResp.CouponDiscount + orderResp.ShippingDiscount
		// 更新订单金额
		orderResp.Amount += orderResp.ShippingFee
	}

	// orderService
	if len(req.ServiceIds) > 0 {
		services, _ := currentSite.GetOrderServices(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("id in (?)", req.ServiceIds)
		}, 1, len(req.ServiceIds))
		var serviceFee int64
		for _, service := range services {
			if service.FeeType == config.OrderServiceTypePercentage {
				// 订单金额的百分比
				tmpFee := int64(float64(orderResp.Amount) * (float64(service.Fee) / 100))
				if service.MaxFee > 0 && tmpFee > service.MaxFee {
					tmpFee = service.MaxFee
				}
				serviceFee += tmpFee
			} else {
				serviceFee += service.Fee
			}
		}
		orderResp.ServiceIds = req.ServiceIds
		orderResp.ServiceFee = serviceFee
		orderResp.Amount += serviceFee
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": orderResp,
	})
}

func ApiCreateOrder(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// check login
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId == 0 && currentSite.PluginOrder.NoNeedLogin == false {
		ctx.JSON(iris.Map{
			"code": config.StatusNoLogin,
			"msg":  currentSite.TplTr("PleaseLogin"),
		})
		return
	}

	order, err := currentSite.CreateOrder(userId, &req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": order,
	})
}

func ApiCancelOrder(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.UserId != userId {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("TheOrderIsNotOperational"),
		})
		return
	}

	err = currentSite.SetOrderCanceled(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("OrderCanceled"),
	})
}

func ApiApplyRefundOrder(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.UserId != userId {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("TheOrderIsNotOperational"),
		})
		return
	}

	err = currentSite.ApplyOrderRefund(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("RefundApplicationSubmitted"),
	})
}

func ApiFinishedOrder(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.UserId != userId {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("TheOrderIsNotOperational"),
		})
		return
	}

	err = currentSite.SetOrderFinished(order)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("OrderCompleted"),
	})
}

func ApiGetOrderAddresses(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)

	addresses, err := currentSite.GetOrderAddressesByUserId(userId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  nil,
		"data": addresses,
	})
}

func ApiGetOrderAddress(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)

	address, err := currentSite.GetOrderAddressByUserId(userId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  nil,
		"data": address,
	})
}

func ApiSaveOrderAddress(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderAddressRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)

	address, err := currentSite.SaveOrderAddress(currentSite.DB, userId, &req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  nil,
		"data": address,
	})
}

func ApiCreateOrderPayment(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PaymentRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId == 0 && currentSite.PluginOrder.NoNeedLogin == false {
		ctx.JSON(iris.Map{
			"code": config.StatusNoLogin,
			"msg":  currentSite.TplTr("PleaseLogin"),
		})
		return
	}
	//注入userID
	req.UserId = userId
	if req.PayWay == "" {
		payWays := currentSite.GetPaymentWays()
		if len(payWays) > 0 {
			req.PayWay = payWays[0]
		} else {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("UnableToCreatePaymentOrder"),
			})
			return
		}
	}

	order, err := currentSite.GetOrderInfoByOrderId(req.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	payment, err := currentSite.GeneratePayment(order, &req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 推送事件
	currentSite.CreateEvent(provider.EventOrderCheckout, map[string]interface{}{
		"order_id": order.OrderId,
		"amount":   float32(order.Amount) / 100,
		"status":   "pending",
	}, &model.User{Id: uint(req.UserId)})

	if req.PayWay == config.PayWayWechat {
		createWechatPayment(ctx, payment)
	} else if req.PayWay == config.PayWayWeapp {
		createWeappPayment(ctx, payment)
	} else if req.PayWay == config.PayWayAlipay {
		createAlipayPayment(ctx, payment)
	} else if req.PayWay == config.PayWayPaypal {
		createPaypalPayment(ctx, payment, order)
	} else if req.PayWay == config.PayWayStripe {
		createStripePayment(ctx, payment, order)
	} else if req.PayWay == config.PayWayBalance {
		createBalancePayment(ctx, payment, order)
	} else {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UnableToCreatePaymentOrder"),
		})
	}
}

func createWechatPayment(ctx iris.Context, payment *model.Payment) {
	currentSite := provider.CurrentSite(ctx)
	//根据订单生成支付信息
	account := currentSite.GetSingleValidPaymentAccount(config.PayWayWechat, payment.Amount)
	if account == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UnableToCreatePaymentOrder"),
		})
		return
	}
	//生成支付信息
	client := wechat.NewClient(account.PayConfig.AppId, account.PayConfig.Account, account.PayConfig.ApiKey, !account.PayConfig.Sandbox)

	bm := make(gopay.BodyMap)
	bm.Set("body", payment.Remark).
		Set("nonce_str", library.GenerateRandString(32)).
		Set("spbill_create_ip", ctx.RemoteAddr()).
		Set("out_trade_no", payment.PaymentId). // 传的是paymentID，因此notify的时候，需要处理paymentID
		Set("total_fee", payment.Amount).
		Set("trade_type", wechat.TradeType_Native).
		Set("notify_url", currentSite.System.BaseUrl+"/notify/wechat/pay").
		Set("sign_type", wechat.SignType_MD5)

	wxRsp, err := client.UnifiedOrder(context.Background(), bm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if wxRsp.ReturnCode != gopay.SUCCESS {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  wxRsp.ReturnMsg,
		})
		return
	}

	if wxRsp.ResultCode != gopay.SUCCESS {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  wxRsp.ErrCodeDes,
		})
		return
	}
	currentSite.DB.Model(payment).UpdateColumn("payment_account_id", account.Id)

	png, _ := qrcode.Encode(wxRsp.CodeUrl, qrcode.Medium, 256)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"pay_way":  config.PayWayWechat,
			"code_url": fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(png)),
		},
	})
	return
}

func createWeappPayment(ctx iris.Context, payment *model.Payment) {
	currentSite := provider.CurrentSite(ctx)
	//根据订单生成支付信息
	account := currentSite.GetSingleValidPaymentAccount(config.PayWayWeapp, payment.Amount)
	if account == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UnableToCreatePaymentOrder"),
		})
		return
	}
	//生成支付信息
	client := wechat.NewClient(account.PayConfig.AppId, account.PayConfig.Account, account.PayConfig.ApiKey, !account.PayConfig.Sandbox)

	userWechat, err := currentSite.GetUserWechatByUserId(payment.UserId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	bm := make(gopay.BodyMap)
	bm.Set("body", payment.Remark).
		Set("nonce_str", library.GenerateRandString(32)).
		Set("spbill_create_ip", ctx.RemoteAddr()).
		Set("out_trade_no", payment.PaymentId). // 传的是paymentID，因此notify的时候，需要处理paymentID
		Set("total_fee", payment.Amount).
		Set("trade_type", wechat.TradeType_Mini).
		Set("notify_url", currentSite.System.BaseUrl+"/notify/wechat/pay").
		Set("sign_type", wechat.SignType_MD5).
		Set("openid", userWechat.Openid)

	wxRsp, err := client.UnifiedOrder(context.Background(), bm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if wxRsp.ReturnCode != gopay.SUCCESS {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  wxRsp.ReturnMsg,
		})
		return
	}

	if wxRsp.ResultCode != gopay.SUCCESS {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  wxRsp.ErrCodeDes,
		})
		return
	}
	currentSite.DB.Model(payment).UpdateColumn("payment_account_id", account.Id)

	// 微信小程序支付需要 paySign
	timeStamp := strconv.FormatInt(time.Now().Unix(), 10)
	packages := "prepay_id=" + wxRsp.PrepayId
	paySign := wechat.GetMiniPaySign(account.PayConfig.AppId, wxRsp.NonceStr, packages, wechat.SignType_MD5, timeStamp, account.PayConfig.ApiKey)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"pay_way":   config.PayWayWeapp,
			"paySign":   paySign,
			"timeStamp": timeStamp,
			"package":   packages,
			"nonceStr":  wxRsp.NonceStr,
			"signType":  wechat.SignType_MD5,
		},
	})
	return
}

func createAlipayPayment(ctx iris.Context, payment *model.Payment) {
	currentSite := provider.CurrentSite(ctx)
	account := currentSite.GetSingleValidPaymentAccount(config.PayWayAlipay, payment.Amount)
	if account == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UnableToCreatePaymentOrder"),
		})
		return
	}
	client, err := alipay.NewClient(account.PayConfig.AppId, account.PayConfig.AppSecret, !account.PayConfig.Sandbox)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	frontUrl := currentSite.System.BaseUrl
	if currentSite.System.FrontUrl != "" {
		frontUrl = currentSite.System.FrontUrl
	}
	//配置公共参数
	client.SetCharset("utf-8").
		SetSignType(alipay.RSA2).
		SetNotifyUrl(currentSite.System.BaseUrl + "/notify/alipay/pay").
		SetReturnUrl(frontUrl + "/")

	// 自动同步验签（只支持证书模式）
	certPath := currentSite.DataPath + "cert/" + account.PayConfig.CertPath
	rootCertPath := currentSite.DataPath + "cert/" + account.PayConfig.RootCertPath
	publicCertPath := currentSite.DataPath + "cert/" + account.PayConfig.PublicCertPath
	publicKey, err := os.ReadFile(publicCertPath)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	client.AutoVerifySign(publicKey)

	// 传入证书内容
	err = client.SetCertSnByPath(certPath, rootCertPath, publicCertPath)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//请求参数
	bm := make(gopay.BodyMap)

	bm.Set("subject", payment.Remark)
	bm.Set("out_trade_no", payment.PaymentId)
	bm.Set("total_amount", fmt.Sprintf("%.2f", float32(payment.Amount)/100))

	//创建订单
	payUrl, err := client.TradePagePay(context.Background(), bm)

	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.DB.Model(payment).UpdateColumn("payment_account_id", account.Id)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"pay_way":  config.PayWayAlipay,
			"jump_url": payUrl,
		},
	})
	return
}

func createPaypalPayment(ctx iris.Context, payment *model.Payment, order *model.Order) {
	currentSite := provider.CurrentSite(ctx)
	account := currentSite.GetSingleValidPaymentAccount(config.PayWayPaypal, payment.Amount)
	if account == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UnableToCreatePaymentOrder"),
		})
		return
	}
	client, err := paypal.NewClient(account.PayConfig.AppId, account.PayConfig.AppSecret, !account.PayConfig.Sandbox)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var purchases = []*paypal.PurchaseUnit{
		{
			ReferenceId: payment.PaymentId,
			Amount: &paypal.Amount{
				CurrencyCode: "USD",
				Value:        fmt.Sprintf("%.2f", float64(payment.Amount)/100),
			},
		},
	}
	frontUrl := currentSite.System.BaseUrl
	if currentSite.System.FrontUrl != "" {
		frontUrl = currentSite.System.FrontUrl
	}
	bm := make(gopay.BodyMap)
	bm.Set("intent", "CAPTURE").
		Set("purchase_units", purchases).
		SetBodyMap("payment_source", func(b gopay.BodyMap) {
			b.SetBodyMap("paypal", func(bb gopay.BodyMap) {
				bb.SetBodyMap("experience_context", func(bbb gopay.BodyMap) {
					bbb.Set("brand_name", currentSite.System.SiteName).
						Set("locale", "en-US").
						Set("shipping_preference", "NO_SHIPPING").
						Set("user_action", "PAY_NOW").
						Set("return_url", frontUrl+"/return/paypal/pay").
						Set("cancel_url", frontUrl+"/return/paypal/cancel")
				})
			})
		})

	ppRsp, err := client.CreateOrder(ctx, bm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if ppRsp.ErrorResponse != nil {
		// do something
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ppRsp.Error,
		})
		return
	}
	currentSite.DB.Model(payment).UpdateColumn("payment_account_id", account.Id)
	// 更新id
	payment.TerraceId = ppRsp.Response.Id
	currentSite.DB.Save(payment)

	// payer link

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"pay_way":  config.PayWayPaypal,
			"jump_url": ppRsp.Response.Links[1].Href,
		},
	})
}

func createStripePayment(ctx iris.Context, payment *model.Payment, order *model.Order) {
	currentSite := provider.CurrentSite(ctx)
	account := currentSite.GetSingleValidPaymentAccount(config.PayWayStripe, payment.Amount)
	if account == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UnableToCreatePaymentOrder"),
		})
		return
	}
	client := stripe.NewClient(account.PayConfig.AppId)

	params := &stripe.PaymentIntentCreateParams{
		Amount:      stripe.Int64(payment.Amount),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String(payment.Remark),
		AutomaticPaymentMethods: &stripe.PaymentIntentCreateAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
	}
	params.AddMetadata("payment_id", payment.PaymentId)

	pi, err := client.V1PaymentIntents.Create(ctx, params)
	if err != nil {
		// 处理创建失败
		log.Println("stripe payment intent create error", err.Error())
		return
	}
	// 更新id
	payment.PaymentAccountId = account.Id
	payment.TerraceId = pi.ID
	currentSite.DB.Save(payment)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"pay_way":       config.PayWayStripe,
			"amount":        payment.Amount,
			"client_secret": pi.ClientSecret,
			"public_key":    account.PayConfig.AppSecret,
			"payment_id":    pi.ID,
		},
	})
}

func createBalancePayment(ctx iris.Context, payment *model.Payment, order *model.Order) {
	currentSite := provider.CurrentSite(ctx)
	// 检查用户余额是否足够
	user, err := currentSite.GetUserInfoById(payment.UserId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("UnableToCreatePaymentOrder"),
		})
		return
	}
	if user.Balance < payment.Amount {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("BalanceNotEnough"),
		})
		return
	}
	tx := currentSite.DB.Begin()
	// this is a pay order
	payment.PayWay = config.PayWayBalance
	payment.PaidTime = time.Now().Unix()
	payment.TerraceId = strconv.FormatInt(payment.PaidTime, 10)
	tx.Save(payment)
	order.PaymentId = payment.PaymentId
	tx.Save(order)

	//生成用户支付记录
	tx.Model(user).Where("id = ?", user.Id).UpdateColumn("balance", gorm.Expr("`balance` - ?", payment.Amount))
	var userBalance int64
	tx.Model(&model.User{}).Where("`id` = ?", payment.UserId).Pluck("balance", &userBalance)
	//状态更改了，增加一条记录到用户
	finance := model.Finance{
		UserId:      payment.UserId,
		Direction:   config.FinanceOutput,
		Amount:      payment.Amount,
		AfterAmount: userBalance,
		Action:      config.FinanceActionBuy,
		OrderId:     payment.OrderId,
		Status:      1,
	}
	err = tx.Create(&finance).Error
	if err != nil {
		tx.Rollback()
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PaymentFailed"),
		})
		return
	}
	tx.Commit()

	//支付成功逻辑处理
	err = currentSite.SuccessPaidOrder(order, payment)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PaymentFailed"),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("PaymentSuccessful"),
		"data": iris.Map{
			"payment_id": payment.PaymentId,
			"order_id":   order.OrderId,
			"status":     1,
		},
	})
}

func ApiPaymentCheck(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	orderId := ctx.URLParam("order_id")
	order, err := currentSite.GetOrderInfoByOrderId(orderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if order.Status != config.OrderStatusWaiting {
		//支付成功
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  currentSite.TplTr("PaymentSuccessful"),
		})
		return
	}
	// 检查一次订单是否支付成功
	payment, err := currentSite.GetPaymentInfoByOrderId(order.OrderId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	_ = currentSite.TraceQuery(payment)
	if payment.PaidTime > 0 {
		//支付成功
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  currentSite.TplTr("PaymentSuccessful"),
		})
		return
	}

	for i := 0; i < 6; i++ {
		order, _ = currentSite.GetOrderInfoByOrderId(orderId)
		if order.Status != config.OrderStatusWaiting {
			//支付成功
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  currentSite.TplTr("PaymentSuccessful"),
			})
			return
		}
		time.Sleep(1 * time.Second)
	}
	// 5秒内未支付成功，并且订单已经下单1分钟以上，则尝试查询一次接口检查支付情况
	if time.Now().Unix()-payment.CreatedTime > 60 {
		_ = currentSite.TraceQuery(payment)
		if payment.PaidTime > 0 {
			//支付成功
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  currentSite.TplTr("PaymentSuccessful"),
			})
			return
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  currentSite.TplTr("Unpaid"),
	})
}

func ApiArchiveOrderCheck(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := ctx.URLParamInt64Default("id", 0)
	userId := ctx.Values().GetUintDefault("userId", 0)

	archiveDetail, err := currentSite.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": false,
		})
	}
	userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
	archiveDetail = currentSite.CheckArchiveHasOrder(userId, archiveDetail, userGroup)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": archiveDetail.HasOrdered,
	})
	return
}

func ApiCheckArchivePassword(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.ArchivePasswordRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		req.Id = ctx.PostValueInt64Default("id", 0)
		req.Password = ctx.PostValueTrim("password")
		if req.Id == 0 || len(req.Password) == 0 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}

	archiveDetail, err := currentSite.GetArchiveById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if len(archiveDetail.Password) == 0 || archiveDetail.Password == req.Password {
		var content string
		archiveData, err := currentSite.GetArchiveDataById(archiveDetail.Id)
		if err == nil {
			content = archiveData.Content
			// render
			if currentSite.Content.Editor == "markdown" {
				frontUrl := currentSite.System.BaseUrl
				if currentSite.System.FrontUrl != "" {
					frontUrl = currentSite.System.FrontUrl
				}
				content = library.MarkdownToHTML(archiveData.Content, frontUrl, currentSite.Content.FilterOutlink)
			}
		}
		detail := library.StructToMap(archiveDetail)
		detail["content"] = content
		detail["status"] = true
		delete(detail, "password")
		// extra
		archiveParams := currentSite.GetArchiveExtra(archiveDetail.ModuleId, archiveDetail.Id, true)
		for i := range archiveParams {
			if (archiveParams[i].Value == nil || archiveParams[i].Value == "") &&
				archiveParams[i].Type != config.CustomFieldTypeRadio &&
				archiveParams[i].Type != config.CustomFieldTypeCheckbox &&
				archiveParams[i].Type != config.CustomFieldTypeSelect {
				archiveParams[i].Value = archiveParams[i].Default
			}
			if archiveParams[i].FollowLevel && !archiveDetail.HasOrdered {
				delete(archiveParams, i)
			}
		}
		detail["extra"] = archiveParams
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": detail,
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"status":  false,
			"content": "",
		},
	})
}

func ApiReviewPublish(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderReviewRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	var user *model.User
	if userId > 0 {
		user, err = currentSite.GetUserInfoById(userId)
		if err == nil {
			req.UserName = user.RealName
			req.Email = user.Email
		}
	}
	var order *model.Order
	if req.OrderId != "" {
		order, err = currentSite.GetOrderInfoByOrderId(req.OrderId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		if order.UserId != userId {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("NoPermission"),
			})
			return
		}
		if order.HasReview == 1 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.TplTr("OrderStatusError"),
			})
			return
		}
	}

	if req.Score == 0 {
		req.Score = 5
	}

	review := request.OrderReviewRequest{
		UserId:       userId,
		OrderId:      req.OrderId,
		ArchiveId:    req.ArchiveId,
		ArchiveSkuId: req.ArchiveSkuId,
		Ip:           ctx.RemoteAddr(),
		UserName:     req.UserName,
		Email:        req.Email,
		Score:        req.Score,
		Content:      req.Content,
		Images:       req.Images,
		Status:       0,
	}
	if userId > 0 {
		review.Status = 1
	}

	if order != nil {
		for _, v := range order.Details {
			newItem := review
			newItem.ArchiveId = v.GoodsId
			newItem.ArchiveSkuId = v.GoodsSkuId
			err = currentSite.SaveOrderReview(&newItem)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
		}
	} else {
		err = currentSite.SaveOrderReview(&review)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "OK",
	})
}

func ApiReviewPraise(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.OrderReviewRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	userId := ctx.Values().GetIntDefault("userId", 0)
	review, err := currentSite.GetOrderReview(uint(req.Id))
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 检查是否点赞过
	_, err = currentSite.AddReviewPraise(uint(userId), int64(review.Id), review.ArchiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	review.VoteCount += 1
	review.Active = true

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("LikeSuccessfully"),
		"data": review,
	})
}

func ApiGetShippingInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": currentSite.Shipping,
	})
}
