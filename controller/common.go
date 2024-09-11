package controller

import (
	"fmt"
	"github.com/jinzhu/now"
	"github.com/kataras/iris/v12"
	captcha "github.com/mojocn/base64Captcha"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/provider"
	"kandaoni.com/anqicms/response"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var Store = captcha.DefaultMemStore

type Button struct {
	Name string
	Link string
}

type NameVal struct {
	Name string
	Val  string
}

var SpiderNames = []NameVal{
	{Name: "googlebot", Val: "google"},
	{Name: "bingbot", Val: "bing"},
	{Name: "baiduspider", Val: "baidu"},
	{Name: "360spider", Val: "360"},
	{Name: "yahoo!", Val: "yahoo"},
	{Name: "sogou", Val: "sogou"},
	{Name: "bytespider", Val: "byte"},
	{Name: "yisouspider", Val: "yisou"},
	{Name: "yandexbot", Val: "yandex"},
	{Name: "spider", Val: "other"},
	{Name: "bot", Val: "other"},
}

var DeviceNames = []NameVal{
	{Name: "android", Val: "android"},
	{Name: "iphone", Val: "iphone"},
	{Name: "windows", Val: "windows"},
	{Name: "macintosh", Val: "mac"},
	{Name: "linux", Val: "linux"},
	{Name: "mobile", Val: "mobile"},
	{Name: "curl", Val: "curl"},
	{Name: "python", Val: "python"},
	{Name: "client", Val: "client"},
	{Name: "spider", Val: "spider"},
	{Name: "bot", Val: "spider"},
}

func NotFound(ctx iris.Context) {
	webInfo := &response.WebInfo{}
	currentSite := provider.CurrentSite(ctx)
	if currentSite != nil {
		webInfo.Title = currentSite.TplTr("404NotFound")
	} else {
		webInfo.Title = "404 Not Found"
	}
	ctx.ViewData("webInfo", webInfo)

	tplName := "errors/404.html"
	if ViewExists(ctx, "errors_404.html") {
		tplName = "errors_404.html"
	}
	ctx.StatusCode(404)
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.StatusCode(404)
		ShowMessage(ctx, "404 Not Found", nil)
	}
}

func ShowMessage(ctx iris.Context, message string, buttons []Button) {
	currentSite := provider.CurrentSite(ctx)
	var tr func(str string, args ...interface{}) string
	if currentSite != nil {
		tr = currentSite.TplTr
	} else {
		tr = func(str string, args ...interface{}) string {
			return str
		}
	}
	str := "<!DOCTYPE html><html><head><meta charset=utf-8><meta name=\"viewport\" content=\"width=device-width,height=device-height,initial-scale=1.0, minimum-scale=1.0, maximum-scale=1.0, user-scalable=no,viewport-fit=cover\"><meta http-equiv=X-UA-Compatible content=\"IE=edge,chrome=1\"><title>" + tr("提示信息") + "</title><style>a{text-decoration: none;color: #777;}</style></head><body style=\"background: #f4f5f7;margin: 0;padding: 20px;\"><div style=\"margin-left: auto;margin-right: auto;margin-top: 50px;padding: 20px;border: 1px solid #eee;background:#fff;max-width: 640px;\"><div>" + message + "</div><div style=\"margin-top: 30px;text-align: right;\"><a style=\"display: inline-block;border:1px solid #777;padding: 8px 16px;\" href=\"javascript:history.back();\">" + tr("返回") + "</a>"

	if len(buttons) > 0 {
		for _, btn := range buttons {
			str += "<a style=\"display: inline-block;border:1px solid #29d;color: #29d;padding: 8px 16px;margin-left: 16px;\" href=\"" + btn.Link + "\">" + tr(btn.Name) + "</a><script type=\"text/javascript\">setTimeout(function(){window.location.href=\"" + btn.Link + "\"}, 3000);</script>"
		}
		str += "<script type=\"text/javascript\">setTimeout(function(){window.location.href=\"" + buttons[0].Link + "\"}, 3000);</script>"
	}
	if currentSite != nil {
		var jsCodes string
		for _, v := range currentSite.PluginPush.JsCodes {
			jsCodes += v.Value + "\n"
		}
		if jsCodes != "" {
			str += jsCodes
		}
	}

	str += "</div></body></html>"

	ctx.WriteString(str)
}

