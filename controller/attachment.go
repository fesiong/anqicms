package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"strings"
)

func AttachmentUpload(ctx iris.Context) {
	// 增加分类
	categoryId := uint(ctx.PostValueIntDefault("category_id", 0))
	attachId := uint(ctx.PostValueIntDefault("id", 0))
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}
	defer file.Close()

	if attachId > 0 {
		attachment, err := provider.GetAttachmentById(attachId)
		if err != nil {
			ctx.JSON(iris.Map{
				"status": config.StatusFailed,
				"msg":    "需要替换的图片资源不存在",
			})
			return
		}
		if strings.HasSuffix(attachment.FileLocation, ".mp4") {
			ctx.JSON(iris.Map{
				"status": config.StatusFailed,
				"msg":    "仅支持替换图片资源",
			})
			return
		}
	}

	attachment, err := provider.AttachmentUpload(file, info, categoryId, attachId)
	if err != nil {
		ctx.JSON(iris.Map{
			"status": config.StatusFailed,
			"msg":    err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("上传图片：%d => %s", attachment.Id, attachment.FileLocation))

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
