package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"log"
	"reflect"
)

type tagUserGroupDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagUserGroupDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	var queryDetail *model.UserGroup

	if args["id"] != nil {
		id := uint(args["id"].Integer())
		queryDetail, _ = currentSite.GetUserGroupInfo(id)
	}
	if args["level"] != nil {
		level := args["level"].Integer()
		log.Println("level", level)
		queryDetail, _ = currentSite.GetUserGroupInfoByLevel(level)
	}
	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	groupDetail, ok := ctx.Public["userGroup"].(*model.UserGroup)
	if !ok && queryDetail == nil {
		return nil
	}
	//不是同一个，重新获取
	if groupDetail == nil || (queryDetail != nil && queryDetail.Id != groupDetail.Id) {
		groupDetail = queryDetail
	}

	if groupDetail == nil {
		return nil
	}

	v := reflect.ValueOf(*groupDetail)

	f := v.FieldByName(fieldName)
	var content interface{}
	if f.IsValid() {
		content = f.Interface()
	}
	if node.name == "" {
		writer.WriteString(fmt.Sprintf("%v", content))
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagUserGroupDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagUserGroupDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("userGroupDetail-tag needs a userGroup field name.", nil)
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
		return nil, arguments.Error("Malformed userGroupDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
