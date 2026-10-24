package tags

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/kataras/iris/v12/context"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagReviewListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagReviewListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	// 站点运行于普通内容模式时不渲染商城元素（合并计划阶段 5.3）
	if currentSite.System.ShopMode == 0 {
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
	archiveId := int64(0)
	order := "id desc"
	limit := 10
	offset := 0
	currentPage := 1
	listType := "list"

	if args["archiveId"] != nil {
		archiveId = int64(args["archiveId"].Integer())
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

	archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
	if ok && archiveDetail != nil {
		archiveId = archiveDetail.Id
	}

	if args["order"] != nil {
		order = args["order"].String()
	}
	if args["limit"] != nil {
		limitArgs := strings.Split(args["limit"].String(), ",")
		if len(limitArgs) == 2 {
			offset, _ = strconv.Atoi(limitArgs[0])
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
	var authorId = uint(0)
	if args["userId"] != nil {
		authorId = uint(args["userId"].Integer())
	}
	if listType != "page" {
		currentPage = 1
	}
	reviewList, total := currentSite.GetOrderReviews(func(tx *gorm.DB) *gorm.DB {
		if authorId > 0 {
			tx = tx.Where("user_id = ?", authorId)
		}
		if archiveId > 0 {
			tx = tx.Where("archive_id = ?", archiveId)
		}
		return tx.Where("status = ?", 1)
	}, order, currentPage, limit, offset)

	if listType == "page" {
		// 如果评论是在文章详情页或产品详情页，则根据具体来判断页码
		var urlPatten string
		var link string
		if archiveDetail != nil {
			// 在文章中
			link = currentSite.GetUrl("archive", archiveDetail, 0)
		}
		if link != "" {
			if strings.Contains(link, "?") {
				urlPatten = fmt.Sprintf("%s(&page={page})", link)
			} else {
				urlPatten = fmt.Sprintf("%s(?page={page})", link)
			}
		}

		ctx.Public["pagination"] = makePagination(currentSite, total, currentPage, limit, urlPatten, 5)
	}
	ctx.Private[node.name] = reviewList
	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagReviewListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagReviewListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("reviewList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed reviewList-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endreviewList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endreviewList' must equal to 'reviewList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endreviewList'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
