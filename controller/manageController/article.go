package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
	"irisweb/request"
)

func ArticleList(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("limit", 20)
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))
	articles, total, err := provider.GetArticleList(categoryId, "id desc", currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	//读取列表的分类
	categories, _ := provider.GetCategories()
	for i, v := range articles {
		if v.CategoryId > 0 {
			for _, c := range categories {
				if c.Id == v.CategoryId {
					articles[i].Category = c
				}
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"count": total,
		"data": articles,
	})
}

func ArticleDetail(ctx iris.Context) {
	id := uint(ctx.URLParamIntDefault("id", 0))

	article, err := provider.GetArticleById(id)
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
		"data": article,
	})
}

func ArticleDetailForm(ctx iris.Context) {
	var req request.Article
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	article, err := provider.SaveArticle(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文章已更新",
		"data": article,
	})
}

func ArticleDelete(ctx iris.Context) {
	var req request.Article
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	article, err := provider.GetArticleById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = article.Delete(config.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文章已删除",
	})
}