func InternalServerError(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	webInfo := &response.WebInfo{}
	webInfo.Title = currentSite.TplTr("500InternalError")
	ctx.ViewData("webInfo", webInfo)
	var errMessage string
	err := ctx.GetErr()
	message := ctx.Values().GetString("message")
	if err != nil {
		errMessage = err.Error()
	} else if message != "" {
		errMessage = message
	} else {
		errMessage = "(Unexpected) internal server error"
	}
	ctx.ViewData("errMessage", errMessage)
	tplName := "errors/500.html"
	if ViewExists(ctx, "errors_500.html") {
		tplName = "errors_500.html"
	}
	ctx.StatusCode(500)
	err = ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ShowMessage(ctx, errMessage, nil)
	}
}

func CheckCloseSite(ctx iris.Context) bool {
	currentSite := provider.CurrentSite(ctx)
	if !strings.HasPrefix(ctx.GetCurrentRoute().Path(), "/system") {
		// 闭站
		siteClose := currentSite.System.SiteClose == 1
		if currentSite.System.SiteClose == 2 {
			ua := strings.ToLower(ctx.GetHeader("User-Agent"))
			if !strings.Contains(ua, "spider") && !strings.Contains(ua, "bot") {
				// 仅蜘蛛可见
				siteClose = true
			}
		}
		if siteClose {
			closeTips := currentSite.System.SiteCloseTips
			ctx.ViewData("closeTips", closeTips)
			tplName := "errors/close.html"
			if ViewExists(ctx, "errors_close.html") {
				tplName = "errors_close.html"
			}

			if webInfo, ok := ctx.Value("webInfo").(*response.WebInfo); ok {
				webInfo.Title = currentSite.TplTr(closeTips)
				ctx.ViewData("webInfo", webInfo)
			}

			ctx.StatusCode(403)
			err := ctx.View(GetViewPath(ctx, tplName))
			if err != nil {
				ShowMessage(ctx, closeTips, nil)
			}
			return true
		}
		// 禁止蜘蛛抓取
		if currentSite.System.BanSpider == 1 {
			ua := strings.ToLower(ctx.GetHeader("User-Agent"))
			if strings.Contains(ua, "spider") || strings.Contains(ua, "bot") {
				ctx.StatusCode(403)
				ShowMessage(ctx, currentSite.TplTr("YouHaveBeenBanned"), nil)
				return true
			}
		}
		// UA 禁止
		if currentSite.Safe.UAForbidden != "" {
			ua := ctx.GetHeader("User-Agent")
			forbiddens := strings.Split(currentSite.Safe.UAForbidden, "\n")
			for _, v := range forbiddens {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}
				if strings.Contains(ua, v) {
					ctx.StatusCode(403)
					ShowMessage(ctx, currentSite.TplTr("YouHaveBeenBanned"), nil)
					return true
				}
			}
		}
		// ip禁止
		if currentSite.Safe.IPForbidden != "" {
			ip := ctx.RemoteAddr()
			if ip != "127.0.0.1" {
				forbiddens := strings.Split(currentSite.Safe.IPForbidden, "\n")
				for _, v := range forbiddens {
					v = strings.TrimSpace(v)
					if v == "" {
						continue
					}
					// 移除子网掩码
					vs := strings.SplitN(v, "/", 2)
					v = vs[0]
					// 移除后缀.0
					for strings.HasSuffix(v, ".0") {
						v = strings.TrimSuffix(v, ".0")
					}
					if strings.HasPrefix(ip, v) {
						ctx.StatusCode(403)
						ShowMessage(ctx, currentSite.TplTr("YouHaveBeenBanned"), nil)
						return true
					}
				}
			}
		}
	}

	return false
}

