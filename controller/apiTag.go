package controller

import (
	"fmt"
	"gorm.io/gorm"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/structs"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func ApiArchiveDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")
	var archive *model.Archive
	var err error
	archive = currentSite.GetArchiveByIdFromCache(id)
	if archive == nil {
		archive, err = currentSite.GetArchiveById(id)
		if archive != nil {
			currentSite.AddArchiveCache(archive)
		}
	}
	if err != nil {
		if filename != "" {
			archive, err = currentSite.GetArchiveByUrlToken(filename)
		}
	}
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	userId := ctx.Values().GetUintDefault("userId", 0)
	// if read level larger than 0, then need to check permission
	userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
	archive = currentSite.CheckArchiveHasOrder(userId, archive, userGroup)
	if archive.Price > 0 {
		userInfo, _ := ctx.Values().Get("userInfo").(*model.User)
		discount := currentSite.GetUserDiscount(userId, userInfo)
		if discount > 0 {
			archive.FavorablePrice = archive.Price * discount / 100
		}
	}

	// if read level larger than 0, then need to check permission
	if archive.ReadLevel > 0 && !archive.HasOrdered {
		archive.ArchiveData = &model.ArchiveData{
			Content: fmt.Sprintf(currentSite.Lang("该内容需要用户等级%d以上才能阅读"), archive.ReadLevel),
		}
	} else {
		// 读取data
		archive.ArchiveData, _ = currentSite.GetArchiveDataById(archive.Id)
	}
	// 读取分类
	archive.Category = currentSite.GetCategoryFromCache(archive.CategoryId)
	// 读取 extraDate
	archive.Extra = currentSite.GetArchiveExtra(archive.ModuleId, archive.Id, true)
	for i := range archive.Extra {
		if archive.Extra[i].Value == nil || archive.Extra[i].Value == "" {
			archive.Extra[i].Value = archive.Extra[i].Default
		}
		if archive.Extra[i].FollowLevel && !archive.HasOrdered {
			delete(archive.Extra, i)
		}
	}
	tags := currentSite.GetTagsByItemId(archive.Id)
	if len(tags) > 0 {
		var tagNames = make([]string, 0, len(tags))
		for _, v := range tags {
			tagNames = append(tagNames, v.Title)
		}
		archive.Tags = tagNames
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": archive,
	})
}

func ApiArchiveFilters(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	moduleId := uint(ctx.URLParamIntDefault("moduleId", 0))

	module := currentSite.GetModuleFromCache(moduleId)
	if module == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Lang("模型不存在"),
		})
		return
	}

	allText := currentSite.Lang("全部")

	tmpText := ctx.URLParam("allText")
	if tmpText != "" {
		if tmpText == "false" {
			allText = ""
		} else {
			allText = tmpText
		}
	}

	// 只有有多项选择的才能进行筛选，如 单选，多选，下拉
	var filterFields []config.CustomField
	var filterGroups []response.FilterGroup

	if len(module.Fields) > 0 {
		for _, v := range module.Fields {
			if v.IsFilter {
				filterFields = append(filterFields, v)
			}
		}

		// 所有参数的url都附着到query中
		for _, v := range filterFields {
			values := v.SplitContent()
			if len(values) == 0 {
				continue
			}

			var filterItems []response.FilterItem
			if allText != "" {
				// 需要插入 全部 标签
				filterItems = append(filterItems, response.FilterItem{
					Label: allText,
				})
			}
			for _, val := range values {
				filterItems = append(filterItems, response.FilterItem{
					Label: val,
				})
			}
			filterGroups = append(filterGroups, response.FilterGroup{
				Name:      v.Name,
				FieldName: v.FieldName,
				Items:     filterItems,
			})
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": filterGroups,
	})
}

