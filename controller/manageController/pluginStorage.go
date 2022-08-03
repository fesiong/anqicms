package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"strings"
)

func PluginStorageConfig(ctx iris.Context) {
	setting := config.JsonData.PluginStorage

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginStorageConfigForm(ctx iris.Context) {
	var req request.PluginStorageConfigRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	config.JsonData.PluginStorage.StorageUrl = strings.TrimRight(req.StorageUrl, "/")
	config.JsonData.PluginStorage.StorageType = req.StorageType
	config.JsonData.PluginStorage.KeepLocal = req.KeepLocal

	config.JsonData.PluginStorage.AliyunEndpoint = req.AliyunEndpoint
	config.JsonData.PluginStorage.AliyunAccessKeyId = req.AliyunAccessKeyId
	config.JsonData.PluginStorage.AliyunAccessKeySecret = req.AliyunAccessKeySecret
	config.JsonData.PluginStorage.AliyunBucketName = req.AliyunBucketName

	config.JsonData.PluginStorage.TencentSecretId = req.TencentSecretId
	config.JsonData.PluginStorage.TencentSecretKey = req.TencentSecretKey
	config.JsonData.PluginStorage.TencentBucketUrl = req.TencentBucketUrl

	config.JsonData.PluginStorage.QiniuAccessKey = req.QiniuAccessKey
	config.JsonData.PluginStorage.QiniuSecretKey = req.QiniuSecretKey
	config.JsonData.PluginStorage.QiniuBucket = req.QiniuBucket
	config.JsonData.PluginStorage.QiniuRegion = req.QiniuRegion

	config.JsonData.PluginStorage.UpyunBucket = req.UpyunBucket
	config.JsonData.PluginStorage.UpyunOperator = req.UpyunOperator
	config.JsonData.PluginStorage.UpyunPassword = req.UpyunPassword

	err := config.WriteConfig()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新Storage配置"))

	err = provider.Storage.InitBucket()
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
