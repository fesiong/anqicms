package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagPrevArchiveNode struct {
	name     string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagPrevArchiveNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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
		var prevArchive model.Archive
		if err2 := dao.DB.Model(&model.Archive{}).Where("`module_id` = ? AND `category_id` = ?", archiveDetail.ModuleId, archiveDetail.CategoryId).Where("`id` < ?", archiveDetail.Id).Where("`status` = 1").First(&prevArchive).Error; err2 == nil {
			prevArchive.GetThumb()
			prevArchive.Link = provider.GetUrl("archive", &prevArchive, 0)
			ctx.Private[node.name] = prevArchive
		}
	}

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagPrevArchiveParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagPrevArchiveNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("prevArchive-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed prevArchive-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endprevArchive")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endprevArchive' must equal to 'prevArchive'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endprevArchive'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
