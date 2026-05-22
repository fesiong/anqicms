package provider

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
)

func (w *Website) GetJsonLdSetting() *config.PluginJsonLdConfig {
	if w.PluginJsonLd == nil {
		return nil
	}
	if w.PluginJsonLd.OrganizationUrl == "" {
		w.PluginJsonLd.OrganizationUrl = w.System.BaseUrl
	}
	if w.PluginJsonLd.ContactNumber == "" {
		w.PluginJsonLd.ContactNumber = w.Contact.Cellphone
	}
	if len(w.PluginJsonLd.SocialProfiles) == 0 {
		var socialProfiles []string
		if w.Contact.Facebook != "" {
			socialProfiles = append(socialProfiles, w.Contact.Facebook)
		}
		if w.Contact.Twitter != "" {
			socialProfiles = append(socialProfiles, w.Contact.Twitter)
		}
		if w.Contact.Instagram != "" {
			socialProfiles = append(socialProfiles, w.Contact.Instagram)
		}
		if w.Contact.Youtube != "" {
			socialProfiles = append(socialProfiles, w.Contact.Youtube)
		}
		if w.Contact.Linkedin != "" {
			socialProfiles = append(socialProfiles, w.Contact.Linkedin)
		}
		if w.Contact.Pinterest != "" {
			socialProfiles = append(socialProfiles, w.Contact.Pinterest)
		}
		if w.Contact.Tiktok != "" {
			socialProfiles = append(socialProfiles, w.Contact.Tiktok)
		}
		w.PluginJsonLd.SocialProfiles = socialProfiles
	}

	return w.PluginJsonLd
}

func (w *Website) GetJsonLd(ctx iris.Context) string {
	if w.PluginJsonLd == nil || !w.PluginJsonLd.Open {
		return ""
	}

	viewData := ctx.GetViewData()
	webInfo, ok := viewData["webInfo"].(*response.WebInfo)
	if !ok {
		return ""
	}

	var jsonLdList []interface{}

	setting := w.GetJsonLdSetting()

	if setting.IncludeHomepage && webInfo.PageName == "index" {
		homepageLd := w.buildHomepageJsonLd()
		if homepageLd != nil {
			jsonLdList = append(jsonLdList, homepageLd)
		}
	}

	switch webInfo.PageName {
	case "archiveDetail":
		jsonLdList = append(jsonLdList, w.buildArchiveDetailJsonLd(viewData, webInfo)...)
	case "archiveList", "archiveIndex", "search", "tagIndex", "tag", "userDetail":
		jsonLdList = append(jsonLdList, w.buildListPageJsonLd(viewData, webInfo)...)
	case "pageDetail":
		jsonLdList = append(jsonLdList, w.buildPageDetailJsonLd(viewData, webInfo)...)
	}
	// faqs
	if faqs, ok := viewData["faqs"].([]*model.Archive); ok && len(faqs) > 0 {
		jsonLdList = append(jsonLdList, w.buildFaqJsonLd(faqs, webInfo))
	}

	if len(jsonLdList) > 0 {
		var buf []byte
		if len(jsonLdList) == 1 {
			jsonLd := jsonLdList[0].(iris.Map)
			jsonLd["@context"] = "https://schema.org"
			buf, _ = json.MarshalIndent(jsonLd, "", "\t")
		} else {
			jsonLd := iris.Map{
				"@context": "https://schema.org",
				"@graph":   jsonLdList,
			}
			buf, _ = json.MarshalIndent(jsonLd, "", "\t")
		}
		return string(buf)
	}

	return ""
}

func (w *Website) buildHomepageJsonLd() iris.Map {
	setting := w.GetJsonLdSetting()

	var jsonLd iris.Map
	if setting.DataType == 2 && setting.PersonName != "" {
		jsonLd = w.buildPersonJsonLd()
	} else {
		jsonLd = w.buildOrganizationJsonLd()
	}

	if setting.IncludeSearch {
		jsonLd["potentialAction"] = iris.Map{
			"@type":       "SearchAction",
			"target":      w.System.BaseUrl + "/search?q={search_term_string}",
			"query-input": "required name=search_term_string",
		}
	}

	return jsonLd
}

