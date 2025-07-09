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
	currentSite := provider.CurrentSite(ctx)
	task := currentSite.GetTransferTask()

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
	currentSite := provider.CurrentSite(ctx)
	var req request.TransferWebsite
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	w2 := provider.GetWebsite(currentSite.Id)
	task, err := w2.CreateTransferTask(&req)
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

func GetTransferModules(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	task := currentSite.GetTransferTask()
	if task == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("NoExecutableTasks"),
		})
		return
	}

	modules, err := task.GetModules()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// * 需要执行的操作type：
	// -. 同步模型 module
	// -. 同步分类 category
	// -. 同步标签 tag
	// -. 同步锚文本 keyword
	// -. 同步文档 archive
	// -. 同步单页 singlepage
	// -. 同步静态资源 static
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"modules": modules,
			"types": []string{
				"module",
				"category",
				"tag",
				"keyword",
				"archive",
				"singlepage",
				"static",
			},
		},
	})
}

func TransferWebData(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	task := currentSite.GetTransferTask()
	if task == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("NoExecutableTasks"),
		})
		return
	}
	var req request.TransferTypes
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	go task.TransferWebData(&req)

	time.Sleep(1 * time.Second)
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("TaskInProgress"),
	})
}
