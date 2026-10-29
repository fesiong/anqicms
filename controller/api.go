package controller

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func ApiImportArchive(ctx iris.Context) {
	if ctx.Method() == "GET" {
		// 用于检查待发布的文章是否存在
		ApiImportGetArchive(ctx)
		return
	}
	currentSite := provider.CurrentSite(ctx)
	id := ctx.PostValueInt64Default("id", 0)
	parentId := ctx.PostValueInt64Default("parent_id", 0)
	placeId := ctx.PostValueInt64Default("place_id", 0)
	title := ctx.PostValueTrim("title")
	seoTitle := ctx.PostValueTrim("seo_title")
	content := ctx.PostValueTrim("content")
	categoryId := uint(ctx.PostValueIntDefault("category_id", 0))
	// 支持分类名称
	categoryTitle := ctx.PostValueTrim("category_title")
	keywords := ctx.PostValueTrim("keywords")
	description := ctx.PostValueTrim("description")
	logo := ctx.PostValueTrim("logo")
	publishTime := ctx.PostValueTrim("publish_time")
	tmpTag := ctx.PostValueTrim("tag")
	images, _ := ctx.PostValues("images[]")
	urlToken := ctx.PostValueTrim("url_token")
	draft, _ := ctx.PostValueBool("draft")
	cover, _ := ctx.PostValueInt("cover") // 0=不覆盖，提示错误，1=覆盖，2=继续插入
	if cover == 0 {
		// 兼容旧的bool
		boolCover, _ := ctx.PostValueBool("cover")
		if boolCover {
			cover = 1
		}
	}
	if len(images) == 1 && strings.HasPrefix(images[0], "[") {
		err := json.Unmarshal([]byte(images[0]), &images)
		if err != nil {
			images = nil
		}
	}
	// 验证 images
	for i := 0; i < len(images); i++ {
		if len(images[i]) < 10 && !(strings.HasPrefix(images[i], "http") || strings.HasPrefix(images[i], "/")) {
			// 删除它
			images = append(images[:i], images[i+1:]...)
			i--
		}
	}
	template := ctx.PostValueTrim("template")
	canonicalUrl := ctx.PostValueTrim("canonical_url")
	fixedLink := ctx.PostValueTrim("fixed_link")
	flag := ctx.PostValueTrim("flag")
	price := ctx.PostValueInt64Default("price", 0)
	originPrice := ctx.PostValueInt64Default("origin_price", 0)
	stock := ctx.PostValueInt64Default("stock", 0)
	readLevel := ctx.PostValueIntDefault("read_level", 0)
	sort := uint(ctx.PostValueIntDefault("sort", 0))
	views := uint(ctx.PostValueIntDefault("views", 0))
	originUrl := ctx.PostValueTrim("origin_url")
	// 新增字段
	userId := uint(ctx.PostValueIntDefault("user_id", 0))
	password := ctx.PostValueTrim("password")
	costPrice := ctx.PostValueInt64Default("cost_price", 0)
	videoUrl := ctx.PostValueTrim("video_url")
	needLogistics, _ := ctx.PostValueBool("need_logistics")
	isFreeShipping, _ := ctx.PostValueBool("is_free_shipping")
	soldCount := ctx.PostValueInt64Default("sold_count", 0)
	reviewCount := ctx.PostValueInt64Default("review_count", 0)
	optionType := ctx.PostValueIntDefault("option_type", 0)
	weight, _ := strconv.ParseFloat(ctx.PostValueTrim("weight"), 64)
	weightUnit := ctx.PostValueTrim("weight_unit")
	length, _ := strconv.ParseFloat(ctx.PostValueTrim("length"), 64)
	width, _ := strconv.ParseFloat(ctx.PostValueTrim("width"), 64)
	height, _ := strconv.ParseFloat(ctx.PostValueTrim("height"), 64)
	volume, _ := strconv.ParseFloat(ctx.PostValueTrim("volume"), 64)
	lengthUnit := ctx.PostValueTrim("length_unit")
	allowOversold, _ := ctx.PostValueBool("allow_oversold")
	minQuantity := ctx.PostValueIntDefault("min_quantity", 0)
	maxQuantity := ctx.PostValueIntDefault("max_quantity", 0)
	stepQuantity := ctx.PostValueIntDefault("step_quantity", 0)
	productType := ctx.PostValueIntDefault("product_type", 1)
	keywordId := uint(ctx.PostValueIntDefault("keyword_id", 0))
	originTitle := ctx.PostValueTrim("origin_title")
	brand := ctx.PostValueTrim("brand")
	hasPseudo := ctx.PostValueIntDefault("has_pseudo", 0)
	originId := ctx.PostValueIntDefault("origin_id", 0)
	commentCount := uint(ctx.PostValueInt64Default("comment_count", 0))
	favoriteCount := uint(ctx.PostValueInt64Default("favorite_count", 0))
	averageScore, _ := strconv.ParseFloat(ctx.PostValueTrim("average_score"), 32)
	// 处理 JSON 数组字段
	tmpFlags, _ := ctx.PostValues("flags[]")
	var flags []string
	if len(tmpFlags) == 1 && strings.HasPrefix(tmpFlags[0], "[") {
		_ = json.Unmarshal([]byte(tmpFlags[0]), &flags)
	} else {
		flags = tmpFlags
	}
	tmpRelationIds, _ := ctx.PostValues("relation_ids[]")
	var relationIds []int64
	if len(tmpRelationIds) == 1 && strings.HasPrefix(tmpRelationIds[0], "[") {
		_ = json.Unmarshal([]byte(tmpRelationIds[0]), &relationIds)
	} else {
		for _, v := range tmpRelationIds {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				relationIds = append(relationIds, id)
			}
		}
	}
	orderFieldsStr := ctx.PostValueTrim("order_fields")
	skusStr := ctx.PostValueTrim("skus")
	optionsStr := ctx.PostValueTrim("options")
	wholesaleRulesStr := ctx.PostValueTrim("wholesale_rules")
	// 解析JSON字段
	var orderFields []config.CustomField
	if orderFieldsStr != "" {
		_ = json.Unmarshal([]byte(orderFieldsStr), &orderFields)
	}
	var importSkus []*request.ArchiveSku
	if skusStr != "" {
		_ = json.Unmarshal([]byte(skusStr), &importSkus)
	}
	var importOptions []*request.ArchiveOption
	if optionsStr != "" {
		_ = json.Unmarshal([]byte(optionsStr), &importOptions)
	}
	var wholesaleRules []*request.WholesaleRule
	if wholesaleRulesStr != "" {
		_ = json.Unmarshal([]byte(wholesaleRulesStr), &wholesaleRules)
	}
	tmpCategoryIds, _ := ctx.PostValues("category_ids[]")
	var categoryIds []uint
	if len(tmpCategoryIds) > 0 {
		for i := range tmpCategoryIds {
			tmpCatId, _ := strconv.Atoi(tmpCategoryIds[i])
			if tmpCatId > 0 {
				categoryIds = append(categoryIds, uint(tmpCatId))
			}
		}
	}
	if len(categoryIds) == 0 && categoryId > 0 {
		categoryIds = append(categoryIds, categoryId)
	}
	if len(categoryIds) == 0 {
		// 支持分类名称
		if len(categoryTitle) > 0 {
			category, err := currentSite.GetCategoryByTitle(categoryTitle)
			if err != nil {
				// 分类不存在，创建
				moduleId := uint(ctx.PostValueIntDefault("module_id", 0))
				if moduleId == 0 {
					moduleId = 1
				}
				category, err = currentSite.SaveCategory(&request.Category{
					Title:    categoryTitle,
					ModuleId: moduleId,
					Status:   library.Ptr(uint(1)),
					Type:     config.CategoryTypeArchive,
				})
			}
			if category != nil {
				categoryIds = append(categoryIds, category.Id)
			}
		}
		if len(categoryIds) == 0 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("PleaseSelectAColumn"),
			})
			return
		}
	}
	for _, catId := range categoryIds {
		category := currentSite.GetCategoryFromCache(catId)
		if category == nil || category.Type != config.CategoryTypeArchive {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("PleaseSelectAColumn"),
			})
			return
		}
	}
	categoryId = categoryIds[0]
	category := currentSite.GetCategoryFromCache(categoryId)
	if category == nil || category.Type != config.CategoryTypeArchive {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseSelectAColumn"),
		})
		return
	}
	module := currentSite.GetModuleFromCache(category.ModuleId)
	if module == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("UndefinedModel"),
		})
		return
	}

	if title == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheArticleTitle"),
		})
		return
	}
	if content == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheArticleContent"),
		})
		return
	}

	// 表单通道无法区分「字段没传」与「传了空」，用原始文本是否为空判断字段是否出现：
	// 数字 "0" / 布尔 "false" 都是非空文本，因此显式传 0（售罄）与 false（不包邮）
	// 都能被正确识别为「调用方要求写入」。
	provided := func(key string) bool {
		return strings.TrimSpace(ctx.PostValue(key)) != ""
	}
	var req = request.Archive{
		Title:          title,
		CategoryId:     categoryId,
		Content:        content,
		Flag:           flag,
		OriginUrl:      originUrl,
		OriginTitle:    originTitle,
		UrlToken:       urlToken,
		Draft:          draft,
		Extra:          map[string]interface{}{},
		Skus:           importSkus,
		Options:        importOptions,
		WholesaleRules: wholesaleRules,
	}
	// 指针字段：只有字段真的出现在表单里才写，没出现就保持原值（新建时用模型默认值）。
	if provided("parent_id") {
		req.ParentId = library.Ptr(parentId)
	}
	if provided("place_id") {
		req.PlaceId = library.Ptr(uint(placeId))
	}
	if seoTitle != "" {
		req.SeoTitle = library.Ptr(seoTitle)
	}
	if len(categoryIds) > 0 {
		req.CategoryIds = library.Ptr(categoryIds)
	}
	if keywords != "" {
		req.Keywords = library.Ptr(keywords)
	}
	if description != "" {
		req.Description = library.Ptr(description)
	}
	if template != "" {
		req.Template = library.Ptr(template)
	}
	if canonicalUrl != "" {
		req.CanonicalUrl = library.Ptr(canonicalUrl)
	}
	if fixedLink != "" {
		req.FixedLink = library.Ptr(fixedLink)
	}
	if len(flags) > 0 {
		req.Flags = library.Ptr(flags)
	}
	if len(images) > 0 {
		req.Images = library.Ptr(images)
	}
	if len(relationIds) > 0 {
		req.RelationIds = library.Ptr(relationIds)
	}
	if provided("user_id") {
		req.UserId = library.Ptr(userId)
	}
	if provided("price") {
		req.Price = library.Ptr(price)
	}
	if provided("origin_price") {
		req.OriginPrice = library.Ptr(originPrice)
	}
	if provided("cost_price") {
		req.CostPrice = library.Ptr(costPrice)
	}
	if provided("stock") {
		req.Stock = library.Ptr(stock)
	}
	if provided("read_level") {
		req.ReadLevel = library.Ptr(readLevel)
	}
	if provided("password") {
		req.Password = library.Ptr(password)
	}
	if provided("sort") {
		req.Sort = library.Ptr(sort)
	}
	if provided("views") {
		req.Views = library.Ptr(views)
	}
	if provided("keyword_id") {
		req.KeywordId = library.Ptr(keywordId)
	}
	if provided("comment_count") {
		req.CommentCount = library.Ptr(commentCount)
	}
	if provided("favorite_count") {
		req.FavoriteCount = library.Ptr(favoriteCount)
	}
	if provided("average_score") {
		req.AverageScore = library.Ptr(float32(averageScore))
	}
	if videoUrl != "" {
		req.VideoUrl = library.Ptr(videoUrl)
	}
	if provided("need_logistics") {
		req.NeedLogistics = library.Ptr(needLogistics)
	}
	if provided("is_free_shipping") {
		req.IsFreeShipping = library.Ptr(isFreeShipping)
	}
	if provided("sold_count") {
		req.SoldCount = library.Ptr(soldCount)
	}
	if provided("review_count") {
		req.ReviewCount = library.Ptr(reviewCount)
	}
	if provided("option_type") {
		req.OptionType = library.Ptr(optionType)
	}
	if provided("weight") {
		req.Weight = library.Ptr(weight)
	}
	if weightUnit != "" {
		req.WeightUnit = library.Ptr(weightUnit)
	}
	if provided("length") {
		req.Length = library.Ptr(length)
	}
	if provided("width") {
		req.Width = library.Ptr(width)
	}
	if provided("height") {
		req.Height = library.Ptr(height)
	}
	if provided("volume") {
		req.Volume = library.Ptr(volume)
	}
	if lengthUnit != "" {
		req.LengthUnit = library.Ptr(lengthUnit)
	}
	if provided("allow_oversold") {
		req.AllowOversold = library.Ptr(allowOversold)
	}
	if provided("min_quantity") {
		req.MinQuantity = library.Ptr(minQuantity)
	}
	if provided("max_quantity") {
		req.MaxQuantity = library.Ptr(maxQuantity)
	}
	if provided("step_quantity") {
		req.StepQuantity = library.Ptr(stepQuantity)
	}
	if provided("product_type") {
		req.ProductType = library.Ptr(productType)
	}
	if brand != "" {
		req.Brand = library.Ptr(brand)
	}
	if provided("has_pseudo") {
		req.HasPseudo = library.Ptr(hasPseudo)
	}
	if provided("origin_id") {
		req.OriginId = library.Ptr(originId)
	}
	if len(orderFields) > 0 {
		req.OrderFields = library.Ptr(orderFields)
	}

	// 如果传了ID，则采用覆盖的形式
	if id > 0 {
		_, err := currentSite.GetArchiveById(id)
		_, err2 := currentSite.GetArchiveDraftById(id)
		if err != nil && err2 != nil {
			// 不存在，创建一个
			archiveDraft := model.ArchiveDraft{
				Archive: model.Archive{
					ParentId:       parentId,
					PlaceId:        uint(placeId),
					Title:          title,
					SeoTitle:       seoTitle,
					UrlToken:       urlToken,
					Keywords:       keywords,
					Description:    description,
					ModuleId:       category.ModuleId,
					CategoryId:     categoryId,
					Logo:           logo,
					UserId:         userId,
					Price:          price,
					OriginPrice:    originPrice,
					CostPrice:      costPrice,
					Stock:          stock,
					ReadLevel:      readLevel,
					Password:       password,
					Sort:           sort,
					Views:          views,
					OriginUrl:      originUrl,
					OriginTitle:    originTitle,
					KeywordId:      keywordId,
					CommentCount:   commentCount,
					FavoriteCount:  favoriteCount,
					AverageScore:   float32(averageScore),
					Brand:          brand,
					HasPseudo:      hasPseudo,
					OriginId:       originId,
					VideoUrl:       videoUrl,
					NeedLogistics:  needLogistics,
					IsFreeShipping: isFreeShipping,
					SoldCount:      soldCount,
					ReviewCount:    reviewCount,
					OptionType:     optionType,
					Weight:         weight,
					WeightUnit:     weightUnit,
					Length:         length,
					Width:          width,
					Height:         height,
					Volume:         volume,
					LengthUnit:     lengthUnit,
					AllowOversold:  allowOversold,
					MinQuantity:    minQuantity,
					MaxQuantity:    maxQuantity,
					StepQuantity:   stepQuantity,
					ProductType:    productType,
				},
			}
			if views > 0 {
				archiveDraft.Views = views
			}
			archiveDraft.Id = id
			err = currentSite.DB.Create(&archiveDraft).Error

			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("FailedToImportTheArticle"),
				})
				return
			}
			req.Id = id
		} else {
			// 已存在
			if cover == 1 {
				req.Id = id
			} else if cover == 0 {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("DocumentIdIsRepeated"),
				})
				return
			}
		}
	} else {
		// 标题重复的不允许导入
		var existId int64
		exists, err := currentSite.GetArchiveByTitle(title)
		if err == nil {
			existId = exists.Id
		} else {
			// 也需要判断draft表
			exists2, err2 := currentSite.GetArchiveDraftByTitle(title)
			if err2 == nil {
				existId = exists2.Id
			}
		}
		if existId > 0 {
			if cover == 1 {
				req.Id = existId
			} else if cover == 0 {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("DocumentTitleIsRepeated"),
				})
				return
			}
		}
		if len(originUrl) > 0 {
			// 标题重复的不允许导入
			exists, err = currentSite.GetArchiveByOriginUrl(originUrl)
			if err == nil {
				existId = exists.Id
			} else {
				// 也需要判断draft表
				exists2, err2 := currentSite.GetArchiveDraftByOriginUrl(title)
				if err2 == nil {
					existId = exists2.Id
				}
			}
			if existId > 0 {
				if cover == 1 {
					req.Id = existId
				} else if cover == 0 {
					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  ctx.Tr("DocumentIsRepeated"),
					})
					return
				}
			}
		}
	}

	if publishTime != "" {
		timeStamp, err := time.ParseInLocation("2006-01-02 15:04:05", publishTime, time.Local)
		if err == nil {
			req.CreatedTime = timeStamp.Unix()
		}
	}
	if logo != "" {
		imgs := library.Deref(req.Images)
		req.Images = library.Ptr(append(imgs, logo))
	}
	if tmpTag != "" {
		tags := strings.Split(strings.ReplaceAll(tmpTag, "，", ","), ",")
		req.Tags = library.Ptr(tags)
	}

	// 处理extraFields
	if len(module.Fields) > 0 {
		for _, v := range module.Fields {
			if v.Type == config.CustomFieldTypeCheckbox {
				// 多选值
				value, _ := ctx.PostValues(v.FieldName)
				if len(value) > 0 {
					req.Extra[v.FieldName] = map[string]interface{}{
						"value": value,
					}
				}
			} else if v.Type == config.CustomFieldTypeNumber {
				value := ctx.PostValueIntDefault(v.FieldName, 0)
				if value > 0 {
					req.Extra[v.FieldName] = map[string]interface{}{
						"value": value,
					}
				}
			} else {
				value := ctx.PostValue(v.FieldName)
				if value != "" {
					if v.Type == config.CustomFieldTypeImages || v.Type == config.CustomFieldTypeTexts || v.Type == config.CustomFieldTypeArchive || v.Type == config.CustomFieldTypeTimeline {
						// 提交的是JSON字符串
						var extra any
						err := json.Unmarshal([]byte(value), &extra)
						if err == nil {
							req.Extra[v.FieldName] = map[string]interface{}{
								"value": extra,
							}
						} else {
							req.Extra[v.FieldName] = map[string]interface{}{
								"value": value,
							}
						}
					} else {
						req.Extra[v.FieldName] = map[string]interface{}{
							"value": value,
						}
					}
				}
			}
		}
	}
	// UpdateAll 决定「未传的字段」怎么处理：
	//   true  —— 全量覆盖。未传=零值的字段会被清空。这是**表单提交**的语义：
	//            前端提交整个表单，某个字段没出现在表单里就意味着用户清空了它。
	//   false —— PATCH 语义。只覆盖显式传入的字段，未传的一律保持库里的原值。
	//            这是 **AI/程序化调用**需要的语义：只想改标题，不该动其它字段。
	//
	// 用 Partial（反向开关）而非直接暴露 UpdateAll，是为了区分「调用方没传这个字段」
	// 与「调用方显式传了 false」——Go 的 bool 零值做不到，前端又不传该字段。
	// Partial 优先于调用方传的 update_all。
	if !req.Partial {
		req.UpdateAll = true
	}
	archive, err := currentSite.SaveArchive(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("PublishSuccessfully"),
		"data": iris.Map{
			"url": currentSite.GetUrl("archive", archive, 0),
			"id":  archive.Id,
		},
	})
}