func Common(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	//inject ctx
	ctx.ViewData("requestParams", ctx.Params())
	ctx.ViewData("urlParams", ctx.URLParams())
	//version
	ctx.ViewData("version", config.Version)
	// is mobile
	ctx.ViewData("isMobile", ctx.IsMobile())
	//修正baseUrl
	if currentSite.System.BaseUrl == "" {
		urlPath, err := url.Parse(ctx.FullRequestURI())
		if err == nil {
			if ctx.GetHeader("X-Server-Port") == "443" {
				urlPath.Scheme = "https"
			} else if ctx.GetHeader("X-Scheme") == "https" {
				urlPath.Scheme = "https"
			}
			currentSite.System.BaseUrl = urlPath.Scheme + "://" + urlPath.Host
			currentSite.PluginStorage.StorageUrl = currentSite.System.BaseUrl
		}
	}
	//js code
	var jsCodes string
	for _, v := range currentSite.PluginPush.JsCodes {
		jsCodes += v.Value + "\n"
	}
	ctx.ViewData("pluginJsCode", jsCodes)

	// 设置分页
	currentPage := ctx.URLParamIntDefault("page", 1)
	paramPage := ctx.Params().GetIntDefault("page", 0)
	if paramPage > 0 {
		currentPage = paramPage
	}
	ctx.Values().Set("page", currentPage)

	// invite code
	inviteCode := ctx.URLParam("invite")
	if inviteCode != "" {
		_, err := currentSite.CheckUserInviteCode(inviteCode)
		if err != nil {
			inviteCode = ""
		}
	}
	if inviteCode != "" {
		// 生成一个cookie
		ctx.SetCookieKV("invite", inviteCode, iris.CookiePath("/"), iris.CookieExpires(24*30*time.Hour))
	} else {
		// 尝试读取cookie中的数据
		inviteCode = ctx.GetCookie("invite")
	}
	ctx.ViewData("inviteCode", inviteCode)

	ctx.ViewData("today", library.GetToday())

	ctx.Next()
}

func Inspect(ctx iris.Context) {
	uri := ctx.RequestPath(false)
	website := provider.CurrentSite(ctx)
	var siteName string
	if !strings.HasPrefix(uri, "/static") && !strings.HasPrefix(uri, "/install") {
		if provider.GetDefaultDB() == nil {
			ctx.Redirect("/install")
			return
		}

		if website == nil {
			ShowMessage(ctx, ctx.Tr("WebsiteConfigurationError"), nil)
			return
		}
		if !website.Initialed {
			ShowMessage(ctx, ctx.Tr("WebsiteIsClosed"), nil)
			return
		}
		siteName = website.System.SiteName
		// 如果有后台域名，则后台后台将链接跳转到后台
		if strings.HasPrefix(website.System.AdminUrl, "http") {
			parsedUrl, err := url.Parse(website.System.AdminUrl)
			// 如果解析失败，则跳过
			if err == nil {
				if parsedUrl.Hostname() == library.GetHost(ctx) && !strings.HasPrefix(uri, "/system") {
					// 来自后端的域名，但访问的不是后端的业务，则强制跳转到后端。
					ctx.Redirect(strings.TrimRight(website.System.AdminUrl, "/") + "/system")
					return
				}
			}
		}
	}

	ctx.Values().Set("webInfo", &response.WebInfo{Title: siteName, NavBar: 0})
	ctx.ViewData("website", website)
	ctx.Next()
}

func FileServe(ctx iris.Context) bool {
	currentSite := provider.CurrentSite(ctx)
	uri := ctx.RequestPath(false)
	if uri != currentSite.BaseURI && !strings.HasSuffix(uri, "/") {
		baseDir := fmt.Sprintf("%spublic", currentSite.RootPath)
		uriFile := baseDir + strings.TrimPrefix(uri, strings.TrimRight(currentSite.BaseURI, "/"))
		_, err := os.Stat(uriFile)
		if err == nil {
			ctx.ServeFile(uriFile)
			return true
		}
	}
	// 避开 favicon.ico
	if strings.HasSuffix(uri, "favicon.ico") {
		ctx.StatusCode(400)
		return true
	}
	// 自动生成Sitemap
	if ((strings.HasSuffix(uri, "sitemap.xml") && currentSite.PluginSitemap.Type == "xml") ||
		(strings.HasSuffix(uri, "sitemap.txt") && currentSite.PluginSitemap.Type == "txt")) &&
		!ctx.Values().GetBoolDefault("sitemap", false) {
		_ = currentSite.BuildSitemap()
		ctx.Values().Set("sitemap", true)
		return FileServe(ctx)
	}
	// 自动生成robots.txt
	if strings.HasSuffix(uri, "robots.txt") && !ctx.Values().GetBoolDefault("robots", false) {
		robots := "User-agent: *\nDisallow: /system\nDisallow: /static\nSitemap: "
		_ = currentSite.SaveRobots(robots)
		ctx.Values().Set("robots", true)
		return FileServe(ctx)
	}

	return false
}

