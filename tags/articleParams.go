package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

type tagArticleParamsNode struct {
	name string
	args map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagArticleParamsNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if config.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := uint(0)

	if args["id"] != nil {
		id = uint(args["id"].Integer())
	}
	sorted := true
	if args["sorted"] != nil {
		sorted = args["sorted"].Bool()
	}

	articleDetail, ok := ctx.Public["article"].(*model.Article)
	if ok && id == 0 {
		id = articleDetail.Id
	}
	articleParams := provider.GetArticleExtra(id)

	if len(articleParams) == 0 {
		return nil
	}

	if sorted {
		var extraFields []*model.CustomField
		if len(config.JsonData.ArticleExtraFields) > 0 {
			for _, v := range config.JsonData.ArticleExtraFields {
				extraFields = append(extraFields, articleParams[v.FieldName])
			}
		}

		ctx.Private[node.name] = extraFields
	} else {
		ctx.Private[node.name] = articleParams
	}

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagArticleParamsParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagArticleParamsNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("articleParams-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed articleParams-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endarticleParams")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endarticleParams' must equal to 'articleParams'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endarticleParams'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
