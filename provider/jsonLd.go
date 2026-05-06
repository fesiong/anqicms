package provider

import (
	"encoding/json"
	"fmt"
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

	if len(jsonLdList) > 0 {
		buf, _ := json.MarshalIndent(jsonLdList, "", "\t")
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
		"@context": "https://schema.org",
		"@type":    orgType,
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

	if setting.ContactNumber != "" {
		jsonLd["telephone"] = setting.ContactNumber
	}

	if setting.ContactUrl != "" {
		jsonLd["contactUrl"] = setting.ContactUrl
	}

	if setting.ContactType != "" {
		jsonLd["contactType"] = setting.ContactType
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
		"@context": "https://schema.org",
		"@type":    "Person",
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

func (w *Website) buildBreadcrumbJsonLd(crumbs []iris.Map) iris.Map {
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
		"@context":        "https://schema.org",
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

	crumbs := w.buildCrumbs(viewData, webInfo)
	if setting.IncludeBreadcrumb && len(crumbs) > 0 {
		jsonLdList = append(jsonLdList, w.buildBreadcrumbJsonLd(crumbs))
	}

	if setting.DataType == 2 && setting.PersonName != "" {
		jsonLdList = append(jsonLdList, w.buildPersonJsonLd())
	} else {
		jsonLdList = append(jsonLdList, w.buildOrganizationJsonLd())
	}

	detailLd := w.buildDetailJsonLd(schemaType, archive, webInfo, category, viewData)
	jsonLdList = append(jsonLdList, detailLd)

	return jsonLdList
}

func (w *Website) buildDetailJsonLd(schemaType string, archive *model.Archive, webInfo *response.WebInfo, category *model.Category, viewData map[string]any) iris.Map {
	setting := w.GetJsonLdSetting()

	jsonLd := iris.Map{
		"@context": "https://schema.org",
		"@type":    schemaType,
	}

	switch schemaType {
	case "Product":
		jsonLd["name"] = archive.Title

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
			"url":           webInfo.CanonicalUrl,
		}
		if archive.Price > 0 {
			offers["price"] = float32(archive.Price) / 100.00
			offers["priceValidUntil"] = time.Now().AddDate(1, 0, 0).Format("2006-01-02")
		}
		if archive.Stock > 0 {
			offers["inventoryLevel"] = iris.Map{
				"@type": "QuantitativeValue",
				"value": archive.Stock,
			}
		}
		jsonLd["offers"] = offers

		rating := float32((archive.CommentCount+2)%5)*0.1 + 4.1
		jsonLd["aggregateRating"] = iris.Map{
			"@type":       "AggregateRating",
			"ratingValue": fmt.Sprintf("%.1f", rating),
			"reviewCount": archive.CommentCount,
			"bestRating":  5,
			"worstRating": 1,
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
		jsonLd["articleSection"] = category.Title
		if len(category.ParentTitles) > 0 {
			jsonLd["genre"] = category.ParentTitles[len(category.ParentTitles)-1]
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
					rating := float32(5)
					if comment.VoteCount < 10 {
						rating = 4.0
					} else if comment.VoteCount < 50 {
						rating = 4.5
					}
					review["reviewRating"] = iris.Map{
						"@type":       "Rating",
						"ratingValue": fmt.Sprintf("%.1f", rating),
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

	jsonLd["mainEntityOfPage"] = iris.Map{
		"@type": "WebPage",
		"@id":   webInfo.CanonicalUrl,
	}

	return jsonLd
}

func (w *Website) buildListPageJsonLd(viewData map[string]interface{}, webInfo *response.WebInfo) []interface{} {
	var jsonLdList []interface{}
	setting := w.GetJsonLdSetting()

	listType := "CollectionPage‌"
	schemaType := "ItemList"

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
		jsonLdList = append(jsonLdList, w.buildBreadcrumbJsonLd(crumbs))
	}

	if setting.DataType == 2 && setting.PersonName != "" {
		jsonLdList = append(jsonLdList, w.buildPersonJsonLd())
	} else {
		jsonLdList = append(jsonLdList, w.buildOrganizationJsonLd())
	}

	listLd := iris.Map{
		"@context": "https://schema.org",
		"@type":    listType,
		"name":     webInfo.Title,
		"url":      webInfo.CanonicalUrl,
	}

	if webInfo.Description != "" {
		listLd["description"] = webInfo.Description
	}

	listData, ok := viewData["listData"].([]*model.Archive)
	if ok && len(listData) > 0 {
		itemList := w.buildItemList(listData, schemaType)
		if itemList != nil {
			listLd["mainEntityOfPage"] = itemList
		}
	}

	jsonLdList = append(jsonLdList, listLd)

	return jsonLdList
}

func (w *Website) buildItemList(archives []*model.Archive, schemaType string) iris.Map {
	if len(archives) == 0 {
		return nil
	}

	itemListElement := make([]iris.Map, 0, len(archives))
	for i, archive := range archives {
		item := iris.Map{
			"@type":    "ListItem",
			"position": i + 1,
			"name":     archive.Title,
			"url":      archive.Link,
		}
		if archive.Description != "" {
			item["description"] = archive.Description
		}
		if archive.Logo != "" {
			item["image"] = archive.Logo
		}
		itemListElement = append(itemListElement, item)
	}

	listType := "ItemList"
	if schemaType == "DetailedItemList" {
		listType = "DetailedItemList"
	}

	return iris.Map{
		"@type":           listType,
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
		jsonLdList = append(jsonLdList, w.buildBreadcrumbJsonLd(crumbs))
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
		"@context": "https://schema.org",
		"@type":    schemaType,
		"name":     page.Title,
		"url":      webInfo.CanonicalUrl,
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

	if len(page.ParentTitles) > 0 {
		pageLd["genre"] = page.ParentTitles[len(page.ParentTitles)-1]
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
		"link": "/",
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
			if category.ParentTitles != nil {
				for _, parentTitle := range category.ParentTitles {
					crumbs = append(crumbs, iris.Map{
						"name": parentTitle,
						"link": "",
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
				"link": "",
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
