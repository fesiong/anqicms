package provider

import (
	"encoding/json"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
	"strings"
	"time"
)

// GetJsonLd 该方法用于生成 json-ld
func (w *Website) GetJsonLd(ctx iris.Context) string {
	var jsonLd = iris.Map{
		"@context": "https://schema.org",
	}
	viewData := ctx.GetViewData()
	// 判断页面类型
	webInfo, ok := viewData["webInfo"].(*response.WebInfo)
	if ok {
		// 先判断详情
		if webInfo.PageName == "archiveDetail" {
			archive, ok := viewData["archive"].(*model.Archive)
			if ok {
				// 目前仅支持2种type, product 和 article
				module := w.GetModuleFromCache(archive.ModuleId)
				if module != nil {
					if strings.HasPrefix(module.TableName, "product") {
						jsonLd["@type"] = "Product"
						jsonLd["name"] = archive.Title
						jsonLd["offers"] = iris.Map{
							"@type":         "Offer",
							"offerCount":    archive.Stock,
							"price":         float32(archive.Price) / 100.00,
							"priceCurrency": "USD", // todo
							"availability":  "https://schema.org/InStock",
							"itemCondition": "https://schema.org/NewCondition",
							"url":           webInfo.CanonicalUrl,
						}
						jsonLd["brand"] = iris.Map{
							"@type": "Brand",
							"name":  w.PluginJsonLd.DefaultBrand,
						}
					} else {
						jsonLd["@type"] = "Article"
						jsonLd["headline"] = archive.Title
					}
				} else {
					jsonLd["@type"] = "Article"
					jsonLd["headline"] = archive.Title
					jsonLd["author"] = []iris.Map{
						{
							"@type": "Person",
							"name":  w.PluginJsonLd.DefaultAuthor,
						},
					}
				}
				if len(archive.Images) > 0 {
					jsonLd["image"] = archive.Images
				}
				jsonLd["datePublished"] = time.Unix(archive.CreatedTime, 0).Format("2006-01-02T15:04:05+08:00")
				jsonLd["dateModified"] = time.Unix(archive.UpdatedTime, 0).Format("2006-01-02T15:04:05+08:00")
				jsonLd["description"] = archive.Description
				jsonLd["mainEntityOfPage"] = iris.Map{
					"@type": "WebPage",
					"@id":   webInfo.CanonicalUrl,
				}
			}
		} else if webInfo.PageName == "archiveIndex" ||
			webInfo.PageName == "archiveList" ||
			webInfo.PageName == "search" ||
			webInfo.PageName == "tagIndex" ||
			webInfo.PageName == "tag" ||
			webInfo.PageName == "userDetail" {
			// 封面
			jsonLd["@type"] = "CollectionPage"
			jsonLd["name"] = webInfo.Title
			jsonLd["description"] = webInfo.Description

			listData, ok := viewData["listData"].([]*model.Archive)
			if ok {
				if len(listData) > 0 {
					var itemList = make([]iris.Map, 0, len(listData))
					for idx, archive := range listData {
						itemList = append(itemList, iris.Map{
							"@type":       "ListItem",
							"position":    idx + 1,
							"name":        archive.Title,
							"url":         archive.Link,
							"image":       archive.Logo,
							"description": archive.Description,
						})
					}

					jsonLd["mainEntityOfPage"] = iris.Map{
						"@type":           "ItemList",
						"itemListElement": itemList,
					}
				}
			}
		} else if webInfo.PageName == "pageDetail" {
			// 列表
			jsonLd["@type"] = "Article"
			page, ok := viewData["page"].(*model.Category)
			if ok {
				jsonLd["@type"] = "Article"
				jsonLd["headline"] = page.Title
				jsonLd["author"] = []iris.Map{
					{
						"@type": "Person",
						"name":  w.PluginJsonLd.DefaultAuthor,
					},
				}
				if len(page.Images) > 0 {
					jsonLd["image"] = page.Images
				}
				jsonLd["datePublished"] = time.Unix(page.CreatedTime, 0).Format("2006-01-02T15:04:05+08:00")
				jsonLd["dateModified"] = time.Unix(page.UpdatedTime, 0).Format("2006-01-02T15:04:05+08:00")
				jsonLd["description"] = page.Description
				jsonLd["mainEntityOfPage"] = iris.Map{
					"@type": "WebPage",
					"@id":   webInfo.CanonicalUrl,
				}
			}
		} else if webInfo.PageName == "index" {
			jsonLd["@type"] = "WebSite"
			jsonLd["name"] = w.System.SiteName
			jsonLd["url"] = w.System.BaseUrl
			jsonLd["description"] = w.Index.SeoDescription
			siteLogo := w.System.SiteLogo
			if siteLogo != "" && !strings.HasPrefix(siteLogo, "http") && !strings.HasPrefix(siteLogo, "//") {
				siteLogo = w.PluginStorage.StorageUrl + siteLogo
			}
			jsonLd["logo"] = siteLogo
			jsonLd["potentialAction"] = iris.Map{
				"@type":       "SearchAction",
				"target":      w.System.BaseUrl + "/search?q={search_term_string}",
				"query-input": "required name=search_term_string",
			}
		}
	}

	if len(jsonLd) > 1 {
		buf, _ := json.MarshalIndent(jsonLd, "", "\t")
		return string(buf)
	}

	return ""
}
