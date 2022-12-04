package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"reflect"
)

type tagTagDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagTagDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := uint(0)

	tagDetail, _ := ctx.Public["tag"].(*model.Tag)
	if args["id"] != nil {
		id = uint(args["id"].Integer())
		tagDetail, _ = currentSite.GetTagById(id)
	}

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	var content string

	if tagDetail != nil {
		tagDetail.Link = currentSite.GetUrl("tag", tagDetail, 0)

		v := reflect.ValueOf(*tagDetail)

		f := v.FieldByName(fieldName)

		content = fmt.Sprintf("%v", f)
		if content == "" && fieldName == "SeoTitle" {
			content = tagDetail.Title
		}
	}

	// output
	if node.name == "" {
		writer.WriteString(content)
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagTagDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagTagDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("tagDetail-tag needs a accept name.", nil)
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
		return nil, arguments.Error("Malformed tagDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
