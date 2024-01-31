package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"time"
)

func GetDesignList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	// 读取 设计列表
	designList := currentSite.GetDesignList()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": designList,
	})
}

func GetDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改模板信息：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "修改成功",
	})
}

func UseDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("启用新模板：%s", req.Package))
	// 重载模板
	config.RestartChan <- 0
	time.Sleep(1 * time.Second)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "切换成功",
	})
}

func DeleteDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除模板：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func DownloadDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

	err = currentSite.UploadDesignZip(file, info)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("上传模板：%s", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "上传成功",
	})
}

func BackupDesignData(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("备份模板数据：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "数据备份成功",
	})
}

func RestoreDesignData(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
		err := currentSite.BackupData()
		if err != nil {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  err.Error(),
			})
			return
		}
		currentSite.AddAdminLog(ctx, fmt.Sprintf("备份数据"))
	}
	if req.AutoCleanup {
		currentSite.CleanupWebsiteData(false)
		currentSite.AddAdminLog(ctx, fmt.Sprintf("一键清空网站数据"))
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("初始化模板数据：%s", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "数据初始化成功",
	})
}

func UploadDesignFile(ctx iris.Context) {
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("上传模板文件：%s", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "上传成功",
	})
}

func GetDesignFileDetail(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除模板文件历史：%s => %s", req.Package, req.Filepath))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func RestoreDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("从历史恢复模板文件：%s => %s", req.Package, req.Filepath))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "替换成功",
		"data": fileInfo,
	})
}

func SaveDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

	currentSite.AddAdminLog(ctx, fmt.Sprintf("修改模板文件：%s => %s", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "修改成功",
	})
}

func CopyDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("复制模板文件：%s => %s", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "复制成功",
	})
}

func DeleteDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	currentSite.AddAdminLog(ctx, fmt.Sprintf("删除模板文件：%s => %s", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "删除成功",
	})
}

func GetDesignTemplateFiles(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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
	docs := provider.DesignDocs

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": docs,
	})
}

func GetDesignTplHelpers(ctx iris.Context) {
	docs := provider.DesignTplHelpers

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": docs,
	})
}
