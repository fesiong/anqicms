package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
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
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if args["render"] != nil {
		render = args["render"].Bool()
	}

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	categoryDetail, _ := ctx.Public["category"].(*model.Category)
	archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
	if ok && archiveDetail != nil {
		categoryDetail = currentSite.GetCategoryFromCache(archiveDetail.CategoryId)
	}

	if args["id"] != nil {
		if args["id"].String() == "parent" && categoryDetail != nil {
			id = categoryDetail.Id
		} else {
			id = uint(args["id"].Integer())
		}
	}
	if id > 0 {
		categoryDetail = currentSite.GetCategoryFromCache(id)
	} else if token != "" {
		categoryDetail = currentSite.GetCategoryFromCacheByToken(token)
	}

	if categoryDetail != nil {
		categoryDetail.Link = currentSite.GetUrl("category", categoryDetail, 0)
		categoryDetail.Thumb = categoryDetail.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		v := reflect.ValueOf(*categoryDetail)

		f := v.FieldByName(fieldName)
		var content interface{}
		if f.IsValid() {
			content = f.Interface()
		}

		if categoryDetail.SeoTitle == "" && fieldName == "SeoTitle" {
			content = categoryDetail.Title
		}
		// convert markdown to html
		if fieldName == "Content" && render {
			content = library.MarkdownToHTML(categoryDetail.Content)
		}
		// output
		if node.name == "" {
			writer.WriteString(fmt.Sprintf("%v", content))
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
