package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"strings"
)

func AttachmentUpload(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if !strings.HasPrefix(ctx.RequestPath(false), currentSite.BaseURI) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "Not Found",
		})
		return
	}
	// 增加分类
	categoryId := uint(ctx.PostValueIntDefault("category_id", 0))
	attachId := uint(ctx.PostValueIntDefault("id", 0))
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()

	if attachId > 0 {
		_, err := currentSite.GetAttachmentById(attachId)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  currentSite.Lang("需要替换的图片资源不存在"),
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("上传资源附件：%d => %s", attachment.Id, attachment.FileLocation))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": attachment,
	})
}
