package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func CommentPublish(ctx iris.Context) {
	// 支持返回为 json 或html， 默认 html
	returnType := ctx.PostValueTrim("return")
	//登录状态的用户，发布不进审核，否则进审核
	status := uint(0)
	userId := ctx.Values().GetIntDefault("adminId", 0)
	if userId > 0 {
		status = 1
	}

	var req request.PluginComment
	// 采用post接收
	req.ArchiveId = uint(ctx.PostValueIntDefault("archive_id", 0))
	req.UserName = ctx.PostValueTrim("user_name")
	req.Ip = ctx.PostValueTrim("ip")
	req.Content = ctx.PostValueTrim("content")
	req.ParentId = uint(ctx.PostValueIntDefault("parent_id", 0))
	req.ToUid = uint(ctx.PostValueIntDefault("to_uid", 0))

	req.Status = status
	req.UserId = uint(userId)
	if req.Ip == "" {
		req.Ip = ctx.RemoteAddr()
	}
	if req.ParentId > 0 {
		parent, err := provider.GetCommentById(req.ParentId)
		if err == nil {
			req.ToUid = parent.UserId
		}
	}

	comment, err := provider.SaveComment(&req)
	if err != nil {
		msg := config.Lang("保存失败")
		if returnType == "json" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  msg,
			})
		} else {
			ShowMessage(ctx, msg, "")
		}
		return
	}

	msg := config.Lang("发布成功")
	if returnType == "json" {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  msg,
			"data": comment,
		})
	} else {
		var link string
		refer := ctx.GetReferrer()
		if refer.URL != "" {
			link = refer.URL
		}
		ShowMessage(ctx, config.Lang("发布成功"), link)
	}
}

func CommentPraise(ctx iris.Context) {
	var req request.PluginComment
	req.Id = uint(ctx.PostValueIntDefault("id", 0))

	comment, err := provider.GetCommentById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	comment.VoteCount += 1
	err = comment.Save(dao.DB)
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
		"msg":  config.Lang("点赞成功"),
		"data": comment,
	})
}

func CommentList(ctx iris.Context) {
	archiveId := uint(ctx.Params().GetIntDefault("id", 0))
	archive, err := provider.GetArchiveById(archiveId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	archive.Link = provider.GetUrl("archive", archive, 0)
	ctx.ViewData("archive", archive)
	webInfo.Title = config.Lang("评论") + ": " + archive.Title
	webInfo.Keywords = archive.Keywords
	webInfo.Description = archive.Description
	webInfo.PageName = "comments"
	currentPage := ctx.URLParamIntDefault("page", 1)
	webInfo.CanonicalUrl = provider.GetUrl(fmt.Sprintf("/comment/%d(?page={page})", archive.Id), nil, currentPage)
	ctx.ViewData("webInfo", webInfo)

	ctx.ViewData("archiveId", archiveId)
	tplName := "comment/list.html"
	if ViewExists(ctx, "comment_list.html") {
		tplName = "comment_list.html"
	}
	err = ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}