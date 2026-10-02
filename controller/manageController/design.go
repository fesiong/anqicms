package manageController

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
)

// sanitizePackageName 验证包名只包含安全字符
func sanitizePackageName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// sanitizeDesignFilePath 清理设计器文件路径，防止路径穿越
func sanitizeDesignFilePath(filePath string) string {
	if strings.Contains(filePath, "..") {
		return ""
	}
	return filePath
}

// GetDesignList 获取当前站点的模板设计列表。
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

// GetDesignInfo 获取指定模板的详细信息，未指定时返回当前使用的模板。
//
// 参数说明：
//   - 查询参数 "package": 模板包名，为空时使用当前站点模板名。
func GetDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := ctx.URLParam("package")
	if packageName == "" {
		packageName = currentSite.System.TemplateName
	}
	if !sanitizePackageName(packageName) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
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

// SaveDesignInfo 保存模板信息，若为当前使用模板则同步模板类型并重载模板。
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

	if !sanitizePackageName(req.Package) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
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
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}
	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyTemplateLog", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ModifySuccessfully"),
	})
}

// UseDesignInfo 启用指定模板，将其设置为站点当前模板并重载模板。
func UseDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.UseDesignRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if !sanitizePackageName(req.Package) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}

	info, err := currentSite.GetDesignInfo(req.Package, false)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if currentSite.System.TemplateName != req.Package {
		currentSite.System.TemplateName = info.Package
		currentSite.System.TemplateType = info.TemplateType
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
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}
	time.Sleep(1 * time.Second)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("SwitchSuccessfully"),
	})
}

// DeleteDesignInfo 删除指定模板并重载模板。
func DeleteDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.UseDesignRequest
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
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteTemplateLog", req.Package))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

// DownloadDesignInfo 将指定模板打包为 zip 文件并下载。
func DownloadDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.UseDesignRequest
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

// UploadDesignInfo 上传模板并解压。
//
// 参数说明：
//   - 表单字段 "file": 模板压缩包文件（必填）。
//   - 表单字段 "cover": 模板封面图地址（可选）。
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
	// 需要重载模板
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}

	currentSite.AddAdminLog(ctx, ctx.Tr("UploadTemplateLog", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("UploadSuccessfully"),
	})
}

// CheckUploadDesignInfo 验证上传的模板文件名（模板包名）和现有模板包名重复，用于提醒是否覆盖
//
// 参数说明：
//   - 模板包名 "package": 模板包名。
func CheckUploadDesignInfo(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := ctx.URLParam("package")
	if !sanitizePackageName(packageName) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
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

// BackupDesignData 备份指定模板的演示数据（最多备份前500条文档等）。
//
// 参数说明：
//   - 请求体 "package": 模板包名。
func BackupDesignData(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.UseDesignRequest
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

// RestoreDesignData 恢复（初始化）指定模板的演示数据，可按需先备份网站数据或清空网站数据。
//
// 参数说明：
//   - 请求体 "package": 模板包名。
//   - 请求体 "auto_backup": 是否自动备份网站数据。
//   - 请求体 "auto_cleanup": 是否一键清空网站数据（为 true 时会先执行备份）。
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

// UploadDesignFile 上传模板文件到指定模板目录，并重载模板、清理缓存。
//
// 参数说明：
//   - 表单文件 "file": 上传的模板文件。
//   - 表单参数 "package": 模板包名。
//   - 表单参数 "path": 文件保存路径。
//   - 表单参数 "name": 保存的文件名，为空时使用原文件名。
//   - 表单参数 "type": 文件类型：static|template。
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
	if !sanitizePackageName(packageName) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
	filePath := ctx.PostValue("path")
	fileName := ctx.PostValue("name")
	if filePath != "" {
		filePath = sanitizeDesignFilePath(filePath)
	}
	if fileName != "" {
		fileName = sanitizeDesignFilePath(fileName)
		if fileName == "" {
			ctx.JSON(iris.Map{
				"code": config.StatusFailed,
				"msg":  ctx.Tr("InvalidFileName"),
			})
			return
		}
		info.Filename = fileName
	}
	fileType := ctx.PostValue("type")

	err = currentSite.UploadDesignFile(file, info, packageName, fileType, filePath)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 重载模板
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}
	currentSite.RemoveHtmlCache()
	currentSite.AddAdminLog(ctx, ctx.Tr("UploadTemplateFileLog", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("UploadSuccessfully"),
	})
}

