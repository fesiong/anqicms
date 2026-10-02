package manageController

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// PluginBackupList 获取网站备份文件列表。
func PluginBackupList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	list := currentSite.GetBackupList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": list,
	})
}

// PluginBackupDump 创建网站数据备份，异步执行备份任务。
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

// PluginBackupStatus 获取当前备份/恢复任务的执行状态。
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

// PluginBackupRestore 从指定备份文件恢复网站数据，异步执行，恢复后重载配置并清理缓存。
//
// 参数说明：
//   - 请求体 "name": 备份文件名称。
func PluginBackupRestore(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginRestoreRequest
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
				config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}

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

// PluginBackupDelete 删除指定的网站备份文件。
//
// 参数说明：
//   - 请求体 "name": 备份文件名称。
func PluginBackupDelete(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginRestoreRequest
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

// PluginBackupImport 导入网站备份文件。
//
// 参数说明：
//   - 表单参数 "file": 导入的备份文件。
//   - 表单参数 "chunks": 分片总数，分片上传的时候需要使用。
//   - 表单参数 "chunk": 当前分片序号，分片上传的时候需要使用。
//   - 表单参数 "file_name": 导入的备份文件名，分片上传的时候需要使用。
//   - 表单参数 "md5": 备份文件的 md5 值，分片上传的时候需要使用。
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

		if !strings.HasSuffix(info.Filename, ".sql") && !strings.HasSuffix(info.Filename, ".zip") {
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
		if !strings.HasSuffix(info.Filename, ".sql") && !strings.HasSuffix(info.Filename, ".zip") {
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

// PluginBackupExport，但通过 query 读取 name 和 token，
// 这样前端可以用 <a href> / window.open 触发浏览器原生下载（流式落盘，不占内存）。
// 浏览器原生下载无法自定义 header，因此这里允许通过 query 传递 admin token。
//
// 参数说明：
//   - 查询参数 "name": 备份文件名称。
func PluginBackupExport(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	name := ctx.URLParam("name")
	if name == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("BackupFileDoesNotExist"),
		})
		return
	}
	filePath, err := currentSite.GetBackupFilePath(name)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.SendFile(filePath, currentSite.Host+"-"+filepath.Base(filePath))
}

// PluginBackupCleanup 一键清空网站数据，可选择同时清空上传文件。
func PluginBackupCleanup(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginCleanupRequest
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

// PluginBackupRemark 修改网站备份文件的备注。
func PluginBackupRemark(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginBackupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	if req.Name == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("BackupFileDoesNotExist"),
		})
		return
	}
	err := currentSite.SetBackupRemark(req.Name, req.Remark)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateBackupRemark", req.Name))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("Processed"),
	})
}
