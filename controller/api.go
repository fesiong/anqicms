package controller

import (
	"encoding/json"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"strconv"
	"strings"
	"time"
)

func ApiImportArchive(ctx iris.Context) {
	if ctx.Method() == "GET" {
		// 用于检查待发布的文章是否存在
		ApiImportGetArchive(ctx)
		return
	}
	currentSite := provider.CurrentSite(ctx)
	id := ctx.PostValueInt64Default("id", 0)
	parentId := ctx.PostValueInt64Default("parent_id", 0)
	title := ctx.PostValueTrim("title")
	seoTitle := ctx.PostValueTrim("seo_title")
	content := ctx.PostValueTrim("content")
	categoryId := uint(ctx.PostValueIntDefault("category_id", 0))
	// 支持分类名称
	categoryTitle := ctx.PostValueTrim("category_title")
	keywords := ctx.PostValueTrim("keywords")
	description := ctx.PostValueTrim("description")
	logo := ctx.PostValueTrim("logo")
	publishTime := ctx.PostValueTrim("publish_time")
	tmpTag := ctx.PostValueTrim("tag")
	images, _ := ctx.PostValues("images[]")
	urlToken := ctx.PostValueTrim("url_token")
	draft, _ := ctx.PostValueBool("draft")
	cover, _ := ctx.PostValueInt("cover") // 0=不覆盖，提示错误，1=覆盖，2=继续插入
	if cover == 0 {
		// 兼容旧的bool
		boolCover, _ := ctx.PostValueBool("cover")
		if boolCover {
			cover = 1
		}
	}
	if len(images) == 1 && strings.HasPrefix(images[0], "[") {
		err := json.Unmarshal([]byte(images[0]), &images)
		if err != nil {
			images = nil
		}
	}
	// 验证 images
	for i := 0; i < len(images); i++ {
		if len(images[i]) < 10 && !(strings.HasPrefix(images[i], "http") || strings.HasPrefix(images[i], "/")) {
			// 删除它
			images = append(images[:i], images[i+1:]...)
			i--
		}
	}
	template := ctx.PostValueTrim("template")
	canonicalUrl := ctx.PostValueTrim("canonical_url")
	fixedLink := ctx.PostValueTrim("fixed_link")
	flag := ctx.PostValueTrim("flag")
	price := ctx.PostValueInt64Default("price", 0)
	stock := ctx.PostValueInt64Default("stock", 0)
	readLevel := ctx.PostValueIntDefault("read_level", 0)
	sort := uint(ctx.PostValueIntDefault("sort", 0))
	originUrl := ctx.PostValueTrim("origin_url")
	tmpCategoryIds, _ := ctx.PostValues("category_ids[]")
	var categoryIds []uint
	if len(tmpCategoryIds) > 0 {
		for i := range tmpCategoryIds {
			tmpCatId, _ := strconv.Atoi(tmpCategoryIds[i])
			if tmpCatId > 0 {
				categoryIds = append(categoryIds, uint(tmpCatId))
			}
		}
	}
	if len(categoryIds) == 0 && categoryId > 0 {
		categoryIds = append(categoryIds, categoryId)
	}
	if len(categoryIds) == 0 {
		// 支持分类名称
		if len(categoryTitle) > 0 {
			category, err := currentSite.GetCategoryByTitle(categoryTitle)
			if err != nil {
				// 分类不存在，创建
				moduleId := uint(ctx.PostValueIntDefault("module_id", 0))
				if moduleId == 0 {
					moduleId = 1
				}
				category, err = currentSite.SaveCategory(&request.Category{
					Title:    categoryTitle,
					ModuleId: moduleId,
					Status:   1,
					Type:     config.CategoryTypeArchive,
				})
			}
			if category != nil {
				categoryIds = append(categoryIds, category.Id)
			}
		}
		if len(categoryIds) == 0 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("PleaseSelectAColumn"),
			})
			return
		}
	}
	for _, catId := range categoryIds {
		category := currentSite.GetCategoryFromCache(catId)
		if category == nil || category.Type != config.CategoryTypeArchive {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("PleaseSelectAColumn"),
			})
			return
		}
	}
	categoryId = categoryIds[0]
	category := currentSite.GetCategoryFromCache(categoryId)
	if category == nil || category.Type != config.CategoryTypeArchive {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseSelectAColumn"),
		})
		return
	}
	module := currentSite.GetModuleFromCache(category.ModuleId)
	if module == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("UndefinedModel"),
		})
		return
	}

	if title == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheArticleTitle"),
		})
		return
	}
	if content == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseFillInTheArticleContent"),
		})
		return
	}

	var req = request.Archive{
		ParentId:     parentId,
		Title:        title,
		SeoTitle:     seoTitle,
		CategoryId:   categoryId,
		CategoryIds:  categoryIds,
		Keywords:     keywords,
		Description:  description,
		Content:      content,
		Template:     template,
		CanonicalUrl: canonicalUrl,
		FixedLink:    fixedLink,
		Flag:         flag,
		Price:        price,
		Stock:        stock,
		ReadLevel:    readLevel,
		Images:       images,
		UrlToken:     urlToken,
		Extra:        map[string]interface{}{},
		Draft:        draft,
		Sort:         sort,
		OriginUrl:    originUrl,
	}

	// 如果传了ID，则采用覆盖的形式
	if id > 0 {
		_, err := currentSite.GetArchiveById(id)
		_, err2 := currentSite.GetArchiveDraftById(id)
		if err != nil && err2 != nil {
			// 不存在，创建一个
			archiveDraft := model.ArchiveDraft{
				Archive: model.Archive{
					ParentId:    parentId,
					Title:       title,
					SeoTitle:    seoTitle,
					UrlToken:    urlToken,
					Keywords:    keywords,
					Description: description,
					ModuleId:    category.ModuleId,
					CategoryId:  categoryId,
					Logo:        logo,
					Price:       price,
					Stock:       stock,
					ReadLevel:   readLevel,
					Sort:        sort,
					OriginUrl:   originUrl,
				},
			}
			archiveDraft.Id = id
			err = currentSite.DB.Create(&archiveDraft).Error

			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("FailedToImportTheArticle"),
				})
				return
			}
			req.Id = id
		} else {
			// 已存在
			if cover == 1 {
				req.Id = id
			} else if cover == 0 {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("DocumentIdIsRepeated"),
				})
				return
			}
		}
	} else {
		// 标题重复的不允许导入
		var existId int64
		exists, err := currentSite.GetArchiveByTitle(title)
		if err == nil {
			existId = exists.Id
		} else {
			// 也需要判断draft表
			exists2, err2 := currentSite.GetArchiveDraftByTitle(title)
			if err2 == nil {
				existId = exists2.Id
			}
		}
		if existId > 0 {
			if cover == 1 {
				req.Id = existId
			} else if cover == 0 {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  ctx.Tr("DocumentTitleIsRepeated"),
				})
				return
			}
		}
		if len(originUrl) > 0 {
			// 标题重复的不允许导入
			exists, err = currentSite.GetArchiveByOriginUrl(originUrl)
			if err == nil {
				existId = exists.Id
			} else {
				// 也需要判断draft表
				exists2, err2 := currentSite.GetArchiveDraftByOriginUrl(title)
				if err2 == nil {
					existId = exists2.Id
				}
			}
			if existId > 0 {
				if cover == 1 {
					req.Id = existId
				} else if cover == 0 {
					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  ctx.Tr("DocumentIsRepeated"),
					})
					return
				}
			}
		}
	}

	if publishTime != "" {
		timeStamp, err := time.ParseInLocation("2006-01-02 15:04:05", publishTime, time.Local)
		if err == nil {
			req.CreatedTime = timeStamp.Unix()
		}
	}
	if logo != "" {
		req.Images = append(req.Images, logo)
	}
	if tmpTag != "" {
		tags := strings.Split(strings.ReplaceAll(tmpTag, "，", ","), ",")
		req.Tags = tags
	}

	// 处理extraFields
	if len(module.Fields) > 0 {
		for _, v := range module.Fields {
			if v.Type == config.CustomFieldTypeCheckbox {
				// 多选值
				value, _ := ctx.PostValues(v.FieldName)
				if len(value) > 0 {
					req.Extra[v.FieldName] = map[string]interface{}{
						"value": value,
					}
				}
			} else if v.Type == config.CustomFieldTypeNumber {
				value := ctx.PostValueIntDefault(v.FieldName, 0)
				if value > 0 {
					req.Extra[v.FieldName] = map[string]interface{}{
						"value": value,
					}
				}
			} else {
				value := ctx.PostValue(v.FieldName)
				if value != "" {
					req.Extra[v.FieldName] = map[string]interface{}{
						"value": value,
					}
				}
			}
		}
	}

	archive, err := currentSite.SaveArchive(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("PublishSuccessfully"),
		"data": iris.Map{
			"url": currentSite.GetUrl("archive", archive, 0),
			"id":  archive.Id,
		},
	})
}

