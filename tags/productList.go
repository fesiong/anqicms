package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"github.com/kataras/iris/v12/context"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
	"strconv"
)

type tagProductListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagProductListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if config.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	categoryId := uint(0)
	order := "id desc"
	limit := 10
	currentPage := 1
	listType := "list"
	q := ""

	urlParams, ok := ctx.Public["urlParams"].(map[string]string)
	if ok {
		currentPage, _ = strconv.Atoi(urlParams["page"])
		q = urlParams["q"]
	}
	requestParams, ok := ctx.Public["requestParams"].(*context.RequestParams)
	if ok {
		paramPage := requestParams.GetIntDefault("page", 0)
		if paramPage > 0 {
			currentPage = paramPage
		}
	}

	if args["categoryId"] != nil {
		categoryId = uint(args["categoryId"].Integer())
	}
	if args["order"] != nil {
		order = args["order"].String()
	}
	if args["limit"] != nil {
		limit = args["limit"].Integer()
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}
	if args["type"] != nil {
		listType = args["type"].String()
	}

	var productList []*model.Product
	var total int64
	if listType == "related" {
		//获取id
		productId := uint(0)
		productDetail, ok := ctx.Public["product"].(*model.Product)
		if ok {
			productId = productDetail.Id
		}

		productList, _ = provider.GetRelationProductList(categoryId, productId, limit)
	} else {

		productList, total, _ = provider.GetProductList(categoryId, q, order, currentPage, limit)
	}
	for i := range productList {
		productList[i].GetThumb()
		productList[i].Link = provider.GetUrl("product", productList[i], 0)
	}

	if listType == "page" {
		urlMatch := "productIndex"
		var category *model.Category
		var err error
		if categoryId > 0 {
			category, err = provider.GetCategoryById(categoryId)
			if err == nil {
				urlMatch = "category"
			}
		}
		urlPatten := provider.GetUrl(urlMatch, category, -1)
		ctx.Private["pagination"] = makePagination(total, currentPage, limit, urlPatten, 4)
	}
	ctx.Private[node.name] = productList

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagProductListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagProductListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("productList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed productList-tag arguments.", nil)
	}

	wrapper, endtagargs, err := doc.WrapUntilTag("endproductList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endproductList' must equal to 'productList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endproductList'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
