package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"strconv"
	"strings"
)

type tagTagListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagTagListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if dao.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	limit := 10
	offset := 0
	currentPage := 1
	itemId := uint(0)

	if args["itemId"] != nil {
		itemId = uint(args["itemId"].Integer())
	} else {
		// 自动获取
		archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
		if ok{
			itemId = archiveDetail.Id
		}
	}

	letter := ""
	if args["letter"] != nil {
		letter = strings.ToUpper(args["letter"].String())
	}
	urlParams, ok := ctx.Public["urlParams"].(map[string]string)
	if ok {
		currentPage, _ = strconv.Atoi(urlParams["page"])
	}
	requestParams, ok := ctx.Public["requestParams"].(*context.RequestParams)
	if ok {
		paramPage := requestParams.GetIntDefault("page", 0)
		if paramPage > 0 {
			currentPage = paramPage
		}
	}
	if args["limit"] != nil {
		limitArgs := strings.Split(args["limit"].String(), ",")
		if len(limitArgs) == 2 {
			offset, _ = strconv.Atoi(limitArgs[0])
			limit, _ = strconv.Atoi(limitArgs[1])
		} else if len(limitArgs) == 1 {
			limit, _ = strconv.Atoi(limitArgs[0])
		}
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}

	tagList, total, _ := provider.GetTagList(itemId, "", letter, currentPage, limit, offset)
	for i := range tagList {
		tagList[i].Link = provider.GetUrl("tag", tagList[i], 0)
	}
	// 分页
	urlPatten := provider.GetUrl("tagIndex", nil, -1)
	ctx.Private["pagination"] = makePagination(total, currentPage, limit, urlPatten, 5)

	ctx.Private[node.name] = tagList

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagTagListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagTagListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("tagList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed tagList-tag arguments.", nil)
	}
	
	wrapper, endtagargs, err := doc.WrapUntilTag("endtagList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endtagList' must equal to 'tagList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endtagList'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
