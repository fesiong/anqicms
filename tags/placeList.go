package tags

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagPlaceListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagPlaceListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	listType := "list"
	if args["listType"] != nil {
		listType = args["listType"].String()
	}

	currentPage := 1
	urlParams, ok := ctx.Public["urlParams"].(map[string]string)
	if ok {
		currentPage, _ = strconv.Atoi(urlParams["page"])
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

	limit := 0
	offset := 0

	place, _ := ctx.Public["place"].(*model.Place)
	parentId := uint(0)
	excludeId := uint(0)
	if args["parentId"] != nil {
		if args["parentId"].String() == "parent" {
			if place != nil {
				parentId = place.ParentId
			}
		} else {
			parentId = uint(args["parentId"].Integer())
		}
	} else if place != nil {
		parentId = place.Id
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
	}

	placeList := currentSite.GetPlacesFromCache(parentId, all)
	var resultList = make([]model.Place, 0, len(placeList))
	for i := range placeList {
		if offset > i || placeList[i].Id == excludeId {
			continue
		}
		if limit > 0 && i >= (limit+offset) {
			break
		}
		tmpPlace := *placeList[i]
		tmpPlace.Link = currentSite.GetUrl(provider.PatternPlace, &tmpPlace, 0)
		tmpPlace.Thumb = tmpPlace.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.GetDefaultThumb(int(tmpPlace.Id)))
		tmpPlace.IsCurrent = false
		if place != nil && (tmpPlace.Id == place.Id || tmpPlace.Id == place.ParentId) {
			tmpPlace.IsCurrent = true
		}
		resultList = append(resultList, tmpPlace)
	}

	if listType == "page" {
		total := int64(len(placeList))
		// 分页
		urlPatten := currentSite.GetUrl("placeDetail", nil, -1)
		ctx.Public["pagination"] = makePagination(currentSite, total, currentPage, limit, urlPatten, 5)
	}

	ctx.Private[node.name] = resultList

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagPlaceListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagPlaceListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("placeList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed placeList-tag arguments.", nil)
	}

	wrapper, endtagargs, err := doc.WrapUntilTag("endplaceList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endplaceList' must equal to 'placeList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endplaceList'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
