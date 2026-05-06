package tags

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/flosch/pongo2/v6"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
)

func parseArgs(args map[string]pongo2.IEvaluator, ctx *pongo2.ExecutionContext) (map[string]*pongo2.Value, *pongo2.Error) {
	parsedArgs := map[string]*pongo2.Value{}
	for key, value := range args {
		val, err := value.Evaluate(ctx)
		if err != nil {
			return nil, err
		}
		parsedArgs[key] = val
	}

	return parsedArgs, nil
}

func parseWith(arguments *pongo2.Parser) (map[string]pongo2.IEvaluator, *pongo2.Error) {
	args := make(map[string]pongo2.IEvaluator)
	// After having parsed the name we're gonna parse the with options
	if arguments.Match(pongo2.TokenIdentifier, "with") != nil {
		for arguments.Remaining() > 0 {
			// We have at least one key=expr pair (because of starting "with")
			keyToken := arguments.MatchType(pongo2.TokenIdentifier)
			if keyToken == nil {
				return nil, arguments.Error("Expected an identifier", nil)
			}
			if arguments.Match(pongo2.TokenSymbol, "=") == nil {
				return nil, arguments.Error("Expected '='.", nil)
			}
			valueExpr, err := arguments.ParseExpression()
			if err != nil {
				return nil, arguments.Error("Can not parse with args.", keyToken)
			}

			args[keyToken.Val] = valueExpr
		}
	}

	return args, nil
}

func parseTdkParams(content string, currentSite *provider.Website, ctx *pongo2.ExecutionContext, item interface{}) string {
	webInfo, ok := ctx.Public["webInfo"].(*response.WebInfo)
	if !ok {
		return content
	}
	// 特殊处理 (...{page}...) 模式
	if strings.Contains(content, "{page}") && strings.Contains(content, "(") {
		re := regexp.MustCompile(`\(([^)]*\{page\}[^)]*)\)`)
		content = re.ReplaceAllStringFunc(content, func(s string) string {
			if webInfo.CurrentPage > 1 {
				// 移除外层括号并保留内容，{page} 会在后续统一替换
				return s[1 : len(s)-1]
			}
			// Page <= 1, 移除整个括号块
			return ""
		})
	}

	var match = make([]string, 0, 5)
	start := -1
	for i, v := range content {
		if v == '{' {
			start = i + 1
		} else if v == '}' && start != -1 {
			match = append(match, content[start:i])
			start = -1
		}
	}

	if len(match) > 0 {
		if webInfo.Sep == "" {
			webInfo.Sep = " - "
			if currentSite.Index.Sep != "" {
				webInfo.Sep = currentSite.Index.Sep
			}
		}
		replacerPairs := make([]string, 0, len(match)*2+10)
		replacerPairs = append(replacerPairs, "{sep}", webInfo.Sep, "{siteName}", currentSite.System.SiteName)
		// 分页特殊处理
		replacerPairs = append(replacerPairs, "{page}", strconv.Itoa(webInfo.CurrentPage))

		// 分类名称单独处理
		if strings.Contains(content, "{catname}") {
			categoryInfo, _ := ctx.Public["category"].(*model.Category)
			catName := ""
			if categoryInfo != nil {
				catName = categoryInfo.Title
			}
			replacerPairs = append(replacerPairs, "{catname}", catName)
		}
		if strings.Contains(content, "{module}") {
			moduleInfo, _ := ctx.Public["module"].(*model.Module)
			moduleName := ""
			if moduleInfo != nil {
				moduleName = moduleInfo.Title
			}
			replacerPairs = append(replacerPairs, "{module}", moduleName)
		}
		if strings.Contains(content, "{multicatname}") {
			parentId := uint(0)
			webInfo, ok := ctx.Public["webInfo"].(*response.WebInfo)
			if ok {
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
			}
			var titleText = make([]string, 0, 10)
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
			replacerPairs = append(replacerPairs, "{multicatname}", strings.Join(titleText, webInfo.Sep))
		}

		// 处理动态匹配的字段
		for _, m := range match {
			if m == "sep" || m == "siteName" || m == "catname" || m == "multicatname" || m == "module" || m == "page" {
				continue
			}
			val := getModelFieldValue(item, m)
			replacerPairs = append(replacerPairs, "{"+m+"}", val)
		}
		content = strings.NewReplacer(replacerPairs...).Replace(content)
	}
	return content
}

// getModelFieldValue 优化字段获取，优先使用类型断言减少反射
func getModelFieldValue(item interface{}, name string) string {
	if item == nil {
		return "{" + name + "}"
	}
	fieldName := library.Case2Camel(name)
	switch v := item.(type) {
	case *model.Archive:
		switch fieldName {
		case "Title":
			return v.Title
		case "Keywords":
			return v.Keywords
		case "Description":
			return v.Description
		case "SeoTitle":
			return v.SeoTitle
		}
	case *model.Category:
		switch fieldName {
		case "Title":
			return v.Title
		case "Keywords":
			return v.Keywords
		case "Description":
			return v.Description
		case "SeoTitle":
			return v.SeoTitle
		}
	case *model.Module:
		switch fieldName {
		case "Title":
			return v.Title
		case "Keywords":
			return v.Keywords
		case "Description":
			return v.Description
		}
	case *model.Tag:
		switch fieldName {
		case "Title":
			return v.Title
		case "Keywords":
			return v.Keywords
		case "Description":
			return v.Description
		case "SeoTitle":
			return v.SeoTitle
		}
	}

	// 备选方案：使用反射获取其他字段
	val := reflect.ValueOf(item)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return "{" + name + "}"
	}
	f := val.FieldByName(fieldName)
	if f.IsValid() {
		return fmt.Sprint(f.Interface())
	}
	return "{" + name + "}"
}

func parseContent(content string, render bool, currentSite *provider.Website, ctx *pongo2.ExecutionContext) string {
	var value string
	if render {
		value = library.MarkdownToHTML(content, currentSite.System.BaseUrl, currentSite.Content.FilterOutlink)
	} else {
		value = content
	}
	// 对宏函数进行解析
	var showContentTitle bool
	value, showContentTitle = currentSite.RenderTemplateMacro(value, ctx.Private)
	if showContentTitle {
		ctx.Private["showContentTitle"] = showContentTitle
	}
	if isShow, ok := ctx.Private["showContentTitle"]; ok && isShow == true {
		_, value = library.ParseContentTitles(value, "list")
	}
	// end
	content = currentSite.ReplaceContentUrl(value, true)

	return content
}
