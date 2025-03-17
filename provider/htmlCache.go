package provider

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
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
	ErrorCount    int    `json:"error_count"`
	StartTime     int64  `json:"start_time"`
	FinishedTime  int64  `json:"finished_time"`
	Current       string `json:"current"` // 当前执行任务
	ErrorMsg      string `json:"error_msg"`
	Removing      bool   `json:"-"`
}

func (w *Website) GetHtmlCacheStatus() *HtmlCacheStatus {

	return w.HtmlCacheStatus
}

func (w *Website) GetHtmlCachePushStatus() *HtmlCacheStatus {

	return w.HtmlCachePushStatus
}

func (w *Website) BuildHtmlCache(ctx iris.Context) {
	if w.PluginHtmlCache.Open == false {
		return
	}

	w.HtmlCacheStatus = &HtmlCacheStatus{
		StartTime: time.Now().Unix(),
	}

	// 先生成首页
	w.BuildIndexCache()
	// 生成模型
	w.BuildModuleCache(ctx)
	//// 生成栏目页
	w.BuildCategoryCache(ctx)
	// 生成详情页
	w.BuildTagIndexCache(ctx)
	w.BuildTagCache(ctx)
	w.BuildArchiveCache()
	if w.HtmlCacheStatus != nil {
		w.HtmlCacheStatus.Current = w.Tr("AllGenerated")
		w.HtmlCacheStatus.FinishedTime = time.Now().Unix()
	}
	w.PluginHtmlCache.LastBuildTime = time.Now().Unix()
	_ = w.SaveSettingValue(HtmlCacheSettingKey, w.PluginHtmlCache)
	// 如果开启了静态服务器，则执行传输
	_ = w.SyncHtmlCacheToStorage("", "")
}

func (w *Website) BuildIndexCache() {
	if w.PluginHtmlCache.Open == false {
		return
	}
	if w.HtmlCacheStatus == nil {
		w.HtmlCacheStatus = &HtmlCacheStatus{
			StartTime: time.Now().Unix(),
		}
	}
	w.HtmlCacheStatus.FinishedTime = 0
	w.HtmlCacheStatus.Current = w.Tr("StartGeneratingHomepage")
	w.HtmlCacheStatus.Total += 1
	// 先生成首页
	err := w.GetAndCacheHtmlData("/", false)
	if err != nil {
		w.HtmlCacheStatus.ErrorMsg = w.Tr("FailedToGenerateHomepage") + err.Error()
		return
	}
	w.HtmlCacheStatus.FinishedCount += 1
	if w.System.TemplateType != config.TemplateTypeAuto {
		w.HtmlCacheStatus.Total += 1
		err = w.GetAndCacheHtmlData("/", true)
		if err == nil {
			w.HtmlCacheStatus.FinishedCount += 1
		} else {
			w.HtmlCacheStatus.ErrorMsg = w.Tr("FailedToGenerateHomepage") + err.Error()
		}
	}
	w.HtmlCacheStatus.Current = w.Tr("HomepageGenerationCompleted")
}

