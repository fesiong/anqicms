package library

import (
	"context"
	"crypto/tls"
	"github.com/parnurzeal/gorequest"
	"golang.org/x/net/html/charset"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// charsetPatternInDOMStr meta[http-equiv]元素, content属性中charset截取的正则模式.
// 如<meta http-equiv="content-type" content="text/html; charset=utf-8">
var charsetPatternInDOMStr = `charset\s*=\s*(\S*)\s*;?`

// charsetPattern 普通的MatchString可直接接受模式字符串, 无需Compile,
// 但是只能作为判断是否匹配, 无法从中获取其他信息.
var charsetPattern = regexp.MustCompile(charsetPatternInDOMStr)

type RequestData struct {
	Header     http.Header
	Request    *http.Request
	Body       string
	Status     string
	StatusCode int
	Domain     string
	Scheme     string
	IP         string
	Server     string
}

type Options struct {
	Timeout     time.Duration
	Debug       bool
	Method      string
	Type        string
	Query       interface{}
	Data        interface{}
	Header      map[string]string
	Proxy       string
	Cookies     []*http.Cookie
	UserAgent   string
	IsMobile    bool
	DialContext func(ctx context.Context, network, addr string) (net.Conn, error)
}

// Request
// 请求网络页面，并自动检测页面内容的编码，转换成utf-8
func Request(urlPath string, options *Options) (*RequestData, error) {
	if options == nil {
		options = &Options{
			Method:    "GET",
			Timeout:   10,
			UserAgent: GetUserAgent(false),
		}
	}
	if options.Timeout == 0 {
		options.Timeout = 10
	}
	if options.Method == "" {
		options.Method = "GET"
	}
	options.Method = strings.ToUpper(options.Method)

	req := gorequest.New().SetDoNotClearSuperAgent(true).TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).Timeout(options.Timeout * time.Second)
	if options.Debug {
		req = req.SetDebug(true)
	}
	//定义默认的refer
	parsedUrl, err := url.Parse(urlPath)
	if err != nil {
		//log.Println(err)
		return nil, err
	}
	parsedUrl.Path = ""
	parsedUrl.RawQuery = ""
	parsedUrl.Fragment = ""
	req = req.Set("Referer", parsedUrl.String())
	if options.Type != "" {
		req = req.Type(options.Type)
	}
	if options.Cookies != nil {
		req = req.AddCookies(options.Cookies)
	}
	if options.Query != nil {
		req = req.Query(options.Query)
	}
	if options.Data != nil {
		req = req.Send(options.Data)
	}
	if options.Header != nil {
		for i, v := range options.Header {
			req = req.Set(i, v)
		}
	}
	if options.Proxy != "" {
		req = req.Proxy(options.Proxy)
	}

	if options.UserAgent == "" {
		options.UserAgent = GetUserAgent(options.IsMobile)
	}
	req = req.Set("User-Agent", options.UserAgent)

	if options.DialContext != nil {
		req.Transport.DialContext = options.DialContext
	}

	if options.Method == "POST" {
		req = req.Post(urlPath)
	} else {
		req = req.Get(urlPath)
	}

	resp, body, errs := req.End()
	if len(errs) > 0 {
		//如果是https,则尝试退回http请求
		if strings.HasPrefix(urlPath, "https") {
			urlPath = strings.Replace(urlPath, "https://", "http://", 1)
			return Request(urlPath, options)
		}

		return &RequestData{}, errs[0]
	}

	resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "html") {
		// 编码处理
		charsetName, err := getPageCharset(body, contentType)
		if err != nil {
			log.Println("获取页面编码失败: ", err.Error())
		}
		charsetName = strings.ToLower(charsetName)
		//log.Println("当前页面编码:", charsetName)
		charSet, _ := CharsetMap[charsetName]
		if charSet != nil {
			utf8Coutent, err := DecodeToUTF8([]byte(body), charSet)
			if err != nil {
				log.Println("页面解码失败: ", err.Error())
			} else {
				body = string(utf8Coutent)
			}
		}
	}

	requestData := RequestData{
		Header:     resp.Header,
		Request:    resp.Request,
		Body:       body,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Domain:     resp.Request.Host,
		Scheme:     resp.Request.URL.Scheme,
		Server:     resp.Header.Get("Server"),
	}

	return &requestData, nil
}

// getPageCharset 解析页面, 从中获取页面编码信息
func getPageCharset(content, contentType string) (charSet string, err error) {
	//log.Println("服务器返回编码：", contentType)
	if contentType != "" {
		matchedArray := charsetPattern.FindStringSubmatch(strings.ToLower(contentType))
		if len(matchedArray) > 1 {
			for _, matchedItem := range matchedArray[1:] {
				if strings.ToLower(matchedItem) != "utf-8" {
					charSet = matchedItem
					return
				}
			}
		}
	}
	//log.Println("继续查找编码1")
	var checkType string
	reg := regexp.MustCompile(`(?is)<title[^>]*>(.*?)<\/title>`)
	match := reg.FindStringSubmatch(content)
	if len(match) > 1 {
		_, checkType, _ = charset.DetermineEncoding([]byte(match[1]), "")
		//log.Println("Title解析编码：", checkType)
		if checkType == "utf-8" {
			charSet = checkType
			return
		}
	}
	//log.Println("继续查找编码2")
	reg = regexp.MustCompile(`(?is)<meta[^>]*charset\s*=["']?\s*([\w\d\-]+)`)
	match = reg.FindStringSubmatch(content)
	if len(match) > 1 {
		charSet = match[1]
		return
	}
	//log.Println("找不到编码")
	charSet = "utf-8"
	return
}

func GetUserAgent(isMobile bool) string {
	if isMobile {
		return "Mozilla/5.0 (iPhone; CPU iPhone OS 10_3_1 like Mac OS X) AppleWebKit/603.1.30 (KHTML, like Gecko) Version/10.0 Mobile/14E304 Safari/602.1"
	}

	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.71 Safari/537.36"
}

func GetURLData(url, refer string, timeout int) (*RequestData, error) {
	//log.Println(url)
	client := &http.Client{}
	if timeout > 0 {
		client.Timeout = time.Duration(timeout) * time.Second
	} else if timeout < 0 {
		client.Timeout = 10 * time.Second
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", GetUserAgent(false))
	req.Header.Set("Referer", refer)

	resp, err := client.Do(req)
	if err != nil {
		return &RequestData{}, err
	}
	body, _ := io.ReadAll(resp.Body)

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "html") {
		// 编码处理
		charsetName, err := getPageCharset(string(body), contentType)
		if err != nil {
			log.Println("获取页面编码失败: ", err.Error())
		}
		charsetName = strings.ToLower(charsetName)
		//log.Println("当前页面编码:", charsetName)
		charSet, exist := CharsetMap[charsetName]
		if !exist {
			log.Println("未找到匹配的编码")
		}
		if charSet != nil {
			utf8Coutent, err := DecodeToUTF8(body, charSet)
			if err != nil {
				log.Println("页面解码失败: ", err.Error())
			} else {
				body = utf8Coutent
			}
		}
	}

	requestData := RequestData{
		Header:     resp.Header,
		Request:    resp.Request,
		Body:       string(body),
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Domain:     resp.Request.Host,
		Scheme:     resp.Request.URL.Scheme,
		Server:     resp.Header.Get("Server"),
	}

	return &requestData, nil
}
