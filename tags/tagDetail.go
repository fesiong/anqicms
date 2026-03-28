package tags

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagTagDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagTagDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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
	title := ""
	if args["title"] != nil {
		title = args["title"].String()
	}

	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if args["render"] != nil {
		render = args["render"].Bool()
	}

	tagDetail, _ := ctx.Public["tag"].(*model.Tag)
	if args["id"] != nil {
		id = uint(args["id"].Integer())
	}
	// 使用上下文缓存避免重复查找
	cacheKey := "tag_detail_cache_"
	if id > 0 {
		cacheKey += fmt.Sprintf("id_%d", id)
	} else if token != "" {
		cacheKey += "token_" + token
	} else if title != "" {
		cacheKey += "title_" + title
	} else {
		cacheKey += "default"
	}

	if cached, ok := ctx.Private[cacheKey].(*model.Tag); ok {
		tagDetail = cached
	} else {
		if id > 0 {
			tagDetail, _ = currentSite.GetTagById(id)
		} else if token != "" {
			tagDetail, _ = currentSite.GetTagByUrlToken(token)
		} else if title != "" {
			tagDetail, _ = currentSite.GetTagByTitle(title)
		}

		if tagDetail != nil {
			tagDetail.Link = currentSite.GetUrl("tag", tagDetail, 0)
			tagDetail.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.GetDefaultThumb(int(tagDetail.Id)))
			// 存入缓存
			ctx.Private[cacheKey] = tagDetail
		}
	}

	if tagDetail == nil {
		return nil
	}

	fieldName := ""
	inputName := ""
	if args["name"] != nil {
		inputName = args["name"].String()
		fieldName = library.Case2Camel(inputName)
		if fieldName == "Extra" {
			inputName = ""
		}
	}
	// 支持获取整个detail
	if fieldName == "" && node.name != "" {
		ctx.Private[node.name] = tagDetail
		return nil
	}

	var content interface{}
	// 消除反射，改用直接字段访问
	switch fieldName {
	case "Id":
		content = tagDetail.Id
	case "Title":
		content = tagDetail.Title
	case "SeoTitle":
		content = tagDetail.SeoTitle
		if tagDetail.SeoTitle == "" {
			content = tagDetail.Title
		}
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, tagDetail)
		}
	case "Keywords":
		content = tagDetail.Keywords
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, tagDetail)
		}
	case "Description":
		content = tagDetail.Description
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, tagDetail)
		}
	case "Link":
		content = tagDetail.Link
	case "Thumb":
		content = tagDetail.Thumb
	case "Logo":
		content = tagDetail.Logo
	case "CreatedTime":
		content = tagDetail.CreatedTime
	case "UpdatedTime":
		content = tagDetail.UpdatedTime
	case "Content":
		tmpKey := "tagContent" + strconv.Itoa(int(tagDetail.Id))
		var tagContent *model.TagContent
		if ctx.Public[tmpKey] != nil {
			tagContent, _ = ctx.Public[tmpKey].(*model.TagContent)
		}
		if tagContent == nil {
			tagContent, _ = currentSite.GetTagContentById(tagDetail.Id)
			if tagContent != nil {
				ctx.Public[tmpKey] = tagContent
			}
		}
		if tagContent != nil {
			content = parseContent(tagContent.Content, render, currentSite, ctx)
		}
	default:
		// 备选方案：使用反射获取
		if fieldName != "Extra" {
			v := reflect.ValueOf(*tagDetail)
			f := v.FieldByName(fieldName)
			if f.IsValid() {
				content = f.Interface()
			}
		}
		// 数据可能来自自定义字段
		tmpKey := "tagContent" + strconv.Itoa(int(tagDetail.Id))
		var tagContent *model.TagContent
		if ctx.Public[tmpKey] != nil {
			tagContent, _ = ctx.Public[tmpKey].(*model.TagContent)
		}
		if tagContent == nil {
			tagContent, _ = currentSite.GetTagContentById(tagDetail.Id)
			if tagContent != nil {
				ctx.Public[tmpKey] = tagContent
			}
		}
		if tagContent != nil && tagContent.Extra != nil {
			fields := currentSite.GetTagFields()
			if len(fields) > 0 {
				extraData := provider.ProcessExtra(tagContent.Extra, fields, currentSite, render, inputName)
				if fieldName == "Extra" {
					var extras = make([]config.CustomField, 0, len(fields))
					for _, field := range fields {
						extras = append(extras, config.CustomField{
							Name:      field.Name,
							Value:     extraData[field.FieldName],
							Default:   field.Content,
							Type:      field.Type,
							FieldName: field.FieldName,
						})
					}
					content = extras
				} else if item, ok := extraData[inputName]; ok {
					content = item
				}
			}
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

func TagTagDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagTagDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("tagDetail-tag needs a accept name.", nil)
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
		return nil, arguments.Error("Malformed tagDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
