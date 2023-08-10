package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"github.com/kataras/iris/v12/context"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"math"
	"net/url"
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

	moduleId := uint(0)
	var categoryIds []uint
	var authorId = uint(0)

	if args["moduleId"] != nil {
		moduleId = uint(args["moduleId"].Integer())
	}
	if args["authorId"] != nil {
		authorId = uint(args["authorId"].Integer())
	}
	if args["userId"] != nil {
		authorId = uint(args["userId"].Integer())
	}

	categoryDetail, _ := ctx.Public["category"].(*model.Category)
	archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
	if ok {
		categoryDetail = currentSite.GetCategoryFromCache(archiveDetail.CategoryId)
	}
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
	} else if categoryDetail != nil {
		if len(categoryIds) == 0 {
			categoryIds = append(categoryIds, categoryDetail.Id)
		}
		moduleId = categoryDetail.ModuleId
	}

	module := currentSite.GetModuleFromCache(moduleId)
	if module == nil {
		module, _ = ctx.Public["module"].(*model.Module)
		if module != nil {
			moduleId = module.Id
		}
	}

	order := "id desc"
	limit := 10
	offset := 0
	currentPage := 1
	listType := "list"
	flag := ""
	q := ""
	argQ := ""
	child := true

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

	// 支持更多的参数搜索，
	extraParams := make(url.Values)
	urlParams, ok := ctx.Public["urlParams"].(map[string]string)
	if ok {
		for k, v := range urlParams {
			if k == "page" {
				continue
			}
			if listType == "page" {
				if v != "" {
					extraParams.Set(k, v)
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

	if args["order"] != nil {
		order = args["order"].String()
	}
	if args["limit"] != nil {
		limitArgs := strings.Split(args["limit"].String(), ",")
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
	} else {
		currentPage = 1
	}

	var archives []*model.Archive
	var total int64
	if listType == "related" {
		//获取id
		archiveId := uint(0)
		var keywords string
		archiveDetail, ok = ctx.Public["archive"].(*model.Archive)
		var categoryId = uint(0)
		if len(categoryIds) > 0 {
			categoryId = categoryIds[0]
		}
		if ok {
			archiveId = archiveDetail.Id
			categoryId = archiveDetail.CategoryId
			keywords = strings.Split(strings.ReplaceAll(archiveDetail.Keywords, "，", ","), ",")[0]
			category := currentSite.GetCategoryFromCache(categoryId)
			if category != nil {
				moduleId = category.ModuleId
			} else {
				categoryId = 0
			}
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
				if moduleId > 0 {
					tx = tx.Where("`module_id` = ?", moduleId)
				}
				if categoryId > 0 {
					tx = tx.Where("`category_id` = ?", categoryId)
				}
				tx = tx.Where("`status` = 1 AND `keywords` like ? AND `id` != ?", moduleId, categoryId, "%"+keywords+"%", archiveId).
					Order("id ASC")
				return tx
			}, 0, limit, offset)
		} else {
			newLimit := int(math.Ceil(float64(limit) / 2))
			archives, _, _ = currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				tx = tx.Where("`module_id` = ? AND `category_id` = ? AND `status` = 1 AND `id` > ?", moduleId, categoryId, archiveId).
					Order("id ASC")
				return tx
			}, 0, limit, offset)
			if limit-len(archives) < newLimit {
				newLimit = limit - len(archives)
			}
			archives2, _, _ := currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
				tx = tx.Where("`module_id` = ? AND `category_id` = ? AND `status` = 1 AND `id` < ?", moduleId, categoryId, archiveId).
					Order("id DESC")
				return tx
			}, 0, newLimit, offset)
			//列表不返回content
			if len(archives2) > 0 {
				archives = append(archives2, archives...)
			}
			// 如果数量超过，则截取
			if len(archives) > limit {
				archives = archives[:limit]
			}
		}
	} else {
		var fulltextSearch bool
		var fulltextTotal int64
		var err2 error
		var ids []uint
		if (listType == "page" && len(q) > 0) || argQ != "" {
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
				var fields [][2]string
				for _, v := range module.Fields {
					// 如果有筛选条件，从这里开始筛选
					if extraParams.Has(v.FieldName) {
						param := extraParams.Get(v.FieldName)
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
	}
	for i := range archives {
		if len(archives[i].Password) > 0 {
			archives[i].HasPassword = true
		}
	}

	if listType == "page" {
		var urlPatten string
		if categoryDetail != nil {
			urlMatch := "category"
			urlPatten = currentSite.GetUrl(urlMatch, categoryDetail, -1)
		} else {
			webInfo, ok := ctx.Public["webInfo"].(*response.WebInfo)
			if ok && webInfo.PageName == "archiveIndex" {
				urlMatch := "archiveIndex"
				urlPatten = currentSite.GetUrl(urlMatch, module, -1)
			} else {
				// 其他地方
				urlPatten = ""
			}
		}
		ctx.Public["pagination"] = makePagination(currentSite, total, currentPage, limit, urlPatten, 5)
	}
	ctx.Private[node.name] = archives

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagArchiveListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagArchiveListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("archiveList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
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
