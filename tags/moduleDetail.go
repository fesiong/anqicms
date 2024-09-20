package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"reflect"
)

type tagModuleDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagModuleDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := uint(0)
	token := ""
	if args["token"] != nil {
		token = args["token"].String()
	}

	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}
	if args["id"] != nil {
		id = uint(args["id"].Integer())
	}
	module, _ := ctx.Public["module"].(*model.Module)
	if args["id"] != nil {
		id = uint(args["id"].Integer())
	}
	if id > 0 {
		module = currentSite.GetModuleFromCache(id)
	} else if token != "" {
		module = currentSite.GetModuleFromCacheByToken(token)
	}

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	var content interface{}

	if module != nil {
		module.Link = currentSite.GetUrl("archiveIndex", module, 0)

		v := reflect.ValueOf(*module)

		f := v.FieldByName(fieldName)
		if f.IsValid() {
			content = f.Interface()
		}
		if content == "" && fieldName == "SeoTitle" {
			content = module.Title
		}
	}

	// output
	if node.name == "" {
		writer.WriteString(fmt.Sprintf("%v", content))
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagModuleDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagModuleDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("moduleDetail-tag needs a accept name.", nil)
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
		return nil, arguments.Error("Malformed moduleDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
