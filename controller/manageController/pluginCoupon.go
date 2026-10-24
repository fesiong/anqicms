package manageController

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func GetCouponList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	couponType := ctx.URLParamIntDefault("type", 0)

	coupons, total := currentSite.GetCoupons(func(tx *gorm.DB) *gorm.DB {
		if couponType > 0 {
			tx = tx.Where("type = ?", couponType)
		}
		return tx
	}, "id desc", currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  coupons,
	})
}

func GetCouponCodeList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	couponId := ctx.URLParamIntDefault("coupon_id", 0)
	userId := ctx.URLParamIntDefault("user_id", 0)
	code := ctx.URLParam("code")
	status := -1
	if ctx.URLParamExists("status") {
		status = ctx.URLParamIntDefault("status", 0)
	}

	codes, total := currentSite.GetCouponCodes(func(tx *gorm.DB) *gorm.DB {
		if couponId > 0 {
			tx = tx.Where("coupon_id = ?", couponId)
		}
		if userId > 0 {
			tx = tx.Where("user_id = ?", userId)
		}
		if code != "" {
			tx = tx.Where("code = ?", code)
		}
		if status != -1 {
			tx = tx.Where("status = ?", status)
		}

		return tx
	}, "id desc", currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  codes,
	})
}

func MakeCouponCode(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.MakeCouponCodeRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Count < 1 {
		req.Count = 1
	}
	_, err := currentSite.MakeCouponCode(req.CouponId, req.UserId, req.Count)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
}

func CouponDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	couponId := ctx.URLParamInt64Default("coupon_id", 0)
	coupon, err := currentSite.GetCouponById(couponId)
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
		"data": coupon,
	})
}

func CouponDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.CouponRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveCoupon(&req)
	if err != nil {
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

func DeleteCoupon(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.CouponRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteCoupon(req.Id)
	if err != nil {
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

func DeleteCouponCode(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.CouponCodeRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.Id > 0 {
		req.Ids = append(req.Ids, req.Id)
	}
	err := currentSite.DeleteCouponCode(req.Ids)
	if err != nil {
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

func SaveCouponCode(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.CouponCodeRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveCouponCode(&req)
	if err != nil {
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

func GetCouponArchives(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	couponId := ctx.URLParamIntDefault("coupon_id", 0)

	archives, total := currentSite.GetCouponArchives(int64(couponId), currentPage, pageSize)
	//读取列表的分类
	categories := currentSite.GetCacheCategories()
	for i := range categories {
		categories[i].Link = currentSite.GetUrl("category", categories[i], 0)
	}
	for i, v := range archives {
		if currentSite.Content.MultiCategory == 1 {
			var catIds []uint
			currentSite.DB.Model(&model.ArchiveCategory{}).Where("`archive_id` = ?", v.Id).Pluck("category_id", &catIds)
			for _, catId := range catIds {
				for _, cat := range categories {
					if cat.Id == catId {
						title := cat.Title
						if cat.Status != config.ContentStatusOK {
							title += ctx.Tr("(Hide)")
						}
						archives[i].CategoryTitles = append(archives[i].CategoryTitles, title)
					}
				}
			}
			archives[i].CategoryIds = catIds
		} else if v.CategoryId > 0 {
			for _, c := range categories {
				if c.Id == v.CategoryId {
					title := c.Title
					if c.Status != config.ContentStatusOK {
						title += ctx.Tr("(Hide)")
					}
					archives[i].CategoryTitles = []string{title}
					break
				}
			}
			archives[i].CategoryIds = []uint{archives[i].CategoryId}
		}
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  archives,
	})
}

func AddCouponArchives(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CouponArchiveRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.ArchiveId > 0 {
		req.ArchiveIds = append(req.ArchiveIds, req.ArchiveId)
	}

	err := currentSite.AddCouponArchives(req.CouponId, req.ArchiveIds)
	if err != nil {
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

func DeleteCouponArchives(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CouponArchiveRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.ArchiveId > 0 {
		req.ArchiveIds = append(req.ArchiveIds, req.ArchiveId)
	}

	err := currentSite.DeleteCouponArchives(req.CouponId, req.ArchiveIds)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("删除成功"),
	})
}
