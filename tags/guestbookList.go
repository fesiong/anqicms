package tags

import (
	"fmt"
	"gorm.io/gorm"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/provider"
)

type tagGuestbookListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagGuestbookListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	limit := 10
	currentPage := 1
	listType := "list"

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
			limit, _ = strconv.Atoi(limitArgs[1])
		} else if len(limitArgs) == 1 {
			limit, _ = strconv.Atoi(limitArgs[0])
		}
		if limit > currentSite.Content.MaxLimit {
			limit = currentSite.Content.MaxLimit
		}
		if limit < 1 {
			limit = 1
		}
	}
	if args["type"] != nil {
		listType = args["type"].String()
	}
	if listType != "page" {
		currentPage = 1
	}
	keyword := ""
	if args["keyword"] != nil {
		keyword = args["keyword"].String()
	}
	status := 1
	guestbookList, total, _ := currentSite.GetGuestbookList(func(tx *gorm.DB) *gorm.DB {
		if keyword != "" {
			tx = tx.Where("user_name like ? or contact like ? or content like ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
		}
		return tx.Where("`status` = ?", status)
	}, currentPage, limit)

	if listType == "page" {
		ctx.Public["pagination"] = makePagination(currentSite, total, currentPage, limit, "", 5)
	}
	ctx.Private[node.name] = guestbookList
	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagGuestbookListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagGuestbookListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("guestbookList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed guestbookList-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endguestbookList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endguestbookList' must equal to 'guestbookList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endguestbookList'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
