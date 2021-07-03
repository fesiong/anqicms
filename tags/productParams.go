package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

type tagProductParamsNode struct {
	name string
	args map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagProductParamsNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	articleDetail, ok := ctx.Public["product"].(*model.Article)
	if ok && id == 0 {
		id = articleDetail.Id
	}

	productParams := provider.GetProductExtra(id)
	if len(productParams) == 0 {
		return nil
	}

	if sorted {
		var extraFields []*model.CustomField
		if len(config.JsonData.ProductExtraFields) > 0 {
			for _, v := range config.JsonData.ProductExtraFields {
				extraFields = append(extraFields, productParams[v.FieldName])
			}
		}

		ctx.Private[node.name] = extraFields
	} else {
		ctx.Private[node.name] = productParams
	}

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagProductParamsParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagProductParamsNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("productParams-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed productParams-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endproductParams")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endproductParams' must equal to 'productParams'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endproductParams'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
