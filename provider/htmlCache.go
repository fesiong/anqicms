package provider

import (
	"errors"
	"fmt"
	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SpecialCharsMap 查询参数中的特殊字符
var SpecialCharsMap = map[string]string{
	"\\": "xg",
	":":  "mh",
	"*":  "xh",
	"?":  "wh",
	"<":  "xy",
	">":  "dy",
	"|":  "sx",
	" ":  "kg",
}

type HtmlCacheStatus struct {
	Total         int    `json:"total"`
	FinishedCount int    `json:"finished_count"`
	StartTime     int64  `json:"start_time"`
	FinishedTime  int64  `json:"finished_time"`
	Current       string `json:"current"` // 当前执行任务
	ErrorMsg      string `json:"error_msg"`
}

func (w *Website) GetHtmlCacheStatus() *HtmlCacheStatus {

	return w.HtmlCacheStatus
}

func (w *Website) BuildHtmlCache() {
	if w.PluginHtmlCache.Open == false {
		return
	}

	w.HtmlCacheStatus = &HtmlCacheStatus{
		StartTime: time.Now().Unix(),
	}

	// 先生成首页
	w.BuildIndexCache()
	// 生成栏目页
	w.BuildCategoryCache()
	// 生成详情页
	w.BuildTagCache()
	w.HtmlCacheStatus.Current = "全部生成完成"
	w.HtmlCacheStatus.FinishedTime = time.Now().Unix()
}

func (w *Website) BuildIndexCache() {
	if w.PluginHtmlCache.Open == false {
		return
	}
	w.HtmlCacheStatus.Current = "开始生成首页"
	w.HtmlCacheStatus.Total += 1
	// 先生成首页
	err := w.GetAndCacheHtmlData("/", false)
	if err != nil {
		w.HtmlCacheStatus.ErrorMsg = "生成首页失败"
		return
	}
	if w.System.TemplateType != config.TemplateTypeAuto {
		_ = w.GetAndCacheHtmlData("/", true)
	}
	// 生成PC后再生成mobile的
	w.HtmlCacheStatus.FinishedCount += 1
	w.HtmlCacheStatus.Current = "首页生成完成"
}

func (w *Website) BuildCategoryCache() {
	if w.PluginHtmlCache.Open == false {
		return
	}
	// 生成栏目
	w.HtmlCacheStatus.Current = "开始生成栏目"
	var categories []*model.Category
	w.DB.Model(&model.Category{}).Where("`status` = 1").Order("id asc").Find(&categories)
	w.HtmlCacheStatus.Total += len(categories)
	for _, category := range categories {
		w.HtmlCacheStatus.Current = "正在生成栏目：" + category.Title
		// 栏目只生成第一页
		link := w.GetUrl("category", category, 0)
		link = strings.TrimPrefix(link, w.System.BaseUrl)
		err := w.GetAndCacheHtmlData(link, false)
		if err != nil {
			w.HtmlCacheStatus.ErrorMsg = "生成栏目" + category.Title + "失败"
			continue
		}
		if w.System.TemplateType != config.TemplateTypeAuto {
			_ = w.GetAndCacheHtmlData(link, true)
		}
		w.HtmlCacheStatus.FinishedCount += 1
	}
	w.HtmlCacheStatus.Current = "栏目生成完成"
}

func (w *Website) BuildArchiveCache() {
	if w.PluginHtmlCache.Open == false {
		return
	}
	w.HtmlCacheStatus.Current = "开始生成文档"
	// 生成详情
	lastId := uint(0)
	for {
		var archives []*model.Archive
		w.DB.Model(&model.Archive{}).Where("`status` = 1 and `id` > ?", lastId).Limit(1000).Order("id asc").Find(&archives)
		if len(archives) == 0 {
			break
		}
		w.HtmlCacheStatus.Total += len(archives)
		lastId = archives[len(archives)-1].Id
		for _, arc := range archives {
			w.HtmlCacheStatus.Current = "正在生成文档：" + arc.Title
			link := w.GetUrl("archive", arc, 0)
			link = strings.TrimPrefix(link, w.System.BaseUrl)
			err := w.GetAndCacheHtmlData(link, false)
			if err != nil {
				w.HtmlCacheStatus.ErrorMsg = "生成文档" + arc.Title + "失败"
				continue
			}
			if w.System.TemplateType != config.TemplateTypeAuto {
				_ = w.GetAndCacheHtmlData(link, true)
			}
			w.HtmlCacheStatus.FinishedCount += 1
		}
	}
	w.HtmlCacheStatus.Current = "文档生成完成"
}

func (w *Website) BuildTagCache() {
	if w.PluginHtmlCache.Open == false {
		return
	}
	w.HtmlCacheStatus.Current = "开始生成标签"
	// 生成标签
	lastId := uint(0)
	for {
		var tags []*model.Tag
		w.DB.Model(&model.Tag{}).Where("`status` = 1 and `id` > ?", lastId).Limit(1000).Order("id asc").Find(&tags)
		if len(tags) == 0 {
			break
		}
		w.HtmlCacheStatus.Total += len(tags)
		lastId = tags[len(tags)-1].Id
		for _, tag := range tags {
			w.HtmlCacheStatus.Current = "正在生成标签：" + tag.Title
			link := w.GetUrl("tag", tag, 0)
			link = strings.TrimPrefix(link, w.System.BaseUrl)
			err := w.GetAndCacheHtmlData(link, false)
			if err != nil {
				w.HtmlCacheStatus.ErrorMsg = "生成标签" + tag.Title + "失败"
				continue
			}
			if w.System.TemplateType != config.TemplateTypeAuto {
				_ = w.GetAndCacheHtmlData(link, true)
			}
			w.HtmlCacheStatus.FinishedCount += 1
		}
	}
	w.HtmlCacheStatus.Current = "标签生成完成"
}

func (w *Website) GetAndCacheHtmlData(urlPath string, isMobile bool) error {
	if w.PluginHtmlCache.Open == false {
		return errors.New("没开启静态缓存功能")
	}
	ua := library.GetUserAgent(isMobile)
	baseUrl := fmt.Sprintf("http://127.0.0.1:%d", config.Server.Server.Port)
	resp, err := library.Request(baseUrl+urlPath, &library.Options{Header: map[string]string{
		"host":  w.Host,
		"cache": "true",
	}, UserAgent: ua})
	if err != nil {
		return err
	}

	err = w.CacheHtmlData(urlPath, "", isMobile, []byte(resp.Body))

	return err
}

func (w *Website) CacheHtmlData(oriPath, oriQuery string, isMobile bool, body []byte) error {
	cacheFile := w.CachePath
	if isMobile {
		cacheFile += "mobile"
	} else {
		cacheFile += "pc"
	}

	cacheFile = cacheFile + transToLocalPath(oriPath, oriQuery)

	// 创建目录
	info, err := os.Stat(filepath.Dir(cacheFile))
	if os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(cacheFile), os.ModePerm)
		if err != nil {
			return err
		}
	} else if !info.IsDir() {
		_ = os.Remove(filepath.Dir(cacheFile))
		err = os.MkdirAll(filepath.Dir(cacheFile), os.ModePerm)
		if err != nil {
			return err
		}
	}

	return os.WriteFile(cacheFile, body, os.ModePerm)
}

