package provider

import (
	"errors"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func (w *Website) GetWishlist(userId int64, currentPage int, pageSize int) ([]*response.WishListResponse, int64) {
	var wishlist []*response.WishListResponse
	var total int64
	offset := (currentPage - 1) * pageSize

	builder := w.DB.Model(&model.Wishlist{})
	if userId > 0 {
		builder = builder.Where("user_id = ?", userId).Order("priority desc, updated_time desc")
	} else {
		builder = builder.Order("id desc")
	}
	builder.Count(&total)
	builder.Limit(pageSize).Offset(offset).Scan(&wishlist)
	if len(wishlist) == 0 {
		return wishlist, total
	}
	var userIds []int64
	var archiveIds []int64
	var skuIds []int64
	for _, v := range wishlist {
		userIds = append(userIds, v.UserId)
		archiveIds = append(archiveIds, v.ArchiveId)
		if v.SkuId > 0 {
			skuIds = append(skuIds, v.SkuId)
		}
	}
	var users []*model.User
	var archives []*model.Archive
	var skus []*model.ArchiveSku
	w.DB.Where("id IN(?)", userIds).Find(&users)
	w.DB.Where("id IN(?)", archiveIds).Find(&archives)
	w.DB.Where("id IN(?)", skuIds).Find(&skus)
	for i, v := range wishlist {
		for _, u := range users {
			if v.UserId == int64(u.Id) {
				wishlist[i].UserName = u.UserName
				break
			}
		}
		for _, a := range archives {
			a.Link = w.GetUrl(PatternArchive, a, 0)
			if v.ArchiveId == int64(a.Id) {
				wishlist[i].Title = a.Title
				wishlist[i].Description = a.Description
				wishlist[i].Price = a.Price
				wishlist[i].Thumb = a.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
				wishlist[i].Link = a.Link
				break
			}
		}
		if v.SkuId > 0 {
			for _, s := range skus {
				if v.SkuId == int64(s.Id) {
					wishlist[i].SkuTitle = s.Title
					wishlist[i].Price = s.Price
					skuThumb := s.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
					if len(skuThumb) > 0 {
						wishlist[i].Thumb = skuThumb
					}
					break
				}
			}
		}
	}

	return wishlist, total
}

func (w *Website) WishlistAdd(req *request.WishListRequest) error {
	var wishlist = model.Wishlist{
		ArchiveId:   req.ArchiveId,
		UserId:      req.UserId,
		SkuId:       req.SkuId,
		Priority:    req.Priority,
		Status:      req.Status,
		OriginPrice: req.OriginPrice,
		TargetPrice: req.TargetPrice,
		Remark:      req.Remark,
	}
	// 检查产品
	archive, err := w.GetArchiveById(req.ArchiveId)
	if err != nil {
		return err
	}
	wishlist.OriginPrice = archive.Price
	if req.SkuId > 0 {
		archiveSku, err := w.GetArchiveSkuById(req.ArchiveId, req.SkuId)
		if err != nil {
			return err
		}
		wishlist.OriginPrice = archiveSku.Price
	}
	// 入库
	var existId int64
	w.DB.Where("archive_id = ? and user_id = ?", req.ArchiveId, req.UserId).Pluck("id", &existId)
	wishlist.Id = existId
	if existId > 0 {
		w.DB.Save(&wishlist)
	} else {
		w.DB.Create(&wishlist)
	}

	return nil
}

func (w *Website) WishlistDelete(req *request.WishListRequest) error {
	var wishlist model.Wishlist
	err := w.DB.Where("id = ?", req.Id).Take(&wishlist).Error
	if err != nil {
		return err
	}
	if req.UserId > 0 && wishlist.UserId != req.UserId {
		return errors.New(w.TplTr("DocumentDoesNotExist"))
	}

	return w.DB.Delete(&wishlist).Error
}
