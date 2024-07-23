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
	currentSite := provider.CurrentSite(ctx)
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
				categories[i].Template = ctx.Tr("(默认)") + " page/detail.html"
				if controller.ViewExists(ctx, "page_detail.html") {
					categories[i].Template = ctx.Tr("(默认)") + " page_detail.html"
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
				categories[i].Template = ctx.Tr("(默认)") + mapModules[categories[i].ModuleId].TableName + "/list.html"
				tplName2 := mapModules[categories[i].ModuleId].TableName + "_list.html"
				if controller.ViewExists(ctx, tplName2) {
					categories[i].Template = ctx.Tr("(默认)") + tplName2
				}
				if controller.ViewExists(ctx, fmt.Sprintf("%s/list-%d.html", mapModules[categories[i].ModuleId].TableName, categories[i].Id)) {
					categories[i].Template = "(ID) " + fmt.Sprintf("%s/list-%d.html", mapModules[categories[i].ModuleId].TableName, categories[i].Id)
				}
				// 跟随上级
				if categories[i].ParentId > 0 {
					categoryTemplate := currentSite.GetCategoryTemplate(categories[i])
					if categoryTemplate != nil && len(categoryTemplate.Template) > 0 {
						categories[i].Template = ctx.Tr("(继承)") + categoryTemplate.Template
					}
					if categories[i].DetailTemplate == "" && categoryTemplate != nil && len(categoryTemplate.DetailTemplate) > 0 {
						categories[i].DetailTemplate = ctx.Tr("(继承)") + categoryTemplate.DetailTemplate
					}
				}
			}
		}
		// 计算内容template
		if categories[i].DetailTemplate == "" && categories[i].Type != config.CategoryTypePage {
			categories[i].DetailTemplate = ctx.Tr("(默认)") + mapModules[categories[i].ModuleId].TableName + "/detail.html"
			tplName2 := mapModules[categories[i].ModuleId].TableName + "_detail.html"
			if controller.ViewExists(ctx, tplName2) {
				categories[i].DetailTemplate = ctx.Tr("(默认)") + tplName2
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
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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
	// 更新缓存
	go func() {
		currentSite.BuildModuleCache(ctx)
		currentSite.BuildSingleCategoryCache(ctx, category)
		// 上传到静态服务器
		_ = currentSite.SyncHtmlCacheToStorage("", "")
	}()

	currentSite.AddAdminLog(ctx, ctx.Tr("保存文档分类：%d => %s", category.Id, category.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("保存成功"),
		"data": category,
	})
}

func CategoryDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, ctx.Tr("删除文档分类：%d => %s", category.Id, category.Title))

	currentSite.DeleteCacheCategories()
	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("分类已删除"),
	})
}
