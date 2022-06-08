package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
)

func AttachmentUpload(ctx iris.Context) {
	// 增加分类
	categoryId := uint(ctx.PostValueIntDefault("category_id", 0))
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}
	defer file.Close()

	attachment, err := provider.AttachmentUpload(file, info, categoryId)
	if err != nil {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"thumb": attachment.Thumb,
			"src": attachment.Logo,
			"title": attachment.FileName,
		},
	})
}
