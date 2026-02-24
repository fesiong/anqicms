package provider

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
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
			if mainSite.Id == w.Id && mainSite.MultiLanguage.ShowMainDir == false {
				// 无需处理
			} else {
				baseUrl = mainSite.System.BaseUrl + "/" + w.System.Language
			}
		} else if mainSite.MultiLanguage.Type == config.MultiLangTypeSame {
			baseUrl = mainSite.System.BaseUrl
		}
	}
	rewritePattern := mainSite.ParsePattern(false)
	uri := ""
	switch match {
	case PatternArchive:
		patternName := PatternArchive
		uri = rewritePattern.Patterns[patternName]
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
			// 如果自定义了模型的伪静态规则，则使用
			module := w.GetModuleFromCache(item.ModuleId)
			if module != nil {
				tmpPatternName := module.UrlToken + ":archive"
				tmpUri, ok := rewritePattern.Patterns[tmpPatternName]
				if ok {
					uri = tmpUri
					patternName = tmpPatternName
				}
			}

			for _, v := range rewritePattern.Tags[patternName] {
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
	case PatternArchiveIndex:
		uri = rewritePattern.Patterns[PatternArchiveIndex]
		item, ok := data.(*model.Module)
		if !ok {
			item2, ok2 := data.(model.Module)
			if ok2 {
				item = &item2
				ok = ok2
			}
		}
		if ok && item != nil {
			for _, v := range rewritePattern.Tags[PatternArchiveIndex] {
				// 模型首页，只支持module属性
				if v == "module" {
					uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
				}
			}
		}
	case PatternCategory:
		uri = rewritePattern.Patterns[PatternCategory]
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
				for _, v := range rewritePattern.Tags[PatternCategory] {
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
	case PatternPage:
		uri = rewritePattern.Patterns[PatternPage]
		item, ok := data.(*model.Category)
		if !ok {
			item2, ok2 := data.(model.Category)
			if ok2 {
				item = &item2
				ok = ok2
			}
		}
		if ok && item != nil {
			for _, v := range rewritePattern.Tags[PatternPage] {
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
					uri = w.GetUrl(PatternArchiveIndex, module, 0)
				}
			} else if item.NavType == model.NavTypeCategory {
				category := w.GetCategoryFromCache(uint(item.PageId))
				if category != nil {
					uri = w.GetUrl(PatternCategory, category, 0)
				}
			} else if item.NavType == model.NavTypeArchive {
				archive, _ := w.GetArchiveById(item.PageId)
				if archive != nil {
					uri = w.GetUrl(PatternArchive, archive, 0)
				}
			} else if item.NavType == model.NavTypeOutlink {
				//外链
				uri = item.Link
			}
		}
	case PatternPeople:
		uri = rewritePattern.Patterns[PatternPeople]
		item, ok := data.(*model.User)
		if !ok {
			item2, ok2 := data.(model.User)
			if ok2 {
				item = &item2
				ok = ok2
			}
		}
		if ok && item != nil {
			for _, v := range rewritePattern.Tags[PatternPeople] {
				if v == "id" {
					uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), fmt.Sprintf("%d", item.Id))
				} else if v == "filename" {
					uri = strings.ReplaceAll(uri, fmt.Sprintf("{%s}", v), item.UrlToken)
				}
			}
		}
	case PatternSearch:
		uri = rewritePattern.Patterns[PatternSearch]
		var q string
		var module string
		data2, ok := data.(map[string]interface{})
		if ok {
			q, _ = data2["q"].(string)
			module, _ = data2["module"].(string)
		}
		if q != "" {
			uri = strings.ReplaceAll(uri, "{filename}", q)
		}
		if module != "" {
			uri = strings.ReplaceAll(uri, "{module}", module)
		}
		re, _ := regexp.Compile(`\(.*?\)`)
		uri = re.ReplaceAllStringFunc(uri, func(s string) string {
			if strings.Contains(s, "{page}") {
				return s
			} else if strings.Contains(s, "{") {
				return ""
			} else {
				return strings.Trim(s, "()")
			}
		})
	case PatternTagIndex:
		uri = rewritePattern.Patterns[PatternTagIndex]
	case PatternTag:
		uri = rewritePattern.Patterns[PatternTag]
		item, ok := data.(*model.Tag)
		if !ok {
			item2, ok2 := data.(model.Tag)
			if ok2 {
				item = &item2
				ok = ok2
			}
		}
		if ok && item != nil {
			for _, v := range rewritePattern.Tags[PatternTag] {
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
			reg := regexp.MustCompile("\\(.*?\\)")
			uri = reg.ReplaceAllString(uri, "")
		}
	}
	// 处理 ()
	re, _ := regexp.Compile("\\(.*?\\)")
	uri = re.ReplaceAllStringFunc(uri, func(s string) string {
		if strings.Contains(s, "page") {
			return s
		}
		return ""
	})

	if strings.HasPrefix(uri, "/") {
		uri = baseUrl + uri
	}
	return uri
}
