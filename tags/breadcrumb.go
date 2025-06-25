package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

type tagBreadcrumbNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

type crumb struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

func (node *tagBreadcrumbNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	index := currentSite.TplTr("Home")
	if args["index"] != nil {
		index = args["index"].String()
	}

	showTitle := true
	var titleText string
	if args["title"] != nil {
		tmpText := args["title"].String()
		if tmpText == "False" || tmpText == "false" {
			showTitle = false
		} else if tmpText != "True" && tmpText != "true" {
			titleText = tmpText
		}
	}

	var crumbs []*crumb

	crumbs = append(crumbs, &crumb{
		Name: index,
		Link: "/",
	})

	webInfo, ok := ctx.Public["webInfo"].(*response.WebInfo)
	if ok {
		switch webInfo.PageName {
		case "archiveIndex":
			module, ok := ctx.Public["module"].(*model.Module)
			if ok {
				crumbs = append(crumbs, &crumb{
					Name: module.Title,
					Link: currentSite.GetUrl("archiveIndex", module, 0),
				})
			}
			break
		case "archiveList":
			categoryInfo, ok := ctx.Public["category"].(*model.Category)
			if ok {
				crumbs = append(crumbs, buildCategoryCrumbs(currentSite, categoryInfo.ParentId)...)
				crumbs = append(crumbs, &crumb{
					Name: categoryInfo.Title,
					Link: currentSite.GetUrl("category", categoryInfo, 0),
				})
			}
			break
		case "archiveDetail":
			archive, ok := ctx.Public["archive"].(*model.Archive)
			if ok {
				//检查是否存在分类
				crumbs = append(crumbs, buildCategoryCrumbs(currentSite, archive.CategoryId)...)

				if showTitle {
					title := archive.Title
					if titleText != "" {
						title = titleText
					}
					crumbs = append(crumbs, &crumb{
						Name: title,
						Link: "",
					})
				}
			}
			break
		case "comments":
			itemData, ok := ctx.Public["itemData"].(*model.Archive)
			if ok {
				crumbs = append(crumbs, &crumb{
					Name: itemData.Title,
					Link: currentSite.GetUrl("archive", itemData, 0),
				})
			}
			crumbs = append(crumbs, &crumb{
				Name: currentSite.TplTr("Comment"),
				Link: "",
			})
			break
		case "guestbook":
			crumbs = append(crumbs, &crumb{
				Name: currentSite.TplTr("MessageBoard"),
				Link: currentSite.GetUrl("/guestbook.html", nil, 0),
			})
			break
		case "pageDetail":
			pageInfo, ok := ctx.Public["page"].(*model.Category)
			if ok {
				crumbs = append(crumbs, &crumb{
					Name: pageInfo.Title,
					Link: currentSite.GetUrl("page", pageInfo, 0),
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

func buildCategoryCrumbs(currentSite *provider.Website, categoryId uint) []*crumb {
	var crumbs []*crumb
	if categoryId > 0 {
		categories := currentSite.GetParentCategories(categoryId)
		for i := range categories {
			crumbs = append(crumbs, &crumb{
				Name: categories[i].Title,
				Link: currentSite.GetUrl("category", categories[i], 0),
			})
		}
	}

	return crumbs
}
