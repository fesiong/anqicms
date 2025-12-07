package controller

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fatih/structs"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func ApiArchiveDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.URLParamInt64Default("id", 0)
	filename := ctx.URLParam("filename")
	urlToken := ctx.URLParam("url_token")
	if filename != "" {
		urlToken = filename
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
	userInfo, _ := ctx.Values().Get("userInfo").(*model.User)
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if ctx.URLParamExists("render") {
		render, _ = ctx.URLParamBool("render")
	}
	password := ctx.URLParam("password")

	req := &request.ApiArchiveRequest{
		Id:        id,
		UrlToken:  urlToken,
		Render:    render,
		Password:  password,
		UserId:    userId,
		UserGroup: userGroup,
		UserInfo:  userInfo,
	}

	archive, err := currentSite.ApiGetArchive(req)

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
		"data": archive,
	})
}

func ApiArchiveFilters(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	moduleId := uint(ctx.URLParamIntDefault("moduleId", 0))

	allText := ctx.URLParam("allText")
	showAll := ctx.URLParamBoolDefault("showAll", false)
	if allText == "false" {
		showAll = false
	}
	showPrice := ctx.URLParamBoolDefault("showPrice", false)

	req := request.ApiFilterRequest{
		ModuleId:  int64(moduleId),
		ShowAll:   showAll,
		AllText:   allText,
		ShowPrice: showPrice,
	}

	filterGroups, err := currentSite.ApiGetFilters(&req)
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
		"data": filterGroups,
	})
}

func ApiArchiveList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := ctx.URLParamInt64Default("id", 0)
	parentId := ctx.URLParamInt64Default("parentId", 0)
	moduleId := uint(ctx.URLParamIntDefault("moduleId", 0))
	authorId := uint(ctx.URLParamIntDefault("authorId", 0))
	userId := ctx.Values().GetUintDefault("userId", 0)
	showFlag := ctx.URLParamBoolDefault("showFlag", false)
	showContent := ctx.URLParamBoolDefault("showContent", false)
	showExtra := ctx.URLParamBoolDefault("showExtra", false)
	draft := ctx.URLParamBoolDefault("draft", false)

	tmpUserId := ctx.URLParam("userId")
	if tmpUserId == "self" {
		// 获取自己的文章
		userId = ctx.Values().GetUintDefault("userId", 0)
	}
	if userId > 0 {
		authorId = userId
	}
	var categoryIds []int
	var categoryDetail *model.Category
	tmpCatId := ctx.URLParam("categoryId")
	if tmpCatId != "" {
		tmpIds := strings.Split(tmpCatId, ",")
		for _, v := range tmpIds {
			tmpId, _ := strconv.Atoi(v)
			if tmpId > 0 {
				categoryDetail = currentSite.GetCategoryFromCache(uint(tmpId))
				if categoryDetail != nil {
					categoryIds = append(categoryIds, int(categoryDetail.Id))
					moduleId = categoryDetail.ModuleId
				}
			}
		}
	}
	// 增加支持 excludeCategoryId
	var excludeCategoryIds []int
	tmpExcludeCatId := ctx.URLParam("excludeCategoryId")
	if tmpExcludeCatId != "" {
		tmpIds := strings.Split(tmpExcludeCatId, ",")
		for _, v := range tmpIds {
			tmpId, _ := strconv.Atoi(v)
			if tmpId > 0 {
				excludeCategoryIds = append(excludeCategoryIds, tmpId)
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
	render := currentSite.Content.Editor == "markdown"
	if ctx.URLParamExists("render") {
		render, _ = ctx.URLParamBool("render")
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
		if limit > currentSite.Content.MaxLimit {
			limit = currentSite.Content.MaxLimit
		}
		if limit < 1 {
			limit = 1
		}
	}

	// 支持更多的参数搜索，
	extraFields := map[string]interface{}{}

	if listType == "page" {
		if currentPage > 1 {
			offset = (currentPage - 1) * limit
		}
	}

	var fields []string
	fields = append(fields, "id")
	if module != nil && len(module.Fields) > 0 {
		for _, v := range module.Fields {
			if ctx.URLParamExists(v.FieldName) {
				extraFields[v.FieldName] = ctx.URLParam(v.FieldName)
			}
		}
	}
	tag := ctx.URLParam("tag")
	tagId := ctx.URLParamInt64Default("tagId", 0)
	like := ctx.URLParam("like")
	keywords := ctx.URLParam("keywords")

	req := request.ApiArchiveListRequest{
		Id:                 archiveId,
		Render:             render,
		ParentId:           int64(parentId),
		CategoryIds:        categoryIds,
		ExcludeCategoryIds: excludeCategoryIds,
		ModuleId:           int64(moduleId),
		AuthorId:           int64(authorId),
		ShowFlag:           showFlag,
		ShowContent:        showContent,
		ShowExtra:          showExtra,
		Draft:              draft,
		Child:              child,
		Order:              order,
		Tag:                tag,
		TagId:              int64(tagId),
		Flag:               flag,
		Q:                  q,
		Like:               like,
		Keywords:           keywords,
		Type:               listType,
		Page:               currentPage,
		Limit:              limit,
		Offset:             offset,
		UserId:             userId,
		ExtraFields:        extraFields,
	}

	archives, total := currentSite.ApiGetArchives(&req)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  archives,
	})
}

