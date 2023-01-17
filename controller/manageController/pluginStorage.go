package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/provider"
	"strings"
)

func PluginStorageConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	setting := currentSite.PluginStorage

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": setting,
	})
}

func PluginStorageConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
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

	err := currentSite.SaveSettingValue(provider.StorageSettingKey, currentSite.PluginStorage)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新Storage配置"))

	currentSite.InitBucket()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}
