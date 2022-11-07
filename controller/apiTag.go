package controller

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/structs"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
)

func ApiArchiveDetail(ctx iris.Context) {
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")
	archive, err := provider.GetArchiveById(id)
	if err != nil {
		if filename != "" {
			archive, err = provider.GetArchiveByUrlToken(filename)
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
	if userId > 0 {
		if archive.Price > 0 {
			archive.HasOrdered = provider.CheckArchiveHasOrder(userId, archive.Id)
			userInfo, _ := ctx.Values().Get("userInfo").(*model.User)
			discount := provider.GetUserDiscount(userId, userInfo)
			if discount > 0 {
				archive.FavorablePrice = archive.Price * discount / 100
			}
		}
		if archive.ReadLevel > 0 && !archive.HasOrdered {
			userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
			if userGroup != nil && userGroup.Level >= archive.ReadLevel {
				archive.HasOrdered = true
			}
		}
	}
	// if read level larger than 0, then need to check permission
	if archive.ReadLevel > 0 && !archive.HasOrdered {
		archive.ArchiveData = &model.ArchiveData{
			Content: fmt.Sprintf(config.Lang("该内容需要用户等级%d以上才能阅读"), archive.ReadLevel),
		}
	} else {
		// 读取data
		archive.ArchiveData, _ = provider.GetArchiveDataById(archive.Id)
	}
	// 读取分类
	archive.Category = provider.GetCategoryFromCache(archive.CategoryId)
	// 读取 extraDate
	archive.Extra = provider.GetArchiveExtra(archive.ModuleId, archive.Id)
	for i := range archive.Extra {
		if archive.Extra[i].Value == nil || archive.Extra[i].Value == "" {
			archive.Extra[i].Value = archive.Extra[i].Default
		}
	}
	tags := provider.GetTagsByItemId(archive.Id)
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
	moduleId := uint(ctx.URLParamIntDefault("moduleId", 0))

	module := provider.GetModuleFromCache(moduleId)
	if module == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  config.Lang("模型不存在"),
		})
		return
	}

	allText := config.Lang("全部")

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
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	moduleId := uint(ctx.URLParamIntDefault("moduleId", 0))
	var categoryIds []uint
	var categoryDetail *model.Category
	tmpCatId := ctx.URLParam("categoryId")
	if tmpCatId != "" {
		tmpIds := strings.Split(tmpCatId, ",")
		for _, v := range tmpIds {
			tmpId, _ := strconv.Atoi(v)
			if tmpId > 0 {
				categoryDetail = provider.GetCategoryFromCache(uint(tmpId))
				if categoryDetail != nil {
					categoryIds = append(categoryIds, categoryDetail.Id)
					moduleId = categoryDetail.ModuleId
				}
			}
		}
	}

	module := provider.GetModuleFromCache(moduleId)

	order := ctx.URLParam("order")
	limit := 10
	offset := 0
	currentPage := ctx.URLParamIntDefault("page", 1)
	listType := ctx.URLParamDefault("type", "list")
	flag := ctx.URLParam("flag")
	q := ctx.URLParam("q")
	child := true

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

	var archives []*model.Archive
	var total int64
	if listType == "related" {
		//获取id
		var categoryId = uint(0)
		if len(categoryIds) > 0 {
			categoryId = categoryIds[0]
		}
		if archiveId > 0 {
			archive, err := provider.GetArchiveById(archiveId)
			if err == nil {
				categoryId = archive.CategoryId
				category := provider.GetCategoryFromCache(categoryId)
				if category != nil {
					moduleId = category.ModuleId
				}
			}
		}

		var archives2 []*model.Archive
		db := dao.DB
		newLimit := int(math.Ceil(float64(limit) / 2))
		if err := db.Model(&model.Archive{}).Where("`module_id` = ? AND `category_id` = ? AND `status` = 1 AND `id` > ?", moduleId, categoryId, archiveId).Order("id ASC").Limit(newLimit).Offset(offset).Find(&archives).Error; err != nil {
			//no
		}
		preCount := len(archives)
		newLimit += newLimit - len(archives)
		if err := db.Model(&model.Archive{}).Where("`module_id` = ? AND `category_id` = ? AND `status` = 1 AND `id` < ?", moduleId, categoryId, archiveId).Order("id DESC").Limit(newLimit).Offset(offset).Find(&archives2).Error; err != nil {
			//no
		}
		//列表不返回content
		if len(archives2) > 0 {
			archives = append(archives, archives2...)
		}
		// 如果量不够，则再补充
		if len(archives) < limit {
			var archives3 []*model.Archive
			newLimit = limit - len(archives)
			db.Model(&model.Archive{}).Where("`status` = 1").Where("`module_id` = ? AND `category_id` = ? AND `status` = 1 AND `id` > ?", moduleId, categoryId, archiveId).Order("id ASC").Limit(newLimit).Offset(offset + preCount).Find(&archives3)
			if len(archives3) > 0 {
				archives = append(archives, archives3...)
			}
		}
		// 如果数量超过，则截取
		if len(archives) > limit {
			archives = archives[:limit]
		}
	} else {
		builder := dao.DB.Model(&model.Archive{}).Where("`status` = 1")

		if moduleId > 0 {
			builder = builder.Where("module_id = ?", moduleId)
		}

		if flag != "" {
			builder = builder.Where("FIND_IN_SET(?,`flag`)", flag)
		}

		extraFields := map[uint]map[string]*model.CustomField{}
		var results []map[string]interface{}
		var fields []string
		fields = append(fields, "id")

		if module != nil && len(module.Fields) > 0 {
			for _, v := range module.Fields {
				fields = append(fields, "`"+v.FieldName+"`")
				// 如果有筛选条件，从这里开始筛选
				if param, ok := extraParams[v.FieldName]; ok {
					builder = builder.Where("`"+v.FieldName+"` = ?", param)
				}
			}
		}

		if len(categoryIds) > 0 {
			if child {
				var subIds []uint
				for _, v := range categoryIds {
					tmpIds := provider.GetSubCategoryIds(v, nil)
					subIds = append(subIds, tmpIds...)
					subIds = append(subIds, v)
				}
				builder = builder.Where("`category_id` IN(?)", subIds)
			} else if len(categoryIds) == 1 {
				builder = builder.Where("`category_id` = ?", categoryIds[0])
			} else {
				builder = builder.Where("`category_id` IN(?)", categoryIds)
			}
		}
		if order != "" {
			builder = builder.Order(order)
		}
		if listType == "page" {
			if currentPage > 1 {
				offset = (currentPage - 1) * limit
			}
			if q != "" {
				builder = builder.Where("`title` like ?", "%"+q+"%")
			}
			builder.Count(&total)
		}
		builder = builder.Limit(limit).Offset(offset)
		if err := builder.Find(&archives).Error; err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		var archiveIds = make([]uint, 0, len(archives))
		for i := range archives {
			archiveIds = append(archiveIds, archives[i].Id)
		}
		if module != nil && len(fields) > 0 && len(archiveIds) > 0 {
			dao.DB.Table(module.TableName).Where("`id` IN(?)", archiveIds).Select(strings.Join(fields, ",")).Scan(&results)
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

	for i := range archives {
		archives[i].Link = provider.GetUrl("archive", archives[i], 0)
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  archives,
	})
}

