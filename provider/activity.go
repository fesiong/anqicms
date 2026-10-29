package provider

import (
	"time"

	"gorm.io/gorm"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
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

// SaveActivity 新建或更新营销活动。
//
// 语义（2026-10-29 指针化）：nil = 调用方没提（保持库里原值），
// 非 nil = 显式写入（含 0：失效/免费/售罄/不限购/不叠加优惠）。
// UpdateAll（后台表单全量提交）时，nil 才当成「用户清空」写零值。
//
// ⚠️ 规格（skus）单独一套语义：nil=不动、[]=清空全部、非空=按列表同步。
// 改造前「不传」和「传空数组」是同一个零值，于是「只改个标题」就把规格
// 整批删掉，活动价与库存随之失效，而回执是 ok（invokeRouteOverwriteFields
// 当时靠 GET 回查补齐才挡住，回查失败就又漏了）。
func (w *Website) SaveActivity(req *request.ActivityRequest) error {
	var activity model.Activity
	newPost := false
	if req.Id > 0 {
		err := w.DB.Where("id = ?", req.Id).First(&activity).Error
		if err != nil {
			return err
		}
	} else {
		activity = model.Activity{}
		newPost = true
	}
	if newPost {
		// 新建默认生效。未传 status 落到 0 会造出一张「建好了但前台不展示」
		// 的活动——后台列表看得见，用户端永远看不到，且没有任何报错。
		activity.Status = 1
	}

	force := req.UpdateAll
	if req.Title != "" {
		activity.Title = req.Title
	} else if force {
		activity.Title = ""
	}
	library.Assign(&activity.GoodsType, req.GoodsType, force)
	library.Assign(&activity.StartTime, req.StartTime, force)
	library.Assign(&activity.EndTime, req.EndTime, force)
	library.Assign(&activity.UserLimit, req.UserLimit, force)
	library.Assign(&activity.Description, req.Description, force)
	library.Assign(&activity.Images, req.Images, force)
	library.Assign(&activity.GoodsId, req.GoodsId, force)
	library.Assign(&activity.Price, req.Price, force)
	library.Assign(&activity.Stock, req.Stock, force)
	library.Assign(&activity.CanUseCoupon, req.CanUseCoupon, force)
	library.Assign(&activity.Status, req.Status, force)
	// Extra 是 model.ExtraData（命名类型），不能和 map 共用 Assign。
	if req.Extra != nil {
		activity.Extra = model.ExtraData(req.Extra)
	} else if force {
		activity.Extra = nil
	}

	// 传了规格时，活动库存以规格库存合计为准（覆盖上面按字段写入的值）。
	if req.Skus != nil && len(*req.Skus) > 0 {
		tmpStock := int64(0)
		for _, item := range *req.Skus {
			tmpStock += item.Stock
		}
		activity.Stock = tmpStock
	}

	err := w.DB.Save(&activity).Error
	if err != nil {
		return err
	}
	if req.Skus == nil {
		// PATCH 没提规格：保持现有规格不动。
		return nil
	}
	if len(*req.Skus) == 0 {
		// 显式传空数组：删掉全部规格（这是「清空」的唯一入口）。
		w.DB.Where("activity_id = ?", activity.Id).Delete(&model.ActivitySku{})
		return nil
	}
	// 处理sku
	var skuId []int64
	for _, item := range *req.Skus {
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
