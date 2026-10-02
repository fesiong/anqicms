package manageController

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// AttachmentUpload 上传附件。
func AttachmentUpload(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	// 附件的分类 ID
	categoryId := uint(ctx.PostValueIntDefault("category_id", 0))
	// 附件的 ID，用于替换附件
	attachId := uint(ctx.PostValueIntDefault("id", 0))
	// Receive attachment, image/video/file
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
	// 分片上传分片数量
	chunks := ctx.PostValueIntDefault("chunks", 0)
	if chunks > 0 {
		// 分片上传的当前分片索引
		chunk := ctx.PostValueIntDefault("chunk", 0)
		// 使用了分片上传的附件文件名
		fileName := ctx.PostValue("file_name")
		// 使用了分片上传的附件 MD5
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

// AttachmentList 分页查询附件列表，支持按分类与关键词筛选。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页条数，默认为 20。
//   - 查询参数 "category_id": 按附件分类 ID 筛选，0 表示不限。
//   - 查询参数 "q": 按文件名关键词模糊搜索。
//   - 查询参数 "type": 附件类型，0-所有，1-图片，2-视频，默认0。
func AttachmentList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	categoryId := uint(ctx.URLParamIntDefault("category_id", 0))
	q := ctx.URLParam("q")
	imageType := ctx.URLParamIntDefault("type", 0)

	attachments, total, err := currentSite.GetAttachmentList(categoryId, q, imageType, currentPage, pageSize)
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

// AttachmentDetail 获取单个附件的详细信息，包括访问 URL 与缩略图。
//
// 参数说明：
//   - 查询参数 "id": 附件 ID，必填。
func AttachmentDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	id := ctx.URLParamIntDefault("id", 0)
	if id <= 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("TheImageResourceToBeReplacedDoesNotExist"),
		})
		return
	}

	attach, err := currentSite.GetAttachmentById(uint(id))
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	attach.GetThumb(currentSite.PluginStorage.StorageUrl)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": attach,
	})
}

// AttachmentDelete 删除指定附件。
func AttachmentDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.AttachmentDeleteRequest
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

	err = currentSite.DeleteAttachment(attach)
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

// AttachmentEdit 修改附件的基本信息，如文件路径与附件名称。
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

	if req.FileName != "" {
		attach.FileName = req.FileName
		err = currentSite.DB.Save(attach).Error
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}
	// 支持更改路径
	if req.FileLocation != "" && req.FileLocation != attach.FileLocation {
		// 后缀不能改
		if filepath.Ext(req.FileLocation) != filepath.Ext(attach.FileLocation) {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("IncorrectFile"),
			})
			return
		}
		if !provider.CheckContentIsEnglish(req.FileLocation) {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("IncorrectFile"),
			})
			return
		}
		req.FileLocation = strings.ReplaceAll(req.FileLocation, "%20", "-")
		req.FileLocation = strings.ReplaceAll(req.FileLocation, " ", "-")
		req.FileLocation = strings.ToLower(req.FileLocation)
		// 不能超过 200个字符
		if len(req.FileLocation) > 220 {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("IncorrectFile"),
			})
			return
		}
		// 防止跨目录，不能移动到超过 currentSite.PublicPath + "uploads"
		realPath := filepath.Join(currentSite.PublicPath, filepath.Clean(req.FileLocation))
		if !strings.HasPrefix(realPath, currentSite.PublicPath) {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("IncorrectFile"),
			})
			return
		}
		newLocation := strings.TrimPrefix(req.FileLocation, currentSite.PublicPath)

		err = currentSite.MoveFile(attach.FileLocation, newLocation)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("IncorrectFile"),
			})
			return
		}
		_ = currentSite.DB.Save(&attach).Error
		// 如果是视频，并且有缩略图
		if attach.IsImage == 2 && attach.Logo != "" {
			logo := strings.TrimPrefix(attach.Logo, currentSite.PluginStorage.StorageUrl)
			logo = strings.TrimPrefix(logo, "/")
			if logo != "" && logo != attach.FileLocation {
				ext := filepath.Ext(attach.FileLocation)
				newLogo := strings.Replace(logo, strings.TrimSuffix(attach.FileLocation, ext), strings.TrimSuffix(newLocation, ext), 1)
				if logo != newLogo {
					err = currentSite.MoveFile(logo, newLogo)
					if err == nil {
						attach.Logo = newLogo
					}
				}
			}
		} else if attach.Logo != "" {
			// 只有视频有不同的缩略图，其它按同 FileLocation 处理
			attach.Logo = ""
		}
		attach.FileLocation = newLocation
		currentSite.DB.Save(attach)
		attach.GetThumb(currentSite.PluginStorage.StorageUrl)
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyImageNameLog", attach.Id, attach.FileName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ImageNameModified"),
		"data": attach,
	})
}

// AttachmentScanUploads 扫描 uploads 目录，把未登记的文件补录进附件库；后台异步执行，接口立即返回。
func AttachmentScanUploads(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	// 仅扫描uploads目录
	go currentSite.AttachmentScanUploads(currentSite.PublicPath + "uploads")

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SubmittedForBackgroundProcessing"),
	})
}

// AttachmentChangeCategory 批量修改附件所属的分类。
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

// AttachmentAddRemoteUrl 按远程 URL 批量添加附件记录。
func AttachmentAddRemoteUrl(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.AttachmentAddRemoteUrl
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if len(req.Urls) == 0 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("PleaseEnterTheRemoteUrl"),
		})
		return
	}

	err := currentSite.AddRemoteUrls(req.Urls, req.CategoryId)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("RemoteUrlAdded"),
	})
}

// ConvertImageToWebp 批量转换图片为 WebP 格式。后台异步执行，接口立即返回。
func ConvertImageToWebp(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	go currentSite.StartConvertImageToWebp()

	currentSite.AddAdminLog(ctx, ctx.Tr("BatchConvertImagesToWebp"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("TheConversionTaskHasBeenSubmittedToTheBackgroundForRunning"),
	})
}
