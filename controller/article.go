package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
	"irisweb/request"
)

func ArticleDetail(ctx iris.Context) {
	id := ctx.Params().GetUintDefault("id", 0)
	article, err := provider.GetArticleById(id)
	if err != nil {
		NotFound(ctx)
		return
	}
	_ = article.AddViews(config.DB)
	//最新
	newest, _, _ := provider.GetArticleList(article.CategoryId, "id desc", 1, 10)
	//相邻相关文章
	relationList, _ := provider.GetRelationArticleList(article.CategoryId, id, 10)
	//获取上一篇
	prev, _ := provider.GetPrevArticleById(article.CategoryId, id)
	//获取下一篇
	next, _ := provider.GetNextArticleById(article.CategoryId, id)
	//获取评论内容
	comments, _, _ := provider.GetCommentList(model.ItemTypeArticle, article.Id, "id desc", 1, 10)

	webInfo.Title = article.Title
	webInfo.Keywords = article.Keywords
	webInfo.Description = article.Description
	ctx.ViewData("webInfo", webInfo)
	ctx.ViewData("article", article)
	ctx.ViewData("newest", newest)
	ctx.ViewData("relationList", relationList)
	ctx.ViewData("prev", prev)
	ctx.ViewData("next", next)
	ctx.ViewData("comments", comments)

	ctx.View("article/detail.html")
}

func ArticlePublish(ctx iris.Context) {
	//发布必须登录
	if ctx.Values().GetIntDefault("adminId", 0) == 0 {
		InternalServerError(ctx)
		return
	}

	id := uint(ctx.URLParamIntDefault("id", 0))
	if id > 0 {
		article, _ := provider.GetArticleById(id)

		ctx.ViewData("article", article)
	}
	webInfo.Title = "发布文章"
	ctx.ViewData("webInfo", webInfo)
	ctx.View("article/publish.html")
}

func ArticlePublishForm(ctx iris.Context) {
	//发布必须登录
	if ctx.Values().GetIntDefault("adminId", 0) == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusNoLogin,
			"msg":  "登录后方可操作",
		})
		return
	}
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
		"msg":  "发布成功",
		"data": article,
	})
}