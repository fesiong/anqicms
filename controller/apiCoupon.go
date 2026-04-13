package controller

import (
	"time"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
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
		return tx.Debug()
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
	_, err = currentSite.MakeCouponCode(req.Id, int64(userId), 1)
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
	})
}