func (w *Website) buildOrganizationJsonLd() iris.Map {
	setting := w.GetJsonLdSetting()

	orgType := "Organization"
	if setting.OrganizationType != "" {
		orgType = setting.OrganizationType
	}

	jsonLd := iris.Map{
		//	"@context": "https://schema.org",
		"@type": orgType,
	}

	if setting.OrganizationName != "" {
		jsonLd["name"] = setting.OrganizationName
	} else {
		jsonLd["name"] = w.System.SiteName
	}

	if setting.OrganizationLegalName != "" {
		jsonLd["legalName"] = setting.OrganizationLegalName
	}

	if setting.OrganizationUrl != "" {
		jsonLd["url"] = setting.OrganizationUrl
	} else {
		jsonLd["url"] = w.System.BaseUrl
	}
	jsonLd["@id"] = jsonLd["url"]

	if setting.LogoImage != "" {
		jsonLd["logo"] = setting.LogoImage
	} else if w.System.SiteLogo != "" {
		siteLogo := w.System.SiteLogo
		if !strings.HasPrefix(siteLogo, "http") && !strings.HasPrefix(siteLogo, "//") {
			siteLogo = w.PluginStorage.StorageUrl + siteLogo
		}
		jsonLd["logo"] = siteLogo
	}

	if len(setting.SocialProfiles) > 0 {
		jsonLd["sameAs"] = setting.SocialProfiles
	}

	if setting.ContactNumber != "" || setting.ContactUrl != "" || setting.ContactType != "" {
		contactPoint := iris.Map{
			"@type": "ContactPoint",
		}
		if setting.ContactNumber != "" {
			contactPoint["telephone"] = setting.ContactNumber
		}
		if setting.ContactUrl != "" {
			contactPoint["contactUrl"] = setting.ContactUrl
		}
		if setting.ContactType != "" {
			contactPoint["contactType"] = setting.ContactType
		}
		contactPoint["url"] = jsonLd["url"]

		jsonLd["contactPoint"] = contactPoint
	}

	if setting.GeoLatitude != "" && setting.GeoLongitude != "" {
		jsonLd["geo"] = iris.Map{
			"@type":     "GeoCoordinates",
			"latitude":  setting.GeoLatitude,
			"longitude": setting.GeoLongitude,
		}
	}

	if setting.StreetAddress != "" || setting.AddressLocality != "" || setting.AddressRegion != "" || setting.PostalCode != "" || setting.AddressCountry != "" {
		address := iris.Map{
			"@type": "PostalAddress",
		}
		if setting.StreetAddress != "" {
			address["streetAddress"] = setting.StreetAddress
		}
		if setting.AddressLocality != "" {
			address["addressLocality"] = setting.AddressLocality
		}
		if setting.AddressRegion != "" {
			address["addressRegion"] = setting.AddressRegion
		}
		if setting.PostalCode != "" {
			address["postalCode"] = setting.PostalCode
		}
		if setting.AddressCountry != "" {
			address["addressCountry"] = setting.AddressCountry
		}
		jsonLd["address"] = address
	}

	if len(setting.OpeningDayOfWeek) > 0 {
		openingHours := make([]iris.Map, 0)
		for _, day := range setting.OpeningDayOfWeek {
			hoursSpec := iris.Map{
				"@type":     "OpeningHoursSpecification",
				"dayOfWeek": day,
			}
			if setting.OpeningStartTime != "" && setting.OpeningEndTime != "" {
				hoursSpec["opens"] = setting.OpeningStartTime
				hoursSpec["closes"] = setting.OpeningEndTime
			}
			openingHours = append(openingHours, hoursSpec)
		}
		jsonLd["openingHoursSpecification"] = openingHours
	}

	if setting.PriceRange != "" {
		jsonLd["priceRange"] = setting.PriceRange
	}

	return jsonLd
}

func (w *Website) buildPersonJsonLd() iris.Map {
	setting := w.GetJsonLdSetting()

	jsonLd := iris.Map{
		//	"@context": "https://schema.org",
		"@type": "Person",
	}

	if setting.PersonName != "" {
		jsonLd["name"] = setting.PersonName
	} else {
		jsonLd["name"] = w.System.SiteName
	}

	if setting.PersonJobTitle != "" {
		jsonLd["jobTitle"] = setting.PersonJobTitle
	}

	if setting.PersonImage != "" {
		jsonLd["image"] = setting.PersonImage
	}

	if setting.ContactUrl != "" {
		jsonLd["url"] = setting.ContactUrl
	} else {
		jsonLd["url"] = w.System.BaseUrl
	}

	if len(setting.SocialProfiles) > 0 {
		jsonLd["sameAs"] = setting.SocialProfiles
	}

	return jsonLd
}