func ReRouteContext(ctx iris.Context) {
	params, _ := parseRoute(ctx)
	// 先验证文件是否真的存在，如果存在，则fileServe
	exists := FileServe(ctx)
	if exists {
		return
	}
	defer LogAccess(ctx)
	closed := CheckCloseSite(ctx)
	if closed {
		return
	}
	for i, v := range params {
		if len(i) == 0 {
			continue
		}
		ctx.Params().Set(i, v)
		if i == "page" && v > "0" {
			ctx.Values().Set("page", v)
		}
	}

	switch params["match"] {
	case "notfound":
		// 走到 not Found
		break
	case "archive":
		ArchiveDetail(ctx)
		return
	case "archiveIndex":
		ArchiveIndex(ctx)
		return
	case "category":
		CategoryPage(ctx)
		return
	case "page":
		PagePage(ctx)
		return
	case "search":
		SearchPage(ctx)
		return
	case "tagIndex":
		TagIndexPage(ctx)
		return
	case "tag":
		TagPage(ctx)
		return
	case "index":
		IndexPage(ctx)
		return
	case "user":
		UserPage(ctx)
		return
	}

	//如果没有合适的路由，则报错
	NotFound(ctx)
}

func parseRoute(ctx iris.Context) (map[string]string, bool) {
	currentSite := provider.CurrentSite(ctx)
	//这里总共有6条正则规则，需要逐一匹配
	// 由于用户可能会采用相同的配置，因此这里需要尝试多次读取
	matchMap := map[string]string{}
	paramValue := ctx.Params().Get("path")
	paramValue = strings.TrimLeft(strings.TrimPrefix(paramValue, strings.Trim(currentSite.BaseURI, "/")), "/")
	// index
	if paramValue == "" {
		matchMap["match"] = "index"
		return matchMap, true
	}
	// 静态资源直接返回
	if strings.HasPrefix(paramValue, "uploads/") ||
		strings.HasPrefix(paramValue, "static/") ||
		strings.HasPrefix(paramValue, "system/") {
		return matchMap, true
	}
	// 如果匹配到固化链接，则直接返回
	archiveId := currentSite.GetFixedLinkFromCache("/" + paramValue)
	if archiveId > 0 {
		matchMap["match"] = "archive"
		matchMap["id"] = fmt.Sprintf("%d", archiveId)
		return matchMap, true
	}
	// 搜索
	reg := regexp.MustCompile(`^search(/([^/]+?))?$`)
	match := reg.FindStringSubmatch(paramValue)
	if len(match) > 0 {
		matchMap["match"] = "search"
		if len(match) == 3 {
			matchMap["module"] = match[2]
		}
		return matchMap, true
	}
	rewritePattern := currentSite.ParsePatten(false)
	//archivePage
	reg = regexp.MustCompile(rewritePattern.ArchiveIndexRule)
	match = reg.FindStringSubmatch(paramValue)
	if len(match) > 0 {
		matchMap["match"] = "archiveIndex"
		for i, v := range match {
			key := rewritePattern.ArchiveIndexTags[i]
			if i == 0 {
				key = "route"
			}
			matchMap[key] = v
		}
		// 这个规则可能与下面的冲突，因此检查一遍
		module := currentSite.GetModuleFromCacheByToken(matchMap["module"])
		if module != nil {
			return matchMap, true
		}
		matchMap = map[string]string{}
	}
	// people
	reg = regexp.MustCompile("^people/([\\d]+).html$")
	match = reg.FindStringSubmatch(paramValue)

	if len(match) > 1 {
		matchMap["match"] = "user"
		for i, v := range match {
			key := "id"
			if i == 0 {
				key = "route"
			}
			matchMap[key] = v
		}
		return matchMap, true
	}
	//tagIndex
	reg = regexp.MustCompile(rewritePattern.TagIndexRule)
	match = reg.FindStringSubmatch(paramValue)
	if len(match) > 1 {
		matchMap["match"] = "tagIndex"
		for i, v := range match {
			key := rewritePattern.TagIndexTags[i]
			if i == 0 {
				key = "route"
			}
			matchMap[key] = v
		}
		return matchMap, true
	}
	//tag
	reg = regexp.MustCompile(rewritePattern.TagRule)
	match = reg.FindStringSubmatch(paramValue)
	if len(match) > 1 {
		matchMap["match"] = "tag"
		for i, v := range match {
			key := rewritePattern.TagTags[i]
			if i == 0 {
				key = "route"
			}
			matchMap[key] = v
		}
		return matchMap, true
	}
	//page
	reg = regexp.MustCompile(rewritePattern.PageRule)
	match = reg.FindStringSubmatch(paramValue)
	if len(match) > 1 {
		matchMap["match"] = "page"
		for i, v := range match {
			key := rewritePattern.PageTags[i]
			if i == 0 {
				key = "route"
			}
			matchMap[key] = v
		}
		if matchMap["filename"] != "" {
			// 这个规则可能与下面的冲突，因此检查一遍
			category := currentSite.GetCategoryFromCacheByToken(matchMap["filename"])
			if category != nil && category.Type == config.CategoryTypePage {
				return matchMap, true
			}
		} else {
			return matchMap, true
		}
		matchMap = map[string]string{}
	}
	//category
	reg = regexp.MustCompile(rewritePattern.CategoryRule)
	match = reg.FindStringSubmatch(paramValue)
	if len(match) > 1 {
		matchMap["match"] = "category"
		for i, v := range match {
			key := rewritePattern.CategoryTags[i]
			if i == 0 {
				key = "route"
			}
			matchMap[key] = v
		}
		if matchMap["catname"] != "" {
			matchMap["filename"] = matchMap["catname"]
		}
		if matchMap["multicatname"] != "" {
			chunkCatNames := strings.Split(matchMap["multicatname"], "/")
			matchMap["filename"] = chunkCatNames[len(chunkCatNames)-1]
		}
		if matchMap["module"] != "" {
			// 需要先验证是否是module
			module := currentSite.GetModuleFromCacheByToken(matchMap["module"])
			if module != nil {
				if matchMap["filename"] != "" {
					// 这个规则可能与下面的冲突，因此检查一遍
					category := currentSite.GetCategoryFromCacheByToken(matchMap["filename"])
					if category != nil && category.Type != config.CategoryTypePage {
						return matchMap, true
					}
				} else {
					return matchMap, true
				}
			}
		} else {
			if matchMap["filename"] != "" {
				// 这个规则可能与下面的冲突，因此检查一遍
				category := currentSite.GetCategoryFromCacheByToken(matchMap["filename"])
				if category != nil && category.Type != config.CategoryTypePage {
					return matchMap, true
				}
			} else {
				return matchMap, true
			}
		}
		matchMap = map[string]string{}
	}
	//最后archive
	reg = regexp.MustCompile(rewritePattern.ArchiveRule)
	match = reg.FindStringSubmatch(paramValue)
	if len(match) > 1 {
		matchMap["match"] = "archive"
		for i, v := range match {
			key := rewritePattern.ArchiveTags[i]
			if i == 0 {
				key = "route"
			}
			matchMap[key] = v
		}
		if matchMap["module"] != "" {
			// 需要先验证是否是module
			module := currentSite.GetModuleFromCacheByToken(matchMap["module"])
			if module != nil {
				return matchMap, true
			}
		} else {
			return matchMap, true
		}
	}

	//不存在，定义到notfound
	matchMap["match"] = "notfound"
	return matchMap, true
}

