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

func PluginCommentList(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	comments, total, err := provider.GetCommentList(0, "id desc", currentPage, pageSize, 0)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}
	type miniArticle struct {
		Id    uint
		Title string
	}
	for i, v := range comments {
		var article miniArticle
		err := dao.DB.Model(&model.Archive{}).Where("id = ?", v.ArchiveId).Scan(&article).Error
		if err == nil {
			comments[i].ItemTitle = article.Title
		}
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  comments,
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
	if req.Ip == "" {
		req.Ip = ctx.RemoteAddr()
	}
	comment.Ip = req.Ip

	err = comment.Save(dao.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("修改评论内容：%d", comment.Id))

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

	err = comment.Delete(dao.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("修改文档模型：%d => %s", comment.Id, comment.Content))

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
	err = comment.Save(dao.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("审核通过锚文本：%d", comment.Id))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "评论已更新",
	})
}
