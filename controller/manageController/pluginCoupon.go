package manageController

import (
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// GetCouponList 分页查询优惠券列表，支持按券类型筛选。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "type": 券类型筛选：1=注册赠送、2=限量领取、3=下单赠送、4=促销满减（无需领取）、5=自由规则、6=积分兑换、7=活动奖品；0 表示不限。
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

// GetCouponCodeList 分页查询优惠码列表，支持按券、用户、码值、状态筛选。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "coupon_id": 按所属优惠券 ID 筛选，0 表示不限。
//   - 查询参数 "user_id": 按持有人用户 ID 筛选，0 表示不限。
//   - 查询参数 "code": 按优惠码精确匹配（不是模糊搜索）。
//   - 查询参数 "status": 按优惠码状态筛选；只有显式传该参数才生效，不传则不做状态过滤（传 0 表示只查状态为 0 的码）。
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

// MakeCouponCode 为指定优惠券批量生成优惠码，count 小于 1 时按 1 处理。
//
// 不传 user_id 时批量生成 count 张未绑定的码，此时响应 data 为 null，
// 需要再查 code/list 才能拿到码；传了 user_id 则只生成 1 张并直接返回该码，
// 若该用户已领过（注册券/限量券/未使用的下单赠与券）会返回失败原因。
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
	couponCode, err := currentSite.MakeCouponCode(req.CouponId, req.UserId, req.Count)
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

// CouponDetail 按 ID 查询单个优惠券详情。注意参数名是 coupon_id，不是 id。
//
// 参数说明：
//   - 查询参数 "coupon_id": 优惠券 ID，必填；传 0 或不存在时返回错误信息。
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

// CouponDetailForm 新建或更新优惠券。
//
// 默认是全量覆盖（后台表单提交整个表单，字段没出现 = 用户清空了它）；
// 传 partial=true 则走 PATCH，未传字段保持原值（AI/MCP 默认这样调）。
func CouponDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.CouponRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !req.Partial {
		req.UpdateAll = true
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

// DeleteCoupon 删除优惠券，并连带删除该券下的全部兑换码。
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

// DeleteCouponCode 删除优惠码；单个传 id，批量传 ids。
func DeleteCouponCode(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.CouponCodeRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	if !requireAnyID(ctx, req.Id, req.Ids) {
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

// SaveCouponCode 新建或更新单张优惠码（id=0 表示新建，code 留空则自动生成）。
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

// GetCouponArchives 分页查询某张优惠券适用的商品（文档）列表，附带分类与链接。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "coupon_id": 优惠券 ID，必填；传 0 时按 coupon_id=0 过滤，查不到任何商品。
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

// AddCouponArchives 把商品加入优惠券的适用范围；单个传 archive_id，批量传 archive_ids。
func AddCouponArchives(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CouponArchiveRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	// 券 id 和商品 id 都没给，没法建立适用范围。
	if !requireAnyID(ctx, req.CouponId, req.ArchiveIds) {
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

// DeleteCouponArchives 把商品移出优惠券的适用范围；单个传 archive_id，批量传 archive_ids。
func DeleteCouponArchives(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.CouponArchiveRequest
	if !readJSONPayload(ctx, &req) {
		return
	}
	// 没有券 id 不知道该从哪里移除；商品 id 也没给则没有移除目标。
	if !requirePositive(ctx, req.CouponId, "coupon_id") {
		return
	}
	if !requireAnyID(ctx, req.ArchiveId, req.ArchiveIds) {
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
