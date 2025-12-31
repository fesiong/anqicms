package manageController

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kataras/iris/v12"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"net/http"
	"regexp"
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

func AuthAiChat(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req provider.AnqiAiRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.ContentType("text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	isFirst := true
	if currentSite.AiGenerateConfig.AiEngine != config.AiEngineDefault {
		if currentSite.AiGenerateConfig.AiEngine == config.AiEngineOpenAI {
			if !currentSite.AiGenerateConfig.ApiValid {
				ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", currentSite.Tr("InterfaceUnavailable")))
				return
			}
			key := currentSite.GetOpenAIKey()
			if key == "" {
				ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", currentSite.Tr("NoAvailableKey")))
				return
			}
			stream, err := currentSite.GetOpenAIStreamResponse(key, req.Prompt)
			if err != nil {
				msg := err.Error()
				re, _ := regexp.Compile(`code: (\d+),`)
				match := re.FindStringSubmatch(msg)
				if len(match) > 1 {
					if match[1] == "401" || match[1] == "429" {
						// Key 已失效
						currentSite.SetOpenAIKeyInvalid(key)
					}
				}
				ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", msg))
				return
			}
			defer stream.Close()
			for {
				resp, err2 := stream.Recv()
				if errors.Is(err2, io.EOF) {
					break
				}
				if err2 != nil {
					err = err2
					fmt.Printf("\nStream error: %v\n", err2)
					break
				}
				tmpData := provider.AnqiAiChatResult{
					Content: resp.Choices[0].Delta.Content,
					Status:  1,
				}
				if isFirst {
					tmpData.Status = 0
					isFirst = false
				}
				tmpDataJson, _ := json.Marshal(tmpData)
				ctx.WriteString(fmt.Sprintf("data: %s\n\n", tmpDataJson))
				ctx.ResponseWriter().Flush()
			}
			if err != nil {
				if strings.Contains(err.Error(), "You exceeded your current quota") {
					currentSite.SetOpenAIKeyInvalid(key)
				}
				ctx.WriteString(fmt.Sprintf("data: {\"status\": 2, \"content\": \"%s\"}\n\n", err.Error()))
			} else {
				time.Sleep(2 * time.Second)
				ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", ""))
			}
			ctx.ResponseWriter().Flush()
		} else if currentSite.AiGenerateConfig.AiEngine == config.AiEngineSpark {
			buf, _, err := provider.GetSparkStream(currentSite.AiGenerateConfig.Spark, req.Prompt)
			if err != nil {
				ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", err.Error()))
				return
			}
			for {
				line := <-buf

				if line == "EOF" {
					break
				}
				tmpData := provider.AnqiAiChatResult{
					Content: line,
					Status:  1,
				}
				if isFirst {
					tmpData.Status = 0
					isFirst = false
				}
				tmpDataJson, _ := json.Marshal(tmpData)
				ctx.WriteString(fmt.Sprintf("data: %s\n\n", tmpDataJson))
				ctx.ResponseWriter().Flush()
			}
			ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", ""))
		} else {
			// 错误
			ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", currentSite.Tr("NoAiGenerationSourceSelected")))
			return
		}
	} else {
		client := &http.Client{
			Timeout: 300 * time.Second,
		}
		buf, _ := json.Marshal(req)
		anqiReq, err := http.NewRequest("POST", provider.AnqiApi+"/ai/chat", bytes.NewReader(buf))
		if err != nil {
			ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", err.Error()))
			return
		}
		anqiReq.Header.Add("token", config.AnqiUser.Token)
		anqiReq.Header.Add("User-Agent", fmt.Sprintf("anqicms/%s", config.Version))
		anqiReq.Header.Add("domain", currentSite.System.BaseUrl)
		resp, err := client.Do(anqiReq)
		if err != nil {
			ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", err.Error()))
			return
		}

		// 开始处理
		defer resp.Body.Close()
		reader := bufio.NewReader(resp.Body)
		for {
			line, err2 := reader.ReadBytes('\n')
			var isEof bool
			if err2 != nil {
				isEof = true
			}
			var aiResponse provider.AnqiAiStreamResult
			err2 = json.Unmarshal(line, &aiResponse)
			if err2 != nil {
				if isEof {
					time.Sleep(1 * time.Second)
					ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", ""))
					break
				}
				continue
			}
			tmpData := provider.AnqiAiChatResult{
				Content: aiResponse.Data,
				Status:  1,
			}
			if isFirst {
				tmpData.Status = 0
				isFirst = false
			}
			tmpDataJson, _ := json.Marshal(tmpData)
			ctx.WriteString(fmt.Sprintf("data: %s\n\n", tmpDataJson))
			ctx.ResponseWriter().Flush()

			if aiResponse.Code != 0 {
				ctx.WriteString(fmt.Sprintf("event: close\ndata: {\"status\": 2, \"content\": \"%s\"}\n\n", aiResponse.Msg))
				return
			}
		}
	}
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
