package tags

import (
	"encoding/json"
	"fmt"
	"github.com/flosch/pongo2/v6"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"strconv"
)

type tagDiyNode struct {
	name string
	args map[string]pongo2.IEvaluator
}

func (node *tagDiyNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	render := currentSite.Content.Editor == "markdown"
	if args["render"] != nil {
		render = args["render"].Bool()
	}

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		// fieldName = library.Case2Camel(fieldName)
	}

	var content any

	fields := currentSite.GetDiyFieldSetting()
	for _, field := range fields {
		if field.Name == fieldName {
			content = field.Value
			if (field.Value == nil || field.Value == "" || field.Value == 0) &&
				field.Type != config.CustomFieldTypeRadio &&
				field.Type != config.CustomFieldTypeCheckbox &&
				field.Type != config.CustomFieldTypeSelect && field.Content != "" {
				content = field.Content
			}
			if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
				content != nil {
				value, ok2 := content.(string)
				if ok2 {
					if field.Type == config.CustomFieldTypeEditor && render {
						value = library.MarkdownToHTML(value, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
					}
					content = currentSite.ReplaceContentUrl(value, true)
				}
			} else if field.Type == config.CustomFieldTypeImages && content != nil {
				if val, ok := content.([]interface{}); ok {
					for j, v2 := range val {
						v2s, _ := v2.(string)
						val[j] = currentSite.ReplaceContentUrl(v2s, true)
					}
					content = val
				}
			} else if field.Type == config.CustomFieldTypeTexts && content != nil {
				var texts []model.CustomFieldTexts
				buf, _ := json.Marshal(content)
				_ = json.Unmarshal(buf, &texts)
				content = texts
			} else if field.Type == config.CustomFieldTypeArchive && content != nil {
				// 列表
				var arcIds []int64
				buf, _ := json.Marshal(content)
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
					content = archives
				} else {
					content = nil
				}
			} else if field.Type == config.CustomFieldTypeCategory {
				value, err := strconv.ParseInt(fmt.Sprint(content), 10, 64)
				if err != nil && field.Content != "" {
					value, _ = strconv.ParseInt(fmt.Sprint(field.Content), 10, 64)
				}
				if value > 0 {
					content = currentSite.GetCategoryFromCache(uint(value))
				} else {
					content = nil
				}
			}
			break
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

func TagDiyParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagDiyNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("diy-tag needs a accept name.", nil)
	}

	if nameToken.Val == "with" {
		//with 需要退回
		arguments.ConsumeN(-1)
	} else {
		tagNode.name = nameToken.Val
	}

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed diy-tag arguments.", nil)
	}

	return tagNode, nil
}
