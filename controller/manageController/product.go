package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
	"irisweb/request"
)

func ProductList(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("limit", 20)
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))
	products, total, err := provider.GetProductList(categoryId, "id desc", currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	//读取列表的分类
	categories, _ := provider.GetCategories(model.CategoryTypeProduct)
	for i, v := range products {
		if v.CategoryId > 0 {
			for _, c := range categories {
				if c.Id == v.CategoryId {
					products[i].Category = c
				}
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"count": total,
		"data": products,
	})
}

func ProductDetail(ctx iris.Context) {
	id := uint(ctx.URLParamIntDefault("id", 0))

	product, err := provider.GetProductById(id)
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
		"data": product,
	})
}

func ProductDetailForm(ctx iris.Context) {
	var req request.Product
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	product, err := provider.SaveProduct(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "产品已更新",
		"data": product,
	})
}

func ProductDelete(ctx iris.Context) {
	var req request.Product
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	product, err := provider.GetProductById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = product.Delete(config.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "产品已删除",
	})
}

func ProductExtraFieldsSetting(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"fields": config.JsonData.ProductExtraFields,
		},
	})
}

func ProductExtraFieldsSettingForm(ctx iris.Context) {
	var req request.ProductExtraFieldsSetting
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := provider.SaveProductExtraFields(req.Fields)

	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
