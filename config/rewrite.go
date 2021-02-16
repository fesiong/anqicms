package config

import (
    "fmt"
    "strings"
)

const (
    RewriteNumberMode = 0 //数字模式
    RewriteStringMode = 1 //命名模式
    RewriteTinyMode   = 2 //极简数字模式
    RewritePattenMode = 3 //正则模式
)

type PluginRewriteConfig struct {
    Mode   int    `json:"mode"`
    Patten string `json:"patten"`
}

type RewritePatten struct {
    Article      string `json:"article"`
    Product      string `json:"product"`
    Category     string `json:"category"`
    Page         string `json:"page"`
    ArticleIndex string `json:"article_index"`
    ProductIndex string `json:"product_index"`

    ArticleRule      string
    ProductRule      string
    CategoryRule     string
    PageRule         string
    ArticleIndexRule string
    ProductIndexRule string

    ArticleTags  map[int]string
    ProductTags  map[int]string
    CategoryTags map[int]string
    PageTags     map[int]string

    Parsed bool
}

var rewriteNumberModePatten = RewritePatten{
    Article:      "/a/{id}.html",
    Product:      "/p/{id}.html",
    Category:     "/c/{id}(/{page})",
    Page:         "/{id}.html",
    ArticleIndex: "/a",
    ProductIndex: "/p",
}

var rewriteStringModePatten = RewritePatten{
    Article:      "/a/{filename}.html",
    Product:      "/p/{filename}.html",
    Category:     "/c/{filename}(/{page})",
    Page:         "/{filename}.html",
    ArticleIndex: "/a",
    ProductIndex: "/p",
}

var rewriteTinyModePatten = RewritePatten{
    Article:      "/a_{id}.html",
    Product:      "/p_{id}.html",
    Category:     "/c_{id}(_{page})",
    Page:         "/{id}.html",
    ArticleIndex: "/a",
    ProductIndex: "/p",
}

var needReplace = map[string]string{
    "/": "\\/",
    "*": "\\*",
    "+": "\\+",
    "?": "\\?",
    ".": "\\.",
    "-": "\\-",
    "[": "\\[",
    "]": "\\]",
    ")": ")?", //fix?
}

var replaceParams = map[string]string{
    "{id}":       "([\\d]+)",
    "{filename}": "([^\\/\\.\\_]+)",
    "{catname}":  "([^\\/\\.\\_]+)",
    "{catid}":    "([\\d]+)",
    "{page}":     "([\\d]+)",
}

var parsedPatten *RewritePatten

func GetRewritePatten() *RewritePatten {
    if parsedPatten != nil {
        return parsedPatten
    }
    if JsonData.PluginRewrite.Mode == RewriteNumberMode {
        parsedPatten = &rewriteNumberModePatten
    } else if JsonData.PluginRewrite.Mode == RewriteStringMode {
        parsedPatten = &rewriteStringModePatten
    } else if JsonData.PluginRewrite.Mode == RewriteTinyMode {
        parsedPatten = &rewriteTinyModePatten
    } else if JsonData.PluginRewrite.Mode == RewritePattenMode {
        parsedPatten = parseRewritePatten(JsonData.PluginRewrite.Patten)
    }

    return parsedPatten
}

// 只有 RewritePattenMode 模式下，才需要解析
// 一共4行,分别是文章详情、产品详情、分类、页面,===和前面部分不可修改。
// 变量由花括号包裹{},如{id}。可用的变量有:数据ID {id}、数据自定义链接名 {filename}、分类自定义链接名 {catname}、分类ID {catid},分页ID {page}，分页需要使用()处理，用来首页忽略。如：(/{page})或(_{page})
func parseRewritePatten(patten string) *RewritePatten {
    parsedPatten := &RewritePatten{}
    // 再解开
    pattenSlice := strings.Split(patten, "\n")
    for _, v := range pattenSlice {
        singlePatten := strings.Split(v, "===")
        if len(singlePatten) == 2 {
            val := strings.TrimSpace(singlePatten[1])

            switch strings.TrimSpace(singlePatten[0]) {
            case "article":
                parsedPatten.Article = val
            case "product":
                parsedPatten.Product = val
            case "category":
                parsedPatten.Category = val
            case "page":
                parsedPatten.Page = val
            case "articleIndex":
                parsedPatten.ArticleIndex = val
            case "productIndex":
                parsedPatten.ProductIndex = val
            }
        }
    }

    return parsedPatten
}