func ApiArchiveList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	moduleId := uint(ctx.URLParamIntDefault("moduleId", 0))
	authorId := uint(ctx.URLParamIntDefault("authorId", 0))
	var categoryIds []uint
	var categoryDetail *model.Category
	tmpCatId := ctx.URLParam("categoryId")
	if tmpCatId != "" {
		tmpIds := strings.Split(tmpCatId, ",")
		for _, v := range tmpIds {
			tmpId, _ := strconv.Atoi(v)
			if tmpId > 0 {
				categoryDetail = currentSite.GetCategoryFromCache(uint(tmpId))
				if categoryDetail != nil {
					categoryIds = append(categoryIds, categoryDetail.Id)
					moduleId = categoryDetail.ModuleId
				}
			}
		}
	}

	module := currentSite.GetModuleFromCache(moduleId)

	order := ctx.URLParam("order")
	limit := 10
	offset := 0
	currentPage := ctx.URLParamIntDefault("page", 1)
	listType := ctx.URLParamDefault("type", "list")
	flag := ctx.URLParam("flag")
	q := ctx.URLParam("q")
	child := true
	if currentPage < 1 {
		currentPage = 1
	}

	childTmp, err := ctx.URLParamBool("child")
	if err == nil {
		child = childTmp
	}
	limitTmp := ctx.URLParam("limit")
	if limitTmp != "" {
		limitArgs := strings.Split(limitTmp, ",")
		if len(limitArgs) == 2 {
			offset, _ = strconv.Atoi(limitArgs[0])
			limit, _ = strconv.Atoi(limitArgs[1])
		} else if len(limitArgs) == 1 {
			limit, _ = strconv.Atoi(limitArgs[0])
		}
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}

	// 支持更多的参数搜索，
	extraParams := make(url.Values)
	for k, v := range ctx.URLParams() {
		if k == "page" {
			continue
		}
		if listType == "page" {
			if v != "" {
				extraParams.Set(k, v)
			}
		}
	}
	if listType == "page" {
		if currentPage > 1 {
			offset = (currentPage - 1) * limit
		}
	}

	var archives []*model.Archive
	var total int64
	if listType == "related" {
		//获取id
		var categoryId = uint(0)
		var keywords string
		if len(categoryIds) > 0 {
			categoryId = categoryIds[0]
		}
		if archiveId > 0 {
			archive, err := currentSite.GetArchiveById(archiveId)
			if err == nil {
				categoryId = archive.CategoryId
				keywords = strings.Split(strings.ReplaceAll(archive.Keywords, "，", ","), ",")[0]
				category := currentSite.GetCategoryFromCache(categoryId)
				if category != nil {
					moduleId = category.ModuleId
				}
			}
		}
		// 允许通过keywords调用
		like := ctx.URLParam("like")
		tmpKeyword := ctx.URLParam("keywords")
		if len(tmpKeyword) > 0 {
			keywords = tmpKeyword
		}

		if like == "keywords" {
			archives, _, _ = currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				tx = tx.Where("`module_id` = ? AND `category_id` = ? AND `status` = 1 AND `keywords` like ? AND `id` != ?", moduleId, categoryId, "%"+keywords+"%", archiveId).
					Order("id ASC")
				return tx
			}, 0, limit, offset)
		} else {
			newLimit := int(math.Ceil(float64(limit) / 2))
			archives, _, _ = currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				tx = tx.Where("`module_id` = ? AND `category_id` = ? AND `status` = 1 AND `id` > ?", moduleId, categoryId, archiveId).
					Order("id ASC")
				return tx
			}, 0, newLimit, offset)
			newLimit += newLimit - len(archives)
			archives2, _, _ := currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				tx = tx.Where("`module_id` = ? AND `category_id` = ? AND `status` = 1 AND `id` < ?", moduleId, categoryId, archiveId).
					Order("id DESC")
				return tx
			}, 0, newLimit, offset)
			//列表不返回content
			if len(archives2) > 0 {
				archives = append(archives, archives2...)
			}
			// 如果数量超过，则截取
			if len(archives) > limit {
				archives = archives[:limit]
			}
		}
	} else {
		extraFields := map[uint]map[string]*model.CustomField{}
		var results []map[string]interface{}
		var fields []string
		fields = append(fields, "id")

		var fulltextSearch bool
		var fulltextTotal int64
		var err2 error
		var ids []uint
		if listType == "page" && len(q) > 0 {
			ids, fulltextTotal, err2 = currentSite.Search(q, moduleId, currentPage, limit)
			if err2 == nil {
				fulltextSearch = true
				if len(ids) == 0 {
					ids = append(ids, 0)
				}
				offset = 0
			}
		}
		ops := func(tx *gorm.DB) *gorm.DB {
			tx.Where("`status` = 1")
			if authorId > 0 {
				tx = tx.Where("user_id = ?", authorId)
			}
			if moduleId > 0 {
				tx = tx.Where("`module_id` = ?", moduleId)
			}
			if flag != "" {
				tx = tx.Where("FIND_IN_SET(?,`flag`)", flag)
			}
			if module != nil && len(module.Fields) > 0 {
				for _, v := range module.Fields {
					fields = append(fields, "`"+v.FieldName+"`")
					// 如果有筛选条件，从这里开始筛选
					if param, ok := extraParams[v.FieldName]; ok {
						tx = tx.Where("`"+v.FieldName+"` = ?", param)
					}
				}
			}
			if len(categoryIds) > 0 {
				if child {
					var subIds []uint
					for _, v := range categoryIds {
						tmpIds := currentSite.GetSubCategoryIds(v, nil)
						subIds = append(subIds, tmpIds...)
						subIds = append(subIds, v)
					}
					tx = tx.Where("`category_id` IN(?)", subIds)
				} else if len(categoryIds) == 1 {
					tx = tx.Where("`category_id` = ?", categoryIds[0])
				} else {
					tx = tx.Where("`category_id` IN(?)", categoryIds)
				}
			}
			if order != "" {
				tx = tx.Order(order)
			}
			if len(ids) > 0 {
				tx = tx.Where("`id` IN(?)", ids)
			} else if q != "" {
				tx = tx.Where("`title` like ?", "%"+q+"%")
			}
			return tx
		}
		if listType != "page" {
			// 如果不是分页，则不查询count
			currentPage = 0
		}
		archives, total, _ = currentSite.GetArchiveList(ops, currentPage, limit, offset)
		if fulltextSearch {
			total = fulltextTotal
		}
		var archiveIds = make([]uint, 0, len(archives))
		for i := range archives {
			archiveIds = append(archiveIds, archives[i].Id)
		}
		if module != nil && len(fields) > 0 && len(archiveIds) > 0 {
			currentSite.DB.Table(module.TableName).Where("`id` IN(?)", archiveIds).Select(strings.Join(fields, ",")).Scan(&results)
			for _, field := range results {
				item := map[string]*model.CustomField{}
				for _, v := range module.Fields {
					item[v.FieldName] = &model.CustomField{
						Name:  v.Name,
						Value: field[v.FieldName],
					}
				}
				if id, ok := field["id"].(uint32); ok {
					extraFields[uint(id)] = item
				}
			}
			for i := range archives {
				if extraFields[archives[i].Id] != nil {
					archives[i].Extra = extraFields[archives[i].Id]
				}
			}
		}
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  archives,
	})
}