func ApiArchiveParams(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := ctx.URLParamInt64Default("id", 0)
	render := currentSite.Content.Editor == "markdown"
	if ctx.URLParamExists("render") {
		render, _ = ctx.URLParamBool("render")
	}
	filename := ctx.URLParam("filename")
	urlToken := ctx.URLParam("url_token")
	if filename != "" {
		urlToken = filename
	}
	userId := ctx.Values().GetUintDefault("userId", 0)
	userGroup, _ := ctx.Values().Get("userGroup").(*model.UserGroup)
	userInfo, _ := ctx.Values().Get("userInfo").(*model.User)
	sorted := true
	sortedTmp, err := ctx.URLParamBool("sorted")
	if err == nil {
		sorted = sortedTmp
	}

	req := request.ApiArchiveRequest{
		Id:        archiveId,
		UrlToken:  urlToken,
		Render:    render,
		UserId:    userId,
		UserGroup: userGroup,
		UserInfo:  userInfo,
	}

	params, err := currentSite.ApiGetArchiveParams(&req)
	if sorted == false {
		var extras = make(map[string]model.CustomField, len(params))
		for _, v := range params {
			extras[v.FieldName] = v
		}

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": extras,
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": params,
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
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if ctx.URLParamExists("render") {
		render, _ = ctx.URLParamBool("render")
	}

	req := &request.ApiCategoryRequest{
		Id:       int64(id),
		UrlToken: filename,
		Render:   render,
	}

	category, err := currentSite.ApiGetCategory(req)
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
	all := ctx.URLParamBoolDefault("all", false)
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
		if limit > currentSite.Content.MaxLimit {
			limit = currentSite.Content.MaxLimit
		}
		if limit < 1 {
			limit = 1
		}
	}

	req := &request.ApiCategoryListRequest{
		ModuleId: int64(moduleId),
		ParentId: int64(parentId),
		All:      all,
		Limit:    limit,
		Offset:   offset,
	}

	categories, _ := currentSite.ApiGetCategories(req)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": categories,
	})
}

func ApiModuleDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))
	filename := ctx.URLParam("filename")

	module := currentSite.GetModuleFromCache(id)
	if module == nil {
		if filename != "" {
			module = currentSite.GetModuleFromCacheByToken(filename)
		}
	}
	if module == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.TplTr("ModelDoesNotExist"),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": module,
	})
}

func ApiModuleList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	moduleList := currentSite.GetCacheModules()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": moduleList,
	})
}

func ApiCommentList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := ctx.URLParamInt64Default("id", 0)
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
		if limit > currentSite.Content.MaxLimit {
			limit = currentSite.Content.MaxLimit
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
	setting := currentSite.ApiGetContactSetting()
	reflectFields := structs.Fields(setting)

	for _, v := range reflectFields {
		if v.Name() != "ExtraFields" {
			settings[v.Name()] = v.Value()
		}
	}

	if setting.ExtraFields != nil {
		for i := range setting.ExtraFields {
			settings[setting.ExtraFields[i].Name] = setting.ExtraFields[i].Value
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
	setting := currentSite.ApiGetSystemSetting()
	reflectFields := structs.Fields(setting)

	for _, v := range reflectFields {
		if v.Name() != "ExtraFields" {
			settings[v.Name()] = v.Value()
		}
	}

	if setting.ExtraFields != nil {
		for i := range setting.ExtraFields {
			settings[setting.ExtraFields[i].Name] = setting.ExtraFields[i].Value
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": settings,
	})
}

func ApiDiyField(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	render := currentSite.Content.Editor == "markdown"
	if ctx.URLParamExists("render") {
		render, _ = ctx.URLParamBool("render")
	}
	var settings = map[string]interface{}{}
	fields := currentSite.ApiGetDiyFields(render)
	for i := range fields {
		settings[fields[i].Name] = fields[i].Value
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": settings,
	})
}

func ApiGuestbook(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	fields := currentSite.ApiGetGuestbookFields()

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
	showType := ctx.URLParamDefault("showType", "children")
	navList := currentSite.GetNavsFromCache(uint(typeId), showType)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": navList,
	})
}

