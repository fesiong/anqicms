package manageController

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/request"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func PluginHtmlCacheConfig(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	pluginHtmlCache := currentSite.PluginHtmlCache

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": pluginHtmlCache,
	})
}

func PluginHtmlCacheConfigForm(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginHtmlCache
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	w2 := provider.GetWebsite(currentSite.Id)

	w2.PluginHtmlCache.Open = req.Open
	w2.PluginHtmlCache.IndexCache = req.IndexCache
	w2.PluginHtmlCache.ListCache = req.ListCache
	w2.PluginHtmlCache.DetailCache = req.DetailCache
	// storage 部分
	w2.PluginHtmlCache.KeepLocal = false
	w2.PluginHtmlCache.StorageUrl = strings.TrimRight(req.StorageUrl, "/")
	w2.PluginHtmlCache.StorageType = req.StorageType

	w2.PluginHtmlCache.AliyunEndpoint = req.AliyunEndpoint
	w2.PluginHtmlCache.AliyunAccessKeyId = req.AliyunAccessKeyId
	w2.PluginHtmlCache.AliyunAccessKeySecret = req.AliyunAccessKeySecret
	w2.PluginHtmlCache.AliyunBucketName = req.AliyunBucketName

	w2.PluginHtmlCache.TencentSecretId = req.TencentSecretId
	w2.PluginHtmlCache.TencentSecretKey = req.TencentSecretKey
	w2.PluginHtmlCache.TencentBucketUrl = req.TencentBucketUrl

	w2.PluginHtmlCache.QiniuAccessKey = req.QiniuAccessKey
	w2.PluginHtmlCache.QiniuSecretKey = req.QiniuSecretKey
	w2.PluginHtmlCache.QiniuBucket = req.QiniuBucket
	w2.PluginHtmlCache.QiniuRegion = req.QiniuRegion

	w2.PluginHtmlCache.UpyunBucket = req.UpyunBucket
	w2.PluginHtmlCache.UpyunOperator = req.UpyunOperator
	w2.PluginHtmlCache.UpyunPassword = req.UpyunPassword

	w2.PluginHtmlCache.GoogleProjectId = req.GoogleProjectId
	w2.PluginHtmlCache.GoogleBucketName = req.GoogleBucketName
	w2.PluginHtmlCache.GoogleCredentialsJson = req.GoogleCredentialsJson

	w2.PluginHtmlCache.S3Region = req.S3Region
	w2.PluginHtmlCache.S3AccessKey = req.S3AccessKey
	w2.PluginHtmlCache.S3SecretKey = req.S3SecretKey
	w2.PluginHtmlCache.S3Bucket = req.S3Bucket
	w2.PluginHtmlCache.S3Endpoint = req.S3Endpoint

	w2.PluginHtmlCache.FTPHost = req.FTPHost
	w2.PluginHtmlCache.FTPPort = req.FTPPort
	w2.PluginHtmlCache.FTPUsername = req.FTPUsername
	w2.PluginHtmlCache.FTPPassword = req.FTPPassword
	w2.PluginHtmlCache.FTPWebroot = strings.TrimRight(req.FTPWebroot, "\\/")

	w2.PluginHtmlCache.SSHHost = req.SSHHost
	w2.PluginHtmlCache.SSHPort = req.SSHPort
	w2.PluginHtmlCache.SSHUsername = req.SSHUsername
	w2.PluginHtmlCache.SSHPassword = req.SSHPassword
	w2.PluginHtmlCache.SSHPrivateKey = req.SSHPrivateKey
	w2.PluginHtmlCache.SSHWebroot = strings.TrimRight(req.SSHWebroot, "\\/")

	err := w2.SaveSettingValue(provider.HtmlCacheSettingKey, w2.PluginHtmlCache)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateCacheConfiguration"))
	w2.InitCacheBucket()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("ConfigurationUpdated"),
	})
}

func PluginHtmlCacheBuild(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req config.PluginHtmlCache
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	w2 := provider.GetWebsite(currentSite.Id)
	//开始生成
	go w2.BuildHtmlCache(ctx)

	currentSite.AddAdminLog(ctx, ctx.Tr("GenerateCacheManually"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("GenerateTaskInProgress"),
	})
}

func PluginHtmlCacheBuildIndex(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	go func() {
		w2 := provider.GetWebsite(currentSite.Id)
		w2.BuildIndexCache()
		w2.HtmlCacheStatus.FinishedTime = time.Now().Unix()
		cachePath := w2.CachePath + "pc"
		_ = w2.SyncHtmlCacheToStorage(cachePath+"/index.html", "index.html")
	}()
	currentSite.AddAdminLog(ctx, ctx.Tr("GenerateHomepageCacheManually"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("GenerateTaskInProgress"),
	})
}

