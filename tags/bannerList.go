package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"strings"
)

type tagBannerListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagBannerListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	bannerType := "default"
	if args["type"] != nil {
		bannerType = args["type"].String()
	}
	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}

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

	ctx.Private[node.name] = bannerList

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagBannerListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagBannerListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("bannerList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed bannerList-tag arguments.", nil)
	}

	wrapper, endtagargs, err := doc.WrapUntilTag("endbannerList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endbannerList' must equal to 'bannerList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endbannerList'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
