package controller

import (
	"errors"
	"time"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func ApiGetCouponCodes(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	status := ctx.URLParam("status")
	userId := ctx.Values().GetUintDefault("userId", 0)

	couponCodes, total := currentSite.GetCouponCodes(func(tx *gorm.DB) *gorm.DB {
		tx = tx.Where("user_id = ?", userId)
		if status != "" {
			tx = tx.Where("status = ?", status)
		} else {
			// 默认是待使用
			tx = tx.Where("status = ? and end_time > ?", config.CouponStatusUnused, time.Now().Unix())
		}
		return tx
	}, "", currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  couponCodes,
	})
}

func ApiGetValidCoupons(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	// 如果传递了金额，则返回该金额下可用的优惠券
	amount := ctx.URLParamInt64Default("amount", 0)
	archiveId := ctx.URLParamInt64Default("archive_id", 0)
	// 如果传递了用户ID，则返回该用户下的优惠券，否则只返回满减券
	userId := ctx.Values().GetUintDefault("userId", 0)

	var validCodes []*model.CouponCode
	coupons, _ := currentSite.GetCoupons(func(tx *gorm.DB) *gorm.DB {
		tx = tx.Where("`type` = ? and status = ? and end_time > ?", config.CouponTypePromotion, config.CouponStatusUsing, time.Now().Unix())
		return tx
	}, "", 1, 100)
	for _, coupon := range coupons {
		if amount > 0 && coupon.NeedMoney > 0 && coupon.NeedMoney > amount {
			continue
		}
		valid := currentSite.CheckCouponValidByProductId(coupon, archiveId)
		if !valid {
			continue
		}
		couponCode := &model.CouponCode{
			Id:          0,
			CouponId:    coupon.Id,
			StartTime:   coupon.StartTime,
			EndTime:     coupon.EndTime,
			UserId:      int64(userId),
			Status:      0,
			Money:       coupon.Money,
			MoneyType:   coupon.MoneyType,
			MaxMoney:    coupon.MaxMoney,
			Title:       coupon.Title,
			Description: coupon.Description,
			Type:        coupon.Type,
			Coupon:      coupon,
		}
		validCodes = append(validCodes, couponCode)
	}

	if userId > 0 {
		couponCodes, _ := currentSite.GetCouponCodes(func(tx *gorm.DB) *gorm.DB {
			tx = tx.Where("user_id = ?", userId)
			// 默认是待使用
			tx = tx.Where("status = ? and end_time > ?", config.CouponStatusUnused, time.Now().Unix())
			return tx
		}, "", 1, 100)
		for _, couponCode := range couponCodes {
			coupon, err := currentSite.GetCouponById(couponCode.CouponId)
			if err != nil {
				continue
			}
			if amount > 0 && coupon.NeedMoney > 0 && coupon.NeedMoney < amount {
				continue
			}
			valid := currentSite.CheckCouponValidByProductId(coupon, archiveId)
			if !valid {
				continue
			}
			validCodes = append(validCodes, couponCode)
		}
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": len(validCodes),
		"data":  validCodes,
	})
}

func ApiGenerateCouponCode(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CouponRequest
	err := ctx.ReadJSON(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	couponCode, err := currentSite.MakeCouponCode(req.Id, int64(userId), 1)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "success",
		"data": couponCode,
	})
}

// 验证优惠券是否可用，是否过期，是否已被使用
// 如果传了商品的checkout信息，则验证优惠券是否可用这些商品，以及订单金额是否符合优惠券要求
func ApiCheckCoupon(ctx iris.Context) {
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
	user, err := currentSite.GetUserInfoById(userId)
	if err != nil {
		user = nil
	}

	var couponCode *model.CouponCode
	// 使用优惠券
	if req.CouponCode != "" {
		couponCode, err = currentSite.GetCouponCodeByCode(req.CouponCode)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("CouponCodeCanNotUse"),
			})
			return
		}
	} else if req.CouponCodeId > 0 {
		couponCode, err = currentSite.GetCouponCodeById(req.CouponCodeId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("CouponCodeCanNotUse"),
			})
			return
		}
	} else if req.CouponId > 0 {
		// 优惠券
		coupon, err := currentSite.GetCouponById(req.CouponId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("CouponCodeCanNotUse"),
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
				"msg":  ctx.Tr("CouponCodeCanNotUse"),
			})
			return
		}
		couponCode.Coupon = coupon
	}
	// 验证优惠券是否可用
	if couponCode == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("CouponCodeCanNotUse"),
		})
		return
	}
	// 验证优惠券是否过期，是否已被使用
	if couponCode.Status != 0 || couponCode.EndTime < time.Now().Unix() {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("CouponCodeCanNotUse"),
		})
		return
	}

	if len(req.Details) == 0 && req.GoodsId == 0 {
		// 没有商品，则返回优惠券可用
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("CouponCodeValid"),
			"data": iris.Map{
				"coupon":       couponCode,
				"coupon_money": couponCode.Money,
			},
		})
		return
	}

	// 传了商品，则对商品进行验证
	var amount int64
	var originAmount int64
	var userDiscountAmount int64 = 0
	var orderResp response.OrderConfirmResponse
	orderResp.CurrencyCode = currentSite.Currency.Currency
	orderResp.CurrencySymbol = currentSite.Currency.Symbol
	orderResp.PayWays = currentSite.GetPaymentWays()

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
		if activity.CanUseCoupon == false {
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
	userDiscount, _ := currentSite.GetUserDiscount(userId, user)

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

	couponMoney := couponCode.GetMoney(orderResp.Amount)
	orderResp.CouponDiscount = couponMoney
	orderResp.UserDiscount = userDiscountAmount
	orderResp.DiscountAmount = orderResp.CouponDiscount + orderResp.UserDiscount
	orderResp.CouponCode = couponCode.Code
	orderResp.CouponCodeId = couponCode.Id
	orderResp.CouponId = couponCode.CouponId
	orderResp.Amount = orderResp.Amount - couponMoney

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CouponCodeValid"),
		"data": iris.Map{
			"coupon":          couponCode,
			"discount_amount": orderResp.DiscountAmount,
			"coupon_discount": orderResp.CouponDiscount,
			"user_discount":   orderResp.UserDiscount,
			"coupon_code":     orderResp.CouponCode,
			"coupon_code_id":  orderResp.CouponCodeId,
			"coupon_id":       orderResp.CouponId,
			"amount":          orderResp.Amount,
			"origin_amount":   orderResp.OriginAmount,
		},
	})
}
