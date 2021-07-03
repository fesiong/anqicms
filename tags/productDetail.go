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

type tagProductDetailNode struct {
	args    map[string]pongo2.IEvaluator
	name     string
}

func (node *tagProductDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	productDetail, ok := ctx.Public["product"].(*model.Product)
	if !ok && id == 0 {
		return nil
	}
	//不是同一个，从新获取
	if productDetail != nil && (id > 0 && productDetail.Id != id) {
		productDetail = nil
	}

	if productDetail == nil && id > 0 {
		var err error
		productDetail, err = provider.GetProductById(id)
		if err != nil {
			return nil
		}
	}

	if productDetail == nil {
		return nil
	}
	productDetail.GetThumb()
	productDetail.Link = provider.GetUrl("product", productDetail, 0)

	v := reflect.ValueOf(*productDetail)

	f := v.FieldByName(fieldName)

	content := fmt.Sprintf("%v", f)

	if fieldName == "CreatedTime" || fieldName == "UpdatedTime" {
		content = time.Unix(f.Int(), 0).Format(format)
	}
	if fieldName == "Content" {
		content = productDetail.ProductData.Content
	}
	if fieldName == "Images" {
		content = fmt.Sprintf("%v", productDetail.Images)
	}

	// output
	if node.name == "" {
		writer.WriteString(content)
	} else {
		//不是所有都是字符串
		if fieldName == "Images" {
			ctx.Private[node.name] = productDetail.Images
		} else {
			ctx.Private[node.name] = content
		}
	}

	return nil
}

func TagProductDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagProductDetailNode{
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
		return nil, arguments.Error("Malformed productDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
