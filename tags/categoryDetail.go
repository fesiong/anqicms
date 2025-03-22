package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"reflect"
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
	}

	categoryDetail, _ := ctx.Public["category"].(*model.Category)
	archiveDetail, ok := ctx.Public["archive"].(*model.Archive)
	if ok && archiveDetail != nil {
		categoryDetail = currentSite.GetCategoryFromCache(archiveDetail.CategoryId)
	}

	if args["id"] != nil {
		if args["id"].String() == "parent" && categoryDetail != nil {
			id = categoryDetail.Id
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
		categoryDetail.Link = currentSite.GetUrl("category", categoryDetail, 0)
		categoryDetail.Thumb = categoryDetail.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		v := reflect.ValueOf(*categoryDetail)

		f := v.FieldByName(fieldName)
		var content interface{}
		if f.IsValid() {
			content = f.Interface()
		}
		// 支持 extra
		if categoryDetail.Extra != nil {
			module := currentSite.GetModuleFromCache(categoryDetail.ModuleId)
			if module != nil && len(module.CategoryFields) > 0 {
				for _, field := range module.CategoryFields {
					if categoryDetail.Extra[field.FieldName] == nil || categoryDetail.Extra[field.FieldName] == "" {
						// default
						categoryDetail.Extra[field.FieldName] = field.Content
					}
					if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
						categoryDetail.Extra[field.FieldName] != nil {
						value, ok2 := categoryDetail.Extra[field.FieldName].(string)
						if ok2 {
							if field.Type == config.CustomFieldTypeEditor && render {
								value = library.MarkdownToHTML(value, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
							}
							categoryDetail.Extra[field.FieldName] = currentSite.ReplaceContentUrl(value, true)
						}
					}
					if field.Type == config.CustomFieldTypeImages && categoryDetail.Extra[field.FieldName] != nil {
						if val, ok := categoryDetail.Extra[field.FieldName].([]interface{}); ok {
							for j, v2 := range val {
								v2s, _ := v2.(string)
								val[j] = currentSite.ReplaceContentUrl(v2s, true)
							}
							categoryDetail.Extra[field.FieldName] = val
						}
					}
				}
				if fieldName == "Extra" {
					var extras = make([]model.CustomField, 0, len(module.CategoryFields))
					for _, field := range module.CategoryFields {
						extras = append(extras, model.CustomField{
							Name:      field.Name,
							Value:     categoryDetail.Extra[field.FieldName],
							Default:   field.Content,
							Type:      field.Type,
							FieldName: field.FieldName,
						})
					}
					content = extras
				}
			}
		}
		if item, ok := categoryDetail.Extra[inputName]; ok {
			content = item
		}

		if categoryDetail.SeoTitle == "" && fieldName == "SeoTitle" {
			content = categoryDetail.Title
		}

		// convert markdown to html
		if fieldName == "Content" {
			var value string
			if render {
				value = library.MarkdownToHTML(categoryDetail.Content, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
			} else {
				value = categoryDetail.Content
			}
			content = currentSite.ReplaceContentUrl(value, true)
		}
		// output
		if node.name == "" {
			writer.WriteString(fmt.Sprintf("%v", content))
		} else {
			if fieldName == "Images" {
				ctx.Private[node.name] = categoryDetail.Images
			} else {
				ctx.Private[node.name] = content
			}
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