func (w *Website) BuildModuleCache(ctx iris.Context) {
	if w.PluginHtmlCache.Open == false {
		return
	}
	if w.HtmlCacheStatus == nil {
		w.HtmlCacheStatus = &HtmlCacheStatus{
			StartTime: time.Now().Unix(),
		}
	}
	w.HtmlCacheStatus.FinishedTime = 0
	// 生成栏目
	w.HtmlCacheStatus.Current = w.Tr("StartGeneratingModel")
	var modules []*model.Module
	w.DB.Model(&model.Module{}).Where("`status` = 1").Order("id asc").Find(&modules)
	w.HtmlCacheStatus.Total += len(modules)
	for _, module := range modules {
		w.HtmlCacheStatus.Current = w.Tr("GeneratingModelLog", module.Title)
		// 模型只生成第一页
		link := w.GetUrl("archiveIndex", module, 0)
		link = strings.TrimPrefix(link, w.System.BaseUrl)
		err := w.GetAndCacheHtmlData(link, false)
		if err != nil {
			w.HtmlCacheStatus.ErrorMsg = w.Tr("GenerateModelFailed", module.Title, err.Error())
			continue
		}
		w.HtmlCacheStatus.FinishedCount += 1
		// 检查模型是否有分页，如果有，则继续生成分页
		newCtx := ctx.Clone()
		writer := newResponseWriter()
		respWriter := &responseWriter{ResponseWriter: writer}
		newCtx.ResetResponseWriter(respWriter)
		newCtx.ViewData("module", module)
		newCtx.ViewData("pageName", "archiveIndex")
		webInfo := &response.WebInfo{
			Title:    module.Title,
			PageName: "archiveIndex",
			NavBar:   int64(module.Id),
		}
		newCtx.ViewData("webInfo", webInfo)
		tplName := module.TableName + "/index.html"
		tplName2 := module.TableName + "_index.html"
		if ViewExists(newCtx, tplName2) {
			tplName = tplName2
		}
		_ = newCtx.Application().View(newCtx, tplName, "", newCtx.GetViewData())
		if webInfo.TotalPages > 1 {
			w.HtmlCacheStatus.Total += webInfo.TotalPages - 1
			// 当存在多页的时候，则循环生成
			for page := 2; page <= webInfo.TotalPages; page++ {
				link = w.GetUrl("archiveIndex", module, page)
				link = strings.TrimPrefix(link, w.System.BaseUrl)
				err = w.GetAndCacheHtmlData(link, false)
				if err != nil {
					w.HtmlCacheStatus.ErrorMsg = w.Tr("GenerateModelFailed", module.Title, err.Error())
					continue
				}
				w.HtmlCacheStatus.FinishedCount += 1
			}
		}
		// mobile
		if w.System.TemplateType != config.TemplateTypeAuto {
			w.HtmlCacheStatus.Total += 1
			err = w.GetAndCacheHtmlData(link, true)
			if err == nil {
				w.HtmlCacheStatus.FinishedCount += 1
			}
			tplName = "mobile/" + tplName
			_ = newCtx.View(tplName)
			if webInfo.TotalPages > 1 {
				w.HtmlCacheStatus.Total += webInfo.TotalPages - 1
				// 当存在多页的时候，则循环生成
				for page := 2; page <= webInfo.TotalPages; page++ {
					link = w.GetUrl("archiveIndex", module, page)
					link = strings.TrimPrefix(link, w.System.BaseUrl)
					err = w.GetAndCacheHtmlData(link, true)
					if err != nil {
						w.HtmlCacheStatus.ErrorMsg = w.Tr("GenerateModelFailed", module.Title, err.Error())
						continue
					}
					w.HtmlCacheStatus.FinishedCount += 1
				}
			}
		}
	}
	w.HtmlCacheStatus.Current = w.Tr("ModelGenerationCompleted")
}

func (w *Website) BuildCategoryCache(ctx iris.Context) {
	if w.PluginHtmlCache.Open == false {
		return
	}
	if w.HtmlCacheStatus == nil {
		w.HtmlCacheStatus = &HtmlCacheStatus{
			StartTime: time.Now().Unix(),
		}
	}
	w.HtmlCacheStatus.FinishedTime = 0
	// 生成栏目
	w.HtmlCacheStatus.Current = w.Tr("StartGeneratingColumns")
	var categories []*model.Category
	w.DB.Model(&model.Category{}).Where("`status` = 1").Order("id asc").Find(&categories)
	w.HtmlCacheStatus.Total += len(categories)
	for _, category := range categories {
		w.BuildSingleCategoryCache(ctx, category)
	}
	w.HtmlCacheStatus.Current = w.Tr("ColumnGenerationCompleted")
}

