package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"github.com/kataras/iris/v12/context"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"strconv"
	"strings"
)

type tagTagDataListNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagTagDataListNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	limit := 10
	offset := 0
	currentPage := 1
	order := "archives.`created_time` desc"
	if currentSite.Content.UseSort == 1 {
		order = "archives.`sort` desc, archives.`created_time` desc"
	}
	tagId := uint(0)
	listType := "list"
	moduleId := uint(0)
	if args["moduleId"] != nil {
		moduleId = uint(args["moduleId"].Integer())
	}

	if args["type"] != nil {
		listType = args["type"].String()
	}

	tagDetail, _ := ctx.Public["tag"].(*model.Tag)
	if args["tagId"] != nil {
		tagId = uint(args["tagId"].Integer())
		tagDetail, _ = currentSite.GetTagById(tagId)
	}

	if tagDetail != nil {
		urlParams, ok := ctx.Public["urlParams"].(map[string]string)
		if ok {
			currentPage, _ = strconv.Atoi(urlParams["page"])
		}
		requestParams, ok := ctx.Public["requestParams"].(*context.RequestParams)
		if ok {
			paramPage := requestParams.GetIntDefault("page", 0)
			if paramPage > 0 {
				currentPage = paramPage
			}
		}
		if currentPage < 1 {
			currentPage = 1
		}
		if args["limit"] != nil {
			limitArgs := strings.Split(args["limit"].String(), ",")
			if len(limitArgs) == 2 {
				offset, _ = strconv.Atoi(limitArgs[0])
				limit, _ = strconv.Atoi(limitArgs[1])
			} else if len(limitArgs) == 1 {
				limit, _ = strconv.Atoi(limitArgs[0])
			}
			if limit > currentSite.Content.MaxLimit {
				limit = currentSite.Content.MaxLimit
			}
			if limit < 1 {
				limit = 1
			}
		}

		if listType == "page" {
			if currentPage > 1 {
				offset = (currentPage - 1) * limit
			}
		} else {
			currentPage = 1
		}
		archives, total, _ := currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
			tx = tx.Table("`archives` as archives").
				Joins("INNER JOIN `tag_data` as t ON archives.id = t.item_id AND t.`tag_id` = ?", tagDetail.Id)
			if moduleId > 0 {
				tx = tx.Where("archives.`module_id` = ?", moduleId)
			}
			return tx
		}, order, currentPage, limit, offset)
		var archiveIds = make([]int64, 0, len(archives))
		for i := range archives {
			archiveIds = append(archiveIds, archives[i].Id)
			if len(archives[i].Password) > 0 {
				archives[i].HasPassword = true
			}
		}
		// 读取flags
		if len(archiveIds) > 0 {
			var flags []*model.ArchiveFlags
			currentSite.DB.WithContext(currentSite.Ctx()).Model(&model.ArchiveFlag{}).Where("`archive_id` IN (?)", archiveIds).Select("archive_id", "GROUP_CONCAT(`flag`) as flags").Group("archive_id").Scan(&flags)
			for i := range archives {
				for _, f := range flags {
					if f.ArchiveId == archives[i].Id {
						archives[i].Flag = f.Flags
						break
					}
				}
			}
		}
		ctx.Private[node.name] = archives
		if listType == "page" {
			// 分页
			urlPatten := currentSite.GetUrl("tag", tagDetail, -1)
			ctx.Public["pagination"] = makePagination(currentSite, total, currentPage, limit, urlPatten, 5)
		}
	}

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagTagDataListParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagTagDataListNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("tagList-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed tagList-tag arguments.", nil)
	}

	wrapper, endtagargs, err := doc.WrapUntilTag("endtagDataList")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endtagDataList' must equal to 'tagDataList'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endtagDataList'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