func ApiNextArchive(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := ctx.URLParamInt64Default("id", 0)
	archiveDetail, err := currentSite.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	nextArchive, _ := currentSite.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`category_id` = ?", archiveDetail.CategoryId).Where("`id` > ?", archiveDetail.Id).Order("`id` ASC")
	})
	if nextArchive != nil && len(nextArchive.Password) > 0 {
		// password is not visible for user
		nextArchive.Password = ""
		nextArchive.HasPassword = true
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": nextArchive,
	})
}

func ApiPrevArchive(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := ctx.URLParamInt64Default("id", 0)
	archiveDetail, err := currentSite.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	prevArchive, _ := currentSite.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("`category_id` = ?", archiveDetail.CategoryId).Where("`id` < ?", archiveDetail.Id).Order("`id` DESC")
	})
	if prevArchive != nil && len(prevArchive.Password) > 0 {
		// password is not visible for user
		prevArchive.Password = ""
		prevArchive.HasPassword = true
	}
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
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if ctx.URLParamExists("render") {
		render, _ = ctx.URLParamBool("render")
	}
	category := currentSite.GetCategoryFromCache(id)
	if category == nil {
		if filename != "" {
			category = currentSite.GetCategoryFromCacheByToken(filename)
		}
	}
	if category == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "not found",
		})
		return
	}
	category.Thumb = category.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
	// convert markdown to html
	if render {
		category.Content = library.MarkdownToHTML(category.Content, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": category,
	})
}

