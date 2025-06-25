package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"math"
	"net/url"
	"regexp"
	"strings"
)

const PagePlaceholder = "{page}"

type pageItem struct {
	Name      string
	Link      string
	IsCurrent bool
}

type pagination struct {
	w            *provider.Website
	TotalItems   int64
	TotalPages   int
	pageSize     int
	CurrentPage  int
	urlPatten    string
	maxPagesShow int
	maxPages     int

	FirstPage *pageItem
	LastPage  *pageItem
	PrevPage  *pageItem
	NextPage  *pageItem
	Pages     []*pageItem
}

type tagPaginationNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagPaginationNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	paginator, ok := ctx.Public["pagination"].(*pagination)
	if !ok {
		return nil
	}

	if args["show"] != nil {
		paginator.maxPagesShow = args["show"].Integer()
	}

	// 支持重定义pattern
	if args["prefix"] != nil {
		paginator.urlPatten = args["prefix"].String()
	}

	if paginator.urlPatten == "" {
		webInfo, ok := ctx.Public["webInfo"].(*response.WebInfo)
		if ok {
			paginator.urlPatten = webInfo.CanonicalUrl
		}
	}
	if !strings.Contains(paginator.urlPatten, PagePlaceholder) {
		// 先判断是否已经有page了，这里只判断 page=\d 模式
		if strings.Contains(paginator.urlPatten, "page=") {
			reg, _ := regexp.Compile(`(([&?])page=\d*)`)
			paginator.urlPatten = reg.ReplaceAllStringFunc(paginator.urlPatten, func(s string) string {
				// 移除
				return ""
			})
		}
		if strings.Contains(paginator.urlPatten, "?") {
			paginator.urlPatten += "&page=" + PagePlaceholder
		} else {
			paginator.urlPatten += "?page=" + PagePlaceholder
		}
	}

	// 分页的时候，还需要检查是否有搜索参数，有的话，也要加上
	urlParams, ok := ctx.Public["urlParams"].(map[string]string)
	if ok && len(urlParams) > 0 {
		urlPatten, err := url.Parse(paginator.urlPatten)
		if err == nil {
			urlQuery := urlPatten.Query()
			for k, v := range urlParams {
				if k == "page" {
					continue
				}
				urlQuery.Set(k, v)
			}
			urlPatten.RawQuery = urlQuery.Encode()
			paginator.urlPatten = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(urlPatten.String(), "%7Bpage%7D", PagePlaceholder), "%28", "("), "%29", ")")
		}
	}

	if paginator.TotalPages <= 1 {
		return nil
	}

	//计算
	paginator.FirstPage = paginator.getFirstPage()
	paginator.LastPage = paginator.getLastPage()
	paginator.PrevPage = paginator.getPrevPage()
	paginator.NextPage = paginator.getNextPage()
	paginator.Pages = paginator.getPages()

	ctx.Private[node.name] = paginator

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagPaginationParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagPaginationNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("pagination-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed pagination-tag arguments.", nil)
	}

	wrapper, endtagargs, err := doc.WrapUntilTag("endpagination")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endpagination' must equal to 'pagination'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endpagination'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}

func makePagination(currentSite *provider.Website, TotalItems int64, currentPage, pageSize int, urlPatten string, maxPagesShow int) *pagination {
	if currentPage < 1 {
		currentPage = 1
	}
	pager := &pagination{
		w:            currentSite,
		TotalItems:   TotalItems,
		TotalPages:   0,
		pageSize:     pageSize,
		CurrentPage:  currentPage,
		urlPatten:    urlPatten,
		maxPagesShow: maxPagesShow,
		maxPages:     currentSite.Content.MaxPage, // 最大显示1000页
	}

	//计算TotalPages
	pager.TotalPages = int(math.Ceil(float64(TotalItems) / float64(pageSize)))

	return pager
}

func (p *pagination) getFirstPage() *pageItem {
	item := &pageItem{
		Name: p.w.TplTr("FirstPage"),
		Link: p.getPageUrl(1),
	}

	if p.CurrentPage == 1 {
		item.IsCurrent = true
	}

	return item
}

func (p *pagination) getLastPage() *pageItem {
	lastPage := p.TotalPages
	if lastPage > p.maxPages {
		lastPage = p.maxPages
	}
	item := &pageItem{
		Name: p.w.TplTr("LastPage"),
		Link: p.getPageUrl(lastPage),
	}

	if p.CurrentPage == lastPage {
		item.IsCurrent = true
	}

	return item
}

func (p *pagination) getPrevPage() *pageItem {
	if p.CurrentPage == 1 {
		return nil
	}

	item := &pageItem{
		Name: p.w.TplTr("PreviousPage"),
		Link: p.getPageUrl(p.CurrentPage - 1),
	}

	return item
}

func (p *pagination) getNextPage() *pageItem {
	if p.CurrentPage == p.TotalPages {
		return nil
	}
	item := &pageItem{
		Name: p.w.TplTr("NextPage"),
		Link: p.getPageUrl(p.CurrentPage + 1),
	}

	return item
}

func (p *pagination) createPage(page int) *pageItem {
	item := &pageItem{
		Name:      fmt.Sprintf("%d", page),
		Link:      p.getPageUrl(page),
		IsCurrent: page == p.CurrentPage,
	}

	return item
}

func (p *pagination) createPageEllipsis() *pageItem {
	item := &pageItem{
		Name:      "...",
		Link:      "",
		IsCurrent: false,
	}

	return item
}

func (p *pagination) getPages() []*pageItem {
	var pages []*pageItem
	if p.TotalPages <= 1 {
		return pages
	}
	lastPage := p.TotalPages
	if lastPage > p.maxPages {
		lastPage = p.maxPages
	}
	if lastPage <= p.maxPagesShow {
		for i := 1; i <= lastPage; i++ {
			pages = append(pages, p.createPage(i))
		}
	} else {
		slidingStart := 1
		numAdjacent := (p.maxPagesShow - 3) / 2
		if p.CurrentPage+numAdjacent > lastPage {
			slidingStart = lastPage - p.maxPagesShow + 2
		} else {
			slidingStart = p.CurrentPage - numAdjacent
		}
		if slidingStart < 2 {
			slidingStart = 2
		}
		slidingEnd := slidingStart + p.maxPagesShow - 3
		if slidingEnd >= lastPage {
			slidingEnd = lastPage - 1
		}
		pages = append(pages, p.createPage(1))
		if slidingStart > 2 {
			pages = append(pages, p.createPageEllipsis())
		}
		for i := slidingStart; i <= slidingEnd; i++ {
			pages = append(pages, p.createPage(i))
		}
		if slidingEnd < lastPage-1 {
			pages = append(pages, p.createPageEllipsis())
		}

		pages = append(pages, p.createPage(lastPage))
	}

	return pages
}

func (p *pagination) getPageUrl(page int) string {
	link := p.urlPatten

	//如果是第一页，不需要携带页码
	if page > 1 {
		link = strings.ReplaceAll(link, PagePlaceholder, fmt.Sprintf("%d", page))
		link = strings.ReplaceAll(link, "(", "")
		link = strings.ReplaceAll(link, ")", "")
	} else {
		reg := regexp.MustCompile("\\(.*\\)")
		link = reg.ReplaceAllString(link, "")
		reg = regexp.MustCompile(`page=\{page\}&?`)
		link = reg.ReplaceAllString(link, "")
		link = strings.Trim(link, "?")
		link = strings.Trim(link, "&")
	}

	return link
}