func ApiImportGetArchive(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.URLParamInt64Default("id", 0)
	title := ctx.URLParam("title")
	urlToken := ctx.URLParam("url_token")
	originUrl := ctx.URLParam("origin_url")

	if id > 0 {
		archive, err := currentSite.GetArchiveById(id)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archive,
			})
			return
		}
		archiveDraft, err := currentSite.GetArchiveDraftById(id)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archiveDraft,
			})
			return
		}
	}
	if len(title) > 0 {
		archive, err := currentSite.GetArchiveByTitle(title)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archive,
			})
			return
		}
		archiveDraft, err := currentSite.GetArchiveDraftByTitle(title)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archiveDraft,
			})
			return
		}
	}
	if len(urlToken) > 0 {
		archive, err := currentSite.GetArchiveByUrlToken(urlToken)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archive,
			})
			return
		}
		archiveDraft, err := currentSite.GetArchiveDraftByOriginUrl(urlToken)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archiveDraft,
			})
			return
		}
	}
	if len(originUrl) > 0 {
		archive, err := currentSite.GetArchiveByOriginUrl(originUrl)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archive,
			})
			return
		}
		archiveDraft, err := currentSite.GetArchiveDraftByOriginUrl(originUrl)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archiveDraft,
			})
			return
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  ctx.Tr("DocumentDoesNotExist"),
	})
}

