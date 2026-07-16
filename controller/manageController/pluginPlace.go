package manageController

import (
	"encoding/json"
	"fmt"

	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
)

func PluginGetPlaceSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginPlace

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginSavePlaceSetting(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginPlaceConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	w2 := provider.GetWebsite(currentSite.Id)
	w2.PluginPlace = &req
	err := currentSite.SaveSettingValue(provider.PlaceSettingKey, w2.PluginPlace)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, currentSite.Tr("UpdatePlaceConfiguration"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginPlaceList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	showType := ctx.URLParamIntDefault("show_type", 0)
	parentId := uint(ctx.URLParamIntDefault("parent_id", 0))
	title := ctx.URLParam("title")

	var places []*model.Place
	var err error
	var ops func(tx *gorm.DB) *gorm.DB
	ops = func(tx *gorm.DB) *gorm.DB {
		tx = tx.Order("sort asc")
		if title != "" {
			tx = tx.Where("`title` like ?", "%"+title+"%")
		}
		return tx
	}
	// 搜索模式下，不构建tree
	if title != "" {
		showType = config.CategoryShowTypeList
	}
	places, err = currentSite.GetPlaces(ops, parentId, showType)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	for i := range places {
		places[i].Link = currentSite.GetUrl("place", places[i], 0)
		// 计算template
		if places[i].Template == "" {
			places[i].Template = ctx.Tr("(Default)") + "place/detail.html"
			// 跟随上级
			if places[i].ParentId > 0 {
				placeTemplate := currentSite.GetPlaceTemplate(places[i])
				if placeTemplate != "" {
					places[i].Template = ctx.Tr("(Inherited)") + placeTemplate
				}
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": places,
	})
}

func PlaceDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	place, err := currentSite.GetPlaceById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	place.Content = currentSite.ReplaceContentUrl(place.Content, true)
	// extra replace
	if place.Extra != nil {
		placeFields := currentSite.PluginPlace.Fields
		if len(placeFields) > 0 {
			for _, field := range placeFields {
				if (field.Type == config.CustomFieldTypeImage || field.Type == config.CustomFieldTypeFile || field.Type == config.CustomFieldTypeEditor) &&
					place.Extra[field.FieldName] != nil {
					place.Extra[field.FieldName] = currentSite.ReplaceContentUrl(place.Extra[field.FieldName].(string), true)
				}
				if field.Type == config.CustomFieldTypeImages && place.Extra[field.FieldName] != nil {
					if val, ok := place.Extra[field.FieldName].([]interface{}); ok {
						for j, v2 := range val {
							v2s, _ := v2.(string)
							val[j] = currentSite.ReplaceContentUrl(v2s, true)
						}
						place.Extra[field.FieldName] = val
					}
				} else if field.Type == config.CustomFieldTypeTexts && place.Extra[field.FieldName] != nil {
					var texts []config.CustomFieldTexts
					_ = json.Unmarshal([]byte(fmt.Sprint(place.Extra[field.FieldName])), &texts)
					place.Extra[field.FieldName] = texts
				} else if field.Type == config.CustomFieldTypeTimeline && place.Extra[field.FieldName] != nil {
					var val config.TimelineField
					_ = json.Unmarshal([]byte(fmt.Sprint(place.Extra[field.FieldName])), &val)
					place.Extra[field.FieldName] = val
				}
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": place,
	})
}

func PlaceDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req model.Place
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	place, err := currentSite.SavePlace(&req)
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
			if sub.Id == currentSite.Id || sub.Id == 0 {
				continue
			}
			// 同步分类，先同步，再添加翻译计划
			subSite := provider.GetWebsite(sub.Id)
			if subSite != nil && subSite.Initialed {
				// 插入记录
				if req.Id == 0 {
					req.Id = place.Id
					subPlace, err := subSite.SavePlace(&req)
					if err == nil {
						// 同步成功，进行翻译
						if currentSite.MultiLanguage.AutoTranslate {
							transReq := &provider.AnqiTranslateTextRequest{
								Text: []string{
									subPlace.Title,       // 0
									subPlace.Description, // 1
									subPlace.Keywords,    // 2
									subPlace.Content,     // 3
								},
								Language:   currentSite.System.Language,
								ToLanguage: subSite.System.Language,
							}
							res, err := currentSite.AnqiTranslateString(transReq)
							if err == nil {
								// 只处理成功的结果
								subSite.DB.Model(subPlace).UpdateColumns(map[string]interface{}{
									"title":       res.Text[0],
									"description": res.Text[1],
									"keywords":    res.Text[2],
									"content":     res.Text[3],
								})
							}
						}
					}
				} else {
					// 修改的话，就排除 title, content，description，keywords 字段
					tmpPlace, err := subSite.GetPlaceById(req.Id)
					if err == nil {
						req.Title = tmpPlace.Title
						req.Content = tmpPlace.Content
						req.Description = tmpPlace.Description
						req.Keywords = tmpPlace.Keywords
					}
					_, _ = subSite.SavePlace(&req)
				}
			}
		}
	}

	// 更新缓存
	go func() {
		currentSite.BuildModuleCache(ctx)
		currentSite.BuildSinglePlaceCache(ctx, place)
		// 上传到静态服务器
		_ = currentSite.SyncHtmlCacheToStorage("", "")
	}()

	currentSite.AddAdminLog(ctx, ctx.Tr("SaveDocumentCategoryLog", place.Id, place.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": place,
	})
}

func PlaceDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req model.Place
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	place, err := currentSite.GetPlaceById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.DeletePlace(place)
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
			if sub.Id == currentSite.Id || sub.Id == 0 {
				continue
			}
			// 同步分类，先同步，再添加翻译计划
			subSite := provider.GetWebsite(sub.Id)
			if subSite != nil && subSite.Initialed {
				// 同步删除
				_ = subSite.DeletePlace(place)
			}
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeletePlaceLog", place.Id, place.Title))

	currentSite.DeleteCachePlaces()
	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("PlaceDeleted"),
	})
}