func ParsePatten() *RewritePatten {
    GetRewritePatten()
    if parsedPatten.Parsed {
        return parsedPatten
    }

    parsedPatten.ArticleTags = map[int]string{}
    parsedPatten.ProductTags = map[int]string{}
    parsedPatten.CategoryTags = map[int]string{}
    parsedPatten.PageTags = map[int]string{}

    pattens := map[string]string{
        "article":  parsedPatten.Article,
        "product":  parsedPatten.Product,
        "category": parsedPatten.Category,
        "page":     parsedPatten.Page,
    }
    for key, item := range pattens {
        n := 0
        str := ""
        for _, v := range item {
            if v == '{' {
                n++
                str += string(v)
            } else if v == '}' {
                str = strings.TrimLeft(str, "{")
                if str == "page" {
                    //page+1
                    n++
                }
                switch key {
                case "article":
                    parsedPatten.ArticleTags[n] = str
                case "product":
                    parsedPatten.ProductTags[n] = str
                case "category":
                    parsedPatten.CategoryTags[n] = str
                case "page":
                    parsedPatten.PageTags[n] = str
                }
                //重置
                n = 0
                str = ""
            } else if str != "" {
                str += string(v)
            }
        }
    }

    //移除首个 /
    parsedPatten.ArticleRule = strings.TrimLeft(parsedPatten.Article, "/")
    parsedPatten.ProductRule = strings.TrimLeft(parsedPatten.Product, "/")
    parsedPatten.CategoryRule = strings.TrimLeft(parsedPatten.Category, "/")
    parsedPatten.PageRule = strings.TrimLeft(parsedPatten.Page, "/")
    parsedPatten.ArticleIndexRule = strings.TrimLeft(parsedPatten.ArticleIndex, "/")
    parsedPatten.ProductIndexRule = strings.TrimLeft(parsedPatten.ProductIndex, "/")

    for s, r := range needReplace {
        if strings.Contains(parsedPatten.ArticleRule, s) {
            parsedPatten.ArticleRule = strings.ReplaceAll(parsedPatten.ArticleRule, s, r)
        }
        if strings.Contains(parsedPatten.ProductRule, s) {
            parsedPatten.ProductRule = strings.ReplaceAll(parsedPatten.ProductRule, s, r)
        }
        if strings.Contains(parsedPatten.CategoryRule, s) {
            parsedPatten.CategoryRule = strings.ReplaceAll(parsedPatten.CategoryRule, s, r)
        }
        if strings.Contains(parsedPatten.PageRule, s) {
            parsedPatten.PageRule = strings.ReplaceAll(parsedPatten.PageRule, s, r)
        }
    }

    for s, r := range replaceParams {
        if strings.Contains(parsedPatten.ArticleRule, s) {
            parsedPatten.ArticleRule = strings.ReplaceAll(parsedPatten.ArticleRule, s, r)
        }
        if strings.Contains(parsedPatten.ProductRule, s) {
            parsedPatten.ProductRule = strings.ReplaceAll(parsedPatten.ProductRule, s, r)
        }
        if strings.Contains(parsedPatten.CategoryRule, s) {
            parsedPatten.CategoryRule = strings.ReplaceAll(parsedPatten.CategoryRule, s, r)
        }
        if strings.Contains(parsedPatten.PageRule, s) {
            parsedPatten.PageRule = strings.ReplaceAll(parsedPatten.PageRule, s, r)
        }
    }
    //修改为强制包裹
    parsedPatten.ArticleRule = fmt.Sprintf("^%s$", parsedPatten.ArticleRule)
    parsedPatten.ProductRule = fmt.Sprintf("^%s$", parsedPatten.ProductRule)
    parsedPatten.CategoryRule = fmt.Sprintf("^%s$", parsedPatten.CategoryRule)
    parsedPatten.PageRule = fmt.Sprintf("^%s$", parsedPatten.PageRule)
    parsedPatten.ArticleIndexRule = fmt.Sprintf("^%s$", parsedPatten.ArticleIndexRule)
    parsedPatten.ProductIndexRule = fmt.Sprintf("^%s$", parsedPatten.ProductIndexRule)

    //标记替换过
    parsedPatten.Parsed = true

    return parsedPatten
}