func (w *Website) buildBreadcrumbJsonLd(webInfo *response.WebInfo, crumbs []iris.Map) iris.Map {
	breadcrumbList := make([]iris.Map, 0, len(crumbs))
	for i, crumb := range crumbs {
		breadcrumbList = append(breadcrumbList, iris.Map{
			"@type":    "ListItem",
			"position": i + 1,
			"name":     crumb["name"],
			"item":     crumb["link"],
		})
	}

	return iris.Map{
		// "@context":        "https://schema.org",
		"@id":             webInfo.CanonicalUrl + "#breadcrumb",
		"@type":           "BreadcrumbList",
		"itemListElement": breadcrumbList,
	}
}

func (w *Website) buildArchiveDetailJsonLd(viewData map[string]interface{}, webInfo *response.WebInfo) []interface{} {
	var jsonLdList []interface{}
	setting := w.GetJsonLdSetting()

	archive, ok := viewData["archive"].(*model.Archive)
	if !ok {
		return nil
	}

	module := w.GetModuleFromCache(archive.ModuleId)
	var schemaType string

	for _, modSchema := range setting.Module {
		if modSchema.Id == module.Id {
			schemaType = modSchema.SchemaType
			break
		}
	}

	category, _ := viewData["category"].(*model.Category)
	if category != nil {
		for _, catSchema := range setting.Category {
			if catSchema.Id == category.Id {
				if catSchema.SchemaType != "" {
					schemaType = catSchema.SchemaType
				}
				break
			}
		}
	}

	if schemaType == "" {
		if archive.ModuleId == 2 {
			schemaType = "Product"
		} else {
			schemaType = "Article"
		}
	}

	webPage := iris.Map{
		// "@context": "https://schema.org",
		"@type":      "WebPage",
		"@id":        webInfo.CanonicalUrl,
		"url":        webInfo.CanonicalUrl,
		"name":       archive.Title,
		"inLanguage": w.System.Language,
		"mainEntity": iris.Map{"@id": webInfo.CanonicalUrl + "#" + schemaType},
	}

	if setting.IncludeBreadcrumb {
		crumbs := w.buildCrumbs(viewData, webInfo)
		if len(crumbs) > 0 {
			jsonLdList = append(jsonLdList, w.buildBreadcrumbJsonLd(webInfo, crumbs))
			webPage["breadcrumb"] = iris.Map{"@id": webInfo.CanonicalUrl + "#breadcrumb"}
		}
	}

	if setting.DataType == 2 && setting.PersonName != "" {
		jsonLdList = append(jsonLdList, w.buildPersonJsonLd())
	} else {
		jsonLdList = append(jsonLdList, w.buildOrganizationJsonLd())
	}

	detailLd := w.buildDetailJsonLd(schemaType, archive, webInfo, category, viewData)
	jsonLdList = append(jsonLdList, detailLd, webPage)

	return jsonLdList
}