func ApiArchiveParams(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	sorted := true
	sortedTmp, err := ctx.URLParamBool("sorted")
	if err == nil {
		sorted = sortedTmp
	}

	archiveDetail, err := currentSite.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	archiveParams := currentSite.GetArchiveExtra(archiveDetail.ModuleId, archiveDetail.Id, true)
	userId := ctx.Values().GetUintDefault("userId", 0)
	// if read level larger than 0, then need to check permission
	userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
	archiveDetail = currentSite.CheckArchiveHasOrder(userId, archiveDetail, userGroup)

	for i := range archiveParams {
		if archiveParams[i].Value == nil || archiveParams[i].Value == "" {
			archiveParams[i].Value = archiveParams[i].Default
		}
		if archiveParams[i].FollowLevel && !archiveDetail.HasOrdered {
			delete(archiveParams, i)
		}
	}
	if sorted {
		var extraFields []*model.CustomField
		module := currentSite.GetModuleFromCache(archiveDetail.ModuleId)
		if module != nil && len(module.Fields) > 0 {
			for _, v := range module.Fields {
				extraFields = append(extraFields, archiveParams[v.FieldName])
			}
		}

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": extraFields,
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": archiveParams,
	})
}

func ApiCategoryDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")
	catname := ctx.URLParam("catname")
	if catname != "" {
		filename = catname
	}
	category, err := currentSite.GetCategoryById(id)
	if err != nil {
		if filename != "" {
			category, err = currentSite.GetCategoryByUrlToken(filename)
		}
	}
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
		"data": category,
	})
}