func ApiImportGetArchive(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	id := ctx.URLParamInt64Default("id", 0)
	title := ctx.URLParam("title")
	urlToken := ctx.URLParam("url_token")
	originUrl := ctx.URLParam("origin_url")

	if id > 0 {
		archive, err := currentSite.GetArchiveById(id)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archive,
			})
			return
		}
	}
	if len(title) > 0 {
		archive, err := currentSite.GetArchiveByTitle(title)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archive,
			})
			return
		}
	}
	if len(urlToken) > 0 {
		archive, err := currentSite.GetArchiveByUrlToken(urlToken)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archive,
			})
			return
		}
	}
	if len(originUrl) > 0 {
		archive, err := currentSite.GetArchiveByOriginUrl(originUrl)
		if err == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"data": archive,
			})
			return
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusFailed,
		"msg":  ctx.Tr("DocumentDoesNotExist"),
	})
	return
}

func ApiImportGetCategories(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	tmpModuleId := ctx.FormValue("module_id")
	moduleId, _ := strconv.Atoi(tmpModuleId)
	showType, _ := strconv.Atoi(ctx.FormValue("show_type"))

	if moduleId > 0 {
		module := currentSite.GetModuleFromCache(uint(moduleId))

		if module == nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("UndefinedModel"),
			})
			return
		}
	}

	tmpCategories, _ := currentSite.GetCategories(func(tx *gorm.DB) *gorm.DB {
		tx = tx.Where("`type` = ? and `status` = ?", config.CategoryTypeArchive, config.ContentStatusOK)
		if moduleId > 0 {
			tx = tx.Where("`module_id` = ?", moduleId)
		}
		return tx
	}, 0, showType)

	var categories []response.ApiCategory
	for i := range tmpCategories {
		categories = append(categories, response.ApiCategory{
			Id:       tmpCategories[i].Id,
			ParentId: tmpCategories[i].ParentId,
			Title:    tmpCategories[i].Title,
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusApiSuccess,
		"msg":  ctx.Tr("SuccessfulAcquisition"),
		"data": categories,
	})
}

