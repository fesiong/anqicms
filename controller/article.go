package controller

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
	"irisweb/request"
	"strings"
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
	//相邻政策
	relationList, _ := provider.GetRelationArticleList(article.CategoryId, id, 10)
	//获取上一篇
	prev, _ := provider.GetPrevArticleById(article.CategoryId, id)
	//获取下一篇
	next, _ := provider.GetNextArticleById(article.CategoryId, id)

	webInfo.Title = article.Title
	webInfo.Keywords = article.Keywords
	webInfo.Description = article.Description
	ctx.ViewData("webInfo", webInfo)
	ctx.ViewData("article", article)
	ctx.ViewData("newest", newest)
	ctx.ViewData("relationList", relationList)
	ctx.ViewData("prev", prev)
	ctx.ViewData("next", next)

	ctx.View("article/detail.html")
}

func ArticlePublish(ctx iris.Context) {
	//发布必须登录
	if !ctx.Values().GetBoolDefault("hasLogin", false) {
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
	if !ctx.Values().GetBoolDefault("hasLogin", false) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "登录后方可操作",
		})
		return
	}
	var req request.Article
	if err := ctx.ReadForm(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	var category *model.Category
	var err error
	//检查分类
	if req.CategoryName != "" {
		category, err = provider.GetCategoryByTitle(req.CategoryName)
		if err != nil {
			category = &model.Category{
				Title:       req.CategoryName,
				Status:      1,
			}
			err = category.Save(config.DB)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
		}
	}

	var article *model.Article
	if req.Id > 0 {
		article, err = provider.GetArticleById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		article = &model.Article{
			Title:       req.Title,
			Keywords:    req.Keywords,
			Description: req.Description,
			Status:      1,
			ArticleData: model.ArticleData{
				Content: req.Content,
			},
		}
	}
	//提取描述
	if req.Description == "" {
		htmlR := strings.NewReader(req.Content)
		doc, err := goquery.NewDocumentFromReader(htmlR)
		if err == nil {
			textRune := []rune(strings.TrimSpace(doc.Text()))
			if len(textRune) > 150 {
				article.Description = string(textRune[:150])
			} else {
				article.Description = string(textRune)
			}
		}
	}
	if category != nil {
		article.CategoryId = category.Id
	}
	err = article.Save(config.DB)
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