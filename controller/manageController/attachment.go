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

	currentSite.AddAdminLog(ctx, ctx.Tr("删除图片：%d => %s", attach.Id, attach.FileLocation))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("图片已删除"),
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

	currentSite.AddAdminLog(ctx, ctx.Tr("修改图片名称：%d => %s", attach.Id, attach.FileName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("图片名称已修改"),
	})
}

func AttachmentScanUploads(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	// 仅扫描uploads目录
	go currentSite.AttachmentScanUploads(currentSite.PublicPath + "uploads")

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("已提交后台处理，请稍后查看结果"),
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

	currentSite.AddAdminLog(ctx, ctx.Tr("更改图片的分类：%d => %v", req.CategoryId, req.Ids))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("分类已更新"),
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

	currentSite.AddAdminLog(ctx, ctx.Tr("保存图片分类：%d => %s", category.Id, category.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("分类已更新"),
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

	currentSite.AddAdminLog(ctx, ctx.Tr("删除图片分类：%d => %s", req.Id, req.Title))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("分类已删除"),
	})
}

func ConvertImageToWebp(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	go currentSite.StartConvertImageToWebp()

	currentSite.AddAdminLog(ctx, ctx.Tr("批量转换图片为webp"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("转换任务已提交到后台运行，具体结束时间与实际图片数量有关。"),
	})
}
