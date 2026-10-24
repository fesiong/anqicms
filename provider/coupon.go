package provider

import (
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) GetCoupons(ops func(tx *gorm.DB) *gorm.DB, order string, page, pageSize int) ([]*model.Coupon, int64) {
	var coupons []*model.Coupon
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.Coupon{})
	if ops != nil {
		tx = ops(tx)
	}

	tx.Count(&total)

	tx.Order("id desc").Limit(pageSize).Offset(offset).Find(&coupons)

	return coupons, total
}

func (w *Website) GetCouponById(id int64) (*model.Coupon, error) {
	var coupon model.Coupon
	err := w.DB.Where("id = ?", id).First(&coupon).Error
	if err != nil {
		return nil, err
	}
	return &coupon, nil
}

func (w *Website) SaveCoupon(req *request.CouponRequest) error {
	var coupon model.Coupon
	if req.Id > 0 {
		err := w.DB.Where("id = ?", req.Id).First(&coupon).Error
		if err != nil {
			return err
		}
	} else {
		coupon = model.Coupon{}
	}
	coupon.Money = req.Money
	coupon.StartTime = req.StartTime
	coupon.EndTime = req.EndTime
	coupon.Title = req.Title
	coupon.Description = req.Description
	coupon.Expire = req.Expire
	coupon.Status = req.Status
	coupon.Type = req.Type
	coupon.MaxMoney = req.MaxMoney
	coupon.MoneyType = req.MoneyType
	coupon.NeedIntegral = req.NeedIntegral
	coupon.NeedMoney = req.NeedMoney
	coupon.RangeType = req.RangeType

	return w.DB.Save(&coupon).Error
}

func (w *Website) DeleteCoupon(couponId int64) error {
	var coupon model.Coupon
	err := w.DB.Where("id = ?", couponId).Take(&coupon).Error
	if err != nil {
		return err
	}
	// 删除code
	w.DB.Where("coupon_id = ?", couponId).Delete(&model.CouponCode{})

	return w.DB.Delete(&coupon).Error
}

func (w *Website) DeleteCouponCode(codeIds []int64) error {
	return w.DB.Where("id IN (?)", codeIds).Delete(&model.CouponCode{}).Error
}

func (w *Website) SaveCouponCode(req *request.CouponCodeRequest) error {
	var couponCode model.CouponCode
	err := w.DB.Where("id = ?", req.Id).Take(&couponCode).Error
	if err != nil {
		return err
	}
	couponCode.UserId = req.UserId
	if req.Code != "" {
		couponCode.Code = req.Code
	}
	if couponCode.GetTime == 0 {
		couponCode.GetTime = time.Now().Unix()
		_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypeCouponGet, Id: couponCode.Id})
	}
	if req.Money > 0 {
		couponCode.Money = req.Money
	}
	if req.StartTime > 0 {
		couponCode.StartTime = req.StartTime
	}
	if req.EndTime > 0 {
		couponCode.EndTime = req.EndTime
	}
	if req.Status > 0 {
		couponCode.Status = req.Status
	}

	return w.DB.Save(&couponCode).Error
}

func (w *Website) GetCouponCodes(ops func(tx *gorm.DB) *gorm.DB, order string, page, pageSize int) ([]*model.CouponCode, int64) {
	var couponCodes []*model.CouponCode
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.CouponCode{})
	if ops != nil {
		tx = ops(tx)
	}

	tx.Count(&total)
	tx.Order(order).Limit(pageSize).Offset(offset).Find(&couponCodes)
	var userIds []uint
	for i := range couponCodes {
		userIds = append(userIds, uint(couponCodes[i].UserId))
	}
	users := w.GetUsersInfoByIds(userIds)
	for i := range couponCodes {
		for u := range users {
			if uint(couponCodes[i].UserId) == users[u].Id {
				couponCodes[i].UserName = users[u].UserName
				break
			}
		}
	}
	var couponIds []int64
	for i := range couponCodes {
		couponIds = append(couponIds, couponCodes[i].CouponId)
	}
	var coupons []model.Coupon
	w.DB.Where("id in (?)", couponIds).Find(&coupons)
	for i := range couponCodes {
		for c := range coupons {
			if couponCodes[i].CouponId == coupons[c].Id {
				couponCodes[i].Title = coupons[c].Title
				couponCodes[i].Description = coupons[c].Description
				couponCodes[i].Type = coupons[c].Type
				couponCodes[i].Coupon = &coupons[c]
				break
			}
		}
	}

	return couponCodes, total
}

