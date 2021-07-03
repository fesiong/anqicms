package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

type tagPrevProductNode struct {
	name     string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagPrevProductNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if config.DB == nil {
		return nil
	}
	productDetail, ok := ctx.Public["product"].(*model.Product)
	if !ok {
		return nil
	}

	prevProduct, _ := provider.GetPrevProductById(productDetail.CategoryId, productDetail.Id)
	if prevProduct != nil {
		prevProduct.GetThumb()
		prevProduct.Link = provider.GetUrl("product", prevProduct, 0)
	}
	ctx.Private[node.name] = prevProduct
	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagPrevProductParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagPrevProductNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("prevProduct-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed prevProduct-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endprevProduct")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endprevProduct' must equal to 'prevProduct'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endprevProduct'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
