package tags

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
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
	var categoryIds []int
	var defaultCategoryId uint
	var authorId = uint(0)
	var parentId = int64(0)
	var tagIds []int64
	var tag string
	var argIds []int64
	var categoryDetail *model.Category
	render := currentSite.Content.Editor == "markdown"
	if args["render"] != nil {
		render = args["render"].Bool()
	}
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
	if args["tagId"] != nil {
		tmpIds := strings.Split(args["tagId"].String(), ",")
		for _, v := range tmpIds {
			tmpId, _ := strconv.Atoi(v)
			if tmpId > 0 {
				tagIds = append(tagIds, int64(tmpId))
			}
		}
	}
	if args["tag"] != nil {
		tag = args["tag"].String()
	}
	if args["ids"] != nil {
		tmpIds := strings.Split(args["ids"].String(), ",")
		for _, v := range tmpIds {
			tmpId, _ := strconv.ParseInt(v, 10, 64)
			if tmpId > 0 {
				argIds = append(argIds, tmpId)
			}
		}
	}
	price := ""
	if args["price"] != nil {
		price = args["price"].String()
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
					categoryIds = append(categoryIds, int(categoryDetail.Id))
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
			categoryIds = append(categoryIds, int(defaultCategoryId))
		}
		if defaultModuleId > 0 {
			moduleId = defaultModuleId
		}
	}
	// 增加支持 excludeCategoryId
	var excludeCategoryIds []int
	if args["excludeCategoryId"] != nil {
		tmpIds := strings.Split(args["excludeCategoryId"].String(), ",")
		for _, v := range tmpIds {
			tmpId, _ := strconv.Atoi(v)
			if tmpId > 0 {
				excludeCategoryIds = append(excludeCategoryIds, tmpId)
			}
		}
	}
	var excludeFlags []string
	if args["excludeFlag"] != nil {
		excludeFlags = strings.Split(args["excludeFlag"].String(), ",")
	}
	var combineId = int64(0)
	var combineMode = "to"
	var combineArchive *model.Archive
	if args["combineId"] != nil {
		combineId = int64(args["combineId"].Integer())
	}
	if args["combineFromId"] != nil {
		combineMode = "from"
		combineId = int64(args["combineFromId"].Integer())
	}

	var order string
	if args["order"] != nil {
		order = args["order"].String()
		order = provider.ParseOrderBy(order, "archives")
	}
	if order == "" {
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
	showContent := false
	showExtra := false
	showCategory := false

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
	if args["showContent"] != nil {
		showContent = args["showContent"].Bool()
	}
	if args["showExtra"] != nil {
		showExtra = args["showExtra"].Bool()
	}
	if args["showCategory"] != nil {
		showCategory = args["showCategory"].Bool()
	}

	// 支持更多的参数搜索，
	extraParams := map[string]interface{}{}
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
	if price != "" {
		extraParams["price"] = price
	}
	// 支持标签参数搜索
	module = currentSite.GetModuleFromCache(moduleId)
	if module != nil {
		if len(module.Fields) > 0 {
			// 所有参数的url都附着到query中
			for _, v := range module.Fields {
				if v.FieldName != "type" && args[v.FieldName] != nil {
					extraParams[v.FieldName] = args[v.FieldName].String()
				}
			}
		}
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
		argIds = nil
	} else {
		currentPage = 1
		// list模式则始终使用 argQ
		q = argQ
	}
	userId, _ := ctx.Public["userId"].(uint)
	//获取id
	archiveId := int64(0)
	var keywords string
	archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
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
			categoryIds = categoryIds[:0]
		}
	}

	req := request.ApiArchiveListRequest{
		Id:                 archiveId,
		Ids:                argIds,
		Render:             render,
		ParentId:           int64(parentId),
		CategoryIds:        categoryIds,
		ExcludeCategoryIds: excludeCategoryIds,
		ExcludeFlags:       excludeFlags,
		ModuleId:           int64(moduleId),
		AuthorId:           int64(authorId),
		ShowFlag:           showFlag,
		ShowContent:        showContent,
		ShowExtra:          showExtra,
		ShowCategory:       showCategory,
		Draft:              false,
		Child:              child,
		Order:              order,
		Tag:                tag,
		TagId:              0,
		TagIds:             tagIds,
		Flag:               flag,
		Q:                  q,
		Like:               like,
		Keywords:           keywords,
		Type:               listType,
		Page:               currentPage,
		Limit:              limit,
		Offset:             offset,
		UserId:             userId,
		ExtraFields:        extraParams,
		CombineId:          combineId,
		CombineMode:        combineMode,
	}

	archives, total := currentSite.ApiGetArchives(&req)

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
			} else if ok2 && webInfo.PageName == "search" {
				urlMatch := "search"
				moduleToken := ""
				if module != nil {
					moduleToken = module.UrlToken
				}
				urlPatten = currentSite.GetUrl(urlMatch, map[string]interface{}{
					"q":      url.QueryEscape(q),
					"module": moduleToken,
				}, -1)
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
				ctxOri.ViewData("listData", archives)
			}
		}
	}

	ctx.Private[node.name] = archives
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
