package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
	"irisweb/request"
)

func PluginMaterialList(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("limit", 20)
	keyword := ctx.URLParam("keyword")
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))

	materialList, total, err := provider.GetMaterialList(categoryId, keyword, currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"count": total,
		"data": materialList,
	})
}

func PluginMaterialCategoryList(ctx iris.Context) {

	categories, err := provider.GetMaterialCategories()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": categories,
	})
}

func PluginMaterialDetailForm(ctx iris.Context) {
	var req request.PluginMaterial
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	category, err := provider.SaveMaterial(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "素材已更新",
		"data": category,
	})
}

func PluginMaterialDelete(ctx iris.Context) {
	var req request.PluginMaterial
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.DeleteMaterial(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "素材已删除",
	})
}

func PluginMaterialCategoryDetailForm(ctx iris.Context) {
	var req request.PluginMaterialCategory
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	category, err := provider.SaveMaterialCategory(&req)
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

func PluginMaterialCategoryDelete(ctx iris.Context) {
	var req request.PluginMaterialCategory
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.DeleteMaterialCategory(req.Id)
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