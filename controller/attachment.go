package controller

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"os"
)

func AttachmentUpload(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	userId := ctx.Values().GetUintDefault("userId", 0)
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
		// 前端用户不允许编辑
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("UnableToModifyTheImage"),
		})
		return
	}

	var attachment *model.Attachment
	// 增加支持分片上传
	chunks := ctx.PostValueIntDefault("chunks", 0)
	if chunks > 0 {
		chunk := ctx.PostValueIntDefault("chunk", 0)
		fileName := ctx.PostValue("file_name")
		fileMd5 := ctx.PostValue("md5")
		// 使用了分片上传
		tmpFile, err := currentSite.UploadByChunks(file, fileMd5, chunk, chunks)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		if tmpFile == nil {
			// 表示分片上传，不需要返回结果
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  "",
			})
			return
		}
		defer func() {
			tmpName := tmpFile.Name()
			_ = tmpFile.Close()
			_ = os.Remove(tmpName)
		}()
		stat, err := tmpFile.Stat()
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		info.Filename = fileName
		info.Size = stat.Size()
		tmpFile.Seek(0, 0)

		attachment, err = currentSite.AttachmentUpload(tmpFile, info, categoryId, attachId, userId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		// 普通上传
		attachment, err = currentSite.AttachmentUpload(file, info, categoryId, attachId, userId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UploadResourceAttachmentLog", attachment.Id, attachment.FileLocation))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": attachment,
	})
}
