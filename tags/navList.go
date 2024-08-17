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
		for i := range navList {
			navList[i].IsCurrent = false
			if (navList[i].NavType == model.NavTypeSystem && (webInfo.PageName == "index" || webInfo.PageName == "archiveIndex") && navList[i].PageId == webInfo.NavBar) || (navList[i].NavType == model.NavTypeCategory && (webInfo.PageName == "archiveDetail" || webInfo.PageName == "archiveList" || webInfo.PageName == "pageDetail") && navList[i].PageId == webInfo.NavBar) || (navList[i].NavType == model.NavTypeArchive && webInfo.PageName == "archiveDetail" && navList[i].PageId == webInfo.PageId) {
				navList[i].IsCurrent = true
			}
			if navList[i].NavList != nil {
				for j := range navList[i].NavList {
					navList[i].NavList[j].IsCurrent = false
					if (navList[i].NavList[j].NavType == model.NavTypeSystem && (webInfo.PageName == "index" || webInfo.PageName == "archiveIndex") && navList[i].NavList[j].PageId == webInfo.NavBar) || (navList[i].NavList[j].NavType == model.NavTypeCategory && (webInfo.PageName == "archiveDetail" || webInfo.PageName == "archiveList" || webInfo.PageName == "pageDetail") && navList[i].NavList[j].PageId == webInfo.NavBar) || (navList[i].NavType == model.NavTypeArchive && webInfo.PageName == "archiveDetail" && navList[i].PageId == webInfo.PageId) {
						navList[i].NavList[j].IsCurrent = true
						navList[i].IsCurrent = true
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
