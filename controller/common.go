package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"irisweb/config"
	"irisweb/model"
	"irisweb/response"
	"net/url"
	"strings"
)

var webInfo response.WebInfo

func NotFound(ctx iris.Context) {
	ctx.View(GetViewPath(ctx, "errors/404.html"))
}

func InternalServerError(ctx iris.Context) {
	errMessage := ctx.Values().GetString("message")
	if errMessage == "" {
		errMessage = "(Unexpected) internal server error"
	}
	ctx.ViewData("errMessage", errMessage)
	ctx.View(GetViewPath(ctx, "errors/500.html"))
}

func CheckCloseSite(ctx iris.Context) {
	if config.JsonData.System.SiteClose == 1 && !strings.HasPrefix(ctx.GetCurrentRoute().Path(), config.JsonData.System.AdminUri) {
		closeTips := config.JsonData.System.SiteCloseTips
		ctx.ViewData("closeTips", closeTips)
		ctx.View(GetViewPath(ctx, "errors/close.html"))
		return
	}

	ctx.Next()
}

func Common(ctx iris.Context) {
	//inject ctx
	ctx.ViewData("requestParams", ctx.Params())
	ctx.ViewData("urlParams", ctx.URLParams())
	//version
	ctx.ViewData("version", config.Version)
	//修正baseUrl
	if config.JsonData.System.BaseUrl == "" {
		urlPath, err := url.Parse(ctx.FullRequestURI())
		if err == nil {
			config.JsonData.System.BaseUrl = urlPath.Scheme + "://" + urlPath.Host
		}
	}
	//js code
	ctx.ViewData("pluginJsCode", config.JsonData.PluginPush.JsCode)

	webInfo.NavBar = 0
	ctx.Next()
}

func Inspect(ctx iris.Context) {
	if config.DB == nil && ctx.GetCurrentRoute().Path() != "/install" {
		ctx.Redirect("/install")
		return
	}

	ctx.Next()
}

func ReRouteContext(ctx iris.Context) {
	params := ctx.Params().GetEntry("params").Value().(map[string]string)
	for i, v := range params {
		ctx.Params().Set(i, v)
	}

	switch params["match"] {
	case "article":
		ArticleDetail(ctx)
		return
	case "product":
		ProductDetail(ctx)
		return
	case "category":
		CategoryPage(ctx)
		return
	case "page":
		PagePage(ctx)
		return
	case "articleIndex":
		ArticleIndexPage(ctx)
		return
	case "productIndex":
		ProductIndexPage(ctx)
		return
	}

	//如果没有合适的路由，则报错
	NotFound(ctx)
}

// 区分mobile的模板和pc的模板
func GetViewPath(ctx iris.Context, tplName string) string {
	mobileTemplate := ctx.Values().GetBoolDefault("mobileTemplate", false)
	if mobileTemplate {
		tplName = fmt.Sprintf("mobile/%s", tplName)
	}

	return tplName
}

func CheckTemplateType(ctx iris.Context) {
	//后台不需要处理
	if strings.HasPrefix(ctx.GetCurrentRoute().Path(), config.JsonData.System.AdminUri) {
		ctx.Next()
		return
	}

	mobileTemplate := false
	switch config.JsonData.System.TemplateType {
	case config.TemplateTypeSeparate:
		// 电脑+手机，需要根据当前的域名情况来处理模板
		// 三种情况要处理：手机端访问pc端域名，需要执行301操作
		// 手机端访问手机端域名，加载手机端模板
		// pc端访问访问移动端域名，加载手机端模板
		// 特殊情况，没有填写手机端域名
		if config.JsonData.System.MobileUrl == "" {
			break
		}
		//解析mobileUrl
		mobileUrl, err := url.Parse(config.JsonData.System.MobileUrl)
		if err != nil {
			break
		}

		if !strings.EqualFold(ctx.Host(), mobileUrl.Hostname()) {
			// 电脑端访问，检查是否需要301
			if ctx.IsMobile() {
				ctx.Redirect(config.JsonData.System.MobileUrl, 301)
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
	if config.DB == nil {
		ctx.Next()
		return
	}
	//后台不做记录
	if strings.HasPrefix(ctx.GetCurrentRoute().Path(), config.JsonData.System.AdminUri) {
		ctx.Next()
		return
	}

	//获取蜘蛛
	spider := GetSpider(ctx)
	//获取设备
	device := GetDevice(ctx)

	statistic := &model.Statistic{
		Spider:    spider,
		Host:      ctx.Host(),
		Url:       ctx.RequestPath(false),
		Ip:        ctx.RemoteAddr(),
		Device:    device,
		HttpCode:  ctx.GetStatusCode(),
		UserAgent: ctx.GetHeader("User-Agent"),
	}
	config.DB.Save(statistic)

	ctx.Next()
}

func GetSpider(ctx iris.Context) string {
	ua := strings.ToLower(ctx.GetHeader("User-Agent"))
	//获取蜘蛛
	spiders := map[string]string{
		"googlebot":   "google",
		"bingbot":     "bing",
		"baiduspider": "baidu",
		"360spider":   "360",
		"yahoo!":      "yahoo",
		"sogou":       "sogou",
		"bytespider":  "byte",
		"spider":      "other",
		"bot":         "other",
	}

	for k, v := range spiders {
		if strings.Contains(ua, k) {
			return v
		}
	}

	return ""
}

func GetDevice(ctx iris.Context) string {
	ua := strings.ToLower(ctx.GetHeader("User-Agent"))

	devices := map[string]string{
		"android":   "android",
		"iphone":    "iphone",
		"windows":   "windows",
		"macintosh": "mac",
		"linux":     "linux",
		"mobile":    "mobile",
		//其他设备
		"spider": "spider",
		"bot":    "spider",
	}

	for k, v := range devices {
		if strings.Contains(ua, k) {
			return v
		}
	}

	return "proxy"
}
