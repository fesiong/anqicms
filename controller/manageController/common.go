package manageController

import (
	"archive/zip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/parnurzeal/gorequest"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func AdminFileServ(ctx iris.Context) {
	tmpSiteId := ctx.GetHeader("Site-Id")
	if len(tmpSiteId) > 0 {
		siteId, _ := strconv.Atoi(tmpSiteId)
		if siteId > 0 {
			// 只有二级目录安装的站点允许这么操作
			website := provider.GetWebsite(uint(siteId))
			if website != nil && len(website.BaseURI) > 1 {
				ctx.Values().Set("siteId", uint(siteId))
			}
		}
	}

	uri := ctx.RequestPath(false)
	if uri != "/" {
		uriFile := config.ExecPath + strings.TrimLeft(uri, "/")
		_, err := os.Stat(uriFile)
		if err == nil {
			ctx.ServeFile(uriFile)
			return
		}

		if !strings.Contains(filepath.Base(uri), ".") {
			uriFile = strings.TrimRight(uriFile, "/") + "/index.html"
			_, err = os.Stat(uriFile)
			if err == nil {
				ctx.ServeFile(uriFile)
				return
			}
		}
	}

	ctx.Next()
}

func Version(ctx iris.Context) {
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"version": config.Version,
		},
	})
}

func GetStatisticsSummary(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	exact := ctx.URLParamBoolDefault("exact", false)
	result := currentSite.GetStatisticsSummary(exact)

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

func GetStatisticsDashboard(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var result = iris.Map{}
	// 登录信息
	var loginLogs []model.AdminLoginLog
	currentSite.DB.Order("id desc").Limit(2).Find(&loginLogs)
	if len(loginLogs) == 2 {
		result["last_login"] = iris.Map{
			"created_time": loginLogs[1].CreatedTime,
			"ip":           loginLogs[1].Ip,
		}
	}
	if len(loginLogs) > 0 {
		result["now_login"] = iris.Map{
			"created_time": loginLogs[0].CreatedTime,
			"ip":           loginLogs[0].Ip,
		}
	}
	// 配置信息
	result["system"] = currentSite.System

	result["version"] = config.Version
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	result["memory_usage"] = ms.Alloc

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

// CheckVersion 检查新版
func CheckVersion(ctx iris.Context) {
	link := "https://www.anqicms.com/downloads/version.json?goos=" + runtime.GOOS + "&goarch=" + runtime.GOARCH
	var lastVersion response.LastVersion
	_, body, errs := gorequest.New().SetDoNotClearSuperAgent(true).TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).Timeout(10 * time.Second).Get(link).EndBytes()
	if errs != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  ctx.Tr("CheckThatTheVersionIsTheLatestVersion"),
		})
		return
	}

	err := json.Unmarshal(body, &lastVersion)
	if err == nil {
		result := library.VersionCompare(lastVersion.Version, config.Version)
		if result == 1 {
			// 版本有更新
			ctx.JSON(iris.Map{
				"code": config.StatusOK,
				"msg":  ctx.Tr("FoundANewVersion"),
				"data": lastVersion,
			})
			return
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  ctx.Tr("CheckThatTheVersionIsTheLatestVersion"),
	})
}

func VersionUpgrade(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	var lastVersion response.LastVersion
	if err := ctx.ReadJSON(&lastVersion); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 下载压缩包
	link := fmt.Sprintf("https://www.anqicms.com/downloads/anqicms-%s-%s-v%s.zip", runtime.GOOS, runtime.GOARCH, lastVersion.Version)
	// 最长等待10分钟
	resp, body, errs := gorequest.New().SetDoNotClearSuperAgent(true).TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).Timeout(15 * time.Minute).Get(link).EndBytes()
	if errs != nil || resp.StatusCode != 200 {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VersionUpdateFailed"),
		})
		return
	}
	// 将文件写入
	tmpFile := config.ExecPath + filepath.Base(link)
	err := os.WriteFile(tmpFile, body, os.ModePerm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VersionUpdateFailed"),
		})
		return
	}
	// 解压
	zipReader, err := zip.OpenReader(tmpFile)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VersionUpdateFailed"),
		})
		return
	}
	defer func() {
		zipReader.Close()

		// 删除压缩包
		os.Remove(tmpFile)
	}()

	// 删除 system
	_ = os.RemoveAll(config.ExecPath + "system")

	var errorFiles []string
	path, _ := os.Executable()
	exec := filepath.Base(path)

	execPath := filepath.Join(config.ExecPath, exec)
	tmpExecPath := execPath + ".new"

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// 模板文件不更新
		if strings.Contains(f.Name, "static/") ||
			strings.Contains(f.Name, "template/") ||
			strings.Contains(f.Name, ".sh") ||
			strings.Contains(f.Name, ".bat") ||
			strings.Contains(f.Name, "favicon.ico") {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			continue
		}
		realName := filepath.Join(config.ExecPath, f.Name)
		if f.Name == "anqicms" || f.Name == "anqicms.exe" {
			// 先以临时文件存储
			realName = tmpExecPath
		}
		_ = os.MkdirAll(filepath.Dir(realName), os.ModePerm)
		newFile, err := os.Create(realName)
		if err != nil {
			reader.Close()
			errorFiles = append(errorFiles, realName)
			continue
		}
		_, err = io.Copy(newFile, reader)
		if err != nil {
			reader.Close()
			newFile.Close()
			errorFiles = append(errorFiles, realName)
			continue
		}

		reader.Close()
		_ = newFile.Close()
		// 对于可执行文件，需要赋予可执行权限，可执行文件有：anqicms,cwebp
		if f.Name == "anqicms" || f.Name == "anqicms.exe" || strings.HasPrefix(f.Name, "cwebp_") {
			_ = os.Chmod(realName, os.ModePerm)
		}
	}
	// 尝试更换主程序
	oldPath := execPath + ".old"
	_ = os.Remove(oldPath)
	err = os.Rename(execPath, oldPath)
	if err == nil {
		err = os.Rename(tmpExecPath, execPath)
		if err == nil {
			_ = os.Chmod(execPath, os.ModePerm)
			// 移动成功
			_ = os.Remove(oldPath)
		} else {
			//移动失败
			err = os.Rename(oldPath, execPath)
		}
	} else {
		log.Println("fail to rename old executable.", err)
	}
	if len(errorFiles) > 1 {
		log.Println("Upgrade error files: ", errorFiles)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  ctx.Tr("VersionUpdatePartiallyFailed", errorFiles),
		})
		return
	}

	currentSite.AddAdminLog(ctx, ctx.Tr("UpdateSystemVersion", config.Version, lastVersion.Version))
	msg := ctx.Tr("TheVersionHasBeenUpgraded")
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  msg,
	})
}