func (w *Website) buildDetailJsonLd(schemaType string, archive *model.Archive, webInfo *response.WebInfo, category *model.Category, viewData map[string]any) iris.Map {
	setting := w.GetJsonLdSetting()

	jsonLd := iris.Map{
		// "@context": "https://schema.org",
		"@id":   webInfo.CanonicalUrl + "#" + schemaType,
		"@type": schemaType,
	}

	switch schemaType {
	case "Product":
		jsonLd["name"] = archive.Title
		jsonLd["sku"] = strconv.Itoa(int(archive.Id))

		if setting.DefaultBrand != "" {
			jsonLd["brand"] = iris.Map{
				"@type": "Brand",
				"name":  setting.DefaultBrand,
			}
		}

		if setting.OrganizationName != "" {
			jsonLd["manufacturer"] = iris.Map{
				"@type": "Organization",
				"name":  setting.OrganizationName,
			}
		}

		if archive.Description != "" {
			jsonLd["description"] = archive.Description
		}

		if len(archive.Images) > 0 {
			jsonLd["image"] = archive.Images
		} else if setting.DefaultImage != "" {
			jsonLd["image"] = setting.DefaultImage
		}

		availability := "https://schema.org/InStock"
		itemCondition := "https://schema.org/NewCondition"
		if archive.Stock <= 0 {
			availability = "https://schema.org/OutOfStock"
		} else if archive.Stock < 10 {
			availability = "https://schema.org/LowStock"
		}

		offers := iris.Map{
			"@type":         "Offer",
			"priceCurrency": "USD",
			"availability":  availability,
			"itemCondition": itemCondition,
			"category":      category.Title,
			"url":           webInfo.CanonicalUrl,
		}
		if archive.Price > 0 {
			offers["price"] = fmt.Sprintf("%.2f", float32(archive.Price)/100.00)
			offers["priceValidUntil"] = time.Now().AddDate(1, 0, 0).Format("2006-01-02")
		}
		if archive.Stock > 0 {
			offers["inventoryLevel"] = iris.Map{
				"@type": "QuantitativeValue",
				"value": archive.Stock,
			}
		}
		jsonLd["offers"] = offers

		if archive.CommentCount > 0 {
			rating := float64((int(archive.CommentCount)+5+int(archive.Id%10))%5)*0.1 + 4.1
			jsonLd["aggregateRating"] = iris.Map{
				"@type":       "AggregateRating",
				"ratingValue": math.Round(rating*10) / 10,
				"reviewCount": archive.CommentCount,
				"bestRating":  5,
				"worstRating": 1,
			}
		}

		if setting.OrganizationName != "" {
			jsonLd["seller"] = iris.Map{
				"@type": "Organization",
				"name":  setting.OrganizationName,
			}
		}

	default:
		if schemaType == "Article" || strings.HasSuffix(schemaType, "Article") || schemaType == "BlogPosting" || schemaType == "NewsArticle" {
			jsonLd["headline"] = archive.Title
		} else {
			jsonLd["name"] = archive.Title
		}
	}

	if archive.Description != "" {
		jsonLd["description"] = archive.Description
	}

	if len(archive.Images) > 0 {
		jsonLd["image"] = archive.Images
	} else if setting.DefaultImage != "" {
		jsonLd["image"] = setting.DefaultImage
	}

	if archive.CreatedTime > 0 {
		jsonLd["datePublished"] = time.Unix(archive.CreatedTime, 0).Format("2006-01-02T15:04:05+08:00")
	}
	if archive.UpdatedTime > 0 {
		jsonLd["dateModified"] = time.Unix(archive.UpdatedTime, 0).Format("2006-01-02T15:04:05+08:00")
	}

	if setting.IncludeAuthor && setting.Author != "" {
		author := iris.Map{
			"@type": "Person",
			"name":  setting.Author,
		}
		if setting.AuthorUrl != "" {
			author["url"] = setting.AuthorUrl
		}
		jsonLd["author"] = author
	}

	if category != nil {
		if schemaType == "Article" || strings.HasSuffix(schemaType, "Article") || schemaType == "BlogPosting" || schemaType == "NewsArticle" {
			jsonLd["articleSection"] = category.Title
		} else if schemaType != "Product" {
			jsonLd["category"] = category.Title
		}
		if len(category.Parents) > 0 {
			jsonLd["genre"] = category.Parents[len(category.Parents)-1].Title
		}
	}

	// 评论
	if setting.IncludeComments {
		comments, _, _ := w.GetCommentList(archive.Id, 0, "id desc", 1, 10, 0)
		if len(comments) > 0 {
			var reviews []interface{}
			for _, comment := range comments {
				if comment.Status != 1 {
					continue
				}
				review := iris.Map{
					"@type":         "Review",
					"reviewBody":    comment.Content,
					"datePublished": time.Unix(comment.CreatedTime, 0).Format("2006-01-02T15:04:05+08:00"),
					"author": iris.Map{
						"@type": "Person",
						"name":  comment.UserName,
					},
				}
				if comment.VoteCount > 0 {
					rating := float64(5.0)
					if comment.VoteCount < 50 {
						rating = 4.0 + float64(comment.VoteCount*2/10)
					}
					review["reviewRating"] = iris.Map{
						"@type":       "Rating",
						"ratingValue": math.Round(rating*10) / 10,
						"bestRating":  5,
						"worstRating": 1,
					}
				}
				reviews = append(reviews, review)
				if len(reviews) >= 5 {
					break
				}
			}
			if len(reviews) > 0 {
				jsonLd["review"] = reviews
			}
		}
	}

	return jsonLd
}

