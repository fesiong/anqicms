package tags

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/flosch/pongo2/v6"
	"gorm.io/gorm"
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
	if id > 0 {
		tagDetail, _ = currentSite.GetTagById(id)
	} else if token != "" {
		tagDetail, _ = currentSite.GetTagByUrlToken(token)
	} else if title != "" {
		tagDetail, _ = currentSite.GetTagByTitle(title)
	}

	fieldName := ""
	inputName := ""
	if args["name"] != nil {
		inputName = args["name"].String()
		fieldName = library.Case2Camel(inputName)
	}

	var content interface{}

	if tagDetail != nil {
		tagDetail.Link = currentSite.GetUrl("tag", tagDetail, 0)
		tagDetail.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.GetDefaultThumb(int(tagDetail.Id)))
		v := reflect.ValueOf(*tagDetail)

		f := v.FieldByName(fieldName)
		if f.IsValid() {
			content = f.Interface()
		}
		if tagDetail.SeoTitle == "" && fieldName == "SeoTitle" {
			content = tagDetail.Title
		}
		tmpKey := "tagContent" + fmt.Sprintf("%d", tagDetail.Id)
		var tagContent *model.TagContent
		if ctx.Public[tmpKey] != nil {
			tagContent, _ = ctx.Public[tmpKey].(*model.TagContent)
		}
		if tagContent == nil {
			tagContent, _ = currentSite.GetTagContentById(tagDetail.Id)
		}
		if fieldName == "Content" && tagContent != nil {
			content = tagContent.Content
			// convert markdown to html
			if render {
				content = library.MarkdownToHTML(tagContent.Content, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
			}
		}
		if tagContent != nil && tagContent.Extra != nil {
			fields := currentSite.GetTagFields()
			if len(fields) > 0 {
				for _, field := range fields {
					if (tagContent.Extra[field.FieldName] == nil || tagContent.Extra[field.FieldName] == "" || tagContent.Extra[field.FieldName] == 0) &&
						field.Type != config.CustomFieldTypeRadio &&
						field.Type != config.CustomFieldTypeCheckbox &&
						field.Type != config.CustomFieldTypeSelect {
						// default
						tagContent.Extra[field.FieldName] = field.Content
					}
					if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
						tagContent.Extra[field.FieldName] != nil {
						value, ok2 := tagContent.Extra[field.FieldName].(string)
						if ok2 {
							if field.Type == config.CustomFieldTypeEditor && render {
								value = library.MarkdownToHTML(value, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
							}
							tagContent.Extra[field.FieldName] = currentSite.ReplaceContentUrl(value, true)
						}
					}
					if field.Type == config.CustomFieldTypeImages && tagContent.Extra[field.FieldName] != nil {
						if val, ok := tagContent.Extra[field.FieldName].([]interface{}); ok {
							for j, v2 := range val {
								v2s, _ := v2.(string)
								val[j] = currentSite.ReplaceContentUrl(v2s, true)
							}
							tagContent.Extra[field.FieldName] = val
						}
					} else if field.Type == config.CustomFieldTypeTexts && tagContent.Extra[field.FieldName] != nil {
						var texts []model.CustomFieldTexts
						_ = json.Unmarshal([]byte(fmt.Sprint(tagContent.Extra[field.FieldName])), &texts)
						tagContent.Extra[field.FieldName] = texts
					} else if field.Type == config.CustomFieldTypeArchive && tagContent.Extra[field.FieldName] != nil {
						// 列表
						var arcIds []int64
						buf, _ := json.Marshal(tagContent.Extra[field.FieldName])
						_ = json.Unmarshal(buf, &arcIds)
						if len(arcIds) == 0 && field.Content != "" {
							value, _ := strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
							if value > 0 {
								arcIds = append(arcIds, value)
							}
						}
						if len(arcIds) > 0 {
							archives, _, _ := currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
								return tx.Where("archives.`id` IN (?)", arcIds)
							}, "archives.id ASC", 0, len(arcIds))
							tagContent.Extra[field.FieldName] = archives
						} else {
							tagContent.Extra[field.FieldName] = nil
						}
					} else if field.Type == config.CustomFieldTypeCategory {
						value, err := strconv.ParseInt(fmt.Sprint(tagContent.Extra[field.FieldName]), 10, 64)
						if err != nil && field.Content != "" {
							value, _ = strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
						}
						if value > 0 {
							tagContent.Extra[field.FieldName] = currentSite.GetCategoryFromCache(uint(value))
						} else {
							tagContent.Extra[field.FieldName] = nil
						}
					}
				}
				if fieldName == "Extra" {
					var extras = make([]model.CustomField, 0, len(fields))
					for _, field := range fields {
						extras = append(extras, model.CustomField{
							Name:      field.Name,
							Value:     tagContent.Extra[field.FieldName],
							Default:   field.Content,
							Type:      field.Type,
							FieldName: field.FieldName,
						})
					}
					content = extras
				}
			}
			if item, ok := tagContent.Extra[inputName]; ok {
				content = item
			}
		}

	}

	// output
	if node.name == "" {
		writer.WriteString(fmt.Sprintf("%v", content))
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
