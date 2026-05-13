package provider

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	content "google.golang.org/api/content/v2.1"
	"google.golang.org/api/option"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
)

func (w *Website) GetGoogleMerchantConfig(focus bool) *oauth2.Config {
	if w.googleMerchantConfig == nil || focus {
		setting := w.GetGoogleMerchantSetting()
		if setting.ClientId == "" || setting.ClientSecret == "" {
			w.googleMerchantConfig = nil
			return nil
		}

		w.googleMerchantConfig = &oauth2.Config{
			ClientID:     setting.ClientId,
			ClientSecret: setting.ClientSecret,
			RedirectURL:  setting.RedirectUrl,
			Scopes: []string{
				"openid",
				"https://www.googleapis.com/auth/userinfo.email", // You have to select your own scope from here -> https://developers.google.com/identity/protocols/googlescopes#google_sign-in
				"https://www.googleapis.com/auth/userinfo.profile",
				"https://www.googleapis.com/auth/content",
			},
			Endpoint: google.Endpoint,
		}

		w2 := GetWebsite(w.Id)
		w2.googleMerchantConfig = w.googleMerchantConfig
	}

	return w.googleMerchantConfig
}

func (w *Website) GetGoogleMerchantSyncLogs(currentPage, pageSize int) ([]*model.ProductSyncLog, int64) {
	var logs []*model.ProductSyncLog
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := w.DB.Model(&model.ProductSyncLog{}).Order("id desc")

	builder.Count(&total).Limit(pageSize).Offset(offset).Find(&logs)

	return logs, total
}

func (w *Website) GetGoogleMerchantSyncStatuses(currentPage, pageSize int) ([]*model.ProductSyncStatus, int64) {
	var list []*model.ProductSyncStatus
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := w.DB.Model(&model.ProductSyncStatus{}).Order("id desc")

	builder.Count(&total).Limit(pageSize).Offset(offset).Find(&list)
	var productIds []int64
	for _, item := range list {
		productIds = append(productIds, item.ProductId)
	}
	products, _, _ := w.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("id in (?)", productIds)
	}, "id desc", 0, len(productIds))
	for _, item := range list {
		for _, product := range products {
			if item.ProductId == product.Id {
				item.Archive = product
				break
			}
		}
	}

	return list, total
}

func (w *Website) GetLastSyncGoogleLog() (*model.ProductSyncLog, error) {
	var logData model.ProductSyncLog

	err := w.DB.Where("status = ?", 2).Last(&logData).Error
	if err != nil {
		return nil, err
	}
	return &logData, nil
}

func (w *Website) StartSyncGoogleMerchantProducts(full bool, archive *model.Archive) error {
	googleCfg := w.GetGoogleMerchantConfig(true)
	if googleCfg == nil {
		return fmt.Errorf("Invalid Google Merchant Config")
	}

	setting := w.GetGoogleMerchantSetting()
	if setting.Token == nil {
		return fmt.Errorf("Invalid Google Merchant Token")
	}
	ctx := context.Background()
	// 先更新Token
	tokenSource := googleCfg.TokenSource(ctx, setting.Token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("Failed to update Google Merchant Token: %v", err)
	}
	setting.Token = newToken
	err = w.SaveSettingValue(GoogleMerchantSettingKey, setting)
	// todo 同步 model.Archive 到 google merchant center
	// 这里应该实现将产品数据同步到Google Merchant Center的逻辑
	// 需要使用Google Content API for Shopping
	// 1. 获取访问令牌
	// 2. 构造产品数据
	// 3. 调用API插入或更新产品
	client := googleCfg.Client(ctx, setting.Token)
	service, err := content.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		// 处理错误
		return err
	}

	var syncLog = model.ProductSyncLog{
		MerchantId: setting.MerchantId,
		SyncType:   "full",
		Status:     0,
		StartId:    0,
		EndId:      0,
		Message:    "",
	}
	var lastId = int64(0)
	if full {
		// 全量同步
		syncLog.SyncType = "full"
		syncLog.Status = 1
	} else {
		syncLog.SyncType = "increment"
		// 增量同步
		// 先读取上次同步记录
		lastLog, err := w.GetLastSyncGoogleLog()
		if err == nil {
			lastId = lastLog.EndId
		}
		syncLog.StartId = lastId
		syncLog.Status = 1
		syncLog.EndId = lastId
	}
	// 如果 archive ！= nil， 则只同步该产品
	if archive != nil && lastId > 0 {
		err = w.GoogleSyncSingleProduct(service, syncLog.MerchantId, archive)
		if err != nil {
			log.Println(err)
			syncLog.Status = 3
			syncLog.Message = err.Error()
			w.DB.Save(&syncLog)
			return err
		}
		syncLog.Status = 2
		w.DB.Save(&syncLog)

		return nil
	}

	for {
		var archives []*model.Archive
		w.DB.Model(model.Archive{}).Where("id > ?", lastId).Order("id asc").Limit(1000).Find(&archives)
		if len(archives) == 0 {
			break
		}
		lastId = archives[len(archives)-1].Id
		for _, arc := range archives {
			for i := 0; i < 3; i++ {
				err = w.GoogleSyncSingleProduct(service, syncLog.MerchantId, arc)
				if err == nil {
					break
				}
			}
			if err != nil {
				log.Println(err)
				syncLog.Status = 3
				syncLog.Message = err.Error()
				w.DB.Save(&syncLog)
				return err
			}
			syncLog.EndId = arc.Id
			w.DB.Save(&syncLog)
		}
	}
	syncLog.Status = 2
	w.DB.Save(&syncLog)

	return nil
}

