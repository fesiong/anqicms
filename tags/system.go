package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"reflect"
	"strings"
)

type tagSystemNode struct {
	name string
	args map[string]pongo2.IEvaluator
}

func (node *tagSystemNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
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

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	var content string

	// TemplateUrl 实时算出来, 它的计算方式是 /static/{TemplateName}
	if fieldName == "TemplateUrl" {
		// 多语言的调整
		mainSite := currentSite.GetMainWebsite()
		baseUrl := currentSite.System.BaseUrl
		if mainSite.MultiLanguage.Open && mainSite.Id != currentSite.Id {
			if mainSite.MultiLanguage.Type == config.MultiLangTypeDirectory {
				// 替换目录
				baseUrl = mainSite.System.BaseUrl + "/" + currentSite.System.Language
			} else if mainSite.MultiLanguage.Type == config.MultiLangTypeSame {
				baseUrl = mainSite.System.BaseUrl
			}
		}
		content = fmt.Sprintf("%s/static/%s/", strings.TrimRight(baseUrl, "/"), currentSite.System.TemplateName)
		// 如果是多站点，除了独立域名外，另外两种方式的静态资源，都采用目录形式，否则会导致加载异常
	} else if fieldName == "SiteLogo" {
		content = currentSite.System.SiteLogo
		if !strings.HasPrefix(content, "http") {
			content = currentSite.PluginStorage.StorageUrl + currentSite.System.SiteLogo
		}
	} else if currentSite.System.ExtraFields != nil {
		for i := range currentSite.System.ExtraFields {
			if currentSite.System.ExtraFields[i].Name == fieldName {
				content = currentSite.System.ExtraFields[i].Value
				break
			}
		}
	}
	if content == "" {
		v := reflect.ValueOf(*currentSite.System)
		f := v.FieldByName(fieldName)

		content = fmt.Sprintf("%v", f)
	}

	// output
	if node.name == "" {
		writer.WriteString(content)
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagSystemParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagSystemNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("system-tag needs a accept name.", nil)
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
		return nil, arguments.Error("Malformed system-tag arguments.", nil)
	}

	return tagNode, nil
}
