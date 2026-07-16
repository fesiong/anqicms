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

type tagPlaceDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagPlaceDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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
	place, _ := ctx.Public["place"].(*model.Place)
	if args["id"] != nil {
		id = uint(args["id"].Integer())
	}
	if id > 0 {
		place = currentSite.GetPlaceFromCache(id)
	} else if token != "" {
		place = currentSite.GetPlaceFromCacheByToken(token)
	}

	fieldName := ""
	inputName := ""
	if args["name"] != nil {
		inputName = args["name"].String()
		fieldName = library.Case2Camel(inputName)
	}
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if args["render"] != nil {
		render = args["render"].Bool()
	}

	var content interface{}

	if place != nil {
		place.Link = currentSite.GetUrl("place", place, 0)
		// 支持获取整个detail
		if fieldName == "" && node.name != "" {
			ctx.Private[node.name] = place
			return nil
		}

		// 消除反射，改用直接字段访问
		switch fieldName {
		case "Id":
			content = place.Id
		case "Title":
			content = place.Title
		case "SeoTitle":
			content = place.SeoTitle
			if place.SeoTitle == "" {
				content = place.Title
			}
			if strings.Contains(content.(string), "{") {
				content = parseTdkParams(content.(string), currentSite, ctx, place)
			}
		case "Keywords":
			content = place.Keywords
			if strings.Contains(content.(string), "{") {
				content = parseTdkParams(content.(string), currentSite, ctx, place)
			}
		case "Description":
			content = place.Description
			if strings.Contains(content.(string), "{") {
				content = parseTdkParams(content.(string), currentSite, ctx, place)
			}
		case "Content":
			content = parseContent(place.Content, render, currentSite, ctx)
		case "Link":
			place.Link = currentSite.GetUrl(provider.PatternPlace, place, 0)
			content = place.Link
		case "Thumb":
			content = place.Thumb
		case "Logo":
			content = place.Logo
		case "Images":
			content = place.Images
		case "ParentId":
			content = place.ParentId
		case "CreatedTime":
			content = place.CreatedTime
		case "UpdatedTime":
			content = place.UpdatedTime
		case "Latitude":
			content = place.Latitude
		case "Longitude":
			content = place.Longitude
		case "Timezone":
			content = place.Timezone
		case "TopId":
			content = currentSite.GetTopPlaceId(place.Id)
		default:
			// 备选方案：非核心字段使用反射
			if fieldName != "Extra" {
				v := reflect.ValueOf(*place)
				f := v.FieldByName(fieldName)
				if f.IsValid() {
					content = f.Interface()
				}
			}
			// 支持 extra
			if content == nil && place.Extra != nil {
				placeFields := currentSite.PluginPlace.Fields
				if len(placeFields) > 0 {
					extraData := provider.ProcessExtra(place.Extra, placeFields, currentSite, render, inputName)
					if fieldName == "Extra" {
						var extras = make([]config.CustomField, 0, len(placeFields))
						for _, field := range placeFields {
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
	}

	// output
	if node.name == "" {
		writer.WriteString(fmt.Sprint(content))
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagPlaceDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagPlaceDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("placeDetail-tag needs a accept name.", nil)
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
		return nil, arguments.Error("Malformed placeDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