func ApiCategoryList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	moduleId := uint(ctx.URLParamIntDefault("moduleId", 0))
	parentId := uint(ctx.URLParamIntDefault("parentId", 0))
	limit := 0
	offset := 0
	limitTmp := ctx.URLParam("limit")
	if limitTmp != "" {
		limitArgs := strings.Split(limitTmp, ",")
		if len(limitArgs) == 2 {
			offset, _ = strconv.Atoi(limitArgs[0])
			limit, _ = strconv.Atoi(limitArgs[1])
		} else if len(limitArgs) == 1 {
			limit, _ = strconv.Atoi(limitArgs[0])
		}
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}

	categoryList := currentSite.GetCategoriesFromCache(moduleId, parentId, config.CategoryTypeArchive)
	var resultList []*model.Category
	for i := 0; i < len(categoryList); i++ {
		if offset > i {
			continue
		}
		if limit > 0 && i >= (limit+offset) {
			break
		}
		categoryList[i].GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		categoryList[i].Link = currentSite.GetUrl("category", categoryList[i], 0)
		categoryList[i].IsCurrent = false
		resultList = append(resultList, categoryList[i])
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": resultList,
	})
}

func ApiCommentList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	userId := uint(ctx.URLParamIntDefault("user_id", 0))
	order := ctx.URLParamDefault("order", "id desc")
	limit := 10
	offset := 0
	currentPage := ctx.URLParamIntDefault("page", 1)
	//listType :=  ctx.URLParamDefault("type", "list")

	limitTmp := ctx.URLParam("limit")
	if limitTmp != "" {
		limitArgs := strings.Split(limitTmp, ",")
		if len(limitArgs) == 2 {
			offset, _ = strconv.Atoi(limitArgs[0])
			limit, _ = strconv.Atoi(limitArgs[1])
		} else if len(limitArgs) == 1 {
			limit, _ = strconv.Atoi(limitArgs[0])
		}
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}

	commentList, total, _ := currentSite.GetCommentList(archiveId, userId, order, currentPage, limit, offset)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  commentList,
	})
}

func ApiContact(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var settings = map[string]interface{}{}

	reflectFields := structs.Fields(currentSite.Contact)

	for _, v := range reflectFields {
		if v.Name() != "ExtraFields" {
			value := v.Value()
			if v.Name() == "Qrcode" {
				value = currentSite.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(value.(string), "/")
			}
			settings[v.Name()] = value
		}
	}

	if currentSite.Contact.ExtraFields != nil {
		for i := range currentSite.Contact.ExtraFields {
			settings[currentSite.Contact.ExtraFields[i].Name] = currentSite.Contact.ExtraFields[i].Value
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": settings,
	})
}

func ApiSystem(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var settings = map[string]interface{}{}

	reflectFields := structs.Fields(currentSite.System)

	for _, v := range reflectFields {
		if v.Name() != "ExtraFields" {
			value := v.Value()
			if v.Name() == "SiteLogo" {
				value = currentSite.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(value.(string), "/")
			}
			settings[v.Name()] = value
		}
	}

	if currentSite.System.ExtraFields != nil {
		for i := range currentSite.System.ExtraFields {
			settings[currentSite.System.ExtraFields[i].Name] = currentSite.System.ExtraFields[i].Value
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": settings,
	})
}

func ApiGuestbook(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	fields := currentSite.GetGuestbookFields()
	for i := range fields {
		//分割items
		fields[i].SplitContent()
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": fields,
	})
}

func ApiLinkList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	linkList, _ := currentSite.GetLinkList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": linkList,
	})
}

func ApiNavList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	typeId := ctx.URLParamIntDefault("typeId", 1)
	navList := currentSite.GetNavsFromCache(uint(typeId))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": navList,
	})
}

