package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func AttachmentUpload(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	// 增加分类
	categoryId := uint(ctx.PostValueIntDefault("category_id", 0))
	attachId := uint(ctx.PostValueIntDefault("id", 0))
	file, info, err := ctx.FormFile("file")
	if err != nil {
		file, info, err = ctx.FormFile("file1")
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}
	defer file.Close()

	if attachId > 0 {
		adminId := ctx.Values().GetUintDefault("adminId", 0)
		if adminId == 0 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("无法修改图片"),
			})
			return
		}
		_, err := currentSite.GetAttachmentById(attachId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("需要替换的图片资源不存在"),
			})
			return
		}
	}

	attachment, err := currentSite.AttachmentUpload(file, info, categoryId, attachId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("上传资源附件：%d => %s", attachment.Id, attachment.FileLocation))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": attachment,
	})
}
