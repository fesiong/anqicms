package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"net/url"
	"strings"
)

type tagArchiveFiltersNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagArchiveFiltersNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	if args["moduleId"] == nil {
		return nil
	}

	moduleId := uint(args["moduleId"].Integer())
	module := currentSite.GetModuleFromCache(moduleId)
	if module == nil {
		return nil
	}

	allText := currentSite.TplTr("All")

	if args["allText"] != nil {
		if args["allText"].IsBool() {
			allText = ""
		} else {
			allText = args["allText"].String()
			if allText == "false" {
				// 文字版的也等同于不显示
				allText = ""
			}
		}
	}

	// 只有有多项选择的才能进行筛选，如 单选，多选，下拉，并且不是跟随阅读等级
	var fields []config.CustomField
	var filterGroups []response.FilterGroup
	var newParams = make(url.Values)
	urlParams, ok := ctx.Public["urlParams"].(map[string]string)
	if ok && len(urlParams) > 0 {
		for k, v := range urlParams {
			if k == "page" {
				continue
			}
			newParams.Set(k, v)
		}
	}
	newQuery := newParams.Encode()
	urlMatch := ""
	matchParams, ok := ctx.Public["requestParams"].(*context.RequestParams)
	if ok {
		urlMatch = matchParams.Get("match")
	}
	var matchData interface{}
	categoryDetail, ok := ctx.Public["category"].(*model.Category)
	if ok && categoryDetail != nil {
		matchData = categoryDetail
		urlMatch = "category"
	} else {
		// 在 module 下
		moduleDetail, ok := ctx.Public["module"].(*model.Module)
		if ok && moduleDetail != nil {
			matchData = moduleDetail
			urlMatch = "archiveIndex"
		}
	}
	urlPatten := currentSite.GetUrl(urlMatch, matchData, 1)
	if strings.Contains(urlPatten, "?") {
		urlPatten += "&"
	} else {
		urlPatten += "?"
	}

	if len(module.Fields) > 0 {
		for _, v := range module.Fields {
			if v.IsFilter {
				fields = append(fields, v)
			}
		}

		// 所有参数的url都附着到query中
		for _, v := range fields {
			values := v.SplitContent()
			if len(values) == 0 {
				continue
			}

			var filterItems []response.FilterItem
			if allText != "" {
				tmpParams, _ := url.ParseQuery(newQuery)
				tmpParams.Set(v.FieldName, "")
				isCurrent := false
				if urlParams == nil || (urlParams != nil && urlParams[v.FieldName] == "") {
					isCurrent = true
				}
				// 需要插入 全部 标签
				filterItems = append(filterItems, response.FilterItem{
					Label:     allText,
					Link:      urlPatten + tmpParams.Encode(),
					IsCurrent: isCurrent,
				})
			}
			for _, val := range values {
				tmpParams, _ := url.ParseQuery(newQuery)
				tmpParams.Set(v.FieldName, val)
				isCurrent := false
				if urlParams != nil && urlParams[v.FieldName] == val {
					isCurrent = true
				}
				filterItems = append(filterItems, response.FilterItem{
					Label:     val,
					Link:      urlPatten + tmpParams.Encode(),
					IsCurrent: isCurrent,
				})
			}
			filterGroups = append(filterGroups, response.FilterGroup{
				Name:      v.Name,
				FieldName: v.FieldName,
				Items:     filterItems,
			})
		}
	}

	if len(filterGroups) == 0 {
		return nil
	}

	ctx.Private[node.name] = filterGroups

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagArchiveFiltersParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagArchiveFiltersNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("archiveFilters-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed archiveFilters-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endarchiveFilters")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endarchiveFilters' must equal to 'archiveFilters'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endarchiveFilters'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
