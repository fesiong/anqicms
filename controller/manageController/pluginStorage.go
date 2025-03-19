package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"os"
	"path/filepath"
	"strings"
)

func PluginStorageConfig(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	setting := currentSite.PluginStorage

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginStorageConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginStorageConfig
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.PluginStorage.StorageUrl = strings.TrimRight(req.StorageUrl, "/")
	currentSite.PluginStorage.StorageType = req.StorageType
	currentSite.PluginStorage.KeepLocal = req.KeepLocal

	currentSite.PluginStorage.AliyunEndpoint = req.AliyunEndpoint
	currentSite.PluginStorage.AliyunAccessKeyId = req.AliyunAccessKeyId
	currentSite.PluginStorage.AliyunAccessKeySecret = req.AliyunAccessKeySecret
	currentSite.PluginStorage.AliyunBucketName = req.AliyunBucketName

	currentSite.PluginStorage.TencentSecretId = req.TencentSecretId
	currentSite.PluginStorage.TencentSecretKey = req.TencentSecretKey
	currentSite.PluginStorage.TencentBucketUrl = req.TencentBucketUrl

	currentSite.PluginStorage.QiniuAccessKey = req.QiniuAccessKey
	currentSite.PluginStorage.QiniuSecretKey = req.QiniuSecretKey
	currentSite.PluginStorage.QiniuBucket = req.QiniuBucket
	currentSite.PluginStorage.QiniuRegion = req.QiniuRegion

	currentSite.PluginStorage.UpyunBucket = req.UpyunBucket
	currentSite.PluginStorage.UpyunOperator = req.UpyunOperator
	currentSite.PluginStorage.UpyunPassword = req.UpyunPassword

	currentSite.PluginStorage.FTPHost = req.FTPHost
	currentSite.PluginStorage.FTPPort = req.FTPPort
	currentSite.PluginStorage.FTPUsername = req.FTPUsername
	currentSite.PluginStorage.FTPPassword = req.FTPPassword
	currentSite.PluginStorage.FTPWebroot = strings.TrimRight(req.FTPWebroot, "\\/")

	currentSite.PluginStorage.SSHHost = req.SSHHost
	currentSite.PluginStorage.SSHPort = req.SSHPort
	currentSite.PluginStorage.SSHUsername = req.SSHUsername
	currentSite.PluginStorage.SSHPassword = req.SSHPassword
	currentSite.PluginStorage.SSHPrivateKey = req.SSHPrivateKey
	currentSite.PluginStorage.SSHWebroot = strings.TrimRight(req.SSHWebroot, "\\/")

	err := currentSite.SaveSettingValue(provider.StorageSettingKey, currentSite.PluginStorage)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateStorageConfiguration"))
	w2 := provider.GetWebsite(currentSite.Id)
	w2.InitBucket()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginStorageUploadFile(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)

	file, _, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()
	fileName := "ssh_private_key.key"
	filePath := fmt.Sprintf(currentSite.DataPath + "cert/" + fileName)
	buff, err := io.ReadAll(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("ReadFailed"),
		})
		return
	}

	err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("DirectoryCreationFailed"),
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
	currentSite.PluginStorage.SSHPrivateKey = fileName

	err = currentSite.SaveSettingValue(provider.StorageSettingKey, currentSite.PluginStorage)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UploadSshCertificateFileLog", fileName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("FileUploadCompleted"),
		"data": fileName,
	})
}
