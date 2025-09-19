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
	currentSite := provider.CurrentSubSite(ctx)
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
	currentSite := provider.CurrentSubSite(ctx)
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
			w2 := provider.GetWebsite(currentSite.Id)
			w2.PluginUploadFiles = append(w2.PluginUploadFiles[:i], currentSite.PluginUploadFiles[i+1:]...)
		}
	}

	if fileName == "" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("IncorrectFile"),
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

	currentSite.AddAdminLog(ctx, ctx.Tr("DeleteUploadVerificationFileLog", fileName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("DeleteSuccessful"),
	})
}

// PluginFileUploadUpload
// 上传，只允许上传txt,htm,html
func PluginFileUploadUpload(ctx iris.Context) {
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

	info.Filename = strings.ReplaceAll(info.Filename, "..", "")
	info.Filename = strings.ReplaceAll(info.Filename, "/", "")
	info.Filename = strings.ReplaceAll(info.Filename, "\\", "")

	ext := path.Ext(info.Filename)

	if ext != ".txt" && ext != ".htm" && ext != ".html" && ext != ".xml" {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("OnlyAllowUploadOfTxtHtmHtmlXml"),
		})
		return
	}

	filePath := fmt.Sprintf(currentSite.PublicPath + info.Filename)
	buff, err := io.ReadAll(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("ReadFailed"),
		})
		return
	}

	err = os.WriteFile(filePath, buff, 0644)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("FileSaveFailed"),
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

	w2 := provider.GetWebsite(currentSite.Id)
	if !exists {
		//追加
		w2.PluginUploadFiles = append(w2.PluginUploadFiles, config.PluginUploadFile{
			Hash:        library.Md5(info.Filename),
			FileName:    info.Filename,
			CreatedTime: time.Now().Unix(),
		})
	}

	err = w2.SaveSettingValue(provider.UploadFilesSettingKey, w2.PluginUploadFiles)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	// 上传到静态服务器
	_ = w2.SyncHtmlCacheToStorage(filePath, filepath.Base(filePath))

	currentSite.AddAdminLog(ctx, ctx.Tr("UploadVerificationFileLog", info.Filename))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("FileUploadCompleted"),
	})
}
