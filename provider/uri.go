package provider

import (
	"fmt"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GetUrl 生成url
// 支持的规则：getUrl("archive"|"category"|"page"|"nav"|"archiveIndex", item, int)
// 如果page == -1，则不对page进行转换。
// 支持多语言站点功能
func (w *Website) GetUrl(match string, data interface{}, page int, args ...interface{}) string {
	mainSite := w.GetMainWebsite()
	baseUrl := w.System.BaseUrl
	if mainSite.MultiLanguage.Open {
		if mainSite.MultiLanguage.Type == config.MultiLangTypeDirectory {
			// 替换目录
			baseUrl = mainSite.System.BaseUrl + "/" + w.System.Language
		} else if mainSite.MultiLanguage.Type == config.MultiLangTypeSame {
			baseUrl = mainSite.System.BaseUrl
		}
	}
	rewritePattern := mainSite.ParsePatten(false)
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
			} else {
				item3, ok3 := data.(*model.ArchiveDraft)
				if ok3 {
					item = &item3.Archive
					ok = ok3
				}
			}
		}
		if ok && item != nil {
			if item.FixedLink != "" {
				uri = item.FixedLink
				break
			}
			// 修正空白的urlToken
			_ = w.UpdateArchiveUrlToken(item)

			for _, v := range rewritePattern.ArchiveTags {
				if v == "id" {
					uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
					if len(args) > 0 {
						if combines, ok := args[0].([]*model.Archive); ok && combines != nil {
							var combineIds []string
							for _, combine := range combines {
								combineIds = append(combineIds, strconv.FormatInt(combine.Id, 10))
							}
							uri = strings.ReplaceAll(uri, "(/c-{combine})", "/c-"+strings.Join(combineIds, "-"))
						}
					}
				} else if v == "catid" {
					uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.CategoryId))
				} else if v == "filename" {
					uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
					if len(args) > 0 {
						// combine 只支持ID
						if combines, ok := args[0].([]*model.Archive); ok && combines != nil {
							var combineIds []string
							for _, combine := range combines {
								combineIds = append(combineIds, strconv.FormatInt(combine.Id, 10))
							}
							uri = strings.ReplaceAll(uri, "(/c-{combine})", "/c-"+strings.Join(combineIds, "-"))
						}
					}
				} else if v == "catname" {
					catName := ""
					if item.Category != nil {
						catName = item.Category.UrlToken
					} else {
						category := w.GetCategoryFromCache(item.CategoryId)
						if category != nil {
							catName = category.UrlToken
						}
					}
					uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), catName)
				} else if v == "multicatname" {
					var catNames string
					category := w.GetCategoryFromCache(item.CategoryId)
					for category != nil {
						catNames = category.UrlToken + "/" + catNames
						category = w.GetCategoryFromCache(category.ParentId)
					}
					uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), strings.Trim(catNames, "/"))
				} else if v == "module" {
					moduleToken := ""
					module := w.GetModuleFromCache(item.ModuleId)
					if module != nil {
						moduleToken = module.UrlToken
					}
					uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), moduleToken)
				} else if v == "year" || v == "month" || v == "day" || v == "hour" || v == "minute" || v == "second" {
					var timeFormat string
					if v == "year" {
						timeFormat = "2006"
					} else if v == "month" {
						timeFormat = "01"
					} else if v == "day" {
						timeFormat = "02"
					} else if v == "hour" {
						timeFormat = "15"
					} else if v == "minute" {
						timeFormat = "04"
					} else if v == "second" {
						timeFormat = "05"
					}
					uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), time.Unix(item.CreatedTime, 0).Format(timeFormat))
				}
			}
		}
		//否则删除combine
		uri = strings.ReplaceAll(uri, "(/c-{combine})", "")
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
				uri = w.GetUrl("page", item, 0)
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
					} else if v == "multicatname" {
						var catNames string
						category := w.GetCategoryFromCache(item.Id)
						for category != nil {
							catNames = category.UrlToken + "/" + catNames
							category = w.GetCategoryFromCache(category.ParentId)
						}
						uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), strings.Trim(catNames, "/"))
					} else if v == "module" {
						moduleToken := ""
						module := w.GetModuleFromCache(item.ModuleId)
						if module != nil {
							moduleToken = module.UrlToken
						}
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
				} else if v == "filename" || v == "catname" {
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
					module := w.GetModuleFromCache(uint(item.PageId))
					uri = w.GetUrl("archiveIndex", module, 0)
				}
			} else if item.NavType == model.NavTypeCategory {
				category := w.GetCategoryFromCache(uint(item.PageId))
				if category != nil {
					uri = w.GetUrl("category", category, 0)
				}
			} else if item.NavType == model.NavTypeArchive {
				archive, _ := w.GetArchiveById(item.PageId)
				if archive != nil {
					uri = w.GetUrl("archive", archive, 0)
				}
			} else if item.NavType == model.NavTypeOutlink {
				//外链
				uri = item.Link
			}
		}
	case "user":
		uri = "/people/{id}.html"
		item, ok := data.(*model.User)
		if !ok {
			item2, ok2 := data.(model.User)
			if ok2 {
				item = &item2
				ok = ok2
			}
		}
		if ok && item != nil {
			uri = strings.ReplaceAll(uri, "{id}", fmt.Sprintf("%d", item.Id))
			//uri = strings.ReplaceAll(uri, "{filename}", item.UserName)
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
				} else if v == "filename" || v == "catname" {
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

	if strings.HasPrefix(uri, "/") {
		uri = baseUrl + uri
	}
	return uri
}