func (w *Website) BuildSingleCategoryCache(ctx iris.Context, category *model.Category) {
	if w.HtmlCacheStatus != nil {
		w.HtmlCacheStatus.Current = w.Tr("GeneratingColumnsLog", category.Title)
	}
	// 栏目只生成第一页
	link := w.GetUrl("category", category, 0)
	link = strings.TrimPrefix(link, w.System.BaseUrl)
	err := w.GetAndCacheHtmlData(link, false)
	if err != nil {
		if w.HtmlCacheStatus != nil {
			w.HtmlCacheStatus.ErrorMsg = w.Tr("GeneratingColumnFailed", category.Title, err.Error())
		}
		return
	}
	if w.HtmlCacheStatus != nil {
		w.HtmlCacheStatus.FinishedCount += 1
	}
	// 检查模型是否有分页，如果有，则继续生成分页
	module := w.GetModuleFromCache(category.ModuleId)
	if module == nil {
		return
	}
	newCtx := ctx.Clone()
	writer := newResponseWriter()
	respWriter := &responseWriter{ResponseWriter: writer}
	newCtx.ResetResponseWriter(respWriter)
	newCtx.ViewData("category", category)
	newCtx.ViewData("pageName", "archiveList")
	webInfo := &response.WebInfo{
		Title:    category.Title,
		PageName: "archiveList",
		NavBar:   int64(category.Id),
	}
	newCtx.ViewData("webInfo", webInfo)
	tplName := module.TableName + "/list.html"
	tplName2 := module.TableName + "_list.html"
	if ViewExists(newCtx, tplName2) {
		tplName = tplName2
	}
	//模板优先级：1、设置的template；2、存在分类id为名称的模板；3、继承的上级模板；4、默认模板，如果发现上一级不继承，则不需要处理
	if category.Template != "" {
		tplName = category.Template
	} else if ViewExists(newCtx, fmt.Sprintf("%s/list-%d.html", module.TableName, category.Id)) {
		tplName = fmt.Sprintf("%s/list-%d.html", module.TableName, category.Id)
	} else {
		categoryTemplate := w.GetCategoryTemplate(category)
		if categoryTemplate != nil && len(categoryTemplate.Template) > 0 {
			tplName = categoryTemplate.Template
		}
	}
	if !strings.HasSuffix(tplName, ".html") {
		tplName += ".html"
	}
	_ = newCtx.Application().View(newCtx, tplName, "", newCtx.GetViewData())
	if webInfo.TotalPages > 1 {
		if w.HtmlCacheStatus != nil {
			w.HtmlCacheStatus.Total += webInfo.TotalPages - 1
		}
		// 当存在多页的时候，则循环生成
		for page := 2; page <= webInfo.TotalPages; page++ {
			link = w.GetUrl("category", category, page)
			link = strings.TrimPrefix(link, w.System.BaseUrl)
			err = w.GetAndCacheHtmlData(link, false)
			if err != nil {
				if w.HtmlCacheStatus != nil {
					w.HtmlCacheStatus.ErrorMsg = w.Tr("GeneratingColumnFailed", category.Title, err.Error())
				}
				continue
			}
			if w.HtmlCacheStatus != nil {
				w.HtmlCacheStatus.FinishedCount += 1
			}
		}
	}
	// mobile
	if w.System.TemplateType != config.TemplateTypeAuto {
		if w.HtmlCacheStatus != nil {
			w.HtmlCacheStatus.Total += 1
		}
		err = w.GetAndCacheHtmlData(link, true)
		if err == nil {
			if w.HtmlCacheStatus != nil {
				w.HtmlCacheStatus.FinishedCount += 1
			}
		}
		tplName = "mobile/" + tplName
		webInfo.TotalPages = 0
		err = newCtx.View(tplName)
		if webInfo.TotalPages > 1 {
			if w.HtmlCacheStatus != nil {
				w.HtmlCacheStatus.Total += webInfo.TotalPages - 1
			}
			// 当存在多页的时候，则循环生成
			for page := 2; page <= webInfo.TotalPages; page++ {
				link = w.GetUrl("category", category, page)
				link = strings.TrimPrefix(link, w.System.BaseUrl)
				err = w.GetAndCacheHtmlData(link, true)
				if err != nil {
					if w.HtmlCacheStatus != nil {
						w.HtmlCacheStatus.ErrorMsg = w.Tr("GeneratingColumnFailed", category.Title, err.Error())
					}
					continue
				}
				if w.HtmlCacheStatus != nil {
					w.HtmlCacheStatus.FinishedCount += 1
				}
			}
		}
	}
}

