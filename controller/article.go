package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
)

func ArticleDetail(ctx iris.Context) {
	id := ctx.Params().GetUintDefault("id", 0)
	urlToken := ctx.Params().GetString("filename")
	var article *model.Article
	var err error
	if urlToken != "" {
		//优先使用urlToken
		article, err = provider.GetArticleByUrlToken(urlToken)
	} else {
		article, err = provider.GetArticleById(id)
	}
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
