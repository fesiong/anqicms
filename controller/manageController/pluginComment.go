package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/provider"
	"irisweb/request"
)

func PluginCommentList(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("limit", 20)
	comments, total, err := provider.GetCommentList("", 0, "id desc", currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}
	type miniArticle struct {
		Id uint
		Title string
	}
	for i, v := range comments {
		if v.ItemType == model.ItemTypeArticle {
			var article miniArticle
			err := config.DB.Model(&model.Article{}).Where("id = ?", v.ItemId).Scan(&article).Error
			if err == nil {
				comments[i].ItemTitle = article.Title
			}
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"count": total,
		"data": comments,
	})
}


func PluginCommentDetail(ctx iris.Context) {
	id := uint(ctx.URLParamIntDefault("id", 0))
	comment, err := provider.GetCommentById(id)
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
		"data": comment,
	})
}

func PluginCommentDetailForm(ctx iris.Context) {
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

	comment.UserName = req.UserName
	comment.Content = req.Content
	comment.Status = 1

	err = comment.Save(config.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "评论已更新",
	})
}

func PluginCommentDelete(ctx iris.Context) {
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

	err = comment.Delete(config.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "评论已删除",
	})
}

//处理审核状态
func PluginCommentCheck(ctx iris.Context) {
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

	if comment.Status != model.StatusOk {
		comment.Status = model.StatusOk
	} else {
		comment.Status = model.StatusWait
	}
	err = comment.Save(config.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "评论已更新",
	})
}