func (w *Website) mapSpecValuesToGoogleFields(product *content.Product, variant *model.ArchiveSku) {
	if variant == nil {
		return
	}
	// 处理自定义规格：存储到customAttributes字段
	var customAttrs []*content.CustomAttribute
	for _, item := range variant.Values {
		// 颜色映射：你的"color" -> Google的color
		if strings.ToLower(item.OptionTitle) == "color" {
			product.Color = item.Title
			continue
		}
		// 尺寸映射
		if strings.ToLower(item.OptionTitle) == "size" {
			product.Sizes = []string{item.Title}
			product.SizeSystem = "US" // 根据你的市场调整
			continue
		}
		// 材质映射（如果有）
		if strings.ToLower(item.OptionTitle) == "material" {
			product.Material = item.Title
			continue
		}
		// 图案映射（如果有）
		if strings.ToLower(item.OptionTitle) == "pattern" {
			product.Pattern = item.Title
			continue
		}
		customAttrs = append(customAttrs, &content.CustomAttribute{
			Name:  item.OptionTitle,
			Value: item.Title,
		})
	}
	if len(customAttrs) > 0 {
		product.CustomAttributes = customAttrs
	}
}

func (w *Website) convertToGoogleProduct(archive *model.Archive, variant *model.ArchiveSku) (*content.Product, error) {
	currency := w.Currency.Currency
	price := &content.Price{
		Value:    fmt.Sprintf("%.2f", float64(archive.Price)/100),
		Currency: currency,
	}
	link := archive.Link
	title := archive.Title
	offerId := fmt.Sprintf("product_%d", archive.Id)
	// 库存状态映射
	availability := "out of stock"
	if archive.Stock > 0 {
		availability = "in stock"
	}
	if variant != nil {
		offerId = fmt.Sprintf("product_%d_%s", archive.Id, variant.Symbol)
		title = fmt.Sprintf("%s - %s", archive.Title, variant.Title)
		link = archive.Link + "?variant=" + variant.Symbol
		if variant.Stock > 0 {
			availability = "in stock"
		} else {
			availability = "out of stock"
		}
	}

	// 构造产品数据
	product := &content.Product{
		OfferId:              offerId,
		Title:                title,
		Description:          archive.Description,
		Link:                 link,
		ImageLink:            archive.Logo,
		AdditionalImageLinks: archive.Images,
		Price:                price,
		Availability:         availability,
		Brand:                archive.Brand,
		Condition:            "new",
	}
	if archive.OptionType == config.OptionTypeCombine {
		// 多规格处理,标识这些变体属于同一个商品组
		product.ItemGroupId = fmt.Sprintf("group_%d", archive.Id)
	}

	if variant != nil {
		// 处理规格值
		w.mapSpecValuesToGoogleFields(product, variant)
	}

	return product, nil
}

func (w *Website) GetGoogleProductSyncStatusInfo(archiveId, skuId int64) (*model.ProductSyncStatus, error) {
	var syncStatus model.ProductSyncStatus
	tx := w.DB.Model(&model.ProductSyncStatus{}).Where("product_id = ?", archiveId)
	if skuId > 0 {
		tx = tx.Where("sku_id = ?", skuId)
	} else {
		tx = tx.Where("sku_id = 0")
	}
	err := tx.Take(&syncStatus).Error
	if err != nil {
		return nil, err
	}
	return &syncStatus, nil
}