func ApiImportGetCategories(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	tmpModuleId := ctx.FormValue("module_id")
	moduleId, _ := strconv.Atoi(tmpModuleId)
	showType, _ := strconv.Atoi(ctx.FormValue("show_type"))

	if moduleId > 0 {
		module := currentSite.GetModuleFromCache(uint(moduleId))

		if module == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("UndefinedModel"),
			})
			return
		}
	}

	tmpCategories, _ := currentSite.GetCategories(func(tx *gorm.DB) *gorm.DB {
		tx = tx.Where("`type` = ? and `status` = ?", config.CategoryTypeArchive, config.ContentStatusOK)
		if moduleId > 0 {
			tx = tx.Where("`module_id` = ?", moduleId)
		}
		return tx
	}, 0, showType)

	var categories []response.ApiCategory
	for i := range tmpCategories {
		categories = append(categories, response.ApiCategory{
			Id:       tmpCategories[i].Id,
			ParentId: tmpCategories[i].ParentId,
			Title:    tmpCategories[i].Title,
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusApiSuccess,
		"msg":  ctx.Tr("SuccessfulAcquisition"),
		"data": categories,
	})
}

// ApiImportMakeSitemap 通过API接口生成Sitemap
func ApiImportMakeSitemap(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	async := ctx.FormValue("async")

	if async == "true" || async == "1" {
		go func() {
			err := currentSite.BuildSitemap()
			if err == nil {
				pluginSitemap := currentSite.PluginSitemap

				//由于sitemap的更新可能很频繁，因此sitemap的更新时间直接写入一个文件中
				pluginSitemap.UpdatedTime = currentSite.GetSitemapTime()
				// 写入Sitemap的url
				frontUrl := currentSite.System.BaseUrl
				if currentSite.System.FrontUrl != "" {
					frontUrl = currentSite.System.FrontUrl
				}
				pluginSitemap.SitemapURL = frontUrl + "/sitemap." + pluginSitemap.Type

				currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSitemapManually"))
			}
		}()

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("SubmittedForBackgroundProcessing"),
		})
		return
	}
	//开始生成sitemap
	err := currentSite.BuildSitemap()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	pluginSitemap := currentSite.PluginSitemap

	//由于sitemap的更新可能很频繁，因此sitemap的更新时间直接写入一个文件中
	pluginSitemap.UpdatedTime = currentSite.GetSitemapTime()
	// 写入Sitemap的url
	frontUrl := currentSite.System.BaseUrl
	if currentSite.System.FrontUrl != "" {
		frontUrl = currentSite.System.FrontUrl
	}
	pluginSitemap.SitemapURL = frontUrl + "/sitemap." + pluginSitemap.Type

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSitemapManually"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SitemapUpdated"),
		"data": pluginSitemap,
	})
}
func ApiImportCreateFriendLink(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	// 增加支持 didi 友链的批量导入
	form := library.NewForm(ctx.Request().Form)
	var otherList []map[string]string
	err := form.Bind(&otherList, "other_list")
	if err == nil && len(otherList) > 0 {
		for _, item := range otherList {
			friendLink, err := currentSite.GetLinkByLinkAndTitle(item["url"], item["name"])
			if err != nil {
				friendLink = &model.Link{}
			}
			friendLink.Title = item["name"]
			friendLink.Link = item["url"]
			friendLink.Contact = item["qq"]
			friendLink.Status = 0
			friendLink.Save(currentSite.DB)
		}

		currentSite.DeleteCacheIndex()

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("LinkSaved"),
		})
		return
	}

	title := ctx.PostValueTrim("title")
	link := ctx.PostValueTrim("link")
	if linkUrl := ctx.PostValueTrim("url"); linkUrl != "" {
		link = linkUrl
	}
	logo := ctx.PostValueTrim("logo")
	nofollow := uint(ctx.PostValueIntDefault("nofollow", 0))
	backLink := ctx.PostValueTrim("back_link")
	myTitle := ctx.PostValueTrim("my_title")
	myLink := ctx.PostValueTrim("my_link")
	contact := ctx.PostValueTrim("contact")
	if qq := ctx.PostValueTrim("qq"); qq != "" {
		contact = qq
	}
	if email := ctx.PostValueTrim("email"); email != "" {
		contact = email
	}
	remark := ctx.PostValueTrim("remark")

	friendLink, err := currentSite.GetLinkByLinkAndTitle(link, title)
	if err != nil {
		friendLink = &model.Link{
			Status: 0,
		}
	}

	friendLink.Title = title
	friendLink.Link = link
	if backLink != "" {
		friendLink.BackLink = backLink
	}
	if myTitle != "" {
		friendLink.MyTitle = myTitle
	}
	if myLink != "" {
		friendLink.MyLink = myLink
	}
	if contact != "" {
		friendLink.Contact = contact
	}
	if remark != "" {
		friendLink.Remark = remark
	}
	friendLink.Logo = logo
	friendLink.Nofollow = nofollow
	friendLink.Status = 0

	err = friendLink.Save(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 保存完毕，实时监测
	go currentSite.PluginLinkCheck(friendLink)

	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LinkSaved"),
	})
}

