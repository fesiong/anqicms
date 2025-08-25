package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/config"
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
	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if args["render"] != nil {
		render = args["render"].Bool()
	}

	pageDetail, _ := ctx.Public["page"].(*model.Category)
	if id > 0 {
		pageDetail = currentSite.GetCategoryFromCache(id)
	} else if token != "" {
		pageDetail = currentSite.GetCategoryFromCacheByToken(token)
	}
	if pageDetail == nil {
		return nil
	}

	if pageDetail.Type != config.CategoryTypePage {
		return nil
	}
	pageDetail.Link = currentSite.GetUrl("page", pageDetail, 0)
	pageDetail.Thumb = pageDetail.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)

	v := reflect.ValueOf(*pageDetail)

	f := v.FieldByName(fieldName)
	var content interface{}
	if f.IsValid() {
		content = f.Interface()
	}

	if pageDetail.SeoTitle == "" && fieldName == "SeoTitle" {
		content = pageDetail.Title
	}
	// convert markdown to html
	if fieldName == "Content" && render {
		content = library.MarkdownToHTML(pageDetail.Content, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
	}
	if node.name == "" {
		writer.WriteString(fmt.Sprintf("%v", content))
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
		return nil, arguments.Error("pageDetail-tag needs a page field name.", nil)
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
