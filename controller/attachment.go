package controller

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/provider"
)

func AttachmentUpload(ctx iris.Context) {
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}
	defer file.Close()

	attachment, err := provider.AttachmentUpload(file, info)
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
			"src": attachment.Logo,
			"title": attachment.FileName,
		},
	})
}
