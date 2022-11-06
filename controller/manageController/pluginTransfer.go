package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"os"
	"time"
)

func GetTransferTask(ctx iris.Context) {
	task := provider.GetTransferTask()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": task,
	})
}

func DownloadClientFile(ctx iris.Context) {
	var req request.TransferWebsite
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 下载指定的文件
	clientFile := config.ExecPath + "clientFiles/" + req.Provider + "2anqicms.php"
	if req.Provider == "train" {
		clientFile = config.ExecPath + "clientFiles/train2anqicms.wpm"
	}
	_, err := os.Stat(clientFile)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.ServeFile(clientFile)
}

func CreateTransferTask(ctx iris.Context) {
	var req request.TransferWebsite
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	task, err := provider.CreateTransferTask(&req)
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
		"data": task,
	})
}

func TransferWebData(ctx iris.Context) {
	task := provider.GetTransferTask()
	if task == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "没有可执行的任务",
		})
		return
	}
	go task.TransferWebData()

	time.Sleep(1 * time.Second)
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "任务正在执行中",
	})
}
