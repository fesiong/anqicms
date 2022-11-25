package manageController

import (
	"archive/zip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/jinzhu/now"
	"github.com/kataras/iris/v12"
	"github.com/parnurzeal/gorequest"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func AdminFileServ(ctx iris.Context) {
	uri := ctx.RequestPath(false)
	if uri != "/" {
		baseDir := config.ExecPath
		uriFile := baseDir + uri
		_, err := os.Stat(uriFile)
		if err == nil {
			ctx.ServeFile(uriFile)
			return
		}

		if !strings.Contains(filepath.Base(uri), ".") {
			uriFile = uriFile + "/index.html"
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

type moduleCount struct {
	Name  string `json:"name"`
	Total int64  `json:"total"`
}
type archiveCount struct {
	Total     int64 `json:"total"`
	LastWeek  int64 `json:"last_week"`
	UnRelease int64 `json:"un_release"`
	Today     int64 `json:"today"`
}
type splitCount struct {
	Total int64 `json:"total"`
	Today int64 `json:"today"`
}
type statistics struct {
	cacheTime       int64
	ModuleCounts    []moduleCount       `json:"archive_counts"`
	ArchiveCount    archiveCount        `json:"archive_count"`
	CategoryCount   int64               `json:"category_count"`
	LinkCount       int64               `json:"link_count"`
	GuestbookCount  int64               `json:"guestbook_count"`
	TrafficCount    splitCount          `json:"traffic_count"`
	SpiderCount     splitCount          `json:"spider_count"`
	IncludeCount    model.SpiderInclude `json:"include_count"`
	TemplateCount   int64               `json:"template_count"`
	PageCount       int64               `json:"page_count"`
	AttachmentCount int64               `json:"attachment_count"`
}

var cachedStatistics *statistics

func GetStatisticsSummary(ctx iris.Context) {
	var result = statistics{}
	if cachedStatistics == nil || cachedStatistics.cacheTime < time.Now().Add(-60*time.Second).Unix() {
		modules := provider.GetCacheModules()
		for _, v := range modules {
			counter := moduleCount{
				Name: v.Title,
			}
			dao.DB.Model(&model.Archive{}).Where("`module_id` = ?", v.Id).Count(&counter.Total)
			result.ModuleCounts = append(result.ModuleCounts, counter)
			result.ArchiveCount.Total += counter.Total
		}
		lastWeek := now.BeginningOfWeek()
		today := now.BeginningOfDay()
		dao.DB.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", lastWeek.AddDate(0, 0, -7).Unix(), lastWeek.Unix()).Count(&result.ArchiveCount.LastWeek)
		dao.DB.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", today.Unix(), time.Now().Unix()).Count(&result.ArchiveCount.Today)
		dao.DB.Model(&model.Archive{}).Where("created_time > ?", time.Now().Unix()).Count(&result.ArchiveCount.UnRelease)

		dao.DB.Model(&model.Category{}).Where("`type` != ?", config.CategoryTypePage).Count(&result.CategoryCount)
		dao.DB.Model(&model.Link{}).Count(&result.LinkCount)
		dao.DB.Model(&model.Guestbook{}).Count(&result.GuestbookCount)
		designList := provider.GetDesignList()
		result.TemplateCount = int64(len(designList))
		dao.DB.Model(&model.Category{}).Where("`type` = ?", config.CategoryTypePage).Count(&result.PageCount)
		dao.DB.Model(&model.Attachment{}).Count(&result.AttachmentCount)

		dao.DB.Model(&model.Statistic{}).Where("`spider` = '' and `created_time` >= ?", time.Now().AddDate(0, 0, -7).Unix()).Count(&result.TrafficCount.Total)
		dao.DB.Model(&model.Statistic{}).Where("`spider` = '' and `created_time` >= ?", today.Unix()).Count(&result.TrafficCount.Today)

		dao.DB.Model(&model.Statistic{}).Where("`spider`!= '' and `created_time` >= ?", time.Now().AddDate(0, 0, -7).Unix()).Count(&result.SpiderCount.Total)
		dao.DB.Model(&model.Statistic{}).Where("`spider` != '' and `created_time` >= ?", today.Unix()).Count(&result.SpiderCount.Today)

		var lastInclude model.SpiderInclude
		dao.DB.Model(&model.SpiderInclude{}).Order("id desc").Take(&lastInclude)
		result.IncludeCount = lastInclude

		result.cacheTime = time.Now().Unix()

		cachedStatistics = &result
	} else {
		result = *cachedStatistics
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": result,
	})
}

func GetStatisticsDashboard(ctx iris.Context) {
	var result = iris.Map{}
	// 登录信息
	var loginLogs []model.AdminLoginLog
	dao.DB.Order("id desc").Limit(2).Find(&loginLogs)
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
	result["system"] = config.JsonData.System

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
	link := "https://www.anqicms.com/downloads/version.json"
	var lastVersion response.LastVersion
	_, body, errs := gorequest.New().SetDoNotClearSuperAgent(true).TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).Timeout(10 * time.Second).Get(link).EndBytes()
	if errs != nil {
		log.Println("获取新版信息失败")
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "检查版本已是最新版",
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
				"msg":  "发现新版",
				"data": lastVersion,
			})
			return
		}
	}

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "检查版本已是最新版",
	})
}

func VersionUpgrade(ctx iris.Context) {
	var lastVersion response.LastVersion
	if err := ctx.ReadJSON(&lastVersion); err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  err.Error(),
		})
		return
	}
	// 下载压缩包
	link := fmt.Sprintf("https://www.anqicms.com/downloads/anqicms-%s-v%s.zip", runtime.GOOS, lastVersion.Version)
	// 最长等待10分钟
	resp, body, errs := gorequest.New().SetDoNotClearSuperAgent(true).TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).Timeout(10 * time.Minute).Get(link).EndBytes()
	if errs != nil || resp.StatusCode != 200 {
		log.Println("版本更新失败")
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "版本更新失败",
		})
		return
	}
	// 将文件写入
	tmpFile := config.ExecPath + filepath.Base(link)
	err := os.WriteFile(tmpFile, body, os.ModePerm)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "版本更新失败",
		})
		return
	}
	// 解压
	zipReader, err := zip.OpenReader(tmpFile)
	if err != nil {
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  "版本更新失败",
		})
		return
	}
	defer func() {
		zipReader.Close()

		// 删除压缩包
		os.Remove(tmpFile)
	}()

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
		if strings.Contains(f.Name, "static/") || strings.Contains(f.Name, "template/") || strings.Contains(f.Name, "favicon.ico") {
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
		log.Println("fail to rename old executable.")
	}
	if len(errorFiles) > 1 {
		log.Println("Upgrade error files: ", errorFiles)
		ctx.JSON(iris.Map{
			"code": config.StatusFailed,
			"msg":  fmt.Sprintf("版本更新部分失败, 以下文件未更新：%v", errorFiles),
		})
		return
	}

	provider.AddAdminLog(ctx, fmt.Sprintf("更新系统版本：%s => %s", config.Version, lastVersion.Version))
	msg := "已升级版本，请重启软件以使用新版。"
	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  msg,
	})
}
