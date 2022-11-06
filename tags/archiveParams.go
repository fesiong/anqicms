package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v4"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagArchiveParamsNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagArchiveParamsNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if dao.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := uint(0)

	sorted := true
	if args["sorted"] != nil {
		sorted = args["sorted"].Bool()
	}

	archiveDetail, _ := ctx.Public["archive"].(*model.Archive)

	if args["id"] != nil {
		id = uint(args["id"].Integer())
		archiveDetail, _ = provider.GetArchiveById(id)
	}

	if archiveDetail != nil {
		archiveParams := provider.GetArchiveExtra(archiveDetail.ModuleId, archiveDetail.Id)

		if len(archiveParams) > 0 {
			for i := range archiveParams {
				if archiveParams[i].Value == nil || archiveParams[i].Value == "" {
					archiveParams[i].Value = archiveParams[i].Default
				}
			}
			if sorted {
				var extraFields []*model.CustomField
				module := provider.GetModuleFromCache(archiveDetail.ModuleId)
				if module != nil && len(module.Fields) > 0 {
					for _, v := range module.Fields {
						extraFields = append(extraFields, archiveParams[v.FieldName])
					}
				}

				ctx.Private[node.name] = extraFields
			} else {
				ctx.Private[node.name] = archiveParams
			}
		}
	}

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagArchiveParamsParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagArchiveParamsNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("archiveParams-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed archiveParams-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endarchiveParams")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endarchiveParams' must equal to 'archiveParams'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endarchiveParams'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