func ApiNextArchive(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	archiveDetail, err := currentSite.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	nextArchive, _ := currentSite.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`module_id` = ? AND `category_id` = ?", archiveDetail.ModuleId, archiveDetail.CategoryId).Where("`id` > ?", archiveDetail.Id).Where("`status` = 1").Order("`id` ASC")
	})

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": nextArchive,
	})
}

func ApiPrevArchive(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	archiveDetail, err := currentSite.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	prevArchive, _ := currentSite.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`module_id` = ? AND `category_id` = ?", archiveDetail.ModuleId, archiveDetail.CategoryId).Where("`id` < ?", archiveDetail.Id).Where("`status` = 1").Order("`id` DESC")
	})

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": prevArchive,
	})
}

func ApiPageDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")

	category, err := currentSite.GetCategoryById(id)
	if err != nil {
		if filename != "" {
			category, err = currentSite.GetCategoryByUrlToken(filename)
		}
	}
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
		"data": category,
	})
}

func ApiPageList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	pageList := currentSite.GetCategoriesFromCache(0, 0, config.CategoryTypePage)
	for i := range pageList {
		pageList[i].Link = currentSite.GetUrl("page", pageList[i], 0)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pageList,
	})
}

func ApiTagDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")

	tagDetail, err := currentSite.GetTagById(id)
	if err != nil {
		if filename != "" {
			tagDetail, err = currentSite.GetTagByUrlToken(filename)
		}
	}
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if tagDetail != nil {
		tagDetail.Link = currentSite.GetUrl("tag", tagDetail, 0)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": tagDetail,
	})
}

func ApiTagDataList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")

	tagDetail, err := currentSite.GetTagById(id)
	if err != nil {
		if filename != "" {
			tagDetail, err = currentSite.GetTagByUrlToken(filename)
		}
	}
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	limit := 10
	offset := 0
	currentPage := ctx.URLParamIntDefault("page", 1)
	order := ctx.URLParamDefault("order", "id desc")
	listType := ctx.URLParamDefault("type", "list")

	limitTmp := ctx.URLParam("limit")
	if limitTmp != "" {
		limitArgs := strings.Split(limitTmp, ",")
		if len(limitArgs) == 2 {
			offset, _ = strconv.Atoi(limitArgs[0])
			limit, _ = strconv.Atoi(limitArgs[1])
		} else if len(limitArgs) == 1 {
			limit, _ = strconv.Atoi(limitArgs[0])
		}
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}

	if listType == "page" {
		if currentPage > 1 {
			offset = (currentPage - 1) * limit
		}
	}
	archives, total, _ := currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
		tx = tx.Table("`archives` as a").
			Joins("INNER JOIN `tag_data` as t ON a.id = t.item_id AND t.`tag_id` = ?", tagDetail.Id).
			Where("a.`status` = 1").
			Order(order)
		return tx
	}, currentPage, limit, offset)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  archives,
	})
}

func ApiTagList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	limit := 10
	offset := 0
	currentPage := ctx.URLParamIntDefault("page", 1)
	itemId := uint(ctx.URLParamIntDefault("itemId", 0))
	listType := ctx.URLParamDefault("type", "list")
	letter := ctx.URLParam("letter")

	limitTmp := ctx.URLParam("limit")
	if limitTmp != "" {
		limitArgs := strings.Split(limitTmp, ",")
		if len(limitArgs) == 2 {
			offset, _ = strconv.Atoi(limitArgs[0])
			limit, _ = strconv.Atoi(limitArgs[1])
		} else if len(limitArgs) == 1 {
			limit, _ = strconv.Atoi(limitArgs[0])
		}
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}

	if listType == "page" {
		if currentPage > 1 {
			offset = (currentPage - 1) * limit
		}
	}

	tagList, total, _ := currentSite.GetTagList(itemId, "", letter, currentPage, limit, offset)
	for i := range tagList {
		tagList[i].Link = currentSite.GetUrl("tag", tagList[i], 0)
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  tagList,
	})
}

func ApiBannerList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	bannerList := currentSite.Banner
	for i := range bannerList {
		if !strings.HasPrefix(bannerList[i].Logo, "http") && !strings.HasPrefix(bannerList[i].Logo, "//") {
			bannerList[i].Logo = currentSite.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(bannerList[i].Logo, "/")
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": bannerList,
	})
}

