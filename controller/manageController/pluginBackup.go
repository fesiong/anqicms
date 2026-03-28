package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func PluginBackupList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	list := currentSite.GetBackupList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": list,
	})
}

func PluginBackupDump(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	status, err := currentSite.NewBackup()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	go status.BackupData()

	currentSite.AddAdminLog(ctx, ctx.Tr("BackupData"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("BackupIsStarted"),
	})
}

func PluginBackupStatus(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	status := currentSite.GetBackupStatus()
	if status == nil {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("ThereAreNoActiveTask"),
			"data": nil,
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": status,
	})
}

func PluginBackupRestore(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginBackupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	status, err := currentSite.NewBackup()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	go func() {
		err = status.RestoreData(req.Name)
		if err == nil {
			// 重新读取配置
			currentSite.InitSetting()
			currentSite.AddAdminLog(ctx, ctx.Tr("RestoreDataFromBackup"))
			go func() {
				// 如果切换了模板，需要重启
				config.RestartChan <- 0

				time.Sleep(1 * time.Second)
				// 删除索引
				currentSite.DeleteCache()
				currentSite.RemoveHtmlCache()
				currentSite.CloseFulltext()
				currentSite.InitFulltext(true)
			}()

			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  ctx.Tr("DataRestored"),
			})
		}
	}()

	currentSite.AddAdminLog(ctx, ctx.Tr("BackupData"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("RestoreIsStarted"),
	})
}

func PluginBackupDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
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
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteBackupData"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("Processed"),
	})
}

func PluginBackupImport(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()
	// 增加支持分片上传
	chunks := ctx.PostValueIntDefault("chunks", 0)
	if chunks > 0 {
		chunk := ctx.PostValueIntDefault("chunk", 0)
		fileName := ctx.PostValue("file_name")
		fileMd5 := ctx.PostValue("md5")
		// 使用了分片上传
		tmpFile, err := currentSite.UploadByChunks(file, fileMd5, chunk, chunks)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		if tmpFile == nil {
			// 表示分片上传，不需要返回结果
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  "",
			})
			return
		}
		defer func() {
			tmpName := tmpFile.Name()
			_ = tmpFile.Close()
			_ = os.Remove(tmpName)
		}()
		stat, err := tmpFile.Stat()
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}

		info.Filename = fileName
		info.Size = stat.Size()
		tmpFile.Seek(0, 0)

		if !strings.HasSuffix(info.Filename, ".sql") {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("IncorrectImportedFileFormat"),
			})
			return
		}
		err = currentSite.ImportBackupFile(tmpFile, info.Filename)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("FileSaveFailed"),
			})
			return
		}
	} else {
		if !strings.HasSuffix(info.Filename, ".sql") {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("IncorrectImportedFileFormat"),
			})
			return
		}
		err = currentSite.ImportBackupFile(file, info.Filename)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("FileSaveFailed"),
			})
			return
		}
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("ImportBackupFileLog", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("BackupFileImportCompleted"),
		"data": iris.Map{
			"status": "success",
			"file":   info.Filename,
		},
	})
}

func PluginBackupExport(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
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

	ctx.SendFile(filePath, currentSite.Host+"-"+filepath.Base(filePath))
}

func PluginBackupCleanup(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginBackupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.CleanupWebsiteData(req.CleanUploads)
	currentSite.AddAdminLog(ctx, ctx.Tr("OneClickClearingOfWebsiteData"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CleanUpCompleted"),
	})
}