func (w *Website) buildListPageJsonLd(viewData map[string]interface{}, webInfo *response.WebInfo) []interface{} {
	var jsonLdList []interface{}
	setting := w.GetJsonLdSetting()

	listType := "CollectionPage"
	schemaType := "Article"

	module, _ := viewData["module"].(*model.Module)
	if module != nil {
		for _, modSchema := range setting.Module {
			if modSchema.Id == module.Id {
				if modSchema.ListType != "" {
					listType = modSchema.ListType
				}
				if modSchema.SchemaType != "" {
					schemaType = modSchema.SchemaType
				}
				break
			}
		}
	}

	category, _ := viewData["category"].(*model.Category)
	if category != nil {
		for _, catSchema := range setting.Category {
			if catSchema.Id == category.Id {
				if catSchema.ListType != "" {
					listType = catSchema.ListType
				}
				if catSchema.SchemaType != "" {
					schemaType = catSchema.SchemaType
				}
				break
			}
		}
	}

	crumbs := w.buildCrumbs(viewData, webInfo)
	if setting.IncludeBreadcrumb && len(crumbs) > 0 {
		jsonLdList = append(jsonLdList, w.buildBreadcrumbJsonLd(webInfo, crumbs))
	}

	if setting.DataType == 2 && setting.PersonName != "" {
		jsonLdList = append(jsonLdList, w.buildPersonJsonLd())
	} else {
		jsonLdList = append(jsonLdList, w.buildOrganizationJsonLd())
	}

	listLd := iris.Map{
		// "@context": "https://schema.org",
		"@type": listType,
		"name":  webInfo.Title,
		"url":   webInfo.CanonicalUrl,
		"@id":   webInfo.CanonicalUrl,
	}
	if listType == "DetailedItemList" {
		listLd["@type"] = "CollectionPage"
	}

	if webInfo.Description != "" {
		listLd["description"] = webInfo.Description
	}

	listData, ok := viewData["listData"].([]*model.Archive)
	if ok && len(listData) > 0 {
		itemList := w.buildItemList(listData, listType, schemaType)
		if itemList != nil {
			if category != nil {
				itemList["name"] = category.Title
				if category.Description != "" {
					itemList["description"] = category.Description
				}
			}
			listLd["mainEntity"] = itemList
		}
	}

	jsonLdList = append(jsonLdList, listLd)

	return jsonLdList
}

func (w *Website) buildItemList(archives []*model.Archive, listType string, schemaType string) iris.Map {
	if len(archives) == 0 {
		return nil
	}

	itemListElement := make([]iris.Map, 0, len(archives))
	for i, archive := range archives {
		item := iris.Map{
			"@type":    "ListItem",
			"position": i + 1,
		}
		if listType == "DetailedItemList" || listType == "CollectionPage" {
			// 更丰富的结构
			setting := w.GetJsonLdSetting()
			subItem := iris.Map{}
			if schemaType == "Product" {
				subItem["@type"] = "Product"
				subItem["sku"] = strconv.Itoa(int(archive.Id))

				if setting.DefaultBrand != "" {
					subItem["brand"] = iris.Map{
						"@type": "Brand",
						"name":  setting.DefaultBrand,
					}
				}

				if setting.OrganizationName != "" {
					subItem["manufacturer"] = iris.Map{
						"@type": "Organization",
						"name":  setting.OrganizationName,
					}
				}

				availability := "https://schema.org/InStock"
				itemCondition := "https://schema.org/NewCondition"
				if archive.Stock <= 0 {
					availability = "https://schema.org/OutOfStock"
				} else if archive.Stock < 10 {
					availability = "https://schema.org/LowStock"
				}

				offers := iris.Map{
					"@type":         "Offer",
					"priceCurrency": "USD",
					"availability":  availability,
					"itemCondition": itemCondition,
					"url":           archive.Link,
				}
				if archive.Price > 0 {
					offers["price"] = fmt.Sprintf("%.2f", float32(archive.Price)/100.00)
					offers["priceValidUntil"] = time.Now().AddDate(1, 0, 0).Format("2006-01-02")
				}
				if archive.Stock > 0 {
					offers["inventoryLevel"] = iris.Map{
						"@type": "QuantitativeValue",
						"value": archive.Stock,
					}
				}
				subItem["offers"] = offers
				if setting.IncludeComments || archive.CommentCount > 0 {
					rating := float64((int(archive.CommentCount)+5+int(archive.Id%10))%5)*0.1 + 4.1
					subItem["aggregateRating"] = iris.Map{
						"@type":       "AggregateRating",
						"ratingValue": math.Round(rating*10) / 10,
						"reviewCount": int(archive.CommentCount) + 5 + int(archive.Id%10),
						"bestRating":  5,
						"worstRating": 1,
					}
				}
			} else {
				subItem["@type"] = schemaType
			}
			subItem["name"] = archive.Title
			subItem["url"] = archive.Link
			if archive.Description != "" {
				subItem["description"] = archive.Description
			}
			if len(archive.Logo) > 0 {
				subItem["image"] = archive.Logo
			} else if setting.DefaultImage != "" {
				subItem["image"] = setting.DefaultImage
			}
			if archive.CreatedTime > 0 {
				subItem["datePublished"] = time.Unix(archive.CreatedTime, 0).Format("2006-01-02T15:04:05+08:00")
			}
			if archive.UpdatedTime > 0 {
				subItem["dateModified"] = time.Unix(archive.UpdatedTime, 0).Format("2006-01-02T15:04:05+08:00")
			}

			item["item"] = subItem
		} else {
			item["name"] = archive.Title
			item["item"] = archive.Link
		}
		itemListElement = append(itemListElement, item)
	}

	return iris.Map{
		"@type":           "ItemList",
		"numberOfItems":   len(archives),
		"itemListElement": itemListElement,
	}
}