// GetDesignFileDetail 获取模板中指定文件的详情。
//
// 参数说明：
//   - 查询参数 "package": 模板包名。
//   - 查询参数 "path": 文件路径。
//   - 查询参数 "type": 文件类型：static|template。
func GetDesignFileDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := ctx.URLParam("package")
	if !sanitizePackageName(packageName) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
	fileName := ctx.URLParam("path")
	if fileName != "" {
		fileName = sanitizeDesignFilePath(fileName)
	}
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

// GetDesignFileHistories 获取模板中指定文件的历史版本列表。
//
// 参数说明：
//   - 查询参数 "package": 模板包名。
//   - 查询参数 "path": 文件路径。
//   - 查询参数 "type": 文件类型：static|template。
func GetDesignFileHistories(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := ctx.URLParam("package")
	if !sanitizePackageName(packageName) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
	fileName := ctx.URLParam("path")
	if fileName != "" {
		fileName = sanitizeDesignFilePath(fileName)
	}
	fileType := ctx.URLParam("type")

	histories := currentSite.GetDesignFileHistories(packageName, fileName, fileType)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": histories,
	})
}

// GetDesignFileHistoryDetail 获取模板文件某个历史版本的详情。
//
// 参数说明：
//   - 查询参数 "package": 模板包名。
//   - 查询参数 "path": 文件路径。
//   - 查询参数 "type": 文件类型：static|template。
//   - 查询参数 "hash": 历史版本哈希。
func GetDesignFileHistoryDetail(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	packageName := ctx.URLParam("package")
	if !sanitizePackageName(packageName) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
	fileName := ctx.URLParam("path")
	if fileName != "" {
		fileName = sanitizeDesignFilePath(fileName)
	}
	fileType := ctx.URLParam("type")
	historyHash := ctx.URLParam("hash")
	if historyHash != "" {
		historyHash = sanitizeDesignFilePath(historyHash)
	}

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

// DeleteDesignFileHistories 删除模板文件的历史版本。
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

	if !sanitizePackageName(req.Package) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
	req.Filepath = sanitizeDesignFilePath(req.Filepath)
	req.Hash = sanitizeDesignFilePath(req.Hash)

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

// RestoreDesignFile 将模板文件恢复到指定历史版本，并重载模板、清理缓存。
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

	if !sanitizePackageName(req.Package) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
	req.Filepath = sanitizeDesignFilePath(req.Filepath)
	req.Hash = sanitizeDesignFilePath(req.Hash)

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
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}
	currentSite.DeleteCacheIndex()
	currentSite.AddAdminLog(ctx, ctx.Tr("RestoreTemplateFileFromHistory", req.Package, req.Filepath))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ReplaceSuccessfully"),
		"data": fileInfo,
	})
}

// SaveDesignFile 保存模板文件内容，并重载模板、清理缓存索引。
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

	if !sanitizePackageName(req.Package) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
	req.Path = sanitizeDesignFilePath(req.Path)

	err := currentSite.SaveDesignFile(req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 重载模板
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}
	currentSite.DeleteCacheIndex()

	currentSite.AddAdminLog(ctx, ctx.Tr("ModifyTemplateFile", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ModifySuccessfully"),
	})
}

// CopyDesignFile 复制模板文件为新文件，并重载模板。
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

	if !sanitizePackageName(req.Package) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
	req.Path = sanitizeDesignFilePath(req.Path)

	err := currentSite.CopyDesignFile(req)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 重载模板
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}
	currentSite.AddAdminLog(ctx, ctx.Tr("CopyTemplateFile", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CopySuccessfully"),
	})
}

// DeleteDesignFile 删除指定模板文件，并重载模板。
func DeleteDesignFile(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.DeleteDesignFileRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	if !sanitizePackageName(req.Package) {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("InvalidPackageName"),
		})
		return
	}
	req.Path = sanitizeDesignFilePath(req.Path)

	err := currentSite.DeleteDesignFile(req.Package, req.Path, req.Type)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 重载模板
	config.RestartChan <- config.RestartConfig{Code: 0, SiteId: currentSite.Id}
	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteTemplateFile", req.Package, req.Path))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

// GetDesignTemplateFiles 获取当前使用模板的模板文件列表。
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

// GetDesignDocs 获取模板开发文档列表。
func GetDesignDocs(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	docs := currentSite.GetDesignDocs()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": docs,
	})
}

// GetDesignTplHelpers 获取模板开发文档助手内容。
func GetDesignTplHelpers(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	docs := currentSite.GetDesignTplHelpers()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": docs,
	})
}
