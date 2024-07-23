package tags

import (
	"fmt"
	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"reflect"
)

type tagTdkNode struct {
	args map[string]pongo2.IEvaluator
	name string
}

func (node *tagTdkNode) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	currentSite, _ := ctx.Public["website"].(*provider.Website)
	if currentSite == nil || currentSite.DB == nil {
		return nil
	}
	args, err := parseArgs(node.args, ctx)
	if err != nil {
		return err
	}

	siteName := false
	if args["siteName"] != nil {
		siteName = args["siteName"].Bool()
	}

	fieldName := ""
	if args["name"] != nil {
		fieldName = args["name"].String()
		fieldName = library.Case2Camel(fieldName)
	}

	webInfo, ok := ctx.Public["webInfo"].(*response.WebInfo)
	if !ok {
		return nil
	}

	v := reflect.ValueOf(*webInfo)

	f := v.FieldByName(fieldName)

	content := fmt.Sprintf("%v", f)
	if fieldName == "Title" {
		var pateText string
		// 增加分页
		paginator, ok := ctx.Public["pagination"].(*pagination)
		if ok {
			// 从第二页开始，增加分页
			if paginator.CurrentPage > 1 {
				pateText = currentSite.Tr("第%d页", paginator.CurrentPage)
			}
			return nil
		}
		if len(pateText) > 0 {
			if content != "" {
				content += " - "
			}
			content += pateText
		}
		if siteName {
			if content != "" {
				content += " - "
			}
			content += currentSite.System.SiteName
		}
		if content == "" {
			// 保持标题至少是网站名称
			content = currentSite.System.SiteName
		}
	}

	// output
	if node.name == "" {
		writer.WriteString(content)
	} else {
		ctx.Private[node.name] = content
	}

	return nil
}

func TagTdkParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tagNode := &tagTdkNode{
		args: make(map[string]pongo2.IEvaluator),
	}

	nameToken := arguments.MatchType(pongo2.TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("tdk-tag needs a accept name.", nil)
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
		return nil, arguments.Error("Malformed tdk-tag arguments.", nil)
	}

	return tagNode, nil
}
