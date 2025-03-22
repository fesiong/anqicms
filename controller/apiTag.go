package controller

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider/fulltext"
	"math"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	id := ctx.URLParamInt64Default("id", 0)
	filename := ctx.URLParam("filename")
	userId := ctx.Values().GetUintDefault("userId", 0)
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if ctx.URLParamExists("render") {
		render = ctx.URLParamBoolDefault("render", render)
	}
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
	// 支持读取草稿，只有登录了才能读取草稿
	if err != nil && userId > 0 {
		archiveDraft, err2 := currentSite.GetArchiveDraftById(id)
		if err2 == nil {
			if archiveDraft.UserId != userId {
				err = errors.New("record not found")
			} else {
				archive = &archiveDraft.Archive
				err = nil
			}
		}
	}
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

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
			Content: currentSite.TplTr("ThisContentRequiresUserLevelOrAboveToRead", archive.ReadLevel),
		}
	} else {
		// 读取data
		archive.ArchiveData, _ = currentSite.GetArchiveDataById(archive.Id)
	}
	// 读取flag
	archive.Flag = currentSite.GetArchiveFlags(archive.Id)
	// 读取分类
	archive.Category = currentSite.GetCategoryFromCache(archive.CategoryId)
	if archive.Category != nil {
		archive.Category.Link = currentSite.GetUrl("category", archive.Category, 0)
	}
	// 读取 extraDate
	archive.Extra = currentSite.GetArchiveExtra(archive.ModuleId, archive.Id, true)
	for i := range archive.Extra {
		if archive.Extra[i].Value == nil || archive.Extra[i].Value == "" {
			archive.Extra[i].Value = archive.Extra[i].Default
		}
		if archive.Extra[i].FollowLevel && !archive.HasOrdered {
			delete(archive.Extra, i)
		} else if archive.Extra[i].Type == config.CustomFieldTypeEditor && render {
			if value, ok := archive.Extra[i].Value.(string); ok {
				archive.Extra[i].Value = library.MarkdownToHTML(value, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
			}
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
	if len(archive.Password) > 0 {
		// password is not visible for user
		password := ctx.URLParam("password")
		if password == archive.Password {
			archive.PasswordValid = true
		}
		archive.Password = ""
		archive.HasPassword = true
		// 带密码的文档，如果密码不正确，则不显示内容
		if archive.PasswordValid == false {
			archive.ArchiveData = nil
		}
	}
	if archive.ArchiveData != nil {
		// convert markdown to html
		if render {
			archive.ArchiveData.Content = library.MarkdownToHTML(archive.ArchiveData.Content, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
		}
		re, _ := regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
		archive.ArchiveData.Content = re.ReplaceAllStringFunc(archive.ArchiveData.Content, func(s string) string {
			match := re.FindStringSubmatch(s)
			if len(match) < 2 {
				return s
			}
			if !strings.HasPrefix(match[1], "http") {
				res := currentSite.System.BaseUrl + match[1]
				s = strings.Replace(s, match[1], res, 1)
			}
			return s
		})
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
			"msg":  currentSite.TplTr("ModelDoesNotExist"),
		})
		return
	}

	allText := currentSite.TplTr("All")

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
	archiveId := ctx.URLParamInt64Default("id", 0)
	parentId := ctx.URLParamInt64Default("parentId", 0)
	moduleId := uint(ctx.URLParamIntDefault("moduleId", 0))
	authorId := uint(ctx.URLParamIntDefault("authorId", 0))
	userId := ctx.Values().GetUintDefault("userId", 0)
	draft := ctx.URLParamBoolDefault("draft", false)
	draftInt := 0
	if draft {
		draftInt = 1
	}
	tmpUserId := ctx.URLParam("userId")
	if tmpUserId == "self" {
		// 获取自己的文章
		userId = ctx.Values().GetUintDefault("userId", 0)
	}
	if userId > 0 {
		authorId = userId
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
					moduleId = categoryDetail.ModuleId
				}
			}
		}
	}
	// 增加支持 excludeCategoryId
	var excludeCategoryIds []uint
	tmpExcludeCatId := ctx.URLParam("excludeCategoryId")
	if tmpExcludeCatId != "" {
		tmpIds := strings.Split(tmpExcludeCatId, ",")
		for _, v := range tmpIds {
			tmpId, _ := strconv.Atoi(v)
			if tmpId > 0 {
				excludeCategoryIds = append(excludeCategoryIds, uint(tmpId))
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
		if limit > currentSite.Content.MaxLimit {
			limit = currentSite.Content.MaxLimit
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

	extraFields := map[int64]map[string]*model.CustomField{}
	var fields []string
	fields = append(fields, "id")
	if module != nil && len(module.Fields) > 0 {
		for _, v := range module.Fields {
			fields = append(fields, v.FieldName)
		}
	}

	var tmpResult = make([]*model.Archive, 0, limit)
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
				if currentSite.Content.MultiCategory == 1 && (categoryId > 0 || len(excludeCategoryIds) > 0) {
					tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id")
				}
				if categoryId > 0 {
					if currentSite.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id = ?", categoryId)
					} else {
						tx = tx.Where("`category_id` = ?", categoryId)
					}
				} else if moduleId > 0 {
					tx = tx.Where("`module_id` = ?", moduleId)
				}
				if len(excludeCategoryIds) > 0 {
					if currentSite.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id NOT IN (?)", excludeCategoryIds)
					} else {
						tx = tx.Where("`category_id` NOT IN (?)", excludeCategoryIds)
					}
				}
				tx = tx.Where("`keywords` like ? AND archives.`id` != ?", "%"+keywords+"%", archiveId)
				return tx
			}, "archives.id ASC", 0, limit, offset)
		} else if like == "relation" {
			archives = currentSite.GetArchiveRelations(archiveId)
		} else {
			archives = currentSite.GetArchiveRelations(archiveId)
			if len(archives) == 0 {
				halfLimit := int(math.Ceil(float64(limit) / 2))
				archives1, _, _ := currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
					if currentSite.Content.MultiCategory == 1 {
						// 多分类支持
						tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id and archive_categories.category_id = ?", categoryId)
					} else {
						tx = tx.Where("`category_id` = ?", categoryId)
					}
					if len(excludeCategoryIds) > 0 {
						if currentSite.Content.MultiCategory == 1 {
							tx = tx.Where("archive_categories.category_id NOT IN (?)", excludeCategoryIds)
						} else {
							tx = tx.Where("`category_id` NOT IN (?)", excludeCategoryIds)
						}
					}
					tx = tx.Where("archives.`id` > ?", archiveId)
					return tx
				}, "archives.id ASC", 0, limit, offset)
				archives2, _, _ := currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
					if currentSite.Content.MultiCategory == 1 {
						// 多分类支持
						tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id and archive_categories.category_id = ?", categoryId)
					} else {
						tx = tx.Where("`category_id` = ?", categoryId)
					}
					if len(excludeCategoryIds) > 0 {
						if currentSite.Content.MultiCategory == 1 {
							tx = tx.Where("archive_categories.category_id NOT IN (?)", excludeCategoryIds)
						} else {
							tx = tx.Where("`category_id` NOT IN (?)", excludeCategoryIds)
						}
					}
					tx = tx.Where("archives.`id` < ?", archiveId)
					return tx
				}, "archives.id DESC", 0, limit, offset)
				if len(archives1)+len(archives2) > limit {
					if len(archives1) > halfLimit && len(archives2) > halfLimit {
						archives1 = archives1[:halfLimit]
						archives2 = archives2[:halfLimit]
					} else if len(archives1) > len(archives2) {
						archives1 = archives1[:limit-len(archives2)]
					} else if len(archives2) > len(archives1) {
						archives2 = archives2[:limit-len(archives1)]
					}
				}
				archives = append(archives2, archives1...)
				// 如果数量超过，则截取
				if len(archives) > limit {
					archives = archives[:limit]
				}
			}
		}
	} else {
		var fulltextSearch bool
		var fulltextTotal int64
		var err2 error
		var ids []int64
		var searchCatIds []uint
		var searchTagIds []uint
		if listType == "page" && len(q) > 0 {
			var tmpDocs []fulltext.TinyArchive
			tmpDocs, fulltextTotal, err2 = currentSite.Search(q, moduleId, currentPage, limit)
			if err2 == nil {
				fulltextSearch = true
				for _, doc := range tmpDocs {
					if doc.Type == fulltext.ArchiveType {
						ids = append(ids, doc.Id)
					} else if doc.Type == fulltext.CategoryType {
						searchCatIds = append(searchCatIds, uint(doc.Id))
					} else if doc.Type == fulltext.TagType {
						searchTagIds = append(searchTagIds, uint(doc.Id))
					} else {
						// 其他值
					}
				}
				if len(tmpDocs) == 0 || len(ids) == 0 {
					ids = append(ids, 0)
				}
				offset = 0
			}
		}
		if len(searchCatIds) > 0 {
			cats := currentSite.GetCacheCategoriesByIds(searchCatIds)
			for _, cat := range cats {
				cat.Link = currentSite.GetUrl("category", cat, 0)
				tmpResult = append(tmpResult, &model.Archive{
					Type:        "category",
					Id:          int64(cat.Id),
					CreatedTime: cat.CreatedTime,
					UpdatedTime: cat.UpdatedTime,
					Title:       cat.Title,
					SeoTitle:    cat.SeoTitle,
					UrlToken:    cat.UrlToken,
					Keywords:    cat.Keywords,
					Description: cat.Description,
					ModuleId:    cat.ModuleId,
					CategoryId:  cat.ParentId,
					Images:      cat.Images,
					Logo:        cat.Logo,
					Link:        cat.Link,
					Thumb:       cat.Thumb,
					Sort:        cat.Sort,
				})
			}
		}
		if len(searchTagIds) > 0 {
			tags := currentSite.GetTagsByIds(searchTagIds)
			for _, tag := range tags {
				tag.Link = currentSite.GetUrl("tag", tag, 0)
				tag.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
				tmpResult = append(tmpResult, &model.Archive{
					Type:        "tag",
					Id:          int64(tag.Id),
					CreatedTime: tag.CreatedTime,
					UpdatedTime: tag.UpdatedTime,
					Title:       tag.Title,
					SeoTitle:    tag.SeoTitle,
					UrlToken:    tag.UrlToken,
					Keywords:    tag.Keywords,
					Description: tag.Description,
					Link:        tag.Link,
					Logo:        tag.Logo,
					Thumb:       tag.Thumb,
				})
			}
		}
		ops := func(tx *gorm.DB) *gorm.DB {
			if authorId > 0 {
				tx = tx.Where("user_id = ?", authorId)
			}
			if parentId > 0 {
				tx = tx.Where("parent_id = ?", parentId)
			}
			if flag != "" {
				tx = tx.Joins("INNER JOIN archive_flags ON archives.id = archive_flags.archive_id and archive_flags.flag = ?", flag)
			}
			if len(fields) > 1 {
				for _, v := range fields {
					// 如果有筛选条件，从这里开始筛选
					if param, ok := extraParams[v]; ok {
						tx = tx.Where("`"+v+"` = ?", param)
					}
				}
			}
			if currentSite.Content.MultiCategory == 1 && (len(categoryIds) > 0 || len(excludeCategoryIds) > 0) {
				tx = tx.Joins("INNER JOIN archive_categories ON archives.id = archive_categories.archive_id")
			}
			if len(categoryIds) > 0 {
				if child {
					var subIds []uint
					for _, v := range categoryIds {
						tmpIds := currentSite.GetSubCategoryIds(v, nil)
						subIds = append(subIds, tmpIds...)
						subIds = append(subIds, v)
					}
					if currentSite.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id IN (?)", subIds)
					} else {
						if len(subIds) == 1 {
							tx = tx.Where("`category_id` = ?", subIds[0])
						} else {
							tx = tx.Where("`category_id` IN(?)", subIds)
						}
					}
				} else if len(categoryIds) == 1 {
					if currentSite.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id = ?", categoryIds[0])
					} else {
						tx = tx.Where("`category_id` = ?", categoryIds[0])
					}
				} else {
					if currentSite.Content.MultiCategory == 1 {
						tx = tx.Where("archive_categories.category_id IN (?)", categoryIds)
					} else {
						tx = tx.Where("`category_id` IN(?)", categoryIds)
					}
				}
			} else if moduleId > 0 {
				tx = tx.Where("`module_id` = ?", moduleId)
			}
			if len(excludeCategoryIds) > 0 {
				if currentSite.Content.MultiCategory == 1 {
					tx = tx.Where("archive_categories.category_id NOT IN (?)", excludeCategoryIds)
				} else {
					tx = tx.Where("`category_id` NOT IN (?)", excludeCategoryIds)
				}
			}
			if len(ids) > 0 {
				tx = tx.Where("archives.`id` IN(?)", ids)
			} else if q != "" {
				tx = tx.Where("`title` like ?", "%"+q+"%")
			}
			return tx
		}
		if listType != "page" {
			// 如果不是分页，则不查询count
			currentPage = 0
		}
		if order != "" {
			if !strings.Contains(order, "rand") {
				order = "archives." + order
			}
		} else {
			if currentSite.Content.UseSort == 1 {
				order = "archives.`sort` desc, archives.`created_time` desc"
			} else {
				order = "archives.`created_time` desc"
			}
		}
		archives, total, _ = currentSite.GetArchiveList(ops, order, currentPage, limit, offset, draftInt)
		if fulltextSearch {
			total = fulltextTotal
		}
	}
	var archiveIds = make([]int64, 0, len(archives))
	for i := range archives {
		archiveIds = append(archiveIds, archives[i].Id)
		if len(archives[i].Password) > 0 {
			archives[i].Password = ""
			archives[i].HasPassword = true
		}
	}

	if module != nil && len(fields) > 1 && len(archiveIds) > 0 {
		var results []map[string]interface{}
		currentSite.DB.WithContext(currentSite.Ctx()).Table(module.TableName).Where("`id` IN(?)", archiveIds).Select("`" + strings.Join(fields, "`,`") + "`").Scan(&results)
		for _, field := range results {
			item := map[string]*model.CustomField{}
			for _, v := range module.Fields {
				item[v.FieldName] = &model.CustomField{
					Name:  v.Name,
					Value: field[v.FieldName],
				}
			}
			if id, ok := field["id"].(int64); ok {
				extraFields[id] = item
			} else if id2, ok2 := field["id"]; ok2 {
				tmpId, _ := strconv.ParseInt(fmt.Sprintf("%d", id2), 10, 64)
				extraFields[tmpId] = item
			}
		}
		for i := range archives {
			if extraFields[archives[i].Id] != nil {
				archives[i].Extra = extraFields[archives[i].Id]
			}
		}
	}
	// 读取flags
	if len(archiveIds) > 0 {
		var flags []*model.ArchiveFlags
		currentSite.DB.WithContext(currentSite.Ctx()).Model(&model.ArchiveFlag{}).Where("`archive_id` IN (?)", archiveIds).Select("archive_id", "GROUP_CONCAT(`flag`) as flags").Group("archive_id").Scan(&flags)
		for i := range archives {
			for _, f := range flags {
				if f.ArchiveId == archives[i].Id {
					archives[i].Flag = f.Flags
					break
				}
			}
		}
	}

	tmpResult = append(archives, tmpResult...)
	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  tmpResult,
	})
}

