package manageController

import (
	"github.com/kataras/iris/v12"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"time"
)

func AnqiLogin(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AnqiLoginRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.AnqiLogin(&req)
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
		"data": config.AnqiUser,
	})
}

func GetAnqiInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	go currentSite.AnqiCheckLogin()
	authUser := config.AnqiUser
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": authUser,
	})

	return
}

func AnqiUploadAttachment(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	attachment, err := currentSite.AnqiUploadAttachment(fileBytes, info.Filename)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "上传成功",
		"data": attachment,
	})
}

func AnqiShareTemplate(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AnqiTemplateRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.AnqiShareTemplate(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "提交成功",
	})
}

func AnqiDownloadTemplate(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AnqiTemplateRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.AnqiDownloadTemplate(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "下载成功",
	})
}

func AnqiSendFeedback(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AnqiFeedbackRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.AnqiSendFeedback(&req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "提交成功",
	})
}

func RestartAnqicms(ctx iris.Context) {
	// first need to stop iris
	config.RestartChan <- false

	time.Sleep(3 * time.Second)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "重启成功",
	})
}
