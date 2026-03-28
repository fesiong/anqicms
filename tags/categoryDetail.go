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

type tagCategoryDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagCategoryDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if args["render"] != nil {
		render = args["render"].Bool()
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

	categoryDetail, _ := ctx.Public["category"].(*model.Category)
	archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
	if ok && archiveDetail != nil {
		categoryDetail = currentSite.GetCategoryFromCache(archiveDetail.CategoryId)
	}

	if args["id"] != nil {
		if args["id"].String() == "parent" && categoryDetail != nil {
			id = categoryDetail.ParentId
		} else {
			id = uint(args["id"].Integer())
		}
	}
	if id > 0 {
		categoryDetail = currentSite.GetCategoryFromCache(id)
	} else if token != "" {
		categoryDetail = currentSite.GetCategoryFromCacheByToken(token)
	}

	if categoryDetail != nil {
		// 支持获取整个detail
		if fieldName == "" && node.name != "" {
			categoryDetail.Link = currentSite.GetUrl("category", categoryDetail, 0)
			ctx.Private[node.name] = categoryDetail
			return nil
		}

		var content interface{}
		// 消除反射，改用直接字段访问
		switch fieldName {
		case "Id":
			content = categoryDetail.Id
		case "Title":
			content = categoryDetail.Title
		case "SeoTitle":
			content = categoryDetail.SeoTitle
			if categoryDetail.SeoTitle == "" {
				content = categoryDetail.Title
			}
			if strings.Contains(content.(string), "{") {
				content = parseTdkParams(content.(string), currentSite, ctx, categoryDetail)
			}
		case "Keywords":
			content = categoryDetail.Keywords
			if strings.Contains(content.(string), "{") {
				content = parseTdkParams(content.(string), currentSite, ctx, categoryDetail)
			}
		case "Description":
			content = categoryDetail.Description
			if strings.Contains(content.(string), "{") {
				content = parseTdkParams(content.(string), currentSite, ctx, categoryDetail)
			}
		case "Content":
			content = parseContent(categoryDetail.Content, render, currentSite, ctx)
		case "Link":
			categoryDetail.Link = currentSite.GetUrl("category", categoryDetail, 0)
			content = categoryDetail.Link
		case "Thumb":
			content = categoryDetail.Thumb
		case "Logo":
			content = categoryDetail.Logo
		case "Images":
			content = categoryDetail.Images
		case "ParentId":
			content = categoryDetail.ParentId
		case "ModuleId":
			content = categoryDetail.ModuleId
		case "CreatedTime":
			content = categoryDetail.CreatedTime
		case "UpdatedTime":
			content = categoryDetail.UpdatedTime
		case "ArchiveCount":
			content = categoryDetail.ArchiveCount
		case "TopId":
			content = currentSite.GetTopCategoryId(categoryDetail.Id)
		default:
			// 备选方案：非核心字段使用反射
			if fieldName != "Extra" {
				v := reflect.ValueOf(*categoryDetail)
				f := v.FieldByName(fieldName)
				if f.IsValid() {
					content = f.Interface()
				}
			}
			// 支持 extra
			if content == nil && categoryDetail.Extra != nil {
				module := currentSite.GetModuleFromCache(categoryDetail.ModuleId)
				if module != nil && len(module.CategoryFields) > 0 {
					extraData := provider.ProcessExtra(categoryDetail.Extra, module.CategoryFields, currentSite, render, inputName)
					if fieldName == "Extra" {
						var extras = make([]config.CustomField, 0, len(module.CategoryFields))
						for _, field := range module.CategoryFields {
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
	}

	return nil
}

func TagCategoryDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagCategoryDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("categoryDetail-tag needs a accept name.", nil)
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
		return nil, arguments.Error("Malformed categoryDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
