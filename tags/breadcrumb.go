package tags

import (
	"fmt"
	"github.com/iris-contrib/pongo2"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

type tagBreadcrumbNode struct {
	name string
	args map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

type crumb struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

func (node *tagBreadcrumbNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if dao.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	index := config.Lang("首页")
	if args["index"] != nil {
		index = args["index"].String()
	}

	showTitle := true
	if args["title"] != nil {
		showTitle = args["title"].Bool()
	}

	var crumbs []*crumb

	crumbs = append(crumbs, &crumb{
		Name: index,
		Link: "/",
	})

	webInfo, ok := ctx.Public["webInfo"].(response.WebInfo)
	if ok {
		switch webInfo.PageName {
		case "archiveIndex":
			module, ok := ctx.Public["module"].(*model.Module)
			if ok {
				crumbs = append(crumbs, &crumb{
					Name: module.Title,
					Link: provider.GetUrl("archiveIndex", module, 0),
				})
			}
			break
		case "archiveList":
			categoryInfo, ok := ctx.Public["category"].(*model.Category)
			if ok {
				crumbs = append(crumbs, buildCategoryCrumbs(categoryInfo.ParentId)...)
				crumbs = append(crumbs, &crumb{
					Name: categoryInfo.Title,
					Link: provider.GetUrl("category", categoryInfo, 0),
				})
			}
			break
		case "archiveDetail":
			archive, ok := ctx.Public["archive"].(*model.Archive)
			if ok {
				//检查是否存在分类
				crumbs = append(crumbs, buildCategoryCrumbs(archive.CategoryId)...)

				title := archive.Title
				if !showTitle {
					title = config.Lang("正文")
				}
				crumbs = append(crumbs, &crumb{
					Name: title,
					Link: "",
				})
			}
			break
		case "comments":
			itemData, ok := ctx.Public["itemData"].(*model.Archive)
			if ok {
				crumbs = append(crumbs, &crumb{
					Name: itemData.Title,
					Link: provider.GetUrl("archive", itemData, 0),
				})
			}
			crumbs = append(crumbs, &crumb{
				Name: config.Lang("评论"),
				Link: "",
			})
			break
		case "guestbook":
			crumbs = append(crumbs, &crumb{
				Name: config.Lang("留言板"),
				Link: provider.GetUrl("/guestbook.html", nil, 0),
			})
			break
		case "pageDetail":
			pageInfo, ok := ctx.Public["page"].(*model.Category)
			if ok {
				crumbs = append(crumbs, &crumb{
					Name: pageInfo.Title,
					Link: provider.GetUrl("page", pageInfo, 0),
				})
			}
			break
		default:
			crumbs = append(crumbs, &crumb{
				Name: webInfo.Title,
				Link: "",
			})
		}
	}

	ctx.Private[node.name] = crumbs
	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagBreadcrumbParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagBreadcrumbNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("breadcrumb-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed breadcrumb-tag arguments.", nil)
	}

	wrapper, endtagargs, err := doc.WrapUntilTag("endbreadcrumb")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endbreadcrumb' must equal to 'breadcrumb'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endbreadcrumb'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}

func buildCategoryCrumbs(categoryId uint) []*crumb {
	var crumbs []*crumb
	if categoryId > 0 {
		category := provider.GetCategoryFromCache(categoryId)
		if category != nil {
			if category.ParentId > 0 {
				crumbs = buildCategoryCrumbs(category.ParentId)
			}

			crumbs = append(crumbs, &crumb{
				Name: category.Title,
				Link: provider.GetUrl("category", category, 0),
			})
		}
	}

	return crumbs
}