func ApiPageList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	pageList := currentSite.GetCategoriesFromCache(0, 0, config.CategoryTypePage, true)
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
		if limit > currentSite.Content.MaxLimit {
			limit = currentSite.Content.MaxLimit
		}
		if limit < 1 {
			limit = 1
		}
	}

	var resultList []*model.Category

	for i := range pageList {
		if offset > i {
			continue
		}
		if limit > 0 && i >= (limit+offset) {
			break
		}
		pageList[i].Link = currentSite.GetUrl("page", pageList[i], 0)
		pageList[i].Thumb = pageList[i].GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)

		resultList = append(resultList, pageList[i])
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
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if ctx.URLParamExists("render") {
		render, _ = ctx.URLParamBool("render")
	}
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
		tagDetail.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		tagContent, err := currentSite.GetTagContentById(tagDetail.Id)
		if err == nil {
			tagDetail.Content = tagContent.Content
			// convert markdown to html
			if render {
				tagDetail.Content = library.MarkdownToHTML(tagDetail.Content, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
			}
			tagDetail.Extra = tagContent.Extra
			if tagDetail.Extra != nil {
				fields := currentSite.GetTagFields()
				if len(fields) > 0 {
					for _, field := range fields {
						if (tagDetail.Extra[field.FieldName] == nil || tagDetail.Extra[field.FieldName] == "" || tagDetail.Extra[field.FieldName] == 0) &&
							field.Type != config.CustomFieldTypeRadio &&
							field.Type != config.CustomFieldTypeCheckbox &&
							field.Type != config.CustomFieldTypeSelect {
							// default
							tagDetail.Extra[field.FieldName] = field.Content
						}
						if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
							tagDetail.Extra[field.FieldName] != nil {
							value, ok2 := tagDetail.Extra[field.FieldName].(string)
							if ok2 {
								if field.Type == config.CustomFieldTypeEditor && render {
									value = library.MarkdownToHTML(value, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
								}
								tagDetail.Extra[field.FieldName] = currentSite.ReplaceContentUrl(value, true)
							}
						}
						if field.Type == config.CustomFieldTypeImages && tagDetail.Extra[field.FieldName] != nil {
							if val, ok := tagDetail.Extra[field.FieldName].([]interface{}); ok {
								for j, v2 := range val {
									v2s, _ := v2.(string)
									val[j] = currentSite.ReplaceContentUrl(v2s, true)
								}
								tagDetail.Extra[field.FieldName] = val
							}
						} else if field.Type == config.CustomFieldTypeTexts && tagDetail.Extra[field.FieldName] != nil {
							var texts []model.CustomFieldTexts
							_ = json.Unmarshal([]byte(fmt.Sprint(tagDetail.Extra[field.FieldName])), &texts)
							tagDetail.Extra[field.FieldName] = texts
						} else if field.Type == config.CustomFieldTypeArchive && tagDetail.Extra[field.FieldName] != nil {
							// 列表
							var arcIds []int64
							buf, _ := json.Marshal(tagDetail.Extra[field.FieldName])
							_ = json.Unmarshal(buf, &arcIds)
							if len(arcIds) == 0 && field.Content != "" {
								value, _ := strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
								if value > 0 {
									arcIds = append(arcIds, value)
								}
							}
							if len(arcIds) > 0 {
								archives, _, _ := currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
									return tx.Where("archives.`id` IN (?)", arcIds)
								}, "archives.id ASC", 0, len(arcIds))
								tagDetail.Extra[field.FieldName] = archives
							} else {
								tagDetail.Extra[field.FieldName] = nil
							}
						} else if field.Type == config.CustomFieldTypeCategory {
							value, err := strconv.ParseInt(fmt.Sprint(tagDetail.Extra[field.FieldName]), 10, 64)
							if err != nil && field.Content != "" {
								value, _ = strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
							}
							if value > 0 {
								tagDetail.Extra[field.FieldName] = currentSite.GetCategoryFromCache(uint(value))
							} else {
								tagDetail.Extra[field.FieldName] = nil
							}
						}
					}
				}
			}
		}
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
	order := ctx.URLParamDefault("order", "")
	if order == "" {
		if currentSite.Content.UseSort == 1 {
			order = "archives.`sort` desc, archives.`created_time` desc"
		} else {
			order = "archives.`created_time` desc"
		}
	}
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
		if limit > currentSite.Content.MaxLimit {
			limit = currentSite.Content.MaxLimit
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
		tx = tx.Table("`archives` as archives").
			Joins("INNER JOIN `tag_data` as t ON archives.id = t.item_id AND t.`tag_id` = ?", tagDetail.Id)
		return tx
	}, order, currentPage, limit, offset)
	var archiveIds = make([]int64, 0, len(archives))
	for i := range archives {
		archiveIds = append(archiveIds, archives[i].Id)
		if len(archives[i].Password) > 0 {
			archives[i].Password = ""
			archives[i].HasPassword = true
		}
	}
	// 读取flags
	if len(archiveIds) > 0 {
		var flags []*model.ArchiveFlags
		currentSite.DB.Model(&model.ArchiveFlag{}).Where("`archive_id` IN (?)", archiveIds).Select("archive_id", "GROUP_CONCAT(`flag`) as flags").Group("archive_id").Scan(&flags)
		for i := range archives {
			for _, f := range flags {
				if f.ArchiveId == archives[i].Id {
					archives[i].Flag = f.Flags
					break
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

func ApiTagList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	limit := 10
	offset := 0
	currentPage := ctx.URLParamIntDefault("page", 1)
	itemId := ctx.URLParamInt64Default("itemId", 0)
	listType := ctx.URLParamDefault("type", "list")
	letter := ctx.URLParam("letter")
	order := ctx.URLParamDefault("order", "id desc")

	limitTmp := ctx.URLParam("limit")
	if limitTmp != "" {
		limitArgs := strings.Split(limitTmp, ",")
		if len(limitArgs) == 2 {
			offset, _ = strconv.Atoi(limitArgs[0])
			limit, _ = strconv.Atoi(limitArgs[1])
		} else if len(limitArgs) == 1 {
			limit, _ = strconv.Atoi(limitArgs[0])
		}
		if limit > currentSite.Content.MaxLimit {
			limit = currentSite.Content.MaxLimit
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
				}
			}
		}
	}
	tagList, total, _ := currentSite.GetTagList(itemId, "", categoryIds, letter, currentPage, limit, offset, order)
	for i := range tagList {
		tagList[i].Link = currentSite.GetUrl("tag", tagList[i], 0)
		tagList[i].GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
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
	bannerType := ctx.URLParamDefault("type", "default")

	banners, _ := currentSite.ApiGetBanners(bannerType)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": banners,
	})
}

