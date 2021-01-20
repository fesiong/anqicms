package manageController

import (
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/controller"
	"irisweb/provider"
	"irisweb/request"
)

func AttachmentUpload(ctx iris.Context) {
	//复用上传接口
	controller.AttachmentUpload(ctx)
}

func AttachmentList(ctx iris.Context) {
	currentPage := ctx.URLParamIntDefault("page", 1)
	pageSize := ctx.URLParamIntDefault("limit", 30)

	attachments, total, err := provider.GetAttachmentList(currentPage, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"count": total,
		"limit": pageSize,
		"data": attachments,
	})
}

func AttachmentDelete(ctx iris.Context) {
	var req request.Attachment
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	attach, err := provider.GetAttachmentById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = attach.Delete(config.DB)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "分类已删除",
	})
}
