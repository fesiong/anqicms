package provider

import (
    "fmt"
    "irisweb/config"
    "irisweb/model"
    "regexp"
    "strings"
)

//生成url
//支持的规则：getUrl("article"|"product"|"category"|"page"|"nav"|"articleIndex"|"productIndex", item, int)
func GetUrl(match string, data interface{}, page int) string {
    rewritePattern := config.ParsePatten()
    uri := ""
    switch match {
    case "article":
        uri = rewritePattern.Article
        item, ok := data.(*model.Article)
        if ok && item != nil {
            //拿到值
            catName := ""
            if strings.Contains(rewritePattern.Article, "catname") {
                category, err := GetCategoryById(item.CategoryId)
                if err == nil {
                    catName = category.UrlToken
                }
            }
            for _, v := range rewritePattern.ArticleTags {
                if v == "id" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
                } else if v == "catid" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.CategoryId))
                } else if v == "filename" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
                } else if v == "catname" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), catName)
                }
            }
        }
    case "product":
        uri = rewritePattern.Product
        item, ok := data.(*model.Product)
        if ok && item != nil {
            catName := ""
            if strings.Contains(rewritePattern.Article, "catname") {
                category, err := GetCategoryById(item.CategoryId)
                if err == nil {
                    catName = category.UrlToken
                }
            }
            for _, v := range rewritePattern.ProductTags {
                if v == "id" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
                } else if v == "catid" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.CategoryId))
                } else if v == "filename" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
                } else if v == "catname" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), catName)
                }
            }
        }
    case "articleIndex":
        uri = rewritePattern.ArticleIndex
    case "productIndex":
        uri = rewritePattern.ProductIndex
    case "category":
        uri = rewritePattern.Category
        item, ok := data.(*model.Category)
        if ok && item != nil {
            //自动修正
            if item.Type == model.CategoryTypePage {
                uri = GetUrl("page", item, 0)
            } else {
                for _, v := range rewritePattern.CategoryTags {
                    if v == "id" {
                        uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
                    } else if v == "catid" {
                        uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
                    } else if v == "filename" {
                        uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
                    } else if v == "catname" {
                        uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
                    }
                }
            }
        }
    case "page":
        uri = rewritePattern.Page
        item, ok := data.(*model.Category)
        if ok && item != nil {
            for _, v := range rewritePattern.PageTags {
                if v == "id" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
                } else if v == "catid" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
                } else if v == "filename" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
                } else if v == "catname" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
                }
            }
        }
    case "nav":
        uri = ""
        item, ok := data.(*model.Nav)
        if ok {
            if item.NavType == model.NavTypeSystem {
                if item.PageId == 0 {
                    //首页
                    uri = "/"
                } else if item.PageId == 1 {
                    //文章首页
                    uri = GetUrl("articleIndex", nil, 0)
                } else if item.PageId == 2 {
                    //产品首页
                    uri = GetUrl("productIndex", nil, 0)
                }
            } else if item.NavType == model.NavTypeCategory {
                category, err := GetCategoryById(item.PageId)
                if err == nil {
                    uri = GetUrl("category", category, 0)
                }
            } else if item.NavType == model.NavTypeOutlink {
                //外链
                uri = item.Link
            }
        }
    }
    //处理分页问题
    if strings.Contains(uri, "{page}") {
        if page > 1 {
            //需要展示分页
            uri = strings.ReplaceAll(uri, "{page}", fmt.Sprintf("%d", page))
            uri = strings.ReplaceAll(uri, "(", "")
            uri = strings.ReplaceAll(uri, ")", "")
        } else {
            //否则删除分页内容
            reg := regexp.MustCompile("\\(.*\\)")
            uri = reg.ReplaceAllString(uri, "")
        }
    }

    if uri == "" {
        //将连接设置为首页
        uri = "/"
    }

    return uri
}
