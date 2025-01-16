package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"os"
)

func AttachmentUpload(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
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
				"msg":  ctx.Tr("UnableToModifyTheImage"),
			})
			return
		}
		_, err := currentSite.GetAttachmentById(attachId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("TheImageResourceToBeReplacedDoesNotExist"),
			})
			return
		}
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

		attachment, err = currentSite.AttachmentUpload(tmpFile, info, categoryId, attachId, 0)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	} else {
		// 普通上传
		attachment, err = currentSite.AttachmentUpload(file, info, categoryId, attachId, 0)
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

func AttachmentList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))
	q := ctx.URLParam("q")

	attachments, total, err := currentSite.GetAttachmentList(categoryId, q, currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"limit": pageSize,
		"data":  attachments,
	})
}

func AttachmentDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.Attachment
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	attach, err := currentSite.GetAttachmentById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = attach.Delete(currentSite.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteImageLog", attach.Id, attach.FileLocation))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ImageDeleted"),
	})
}

func AttachmentEdit(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.Attachment
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	attach, err := currentSite.GetAttachmentById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	attach.FileName = req.FileName
	err = currentSite.DB.Save(attach).Error
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyImageNameLog", attach.Id, attach.FileName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ImageNameModified"),
	})
}

func AttachmentScanUploads(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	// 仅扫描uploads目录
	go currentSite.AttachmentScanUploads(currentSite.PublicPath + "uploads")

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SubmittedForBackgroundProcessing"),
	})
}

func AttachmentChangeCategory(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.ChangeAttachmentCategory
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.ChangeAttachmentCategory(req.CategoryId, req.Ids)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ChangeImageCategoryLog", req.CategoryId, req.Ids))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CategoryUpdated"),
	})
}

func AttachmentCategoryList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	categories, err := currentSite.GetAttachmentCategories()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": categories,
	})
}

func AttachmentCategoryDetailForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.AttachmentCategory
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	category, err := currentSite.SaveAttachmentCategory(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("SaveImageCategoryLog", category.Id, category.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CategoryUpdated"),
		"data": category,
	})
}

func AttachmentCategoryDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.AttachmentCategory
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteAttachmentCategory(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteImageCategoryLog", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CategoryDeleted"),
	})
}

func ConvertImageToWebp(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	go currentSite.StartConvertImageToWebp()

	currentSite.AddAdminLog(ctx, ctx.Tr("BatchConvertImagesToWebp"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("TheConversionTaskHasBeenSubmittedToTheBackgroundForRunning"),
	})
}