func (w *Website) LoadCachedHtml(ctx iris.Context) (cacheFile string, ok bool) {
	if w.PluginHtmlCache.Open == false {
		return "", false
	}
	if ctx.GetHeader("Cache-Control") == "no-cache" {
		return "", false
	}
	// 用户登录后，也不缓存
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId > 0 {
		return "", false
	}
	// 获得路由
	match := ctx.Params().Get("match")
	if match == "index" {
		if w.PluginHtmlCache.IndexCache == 0 {
			return "", false
		}
	} else if match == "archive" {
		if w.PluginHtmlCache.DetailCache == 0 {
			return "", false
		}
	} else if w.PluginHtmlCache.ListCache == 0 {
		return "", false
	}

	cacheFile = w.CachePath
	if ctx.IsMobile() {
		cacheFile += "mobile"
	} else {
		cacheFile += "pc"
	}
	localPath := transToLocalPath(ctx.RequestPath(false), ctx.Request().URL.RawQuery)
	cacheFile = cacheFile + localPath

	info, err := os.Stat(cacheFile)
	if err != nil {
		return "", false
	}
	// 检查是否过期
	if match == "index" {
		if info.ModTime().Before(time.Now().Add(time.Duration(-w.PluginHtmlCache.IndexCache) * time.Second)) {
			return "", false
		}
	} else if match == "archive" {
		if info.ModTime().Before(time.Now().Add(time.Duration(-w.PluginHtmlCache.DetailCache) * time.Second)) {
			return "", false
		}
	} else if info.ModTime().Before(time.Now().Add(time.Duration(-w.PluginHtmlCache.ListCache) * time.Second)) {
		return "", false
	}

	return cacheFile, true
}

func (w *Website) RemoveHtmlCache(oriPaths ...string) {
	cacheFilePc := w.CachePath + "mobile"
	cacheFileMobile := w.CachePath + "pc"

	if len(oriPaths) > 0 {
		for _, oriPath := range oriPaths {
			if strings.HasPrefix(oriPath, w.System.BaseUrl) {
				oriPath = strings.TrimPrefix(oriPath, w.System.BaseUrl)
			}
			oriPath = transToLocalPath(oriPath, "")
			_ = os.Remove(cacheFilePc + oriPath)
			_ = os.Remove(cacheFileMobile + oriPath)
		}
	} else {
		_ = os.RemoveAll(cacheFilePc)
		_ = os.RemoveAll(cacheFileMobile)
	}
}

func transToLocalPath(oriPath string, oriQuery string) string {
	localLink := oriPath
	// 如果path以/结尾
	boolean := strings.HasSuffix(localLink, "/")
	if boolean && oriQuery == "" {
		localLink += "index.html"
	}
	// 替换query参数中的特殊字符
	if oriQuery != "" {
		queryStr := oriQuery
		for key, val := range SpecialCharsMap {
			queryStr = strings.Replace(queryStr, key, val, -1)
		}
		localLink = localLink + SpecialCharsMap["?"] + queryStr + ".html"
	} else if !strings.Contains(filepath.Base(localLink), ".") {
		localLink += "/index.html"
	}

	return localLink
}
