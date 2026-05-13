package provider

import (
	"errors"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) GetCartList(userId int64, cartToken string, currentPage int, pageSize int) ([]*model.Cart, int64) {
	var cartItems []*model.Cart
	var total int64
	offset := (currentPage - 1) * pageSize

	if userId > 0 && cartToken != "" {
		// 更新 购物车
		_ = w.DB.Model(&model.Cart{}).Where("token = ?", cartToken).Update("user_id", userId)
	}

	builder := w.DB.Model(&model.Cart{})
	if userId > 0 {
		builder = builder.Where("user_id = ?", userId).Order("id desc")
	} else if cartToken != "" {
		builder = builder.Where("token = ?", cartToken).Order("id desc")
	} else {
		return nil, total
	}
	builder.Count(&total)
	builder.Limit(pageSize).Offset(offset).Find(&cartItems)
	if len(cartItems) == 0 {
		return nil, total
	}
	var userIds []int64
	var archiveIds []int64
	var skuIds []int64
	for _, v := range cartItems {
		userIds = append(userIds, v.UserId)
		archiveIds = append(archiveIds, v.ArchiveId)
		if v.SkuId > 0 {
			skuIds = append(skuIds, v.SkuId)
		}
	}
	var archives []model.Archive
	var skus []model.ArchiveSku
	w.DB.Where("id IN(?)", archiveIds).Find(&archives)
	w.DB.Where("id IN(?)", skuIds).Find(&skus)
	for i, v := range cartItems {
		cartItems[i].Price = v.OriginPrice
		for _, a := range archives {
			if a.Id == v.ArchiveId {
				a.Link = w.GetUrl(PatternArchive, a, 0)
				a.Thumb = a.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
				cartItems[i].Price = a.Price
				cartItems[i].Amount = int64(v.Quantity) * a.Price
				if a.ProductType == config.ProductTypeWholesale {
					rules, err := w.GetArchiveWholesaleRules(a.Id)
					if err == nil {
						for _, rule := range rules {
							if v.Quantity >= rule.MinQuantity {
								a.WholesaleRule = rule
								cartItems[i].Amount = int64(v.Quantity) * a.Price * int64(rule.DiscountRate) / 100
							}
						}
					}
				}
				if a.OptionType == config.OptionTypePrice && len(v.SkuValues) > 0 {
					// 加价属性处理
					var optionPrice int64 = 0
					for _, sv := range v.SkuValues {
						// 如果option是属性，不需要处理库存
						option, err := w.GetArchiveOption(v.ArchiveId, sv.OptionId)
						if err != nil {
							continue
						}
						// 获取optionValue
						optionValue, err := w.GetArchiveOptionValue(v.ArchiveId, sv.Id)
						if err != nil {
							continue
						}
						if !option.IsAttribute && a.AllowOversold == false {
							// 需要处理库存
							if int(optionValue.Stock) < v.Quantity {
								// 处理？
							}
							optionPrice += optionValue.Price
						}
					}
					// 更新价格
					cartItems[i].Price += optionPrice
				}
				cartItems[i].Archive = &a
				break
			}
		}
		if v.SkuId > 0 {
			for _, s := range skus {
				if v.SkuId == int64(s.Id) {
					skuThumb := s.GetThumb(w.PluginStorage.StorageUrl, w.Content.DefaultThumb)
					if cartItems[i].Archive != nil && len(skuThumb) > 0 {
						cartItems[i].Archive.Thumb = skuThumb
					}
					cartItems[i].Price = s.Price
					cartItems[i].Sku = &s
					break
				}
			}
		}
		cartItems[i].Amount = int64(v.Quantity) * cartItems[i].Price
		cartItems[i].OriginAmount = int64(v.Quantity) * cartItems[i].OriginPrice
	}

	return cartItems, total
}

