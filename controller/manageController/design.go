package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"os"
	"time"
)

func GetDesignList(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	// 读取 设计列表
	designList := currentSite.GetDesignList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": designList,
	})
}

func GetDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := ctx.URLParam("package")

	designInfo, err := currentSite.GetDesignInfo(packageName, true)
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
		"data": designInfo,
	})
}

func SaveDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveDesignInfo(req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if currentSite.System.TemplateName == req.Package {
		// 更改当前
		if currentSite.System.TemplateType != req.TemplateType {
			currentSite.System.TemplateType = req.TemplateType
			err = currentSite.SaveSettingValue(provider.SystemSettingKey, currentSite.System)
			if err != nil {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  err.Error(),
				})
				return
			}
		}
	}
	// 重载模板
	config.RestartChan <- 0
	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyTemplateLog", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ModifySuccessfully"),
	})
}

func UseDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	_, err := currentSite.GetDesignInfo(req.Package, false)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if currentSite.System.TemplateName != req.Package {
		currentSite.System.TemplateName = req.Package
		currentSite.System.TemplateType = req.TemplateType
		err = currentSite.SaveSettingValue(provider.SystemSettingKey, currentSite.System)
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
	}
	currentSite.AddAdminLog(ctx, ctx.Tr("EnableNewTemplateLog", req.Package))
	// 重载模板
	config.RestartChan <- 0
	time.Sleep(1 * time.Second)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SwitchSuccessfully"),
	})
}

func DeleteDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteDesignInfo(req.Package)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 重载模板
	config.RestartChan <- 0
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteTemplateLog", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

func DownloadDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.DesignInfoRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	data, err := currentSite.CreateDesignZip(req.Package)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//读取文件
	ctx.ResponseWriter().Header().Set(context.ContentDispositionHeaderKey, fmt.Sprintf("attachment;filename=%s.zip", req.Package))
	ctx.Binary(data.Bytes())
}

func UploadDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	cover := ctx.FormValue("cover")
	file, info, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()

	err = currentSite.UploadDesignZip(file, info, cover)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UploadTemplateLog", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("UploadSuccessfully"),
	})
}

// CheckUploadDesignInfo 如果文件名重复，则需要确认是否覆盖
func CheckUploadDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := ctx.URLParam("package")
	packagePath := currentSite.RootPath + "template/" + packageName
	_, err := os.Stat(packagePath)
	if err == nil {
		// 已存在
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "",
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
	})
}

func BackupDesignData(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.DesignDataRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.BackupDesignData(req.Package)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("BackupTemplateDataLog", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DataBackupSuccessful"),
	})
}

func RestoreDesignData(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.DesignDataRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if req.AutoCleanup {
		req.AutoBackup = true
	}
	if req.AutoBackup {
		// 如果用户勾选了自动备份
		status, err := currentSite.NewBackup()
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		err = status.BackupData()
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		currentSite.AddAdminLog(ctx, ctx.Tr("BackupData"))
	}
	if req.AutoCleanup {
		currentSite.CleanupWebsiteData(false)
		currentSite.AddAdminLog(ctx, ctx.Tr("OneClickClearingOfWebsiteData"))
	}

	err := currentSite.RestoreDesignData(req.Package)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.RemoveHtmlCache()

	currentSite.AddAdminLog(ctx, ctx.Tr("InitializeTemplateDataLog", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DataInitializationSuccessful"),
	})
}

func UploadDesignFile(ctx iris.Context) {
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

	packageName := ctx.PostValue("package")
	filePath := ctx.PostValue("path")
	fileName := ctx.PostValue("name")
	fileType := ctx.PostValue("type")
	if fileName != "" {
		info.Filename = fileName
	}

	err = currentSite.UploadDesignFile(file, info, packageName, fileType, filePath)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 重载模板
	config.RestartChan <- 0
	currentSite.RemoveHtmlCache()
	currentSite.AddAdminLog(ctx, ctx.Tr("UploadTemplateFileLog", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("UploadSuccessfully"),
	})
}

func GetDesignFileDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := ctx.URLParam("package")
	fileName := ctx.URLParam("path")
	fileType := ctx.URLParam("type")

	fileInfo, err := currentSite.GetDesignFileDetail(packageName, fileName, fileType, true)
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
		"data": fileInfo,
	})
}

func GetDesignFileHistories(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := ctx.URLParam("package")
	fileName := ctx.URLParam("path")
	fileType := ctx.URLParam("type")

	histories := currentSite.GetDesignFileHistories(packageName, fileName, fileType)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": histories,
	})
}

func GetDesignFileHistoryDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := ctx.URLParam("package")
	fileName := ctx.URLParam("path")
	fileType := ctx.URLParam("type")
	historyHash := ctx.URLParam("hash")

	fileInfo, err := currentSite.GetDesignFileHistoryInfo(packageName, fileName, historyHash, fileType)
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
		"data": fileInfo,
	})
}

func DeleteDesignFileHistories(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.RestoreDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteDesignHistoryFile(req.Package, req.Filepath, req.Hash, req.Type)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteTemplateFileHistory", req.Package, req.Filepath))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

func RestoreDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.RestoreDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.RestoreDesignFile(req.Package, req.Filepath, req.Hash, req.Type)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	fileInfo, _ := currentSite.GetDesignFileDetail(req.Package, req.Filepath, req.Type, true)
	// 重载模板
	config.RestartChan <- 0
	currentSite.DeleteCacheIndex()
	currentSite.AddAdminLog(ctx, ctx.Tr("RestoreTemplateFileFromHistory", req.Package, req.Filepath))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ReplaceSuccessfully"),
		"data": fileInfo,
	})
}

func SaveDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SaveDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.SaveDesignFile(req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 重载模板
	config.RestartChan <- 0
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyTemplateFile", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ModifySuccessfully"),
	})
}

func CopyDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.CopyDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.CopyDesignFile(req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 重载模板
	config.RestartChan <- 0
	currentSite.AddAdminLog(ctx, ctx.Tr("CopyTemplateFile", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CopySuccessfully"),
	})
}

func DeleteDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.SaveDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	err := currentSite.DeleteDesignFile(req.Package, req.Path, req.Type)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 重载模板
	config.RestartChan <- 0
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteTemplateFile", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

func GetDesignTemplateFiles(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := currentSite.System.TemplateName
	templates, err := currentSite.GetDesignTemplateFiles(packageName)
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
		"data": templates,
	})
}

func GetDesignDocs(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	docs := currentSite.GetDesignDocs()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": docs,
	})
}

func GetDesignTplHelpers(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	docs := currentSite.GetDesignTplHelpers()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": docs,
	})
}