// GoogleSyncSingleProduct 同步单个产品
func (w *Website) GoogleSyncSingleProduct(service *content.APIService, merchantId uint64, archive *model.Archive) error {
	// 先上传主产品
	// 2. 转换产品格式
	googleProduct, err := w.convertToGoogleProduct(archive, nil)
	if err != nil {
		return err
	}
	syncStatus, err := w.GetGoogleProductSyncStatusInfo(archive.Id, 0)
	if err != nil {
		syncStatus = &model.ProductSyncStatus{
			MerchantId: merchantId,
			ProductId:  archive.Id,
			SkuId:      0,
		}
	}
	//调用Google API (插入或更新)
	productCall := service.Products.Insert(merchantId, googleProduct)

	result, err := productCall.Do()
	if err != nil {
		syncStatus.LastSyncTime = time.Now().Unix()
		syncStatus.LastMessage = err.Error()
		if len(syncStatus.LastMessage) > 250 {
			syncStatus.LastMessage = syncStatus.LastMessage[:250]
		}
		w.DB.Save(syncStatus)
		return err
	}
	syncStatus.GoogleProductId = result.Id
	syncStatus.LastSyncTime = time.Now().Unix()
	syncStatus.LastMessage = "success"
	syncStatus.LastSuccessTime = time.Now().Unix()
	w.DB.Save(syncStatus)

	log.Printf("update google merchant product success: %s (Google ID: %s)", result.OfferId, result.Id)
	// 检查是否是多规格
	skuResp, err := w.ApiGetSku(&request.ApiArchiveRequest{
		Id: archive.Id,
	})
	if err != nil {
		return err
	}
	if !skuResp.HasSku {
		return nil
	}
	for _, variant := range skuResp.Skus {
		variantProduct, err := w.convertToGoogleProduct(archive, variant)
		if err != nil {
			return err
		}
		variantStatus, err := w.GetGoogleProductSyncStatusInfo(archive.Id, variant.Id)
		if err != nil {
			variantStatus = &model.ProductSyncStatus{
				MerchantId: merchantId,
				ProductId:  archive.Id,
				SkuId:      variant.Id,
			}
		}
		//调用Google API (插入或更新)
		variantCall := service.Products.Insert(merchantId, variantProduct)

		result, err = variantCall.Do()
		if err != nil {
			variantStatus.LastSyncTime = time.Now().Unix()
			variantStatus.LastMessage = err.Error()
			if len(variantStatus.LastMessage) > 250 {
				variantStatus.LastMessage = variantStatus.LastMessage[:250]
			}
			w.DB.Save(variantStatus)
			return err
		}
		variantStatus.GoogleProductId = result.Id
		variantStatus.LastSyncTime = time.Now().Unix()
		variantStatus.LastMessage = "success"
		variantStatus.LastSuccessTime = time.Now().Unix()
		w.DB.Save(variantStatus)

		log.Printf("update google merchant product success: %s (Google ID: %s)", result.OfferId, result.Id)
	}

	return nil
}

func (w *Website) DeleteSyncGoogleMerchantProducts(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	var statusLogs []model.ProductSyncStatus
	w.DB.Where("product_id IN (?)", ids).Find(&statusLogs)

	googleCfg := w.GetGoogleMerchantConfig(true)
	if googleCfg == nil {
		return fmt.Errorf("Invalid Google Merchant Config")
	}

	setting := w.GetGoogleMerchantSetting()
	if setting.Token == nil {
		return fmt.Errorf("Invalid Google Merchant Token")
	}
	ctx := context.Background()
	// 先更新Token
	tokenSource := googleCfg.TokenSource(ctx, setting.Token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("Failed to update Google Merchant Token: %v", err)
	}
	setting.Token = newToken
	err = w.SaveSettingValue(GoogleMerchantSettingKey, setting)
	// todo 同步 model.Archive 到 google merchant center
	// 这里应该实现将产品数据同步到Google Merchant Center的逻辑
	// 需要使用Google Content API for Shopping
	// 1. 获取访问令牌
	// 2. 构造产品数据
	// 3. 调用API插入或更新产品
	client := googleCfg.Client(ctx, setting.Token)
	service, err := content.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		// 处理错误
		return err
	}

	var syncLog = model.ProductSyncLog{
		MerchantId: setting.MerchantId,
		SyncType:   "delete",
		Status:     0,
		StartId:    0,
		EndId:      0,
		Message:    "",
	}

	for _, item := range statusLogs {
		if syncLog.StartId == 0 {
			syncLog.StartId = item.ProductId
		}
		syncLog.Status = 1
		err = service.Products.Delete(setting.MerchantId, item.GoogleProductId).Do()
		if err != nil {
			syncLog.Message = err.Error()
			syncLog.Status = 2
			w.DB.Save(&syncLog)
			return err
		}
		syncLog.EndId = item.ProductId
		w.DB.Save(&syncLog)
	}

	syncLog.Status = 2
	w.DB.Save(&syncLog)

	return nil
}
