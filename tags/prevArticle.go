package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

type tagPrevArticleNode struct {
	name     string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagPrevArticleNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if config.DB == nil {
		return nil
	}
	articleDetail, ok := ctx.Public["article"].(*model.Article)
	if !ok {
		return nil
	}

	prevArticle, _ := provider.GetPrevArticleById(articleDetail.CategoryId, articleDetail.Id)
	if prevArticle != nil {
		prevArticle.GetThumb()
		prevArticle.Link = provider.GetUrl("article", prevArticle, 0)
	}
	ctx.Private[node.name] = prevArticle
	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagPrevArticleParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagPrevArticleNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("prevArticle-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed prevArticle-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endprevArticle")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endprevArticle' must equal to 'prevArticle'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endprevArticle'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
