package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"reflect"
	"regexp"
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
	id := uint(0)

	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	format := "2006-01-02"
	if args["format"] != nil {
		format = args["format"].String()
	}

	lazy := ""
	if args["lazy"] != nil {
		lazy = args["lazy"].String()
	}

	archiveDetail, _ := ctx.Public["archive"].(*model.Archive)

	if args["id"] != nil {
		id = uint(args["id"].Integer())
		archiveDetail = currentSite.GetArchiveByIdFromCache(id)
		if archiveDetail == nil {
			archiveDetail, _ = currentSite.GetArchiveById(id)
			if archiveDetail != nil {
				currentSite.AddArchiveCache(archiveDetail)
			}
		}
		// check has Order
		if fieldName == "HasOrdered" && archiveDetail != nil {
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
	}

	if archiveDetail != nil {
		v := reflect.ValueOf(*archiveDetail)

		f := v.FieldByName(fieldName)

		content := fmt.Sprintf("%v", f)
		if content == "" && fieldName == "SeoTitle" {
			content = archiveDetail.Title
		}

		if fieldName == "CreatedTime" || fieldName == "UpdatedTime" {
			content = time.Unix(f.Int(), 0).Format(format)
		}
		if fieldName == "Link" {
			// 当是获取链接的时候，再生成
			archiveDetail.Link = currentSite.GetUrl("archive", archiveDetail, 0)
		}
		if fieldName == "Content" {
			// if read level larger than 0, then need to check permission
			if archiveDetail.ReadLevel > 0 {
				userGroup, _ := ctx.Public["userGroup"].(*model.UserGroup)
				if userGroup == nil || userGroup.Level < archiveDetail.ReadLevel {
					content = fmt.Sprintf(currentSite.Lang("该内容需要用户等级%d以上才能阅读"), archiveDetail.ReadLevel)
				}
			} else {
				// 当读取content 的时候，再查询
				archiveData, err := currentSite.GetArchiveDataById(archiveDetail.Id)
				if err == nil {
					content = archiveData.Content
					// lazyload
					if lazy != "" {
						re, _ := regexp.Compile(`(?i)<img.*?src="(.+?)".*?>`)
						content = re.ReplaceAllStringFunc(content, func(s string) string {
							match := re.FindStringSubmatch(s)
							if len(match) < 2 {
								return s
							}
							res := fmt.Sprintf("%s\" %s=\"%s", currentSite.Content.DefaultThumb, lazy, match[1])
							s = strings.Replace(s, match[1], res, 1)
							return s
						})
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
			writer.WriteString(content)
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
