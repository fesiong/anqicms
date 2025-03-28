package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func PluginTagList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	title := ctx.URLParam("title")
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))
	var categoryIds []uint
	if categoryId > 0 {
		categoryIds = append(categoryIds, categoryId)
	}
	tags, total, err := currentSite.GetTagList(0, title, categoryIds, "", currentPage, pageSize, 0, "id desc")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 生成链接
	for i := range tags {
		tags[i].Link = currentSite.GetUrl("tag", tags[i], 0)
		tags[i].GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
		// categoryTitle
		if tags[i].CategoryId > 0 {
			category := currentSite.GetCategoryFromCache(tags[i].CategoryId)
			if category != nil {
				tags[i].CategoryTitle = category.Title
			}
		}
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  tags,
	})
}

func PluginTagDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	id := ctx.URLParamIntDefault("id", 0)

	tag, err := currentSite.GetTagById(uint(id))
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	tag.GetThumb(currentSite.PluginStorage.StorageUrl, currentSite.Content.DefaultThumb)
	tagContent, err := currentSite.GetTagContentById(tag.Id)
	if err == nil {
		tag.Content = tagContent.Content
		tag.Extra = tagContent.Extra
		if tag.Extra != nil {
			fields := currentSite.GetTagFields()
			if len(fields) > 0 {
				for _, field := range fields {
					if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
						tag.Extra[field.FieldName] != nil {
						tag.Extra[field.FieldName] = currentSite.ReplaceContentUrl(tag.Extra[field.FieldName].(string), true)
					}
					if field.Type == config.CustomFieldTypeImages && tag.Extra[field.FieldName] != nil {
						if val, ok := tag.Extra[field.FieldName].([]interface{}); ok {
							for j, v2 := range val {
								v2s, _ := v2.(string)
								val[j] = currentSite.ReplaceContentUrl(v2s, true)
							}
							tag.Extra[field.FieldName] = val
						}
					}
				}
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": tag,
	})
}

func PluginTagDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginTag
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	tag, err := currentSite.SaveTag(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 如果开启了多语言，则自动同步文章,分类
	if currentSite.MultiLanguage.Open {
		for _, sub := range currentSite.MultiLanguage.SubSites {
			// 同步分类，先同步，再添加翻译计划
			subSite := provider.GetWebsite(sub.Id)
			if subSite != nil && subSite.Initialed {
				// 插入记录
				if req.Id == 0 {
					req.Id = tag.Id
					subTag, err := subSite.SaveTag(&req)
					if err == nil {
						// 同步成功，进行翻译
						if currentSite.MultiLanguage.AutoTranslate {
							var content string
							tagContent, _ := subSite.GetTagContentById(subTag.Id)
							if tagContent != nil {
								content = tagContent.Content
							}
							transReq := &provider.AnqiTranslateTextRequest{
								Text: []string{
									subTag.Title,       // 0
									subTag.Description, // 1
									subTag.Keywords,    // 2
									content,            // 3
								},
								Language:   currentSite.System.Language,
								ToLanguage: subSite.System.Language,
							}
							res, err := currentSite.AnqiTranslateString(transReq)
							if err == nil {
								// 只处理成功的结果
								subSite.DB.Model(subTag).UpdateColumns(map[string]interface{}{
									"title":       res.Text[0],
									"description": res.Text[1],
									"keywords":    res.Text[2],
								})
								if res.Text[3] != "" {
									subSite.DB.Model(&model.TagContent{}).Where("id = ?", subTag.Id).UpdateColumn("content", res.Text[3])
								}
							}
						}
					}
				} else {
					// 修改的话，就排除 title, seo_title，description，keywords 字段
					tmpTag, err := subSite.GetTagById(req.Id)
					if err == nil {
						req.Title = tmpTag.Title
						req.SeoTitle = tmpTag.SeoTitle
						req.Description = tmpTag.Description
						req.Keywords = tmpTag.Keywords
						req.Content = tmpTag.Content
					}
					_, _ = subSite.SaveTag(&req)
				}
			}
		}
	}
	// 更新缓存
	go func() {
		currentSite.BuildTagIndexCache(ctx)
		currentSite.BuildSingleTagCache(ctx, tag)
		// 上传到静态服务器
		_ = currentSite.SyncHtmlCacheToStorage("", "")
	}()

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateDocumentTagLog", tag.Id, tag.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": tag,
	})
}

func PluginTagDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginTag
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	tag, err := currentSite.GetTagById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.DeleteTag(tag.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 如果开启了多语言，则自动同步文章,分类
	if currentSite.MultiLanguage.Open {
		for _, sub := range currentSite.MultiLanguage.SubSites {
			// 同步分类，先同步，再添加翻译计划
			subSite := provider.GetWebsite(sub.Id)
			if subSite != nil && subSite.Initialed {
				// 同步删除
				_ = subSite.DeleteTag(tag.Id)
			}
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteDocumentTagLog", tag.Id, tag.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("TagDeleted"),
	})
}

func PluginTagFields(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	fields := currentSite.GetTagFields()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": fields,
	})
}

func PluginTagFieldsForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req []config.CustomField
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if err := currentSite.SaveTagFields(req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}
