package tags

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagModuleDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagModuleDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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
	module, _ := ctx.Public["module"].(*model.Module)
	// 使用上下文缓存避免重复查找
	cacheKey := "module_detail_cache_"
	if id > 0 {
		cacheKey += fmt.Sprintf("id_%d", id)
	} else if token != "" {
		cacheKey += "token_" + token
	} else {
		cacheKey += "default"
	}

	if cached, ok := ctx.Private[cacheKey].(*model.Module); ok {
		module = cached
	} else {
		if id > 0 {
			module = currentSite.GetModuleFromCache(id)
		} else if token != "" {
			module = currentSite.GetModuleFromCacheByToken(token)
		}
		if module != nil {
			module.Link = currentSite.GetUrl("archiveIndex", module, 0)
			// 存入缓存
			ctx.Private[cacheKey] = module
		}
	}

	if module == nil {
		return nil
	}

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	// 支持获取整个detail
	if fieldName == "" && node.name != "" {
		ctx.Private[node.name] = module
		return nil
	}

	var content interface{}
	// 消除反射，改用直接字段访问
	switch fieldName {
	case "Id":
		content = module.Id
	case "TableName":
		content = module.TableName
	case "Name":
		content = module.Name
	case "UrlToken":
		content = module.UrlToken
	case "Title":
		content = module.Title
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, module)
		}
	case "SeoTitle":
		content = module.Title
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, module)
		}
	case "Keywords":
		content = module.Keywords
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, module)
		}
	case "Description":
		content = module.Description
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, module)
		}
	case "TitleName":
		content = module.TitleName
	case "Link":
		content = module.Link
	case "CreatedTime":
		content = module.CreatedTime
	case "UpdatedTime":
		content = module.UpdatedTime
	default:
		// 备选方案：使用反射获取其他字段
		v := reflect.ValueOf(*module)
		f := v.FieldByName(fieldName)
		if f.IsValid() {
			content = f.Interface()
		}
	}

	// output
	if node.name == "" {
		writer.WriteString(fmt.Sprint(content))
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagModuleDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagModuleDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("moduleDetail-tag needs a accept name.", nil)
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
		return nil, arguments.Error("Malformed moduleDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