func (w *Website) BuildArchiveCache() {
	if w.PluginHtmlCache.Open == false {
		return
	}
	if w.HtmlCacheStatus == nil {
		w.HtmlCacheStatus = &HtmlCacheStatus{
			StartTime: time.Now().Unix(),
		}
	}
	w.HtmlCacheStatus.FinishedTime = 0
	w.HtmlCacheStatus.Current = w.Tr("StartGeneratingDocuments")
	// 生成详情
	lastId := int64(0)
	for {
		var archives []*model.Archive
		w.DB.Model(&model.Archive{}).Where("`id` > ?", lastId).Limit(1000).Order("id asc").Find(&archives)
		if len(archives) == 0 {
			break
		}
		w.HtmlCacheStatus.Total += len(archives)
		lastId = archives[len(archives)-1].Id
		for _, arc := range archives {
			w.HtmlCacheStatus.Current = w.Tr("GeneratingDocuments:s", arc.Title)
			link := w.GetUrl("archive", arc, 0)
			link = strings.TrimPrefix(link, w.System.BaseUrl)
			err := w.GetAndCacheHtmlData(link, false)
			if err != nil {
				w.HtmlCacheStatus.ErrorMsg = w.Tr("GeneratingDocumentFailed", arc.Title, err.Error())
				continue
			}
			w.HtmlCacheStatus.FinishedCount += 1
			if w.System.TemplateType != config.TemplateTypeAuto {
				w.HtmlCacheStatus.Total += 1
				err = w.GetAndCacheHtmlData(link, true)
				if err == nil {
					w.HtmlCacheStatus.FinishedCount += 1
				}
			}
		}
	}
	w.HtmlCacheStatus.Current = w.Tr("DocumentGenerationCompleted")
}

func (w *Website) BuildTagIndexCache(ctx iris.Context) {
	if w.PluginHtmlCache.Open == false {
		return
	}
	if w.HtmlCacheStatus == nil {
		w.HtmlCacheStatus = &HtmlCacheStatus{
			StartTime: time.Now().Unix(),
		}
	}
	w.HtmlCacheStatus.FinishedTime = 0
	w.HtmlCacheStatus.Current = w.Tr("StartGeneratingTagHomepage")
	w.HtmlCacheStatus.Total += 1
	link := w.GetUrl("tagIndex", nil, 0)
	// 先生成首页
	err := w.GetAndCacheHtmlData(link, false)
	if err != nil {
		w.HtmlCacheStatus.ErrorMsg = w.Tr("GeneratingTagHomepageFailed") + err.Error()
		return
	}
	// 检查模型是否有分页，如果有，则继续生成分页
	newCtx := ctx.Clone()
	writer := newResponseWriter()
	respWriter := &responseWriter{ResponseWriter: writer}
	newCtx.ResetResponseWriter(respWriter)
	newCtx.ViewData("pageName", "tagIndex")
	webInfo := &response.WebInfo{
		Title:    w.Tr("TagList"),
		PageName: "tagIndex",
	}
	newCtx.ViewData("webInfo", webInfo)
	tplName := "tag/index.html"
	if ViewExists(newCtx, "tag_index.html") {
		tplName = "tag_index.html"
	}
	_ = newCtx.Application().View(newCtx, tplName, "", newCtx.GetViewData())
	if webInfo.TotalPages > 1 {
		w.HtmlCacheStatus.Total += webInfo.TotalPages - 1
		// 当存在多页的时候，则循环生成
		for page := 2; page <= webInfo.TotalPages; page++ {
			link = w.GetUrl("tagIndex", nil, page)
			link = strings.TrimPrefix(link, w.System.BaseUrl)
			err = w.GetAndCacheHtmlData(link, false)
			if err != nil {
				w.HtmlCacheStatus.ErrorMsg = w.Tr("GeneratingTagHomepageFailed") + err.Error()
				continue
			}
			w.HtmlCacheStatus.FinishedCount += 1
		}
	}
	// mobile
	if w.System.TemplateType != config.TemplateTypeAuto {
		w.HtmlCacheStatus.Total += 1
		err = w.GetAndCacheHtmlData(link, true)
		if err == nil {
			w.HtmlCacheStatus.FinishedCount += 1
		}
		// 检查模型是否有分页，如果有，则继续生成分页
		tplName = "mobile/" + tplName
		webInfo.TotalPages = 0
		_ = newCtx.View(tplName)
		if webInfo.TotalPages > 1 {
			w.HtmlCacheStatus.Total += webInfo.TotalPages - 1
			// 当存在多页的时候，则循环生成
			for page := 2; page <= webInfo.TotalPages; page++ {
				link = w.GetUrl("tagIndex", nil, page)
				link = strings.TrimPrefix(link, w.System.BaseUrl)
				err = w.GetAndCacheHtmlData(link, true)
				if err != nil {
					w.HtmlCacheStatus.ErrorMsg = w.Tr("GeneratingTagHomepageFailed") + err.Error()
					continue
				}
				w.HtmlCacheStatus.FinishedCount += 1
			}
		}
	}
	// end
	w.HtmlCacheStatus.FinishedCount += 1
	w.HtmlCacheStatus.Current = w.Tr("HomepageTagGenerationCompleted")
}

