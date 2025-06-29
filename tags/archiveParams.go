package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"strconv"
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
			var extras = make(map[string]model.CustomField, len(archiveParams))
			for i := range archiveParams {
				param := *archiveParams[i]
				if (param.Value == nil || param.Value == "" || param.Value == 0) &&
					param.Type != config.CustomFieldTypeRadio &&
					param.Type != config.CustomFieldTypeCheckbox &&
					param.Type != config.CustomFieldTypeSelect {
					param.Value = param.Default
				}
				if param.FollowLevel && !archiveDetail.HasOrdered {
					continue
				}
				if param.Type == config.CustomFieldTypeEditor && render {
					param.Value = library.MarkdownToHTML(fmt.Sprintf("%v", param.Value), currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
				} else if param.Type == config.CustomFieldTypeArchive {
					// 列表
					arcIds, ok := param.Value.([]int64)
					if !ok && param.Default != "" {
						value, _ := strconv.ParseInt(fmt.Sprint(param.Default), 10, 64)
						if value > 0 {
							arcIds = append(arcIds, value)
						}
					}
					if len(arcIds) > 0 {
						arcs, _, _ := currentSite.GetArchiveList(func(tx *gorm.DB) *gorm.DB {
							return tx.Where("archives.`id` IN (?)", arcIds)
						}, "archives.id ASC", 0, len(arcIds))
						param.Value = arcs
					} else {
						param.Value = nil
					}
				} else if param.Type == config.CustomFieldTypeCategory {
					value, ok := param.Value.(int64)
					if !ok && param.Default != "" {
						value, _ = strconv.ParseInt(fmt.Sprint(param.Default), 10, 64)
					}
					if value > 0 {
						param.Value = currentSite.GetCategoryFromCache(uint(value))
					} else {
						param.Value = nil
					}
				}
				extras[i] = param
			}
			if sorted {
				var extraFields []model.CustomField
				module := currentSite.GetModuleFromCache(archiveDetail.ModuleId)
				if module != nil && len(module.Fields) > 0 {
					for _, v := range module.Fields {
						extraFields = append(extraFields, extras[v.FieldName])
					}
				}

				ctx.Private[node.name] = extraFields
			} else {
				if len(name) > 0 {
					var content interface{}
					if item, ok := extras[name]; ok {
						content = item.Value
					}
					ctx.Private[node.name] = content
				} else {
					ctx.Private[node.name] = extras
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
