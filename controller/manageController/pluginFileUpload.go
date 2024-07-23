package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func PluginFileUploadList(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	uploadFiles := currentSite.PluginUploadFiles

	for i := range uploadFiles {
		uploadFiles[i].Link = currentSite.System.BaseUrl + "/" + uploadFiles[i].FileName
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": uploadFiles,
	})
}

func PluginFileUploadDelete(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req request.PluginFileUploadDelete
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	uploadFiles := currentSite.PluginUploadFiles

	fileName := ""
	for i, v := range uploadFiles {
		if v.Hash == req.Hash {
			fileName = v.FileName

			currentSite.PluginUploadFiles = append(currentSite.PluginUploadFiles[:i], currentSite.PluginUploadFiles[i+1:]...)
		}
	}

	if fileName == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("文件不正确"),
		})
		return
	}

	//执行物理删除
	err := os.Remove(currentSite.PublicPath + fileName)

	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//更新文件列表
	err = currentSite.SaveSettingValue(provider.UploadFilesSettingKey, currentSite.PluginUploadFiles)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("删除上传验证文件：%s", fileName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("删除成功"),
	})
}

// PluginFileUploadUpload
// 上传，只允许上传txt,htm,html
func PluginFileUploadUpload(ctx iris.Context) {
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

	info.Filename = strings.ReplaceAll(info.Filename, "..", "")
	info.Filename = strings.ReplaceAll(info.Filename, "/", "")
	info.Filename = strings.ReplaceAll(info.Filename, "\\", "")

	ext := path.Ext(info.Filename)

	if ext != ".txt" && ext != ".htm" && ext != ".html" && ext != ".xml" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("只允许上传txt/htm/html/xml"),
		})
		return
	}

	filePath := fmt.Sprintf(currentSite.PublicPath + info.Filename)
	buff, err := io.ReadAll(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("读取失败"),
		})
		return
	}

	err = os.WriteFile(filePath, buff, 0644)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("文件保存失败"),
		})
		return
	}

	//检查是否已经在
	exists := false
	for _, v := range currentSite.PluginUploadFiles {
		if v.FileName == info.Filename {
			exists = true
		}
	}

	if !exists {
		//追加
		currentSite.PluginUploadFiles = append(currentSite.PluginUploadFiles, config.PluginUploadFile{
			Hash:        library.Md5(info.Filename),
			FileName:    info.Filename,
			CreatedTime: time.Now().Unix(),
		})
	}

	err = currentSite.SaveSettingValue(provider.UploadFilesSettingKey, currentSite.PluginUploadFiles)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 上传到静态服务器
	_ = currentSite.SyncHtmlCacheToStorage(filePath, filepath.Base(filePath))

	currentSite.AddAdminLog(ctx, ctx.Tr("上传验证文件：%s", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("文件已上传完成"),
	})
}