// GetViewPath
// 区分mobile的模板和pc的模板
func GetViewPath(ctx iris.Context, tplName string) string {
	mobileTemplate := ctx.Values().GetBoolDefault("mobileTemplate", false)
	if mobileTemplate {
		tplName = fmt.Sprintf("mobile/%s", tplName)
	}
	return tplName
}

func ViewExists(ctx iris.Context, tplName string) bool {
	//tpl 存放目录，在bootstrap中有
	currentSite := provider.CurrentSite(ctx)
	baseDir := currentSite.GetTemplateDir()
	tplFile := baseDir + "/" + GetViewPath(ctx, tplName)

	_, err := os.Stat(tplFile)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}

	return false
}

func CheckTemplateType(ctx iris.Context) {
	//后台不需要处理
	if strings.HasPrefix(ctx.GetCurrentRoute().Path(), "/system") {
		ctx.Next()
		return
	}

	currentSite := provider.CurrentSite(ctx)
	mobileTemplate := false
	switch currentSite.System.TemplateType {
	case config.TemplateTypeSeparate:
		// 电脑+手机，需要根据当前的域名情况来处理模板
		// 三种情况要处理：手机端访问pc端域名，需要执行301操作
		// 手机端访问手机端域名，加载手机端模板
		// pc端访问访问移动端域名，加载手机端模板
		// 特殊情况，没有填写手机端域名
		if currentSite.System.MobileUrl == "" {
			break
		}
		//解析mobileUrl
		mobileUrl, err := url.Parse(currentSite.System.MobileUrl)
		if err != nil {
			break
		}

		if !strings.EqualFold(library.GetHost(ctx), mobileUrl.Hostname()) {
			// 电脑端访问，检查是否需要301
			if ctx.IsMobile() {
				ctx.Redirect(currentSite.System.MobileUrl+ctx.Request().RequestURI, 301)
				return
			}
		} else {
			// 这个时候，访问的是mobile域名，不管什么端访问mobile域名，都直接使用手机模板
			mobileTemplate = true
		}
	case config.TemplateTypeAdapt:
		// 代码适配，如果发现是手机端访问，则启用手机模板
		if ctx.IsMobile() {
			mobileTemplate = true
		}
	default:
		// 自适应，不需要处理
	}

	ctx.Values().Set("mobileTemplate", mobileTemplate)

	ctx.Next()
}

