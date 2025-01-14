package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagNextArchiveNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagNextArchiveNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := int64(0)

	archiveDetail, _ := ctx.Public["archive"].(*model.Archive)

	if args["id"] != nil {
		id = int64(args["id"].Integer())
		archiveDetail, _ = currentSite.GetArchiveById(id)
	}

	if archiveDetail != nil {
		nextArchive, _ := currentSite.GetArchiveByFunc(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("`category_id` = ?", archiveDetail.CategoryId).Where("`id` > ?", archiveDetail.Id).Order("`id` ASC")
		})
		if nextArchive != nil && len(nextArchive.Password) > 0 {
			nextArchive.HasPassword = true
		}
		ctx.Private[node.name] = nextArchive
	}
	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagNextArchiveParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagNextArchiveNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("nextArchive-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed nextArchive-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endnextArchive")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endnextArchive' must equal to 'nextArchive'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endnextArchive'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
