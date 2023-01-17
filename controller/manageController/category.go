package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func CategoryList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	moduleId := uint(ctx.URLParamIntDefault("module_id", 0))
	categoryType := uint(ctx.URLParamIntDefault("type", 0))
	title := ctx.URLParam("title")

	var categories []*model.Category
	var err error
	var ops func(tx *gorm.DB) *gorm.DB
	if categoryType == config.CategoryTypePage {
		ops = func(tx *gorm.DB) *gorm.DB {
			tx = tx.Where("`type` = ? and `status` = ?", config.CategoryTypePage, 1).Order("sort asc")
			if title != "" {
				tx = tx.Where("`title` like ?", "%"+title+"%")
			}
			return tx
		}
	} else {
		ops = func(tx *gorm.DB) *gorm.DB {
			tx = tx.Where("`type` = ? and `status` = ?", config.CategoryTypeArchive, 1).Order("module_id asc,sort asc")
			if moduleId > 0 {
				tx = tx.Where("`module_id` = ?", moduleId)
			}
			if title != "" {
				tx = tx.Where("`title` like ?", "%"+title+"%")
			}
			return tx
		}
	}
	categories, err = currentSite.GetCategories(ops, 0)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	for i := range categories {
		categories[i].Link = currentSite.GetUrl("category", categories[i], 0)
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("保存文档分类：%d => %s", category.Id, category.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除文档分类：%d => %s", category.Id, category.Title))

	currentSite.DeleteCacheCategories()
	currentSite.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "分类已删除",
	})
}
