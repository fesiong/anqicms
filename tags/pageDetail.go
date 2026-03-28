package tags

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagPageDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagPageDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := uint(0)
	token := ""
	if args["token"] != nil {
		token = args["token"].String()
	}

	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}

	if args["id"] != nil {
		id = uint(args["id"].Integer())
	}
	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if args["render"] != nil {
		render = args["render"].Bool()
	}

	pageDetail, _ := ctx.Public["page"].(*model.Category)
	// 使用上下文缓存避免重复查找
	cacheKey := "page_detail_cache_"
	if id > 0 {
		cacheKey += fmt.Sprintf("id_%d", id)
	} else if token != "" {
		cacheKey += "token_" + token
	} else {
		cacheKey += "default"
	}

	if cached, ok := ctx.Private[cacheKey].(*model.Category); ok {
		pageDetail = cached
	} else {
		if id > 0 {
			pageDetail = currentSite.GetCategoryFromCache(id)
		} else if token != "" {
			pageDetail = currentSite.GetCategoryFromCacheByToken(token)
		}
		if pageDetail != nil {
			if pageDetail.Type != config.CategoryTypePage {
				return nil
			}
			// 存入缓存
			ctx.Private[cacheKey] = pageDetail
		}
	}

	if pageDetail == nil {
		return nil
	}

	// 支持获取整个detail
	if fieldName == "" && node.name != "" {
		pageDetail.Link = currentSite.GetUrl("page", pageDetail, 0)
		ctx.Private[node.name] = pageDetail
		return nil
	}

	var content interface{}
	// 消除反射，改用直接字段访问
	switch fieldName {
	case "Id":
		content = pageDetail.Id
	case "Title":
		content = pageDetail.Title
	case "SeoTitle":
		content = pageDetail.SeoTitle
		if pageDetail.SeoTitle == "" {
			content = pageDetail.Title
		}
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, pageDetail)
		}
	case "Keywords":
		content = pageDetail.Keywords
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, pageDetail)
		}
	case "Description":
		content = pageDetail.Description
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, pageDetail)
		}
	case "Content":
		content = parseContent(pageDetail.Content, render, currentSite, ctx)
	case "Link":
		pageDetail.Link = currentSite.GetUrl("page", pageDetail, 0)
		content = pageDetail.Link
	case "Thumb":
		content = pageDetail.Thumb
	case "Logo":
		content = pageDetail.Logo
	case "Images":
		content = pageDetail.Images
	case "ParentId":
		content = pageDetail.ParentId
	case "ModuleId":
		content = pageDetail.ModuleId
	case "CreatedTime":
		content = pageDetail.CreatedTime
	case "UpdatedTime":
		content = pageDetail.UpdatedTime
	case "ArchiveCount":
		content = pageDetail.ArchiveCount
	default:
		// 备选方案：极少数非核心字段使用反射
		v := reflect.ValueOf(*pageDetail)
		f := v.FieldByName(fieldName)
		if f.IsValid() {
			content = f.Interface()
		}
	}

	if node.name == "" {
		writer.WriteString(fmt.Sprint(content))
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagPageDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagPageDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("pageDetail-tag needs a page field name.", nil)
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
		return nil, arguments.Error("Malformed pageDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
