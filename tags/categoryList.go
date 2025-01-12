package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"strconv"
	"strings"
)

type tagCategoryListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagCategoryListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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
	all := false
	if args["all"] != nil {
		all = args["all"].Bool()
	}

	limit := 0
	offset := 0
	moduleId := uint(0)
	if args["moduleId"] != nil {
		moduleId = uint(args["moduleId"].Integer())
	}

	categoryDetail, _ := ctx.Public["category"].(*model.Category)
	parentId := uint(0)
	excludeId := uint(0)
	if args["parentId"] != nil {
		if args["parentId"].String() == "parent" {
			if categoryDetail != nil {
				parentId = categoryDetail.ParentId
				//	excludeId = categoryDetail.Id
			}
		} else {
			parentId = uint(args["parentId"].Integer())
		}
	} else if categoryDetail != nil {
		parentId = categoryDetail.Id
	}
	if parentId > 0 {
		// fix moduleId
		parentCategory := currentSite.GetCategoryFromCache(parentId)
		if parentCategory != nil {
			moduleId = parentCategory.ModuleId
		}
	}
	if moduleId == 0 {
		module, _ := ctx.Public["module"].(*model.Module)
		if module != nil {
			moduleId = module.Id
		}
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

	webInfo, webOk := ctx.Public["webInfo"].(*response.WebInfo)

	categoryList := currentSite.GetCategoriesFromCache(moduleId, parentId, config.CategoryTypeArchive, all)
	var resultList []*model.Category
	for i := 0; i < len(categoryList); i++ {
		if offset > i || categoryList[i].Id == excludeId {
			continue
		}
		if limit > 0 && i >= (limit+offset) {
			break
		}
		categoryList[i].Link = currentSite.GetUrl("category", categoryList[i], 0)
		categoryList[i].Thumb = categoryList[i].GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		categoryList[i].IsCurrent = false
		if webOk {
			if (webInfo.PageName == "archiveList" || webInfo.PageName == "archiveDetail") && int64(categoryList[i].Id) == webInfo.NavBar {
				categoryList[i].IsCurrent = true
			}
		}
		resultList = append(resultList, categoryList[i])
	}

	ctx.Private[node.name] = resultList

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagCategoryListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagCategoryListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("categoryList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed categoryList-tag arguments.", nil)
	}

	wrapper, endtagargs, err := doc.WrapUntilTag("endcategoryList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endcategoryList' must equal to 'categoryList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endcategoryList'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
