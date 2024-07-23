package manageController

import (
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"path/filepath"
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

	currentSite.AddAdminLog(ctx, ctx.Tr("备份数据"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("备份已完成"),
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
	currentSite.AddAdminLog(ctx, ctx.Tr("从备份中恢复数据"))
	go func() {
		// 如果切换了模板，需要重启
		config.RestartChan <- 0

		time.Sleep(1 * time.Second)
		// 删除索引
		currentSite.DeleteCache()
		currentSite.RemoveHtmlCache()
		currentSite.CloseFulltext()
		currentSite.InitFulltext()
	}()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("数据已恢复"),
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
	currentSite.AddAdminLog(ctx, ctx.Tr("删除备份数据"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("已处理"),
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
			"msg":  ctx.Tr("导入的文件格式不正确"),
		})
		return
	}

	err = currentSite.ImportBackupFile(file, info.Filename)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("文件保存失败"),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("导入备份文件：%s", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("备份文件导入完成"),
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

	ctx.SendFile(filePath, currentSite.Host+"-"+filepath.Base(filePath))
}

func PluginBackupCleanup(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginBackupRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.CleanupWebsiteData(req.CleanUploads)
	currentSite.AddAdminLog(ctx, ctx.Tr("一键清空网站数据"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("清理完成"),
	})
}
