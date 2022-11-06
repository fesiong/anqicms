package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v4"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"reflect"
)

type tagPageDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagPageDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if dao.DB == nil {
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
	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	pageDetail, ok := ctx.Public["page"].(*model.Category)
	if !ok && id == 0 {
		return nil
	}
	//不是同一个，重新获取
	if pageDetail != nil && (id > 0 && pageDetail.Id != id) {
		pageDetail = nil
	}

	if pageDetail == nil && id > 0 {
		pageDetail = provider.GetCategoryFromCache(id)
		if pageDetail == nil {
			return nil
		}
	}
	if pageDetail == nil {
		return nil
	}

	if pageDetail.Type != config.CategoryTypePage {
		return nil
	}
	pageDetail.Link = provider.GetUrl("page", pageDetail, 0)

	v := reflect.ValueOf(*pageDetail)

	f := v.FieldByName(fieldName)

	content := fmt.Sprintf("%v", f)

	if node.name == "" {
		writer.WriteString(content)
	} else {
		if fieldName == "Images" {
			ctx.Private[node.name] = pageDetail.Images
		} else {
			ctx.Private[node.name] = content
		}
	}

	return nil
}

func TagPageDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagPageDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("System-tag needs a system config name.", nil)
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
		return nil, arguments.Error("Malformed pageDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
