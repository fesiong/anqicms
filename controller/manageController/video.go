package manageController

import (
	"encoding/json"
	"sync"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

func SettingVideoFFmpeg(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": provider.DetectFFmpeg(),
	})
}

func SettingVideoFFmpegInstall(ctx iris.Context) {
	ctx.ContentType("text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")
	writer := ctx.ResponseWriter()
	var mu sync.Mutex
	emit := func(event provider.FFmpegInstallEvent) {
		payload, err := json.Marshal(event)
		if err != nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		_, _ = writer.Write([]byte("event: " + event.Stage + "\ndata: " + string(payload) + "\n\n"))
		writer.Flush()
	}
	err := provider.InstallFFmpeg(ctx.Request().Context(), emit)
	if err != nil {
		emit(provider.FFmpegInstallEvent{Stage: "error", Message: err.Error()})
	}
}

func SettingVideoTaskList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	status := ctx.URLParamIntDefault("status", -1)
	page := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	tasks, total, err := currentSite.ListVideoTasks(status, page, pageSize)
	if err != nil {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": err.Error()})
		return
	}
	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"limit": pageSize,
		"data":  tasks,
	})
}

func SettingVideoTaskRetry(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.VideoTaskRetry
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": err.Error()})
		return
	}
	if req.Id == 0 {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": "视频任务 ID 不能为空"})
		return
	}
	task, err := currentSite.RetryVideoTask(req.Id)
	if err != nil {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": err.Error()})
		return
	}
	ctx.JSON(iris.Map{"code": config.StatusOK, "msg": "已重新排队", "data": task})
}

func SettingVideoTaskHistorical(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.VideoTaskHistorical
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": err.Error()})
		return
	}
	created, retried, skipped, err := currentSite.EnqueueHistoricalVideoTasks(req.Force)
	if err != nil {
		ctx.JSON(iris.Map{"code": config.StatusFailed, "msg": err.Error()})
		return
	}
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "批量任务已入队",
		"data": iris.Map{
			"created": created,
			"retried": retried,
			"skipped": skipped,
		},
	})
}
