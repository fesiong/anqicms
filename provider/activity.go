package provider

import (
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"time"
)

func (w *Website) GetActivities(ops func(tx *gorm.DB) *gorm.DB, order string, page, pageSize int) ([]*model.Activity, int64) {
	var activities []*model.Activity
	var total int64
	offset := (page - 1) * pageSize
	tx := w.DB.Model(&model.Activity{})
	if ops != nil {
		tx = ops(tx)
	}

	tx.Count(&total)

	tx.Order("id desc").Limit(pageSize).Offset(offset).Find(&activities)
	var goodsIds []int64
	for i := range activities {
		goodsIds = append(goodsIds, activities[i].GoodsId)
	}
	var goods []*model.Archive
	w.DB.Where("id IN (?)", goodsIds).Find(&goods)
	for i := range activities {
		for j := range goods {
			if activities[i].GoodsId == goods[j].Id {
				goods[j].Link = w.GetUrl("archive", goods[j], 0)
				goods[j].Thumb = goods[j].GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
				activities[i].Goods = goods[j]
				break
			}
		}
	}

	return activities, total
}

func (w *Website) GetActivityById(id int64) (*model.Activity, error) {
	var activity model.Activity
	err := w.DB.Where("id = ?", id).First(&activity).Error
	if err != nil {
		return nil, err
	}

	return &activity, nil
}

func (w *Website) GetActivityByGoodsId(goodsId int64) (*model.Activity, error) {
	var activity model.Activity
	err := w.DB.Where("goods_id = ? and status = ?", goodsId, 1).Take(&activity).Error
	if err != nil {
		return nil, err
	}

	return &activity, nil
}

func (w *Website) GetActivitySkus(id int64) ([]*model.ActivitySku, error) {
	var skus []*model.ActivitySku
	err := w.DB.Where("activity_id = ?", id).Find(&skus).Error
	if err != nil {
		return nil, err
	}
	var skuIds []int64
	for i := range skus {
		skuIds = append(skuIds, skus[i].SkuId)
	}
	var archiveSkus []*model.ArchiveSku
	w.DB.Where("id IN (?)", skuIds).Find(&archiveSkus)
	for i := range skus {
		for j := range archiveSkus {
			if skus[i].SkuId == archiveSkus[j].Id {
				skus[i].Sku = archiveSkus[j]
				break
			}
		}
	}

	return skus, nil
}

func (w *Website) GetActivitySkuBySkuId(activityId, skuId int64) (*model.ActivitySku, error) {
	var sku model.ActivitySku
	err := w.DB.Where("activity_id = ? AND sku_id = ?", activityId, skuId).First(&sku).Error
	if err != nil {
		return nil, err
	}

	return &sku, nil
}

func (w *Website) SaveActivity(req *request.ActivityRequest) error {
	var activity model.Activity
	if req.Id > 0 {
		err := w.DB.Where("id = ?", req.Id).First(&activity).Error
		if err != nil {
			return err
		}
	} else {
		activity = model.Activity{}
	}
	activity.GoodsType = req.GoodsType
	activity.StartTime = req.StartTime
	activity.EndTime = req.EndTime
	activity.UserLimit = req.UserLimit
	activity.Title = req.Title
	activity.Description = req.Description
	activity.Images = req.Images
	activity.GoodsId = req.GoodsId
	activity.Status = req.Status
	activity.Price = req.Price
	activity.Stock = req.Stock
	activity.Extra = req.Extra
	activity.CanUseCoupon = req.CanUseCoupon
	if len(req.Skus) > 0 {
		tmpStock := int64(0)
		for _, item := range req.Skus {
			tmpStock += item.Stock
		}
		activity.Stock = tmpStock
	}

	err := w.DB.Save(&activity).Error
	if err != nil {
		return err
	}
	if len(req.Skus) == 0 {
		w.DB.Where("activity_id = ?", activity.Id).Delete(&model.ActivitySku{})
		return nil
	}
	// 处理sku
	var skuId []int64
	for _, item := range req.Skus {
		var sku model.ActivitySku
		if item.Id > 0 {
			err = w.DB.Where("id = ?", item.Id).First(&sku).Error
			if err != nil {
				sku = model.ActivitySku{}
			}
		} else {
			sku = model.ActivitySku{}
		}
		sku.ActivityId = activity.Id
		sku.SkuId = item.SkuId
		sku.Price = item.Price
		sku.Stock = item.Stock
		sku.GoodsId = item.GoodsId
		w.DB.Save(&sku)
		skuId = append(skuId, sku.Id)
	}
	// 删除多余的sku
	w.DB.Where("activity_id = ? AND id NOT IN (?)", activity.Id, skuId).Delete(&model.ActivitySku{})

	return nil
}

func (w *Website) DeleteActivity(id int64) error {
	var activity model.Activity
	err := w.DB.Where("id = ?", id).Take(&activity).Error
	if err != nil {
		return err
	}
	// 删除sku
	w.DB.Where("activity_id = ?", activity.Id).Delete(&model.ActivitySku{})

	return w.DB.Delete(&activity).Error
}

func (w *Website) CheckActivities() {
	// 过期的活动，设置 status = 0
	w.DB.Model(&model.Activity{}).Where("status = ? AND end_time < ?", 1, time.Now().Unix()).Update("status", 0)
}