func (w *Website) MakeCouponCode(couponId, userId int64, count int) (*model.CouponCode, error) {
	if couponId == 0 {
		return nil, errors.New("invalid coupon id")
	}
	coupon, err := w.GetCouponById(couponId)
	if err != nil {
		return nil, err
	}
	if userId == 0 {
		// 生成多张未使用的优惠券
		couponCode := model.CouponCode{
			CouponId: coupon.Id,
			//Code:        ,
			StartTime: time.Now().Unix(),
			EndTime:   time.Now().AddDate(0, 0, int(coupon.Expire)).Unix(),
			UserId:    userId,
			GetTime:   0,
			UseTime:   0,
			Status:    0,
			//Money:     coupon.GetMoney(),
			MoneyType: coupon.MoneyType,
			MaxMoney:  coupon.MaxMoney,
		}
		for i := 0; i < count; i++ {
			item := couponCode
			item.Id = 0
			item.Money = coupon.GetMoney()
			item.Code = library.Md5(strconv.FormatInt(time.Now().UnixNano(), 10))[i%16 : i%16+16]
			w.DB.Create(&item)
		}

		return nil, nil
	}
	// 生成指定用户的优惠码
	if coupon.Type == config.CouponTypeRegister || coupon.Type == config.CouponTypeLimit {
		// 先检查是否已经生成过了
		var couponCode model.CouponCode
		w.DB.Where("coupon_id = ? and user_id = ?", coupon.Id, userId).Last(&couponCode)
		// 注册 和限制数量优惠券，只能领取1次。
		if couponCode.Id > 0 {
			return nil, errors.New("您已经领取过该优惠券了。")
		}
		// 开始生成优惠券
		couponCode = model.CouponCode{
			CouponId:  coupon.Id,
			Code:      library.Md5(strconv.FormatInt(time.Now().UnixNano(), 10))[8:24],
			StartTime: time.Now().Unix(),
			EndTime:   time.Now().AddDate(0, 0, int(coupon.Expire)).Unix(),
			UserId:    userId,
			GetTime:   time.Now().Unix(),
			UseTime:   0,
			Status:    0,
			Money:     coupon.GetMoney(),
			MoneyType: coupon.MoneyType,
			MaxMoney:  coupon.MaxMoney,
		}
		w.DB.Create(&couponCode)

		couponCode.Description = coupon.Description
		couponCode.Type = coupon.Type
		couponCode.Coupon = coupon
		_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypeCouponGet, Id: couponCode.Id})
		return &couponCode, nil
	} else if coupon.Type == config.CouponTypeBuy || coupon.Type == config.CouponTypeFree || coupon.Type == config.CouponTypePromotion {
		//该接口和自由规则接口一样,只能拥有一张未使用的，使用了还可以继续领取。
		// 先检查是否已经生成过了
		var couponCode model.CouponCode
		w.DB.Where("coupon_id = ? and user_id = ? and status = ?", coupon.Id, userId, config.CouponStatusUnused).Last(&couponCode)
		if couponCode.Id > 0 {
			return &couponCode, errors.New("已领取优惠券")
		}
		// 开始生成优惠券
		couponCode = model.CouponCode{
			CouponId:  coupon.Id,
			Code:      library.Md5(strconv.FormatInt(time.Now().UnixNano(), 10))[8:24],
			StartTime: time.Now().Unix(),
			EndTime:   time.Now().AddDate(0, 0, int(coupon.Expire)).Unix(),
			UserId:    userId,
			GetTime:   time.Now().Unix(),
			UseTime:   0,
			Status:    0,
			Money:     coupon.GetMoney(),
			MoneyType: coupon.MoneyType,
			MaxMoney:  coupon.MaxMoney,
		}
		w.DB.Create(&couponCode)
		_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypeCouponGet, Id: couponCode.Id})

		couponCode.Description = coupon.Description
		couponCode.Type = coupon.Type
		couponCode.Coupon = coupon
		return &couponCode, nil
	} else if coupon.Type == config.CouponTypeActivity {
		// 活动
		couponCode := model.CouponCode{
			CouponId:  coupon.Id,
			Code:      library.Md5(strconv.FormatInt(time.Now().UnixNano(), 10))[8:24],
			StartTime: time.Now().Unix(),
			EndTime:   time.Now().AddDate(0, 0, int(coupon.Expire)).Unix(),
			UserId:    userId,
			GetTime:   time.Now().Unix(),
			UseTime:   0,
			Status:    0,
			Money:     coupon.GetMoney(),
			MoneyType: coupon.MoneyType,
			MaxMoney:  coupon.MaxMoney,
		}
		w.DB.Create(&couponCode)
		_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypeCouponGet, Id: couponCode.Id})

		couponCode.Description = coupon.Description
		couponCode.Type = coupon.Type
		couponCode.Coupon = coupon
		return &couponCode, nil
	}

	return nil, nil
}