func ApiArchiveParams(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	archiveId := ctx.URLParamInt64Default("id", 0)
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
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if ctx.URLParamExists("render") {
		render = ctx.URLParamBoolDefault("render", render)
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
	category.Content = currentSite.ReplaceContentUrl(category.Content, true)
	// extra replace
	if category.Extra != nil {
		module := currentSite.GetModuleFromCache(category.ModuleId)
		if module != nil && len(module.CategoryFields) > 0 {
			for _, field := range module.CategoryFields {
				if category.Extra[field.FieldName] == nil || category.Extra[field.FieldName] == "" {
					// default
					category.Extra[field.FieldName] = field.Content
				}
				if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
					category.Extra[field.FieldName] != nil {
					value, ok2 := category.Extra[field.FieldName].(string)
					if ok2 {
						if field.Type == config.CustomFieldTypeEditor && render {
							value = library.MarkdownToHTML(value, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
						}
						category.Extra[field.FieldName] = currentSite.ReplaceContentUrl(value, true)
					}
				}
				if field.Type == config.CustomFieldTypeImages && category.Extra[field.FieldName] != nil {
					if val, ok := category.Extra[field.FieldName].([]interface{}); ok {
						for j, v2 := range val {
							v2s, _ := v2.(string)
							val[j] = currentSite.ReplaceContentUrl(v2s, true)
						}
						category.Extra[field.FieldName] = val
					}
				}
			}
		}
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

	categoryList := currentSite.GetCategoriesFromCache(moduleId, parentId, config.CategoryTypeArchive, all)
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

func ApiDiyField(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	render := currentSite.Content.Editor == "markdown"
	if ctx.URLParamExists("render") {
		render = ctx.URLParamBoolDefault("render", render)
	}
	var settings = map[string]interface{}{}

	fields := currentSite.GetDiyFieldSetting()
	for i := range fields {
		settings[fields[i].Name] = fields[i].Value
		if fields[i].Type == config.CustomFieldTypeEditor && render {
			settings[fields[i].Name] = library.MarkdownToHTML(fmt.Sprintf("%v", fields[i].Value), currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
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
		render = ctx.URLParamBoolDefault("render", render)
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
	for i := range pageList {
		pageList[i].Link = currentSite.GetUrl("page", pageList[i], 0)
		pageList[i].Thumb = pageList[i].GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
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
		tagDetail.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		tagContent, err := currentSite.GetTagContentById(tagDetail.Id)
		if err == nil {
			tagDetail.Content = tagContent.Content
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
	var bannerList = make([]*config.BannerItem, 0, 10)
	for _, tmpList := range currentSite.Banner.Banners {
		if tmpList.Type == bannerType {
			for _, banner := range tmpList.List {
				if !strings.HasPrefix(banner.Logo, "http") && !strings.HasPrefix(banner.Logo, "//") {
					banner.Logo = currentSite.PluginStorage.StorageUrl + "/" + strings.TrimPrefix(banner.Logo, "/")
				}
				bannerList = append(bannerList, &banner)
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": bannerList,
	})
}

func ApiIndexTdk(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var settings = map[string]interface{}{}

	reflectFields := structs.Fields(currentSite.Index)

	for _, v := range reflectFields {
		value := v.Value()
		settings[v.Name()] = value
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": settings,
	})
}

func ApiLanguages(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	// 获取当前的链接
	mainId := currentSite.ParentId
	if mainId == 0 {
		mainId = currentSite.Id
	}

	mainSite := provider.GetWebsite(mainId)
	if mainSite.MultiLanguage.Open == false {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": nil,
		})
	}

	languageSites := currentSite.GetMultiLangSites(mainId, false)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": languageSites,
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

	//发送邮件
	subject := currentSite.TplTr("HasNewMessageFromWhere", currentSite.System.SiteName, guestbook.UserName)
	var contents []string
	for _, item := range fields {
		content := currentSite.TplTr("s:s", item.Name, req[item.FieldName]) + "\n"

		contents = append(contents, content)
	}
	// 增加来路和IP返回
	contents = append(contents, currentSite.TplTr("SubmitIpLog", guestbook.Ip)+"\n")
	contents = append(contents, currentSite.TplTr("SourcePageLog", guestbook.Refer)+"\n")
	contents = append(contents, currentSite.TplTr("SubmitTimeLog", time.Now().Format("2006-01-02 15:04:05"))+"\n")

	if currentSite.SendTypeValid(provider.SendTypeGuestbook) {
		// 后台发信
		go currentSite.SendMail(subject, strings.Join(contents, ""))
		// 回复客户
		recipient, ok := result["email"]
		if !ok {
			recipient = result["contact"]
		}
		go currentSite.ReplyMail(recipient)
	}

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
