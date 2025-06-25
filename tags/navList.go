package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

type tagNavListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagNavListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	typeId := uint(1)
	if args["typeId"] != nil {
		typeId = uint(args["typeId"].Integer())
	}

	navList := currentSite.GetNavsFromCache(typeId)

	webInfo, ok := ctx.Public["webInfo"].(*response.WebInfo)
	if ok {
		for _, v := range navList {
			v.IsCurrent = false
			if (v.NavType == model.NavTypeSystem && (webInfo.PageName == "index" || webInfo.PageName == "archiveIndex") && v.PageId == webInfo.NavBar) || (v.NavType == model.NavTypeCategory && (webInfo.PageName == "archiveDetail" || webInfo.PageName == "archiveList" || webInfo.PageName == "pageDetail") && v.PageId == webInfo.NavBar) || (v.NavType == model.NavTypeArchive && webInfo.PageName == "archiveDetail" && v.PageId == webInfo.PageId) {
				v.IsCurrent = true
			}
			if v.NavList != nil {
				for _, vv := range v.NavList {
					vv.IsCurrent = false
					if (vv.NavType == model.NavTypeSystem && (webInfo.PageName == "index" || webInfo.PageName == "archiveIndex") && vv.PageId == webInfo.NavBar) || (vv.NavType == model.NavTypeCategory && (webInfo.PageName == "archiveDetail" || webInfo.PageName == "archiveList" || webInfo.PageName == "pageDetail") && vv.PageId == webInfo.NavBar) || (v.NavType == model.NavTypeArchive && webInfo.PageName == "archiveDetail" && v.PageId == webInfo.PageId) {
						vv.IsCurrent = true
						v.IsCurrent = true
					}
				}
			}
		}
	}

	ctx.Private[node.name] = navList

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagNavListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagNavListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("navList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed navList-tag arguments.", nil)
	}

	wrapper, endtagargs, err := doc.WrapUntilTag("endnavList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endnavList' must equal to 'navList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endnavList'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