func LogAccess(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	if currentSite == nil {
		ctx.Next()
		return
	}
	if ctx.IsAjax() || ctx.Method() != "GET" {
		ctx.Next()
		return
	}
	// html cache 步骤不做记录
	if ctx.GetHeader("Cache") == "true" {
		ctx.Next()
		return
	}
	currentPath := ctx.Request().RequestURI
	//后台不做记录
	if strings.HasPrefix(currentPath, "/system") {
		ctx.Next()
		return
	}
	//静态资源不做记录
	if strings.HasPrefix(currentPath, "/static") ||
		strings.HasPrefix(currentPath, "/uploads") ||
		strings.Contains(currentPath, "/js") ||
		strings.Contains(currentPath, "/css") ||
		strings.Contains(currentPath, "/image") ||
		strings.HasSuffix(currentPath, ".ico") ||
		strings.HasSuffix(currentPath, ".jpg") ||
		strings.HasSuffix(currentPath, ".png") ||
		strings.HasSuffix(currentPath, ".jpeg") ||
		strings.HasSuffix(currentPath, ".gif") ||
		strings.HasSuffix(currentPath, ".js") ||
		strings.HasSuffix(currentPath, ".css") ||
		strings.HasSuffix(currentPath, ".map") ||
		strings.HasSuffix(currentPath, ".webp") {
		ctx.Next()
		return
	}

	userAgent := ctx.GetHeader("User-Agent")
	//获取蜘蛛
	spider := GetSpider(userAgent)
	//获取设备
	device := GetDevice(userAgent)
	// 最多只存储250字符
	if len(currentPath) > 250 {
		currentPath = currentPath[:250]
	}
	// 最多只存储250字符
	if len(userAgent) > 250 {
		userAgent = userAgent[:250]
	}

	statistic := &provider.Statistic{
		Spider:    spider,
		Host:      ctx.Request().Host,
		Url:       currentPath,
		Ip:        ctx.RemoteAddr(),
		Device:    device,
		HttpCode:  ctx.GetStatusCode(),
		UserAgent: userAgent,
	}
	// 这里不需要等待
	go currentSite.StatisticLog.Write(statistic)

	ctx.Next()
}

