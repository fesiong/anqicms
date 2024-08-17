package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/controller"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func AttachmentUpload(ctx iris.Context) {
	//复用上传接口
	controller.AttachmentUpload(ctx)
}

func AttachmentList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)

	// 仅扫描uploads目录
	go currentSite.AttachmentScanUploads(currentSite.PublicPath + "uploads")

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SubmittedForBackgroundProcessing"),
	})
}

func AttachmentChangeCategory(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)

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
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
	go currentSite.StartConvertImageToWebp()

	currentSite.AddAdminLog(ctx, ctx.Tr("BatchConvertImagesToWebp"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("TheConversionTaskHasBeenSubmittedToTheBackgroundForRunning"),
	})
}
