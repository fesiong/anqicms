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

func PluginHtmlCacheConfig(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	pluginHtmlCache := currentSite.PluginHtmlCache

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginHtmlCache,
	})
}

func PluginHtmlCacheConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginHtmlCache
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	currentSite.PluginHtmlCache.Open = req.Open
	currentSite.PluginHtmlCache.IndexCache = req.IndexCache
	currentSite.PluginHtmlCache.ListCache = req.ListCache
	currentSite.PluginHtmlCache.DetailCache = req.DetailCache
	// storage 部分
	currentSite.PluginHtmlCache.StorageUrl = strings.TrimRight(req.StorageUrl, "/")
	currentSite.PluginHtmlCache.StorageType = req.StorageType

	currentSite.PluginHtmlCache.AliyunEndpoint = req.AliyunEndpoint
	currentSite.PluginHtmlCache.AliyunAccessKeyId = req.AliyunAccessKeyId
	currentSite.PluginHtmlCache.AliyunAccessKeySecret = req.AliyunAccessKeySecret
	currentSite.PluginHtmlCache.AliyunBucketName = req.AliyunBucketName

	currentSite.PluginHtmlCache.TencentSecretId = req.TencentSecretId
	currentSite.PluginHtmlCache.TencentSecretKey = req.TencentSecretKey
	currentSite.PluginHtmlCache.TencentBucketUrl = req.TencentBucketUrl

	currentSite.PluginHtmlCache.QiniuAccessKey = req.QiniuAccessKey
	currentSite.PluginHtmlCache.QiniuSecretKey = req.QiniuSecretKey
	currentSite.PluginHtmlCache.QiniuBucket = req.QiniuBucket
	currentSite.PluginHtmlCache.QiniuRegion = req.QiniuRegion

	currentSite.PluginHtmlCache.UpyunBucket = req.UpyunBucket
	currentSite.PluginHtmlCache.UpyunOperator = req.UpyunOperator
	currentSite.PluginHtmlCache.UpyunPassword = req.UpyunPassword

	currentSite.PluginHtmlCache.FTPHost = req.FTPHost
	currentSite.PluginHtmlCache.FTPPort = req.FTPPort
	currentSite.PluginHtmlCache.FTPUsername = req.FTPUsername
	currentSite.PluginHtmlCache.FTPPassword = req.FTPPassword
	currentSite.PluginHtmlCache.FTPWebroot = strings.TrimRight(req.FTPWebroot, "\\/")

	currentSite.PluginHtmlCache.SSHHost = req.SSHHost
	currentSite.PluginHtmlCache.SSHPort = req.SSHPort
	currentSite.PluginHtmlCache.SSHUsername = req.SSHUsername
	currentSite.PluginHtmlCache.SSHPassword = req.SSHPassword
	currentSite.PluginHtmlCache.SSHPrivateKey = req.SSHPrivateKey
	currentSite.PluginHtmlCache.SSHWebroot = strings.TrimRight(req.SSHWebroot, "\\/")

	err := currentSite.SaveSettingValue(provider.HtmlCacheSettingKey, currentSite.PluginHtmlCache)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("更新缓存配置"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "配置已更新",
	})
}

func PluginHtmlCacheBuild(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var req config.PluginHtmlCache
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	//开始生成
	go currentSite.BuildHtmlCache(ctx)

	currentSite.AddAdminLog(ctx, fmt.Sprintf("手动生成缓存"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "生成任务执行中",
	})
}

func PluginHtmlCacheBuildStatus(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	status := currentSite.GetHtmlCacheStatus()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": status,
	})
}

func PluginCleanHtmlCache(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	currentSite.RemoveHtmlCache()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
	})
}

func PluginHtmlCacheUploadFile(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)

	file, _, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	defer file.Close()
	fileName := "htmlcache_ssh_private_key.key"
	filePath := fmt.Sprintf(currentSite.DataPath + "cert/" + fileName)
	buff, err := io.ReadAll(file)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "读取失败",
		})
		return
	}

	err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "目录创建失败",
		})
		return
	}
	err = os.WriteFile(filePath, buff, 0644)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "文件保存失败",
		})
		return
	}
	currentSite.PluginHtmlCache.SSHPrivateKey = fileName

	err = currentSite.SaveSettingValue(provider.HtmlCacheSettingKey, currentSite.PluginHtmlCache)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, fmt.Sprintf("上传ssh证书文件：%s", fileName))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "文件已上传完成",
		"data": fileName,
	})
}
