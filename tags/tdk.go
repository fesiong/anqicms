package tags

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
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

	var content string
	switch fieldName {
	case "Title":
		content = webInfo.Title
	case "Keywords":
		content = webInfo.Keywords
	case "Description":
		content = webInfo.Description
	case "CanonicalUrl":
		content = webInfo.CanonicalUrl
	default:
		// 备选方案：使用反射获取其他字段
		v := reflect.ValueOf(*webInfo)
		f := v.FieldByName(fieldName)
		if f.IsValid() {
			content = fmt.Sprint(f.Interface())
		}
	}

	sep := " - "
	if args["sep"] != nil {
		sep = args["sep"].String()
	} else if currentSite.Index.Sep != "" {
		sep = currentSite.Index.Sep
	}
	webInfo.Sep = sep

	if fieldName == "Title" {
		var titleText = make([]string, 0, 10)
		if len(content) > 0 {
			titleText = append(titleText, content)
		}
		showParent := false
		if args["showParent"] != nil {
			showParent = args["showParent"].Bool()
		}
		// 增加分页
		if webInfo.CurrentPage > 1 && !strings.Contains(content, "{page}") {
			// 从第二页开始，增加分页
			titleText = append(titleText, currentSite.TplTr("Page%d", webInfo.CurrentPage))
		}
		// 增加上级标题
		if showParent && !strings.Contains(content, "{multicatname}") {
			parentId := uint(0)
			if webInfo.PageName == "archiveDetail" {
				archive, ok := ctx.Public["archive"].(*model.Archive)
				if ok {
					parentId = archive.CategoryId
				}
			} else if webInfo.PageName == "archiveList" {
				categoryInfo, ok := ctx.Public["category"].(*model.Category)
				if ok {
					parentId = categoryInfo.ParentId
				}
			}
			if parentId > 0 {
				categories := currentSite.GetParentCategories(parentId)
				// 先翻转categories
				for i, j := 0, len(categories)-1; i < j; i, j = i+1, j-1 {
					categories[i], categories[j] = categories[j], categories[i]
				}
				for _, category := range categories {
					titleText = append(titleText, category.Title)
				}
			}
		}
		if siteName && len(currentSite.System.SiteName) > 0 {
			titleText = append(titleText, currentSite.System.SiteName)
		}
		content = strings.Join(titleText, sep)
		if content == "" {
			// 保持标题至少是网站名称
			content = currentSite.System.SiteName
		}
	}

	// 查找{param}并执行批量替换
	if strings.Contains(content, "{") {
		var currentItem interface{}
		switch webInfo.PageName {
		case "archiveDetail":
			currentItem, _ = ctx.Public["archive"].(*model.Archive)
		case "archiveIndex":
			currentItem, _ = ctx.Public["module"].(*model.Module)
		case "archiveList":
			currentItem, _ = ctx.Public["category"].(*model.Category)
		case "pageDetail":
			currentItem, _ = ctx.Public["page"].(*model.Category)
		case "tag":
			currentItem, _ = ctx.Public["tag"].(*model.Tag)
		}

		content = parseTdkParams(content, currentSite, ctx, currentItem)
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
