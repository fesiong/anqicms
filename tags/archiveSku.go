package tags

import (
	"fmt"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

type tagArchiveSkuNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagArchiveSkuNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	// 站点运行于普通内容模式时不渲染商城元素（合并计划阶段 5.3）
	if currentSite.System.ShopMode == 0 {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := int64(0)
	if args["id"] != nil {
		id = int64(args["id"].Integer())
	}

	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}

	userId, _ := ctx.Public["userId"].(uint)

	req := &request.ApiArchiveRequest{
		Id:     int64(id),
		UserId: userId,
	}

	skuResp, err2 := currentSite.ApiGetSku(req)
	if err2 != nil {
		return nil
	}

	ctx.Private[node.name] = skuResp

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagArchiveSkuParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagArchiveSkuNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("archiveSku-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed archiveSku-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endarchiveSku")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endarchiveSku' must equal to 'archiveSku'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endarchiveSku'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
