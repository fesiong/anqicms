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

type tagArticleListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagArticleListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	if args["q"] != nil {
		q = args["q"].String()
	}

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

	var articleList []*model.Article
	var total int64
	if listType == "related" {
		//获取id
		articleId := uint(0)
		articleDetail, ok := ctx.Public["article"].(*model.Article)
		if ok {
			articleId = articleDetail.Id
		}

		articleList, _ = provider.GetRelationArticleList(categoryId, articleId, limit)
	} else {

		articleList, total, _ = provider.GetArticleList(categoryId, q, order, currentPage, limit)
	}

	for i := range articleList {
		articleList[i].GetThumb()
		articleList[i].Link = provider.GetUrl("article", articleList[i], 0)
	}

	if listType == "page" {
		urlMatch := "articleIndex"
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
	ctx.Private[node.name] = articleList

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagArticleListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagArticleListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("articleList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed articleList-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endarticleList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endarticleList' must equal to 'articleList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endarticleList'.", nil)
		}
	}
	tagNode.wrapper = wrapper
	
	return tagNode, nil
}
