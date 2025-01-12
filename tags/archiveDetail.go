package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"hash/crc32"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
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

	archiveDetail, _ := ctx.Public["archive"].(*model.Archive)

	if args["id"] != nil {
		id = int64(args["id"].Integer())
		if archiveDetail == nil || archiveDetail.Id != id {
			archiveDetail = currentSite.GetArchiveByIdFromCache(id)
			if archiveDetail == nil {
				archiveDetail, _ = currentSite.GetArchiveById(id)
				if archiveDetail != nil {
					currentSite.AddArchiveCache(archiveDetail)
				}
			}
		}
	}
	if id > 0 {
		archiveDetail = currentSite.GetArchiveByIdFromCache(id)
		if archiveDetail == nil {
			archiveDetail, _ = currentSite.GetArchiveById(id)
			if archiveDetail != nil {
				// check has Order
				if fieldName == "HasOrdered" {
					// if read level larger than 0, then need to check permission
					userId := uint(0)
					userInfo, ok := ctx.Public["userInfo"].(*model.User)
					if ok && userInfo.Id > 0 {
						userId = userInfo.Id
						discount := currentSite.GetUserDiscount(userInfo.Id, userInfo)
						if discount > 0 {
							archiveDetail.FavorablePrice = archiveDetail.Price * discount / 100
						}
					}
					userGroup, _ := ctx.Public["userGroup"].(*model.UserGroup)
					archiveDetail = currentSite.CheckArchiveHasOrder(userId, archiveDetail, userGroup)
				}
				currentSite.AddArchiveCache(archiveDetail)
			}
		}
	} else if token != "" {
		archiveDetail, _ = currentSite.GetArchiveByUrlToken(token)
	}

	if archiveDetail != nil {
		if len(archiveDetail.Password) > 0 {
			archiveDetail.HasPassword = true
			urlParams, ok := ctx.Public["urlParams"].(map[string]string)
			if ok {
				password := urlParams["password"]
				if password == archiveDetail.Password {
					// 密码验证正确
					archiveDetail.PasswordValid = true
				}
			}
		}
		// 读取flag
		if fieldName == "Flags" || fieldName == "Flag" {
			fieldName = "Flag"
			archiveDetail.Flag = currentSite.GetArchiveFlags(archiveDetail.Id)
		}
		if fieldName == "Link" {
			// 当是获取链接的时候，再生成
			archiveDetail.Link = currentSite.GetUrl("archive", archiveDetail, 0)
		}

		v := reflect.ValueOf(*archiveDetail)

		f := v.FieldByName(fieldName)
		var content interface{}
		if f.IsValid() {
			content = f.Interface()
		} else {
			// 数据可能来自自定义字段
			archiveParams := currentSite.GetArchiveExtra(archiveDetail.ModuleId, archiveDetail.Id, true)
			if len(archiveParams) > 0 {
				for i := range archiveParams {
					if archiveParams[i].Value == nil || archiveParams[i].Value == "" {
						archiveParams[i].Value = archiveParams[i].Default
					}
				}
				if item, ok := archiveParams[inputName]; ok {
					if item.Value == nil || item.Value == "" {
						item.Value = item.Default
					}
					if item.FollowLevel && !archiveDetail.HasOrdered {
						content = ""
					} else {
						if item.Type == config.CustomFieldTypeEditor && render {
							item.Value = library.MarkdownToHTML(fmt.Sprintf("%v", item.Value), currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
						}
						content = item.Value
					}
				}
			}
		}
		if archiveDetail.SeoTitle == "" && fieldName == "SeoTitle" {
			content = archiveDetail.Title
		}

		if fieldName == "CreatedTime" {
			content = time.Unix(archiveDetail.CreatedTime, 0).Format(format)
		} else if fieldName == "UpdatedTime" {
			content = time.Unix(archiveDetail.UpdatedTime, 0).Format(format)
		}
		if fieldName == "Content" || fieldName == "ContentTitles" {
			// if read level larger than 0, then need to check permission
			if archiveDetail.ReadLevel > 0 && fieldName == "Content" {
				userGroup, _ := ctx.Public["userGroup"].(*model.UserGroup)
				if userGroup == nil || userGroup.Level < archiveDetail.ReadLevel {
					content = fmt.Sprintf(currentSite.TplTr("ThisContentRequiresUserLevelOrAboveToRead"), archiveDetail.ReadLevel)
				}
			} else {
				var tmpContent string
				// 当读取content 的时候，再查询
				archiveData, err := currentSite.GetArchiveDataById(archiveDetail.Id)
				if err == nil {
					tmpContent = archiveData.Content
					// convert markdown to html
					if render {
						tmpContent = library.MarkdownToHTML(archiveData.Content, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
					}
					if fieldName == "ContentTitles" {
						content = library.ParseContentTitles(tmpContent)
					} else {
						// lazy load
						if lazy != "" {
							re, _ := regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
							tmpContent = re.ReplaceAllStringFunc(tmpContent, func(s string) string {
								match := re.FindStringSubmatch(s)
								if len(match) < 2 {
									return s
								}
								res := fmt.Sprintf("%s\" %s=\"%s", currentSite.Content.DefaultThumb, lazy, match[1])
								s = strings.Replace(s, match[1], res, 1)
								return s
							})
						}
						if currentSite.PluginInterference.Open && currentSite.PluginInterference.Mode == config.InterferenceModeText {
							webInfo, ok := ctx.Public["webInfo"].(*response.WebInfo)
							if ok {
								var classes = make([]string, 0, 10)
								for i := 0; i < 10; i++ {
									tmpClass := library.DecimalToLetter(int64(crc32.ChecksumIEEE([]byte(webInfo.CanonicalUrl + strconv.Itoa(i)))))
									classes = append(classes, tmpClass)
								}
								// 给随机位置添加隐藏标签
								crcNum := int(crc32.ChecksumIEEE([]byte(webInfo.CanonicalUrl)))
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
								tmpContent = strings.Join(tmpData, "")
							}
						}
						tmpContent = currentSite.ReplaceContentUrl(tmpContent, true)
						content = tmpContent
					}
				}
			}
		}
		if fieldName == "Images" || fieldName == "Category" {
			content = ""
		}

		var category *model.Category
		if fieldName == "Category" {
			category = currentSite.GetCategoryFromCache(archiveDetail.CategoryId)
			if category != nil {
				category.Link = currentSite.GetUrl("category", category, 0)
			}
		}

		// output
		if node.name == "" {
			writer.WriteString(fmt.Sprintf("%v", content))
		} else {
			//不是所有都是字符串
			if fieldName == "Images" {
				ctx.Private[node.name] = archiveDetail.Images
			} else if fieldName == "Category" {
				ctx.Private[node.name] = category
			} else {
				ctx.Private[node.name] = content
			}
		}
	}

	return nil
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
