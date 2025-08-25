package manageController

import (
	"github.com/kataras/iris/v12"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
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

func AuthExtractKeywords(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AnqiExtractRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	result, err := currentSite.AnqiExtractKeywords(&req)
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
		"data": result,
	})
}

func AuthExtractDescription(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AnqiExtractRequest
	var err error
	if err = ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	result, err := currentSite.AnqiExtractDescription(&req)
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
		"data": strings.Join(result, ""),
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

	isDraft := false
	archive, err := currentSite.GetArchiveById(req.Id)
	if err != nil {
		// 可能是 草稿
		archiveDraft, err := currentSite.GetArchiveDraftById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		isDraft = true
		archive = &archiveDraft.Archive
	}
	// 读取 data
	archiveData, err := currentSite.GetArchiveDataById(archive.Id)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	transReq := &provider.AnqiTranslateTextRequest{
		Text: []string{
			archive.Title,       // 0
			archive.Description, // 1
			archive.Keywords,    // 2
			archiveData.Content, // 3
		},
		Language:   currentSite.System.Language,
		ToLanguage: req.ToLanguage,
	}
	result, err := currentSite.AnqiTranslateString(transReq)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 更新文档
	archive.Title = result.Text[0]
	archive.Description = result.Text[1]
	archive.Keywords = result.Text[2]
	tx := currentSite.DB
	if isDraft {
		tx = tx.Model(&model.ArchiveDraft{})
	} else {
		tx = tx.Model(&model.Archive{})
	}
	tx.Where("id = ?", archive.Id).UpdateColumns(map[string]interface{}{
		"title":       archive.Title,
		"description": archive.Description,
		"keywords":    archive.Keywords,
	})
	// 再保存内容
	archiveData.Content = result.Text[3]
	currentSite.DB.Save(archiveData)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("TranslationHasFinished"),
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

	isDraft := false
	archive, err := currentSite.GetArchiveById(req.Id)
	if err != nil {
		// 可能是 草稿
		archiveDraft, err := currentSite.GetArchiveDraftById(req.Id)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		isDraft = true
		archive = &archiveDraft.Archive
	}

	err = currentSite.AnqiAiPseudoArticle(archive, isDraft)
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
