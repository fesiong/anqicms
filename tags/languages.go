package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"strings"
)

type tagLanguagesNode struct {
	name    string
	args    map[string]pongo2.IEvaluator
	wrapper *pongo2.NodeWrapper
}

func (node *tagLanguagesNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}

	// 获取当前的链接
	mainId := currentSite.ParentId
	if mainId == 0 {
		mainId = currentSite.Id
	}

	mainSite := provider.GetWebsite(mainId)
	if mainSite.MultiLanguage.Open == false {
		return nil
	}

	languageSites := currentSite.GetMultiLangSites(mainId)
	// 需要过滤掉不能用的站点
	for i := 0; i < len(languageSites); i++ {
		if languageSites[i].Status != 1 {
			languageSites = append(languageSites[:i], languageSites[i+1:]...)
		}
	}
	// 检查当前是在哪个页面下
	webInfo, ok := ctx.Public["webInfo"].(*response.WebInfo)
	if !ok {
		return nil
	}

	for i := range languageSites {
		tmpSite := provider.GetWebsite(languageSites[i].Id)
		var link string
		currentPage, _ := ctx.Public["currentPage"].(int)
		// archive
		if item, ok := ctx.Public["archive"].(*model.Archive); ok {
			link = tmpSite.GetUrl("archive", item, 0)
		}
		// category
		if item, ok := ctx.Public["category"].(*model.Category); ok {
			link = tmpSite.GetUrl("category", item, currentPage)
		}
		// tag
		if item, ok := ctx.Public["tag"].(*model.Tag); ok {
			link = tmpSite.GetUrl("tag", item, currentPage)
		}
		// page
		if item, ok := ctx.Public["page"].(*model.Category); ok {
			link = tmpSite.GetUrl("page", item, 0)
		}
		// archiveIndex
		if webInfo.PageName == "archiveIndex" {
			if item, ok := ctx.Public["module"].(*model.Module); ok {
				link = tmpSite.GetUrl("archiveIndex", item, currentPage)
			}
		}
		// other
		if link == "" && mainSite.MultiLanguage.Type != config.MultiLangTypeSame {
			link = tmpSite.GetUrl("", nil, 0)
		}
		// 如果是同链接，则是一个跳转链接
		if mainSite.MultiLanguage.Type == config.MultiLangTypeSame {
			if strings.Contains(link, "?") {
				link = link + "&lang=" + tmpSite.System.Language
			} else {
				link += "?lang=" + tmpSite.System.Language
			}
		}

		languageSites[i].Link = link
		languageSites[i].LanguageName = library.GetLanguageName(tmpSite.System.Language)
		if languageSites[i].LanguageName == "" {
			languageSites[i].LanguageName = languageSites[i].Name
		}
	}

	ctx.Private[node.name] = languageSites
	//execute
	node.wrapper.Execute(ctx, writer)

	return nil
}

func TagLanguagesParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagLanguagesNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("languages-tag needs a accept name.", nil)
	}

	tagNode.name = nameToken.Val

	// After having parsed the name we're gonna parse the with options
	args, err := parseWith(arguments)
	if err != nil {
		return nil, err
	}
	tagNode.args = args

	for arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed languages-tag arguments.", nil)
	}
	wrapper, endtagargs, err := doc.WrapUntilTag("endLanguages")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(pongo2.TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endLanguages' must equal to 'languages'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endLanguages'.", nil)
		}
	}
	tagNode.wrapper = wrapper

	return tagNode, nil
}