func (w *Website) buildPageDetailJsonLd(viewData map[string]interface{}, webInfo *response.WebInfo) []interface{} {
	var jsonLdList []interface{}
	setting := w.GetJsonLdSetting()

	page, ok := viewData["page"].(*model.Category)
	if !ok {
		return nil
	}

	crumbs := w.buildCrumbs(viewData, webInfo)
	if setting.IncludeBreadcrumb && len(crumbs) > 0 {
		jsonLdList = append(jsonLdList, w.buildBreadcrumbJsonLd(webInfo, crumbs))
	}

	if setting.DataType == 2 && setting.PersonName != "" {
		jsonLdList = append(jsonLdList, w.buildPersonJsonLd())
	} else {
		jsonLdList = append(jsonLdList, w.buildOrganizationJsonLd())
	}

	schemaType := "WebPage"

	for _, catSchema := range setting.Category {
		if catSchema.Id == page.Id {
			if catSchema.SchemaType != "" {
				schemaType = catSchema.SchemaType
			}
			break
		}
	}

	isAboutPage := page.Id == setting.AboutPageId
	isContactPage := page.Id == setting.ContactPageId
	if isAboutPage {
		schemaType = "AboutPage"
	} else if isContactPage {
		schemaType = "ContactPage"
	}

	pageLd := iris.Map{
		// "@context": "https://schema.org",
		"@type": schemaType,
		"name":  page.Title,
		"url":   webInfo.CanonicalUrl,
	}

	if page.Description != "" {
		pageLd["description"] = page.Description
	}

	if len(page.Images) > 0 {
		pageLd["image"] = page.Images
	} else if page.Logo != "" {
		pageLd["image"] = page.Logo
	} else if setting.DefaultImage != "" {
		pageLd["image"] = setting.DefaultImage
	}

	if page.CreatedTime > 0 {
		pageLd["datePublished"] = time.Unix(page.CreatedTime, 0).Format("2006-01-02T15:04:05+08:00")
	}
	if page.UpdatedTime > 0 {
		pageLd["dateModified"] = time.Unix(page.UpdatedTime, 0).Format("2006-01-02T15:04:05+08:00")
	}

	if isAboutPage {
		if setting.DataType == 2 && setting.PersonName != "" {
			pageLd["mainEntity"] = w.buildPersonJsonLd()
		} else {
			pageLd["mainEntity"] = w.buildOrganizationJsonLd()
		}
	}

	if isContactPage || isAboutPage {
		if setting.GeoLatitude != "" && setting.GeoLongitude != "" {
			pageLd["geo"] = iris.Map{
				"@type":     "GeoCoordinates",
				"latitude":  setting.GeoLatitude,
				"longitude": setting.GeoLongitude,
			}
		}
		if setting.StreetAddress != "" || setting.AddressLocality != "" || setting.AddressRegion != "" || setting.PostalCode != "" || setting.AddressCountry != "" {
			address := iris.Map{
				"@type": "PostalAddress",
			}
			if setting.StreetAddress != "" {
				address["streetAddress"] = setting.StreetAddress
			}
			if setting.AddressLocality != "" {
				address["addressLocality"] = setting.AddressLocality
			}
			if setting.AddressRegion != "" {
				address["addressRegion"] = setting.AddressRegion
			}
			if setting.PostalCode != "" {
				address["postalCode"] = setting.PostalCode
			}
			if setting.AddressCountry != "" {
				address["addressCountry"] = setting.AddressCountry
			}
			pageLd["address"] = address
		}
	}

	if isContactPage {
		if setting.ContactNumber != "" {
			pageLd["telephone"] = setting.ContactNumber
		}
		if setting.ContactUrl != "" {
			pageLd["url"] = setting.ContactUrl
		}
		if setting.ContactType != "" {
			pageLd["contactType"] = setting.ContactType
		}
		contactOption := iris.Map{}
		if setting.ContactNumber != "" {
			contactOption["telephone"] = setting.ContactNumber
		}
		if setting.ContactType != "" {
			contactOption["contactType"] = setting.ContactType
		}
		if len(contactOption) > 0 {
			contactOption["@type"] = "ContactPoint"
			pageLd["contactPoint"] = contactOption
		}
	}

	if setting.IncludeAuthor && setting.Author != "" {
		pageLd["author"] = iris.Map{
			"@type": "Person",
			"name":  setting.Author,
		}
	}

	if len(page.Parents) > 0 {
		pageLd["genre"] = page.Parents[len(page.Parents)-1].Title
	}

	pageLd["mainEntityOfPage"] = iris.Map{
		"@type": "WebPage",
		"@id":   webInfo.CanonicalUrl,
	}

	jsonLdList = append(jsonLdList, pageLd)

	return jsonLdList
}

