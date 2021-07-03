package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"irisweb/provider"
	"reflect"
	"time"
)

type tagArticleDetailNode struct {
	args    map[string]pongo2.IEvaluator
	name     string
}

func (node *tagArticleDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if config.DB == nil {
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

	format := "2006-01-02"
	if args["format"] != nil {
		format = args["format"].String()
	}

	articleDetail, ok := ctx.Public["article"].(*model.Article)
	if !ok && id == 0 {
		return nil
	}
	//不是同一个，从新获取
	if articleDetail != nil && (id > 0 && articleDetail.Id != id) {
		articleDetail = nil
	}

	if articleDetail == nil && id > 0 {
		var err error
		articleDetail, err = provider.GetArticleById(id)
		if err != nil {
			return nil
		}
	}
	if articleDetail == nil {
		return nil
	}
	articleDetail.GetThumb()
	articleDetail.Link = provider.GetUrl("article", articleDetail, 0)

	v := reflect.ValueOf(*articleDetail)

	f := v.FieldByName(fieldName)

	content := fmt.Sprintf("%v", f)

	if fieldName == "CreatedTime" || fieldName == "UpdatedTime" {
		content = time.Unix(f.Int(), 0).Format(format)
	}
	if fieldName == "Content" {
		content = articleDetail.ArticleData.Content
	}
	if fieldName == "Images" {
		content = fmt.Sprintf("%v", articleDetail.Images)
	}

	// output
	if node.name == "" {
		writer.WriteString(content)
	} else {
		//不是所有都是字符串
		if fieldName == "Images" {
			ctx.Private[node.name] = articleDetail.Images
		} else {
			ctx.Private[node.name] = content
		}
	}

	return nil
}

func TagArticleDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagArticleDetailNode{
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
		return nil, arguments.Error("Malformed articleDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
