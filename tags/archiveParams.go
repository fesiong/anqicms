package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

type tagArchiveParamsNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagArchiveParamsNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}
	id := int64(0)

	if args["site_id"] != nil {
		args["siteId"] = args["site_id"]
	}
	if args["siteId"] != nil {
		siteId := args["siteId"].Integer()
		currentSite = provider.GetWebsite(uint(siteId))
	}

	sorted := true
	if args["sorted"] != nil {
		sorted = args["sorted"].Bool()
	}
	name := ""
	if args["name"] != nil {
		name = args["name"].String()
		if len(name) > 0 {
			sorted = false
		}
	}
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
					// if read level larger than 0, then need to check permission
					userId := uint(0)
					userInfo, ok := ctx.Public["userInfo"].(*model.User)
					if ok && userInfo.Id > 0 {
						userId = userInfo.Id
					}
					userGroup, _ := ctx.Public["userGroup"].(*model.UserGroup)
					archiveDetail = currentSite.CheckArchiveHasOrder(userId, archiveDetail, userGroup)

					currentSite.AddArchiveCache(archiveDetail)
				}
			}
		}
	}

	if archiveDetail != nil {
		archiveParams := currentSite.GetArchiveExtra(archiveDetail.ModuleId, archiveDetail.Id, true)
		if len(archiveParams) > 0 {
			for i := range archiveParams {
				if archiveParams[i].Value == nil || archiveParams[i].Value == "" {
					archiveParams[i].Value = archiveParams[i].Default
				}
				if archiveParams[i].FollowLevel && !archiveDetail.HasOrdered {
					delete(archiveParams, i)
					continue
				}
				if archiveParams[i].Type == config.CustomFieldTypeEditor && render {
					archiveParams[i].Value = library.MarkdownToHTML(fmt.Sprintf("%v", archiveParams[i].Value), currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
				}
			}
			if sorted {
				var extraFields []*model.CustomField
				module := currentSite.GetModuleFromCache(archiveDetail.ModuleId)
				if module != nil && len(module.Fields) > 0 {
					for _, v := range module.Fields {
						extraFields = append(extraFields, archiveParams[v.FieldName])
					}
				}

				ctx.Private[node.name] = extraFields
			} else {
				if len(name) > 0 {
					var content interface{}
					if item, ok := archiveParams[name]; ok {
						content = item.Value
					}
					ctx.Private[node.name] = content
				} else {
					ctx.Private[node.name] = archiveParams
				}
			}
		}
	}

	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagArchiveParamsParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagArchiveParamsNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("archiveParams-tag needs a accept name.", nil)
	}
	tagNode.name = nameToken.Val

	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed archiveParams-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endarchiveParams")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endarchiveParams' must equal to 'archiveParams'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endarchiveParams'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}