func (w *Website) buildCrumbs(viewData map[string]interface{}, webInfo *response.WebInfo) []iris.Map {
	var crumbs []iris.Map

	crumbs = append(crumbs, iris.Map{
		"name": w.TplTr("Home"),
		"link": w.GetUrl("index", nil, 0),
	})

	switch webInfo.PageName {
	case "archiveIndex":
		if module, ok := viewData["module"].(*model.Module); ok {
			crumbs = append(crumbs, iris.Map{
				"name": module.Title,
				"link": w.GetUrl("archiveIndex", module, 0),
			})
		}
	case "archiveList":
		if category, ok := viewData["category"].(*model.Category); ok {
			if category.Parents != nil {
				for _, parent := range category.Parents {
					parentCat := w.GetCategoryFromCache(parent.Id)
					crumbs = append(crumbs, iris.Map{
						"name": parentCat.Title,
						"link": parentCat.Link,
					})
				}
			}
			crumbs = append(crumbs, iris.Map{
				"name": category.Title,
				"link": w.GetUrl("category", category, 0),
			})
		}
	case "archiveDetail":
		if archive, ok := viewData["archive"].(*model.Archive); ok {
			categories := w.GetParentCategories(archive.CategoryId)
			for _, cat := range categories {
				crumbs = append(crumbs, iris.Map{
					"name": cat.Title,
					"link": w.GetUrl("category", cat, 0),
				})
			}
			crumbs = append(crumbs, iris.Map{
				"name": archive.Title,
				"link": w.GetUrl("archive", archive, 0),
			})
		}
	case "pageDetail":
		if page, ok := viewData["page"].(*model.Category); ok {
			crumbs = append(crumbs, iris.Map{
				"name": page.Title,
				"link": w.GetUrl("page", page, 0),
			})
		}
	}

	return crumbs
}

func (w *Website) buildFaqJsonLd(faqs []*model.Archive, webInfo *response.WebInfo) iris.Map {
	if len(faqs) == 0 {
		return nil
	}

	faqPage := iris.Map{
		"@type":      "FAQPage",
		"@id":        webInfo.CanonicalUrl + "#faq",
		"mainEntity": make([]interface{}, 0, len(faqs)),
	}

	for _, faq := range faqs {
		question := iris.Map{
			"@type": "Question",
			"name":  faq.Title,
			"acceptedAnswer": iris.Map{
				"@type": "Answer",
				"text":  faq.Description,
			},
		}

		faqPage["mainEntity"] = append(faqPage["mainEntity"].([]interface{}), question)
	}

	return faqPage
}