func (w *Website) CartAdd(req *request.CartRequest) error {
	// 先检查该商品是否有多规格
	archive, err := w.GetArchiveById(req.ArchiveId)
	if err != nil {
		return err
	}
	if archive.OptionType == config.OptionTypeCombine && req.SkuId == 0 {
		// 多规格产品用户未选择规格，默认选择第一个
		archiveSkus, err := w.GetArchiveSkus(archive.Id)
		if err != nil {
			return err
		}
		if len(archiveSkus) > 0 {
			req.SkuId = archiveSkus[0].Id
		}
	}
	if archive.MinQuantity > 0 && req.Quantity < archive.MinQuantity {
		req.Quantity = archive.MinQuantity
	}
	if archive.MaxQuantity > 0 && req.Quantity > archive.MaxQuantity {
		req.Quantity = archive.MaxQuantity
	}
	var cart = model.Cart{
		ArchiveId:   req.ArchiveId,
		UserId:      req.UserId,
		SkuId:       req.SkuId,
		SkuValues:   req.SkuValues,
		Quantity:    req.Quantity,
		Status:      req.Status,
		OriginPrice: req.OriginPrice,
	}
	if req.OrderExtra != nil {
		cart.OrderExtra = req.OrderExtra
	}
	if req.UserId == 0 {
		cart.Token = req.Token
	} else if req.UserId > 0 && req.Token != "" {
		// 更新 购物车
		_ = w.DB.Model(&model.Cart{}).Where("token = ?", req.Token).Update("user_id", req.UserId)
	}
	var exist model.Cart
	tx := w.DB.Where("archive_id = ? and `sku_id` = ? and user_id = ?", req.ArchiveId, req.SkuId, req.UserId)
	if req.UserId == 0 {
		tx = tx.Where("token = ?", req.Token)
	}
	err = tx.Take(&exist).Error
	if err == nil {
		cart.Id = exist.Id
		if req.Force {
			// 传递多少就是多少
			// cart.Quantity
		} else {
			cart.Quantity += exist.Quantity
		}
		cart.CreatedTime = exist.CreatedTime
	}
	cart.OriginPrice = archive.Price
	if req.SkuId > 0 {
		archiveSku, err := w.GetArchiveSkuById(req.ArchiveId, req.SkuId)
		if err != nil {
			return err
		}
		cart.OriginPrice = archiveSku.Price
		// 如果不允许超卖，需要判断剩余库存
		if archive.AllowOversold == false && int64(cart.Quantity) > archiveSku.Stock {
			return errors.New("out of stock")
		}
	} else {
		// 如果不允许超卖，需要判断剩余库存
		if archive.AllowOversold == false && int64(cart.Quantity) > archive.Stock {
			return errors.New("out of stock")
		}
	}
	// 对 sku_values 进行处理
	if len(cart.SkuValues) > 0 {
		addonPrice := int64(0)
		for i, sv := range cart.SkuValues {
			// 如果option是属性，不需要处理库存
			option, err := w.GetArchiveOption(archive.Id, sv.OptionId)
			if err != nil {
				// 不存在属性
				return errors.New("option not found")
			}
			// 获取optionValue
			optionValue, err := w.GetArchiveOptionValue(archive.Id, sv.Id)
			if err != nil {
				// 不存在属性值
				return errors.New("option value not found")
			}
			cart.SkuValues[i].OptionTitle = option.Title
			// 如果是加价商品，还需要检查optionValue的库存情况
			if archive.OptionType == config.OptionTypePrice {
				if !option.IsAttribute && archive.AllowOversold == false {
					// 需要处理库存
					if archive.AllowOversold == false && int(optionValue.Stock) < req.Quantity {
						return errors.New("out of stock")
					}
					// 加价
					addonPrice += optionValue.Price
				}
			}
		}
		cart.OriginPrice += addonPrice
	}
	// 入库
	if exist.Id > 0 {
		w.DB.Save(&cart)
	} else {
		w.DB.Create(&cart)
	}
	_ = w.ShopCountLog.Write(&request.LogStatisticRequest{Action: request.LogActionOrder, Type: request.LogTypeCart, Id: int64(cart.Id)})
	if req.UserId > 0 {
		// 推送事件
		w.CreateEvent(EventCartUpdated, map[string]interface{}{
			"id":       archive.Id,
			"title":    archive.Title,
			"sku_id":   cart.SkuId,
			"link":     archive.Link,
			"logo":     archive.Logo,
			"price":    float32(archive.Price) / 100,
			"quantity": cart.Quantity,
		}, &model.User{Id: uint(req.UserId)})
	}

	return nil
}

