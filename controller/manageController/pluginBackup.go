package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
	"time"
)

func PluginBackupList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	list := currentSite.GetBackupList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": list,
	})
}

func PluginBackupDump(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	err := currentSite.BackupData()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("备份数据"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "备份已完成",
	})
}

func PluginBackupRestore(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginBackupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.RestoreData(req.Name)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 重新读取配置
	currentSite.InitSetting()
	currentSite.AddAdminLog(ctx, fmt.Sprintf("从备份中恢复数据"))
	go func() {
		// 如果切换了模板，需要重启
		config.RestartChan <- false

		time.Sleep(1 * time.Second)
		// 删除索引
		currentSite.DeleteCache()
		currentSite.CloseFulltext()
		currentSite.InitFulltext()
	}()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "数据已恢复",
	})
}

func PluginBackupDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginBackupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	err := currentSite.DeleteBackupData(req.Name)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除备份数据"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "已处理",
	})
}

func PluginBackupImport(ctx iris.Context) {
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

	if !strings.HasSuffix(info.Filename, ".sql") {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "导入的文件格式不正确",
		})
		return
	}

	err = currentSite.ImportBackupFile(file, info.Filename)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "文件保存失败",
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("导入备份文件：%s", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "备份文件导入完成",
	})
}

func PluginBackupExport(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginBackupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	filePath, err := currentSite.GetBackupFilePath(req.Name)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.SendFile(filePath, "")
}
