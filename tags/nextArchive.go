package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagNextArchiveNode struct {
	name     string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagNextArchiveNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if dao.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := uint(0)

	archiveDetail, _ := ctx.Public["archive"].(*model.Archive)

	if args["id"] != nil {
		id = uint(args["id"].Integer())
		archiveDetail, _ = provider.GetArchiveById(id)
	}

	if archiveDetail != nil {
		var nextArchive model.Archive
		if err2 := dao.DB.Model(&model.Archive{}).Where("`module_id` = ? AND `category_id` = ?", archiveDetail.ModuleId, archiveDetail.CategoryId).Where("`id` > ?", archiveDetail.Id).Where("`status` = 1").First(&nextArchive).Error; err2 == nil {
			nextArchive.GetThumb()
			nextArchive.Link = provider.GetUrl("archive", &nextArchive, 0)

			ctx.Private[node.name] = nextArchive
		}
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
