package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v4"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"reflect"
)

type tagCategoryDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagCategoryDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if dao.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := uint(0)

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	categoryDetail, _ := ctx.Public["category"].(*model.Category)
	archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
	if ok && archiveDetail != nil {
		categoryDetail = provider.GetCategoryFromCache(archiveDetail.CategoryId)
	}

	if args["id"] != nil {
		id = uint(args["id"].Integer())
		categoryDetail = provider.GetCategoryFromCache(id)
	}

	if categoryDetail != nil {
		categoryDetail.Link = provider.GetUrl("category", categoryDetail, 0)

		v := reflect.ValueOf(*categoryDetail)

		f := v.FieldByName(fieldName)

		content := fmt.Sprintf("%v", f)

		// output
		if node.name == "" {
			writer.WriteString(content)
		} else {
			if fieldName == "Images" {
				ctx.Private[node.name] = categoryDetail.Images
			} else {
				ctx.Private[node.name] = content
			}
		}
	}

	return nil
}

func TagCategoryDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagCategoryDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("categoryDetail-tag needs a accept name.", nil)
	}

	if nameToken.Val == "with" {
		//with 需要退回
		arguments.ConsumeN(-1)
	} else {
		tagNode.name = nameToken.Val
	}

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed categoryDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