func (w *Website) GetCouponArchives(couponId int64, page, pageSize int) ([]*model.Archive, int64) {
	var archives []*model.Archive
	var total int64
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	tx := w.DB.Model(&model.CouponArchive{}).
		Joins("left join archives on archives.id = coupon_archives.archive_id").
		Where("coupon_archives.coupon_id = ?", couponId).
		Order("archives.id desc")

	tx.Count(&total)

	tx.Limit(pageSize).Offset(offset).Select("archives.*").Find(&archives)

	return archives, total
}

func (w *Website) AddCouponArchives(couponId int64, archiveIds []int64) error {
	var archives []*model.Archive
	w.DB.Where("id in (?)", archiveIds).Find(&archives)
	for _, archive := range archives {
		couponArchive := model.CouponArchive{
			CouponId:  couponId,
			ArchiveId: archive.Id,
		}
		w.DB.Model(&model.CouponArchive{}).Where("coupon_id = ? and archive_id = ?", couponId, archive.Id).FirstOrCreate(&couponArchive)
	}

	return nil
}

func (w *Website) DeleteCouponArchives(couponId int64, archiveIds []int64) error {
	return w.DB.Where("coupon_id = ? and archive_id in (?)", couponId, archiveIds).Delete(&model.CouponArchive{}).Error
}

func (w *Website) CheckCouponCodeExpire() {
	w.DB.Model(&model.CouponCode{}).Where("status = ? and end_time < ?", config.CouponStatusUnused, time.Now().Unix()).UpdateColumn("status", config.CouponStatusExpired)
}

func (w *Website) CheckCouponValidByProductId(coupon *model.Coupon, archiveId int64) bool {
	if coupon.RangeType == config.CouponRangeTypeAll {
		return true
	}
	if archiveId == 0 {
		return false
	}
	var exist int64
	w.DB.Model(&model.CouponArchive{}).Where("coupon_id = ? and archive_id = ?", coupon.Id, archiveId).Count(&exist)
	if coupon.RangeType == config.CouponRangeTypeProduct && exist > 0 {
		return true
	}
	if coupon.RangeType == config.CouponRangeTypeNotProduct && exist == 0 {
		return true
	}

	return false
}

func (w *Website) GetCouponCodeByCode(code string) (*model.CouponCode, error) {
	var couponCode model.CouponCode
	err := w.DB.Where("code = ?", code).Last(&couponCode).Error
	if err != nil {
		return nil, err
	}
	couponCode.Coupon, _ = w.GetCouponById(couponCode.CouponId)

	return &couponCode, nil
}

func (w *Website) GetCouponCodeById(id int64) (*model.CouponCode, error) {
	var couponCode model.CouponCode
	err := w.DB.Where("id = ?", id).First(&couponCode).Error
	if err != nil {
		return nil, err
	}
	couponCode.Coupon, _ = w.GetCouponById(couponCode.CouponId)

	return &couponCode, nil
}

func (w *Website) UseCouponCode(couponCode *model.CouponCode) error {
	couponCode.UseTime = time.Now().Unix()
	couponCode.Status = 2
	_ = w.DB.Save(couponCode).Error
	_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypeCouponUse, Id: couponCode.Id})

	return nil
}
