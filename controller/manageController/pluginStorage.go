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
	w2 := provider.GetWebsite(currentSite.Id)

	w2.PluginStorage.StorageUrl = strings.TrimRight(req.StorageUrl, "/")
	w2.PluginStorage.StorageType = req.StorageType
	w2.PluginStorage.KeepLocal = req.KeepLocal

	w2.PluginStorage.AliyunEndpoint = req.AliyunEndpoint
	w2.PluginStorage.AliyunAccessKeyId = req.AliyunAccessKeyId
	w2.PluginStorage.AliyunAccessKeySecret = req.AliyunAccessKeySecret
	w2.PluginStorage.AliyunBucketName = req.AliyunBucketName

	w2.PluginStorage.TencentSecretId = req.TencentSecretId
	w2.PluginStorage.TencentSecretKey = req.TencentSecretKey
	w2.PluginStorage.TencentBucketUrl = req.TencentBucketUrl

	w2.PluginStorage.QiniuAccessKey = req.QiniuAccessKey
	w2.PluginStorage.QiniuSecretKey = req.QiniuSecretKey
	w2.PluginStorage.QiniuBucket = req.QiniuBucket
	w2.PluginStorage.QiniuRegion = req.QiniuRegion

	w2.PluginStorage.UpyunBucket = req.UpyunBucket
	w2.PluginStorage.UpyunOperator = req.UpyunOperator
	w2.PluginStorage.UpyunPassword = req.UpyunPassword

	w2.PluginStorage.GoogleProjectId = req.GoogleProjectId
	w2.PluginStorage.GoogleBucketName = req.GoogleBucketName
	w2.PluginStorage.GoogleCredentialsJson = req.GoogleCredentialsJson

	w2.PluginStorage.S3Region = req.S3Region
	w2.PluginStorage.S3AccessKey = req.S3AccessKey
	w2.PluginStorage.S3SecretKey = req.S3SecretKey
	w2.PluginStorage.S3Bucket = req.S3Bucket

	w2.PluginStorage.FTPHost = req.FTPHost
	w2.PluginStorage.FTPPort = req.FTPPort
	w2.PluginStorage.FTPUsername = req.FTPUsername
	w2.PluginStorage.FTPPassword = req.FTPPassword
	w2.PluginStorage.FTPWebroot = strings.TrimRight(req.FTPWebroot, "\\/")

	w2.PluginStorage.SSHHost = req.SSHHost
	w2.PluginStorage.SSHPort = req.SSHPort
	w2.PluginStorage.SSHUsername = req.SSHUsername
	w2.PluginStorage.SSHPassword = req.SSHPassword
	w2.PluginStorage.SSHPrivateKey = req.SSHPrivateKey
	w2.PluginStorage.SSHWebroot = strings.TrimRight(req.SSHWebroot, "\\/")

	err := w2.SaveSettingValue(provider.StorageSettingKey, w2.PluginStorage)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateStorageConfiguration"))
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
