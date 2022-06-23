package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func CategoryList(ctx iris.Context) {
	moduleId := uint(ctx.URLParamIntDefault("module_id", 0))
	categoryType := uint(ctx.URLParamIntDefault("type", 0))
	title := ctx.URLParam("title")

	var categories []*model.Category
	var err error
	if categoryType == config.CategoryTypePage {
		categories, err = provider.GetPages(title)
	} else {
		categories, err = provider.GetCategories(moduleId, title, 0)
	}
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	for i := range categories {
		categories[i].Link = provider.GetUrl("category", categories[i], 0)
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": categories,
	})
}

func CategoryDetail(ctx iris.Context) {
	id := uint(ctx.URLParamIntDefault("id", 0))

	category, err := provider.GetCategoryById(id)
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
	var req request.Category
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	category, err := provider.SaveCategory(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("保存文档分类：%d => %s", category.Id, category.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "保存成功",
		"data": category,
	})
}

func CategoryDelete(ctx iris.Context) {
	var req request.Category
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	category, err := provider.GetCategoryById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = category.Delete(dao.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("删除文档分类：%d => %s", category.Id, category.Title))

	provider.DeleteCacheCategories()
	provider.DeleteCacheIndex()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "分类已删除",
	})
}