func (w *Website) BuildTagCache(ctx iris.Context) {
	if w.PluginHtmlCache.Open == false {
		return
	}
	if w.HtmlCacheStatus == nil {
		w.HtmlCacheStatus = &HtmlCacheStatus{
			StartTime: time.Now().Unix(),
		}
	}
	w.HtmlCacheStatus.FinishedTime = 0
	w.HtmlCacheStatus.Current = w.Tr("StartGeneratingTags")
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
			w.BuildSingleTagCache(ctx, tag)
		}
	}
	w.HtmlCacheStatus.Current = w.Tr("TagGenerationCompleted")
}

func (w *Website) BuildSingleTagCache(ctx iris.Context, tag *model.Tag) {
	if w.PluginHtmlCache.Open == false {
		return
	}
	if w.HtmlCacheStatus != nil {
		w.HtmlCacheStatus.Current = w.Tr("GeneratingTagsLog", tag.Title)
	}
	link := w.GetUrl("tag", tag, 0)
	link = strings.TrimPrefix(link, w.System.BaseUrl)
	err := w.GetAndCacheHtmlData(link, false)
	if err != nil {
		if w.HtmlCacheStatus != nil {
			w.HtmlCacheStatus.ErrorMsg = w.Tr("GeneratingTagsFailed", tag.Title, err.Error())
		}
		return
	}
	if w.HtmlCacheStatus != nil {
		w.HtmlCacheStatus.FinishedCount += 1
	}
	// 检查模型是否有分页，如果有，则继续生成分页
	newCtx := ctx.Clone()
	writer := newResponseWriter()
	respWriter := &responseWriter{ResponseWriter: writer}
	newCtx.ResetResponseWriter(respWriter)
	newCtx.ViewData("tag", tag)
	newCtx.ViewData("pageName", "tag")
	webInfo := &response.WebInfo{
		Title:    tag.Title,
		PageName: "tag",
		NavBar:   int64(tag.Id),
	}
	newCtx.ViewData("webInfo", webInfo)
	tplName := "tag/list.html"
	if ViewExists(ctx, "tag_list.html") {
		tplName = "tag_list.html"
	}
	_ = newCtx.Application().View(newCtx, tplName, "", newCtx.GetViewData())
	if webInfo.TotalPages > 1 {
		if w.HtmlCacheStatus != nil {
			w.HtmlCacheStatus.Total += webInfo.TotalPages - 1
		}
		// 当存在多页的时候，则循环生成
		for page := 2; page <= webInfo.TotalPages; page++ {
			link = w.GetUrl("tag", tag, page)
			link = strings.TrimPrefix(link, w.System.BaseUrl)
			err = w.GetAndCacheHtmlData(link, false)
			if err != nil {
				if w.HtmlCacheStatus != nil {
					w.HtmlCacheStatus.ErrorMsg = w.Tr("GeneratingTagsFailed", tag.Title, err.Error())
				}
				continue
			}
			if w.HtmlCacheStatus != nil {
				w.HtmlCacheStatus.FinishedCount += 1
			}
		}
	}
	// mobile
	if w.System.TemplateType != config.TemplateTypeAuto {
		if w.HtmlCacheStatus != nil {
			w.HtmlCacheStatus.Total += 1
		}
		err = w.GetAndCacheHtmlData(link, true)
		if err == nil {
			if w.HtmlCacheStatus != nil {
				w.HtmlCacheStatus.FinishedCount += 1
			}
		}
		tplName = "mobile/" + tplName
		webInfo.TotalPages = 0
		_ = newCtx.View(tplName)
		if webInfo.TotalPages > 1 {
			if w.HtmlCacheStatus != nil {
				w.HtmlCacheStatus.Total += webInfo.TotalPages - 1
			}
			// 当存在多页的时候，则循环生成
			for page := 2; page <= webInfo.TotalPages; page++ {
				link = w.GetUrl("tag", tag, page)
				link = strings.TrimPrefix(link, w.System.BaseUrl)
				err = w.GetAndCacheHtmlData(link, true)
				if err != nil {
					if w.HtmlCacheStatus != nil {
						w.HtmlCacheStatus.ErrorMsg = w.Tr("GeneratingTagsFailed", tag.Title, err.Error())
					}
					continue
				}
				if w.HtmlCacheStatus != nil {
					w.HtmlCacheStatus.FinishedCount += 1
				}
			}
		}
	}
}