// CartUpdate 只能更新数量，和skuid
func (w *Website) CartUpdate(req *request.CartRequest) error {
	if req.UserId > 0 && req.Token != "" {
		// 更新 购物车
		_ = w.DB.Model(&model.Cart{}).Where("token = ?", req.Token).Update("user_id", req.UserId)
	}
	var cart model.Cart
	if req.Id > 0 {
		err := w.DB.Where("id = ?", req.Id).Take(&cart).Error
		if err != nil {
			return err
		}
	} else {
		tx := w.DB.Where("archive_id = ? and `sku_id` = ?", req.ArchiveId, req.SkuId)
		if req.UserId > 0 {
			tx = tx.Where("user_id = ?", req.UserId)
		} else {
			tx = tx.Where("token = ?", req.Token)
		}
		err := tx.Take(&cart).Error
		if err != nil {
			return err
		}
		req.Id = cart.Id
	}
	if req.UserId > 0 && cart.UserId != req.UserId {
		return errors.New(w.TplTr("DocumentDoesNotExist"))
	}
	if req.UserId == 0 && cart.Token != req.Token {
		return errors.New(w.TplTr("DocumentDoesNotExist"))
	}
	if req.SkuId > 0 {
		cart.SkuId = req.SkuId
		cart.SkuValues = req.SkuValues
	}
	if req.Quantity > 0 {
		cart.Quantity = req.Quantity
	}
	if req.OrderExtra != nil {
		cart.OrderExtra = req.OrderExtra
	}
	// 获取商品
	archive, err := w.GetArchiveById(cart.ArchiveId)
	if err != nil {
		return err
	}
	if archive.MinQuantity > 0 && cart.Quantity < archive.MinQuantity {
		cart.Quantity = archive.MinQuantity
	}
	if archive.MaxQuantity > 0 && cart.Quantity > archive.MaxQuantity {
		cart.Quantity = archive.MaxQuantity
	}
	cart.OriginPrice = archive.Price
	if req.SkuId > 0 {
		archiveSku, err := w.GetArchiveSkuById(cart.ArchiveId, cart.SkuId)
		if err != nil {
			return err
		}
		cart.OriginPrice = archiveSku.Price
		// 如果不允许超卖，需要判断剩余库存
		if archive.AllowOversold == false && int64(cart.Quantity) > archiveSku.Stock {
			return errors.New("out of stock")
		}
	} else {
		// 如果不允许超卖，需要判断剩余库存
		if archive.AllowOversold == false && int64(cart.Quantity) > archive.Stock {
			return errors.New("out of stock")
		}
	}

	return w.DB.Where("id = ?", req.Id).Updates(&cart).Error
}

func (w *Website) CartDelete(req *request.CartRequest) error {
	if req.UserId > 0 && req.Token != "" {
		// 更新 购物车
		_ = w.DB.Model(&model.Cart{}).Where("token = ?", req.Token).Update("user_id", req.UserId)
	}
	var cart model.Cart
	err := w.DB.Where("id = ?", req.Id).Take(&cart).Error
	if err != nil {
		return err
	}
	if req.UserId > 0 && cart.UserId != req.UserId {
		return errors.New(w.TplTr("DocumentDoesNotExist"))
	}
	if req.UserId == 0 && cart.Token != req.Token {
		return errors.New(w.TplTr("DocumentDoesNotExist"))
	}

	return w.DB.Delete(&cart).Error
}
