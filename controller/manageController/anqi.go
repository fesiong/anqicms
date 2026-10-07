package manageController

import (
	"encoding/base64"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// AnqiLogin 安企云账号登录，将账号密码提交到安企云服务进行验证。
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

// GetAnqiInfo 异步检查安企云登录状态并返回当前授权信息。
func GetAnqiInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	go currentSite.AnqiCheckLogin(false)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": provider.GetAuthInfo(),
	})
}

// CheckAnqiInfo 同步检查安企云登录状态并返回当前授权信息。
func CheckAnqiInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentSite.AnqiCheckLogin(true)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": provider.GetAuthInfo(),
	})
}

// AnqiUploadAttachment 上传附件并提交到安企云，返回附件信息。
//
// 参数说明：
//   - 表单字段 "file": 上传的文件。
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

// AnqiShareTemplate 将当前站点模板分享到安企云模板库。
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

// AnqiDownloadTemplate 从安企云模板库下载指定模板并应用到当前站点。
func AnqiDownloadTemplate(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AnqiDownloadTemplateRequest
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

// AnqiSendFeedback 向安企云提交使用反馈，包括标题、类型、内容和截图。
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

// AuthExtractKeywords 调用安企云 AI 接口从指定文本中提取关键词。
//
// 参数说明：
//   - 请求体 "text": 需要提取关键词的文本。
//   - 请求体 "num": 提取的关键词数量。
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

// AuthExtractDescription 调用安企云 AI 接口从指定文本中提取摘要描述。
//
// 参数说明：
//   - 请求体 "text": 需要提取摘要的文本。
//   - 请求体 "num": 摘要长度。
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

// AnqiTranslateArticle 调用安企云翻译接口，将指定文章（或草稿）的标题、描述、关键词和内容翻译为目标语言并保存。
func AnqiTranslateArticle(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.TranslateArticleRequest
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

// AnqiAiPseudoArticle 调用安企云 AI 对指定文章（或草稿）进行伪原创处理，处理任务加入计划异步执行。
//
// 参数说明：
//   - 请求体 "id": 文档 ID，支持正式文档或草稿。
func AnqiAiPseudoArticle(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AiPseudoArticleRequest
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

// AuthAiGenerateStream 调用安企云 AI 发起流式内容生成任务，返回流 ID 供后续读取。
//
// 参数说明：
//   - 请求体 "title": 关键词标题。
//   - 请求体 "demand": AI 生成的额外要求。
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

// AuthAiGenerateStreamData 根据流 ID 轮询读取 AI 流式生成的当前内容，返回内容、提示信息及是否已完成。
//
// 参数说明：
//   - 查询参数 "stream_id": 流式生成任务 ID。
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

// AuthAiGenerateImage 调用安企云 AI 文生图/图生图接口，根据提示词提交生图请求并返回结果。
//
// 参数说明：
//   - 请求体 "prompt": 生图提示词，不能为空。
//   - 请求体 "image": 参考图片地址（图生图时使用，支持URL/base64编码）。
//   - 请求体 "size": 生图尺寸，如："1024x1024"。
//   - 请求体 "type": 生图类型，可选值：0 = 文生图，2 = 图生图。
func AuthAiGenerateImage(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.AnqiImageAiRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Prompt == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  currentSite.Tr("PromptCannotBeEmpty"),
		})
		return
	}
	if req.Image != "" {
		resp, err := library.GetURLData(req.Image, "", 30)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		// 生成base64字符串
		req.Image = base64.StdEncoding.EncodeToString([]byte(resp.Body))
	}
	// 开始提交生图
	aiResp, err := currentSite.AnqiGetImageAiResponse(&req)
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
		"data": aiResp,
	})
}

type AuthAiImageConfirmRequest struct {
	Action    int    `json:"action"`     // 0: 舍弃 1: 保存
	Url       string `json:"url"`        // 图片地址
	Title     string `json:"title"`      // 标题
	Replace   bool   `json:"replace"`    // 是否替换已有附件
	ReplaceId int    `json:"replace_id"` // 被替换的附件 id
}

// AuthAiGenerateImageConfirm 确认 AI 生成的图片：action 为 1 时将图片下载保存为附件，支持替换已有附件；为 0 时仅舍弃。
//
// 参数说明：
//   - 请求体 "action": 操作，0 为舍弃，1 为保存。
//   - 请求体 "url": 图片地址。
//   - 请求体 "title": 保存时的文件标题。
//   - 请求体 "replace": 是否替换已有附件。
//   - 请求体 "replaceId": 被替换的附件 ID。
func AuthAiGenerateImageConfirm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req AuthAiImageConfirmRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Action == 1 {
		// 保存到 数据库
		if req.Replace && req.ReplaceId > 0 {
			attach, err := currentSite.GetAttachmentById(uint(req.ReplaceId))
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
			if req.Title == "" {
				req.Title = attach.FileName
			}
		} else {
			req.ReplaceId = 0
		}
		if len(req.Title) == 0 {
			req.Title = filepath.Base(req.Url)
		}

		_, err := currentSite.DownloadRemoteImage(req.Url, req.Title, uint(req.ReplaceId))
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
	})
}

// AuthAiGenerateImageHistories 分页获取 AI 生图历史记录。
//
// 参数说明：
//   - 查询参数 "current": 当前页码，默认为 1。
//   - 查询参数 "pageSize": 每页数量，默认为 20。
func AuthAiGenerateImageHistories(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)

	histories, total := currentSite.AnqiGetAiGenerateImageHistories(currentPage, pageSize)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  histories,
	})
}

// RestartAnqicms 重启 AnQiCMS 服务进程（通过重启信号通道，3 秒后由守护逻辑重新拉起）。
func RestartAnqicms(ctx iris.Context) {
	// first need to stop iris
	config.RestartChan <- config.RestartConfig{Code: 1, SiteId: 0}

	time.Sleep(3 * time.Second)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("RestartSuccessfully"),
	})
}