func GetSpider(ua string) string {
	ua = strings.ToLower(ua)
	//获取蜘蛛
	for _, v := range SpiderNames {
		if strings.Contains(ua, v.Name) {
			return v.Val
		}
	}

	return ""
}

func GetDevice(ua string) string {
	ua = strings.ToLower(ua)

	for _, v := range DeviceNames {
		if strings.Contains(ua, v.Name) {
			return v.Val
		}
	}

	return "proxy"
}

func NewDriver() *captcha.DriverString {
	driver := new(captcha.DriverString)
	driver.Height = 76
	driver.Width = 240
	//driver.NoiseCount = 4
	//driver.ShowLineOptions = captcha.OptionShowSineLine | captcha.OptionShowSlimeLine | captcha.OptionShowHollowLine
	driver.Fonts = []string{"Flim-Flam.ttf", "RitaSmith.ttf"}
	driver.Length = 4
	driver.Source = "1234567890qwertyuipkjhgfdsazxcvbnm"

	return driver
}

func GenerateCaptcha(ctx iris.Context) {
	currentSite := provider.CurrentSite(ctx)
	safeSetting := currentSite.Safe
	if safeSetting.AdminCaptchaOff == 1 {
		ctx.JSON(iris.Map{
			"code": config.StatusOK,
			"msg":  "",
			"data": iris.Map{
				"captcha_off": true,
			},
		})
		return
	}

	var driver = NewDriver().ConvertFonts()
	c := captcha.NewCaptcha(driver, Store)
	id, content, answer := c.Driver.GenerateIdQuestionAnswer()
	item, _ := c.Driver.DrawCaptcha(content)
	c.Store.Set(id, answer)

	bs64 := item.EncodeB64string()

	ctx.JSON(iris.Map{
		"code": config.StatusOK,
		"msg":  "",
		"data": iris.Map{
			"captcha_off": false,
			"captcha_id":  id,
			"captcha":     bs64,
		},
	})
}

