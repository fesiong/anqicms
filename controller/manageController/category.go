package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
	"irisweb/request"
)

func CategoryList(ctx iris.Context) {
	categoryType := uint(ctx.URLParamIntDefault("type", 0))
	categories, err := provider.GetCategories(categoryType)
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
		"data": categories,
	})
}

func CategoryDetail(ctx iris.Context) {
	id := ctx.Params().GetUintDefault("id", 0)

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

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "分类已更新",
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

	err = category.Delete(config.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "分类已删除",
	})
}