func ApiAttachmentUpload(ctx iris.Context) {
	AttachmentUpload(ctx)
}

func ApiCommentPublish(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginComment
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	userId := ctx.Values().GetIntDefault("userId", 0)
	if userId > 0 {
		req.Status = 1
	}

	req.UserId = uint(userId)
	if req.Ip == "" {
		req.Ip = ctx.RemoteAddr()
	}
	if req.ParentId > 0 {
		parent, err := currentSite.GetCommentById(req.ParentId)
		if err == nil {
			req.ToUid = parent.UserId
		}
	}

	comment, err := currentSite.SaveComment(&req)
	if err != nil {
		msg := currentSite.Lang("保存失败")
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  msg,
		})
	}

	msg := currentSite.Lang("发布成功")
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  msg,
		"data": comment,
	})
}

func ApiCommentPraise(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginComment
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment, err := currentSite.GetCommentById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment.VoteCount += 1
	err = comment.Save(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment.Active = true

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.Lang("点赞成功"),
		"data": comment,
	})
}

func ApiGuestbookForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	fields := currentSite.GetGuestbookFields()
	var req = map[string]interface{}{}
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	var result = map[string]string{}
	extraData := map[string]interface{}{}
	for _, item := range fields {
		var val string
		if item.Type == config.CustomFieldTypeCheckbox {
			tmpVal, ok := req[item.FieldName].([]string)
			if ok {
				val = strings.Trim(strings.Join(tmpVal, ","), ",")
			}
		} else {
			val, _ = req[item.FieldName].(string)
		}

		if item.Required && val == "" {
			msg := fmt.Sprintf("%s必填", item.Name)
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  msg,
			})
			return
		}
		result[item.FieldName] = strings.TrimSpace(val)
		if !item.IsSystem {
			extraData[item.Name] = val
		}
	}

	//先填充默认字段
	guestbook := &model.Guestbook{
		UserName:  result["user_name"],
		Contact:   result["contact"],
		Content:   result["content"],
		Ip:        ctx.RemoteAddr(),
		Refer:     ctx.Request().Referer(),
		ExtraData: extraData,
	}

	err = currentSite.DB.Save(guestbook).Error
	if err != nil {
		msg := currentSite.Lang("保存失败")
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  msg,
		})
		return
	}

	//发送邮件
	subject := fmt.Sprintf(currentSite.Lang("%s有来自%s的新留言"), currentSite.System.SiteName, guestbook.UserName)
	var contents []string
	for _, item := range fields {
		content := fmt.Sprintf("%s：%s\n", item.Name, req[item.FieldName])

		contents = append(contents, content)
	}
	// 增加来路和IP返回
	contents = append(contents, fmt.Sprintf("%s：%s\n", currentSite.Lang("提交IP"), guestbook.Ip))
	contents = append(contents, fmt.Sprintf("%s：%s\n", currentSite.Lang("来源页面"), guestbook.Refer))
	contents = append(contents, fmt.Sprintf("%s：%s\n", currentSite.Lang("提交时间"), time.Now().Format("2006-01-02 15:04:05")))

	// 后台发信
	go currentSite.SendMail(subject, strings.Join(contents, ""))

	msg := currentSite.PluginGuestbook.ReturnMessage
	if msg == "" {
		msg = currentSite.Lang("感谢您的留言！")
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  msg,
	})
}

func ApiArchivePublish(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.Archive
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if currentSite.Safe.APIPublish != 1 {
		req.Draft = true
		return
	}
	userId := ctx.Values().GetIntDefault("userId", 0)
	req.UserId = uint(userId)
	req.Extra = map[string]interface{}{}
	// read body twice
	var extraReq = map[string]interface{}{}
	var err error
	if err = ctx.ReadJSON(&extraReq); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	for k := range extraReq {
		req.Extra[k] = map[string]interface{}{
			"value": extraReq[k],
		}
	}

	archive, err := currentSite.SaveArchive(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	archive.Link = currentSite.GetUrl("archive", archive, 0)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.Lang("发布成功，已进入审核"),
		"data": archive,
	})
}
