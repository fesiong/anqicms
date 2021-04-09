package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
	"irisweb/request"
	"math"
	"strings"
)

func CommentPublish(ctx iris.Context) {
	//登录状态的用户，发布不进审核，否则进审核
	status := uint(0)
	userId := ctx.Values().GetIntDefault("adminId", 0)
	if userId > 0 {
		status = 1
	}

	var req request.PluginComment
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	req.Status = status
	req.UserId = uint(userId)
	req.Ip = ctx.RemoteAddr()
	if req.ParentId > 0 {
		parent, err := provider.GetCommentById(req.ParentId)
		if err == nil {
			req.ToUid = parent.UserId
		}
	}

	comment, err := provider.SaveComment(&req)
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
		"data": comment,
	})
}

func CommentPraise(ctx iris.Context) {
	var req request.PluginComment
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment, err := provider.GetCommentById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment.VoteCount += 1
	err = comment.Save(config.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment.Active = true

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "点赞成功",
		"data": comment,
	})
}

func ArticleCommentList(ctx iris.Context) {
	ctx.Params().Set("itemType", "article")
	itemId := uint(ctx.Params().GetIntDefault("id", 0))

	article, err := provider.GetArticleById(itemId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.ViewData("itemData", article)
	webInfo.Title = "评论文章：" + article.Title
	webInfo.Keywords = article.Keywords
	webInfo.Description = article.Description
	ctx.ViewData("webInfo", webInfo)

	CommentList(ctx)
}

func ProductCommentList(ctx iris.Context) {
	ctx.Params().Set("itemType", "product")
	itemId := uint(ctx.Params().GetIntDefault("id", 0))

	product, err := provider.GetProductById(itemId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.ViewData("itemData", product)
	webInfo.Title = "评论产品：" + product.Title
	webInfo.Keywords = product.Keywords
	webInfo.Description = product.Description
	ctx.ViewData("webInfo", webInfo)

	CommentList(ctx)
}

func CommentList(ctx iris.Context) {
	itemType := ctx.Params().Get("itemType")
	itemId := uint(ctx.Params().GetIntDefault("id", 0))
	currentPage := ctx.URLParamIntDefault("page", 1)
	paramPage := ctx.Params().GetIntDefault("page", 0)
	if paramPage > 0 {
		currentPage = paramPage
	}
	pageSize := 10

	comments, total, err := provider.GetCommentList(itemType, itemId, "id desc", currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	totalPage := math.Ceil(float64(total)/float64(pageSize))

	prevPage := ""
	nextPage := ""
	urlPfx := fmt.Sprintf("/comment/%s/%d?", itemType, itemId)

	if currentPage > 1 {
		prevPage = fmt.Sprintf("%spage=%d", urlPfx, currentPage-1)
	}

	if currentPage < int(totalPage) {
		nextPage = fmt.Sprintf("%spage=%d", urlPfx, currentPage+1)
	}
	if currentPage == 2 {
		prevPage = strings.TrimRight(prevPage, "page=1")
	}

	//热门文章
	populars, _, _ := provider.GetArticleList(0, "views desc", 1, 10)
	ctx.ViewData("populars", populars)

	ctx.ViewData("itemType", itemType)
	ctx.ViewData("itemId", itemId)
	ctx.ViewData("totalPage", totalPage)
	ctx.ViewData("prevPage", prevPage)
	ctx.ViewData("nextPage", nextPage)
	ctx.ViewData("comments", comments)
	ctx.View(GetViewPath(ctx, "comment/list.html"))
}