func ApiImportDeleteFriendLink(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	link := ctx.PostValueTrim("link")
	if linkUrl := ctx.PostValueTrim("url"); linkUrl != "" {
		link = linkUrl
	}
	title := ctx.PostValueTrim("title")

	if link == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("LinkRequired"),
		})
		return
	}

	friendLink, err := currentSite.GetLinkByLinkAndTitle(link, title)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("LinkDoesNotExist"),
		})
		return
	}

	err = friendLink.Delete(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LinkDeleted"),
	})
}

func ApiAddSubscriber(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SubscriberRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Email == "" || !currentSite.VerifyEmailFormat(req.Email) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheEmail"),
		})
		return
	}
	_, err := currentSite.GetSubscriberByEmail(req.Email)
	if err == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("OperationSuccessful"),
		})
		return
	}
	req.Id = 0
	_, err = currentSite.SaveSubscriber(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("OperationSuccessful"),
	})
}

func ApiImportCheckFriendLink(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("VerificationSuccessful"),
	})
}

func VerifyApiToken(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	token := ctx.FormValue("token")
	if token != currentSite.PluginImportApi.Token {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("TokenError"),
		})
		return
	}

	ctx.Next()
}

func VerifyApiLinkToken(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	token := ctx.FormValue("token")
	if didiToken := ctx.GetHeader("didi-token"); didiToken != "" {
		token = didiToken
	}
	if token != currentSite.PluginImportApi.LinkToken {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("TokenError"),
		})
		return
	}

	ctx.Next()
}