func ApiArchiveParams(ctx iris.Context) {
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	sorted := true
	sortedTmp, err := ctx.URLParamBool("sorted")
	if err == nil {
		sorted = sortedTmp
	}

	archiveDetail, err := provider.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	archiveParams := provider.GetArchiveExtra(archiveDetail.ModuleId, archiveDetail.Id)

	for i := range archiveParams {
		if archiveParams[i].Value == nil || archiveParams[i].Value == "" {
			archiveParams[i].Value = archiveParams[i].Default
		}
	}
	if sorted {
		var extraFields []*model.CustomField
		module := provider.GetModuleFromCache(archiveDetail.ModuleId)
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
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")
	catname := ctx.URLParam("catname")
	if catname != "" {
		filename = catname
	}
	category, err := provider.GetCategoryById(id)
	if err != nil {
		if filename != "" {
			category, err = provider.GetCategoryByUrlToken(filename)
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

	categoryList := provider.GetCategoriesFromCache(moduleId, parentId, config.CategoryTypeArchive)
	var resultList []*model.Category
	for i := 0; i < len(categoryList); i++ {
		if offset > i {
			continue
		}
		if limit > 0 && i >= (limit+offset) {
			break
		}
		categoryList[i].Link = provider.GetUrl("category", categoryList[i], 0)
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
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
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

	commentList, total, _ := provider.GetCommentList(archiveId, order, currentPage, limit, offset)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  commentList,
	})
}

func ApiContact(ctx iris.Context) {
	var settings = map[string]interface{}{}

	reflectFields := structs.Fields(config.JsonData.Contact)

	for _, v := range reflectFields {
		if v.Name() != "ExtraFields" {
			value := v.Value()
			if v.Name() == "Qrcode" {
				value = config.JsonData.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(value.(string), "/")
			}
			settings[v.Name()] = value
		}
	}

	if config.JsonData.Contact.ExtraFields != nil {
		for i := range config.JsonData.Contact.ExtraFields {
			settings[config.JsonData.Contact.ExtraFields[i].Name] = config.JsonData.Contact.ExtraFields[i].Value
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": settings,
	})
}

func ApiSystem(ctx iris.Context) {
	var settings = map[string]interface{}{}

	reflectFields := structs.Fields(config.JsonData.System)

	for _, v := range reflectFields {
		if v.Name() != "ExtraFields" {
			value := v.Value()
			if v.Name() == "SiteLogo" {
				value = config.JsonData.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(value.(string), "/")
			}
			settings[v.Name()] = value
		}
	}

	if config.JsonData.System.ExtraFields != nil {
		for i := range config.JsonData.System.ExtraFields {
			settings[config.JsonData.System.ExtraFields[i].Name] = config.JsonData.System.ExtraFields[i].Value
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": settings,
	})
}

func ApiGuestbook(ctx iris.Context) {
	fields := config.GetGuestbookFields()
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
	linkList, _ := provider.GetLinkList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": linkList,
	})
}