func ApiIndexTdk(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	setting := currentSite.ApiGetIndexSetting()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func ApiLanguages(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	languages, _ := currentSite.ApiGetLanguages()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": languages,
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
	var req2 = map[string]string{
		"content":    req.Content,
		"captcha_id": req.CaptchaId,
		"captcha":    req.Captcha,
	}
	if ok := SafeVerify(ctx, req2, "json", "comment"); !ok {
		return
	}

	userId := ctx.Values().GetIntDefault("userId", 0)
	userInfo := ctx.Values().Get("userInfo")
	if userInfo != nil {
		user, ok := userInfo.(*model.User)
		if ok {
			req.UserName = user.UserName
		}
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
	// 是否需要审核
	var contentVerify = true
	userGroup := ctx.Values().Get("userGroup")
	if userGroup != nil {
		group, ok := userGroup.(*model.UserGroup)
		if ok && group != nil && group.Setting.ContentNoVerify {
			contentVerify = !group.Setting.ContentNoVerify
		}
	}
	req.Status = 0
	if contentVerify == false {
		// 不需要审核
		req.Status = 1
	}

	comment, err := currentSite.SaveComment(&req)
	if err != nil {
		msg := currentSite.TplTr("SaveFailed")
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  msg,
		})
	}
	// akismet 验证
	go func() {
		spamStatus, isChecked := currentSite.AkismentCheck(ctx, provider.CheckTypeComment, comment)
		if isChecked {
			currentSite.DB.Model(comment).UpdateColumn("status", spamStatus)
		}
	}()

	msg := currentSite.TplTr("PublishSuccessfully")
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

	userId := ctx.Values().GetIntDefault("userId", 0)
	comment, err := currentSite.GetCommentById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 检查是否点赞过
	_, err = currentSite.AddCommentPraise(uint(userId), int64(comment.Id), comment.ArchiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment.VoteCount += 1
	comment.Active = true

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  currentSite.TplTr("LikeSuccessfully"),
		"data": comment,
	})
}

func ApiGuestbookForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)
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
		} else if item.Type == config.CustomFieldTypeImage || item.Type == config.CustomFieldTypeFile {
			tmpVal, ok := req[item.FieldName].(string)
			if ok {
				// 如果有上传文件，则需要用户登录
				if userId == 0 {
					msg := currentSite.TplTr("ThisOperationRequiresLogin")
					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  msg,
					})
					return
				}
				tmpfile, err := os.CreateTemp("", "upload")
				if err != nil {
					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  "File Not Found",
					})
					return
				}
				if _, err := tmpfile.Write([]byte(tmpVal)); err != nil {
					_ = tmpfile.Close()
					_ = os.Remove(tmpfile.Name())

					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  "File Not Found",
					})
				}
				tmpfile.Seek(0, 0)
				fileHeader := &multipart.FileHeader{
					Filename: filepath.Base(item.FieldName),
					Header:   nil,
					Size:     int64(len(tmpVal)),
				}
				attach, err := currentSite.AttachmentUpload(tmpfile, fileHeader, 0, 0, userId)
				if err == nil {
					val = attach.Logo
					if attach.Logo == "" {
						val = attach.FileLocation
					}
				}

				_ = tmpfile.Close()
				_ = os.Remove(tmpfile.Name())
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
	hookCtx := &provider.HookContext{
		Point: provider.BeforeGuestbookPost,
		Site:  currentSite,
		Data:  req,
	}
	if err = provider.TriggerHook(hookCtx); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if ok := SafeVerify(ctx, result, "json", "guestbook"); !ok {
		return
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
		msg := currentSite.TplTr("SaveFailed")
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  msg,
		})
		return
	}
	// akismet 验证
	go func() {
		spamStatus, isChecked := currentSite.AkismentCheck(ctx, provider.CheckTypeGuestbook, guestbook)
		if isChecked {
			currentSite.DB.Model(guestbook).UpdateColumn("status", spamStatus)
		}
		if spamStatus == 1 {
			// 1 是正常，可以发邮件
			currentSite.SendGuestbookToMail(guestbook)
			if currentSite.ParentId > 0 {
				mainSite := currentSite.GetMainWebsite()
				parentGuestbook := *guestbook
				parentGuestbook.Id = 0
				parentGuestbook.Status = spamStatus
				parentGuestbook.SiteId = currentSite.Id
				_ = mainSite.DB.Save(&parentGuestbook)
				mainSite.SendGuestbookToMail(&parentGuestbook)
			}
		}
	}()

	msg := currentSite.PluginGuestbook.ReturnMessage
	if msg == "" {
		msg = currentSite.TplTr("ThankYouForYourMessage!")
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
	// 是否需要审核
	req.Draft = currentSite.Safe.APIPublish != 1
	userGroup := ctx.Values().Get("userGroup")
	if userGroup != nil {
		group, ok := userGroup.(*model.UserGroup)
		if ok && group != nil && group.Setting.ContentNoVerify {
			req.Draft = !group.Setting.ContentNoVerify
		}
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
	msg := currentSite.TplTr("PublishSuccessfully")
	if req.Draft {
		msg += currentSite.TplTr("ItHasEnteredTheReview")
	}
	archive.Link = currentSite.GetUrl("archive", archive, 0)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  msg,
		"data": archive,
	})
}