func CheckApiOpen(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if 1 != currentSite.Safe.APIOpen {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("ApiInterfaceFunctionIsNotOpen"),
		})
		return
	}

	ctx.Next()
}

// 接收 post 请求
func ApiLogStatistic(ctx iris.Context) {
	// no-index
	// x-robots-tag: noindex
	ctx.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	ctx.Header("Cache-Control", "post-check=0, pre-check=0") // 部分旧版浏览器需要
	ctx.Header("Pragma", "no-cache")                         // HTTP 1.0 兼容
	ctx.Header("X-Robots-Tag", "noindex, nofollow")
	ctx.Header("Expires", "Thu, 01 Jan 1970 00:00:00 GMT") // 过期时间设为过去
	var req request.LogStatisticRequest
	err := ctx.ReadBody(&req)
	if err != nil {
		ctx.StatusCode(iris.StatusNoContent)
		return
	}
	currentSite := provider.CurrentSite(ctx)

	if req.Action == request.LogActionViews {
		LogAccess(ctx, &req)
		// 记录archive的浏览量
		if req.Type == request.LogTypeArchive {
			archive := currentSite.GetArchiveByIdFromCache(req.Id)
			if archive != nil {
				_ = archive.AddViews(currentSite.DB)
				// 产品判定走 modules.kind，不再硬编码 id==2
				if currentSite.IsProductModule(archive.ModuleId) {
					// 统计文章的阅读数
					req.Type = request.LogTypeProductView
					// 推送事件
					userId := ctx.Values().GetUintDefault("userId", 0)
					if userId > 0 {
						userInfo, ok := ctx.Values().Get("userInfo").(*model.User)
						if !ok {
							userInfo = &model.User{
								Id: userId,
							}
						}
						// 推送事件
						currentSite.CreateEvent(provider.EventProductViewd, map[string]interface{}{
							"id":    archive.Id,
							"title": archive.Title,
							"link":  archive.Link,
							"logo":  archive.Logo,
							"price": archive.Price / 100,
						}, userInfo)
					}
				} else {
					// 其它
					req.Type = request.LogTypeArticleView
				}
			}
		}
	}
	_ = currentSite.ShopCountLog.Write(&req)

	ctx.StatusCode(iris.StatusNoContent)
}

