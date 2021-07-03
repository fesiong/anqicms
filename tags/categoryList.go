package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"irisweb/config"
	"irisweb/provider"
)

type tagCategoryListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagCategoryListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if config.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	if args["type"] == nil {
		return nil
	}

	categoryType := args["type"].Integer()
	parentId := uint(0)
	if args["parentId"] != nil {
		parentId = uint(args["parentId"].Integer())
	}

	categoryList, _ := provider.GetCategories(uint(categoryType), parentId)
	for i := range categoryList {
		categoryList[i].Link = provider.GetUrl("category", categoryList[i], 0)
	}

	ctx.Private[node.name] = categoryList

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
