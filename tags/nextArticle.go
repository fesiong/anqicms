package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

type tagNextArticleNode struct {
	name     string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagNextArticleNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if config.DB == nil {
		return nil
	}
	articleDetail, ok := ctx.Public["article"].(*model.Article)
	if !ok {
		return nil
	}

	nextArticle, _ := provider.GetNextArticleById(articleDetail.CategoryId, articleDetail.Id)
	if nextArticle != nil {
		nextArticle.GetThumb()
		nextArticle.Link = provider.GetUrl("article", nextArticle, 0)
	}
	ctx.Private[node.name] = nextArticle
	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagNextArticleParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagNextArticleNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("nextArticle-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed nextArticle-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endnextArticle")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endnextArticle' must equal to 'nextArticle'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endnextArticle'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
