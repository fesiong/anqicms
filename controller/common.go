package controller

import (
	"fmt"
	"github.com/kataras/iris/v12"
	captcha "github.com/mojocn/base64Captcha"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
	"net/url"
	"os"
	"strings"
)

var webInfo response.WebInfo
var Store = captcha.DefaultMemStore

func NotFound(ctx iris.Context) {
	webInfo.Title = config.Lang("404 Not Found")
	ctx.ViewData("webInfo", webInfo)

	tplName := "errors/404.html"
	if ViewExists(ctx, "errors_404.html") {
		tplName = "errors_404.html"
	}
	ctx.StatusCode(404)
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ctx.Values().Set("message", err.Error())
	}
}

func ShowMessage(ctx iris.Context, message string, link string) {
	str := "<!DOCTYPE html><html><head><meta charset=utf-8><meta http-equiv=X-UA-Compatible content=\"IE=edge,chrome=1\"><title>"+config.Lang("提示信息")+"</title><style>a{text-decoration: none;color: #777;}</style></head><body style=\"background: #f4f5f7;margin: 0;padding: 20px;\"><div style=\"margin-left: auto;margin-right: auto;margin-top: 50px;padding: 20px;border: 1px solid #eee;background:#fff;max-width: 640px;\"><div>"+message+"</div><div style=\"margin-top: 30px;text-align: right;\"><a style=\"display: inline-block;border:1px solid #777;padding: 8px 16px;\" href=\"javascript:history.back();\">"+config.Lang("返回")+"</a>"

	if link != "" {
		str += "<a style=\"display: inline-block;border:1px solid #29d;color: #29d;padding: 8px 16px;margin-left: 16px;\" href=\"" + link + "\">"+config.Lang("点击继续")+"</a><script type=\"text/javascript\">setTimeout(function(){window.location.href=\"" + link + "\"}, 3000);</script>"
	}

	str += "</div></body></html>"

	ctx.WriteString(str)
}

func InternalServerError(ctx iris.Context) {
	webInfo.Title = config.Lang("500 Internal Error")
	ctx.ViewData("webInfo", webInfo)

	errMessage := ctx.Values().GetString("message")
	if errMessage == "" {
		errMessage = "(Unexpected) internal server error"
	}
	ctx.ViewData("errMessage", errMessage)
	tplName := "errors/500.html"
	if ViewExists(ctx, "errors_500.html") {
		tplName = "errors_500.html"
	}
	ctx.StatusCode(500)
	err := ctx.View(GetViewPath(ctx, tplName))
	if err != nil {
		ShowMessage(ctx, errMessage, "")
	}
}

func CheckCloseSite(ctx iris.Context) {
	if config.JsonData.System.SiteClose == 1 && !strings.HasPrefix(ctx.GetCurrentRoute().Path(), "/system") {
		closeTips := config.JsonData.System.SiteCloseTips
		ctx.ViewData("closeTips", closeTips)
		tplName := "errors/close.html"
		if ViewExists(ctx, "errors_close.html") {
			tplName = "errors_close.html"
		}

		webInfo.Title = config.Lang(closeTips)
		ctx.ViewData("webInfo", webInfo)

		err := ctx.View(GetViewPath(ctx, tplName))
		if err != nil {
			ctx.Values().Set("message", err.Error())
		}
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
			if ctx.GetHeader("X-Server-Port") == "443" {
				urlPath.Scheme = "https"
			}
			config.JsonData.System.BaseUrl = urlPath.Scheme + "://" + urlPath.Host
		}
	}
	//js code
	var jsCodes string
	for _, v := range config.JsonData.PluginPush.JsCodes {
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

	webInfo.NavBar = 0
	ctx.Next()
}

func Inspect(ctx iris.Context) {
	uri := ctx.RequestPath(false)
	if dao.DB == nil && !strings.HasPrefix(uri, "/static") && !strings.HasPrefix(uri, "/install") {
		ctx.Redirect("/install")
		return
	}

	ctx.Next()
}

func FileServe(ctx iris.Context) bool {
	uri := ctx.RequestPath(false)
	if uri != "/" {
		baseDir := fmt.Sprintf("%spublic", config.ExecPath)
		uriFile := baseDir + uri
		_, err := os.Stat(uriFile)
		if err == nil {
			ctx.ServeFile(uriFile, false)
			return true
		}
	}

	return false
}

func ReRouteContext(ctx iris.Context) {
	defer LogAccess(ctx)
	// 先验证文件是否真的存在，如果存在，则fileServe
	exists := FileServe(ctx)
	if exists {
		return
	}

	params := ctx.Params().GetEntry("params").Value().(map[string]string)
	for i, v := range params {
		ctx.Params().Set(i, v)
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
	}

	//如果没有合适的路由，则报错
	NotFound(ctx)
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
	baseDir := fmt.Sprintf("%stemplate/%s", config.ExecPath, config.JsonData.System.TemplateName)
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
	if dao.DB == nil {
		ctx.Next()
		return
	}
	if ctx.IsAjax() || ctx.Method() != "GET" {
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
		strings.HasSuffix(currentPath, ".webp")  {
		ctx.Next()
		return
	}

	//获取蜘蛛
	spider := GetSpider(ctx)
	//获取设备
	device := GetDevice(ctx)

	statistic := &model.Statistic{
		Spider:    spider,
		Host:      ctx.Request().Host,
		Url:       ctx.Request().RequestURI,
		Ip:        ctx.RemoteAddr(),
		Device:    device,
		HttpCode:  ctx.GetStatusCode(),
		UserAgent: ctx.GetHeader("User-Agent"),
	}
	// 这里不需要等待
	go dao.DB.Save(statistic)

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

func NewDriver() *captcha.DriverString {
	driver := new(captcha.DriverString)
	driver.Height = 76
	driver.Width = 240
	//driver.NoiseCount = 4
	//driver.ShowLineOptions = captcha.OptionShowSineLine | captcha.OptionShowSlimeLine | captcha.OptionShowHollowLine
	driver.Fonts = []string{"Flim-Flam.ttf","RitaSmith.ttf"}
	driver.Length = 4
	driver.Source = "1234567890qwertyuipkjhgfdsazxcvbnm"

	return driver
}

func GenerateCaptcha(ctx iris.Context) {
	var driver = NewDriver().ConvertFonts()
	c := captcha.NewCaptcha(driver, Store)
	id, content, answer := c.Driver.GenerateIdQuestionAnswer()
	item, _ := c.Driver.DrawCaptcha(content)
	c.Store.Set(id, answer)

	bs64 := item.EncodeB64string()

	ctx.JSON(iris.Map{
		"code":       config.StatusOK,
		"msg":        "",
		"data":       iris.Map{
			"captcha_id": id,
			"captcha": bs64,
		},
	})
}