// ApiImportMakeSitemap 通过API接口生成Sitemap
func ApiImportMakeSitemap(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	async := ctx.FormValue("async")

	if async == "true" || async == "1" {
		go func() {
			err := currentSite.BuildSitemap()
			if err == nil {
				pluginSitemap := currentSite.PluginSitemap

				//由于sitemap的更新可能很频繁，因此sitemap的更新时间直接写入一个文件中
				pluginSitemap.UpdatedTime = currentSite.GetSitemapTime()
				// 写入Sitemap的url
				pluginSitemap.SitemapURL = currentSite.System.BaseUrl + "/sitemap." + pluginSitemap.Type

				currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSitemapManually"))
			}
		}()

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("SubmittedForBackgroundProcessing"),
		})
		return
	}
	//开始生成sitemap
	err := currentSite.BuildSitemap()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	pluginSitemap := currentSite.PluginSitemap

	//由于sitemap的更新可能很频繁，因此sitemap的更新时间直接写入一个文件中
	pluginSitemap.UpdatedTime = currentSite.GetSitemapTime()
	// 写入Sitemap的url
	pluginSitemap.SitemapURL = currentSite.System.BaseUrl + "/sitemap." + pluginSitemap.Type

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSitemapManually"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SitemapUpdated"),
		"data": pluginSitemap,
	})
}
func ApiImportCreateFriendLink(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	// 增加支持 didi 友链的批量导入
	form := library.NewForm(ctx.Request().Form)
	var otherList []map[string]string
	err := form.Bind(&otherList, "other_list")
	if err == nil && len(otherList) > 0 {
		for _, item := range otherList {
			friendLink, err := currentSite.GetLinkByLink(item["url"])
			if err != nil {
				friendLink = &model.Link{}
			}
			friendLink.Title = item["name"]
			friendLink.Link = item["url"]
			friendLink.Contact = item["qq"]
			friendLink.Status = 0
			friendLink.Save(currentSite.DB)
		}

		currentSite.DeleteCacheIndex()

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("LinkSaved"),
		})
		return
	}

	title := ctx.PostValueTrim("title")
	link := ctx.PostValueTrim("link")
	if linkUrl := ctx.PostValueTrim("url"); linkUrl != "" {
		link = linkUrl
	}
	nofollow := uint(ctx.PostValueIntDefault("nofollow", 0))
	backLink := ctx.PostValueTrim("back_link")
	myTitle := ctx.PostValueTrim("my_title")
	myLink := ctx.PostValueTrim("my_link")
	contact := ctx.PostValueTrim("contact")
	if qq := ctx.PostValueTrim("qq"); qq != "" {
		contact = qq
	}
	if email := ctx.PostValueTrim("email"); email != "" {
		contact = email
	}
	remark := ctx.PostValueTrim("remark")

	friendLink, err := currentSite.GetLinkByLink(link)
	if err != nil {
		friendLink = &model.Link{
			Status: 0,
		}
	}

	friendLink.Title = title
	friendLink.Link = link
	if backLink != "" {
		friendLink.BackLink = backLink
	}
	if myTitle != "" {
		friendLink.MyTitle = myTitle
	}
	if myLink != "" {
		friendLink.MyLink = myLink
	}
	if contact != "" {
		friendLink.Contact = contact
	}
	if remark != "" {
		friendLink.Remark = remark
	}
	friendLink.Nofollow = nofollow
	friendLink.Status = 0

	err = friendLink.Save(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 保存完毕，实时监测
	go currentSite.PluginLinkCheck(friendLink)

	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LinkSaved"),
	})
}

func ApiImportDeleteFriendLink(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	link := ctx.PostValueTrim("link")
	if linkUrl := ctx.PostValueTrim("url"); linkUrl != "" {
		link = linkUrl
	}

	if link == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("LinkRequired"),
		})
		return
	}

	friendLink, err := currentSite.GetLinkByLink(link)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("LinkDoesNotExist"),
		})
		return
	}

	err = friendLink.Delete(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("LinkDeleted"),
	})
}

func ApiImportCheckFriendLink(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("VerificationSuccessful"),
	})
}

func VerifyApiToken(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	token := ctx.FormValue("token")
	if token != currentSite.PluginImportApi.Token {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("TokenError"),
		})
		return
	}

	ctx.Next()
}

func VerifyApiLinkToken(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	token := ctx.FormValue("token")
	if didiToken := ctx.GetHeader("didi-token"); didiToken != "" {
		token = didiToken
	}
	if token != currentSite.PluginImportApi.LinkToken {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("TokenError"),
		})
		return
	}

	ctx.Next()
}

func CheckApiOpen(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if 1 != currentSite.Safe.APIOpen {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("ApiInterfaceFunctionIsNotOpen"),
		})
		return
	}

	ctx.Next()
}
