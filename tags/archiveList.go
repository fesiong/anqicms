package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"github.com/kataras/iris/v12/context"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/provider/fulltext"
	"kandaoni.com/anqicms/response"
	"math"
	"strconv"
	"strings"
)

type tagArchiveListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagArchiveListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}

	// 如果手工指定了moduleId，并且当前module 不是指定的，则不自动获取分类
	moduleId := uint(0)
	defaultModuleId := uint(0)
	var categoryIds []uint
	var defaultCategoryId uint
	var authorId = uint(0)
	var parentId = int64(0)
	var categoryDetail *model.Category

	if args["moduleId"] != nil {
		moduleId = uint(args["moduleId"].Integer())
	}
	if args["authorId"] != nil {
		authorId = uint(args["authorId"].Integer())
	}
	if args["userId"] != nil {
		authorId = uint(args["userId"].Integer())
	}
	if args["parentId"] != nil {
		parentId = int64(args["parentId"].Integer())
	}
	module, _ := ctx.Public["module"].(*model.Module)
	if module != nil {
		defaultModuleId = module.Id
	}
	// 如果指定了分类ID
	if args["categoryId"] != nil {
		tmpIds := strings.Split(args["categoryId"].String(), ",")
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
	} else {
		// 否则尝试自动获取分类
		categoryDetail, _ = ctx.Public["category"].(*model.Category)
		archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
		if ok {
			categoryDetail = currentSite.GetCategoryFromCache(archiveDetail.CategoryId)
		}
		if categoryDetail != nil {
			defaultCategoryId = categoryDetail.Id
			defaultModuleId = categoryDetail.ModuleId
		}
	}
	if moduleId > 0 && defaultModuleId > 0 && moduleId != defaultModuleId {
		// 指定的模型与自动获取的模型不一致，则不自动获取分类
	} else {
		if len(categoryIds) == 0 && defaultCategoryId > 0 {
			categoryIds = append(categoryIds, defaultCategoryId)
		}
		if defaultModuleId > 0 {
			moduleId = defaultModuleId
		}
	}
	// 增加支持 excludeCategoryId
	var excludeCategoryIds []uint
	if args["excludeCategoryId"] != nil {
		tmpIds := strings.Split(args["excludeCategoryId"].String(), ",")
		for _, v := range tmpIds {
			tmpId, _ := strconv.Atoi(v)
			if tmpId > 0 {
				excludeCategoryIds = append(excludeCategoryIds, uint(tmpId))
			}
		}
	}
	var excludeFlags []string
	if args["excludeFlag"] != nil {
		excludeFlags = strings.Split(args["excludeFlag"].String(), ",")
	}
	var combineMode = "to"
	var combineArchive *model.Archive
	if args["combineId"] != nil {
		combineId := int64(args["combineId"].Integer())
		combineArchive, _ = currentSite.GetArchiveById(combineId)
	}
	if args["combineFromId"] != nil {
		combineMode = "from"
		combineId := int64(args["combineFromId"].Integer())
		combineArchive, _ = currentSite.GetArchiveById(combineId)
	}

	var order string
	if args["order"] != nil {
		order = args["order"].String()
		if !strings.Contains(order, "rand") {
			order = "archives." + order
		}
	} else {
		if currentSite.Content.UseSort == 1 || parentId > 0 {
			order = "archives.`sort` desc, archives.`created_time` desc"
		} else {
			order = "archives.`created_time` desc"
		}
	}

	limit := 10
	offset := 0
	currentPage := 1
	listType := "list"
	flag := ""
	q := ""
	argQ := ""
	child := true
	showFlag := false

	if args["type"] != nil {
		listType = args["type"].String()
	}

	if args["child"] != nil {
		child = args["child"].Bool()
	}

	if args["flag"] != nil {
		flag = args["flag"].String()
	}

	if args["q"] != nil {
		q = strings.TrimSpace(args["q"].String())
		argQ = q
	}
	if args["showFlag"] != nil {
		showFlag = args["showFlag"].Bool()
	}

	// 支持更多的参数搜索，
	extraParams := map[string]string{}
	urlParams, ok := ctx.Public["urlParams"].(map[string]string)
	if ok {
		if listType == "page" {
			for k, v := range urlParams {
				if k == "page" {
					continue
				}
				if v != "" {
					extraParams[k] = v
				}
			}
		}
		currentPage, _ = strconv.Atoi(urlParams["page"])
		q = strings.TrimSpace(urlParams["q"])
	}
	requestParams, ok := ctx.Public["requestParams"].(*context.RequestParams)
	if ok {
		paramPage := requestParams.GetIntDefault("page", 0)
		if paramPage > 0 {
			currentPage = paramPage
		}
	}
	if currentPage < 1 {
		currentPage = 1
	}

	if args["limit"] != nil {
		limitArgs := strings.Split(args["limit"].String(), ",")
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
	} else {
		currentPage = 1
		// list模式则始终使用 argQ
		q = argQ
	}

	var tmpResult = make([]*model.Archive, 0, limit)
	var archives []*model.Archive
	var total int64
	if listType == "related" {
		//获取id
		archiveId := int64(0)
		var keywords string
		archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
		var categoryId = uint(0)
		if len(categoryIds) > 0 {
			categoryId = categoryIds[0]
		}
		if ok {
			archiveId = archiveDetail.Id
			keywords = strings.Split(strings.ReplaceAll(archiveDetail.Keywords, "，", ","), ",")[0]
		}
		// 允许通过keywords调用
		like := ""
		if args["like"] != nil {
			like = args["like"].String()
		}
		if args["keywords"] != nil {
			keywords = strings.Split(args["keywords"].String(), ",")[0]
		}

		if like == "keywords" {
			if args["siteId"] != nil {
				moduleId = 0
				categoryId = 0
			}
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
		} else if like == "tag" {
			// 根据tag来调用相关
			var tagIds []uint
			currentSite.DB.WithContext(currentSite.Ctx()).Model(&model.TagData{}).Where("`item_id` = ?", archiveId).Pluck("tag_id", &tagIds)
			if len(tagIds) > 0 {
				archives, total, _ = currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
					tx = tx.Table("`archives` as archives").
						Joins("INNER JOIN `tag_data` as t ON archives.id = t.item_id AND t.`tag_id` IN (?) AND archives.`id` != ?", tagIds, archiveId)
					return tx
				}, order, 0, limit, offset)
			}
		} else {
			// 检查是否有相关文档
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
		if (listType == "page" && len(q) > 0) || argQ != "" {
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
			} else if len(excludeFlags) > 0 {
				tx = tx.Joins("LEFT JOIN archive_flags ON archives.id = archive_flags.archive_id and archive_flags.flag IN (?)", excludeFlags).Where("archive_flags.archive_id IS NULL")
			}
			if len(extraParams) > 0 {
				module = currentSite.GetModuleFromCache(moduleId)
				if module != nil && len(module.Fields) > 0 {
					var fields [][2]string
					for _, v := range module.Fields {
						// 如果有筛选条件，从这里开始筛选
						if param, ok := extraParams[v.FieldName]; ok {
							fields = append(fields, [2]string{"`" + module.TableName + "`.`" + v.FieldName + "` = ?", param})
						}
					}
					if len(fields) > 0 {
						tx = tx.InnerJoins(fmt.Sprintf("INNER JOIN `%s` on `%s`.id = `archives`.id", module.TableName, module.TableName))
						for _, field := range fields {
							tx = tx.Where(field[0], field[1])
						}
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
				// 使用了全文索引，拿到了ID
				tx = tx.Where("archives.`id` IN(?)", ids)
			} else if q != "" {
				// 如果文章数量达到10万，则只能匹配开头，否则就模糊搜索
				var allArchives int64
				allArchives = currentSite.GetExplainCount("SELECT id FROM archives")
				if allArchives > 100000 {
					tx = tx.Where("`title` like ?", q+"%")
				} else {
					tx = tx.Where("`title` like ?", "%"+q+"%")
				}
			}
			return tx
		}
		if listType != "page" {
			// 如果不是分页，则不查询count
			currentPage = 0
		}
		archives, total, _ = currentSite.GetArchiveList(ops, order, currentPage, limit, offset)
		if fulltextSearch {
			total = fulltextTotal
		}
	}
	var archiveIds = make([]int64, 0, len(archives))
	for i := range archives {
		archiveIds = append(archiveIds, archives[i].Id)
		if len(archives[i].Password) > 0 {
			archives[i].HasPassword = true
		}
		if combineArchive != nil {
			if combineMode == "from" {
				archives[i].Link = currentSite.GetUrl("archive", combineArchive, 0, archives[i])
			} else {
				archives[i].Link = currentSite.GetUrl("archive", archives[i], 0, combineArchive)
			}
		}
	}
	// 读取flags
	if showFlag && len(archiveIds) > 0 {
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
	if listType == "page" {
		var urlPatten string
		webInfo, ok2 := ctx.Public["webInfo"].(*response.WebInfo)
		if categoryDetail != nil {
			urlMatch := "category"
			urlPatten = currentSite.GetUrl(urlMatch, categoryDetail, -1)
		} else {
			if ok2 && webInfo.PageName == "archiveIndex" {
				urlMatch := "archiveIndex"
				urlPatten = currentSite.GetUrl(urlMatch, module, -1)
			} else {
				// 其他地方
				urlPatten = ""
			}
		}
		pager := makePagination(currentSite, total, currentPage, limit, urlPatten, 5)
		webInfo.TotalPages = pager.TotalPages
		ctx.Public["pagination"] = pager

		// 公开列表数据
		if currentSite.PluginJsonLd.Open {
			ctxOri := currentSite.CtxOri()
			if ctxOri != nil {
				ctxOri.ViewData("listData", tmpResult)
			}
		}
	}
	ctx.Private[node.name] = tmpResult
	ctx.Private["combine"] = combineArchive

	//execute
	_ = node.wrapper.Execute(ctx, writer)

	return nil
}

func TagArchiveListParser(doc *pongo2.Parser, _ *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagArchiveListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("archiveList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're going to parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed archiveList-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endarchiveList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endarchiveList' must equal to 'archiveList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endarchiveList'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