func (w *Website) GetAndCacheHtmlData(urlPath string, isMobile bool) error {
	if w.PluginHtmlCache.Open == false {
		return errors.New(w.Tr("StaticCacheFunctionIsNotEnabled"))
	}

	_, err := w.GetHtmlDataByLocal(urlPath, isMobile)

	return err
}

func (w *Website) GetHtmlDataByLocal(urlPath string, isMobile bool) ([]byte, error) {
	if strings.HasPrefix(urlPath, "http") {
		parsed, err := url.Parse(urlPath)
		if err == nil {
			urlPath = parsed.Path
			if len(parsed.RawQuery) > 0 {
				urlPath += "?" + parsed.RawQuery
			}
		}
	}
	host := w.Host
	if isMobile && w.System.TemplateType == config.TemplateTypeSeparate {
		mobileUrl, err := url.Parse(w.System.MobileUrl)
		if err != nil {
			return nil, errors.New(w.Tr("MobileDomainNameResolutionFailed"))
		}
		host = mobileUrl.Hostname()
	}
	ua := library.GetUserAgent(isMobile)
	baseUrl := fmt.Sprintf("http://127.0.0.1:%d", config.Server.Server.Port)

	// 10秒超时
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("GET", baseUrl+urlPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("X-Host", host)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Cache", "true")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	content, _ := io.ReadAll(resp.Body)

	return content, nil
}

func (w *Website) CacheHtmlData(oriPath, oriQuery string, isMobile bool, body []byte) error {
	cachePath := w.CachePath
	if isMobile {
		cachePath += "mobile"
	} else {
		cachePath += "pc"
	}

	cacheFile := cachePath + transToLocalPath(oriPath, oriQuery)
	if len(oriQuery) > 0 {
		// 有查询，判断是否无效查询
		tmpLocalPath := transToLocalPath(oriPath, "")
		cacheNoQueryFile := cachePath + tmpLocalPath
		_, err := os.Stat(cacheNoQueryFile)
		if err == nil {
			// 对比文件内容是否一致，一致则引用
			cacheNoQueryData, err := os.ReadFile(cacheNoQueryFile)
			if err == nil {
				if bytes.Equal(body, cacheNoQueryData) {
					body = []byte(tmpLocalPath)
				}
			}
		}
	}

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
	// 获得路由
	match := ctx.Params().Get("match")
	// 首页不允许通过 no-cache 跳过缓存
	if ctx.GetHeader("Cache-Control") == "no-cache" && match != "index" {
		return "", false
	}
	// 用户登录后，也不缓存
	userId := ctx.Values().GetUintDefault("userId", 0)
	if userId > 0 {
		return "", false
	}
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

	cachePath := w.CachePath
	// 根据实际情况读取缓存
	mobileTemplate := ctx.Values().GetBoolDefault("mobileTemplate", false)
	if mobileTemplate {
		cachePath += "mobile"
	} else {
		cachePath += "pc"
	}
	localPath := transToLocalPath(ctx.RequestPath(false), ctx.Request().URL.RawQuery)
	cacheFile = cachePath + localPath

	info, err := os.Stat(cacheFile)
	if err != nil {
		return "", false
	}
	// 部分缓存文件是引用别的文件，文件长度小于 500 的就做引用检查，只有有query的时候，才会有可能是引用文件，引用文件内容开头是 /
	if len(ctx.Request().URL.RawQuery) > 0 && info.Size() < 500 {
		tmpData, err := os.ReadFile(cacheFile)
		if err != nil {
			return "", false
		}
		if bytes.HasPrefix(tmpData, []byte{'/'}) {
			cacheFile = cachePath + string(tmpData)
			info, err = os.Stat(cacheFile)
			if err != nil {
				return "", false
			}
		}
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

func ViewExists(ctx iris.Context, tplName string) bool {
	//tpl 存放目录，在bootstrap中有
	currentSite := CurrentSite(ctx)
	baseDir := currentSite.GetTemplateDir()
	tplFile := baseDir + "/" + tplName

	_, err := os.Stat(tplFile)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}

	return false
}

// SyncHtmlCacheToStorage
// 传输远程文件
// 如果近期没有生成静态文件，则需要先生成
func (w *Website) SyncHtmlCacheToStorage(localPath, remotePath string) error {
	if localPath == "" {
		// 整站传输
		if w.CacheStorage != nil {
			w.HtmlCachePushStatus = &HtmlCacheStatus{
				StartTime: time.Now().Unix(),
			}
			// send public fist
			err := w.ReadAndSendLocalFiles(w.PublicPath)
			if err != nil {
				log.Println("send public file err", err)
			}
			// and then send cache/pc
			err = w.ReadAndSendLocalFiles(w.CachePath + "pc")

			if w.HtmlCachePushStatus != nil {
				w.HtmlCachePushStatus.Current = w.Tr("AllPushesCompleted")
				w.HtmlCachePushStatus.FinishedTime = time.Now().Unix()
			}
			w.PluginHtmlCache.LastPushTime = time.Now().Unix()
			_ = w.SaveSettingValue(HtmlCacheSettingKey, w.PluginHtmlCache)
		} else {
			return errors.New(w.Tr("StaticServerUndefined"))
		}
	} else {
		// 只传输单个
		if w.CacheStorage != nil {
			info, err := os.Stat(localPath)
			if err != nil {
				return err
			}
			buf, err := os.ReadFile(localPath)
			if err != nil {
				return err
			}

			err = w.ReplaceAndSendCacheFile(remotePath, buf)
			// log
			pushLog := model.HtmlPushLog{
				LocalFile:  strings.TrimPrefix(localPath, w.RootPath),
				RemoteFile: remotePath,
				ModTime:    info.ModTime().Unix(),
				Status:     1,
			}
			if err != nil {
				// 记录错误
				pushLog.Status = 0
				pushLog.ErrorMsg = err.Error()
			}
			w.DB.Save(&pushLog)
			return err
		} else {
			return errors.New(w.Tr("StaticServerUndefined"))
		}
	}
	return nil
}

func (w *Website) ReadAndSendLocalFiles(baseDir string) (err error) {
	if w.CacheStorage == nil {
		return errors.New(w.Tr("StaticServerUndefined"))
	}
	if w.HtmlCachePushStatus == nil {
		w.HtmlCachePushStatus = &HtmlCacheStatus{
			StartTime: time.Now().Unix(),
		}
	}
	baseDir = strings.TrimSuffix(baseDir, "/")
	files, _ := os.ReadDir(baseDir)
	for _, file := range files {
		// .开头的，除了 .htaccess 其他都排除
		if strings.HasPrefix(file.Name(), ".") && file.Name() != ".htaccess" {
			continue
		}
		fullName := baseDir + "/" + file.Name()

		if file.IsDir() {
			// 是目录，继续读取目录内的文件
			err = w.ReadAndSendLocalFiles(fullName)
		} else {
			var buf []byte
			buf, err = os.ReadFile(fullName)
			if err != nil {
				continue
			}
			var remotePath string
			if strings.HasPrefix(fullName, w.PublicPath) {
				// 来自public目录
				remotePath = strings.TrimPrefix(fullName, w.PublicPath)
			} else {
				// 来自cache目录, 只传PC目录
				cachePath := w.CachePath + "pc"
				remotePath = strings.TrimPrefix(fullName, cachePath)
			}

			if len(remotePath) > 0 {
				w.HtmlCachePushStatus.Total++
				w.HtmlCachePushStatus.Current = w.Tr("PushLog", remotePath)
				remotePath = strings.TrimLeft(remotePath, "/")
				// log
				// 如果是记录已存在，并比等待推送的更新，则不推送
				var info fs.FileInfo
				info, err = file.Info()
				if err != nil {
					continue
				}
				// log
				localFile := strings.TrimPrefix(fullName, w.RootPath)
				pushLog, err2 := w.GetHtmlPushLog(localFile)
				if err2 == nil && pushLog.Status == 1 && pushLog.ModTime > info.ModTime().Unix() {
					// 旧文件，忽略
					err = err2
					continue
				}
				if pushLog == nil {
					pushLog = &model.HtmlPushLog{
						LocalFile:  localFile,
						RemoteFile: remotePath,
					}
				}
				pushLog.Status = 1
				pushLog.ModTime = info.ModTime().Unix()
				err = w.ReplaceAndSendCacheFile(remotePath, buf)
				if err == nil {
					w.HtmlCachePushStatus.FinishedCount++
				} else {
					w.HtmlCachePushStatus.ErrorCount++
					w.HtmlCachePushStatus.ErrorMsg = w.Tr("PushFailed:") + remotePath + err.Error()
					// 记录错误
					pushLog.Status = 0
					pushLog.ErrorMsg = err.Error()
				}
				w.DB.Save(&pushLog)
			}
		}
	}

	return err
}

func (w *Website) ReplaceAndSendCacheFile(remotePath string, buf []byte) error {
	// 开始执行一些替换操作
	localUrl := strings.TrimRight(w.System.BaseUrl, "/")
	remoteUrl := strings.TrimRight(w.PluginHtmlCache.StorageUrl, "/")

	if len(localUrl) > 0 && bytes.Contains(buf, []byte(localUrl)) {
		buf = bytes.ReplaceAll(buf, []byte(localUrl), []byte(remoteUrl))
	}
	// 替换为绝对地址
	if len(remoteUrl) > 0 {
		re, _ := regexp.Compile(`(?i)href=["'](/[^/]?.*?)["']`)
		buf = re.ReplaceAllFunc(buf, func(i []byte) []byte {
			matches := re.FindStringSubmatch(string(i))
			if len(matches) < 2 {
				return i
			}
			res := remoteUrl + matches[1]
			str := strings.Replace(matches[0], matches[1], res, 1)
			return []byte(str)
		})
	}

	_, err := w.CacheStorage.UploadFile(remotePath, buf)

	return err
}

func (w *Website) GetCacheBucket() (bucket *BucketStorage, err error) {
	bucket = &BucketStorage{
		DataPath:            w.DataPath,
		PublicPath:          w.PublicPath,
		config:              &w.PluginHtmlCache.PluginStorageConfig,
		tencentBucketClient: nil,
		aliyunBucketClient:  nil,
		qiniuBucketClient:   nil,
		tryTimes:            0,
	}

	err = bucket.initBucket()

	return
}

func (w *Website) InitCacheBucket() {
	if !w.PluginHtmlCache.Open || w.PluginHtmlCache.StorageType == "" {
		// 没设置的不需要
		return
	}
	s, err := w.GetCacheBucket()
	if err != nil {
		w.PluginHtmlCache.ErrorMsg = err.Error()
		log.Println("静态服务器连接失败", err.Error())
		return
	} else {
		w.PluginHtmlCache.ErrorMsg = ""
	}
	w.CacheStorage = s
}

func (w *Website) CleanHtmlPushLog() {
	w.DB.Exec("TRUNCATE `html_push_logs`")
}

func (w *Website) GetHtmlPushLog(localFile string) (*model.HtmlPushLog, error) {
	var pushLog model.HtmlPushLog
	err := w.DB.Where("`local_file` = ?", localFile).Take(&pushLog).Error
	if err != nil {
		return nil, err
	}

	return &pushLog, nil
}

type responseWriterJustWriter struct {
	buf *bytes.Buffer
	io.Writer
}

func newResponseWriter() responseWriterJustWriter {
	buf := &bytes.Buffer{}
	return responseWriterJustWriter{
		buf:    buf,
		Writer: buf,
	}
}

func (responseWriterJustWriter) Header() http.Header {
	log.Println("should not be called")
	return nil
}
func (responseWriterJustWriter) WriteHeader(int) {
	log.Println("should not be called")
}

func (r responseWriterJustWriter) Bytes() []byte {
	return r.buf.Bytes()
}

func (r responseWriterJustWriter) Reset() {
	r.buf.Reset()
}
