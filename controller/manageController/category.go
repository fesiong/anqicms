package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func CategoryList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	moduleId := uint(ctx.URLParamIntDefault("module_id", 0))
	categoryType := uint(ctx.URLParamIntDefault("type", 0))
	showType := ctx.URLParamIntDefault("show_type", 0)
	title := ctx.URLParam("title")

	var categories []*model.Category
	var err error
	var ops func(tx *gorm.DB) *gorm.DB
	if categoryType == config.CategoryTypePage {
		ops = func(tx *gorm.DB) *gorm.DB {
			tx = tx.Where("`type` = ?", config.CategoryTypePage).Order("sort asc")
			if title != "" {
				tx = tx.Where("`title` like ?", "%"+title+"%")
			}
			return tx
		}
	} else {
		ops = func(tx *gorm.DB) *gorm.DB {
			tx = tx.Where("`type` = ?", config.CategoryTypeArchive).Order("module_id asc,sort asc")
			if moduleId > 0 {
				tx = tx.Where("`module_id` = ?", moduleId)
			}
			if title != "" {
				tx = tx.Where("`title` like ?", "%"+title+"%")
			}
			return tx
		}
	}
	categories, err = currentSite.GetCategories(ops, 0, showType)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	modules, _ := currentSite.GetModules()
	var mapModules = make(map[uint]model.Module)
	for i := range modules {
		mapModules[modules[i].Id] = modules[i]
	}

	for i := range categories {
		categories[i].Link = currentSite.GetUrl("category", categories[i], 0)
		// 计算template
		if categories[i].Template == "" {
			// 默认的
			if categories[i].Type == config.CategoryTypePage {
				categories[i].Template = ctx.Tr("(Default)") + " page/detail.html"
				if controller.ViewExists(ctx, "page_detail.html") {
					categories[i].Template = ctx.Tr("(Default)") + " page_detail.html"
				}
				tmpTpl := fmt.Sprintf("page/detail-%d.html", categories[i].Id)
				if controller.ViewExists(ctx, tmpTpl) {
					categories[i].Template = "(ID) " + tmpTpl
				} else if controller.ViewExists(ctx, fmt.Sprintf("page-%d.html", categories[i].Id)) {
					categories[i].Template = "(ID) " + fmt.Sprintf("page-%d.html", categories[i].Id)
				} else {
					categoryTemplate := currentSite.GetCategoryTemplate(categories[i])
					if categoryTemplate != nil {
						categories[i].Template = categoryTemplate.Template
					}
				}
			} else {
				categories[i].Template = ctx.Tr("(Default)") + mapModules[categories[i].ModuleId].TableName + "/list.html"
				tplName2 := mapModules[categories[i].ModuleId].TableName + "_list.html"
				if controller.ViewExists(ctx, tplName2) {
					categories[i].Template = ctx.Tr("(Default)") + tplName2
				}
				if controller.ViewExists(ctx, fmt.Sprintf("%s/list-%d.html", mapModules[categories[i].ModuleId].TableName, categories[i].Id)) {
					categories[i].Template = "(ID) " + fmt.Sprintf("%s/list-%d.html", mapModules[categories[i].ModuleId].TableName, categories[i].Id)
				}
				// 跟随上级
				if categories[i].ParentId > 0 {
					categoryTemplate := currentSite.GetCategoryTemplate(categories[i])
					if categoryTemplate != nil && len(categoryTemplate.Template) > 0 {
						categories[i].Template = ctx.Tr("(Inherited)") + categoryTemplate.Template
					}
					if categories[i].DetailTemplate == "" && categoryTemplate != nil && len(categoryTemplate.DetailTemplate) > 0 {
						categories[i].DetailTemplate = ctx.Tr("(Inherited)") + categoryTemplate.DetailTemplate
					}
				}
			}
		}
		// 计算内容template
		if categories[i].DetailTemplate == "" && categories[i].Type != config.CategoryTypePage {
			categories[i].DetailTemplate = ctx.Tr("(Default)") + mapModules[categories[i].ModuleId].TableName + "/detail.html"
			tplName2 := mapModules[categories[i].ModuleId].TableName + "_detail.html"
			if controller.ViewExists(ctx, tplName2) {
				categories[i].DetailTemplate = ctx.Tr("(Default)") + tplName2
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": categories,
	})
}

func CategoryDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))

	category, err := currentSite.GetCategoryById(id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": category,
	})
}

func CategoryDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.Category
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	category, err := currentSite.SaveCategory(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 如果开启了多语言，则自动同步文章,分类
	if currentSite.MultiLanguage.Open {
		for _, subSiteID := range currentSite.MultiLanguage.SubSites {
			// 同步分类，先同步，再添加翻译计划
			subSite := provider.GetWebsite(subSiteID)
			if subSite != nil && subSite.Initialed {
				// 插入记录
				if req.Id == 0 {
					req.Id = category.Id
					subCategory, err := subSite.SaveCategory(&req)
					if err == nil {
						// 同步成功，进行翻译
						if currentSite.MultiLanguage.AutoTranslate {
							transReq := provider.AnqiAiRequest{
								Title:      subCategory.Title,
								Content:    subCategory.Content,
								Language:   currentSite.System.Language,
								ToLanguage: subSite.System.Language,
								Async:      false, // 同步返回结果
							}
							res, err := currentSite.AnqiTranslateString(&transReq)
							if err == nil {
								// 只处理成功的结果
								subSite.DB.Model(subCategory).UpdateColumns(map[string]interface{}{
									"title":   res.Title,
									"content": res.Content,
								})
							}
							if len(category.Description) > 0 {
								transReq = provider.AnqiAiRequest{
									Title:      "",
									Content:    category.Description,
									Language:   currentSite.System.Language,
									ToLanguage: subSite.System.Language,
									Async:      false, // 同步返回结果
								}
								res, err = currentSite.AnqiTranslateString(&transReq)
								if err == nil {
									// 只处理成功的结果
									subSite.DB.Model(&category).UpdateColumns(map[string]interface{}{
										"description": res.Content,
									})
								}
							}
						}
					}
				} else {
					// 修改的话，就排除 title, content，description，keywords 字段
					tmpCategory, err := subSite.GetCategoryById(req.Id)
					if err == nil {
						req.Title = tmpCategory.Title
						req.Content = tmpCategory.Content
						req.Description = tmpCategory.Description
						req.Keywords = tmpCategory.Keywords
					}
					_, _ = subSite.SaveCategory(&req)
				}
			}
		}
	}

	// 更新缓存
	go func() {
		currentSite.BuildModuleCache(ctx)
		currentSite.BuildSingleCategoryCache(ctx, category)
		// 上传到静态服务器
		_ = currentSite.SyncHtmlCacheToStorage("", "")
	}()

	currentSite.AddAdminLog(ctx, ctx.Tr("SaveDocumentCategoryLog", category.Id, category.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
		"data": category,
	})
}

func CategoryDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.Category
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	category, err := currentSite.GetCategoryById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = category.Delete(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 如果开启了多语言，则自动同步文章,分类
	if currentSite.MultiLanguage.Open {
		for _, subSiteID := range currentSite.MultiLanguage.SubSites {
			// 同步分类，先同步，再添加翻译计划
			subSite := provider.GetWebsite(subSiteID)
			if subSite != nil && subSite.Initialed {
				// 同步删除
				_ = category.Delete(subSite.DB)
			}
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteDocumentCategoryLog", category.Id, category.Title))

	currentSite.DeleteCacheCategories()
	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CategoryDeleted"),
	})
}

func CategoryUpdateArchiveCount(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	currentSite.UpdateCategoryArchiveCounts()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SaveSuccessfully"),
	})
}
