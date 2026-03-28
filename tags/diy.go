package tags

import (
	"fmt"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagDiyNode struct {
	name string
	args map[string]pongo2.IEvaluator
}

func (node *tagDiyNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}

	render := currentSite.Content.Editor == "markdown"
	if args["render"] != nil {
		render = args["render"].Bool()
	}

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		// fieldName = library.Case2Camel(fieldName)
	}

	var content any

	fields := currentSite.GetDiyFieldSetting()
	var extra = model.ExtraData{}
	for _, v := range fields {
		extra[v.Name] = v.Value
	}
	extraData := provider.ProcessExtra(extra, fields, currentSite, render, fieldName)
	if fieldName != "" {
		if field, ok := extraData[fieldName]; ok {
			content = field
		}
	}

	// output
	if node.name == "" {
		writer.WriteString(fmt.Sprint(content))
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagDiyParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagDiyNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("diy-tag needs a accept name.", nil)
	}

	if nameToken.Val == "with" {
		//with 需要退回
		arguments.ConsumeN(-1)
	} else {
		tagNode.name = nameToken.Val
	}

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed diy-tag arguments.", nil)
	}

	return tagNode, nil
}