func SafeVerify(ctx iris.Context, req map[string]string, returnType string, from string) bool {
	currentSite := provider.CurrentSite(ctx)
	// 检查如果用户是否登录
	// 是否需要验证码
	var contentCaptcha = currentSite.Safe.Captcha == 1
	userGroup := ctx.Values().Get("userGroup")
	if userGroup != nil {
		group, ok := userGroup.(*model.UserGroup)
		if ok {
			contentCaptcha = !group.Setting.ContentNoCaptcha
		}
	}
	// 检查验证码
	if contentCaptcha {
		captchaId := ctx.PostValueTrim("captcha_id")
		captchaValue := ctx.PostValueTrim("captcha")
		if req != nil {
			captchaId = req["captcha_id"]
			captchaValue = req["captcha"]
		}
		// 验证 captcha
		if captchaId == "" {
			if returnType == "json" {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  currentSite.TplTr("VerificationCodeIsIncorrect"),
				})
			} else {
				ShowMessage(ctx, currentSite.TplTr("VerificationCodeIsIncorrect"), nil)
			}
			return false
		}
		if ok := Store.Verify(captchaId, captchaValue, true); !ok {
			if returnType == "json" {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  currentSite.TplTr("VerificationCodeIsIncorrect"),
				})
			} else {
				ShowMessage(ctx, currentSite.TplTr("VerificationCodeIsIncorrect"), nil)
			}
			return false
		}
	}
	// 内容长度现在 ContentLimit
	content := ctx.PostValueTrim("content")
	if req != nil {
		content = req["content"]
	}
	if currentSite.Safe.ContentLimit > 0 {
		if utf8.RuneCountInString(content) < currentSite.Safe.ContentLimit {
			if returnType == "json" {
				ctx.JSON(iris.Map{
					"code": config.StatusFailed,
					"msg":  currentSite.TplTr("TheContentYouSubmittedIsTooShort"),
				})
			} else {
				ShowMessage(ctx, currentSite.TplTr("TheContentYouSubmittedIsTooShort"), nil)
			}
			return false
		}
	}
	// 禁止的内容
	if currentSite.Safe.ContentForbidden != "" {
		forbiddens := strings.Split(currentSite.Safe.ContentForbidden, "\n")
		for _, v := range forbiddens {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if strings.Contains(content, v) {
				if returnType == "json" {
					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  currentSite.TplTr("TheContentYouSubmittedContainsCharactersThatAreNotAllowed"),
					})
				} else {
					ShowMessage(ctx, currentSite.TplTr("TheContentYouSubmittedContainsCharactersThatAreNotAllowed"), nil)
				}
				return false
			}
			if req != nil {
				// 对于req的所有内容都进行判断
				for _, rv := range req {
					if len(rv) == 0 {
						continue
					}
					if strings.Contains(rv, v) {
						if returnType == "json" {
							ctx.JSON(iris.Map{
								"code": config.StatusFailed,
								"msg":  currentSite.TplTr("TheContentYouSubmittedContainsCharactersThatAreNotAllowed"),
							})
						} else {
							ShowMessage(ctx, currentSite.TplTr("TheContentYouSubmittedContainsCharactersThatAreNotAllowed"), nil)
						}
					}
				}
			}
		}
	}

	ip := ctx.RemoteAddr()
	if ip != "127.0.0.1" {
		// 检查每日限制
		if currentSite.Safe.DailyLimit > 0 {
			var todayCount int64
			if from == "guestbook" {
				currentSite.DB.Model(&model.Guestbook{}).Where("`ip` = ? and `created_time` >= ?", ip, now.BeginningOfDay().Unix()).Count(&todayCount)
			} else if from == "comment" {
				currentSite.DB.Model(&model.Comment{}).Where("`ip` = ? and `created_time` >= ?", ip, now.BeginningOfDay().Unix()).Count(&todayCount)
			}
			if int(todayCount) >= currentSite.Safe.DailyLimit {
				if returnType == "json" {
					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  currentSite.TplTr("TheUpperLimitOfSubmissionsHasBeenReached"),
					})
				} else {
					ShowMessage(ctx, currentSite.TplTr("TheUpperLimitOfSubmissionsHasBeenReached"), nil)
				}
				return false
			}
		}
		// 检查提交间隔
		if currentSite.Safe.IntervalLimit > 0 {
			var lastTime int64
			if from == "guestbook" {
				if userName, ok := req["user_name"]; ok {
					// if username is longer then normal
					if len(userName) > 50 {
						if returnType == "json" {
							ctx.JSON(iris.Map{
								"code": config.StatusFailed,
								"msg":  currentSite.TplTr("IllegalRequest"),
							})
						} else {
							ShowMessage(ctx, currentSite.TplTr("IllegalRequest"), nil)
						}
						return false
					}
				}
				err := currentSite.DB.Model(&model.Guestbook{}).Where("`ip` = ?", ip).Order("id desc").Pluck("created_time", &lastTime).Error
				if err != nil {
					if contact, ok := req["contact"]; ok {
						err = currentSite.DB.Model(&model.Guestbook{}).Where("`contact` = ?", contact).Order("id desc").Pluck("created_time", &lastTime).Error
					}
				}
				if err != nil {
					if userName, ok := req["user_name"]; ok {
						currentSite.DB.Model(&model.Guestbook{}).Where("`user_name` = ?", userName).Order("id desc").Pluck("created_time", &lastTime)
					}
				}
			} else if from == "comment" {
				currentSite.DB.Model(&model.Comment{}).Where("`ip` = ?", ip).Order("id desc").Pluck("created_time", &lastTime)
			}
			if lastTime > 0 && lastTime >= time.Now().Unix()-int64(currentSite.Safe.IntervalLimit) {
				if returnType == "json" {
					ctx.JSON(iris.Map{
						"code": config.StatusFailed,
						"msg":  currentSite.TplTr("PleaseDoNotSubmitMultipleTimesInAShortPeriodOfTime"),
					})
				} else {
					ShowMessage(ctx, currentSite.TplTr("PleaseDoNotSubmitMultipleTimesInAShortPeriodOfTime"), nil)
				}
				return false
			}
		}
	}

	return true
}