func PluginHtmlCacheBuildCategory(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	go func() {
		w2 := provider.GetWebsite(currentSite.Id)
		w2.BuildModuleCache(ctx)
		w2.BuildCategoryCache(ctx)
		w2.HtmlCacheStatus.FinishedTime = time.Now().Unix()
		cachePath := w2.CachePath + "pc"
		// 更新的html
		_ = w2.ReadAndSendLocalFiles(cachePath)
	}()
	currentSite.AddAdminLog(ctx, ctx.Tr("GenerateColumnCacheManually"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("GenerateTaskInProgress"),
	})
}

func PluginHtmlCacheBuildArchive(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	go func() {
		w2 := provider.GetWebsite(currentSite.Id)
		w2.BuildArchiveCache()
		w2.HtmlCacheStatus.FinishedTime = time.Now().Unix()
		cachePath := w2.CachePath + "pc"
		// 更新的html
		_ = w2.ReadAndSendLocalFiles(cachePath)
	}()
	currentSite.AddAdminLog(ctx, ctx.Tr("GenerateDocumentCacheManually"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("GenerateTaskInProgress"),
	})
}

func PluginHtmlCacheBuildTag(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	go func() {
		w2 := provider.GetWebsite(currentSite.Id)
		w2.BuildTagIndexCache(ctx)
		w2.BuildTagCache(ctx)
		w2.HtmlCacheStatus.FinishedTime = time.Now().Unix()
		cachePath := w2.CachePath + "pc"
		// 更新的html
		_ = w2.ReadAndSendLocalFiles(cachePath)
	}()
	currentSite.AddAdminLog(ctx, ctx.Tr("GenerateTagCacheManually"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("GenerateTaskInProgress"),
	})
}

func PluginHtmlCacheBuildStatus(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	status := currentSite.GetHtmlCacheStatus()

	if status != nil && status.FinishedTime > 0 && !status.Removing {
		status.Removing = true
		time.AfterFunc(30*time.Second, func() {
			w2 := provider.GetWebsite(currentSite.Id)
			w2.HtmlCacheStatus = nil
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": status,
	})
}

func PluginCleanHtmlCache(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	currentSite.RemoveHtmlCache()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
	})
}

func PluginHtmlCacheUploadFile(ctx iris.Context) {
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
	fileName := "htmlcache_ssh_private_key.key"
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
	currentSite.PluginHtmlCache.SSHPrivateKey = fileName

	err = currentSite.SaveSettingValue(provider.HtmlCacheSettingKey, currentSite.PluginHtmlCache)
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

func PluginHtmlCachePush(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	var req request.PluginHtmlCachePushRequest
	if err := ctx.ReadJSON(&req); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	w2 := provider.GetWebsite(currentSite.Id)
	// 开始执行推送
	if len(req.Paths) > 0 {
		// 逐个进行
		for _, v := range req.Paths {
			fullName := currentSite.RootPath + v
			var remotePath string
			if strings.HasPrefix(fullName, currentSite.PublicPath) {
				// 来自public目录
				remotePath = strings.TrimPrefix(fullName, currentSite.PublicPath)
			} else {
				// 来自cache目录, 只传PC目录
				cachePath := currentSite.CachePath + "pc"
				remotePath = strings.TrimPrefix(fullName, cachePath)
			}
			_ = w2.SyncHtmlCacheToStorage(fullName, remotePath)
		}
	} else {
		go func() {
			if req.All {
				// 全量推送，重置所有推送数据
				w2.CleanHtmlPushLog()
			}
			_ = w2.SyncHtmlCacheToStorage("", "")
		}()
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("PushFileToStaticServerManually"))

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("PushTaskInProgress"),
	})
}

func PluginHtmlCachePushStatus(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	status := currentSite.GetHtmlCachePushStatus()

	if status != nil && status.FinishedTime > 0 && !status.Removing {
		status.Removing = true
		time.AfterFunc(30*time.Second, func() {
			w2 := provider.GetWebsite(currentSite.Id)
			w2.HtmlCachePushStatus = nil
		})
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": status,
	})
}

func PluginHtmlCachePushLogs(ctx iris.Context) {
	currentSite := provider.CurrentSubSite(ctx)
	//需要支持分页，还要支持搜索
	currentPage := ctx.URLParamIntDefault("current", 1)
	pageSize := ctx.URLParamIntDefault("pageSize", 20)
	status := ctx.URLParam("status")

	var list []*model.HtmlPushLog
	offset := (currentPage - 1) * pageSize
	var total int64

	tx := currentSite.DB.Model(&model.HtmlPushLog{}).Order("created_time desc")
	if status == "error" {
		//模糊搜索
		tx = tx.Where("`status` = 0")
	}

	tx.Count(&total).Limit(pageSize).Offset(offset).Find(&list)

	ctx.JSON(iris.Map{
		"code":  config.StatusOK,
		"msg":   "",
		"total": total,
		"data":  list,
	})
}