func ApiImportReview(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	orderId := ctx.PostValueTrim("order_id")
	archiveId := ctx.PostValueInt64Default("archive_id", 0)
	archiveSkuId := ctx.PostValueInt64Default("archive_sku_id", 0)
	userName := ctx.PostValueTrim("user_name")
	email := ctx.PostValueTrim("email")
	score := ctx.PostValueIntDefault("score", 5)
	content := ctx.PostValueTrim("content")
	images, _ := ctx.PostValues("images[]")
	if len(images) == 1 && strings.HasPrefix(images[0], "[") {
		err := json.Unmarshal([]byte(images[0]), &images)
		if err != nil {
			images = nil
		}
	}
	userId := uint(ctx.PostValueIntDefault("user_id", 0))
	var user *model.User
	var err error
	if userId > 0 {
		user, err = currentSite.GetUserInfoById(userId)
		if err == nil {
			userName = user.RealName
			email = user.Email
		}
	}
	var order *model.Order
	if orderId != "" {
		order, err = currentSite.GetOrderInfoByOrderId(orderId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}

	if score == 0 {
		score = 5
	}

	review := request.OrderReviewRequest{
		UserId:       userId,
		OrderId:      orderId,
		ArchiveId:    archiveId,
		ArchiveSkuId: archiveSkuId,
		Ip:           ctx.RemoteAddr(),
		UserName:     userName,
		Email:        email,
		Score:        score,
		Content:      content,
		Images:       images,
		Status:       1,
	}

	if order != nil {
		for _, v := range order.Details {
			newItem := review
			newItem.ArchiveId = v.GoodsId
			newItem.ArchiveSkuId = v.GoodsSkuId
			err = currentSite.SaveOrderReview(&newItem)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
		}
	} else {
		err = currentSite.SaveOrderReview(&review)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "OK",
	})
}
