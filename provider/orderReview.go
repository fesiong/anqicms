package provider

import (
	"errors"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) GetOrderReviews(ops func(tx *gorm.DB) *gorm.DB, order string, currentPage, pageSize int, offsets ...int) ([]*model.OrderReview, int64) {
	var reviews []*model.OrderReview
	var offset int
	if currentPage > 1 {
		offset = (currentPage - 1) * pageSize
	}
	if len(offsets) > 0 {
		offset = offsets[0]
	}
	order = ParseOrderBy(order, "")
	var total int64

	tx := w.DB.Model(&model.OrderReview{})
	if ops != nil {
		tx = ops(tx)
	}
	tx.Count(&total)
	tx.Order(order).Offset(offset).Limit(pageSize).Find(&reviews)
	if len(reviews) > 0 {
		var userIds []uint
		var archiveIds []int64
		var archiveSkuIds []int64
		for _, review := range reviews {
			userIds = append(userIds, review.UserId)
			archiveIds = append(archiveIds, review.ArchiveId)
			if review.ArchiveSkuId > 0 {
				archiveSkuIds = append(archiveSkuIds, review.ArchiveSkuId)
			}
		}
		var users []*model.User
		w.DB.Where("id in (?)", userIds).Find(&users)
		var archives []*model.Archive
		w.DB.Where("id in (?)", archiveIds).Find(&archives)
		var skus []*model.ArchiveSku
		w.DB.Where("id in (?)", archiveSkuIds).Find(&skus)
		for i, review := range reviews {
			for _, user := range users {
				if review.UserId == user.Id {
					reviews[i].UserName = user.UserName
					reviews[i].AvatarURL = user.GetThumb(w.PluginStorage.StorageUrl)
					break
				}
			}
			for _, archive := range archives {
				if review.ArchiveId == archive.Id {
					reviews[i].ArchiveTitle = archive.Title
					break
				}
			}
			if review.ArchiveSkuId > 0 {
				for _, sku := range skus {
					if review.ArchiveSkuId == sku.Id {
						reviews[i].SkuTitle = sku.Title
						break
					}
				}
			}
		}
	}

	return reviews, total
}

func (w *Website) GetOrderReview(id uint) (*model.OrderReview, error) {
	var review model.OrderReview
	err := w.DB.First(&review, id).Error
	if err != nil {
		return nil, err
	}
	return &review, nil
}

func (w *Website) SaveOrderReview(req *request.OrderReviewRequest) error {
	// 同一个订单只能评价一次
	var review model.OrderReview
	if req.Id > 0 {
		err := w.DB.Where("id = ?", req.Id).Take(&review).Error
		if err != nil {
			return err
		}
		if review.UserId != req.UserId {
			return errors.New("error Not Found")
		}
	} else if req.OrderId != "" {
		var exist model.OrderReview
		err := w.DB.Where("`user_id` = ? and `order_id` = ? and `archive_id` = ?", req.UserId, req.OrderId, req.ArchiveId).Take(&exist).Error
		if err == nil {
			return nil
		}
	}
	review.ArchiveId = req.ArchiveId
	review.ArchiveSkuId = req.ArchiveSkuId
	review.Content = req.Content
	review.OrderId = req.OrderId
	review.UserName = req.UserName
	review.Email = req.Email
	review.Ip = req.Ip
	review.Score = req.Score
	review.Status = req.Status
	review.UserId = req.UserId
	review.Images = req.Images

	if review.OrderId != "" {
		identifier := "order_review_" + review.OrderId
		w.CancelMailPlan(identifier, true)
	}

	return w.DB.Save(&review).Error
}

func (w *Website) DeleteOrderReview(id int64) error {
	var exist model.OrderReview
	err := w.DB.Where("`id` = ?", id).Take(&exist).Error
	if err != nil {
		return err
	}

	return w.DB.Where("`id` = ?", id).Delete(&model.OrderReview{}).Error
}

func (w *Website) GetReviewPraise(userId uint, reviewId int64) (*model.OrderReviewPraise, error) {
	var praise model.OrderReviewPraise
	if err := w.DB.WithContext(w.Ctx()).Where("user_id = ? and review_id = ?", userId, reviewId).Take(&praise).Error; err != nil {
		return nil, err
	}

	return &praise, nil
}

func (w *Website) AddReviewPraise(userId uint, reviewId int64, archiveId int64) (praise *model.OrderReviewPraise, err error) {
	praise, err = w.GetReviewPraise(userId, reviewId)
	if err != nil {
		praise = &model.OrderReviewPraise{
			UserId:    userId,
			ReviewId:  reviewId,
			ArchiveId: archiveId,
			Rate:      1,
		}
		if err = w.DB.Create(praise).Error; err != nil {
			return nil, err
		}
	}

	return praise, nil
}
