package tags

import (
	"fmt"
	"hash/crc32"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

type tagArchiveDetailNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagArchiveDetailNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := int64(0)
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

	fieldName := ""
	inputName := ""
	if args["name"] != nil {
		inputName = args["name"].String()
		fieldName = library.Case2Camel(inputName)
	}

	format := "2006-01-02"
	if args["format"] != nil {
		format = args["format"].String()
	}

	lazy := ""
	if args["lazy"] != nil {
		lazy = args["lazy"].String()
	}
	// 只有content字段有效
	render := currentSite.Content.Editor == "markdown"
	if args["render"] != nil {
		render = args["render"].Bool()
	}

	userId, _ := ctx.Public["userId"].(uint)

	archiveDetail, _ := ctx.Public["archive"].(*model.Archive)
	if args["id"] != nil {
		id = int64(args["id"].Integer())
	}

	// 使用上下文缓存避免重复查找
	cacheKey := "archive_detail_cache_"
	if id > 0 {
		cacheKey += fmt.Sprintf("id_%d", id)
	} else if token != "" {
		cacheKey += "token_" + token
	} else {
		cacheKey += "default"
	}

	if cached, ok := ctx.Private[cacheKey].(*model.Archive); ok {
		archiveDetail = cached
	} else {
		if id > 0 {
			if archiveDetail == nil || archiveDetail.Id != id {
				archiveDetail = currentSite.GetArchiveByIdFromCache(id)
			}
		} else if token != "" {
			archiveDetail, _ = currentSite.GetArchiveByUrlToken(token)
		}

		if archiveDetail != nil {
			// 预处理
			if len(archiveDetail.Password) > 0 {
				archiveDetail.HasPassword = true
				urlParams, ok := ctx.Public["urlParams"].(map[string]string)
				if ok {
					password := urlParams["password"]
					if !library.IsMd5(password) {
						password = library.Md5(password)
					}
					if password == library.Md5(archiveDetail.Password) {
						archiveDetail.PasswordValid = true
					}
				}
			}
			// 存入缓存
			ctx.Private[cacheKey] = archiveDetail
		}
	}

	if archiveDetail == nil {
		return nil
	}

	// 支持获取整个detail
	if fieldName == "" && node.name != "" {
		if archiveDetail.Link == "" {
			archiveDetail.Link = currentSite.GetUrl("archive", archiveDetail, 0)
		}
		ctx.Private[node.name] = archiveDetail
		return nil
	}

	var content interface{}
	// 消除反射，改用直接字段访问
	switch fieldName {
	case "Id":
		content = archiveDetail.Id
	case "Title":
		content = archiveDetail.Title
	case "SeoTitle":
		content = archiveDetail.SeoTitle
		if archiveDetail.SeoTitle == "" {
			content = archiveDetail.Title
		}
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, archiveDetail)
		}
	case "Keywords":
		content = archiveDetail.Keywords
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, archiveDetail)
		}
	case "Description":
		content = archiveDetail.Description
		if strings.Contains(content.(string), "{") {
			content = parseTdkParams(content.(string), currentSite, ctx, archiveDetail)
		}
	case "Link":
		if archiveDetail.Link == "" {
			archiveDetail.Link = currentSite.GetUrl("archive", archiveDetail, 0)
		}
		content = archiveDetail.Link
	case "CreatedTime":
		content = time.Unix(archiveDetail.CreatedTime, 0).Format(format)
	case "UpdatedTime":
		content = time.Unix(archiveDetail.UpdatedTime, 0).Format(format)
	case "Flag", "Flags":
		if archiveDetail.Flag == "" {
			archiveDetail.Flag = currentSite.GetArchiveFlags(archiveDetail.Id)
		}
		content = archiveDetail.Flag
	case "IsFavorite":
		if userId > 0 && !archiveDetail.IsFavorite {
			exist := currentSite.CheckFavorites(int64(userId), []int64{archiveDetail.Id})
			if len(exist) > 0 {
				archiveDetail.IsFavorite = true
			}
		}
		content = archiveDetail.IsFavorite
	case "HasOrdered":
		if !archiveDetail.HasOrdered {
			currUserId := uint(0)
			userInfo, ok := ctx.Public["userInfo"].(*model.User)
			if ok && userInfo.Id > 0 {
				currUserId = userInfo.Id
				discount := currentSite.GetUserDiscount(userInfo.Id, userInfo)
				if discount > 0 {
					archiveDetail.FavorablePrice = archiveDetail.Price * discount / 100
				}
			}
			userGroup, _ := ctx.Public["userGroup"].(*model.UserGroup)
			archiveDetail = currentSite.CheckArchiveHasOrder(currUserId, archiveDetail, userGroup)
		}
		content = archiveDetail.HasOrdered
	case "PasswordValid":
		content = archiveDetail.PasswordValid
	case "Content", "ContentTitles":
		archiveData, err := currentSite.GetArchiveDataById(archiveDetail.Id)
		if err == nil {
			tmpContent := archiveData.Content
			if render {
				tmpContent = library.MarkdownToHTML(archiveData.Content, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
			}
			if fieldName == "ContentTitles" {
				ctx.Private["showContentTitle"] = true
				showType := "list"
				if args["showType"] != nil {
					showType = args["showType"].String()
				}
				content, _ = library.ParseContentTitles(tmpContent, showType)
			} else {
				if currentSite.PluginAnchor.ReplaceWay == 2 {
					tmpContent, _ = currentSite.ReplaceContentText(nil, tmpContent, archiveDetail.Link)
				}
				var showContentTitle bool
				tmpContent, showContentTitle = currentSite.RenderTemplateMacro(tmpContent, ctx.Private)
				if showContentTitle {
					ctx.Private["showContentTitle"] = showContentTitle
				}
				if isShow, ok := ctx.Private["showContentTitle"]; ok && isShow == true {
					_, tmpContent = library.ParseContentTitles(tmpContent, "list")
				}
				if lazy != "" {
					re, _ := regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
					tmpContent = re.ReplaceAllStringFunc(tmpContent, func(s string) string {
						match := re.FindStringSubmatch(s)
						if len(match) < 2 {
							return s
						}
						res := fmt.Sprintf("%s\" %s=\"%s", currentSite.GetDefaultThumb(int(archiveDetail.Id)), lazy, match[1])
						s = strings.Replace(s, match[1], res, 1)
						return s
					})
				}
				// 干扰码逻辑
				if currentSite.PluginInterference.Open && currentSite.PluginInterference.Mode == config.InterferenceModeText {
					// ... (干扰码逻辑) ...
					// 这里保留原逻辑
					webInfo, ok := ctx.Public["webInfo"].(*response.WebInfo)
					if ok {
						tmpContent = applyInterference(tmpContent, webInfo.CanonicalUrl)
					}
				}
				tmpContent = currentSite.ReplaceContentUrl(tmpContent, true)
				content = tmpContent
			}
		}
	case "Category":
		category := currentSite.GetCategoryFromCache(archiveDetail.CategoryId)
		if category != nil {
			category.Link = currentSite.GetUrl("category", category, 0)
		}
		content = category
	case "Images":
		content = archiveDetail.Images
	default:
		// 备选方案：使用反射获取
		if fieldName != "Extra" {
			v := reflect.ValueOf(*archiveDetail)
			f := v.FieldByName(fieldName)
			if f.IsValid() {
				content = f.Interface()
			}
		}
		if content == nil {
			// 数据可能来自自定义字段
			archiveParams := currentSite.GetArchiveExtra(archiveDetail.ModuleId, archiveDetail.Id, true)
			if len(archiveParams) > 0 {
				if fieldName == "Extra" {
					var extras = make([]config.CustomField, 0, len(archiveParams))
					for _, field := range archiveParams {
						extras = append(extras, config.CustomField{
							Name:      field.Name,
							Value:     field.Value,
							Default:   field.Content,
							Type:      field.Type,
							FieldName: field.FieldName,
						})
					}
					content = extras
				} else if item, ok := archiveParams[inputName]; ok {
					if item.FollowLevel && !archiveDetail.HasOrdered {
						content = ""
					} else {
						content = item.Value
						if (content == nil || content == "" || content == 0) &&
							item.Type != config.CustomFieldTypeRadio &&
							item.Type != config.CustomFieldTypeCheckbox &&
							item.Type != config.CustomFieldTypeSelect {
							content = item.Default
						}
						if item.Type == config.CustomFieldTypeEditor && render {
							content = library.MarkdownToHTML(fmt.Sprint(content), currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
						} else if item.Type == config.CustomFieldTypeArchive {
							// 列表处理
							arcIds, _ := content.([]int64)
							if len(arcIds) == 0 && item.Default != "" {
								value, _ := strconv.ParseInt(fmt.Sprint(item.Default), 10, 64)
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
						} else if item.Type == config.CustomFieldTypeCategory {
							value, ok := content.(int64)
							if !ok && item.Default != "" {
								value, _ = strconv.ParseInt(fmt.Sprint(item.Default), 10, 64)
							}
							if value > 0 {
								content = currentSite.GetCategoryFromCache(uint(value))
							} else {
								content = nil
							}
						}
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

func applyInterference(tmpContent string, canonicalUrl string) string {
	var classes = make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		tmpClass := library.DecimalToLetter(int64(crc32.ChecksumIEEE([]byte(canonicalUrl + strconv.Itoa(i)))))
		classes = append(classes, tmpClass)
	}
	// 给随机位置添加隐藏标签
	crcNum := int(crc32.ChecksumIEEE([]byte(canonicalUrl)))
	first := 2
	var tmpData []string
	var tmpRune = []rune(tmpContent)
	var start = 0
	for i := 0; i < len(tmpRune); i++ {
		if tmpRune[i] == '>' {
			tmpData = append(tmpData, string(tmpRune[start:i+1]))
			start = i + 1
			num := crcNum%10*first/2 + 2
			first = crcNum % 10
			crcNum = crcNum / 10
			j := i + 1
			for ; j < len(tmpRune)-1; j++ {
				if tmpRune[j] == '<' {
					sepLen := j - i - 1
					if sepLen > num {
						tmpData = append(tmpData, string(tmpRune[i+1:i+1+num]))
						if j%2 == 0 {
							var addText []rune
							for k := 0; k < sepLen/2; k++ {
								addText = append(addText, tmpRune[j-k-1])
								if k > first+1 {
									break
								}
							}
							tmpData = append(tmpData, "<span class=\""+classes[i%5]+"\">"+string(addText)+"</span>")
							tmpData = append(tmpData, string(tmpRune[i+1+num:j]))
						} else {
							var addText []rune
							for k := num; k < sepLen; k++ {
								addText = append(addText, tmpRune[i+1+k])
								if k > num+first+1 {
									break
								}
							}
							tmpData = append(tmpData, "<span class=\""+classes[i%5+5]+"\">"+string(addText)+"</span>")
							tmpData = append(tmpData, string(tmpRune[i+1+num+len(addText):j]))
						}
					} else {
						tmpData = append(tmpData, string(tmpRune[i+1:j]))
					}
					start = j
					break
				}
			}
			i = j
			continue
		}

		if crcNum == 0 || i == len(tmpRune)-1 {
			tmpData = append(tmpData, string(tmpRune[start:]))
			break
		}
	}
	return strings.Join(tmpData, "")
}

func TagArchiveDetailParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagArchiveDetailNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("archiveDetail-tag needs a archive field name.", nil)
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
		return nil, arguments.Error("Malformed archiveDetail-tag arguments.", nil)
	}

	return tagNode, nil
}
