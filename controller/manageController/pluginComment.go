package manageController

import (
	"html"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginCommentList 分页获取评论列表，并附上评论所属文档标题。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
func PluginCommentList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	comments, total, err := currentSite.GetCommentList(0, 0, "id desc", currentPage, pageSize, 0)
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
		err = currentSite.DB.Model(&model.Archive{}).Where("id = ?", v.ArchiveId).Scan(&article).Error
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

// PluginCommentDetail 根据ID获取评论详情。
//
// 参数说明：
//   - 查询参数 "id": 评论 ID。
func PluginCommentDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	id := uint(ctx.URLParamIntDefault("id", 0))
	comment, err := currentSite.GetCommentById(id)
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

// PluginCommentDetailForm 修改指定评论的内容、用户名和IP。
func PluginCommentDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginCommentUpdateRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment, err := currentSite.GetCommentById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 将单个&nbsp;替换为空格
	req.Content = library.ReplaceSingleSpace(req.Content)
	comment.UserName = req.UserName
	// 防止XSS: HTML转义评论内容
	comment.Content = html.EscapeString(req.Content)
	comment.Status = 1
	if req.Ip == "" {
		req.Ip = ctx.RemoteAddr()
	}
	comment.Ip = req.Ip

	err = comment.Save(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyCommentContentLog", comment.Id))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CommentUpdated"),
	})
}

// PluginCommentDelete 删除指定评论。
func PluginCommentDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginCommentDeleteRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	comment, err := currentSite.GetCommentById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = comment.Delete(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyDocumentModelLog", comment.Id, comment.Content))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CommentDeleted"),
	})
}

// PluginCommentCheck 处理评论的审核状态
func PluginCommentCheck(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginCommentApprovalRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if len(req.Ids) > 0 {
		err := currentSite.DB.Model(&model.Comment{}).Where("`id` IN (?)", req.Ids).UpdateColumn("status", req.Status).Error
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		currentSite.AddAdminLog(ctx, ctx.Tr("BatchReviewCommentsLog", req.Ids))

		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("CommentUpdated"),
		})
		return
	}

	comment, err := currentSite.GetCommentById(req.Id)
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
	err = comment.Save(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ReviewCommentsLog", comment.Id))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CommentUpdated"),
	})
}
