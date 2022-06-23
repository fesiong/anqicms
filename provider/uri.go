package provider

import (
    "fmt"
    "kandaoni.com/anqicms/config"
    "kandaoni.com/anqicms/model"
    "regexp"
    "strings"
)

// GetUrl 生成url
//支持的规则：getUrl("archive"|"category"|"page"|"nav"|"archiveIndex", item, int)
//如果page == -1，则不对page进行转换。
func GetUrl(match string, data interface{}, page int) string {
    rewritePattern := config.ParsePatten(false)
    uri := ""
    switch match {
    case "archive":
        uri = rewritePattern.Archive
        item, ok := data.(*model.Archive)
        if !ok {
            item2, ok2 := data.(model.Archive)
            if ok2 {
                item = &item2
                ok = ok2
            }
        }
        if ok && item != nil {
            if item.FixedLink != "" {
                uri =  item.FixedLink
                break
            }
            // 修正
            if item.UrlToken == "" {
                _ = UpdateArchiveUrlToken(item)
            }
            //拿到值
            catName := ""
            if strings.Contains(rewritePattern.Archive, "catname") {
                if item.Category != nil {
                    catName = item.Category.UrlToken
                } else {
                    category := GetCategoryFromCache(item.CategoryId)
                    if category != nil {
                        catName = category.UrlToken
                    }
                }
            }
            moduleToken := ""
            module := GetModuleFromCache(item.ModuleId)
            if module != nil {
                moduleToken = module.UrlToken
            }
            for _, v := range rewritePattern.ArchiveTags {
                if v == "id" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
                } else if v == "catid" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.CategoryId))
                } else if v == "filename" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
                } else if v == "catname" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), catName)
                } else if v == "module" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), moduleToken)
                }
            }
        }
    case "archiveIndex":
        uri = rewritePattern.ArchiveIndex
        item, ok := data.(*model.Module)
        if !ok {
            item2, ok2 := data.(model.Module)
            if ok2 {
                item = &item2
                ok = ok2
            }
        }
        if ok && item != nil {
            for _, v := range rewritePattern.ArchiveIndexTags {
                // 模型首页，只支持module属性
                if v == "module" {
                    uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
                }
            }
        }
    case "category":
        uri = rewritePattern.Category
        item, ok := data.(*model.Category)
        if !ok {
            item2, ok2 := data.(model.Category)
            if ok2 {
                item = &item2
                ok = ok2
            }
        }
        if ok && item != nil {
            //自动修正
            if item.Type == config.CategoryTypePage {
                uri = GetUrl("page", item, 0)
            } else {
                moduleToken := ""
                module := GetModuleFromCache(item.ModuleId)
                if module != nil {
                    moduleToken = module.UrlToken
                }
                for _, v := range rewritePattern.CategoryTags {
                    if v == "id" {
                        uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
                    } else if v == "catid" {
                        uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
                    } else if v == "filename" {
                        uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
                    } else if v == "catname" {
                        uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
                    } else if v == "module" {
                        uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), moduleToken)
                    }
                }
            }
        }
    case "page":
        uri = rewritePattern.Page
        item, ok := data.(*model.Category)
        if !ok {
            item2, ok2 := data.(model.Category)
            if ok2 {
                item = &item2
                ok = ok2
            }
        }
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
        if !ok {
            item2, ok2 := data.(model.Nav)
            if ok2 {
                item = &item2
                ok = ok2
            }
        }
        if ok {
            if item.NavType == model.NavTypeSystem {
                if item.PageId == 0 {
                    //首页
                    uri = "/"
                } else if item.PageId > 0 {
                    //文档首页
                    module := GetModuleFromCache(item.PageId)
                    uri = GetUrl("archiveIndex", module, 0)
                }
            } else if item.NavType == model.NavTypeCategory {
                category := GetCategoryFromCache(item.PageId)
                if category != nil {
                    uri = GetUrl("category", category, 0)
                }
            } else if item.NavType == model.NavTypeOutlink {
                //外链
                uri = item.Link
            }
        }
    case "tagIndex":
        uri = rewritePattern.TagIndex
    case "tag":
        uri = rewritePattern.Tag
        item, ok := data.(*model.Tag)
        if !ok {
            item2, ok2 := data.(model.Tag)
            if ok2 {
                item = &item2
                ok = ok2
            }
        }
        if ok && item != nil {
            for _, v := range rewritePattern.TagTags {
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
    if uri == "" {
        if strings.HasPrefix(match, "/") {
            uri = match
        } else {
            //将连接设置为首页
            uri = "/"
        }
    }
    //处理分页问题
    if strings.Contains(uri, "{page}") {
        if page > 1 {
            //需要展示分页
            uri = strings.ReplaceAll(uri, "{page}", fmt.Sprintf("%d", page))
            uri = strings.ReplaceAll(uri, "(", "")
            uri = strings.ReplaceAll(uri, ")", "")
        } else if page == -1 {
            //不做处理
        } else {
            //否则删除分页内容
            reg := regexp.MustCompile("\\(.*\\)")
            uri = reg.ReplaceAllString(uri, "")
        }
    }

    if strings.HasPrefix(uri, "/") && !strings.HasPrefix(uri, "//") {
        uri = config.JsonData.System.BaseUrl + uri
    }
    return uri
}