func ApiNavList(ctx iris.Context) {
	typeId := ctx.URLParamIntDefault("typeId", 1)
	navList := provider.GetNavsFromCache(uint(typeId))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": navList,
	})
}

func ApiNextArchive(ctx iris.Context) {
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	archiveDetail, err := provider.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var nextArchive model.Archive
	if err2 := dao.DB.Model(&model.Archive{}).Where("`module_id` = ? AND `category_id` = ?", archiveDetail.ModuleId, archiveDetail.CategoryId).Where("`id` > ?", archiveDetail.Id).Where("`status` = 1").First(&nextArchive).Error; err2 == nil {
		nextArchive.GetThumb()
		nextArchive.Link = provider.GetUrl("archive", &nextArchive, 0)

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": nextArchive,
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": nil,
	})
}

func ApiPrevArchive(ctx iris.Context) {
	archiveId := uint(ctx.URLParamIntDefault("id", 0))
	archiveDetail, err := provider.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var prevArchive model.Archive
	if err2 := dao.DB.Model(&model.Archive{}).Where("`module_id` = ? AND `category_id` = ?", archiveDetail.ModuleId, archiveDetail.CategoryId).Where("`id` < ?", archiveDetail.Id).Where("`status` = 1").Last(&prevArchive).Error; err2 == nil {
		prevArchive.GetThumb()
		prevArchive.Link = provider.GetUrl("archive", &prevArchive, 0)

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": prevArchive,
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": nil,
	})
}

