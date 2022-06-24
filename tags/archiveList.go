package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/dao"
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
	if dao.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	moduleId := uint(0)
	var categoryIds []uint

	if args["moduleId"] != nil {
		moduleId = uint(args["moduleId"].Integer())
	}

	categoryDetail, _ := ctx.Public["category"].(*model.Category)
	archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
	if ok {
		categoryDetail = provider.GetCategoryFromCache(archiveDetail.CategoryId)
	}
	if args["categoryId"] != nil {
		tmpIds := strings.Split(args["categoryId"].String(), ",")
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
	} else if categoryDetail != nil {
		if len(categoryIds) == 0 {
			categoryIds = append(categoryIds, categoryDetail.Id)
		}
		moduleId = categoryDetail.ModuleId
	}

	module := provider.GetModuleFromCache(moduleId)
	if module == nil {
		module, _ = ctx.Public["module"].(*model.Module)
	}

	order := "id desc"
	limit := 10
	offset := 0
	currentPage := 1
	listType := "list"
	flag := ""
	q := ""
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
		q = args["q"].String()
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
		q = urlParams["q"]
	}
	requestParams, ok := ctx.Public["requestParams"].(*context.RequestParams)
	if ok {
		paramPage := requestParams.GetIntDefault("page", 0)
		if paramPage > 0 {
			currentPage = paramPage
		}
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

	var archives []*model.Archive
	var total int64
	if listType == "related" {
		//获取id
		archiveId := uint(0)
		archiveDetail, ok = ctx.Public["archive"].(*model.Archive)
		var categoryId = uint(0)
		if len(categoryIds) > 0 {
			categoryId = categoryIds[0]
		}
		if ok {
			archiveId = archiveDetail.Id
			categoryId = archiveDetail.CategoryId
			category := provider.GetCategoryFromCache(categoryId)
			if category != nil {
				moduleId = category.ModuleId
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
			return nil
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

	if listType == "page" {
		var urlPatten string
		if categoryDetail != nil {
			urlMatch := "category"
			urlPatten = provider.GetUrl(urlMatch, categoryDetail, -1)
		} else {
			webInfo, ok := ctx.Public["webInfo"].(response.WebInfo)
			if ok && webInfo.PageName == "archiveIndex" {
				urlMatch := "archiveIndex"
				urlPatten = provider.GetUrl(urlMatch, module, -1)
			} else {
				// 其他地方
				urlPatten = ""
			}
		}
		ctx.Public["pagination"] = makePagination(total, currentPage, limit, urlPatten, 5)
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
