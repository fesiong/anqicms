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
	go currentSite.AnqiCheckLogin(false)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": provider.GetAuthInfo(),
	})

	return
}

func CheckAnqiInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentSite.AnqiCheckLogin(true)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": provider.GetAuthInfo(),
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
		"msg":  ctx.Tr("UploadSuccessfully"),
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
		"msg":  ctx.Tr("SubmitSuccessfully"),
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
		"msg":  ctx.Tr("DownloadSuccessfully"),
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
		"msg":  ctx.Tr("SubmitSuccessfully"),
	})
}

func AnqiTranslateArticle(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.Archive
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	archive, err := currentSite.GetArchiveById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.AnqiTranslateArticle(archive, req.ToLanguage)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("TranslationHasBeenAddedToThePlan"),
	})
}

func AnqiAiPseudoArticle(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.Archive
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	archive, err := currentSite.GetArchiveById(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err = currentSite.AnqiAiPseudoArticle(archive)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("AiPseudoOriginalHasBeenAddedToThePlan"),
	})
}

func AuthAiGenerateStream(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.KeywordRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	streamId, err := currentSite.AnqiAiGenerateStream(&req)
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
		"data": streamId,
	})
}

func AuthAiGenerateStreamData(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	streamId := ctx.URLParam("stream_id")

	content, msg, finished := currentSite.AnqiLoadStreamData(streamId)

	if msg != "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  msg,
		})
		return
	}
	if finished {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "finished",
			"data": content,
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": content,
	})
}

func RestartAnqicms(ctx iris.Context) {
	// first need to stop iris
	config.RestartChan <- 1

	time.Sleep(3 * time.Second)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("RestartSuccessfully"),
	})
}