func ApiPageDetail(ctx iris.Context) {
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")

	category, err := provider.GetCategoryById(id)
	if err != nil {
		if filename != "" {
			category, err = provider.GetCategoryByUrlToken(filename)
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
	pageList := provider.GetCategoriesFromCache(0, 0, config.CategoryTypePage)
	for i := range pageList {
		pageList[i].Link = provider.GetUrl("page", pageList[i], 0)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pageList,
	})
}

func ApiTagDetail(ctx iris.Context) {
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")

	tagDetail, err := provider.GetTagById(id)
	if err != nil {
		if filename != "" {
			tagDetail, err = provider.GetTagByUrlToken(filename)
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
		tagDetail.Link = provider.GetUrl("tag", tagDetail, 0)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": tagDetail,
	})
}

func ApiTagDataList(ctx iris.Context) {
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")

	tagDetail, err := provider.GetTagById(id)
	if err != nil {
		if filename != "" {
			tagDetail, err = provider.GetTagByUrlToken(filename)
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

	var total int64
	var archives []*model.Archive

	builder := dao.DB.Table("`archives` as a").Joins("INNER JOIN `tag_data` as t ON a.id = t.item_id AND t.`tag_id` = ?", tagDetail.Id).Where("a.`status` = 1").Order(order)

	if listType == "page" {
		if currentPage > 1 {
			offset = (currentPage - 1) * limit
		}
		builder.Count(&total)
	}

	builder = builder.Limit(limit).Offset(offset)
	if err := builder.Find(&archives).Error; err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	for i := range archives {
		archives[i].Link = provider.GetUrl("archive", archives[i], 0)
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  archives,
	})
}

func ApiTagList(ctx iris.Context) {
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

	tagList, total, _ := provider.GetTagList(itemId, "", letter, currentPage, limit, offset)
	for i := range tagList {
		tagList[i].Link = provider.GetUrl("tag", tagList[i], 0)
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  tagList,
	})
}

func ApiAttachmentUpload(ctx iris.Context) {
	AttachmentUpload(ctx)
}

func ApiCommentPublish(ctx iris.Context) {
	var req request.PluginComment
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		body, _ := ctx.GetBody()
		library.DebugLog("error", err.Error(), string(body))
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
		parent, err := provider.GetCommentById(req.ParentId)
		if err == nil {
			req.ToUid = parent.UserId
		}
	}

	comment, err := provider.SaveComment(&req)
	if err != nil {
		msg := config.Lang("保存失败")
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  msg,
		})
	}

	msg := config.Lang("发布成功")
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  msg,
		"data": comment,
	})
}

func ApiCommentPraise(ctx iris.Context) {
	var req request.PluginComment
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		body, _ := ctx.GetBody()
		library.DebugLog("error", err.Error(), string(body))
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment, err := provider.GetCommentById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment.VoteCount += 1
	err = comment.Save(dao.DB)
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
		"msg":  config.Lang("点赞成功"),
		"data": comment,
	})
}

func ApiGuestbookForm(ctx iris.Context) {
	fields := config.GetGuestbookFields()
	var req = map[string]interface{}{}
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		body, _ := ctx.GetBody()
		library.DebugLog("error", err.Error(), string(body))
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

	err = dao.DB.Save(guestbook).Error
	if err != nil {
		msg := config.Lang("保存失败")
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  msg,
		})
		return
	}

	//发送邮件
	subject := fmt.Sprintf(config.Lang("%s有来自%s的新留言"), config.JsonData.System.SiteName, guestbook.UserName)
	var contents []string
	for _, item := range fields {
		content := fmt.Sprintf("%s：%s\n", item.Name, req[item.FieldName])

		contents = append(contents, content)
	}
	// 增加来路和IP返回
	contents = append(contents, fmt.Sprintf("%s：%s\n", config.Lang("提交IP"), guestbook.Ip))
	contents = append(contents, fmt.Sprintf("%s：%s\n", config.Lang("来源页面"), guestbook.Refer))
	contents = append(contents, fmt.Sprintf("%s：%s\n", config.Lang("提交时间"), time.Now().Format("2006-01-02 15:04:05")))

	// 后台发信
	go provider.SendMail(subject, strings.Join(contents, ""))

	msg := config.JsonData.PluginGuestbook.ReturnMessage
	if msg == "" {
		msg = config.Lang("感谢您的留言！")
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  msg,
	})
}

func ApiArchivePublish(ctx iris.Context) {
	var req request.Archive
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	userId := ctx.Values().GetIntDefault("userId", 0)
	req.Draft = true
	req.UserId = uint(userId)

	// read body twice
	var extraReq = map[string]interface{}{}
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		body, _ := ctx.GetBody()
		library.DebugLog("error", err.Error(), string(body))
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req.Extra = extraReq

	archive, err := provider.SaveArchive(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  config.Lang("发布成功，已进入审核"),
		"data": archive,
	})
}
