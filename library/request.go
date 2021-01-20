package library

import (
	"github.com/axgle/mahonia"
	"github.com/parnurzeal/gorequest"
	"golang.org/x/net/html/charset"
	"log"
	"regexp"
	"strings"
	"time"
)

type RequestData struct {
	Body   string
	Domain string
	Scheme string
	IP     string
	Server string
}

/**
 * 请求域名返回数据
 */
func Request(urlPath string) (*RequestData, error) {
	resp, body, errs := gorequest.New().Timeout(30 * time.Second).Get(urlPath).End()
	if len(errs) > 0 {
		//如果是https,则尝试退回http请求
		if strings.HasPrefix(urlPath, "https") {
			urlPath = strings.Replace(urlPath, "https://", "http://", 1)
			return Request(urlPath)
		}
		return nil, errs[0]
	}
	defer resp.Body.Close()
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	var htmlEncode string

	if strings.Contains(contentType, "gbk") || strings.Contains(contentType, "gb2312") || strings.Contains(contentType, "gb18030") || strings.Contains(contentType, "windows-1252") {
		htmlEncode = "gb18030"
	} else if strings.Contains(contentType, "big5") {
		htmlEncode = "big5"
	} else if strings.Contains(contentType, "utf-8") {
		htmlEncode = "utf-8"
	}

	if htmlEncode == "" {
		//先尝试读取charset
		reg := regexp.MustCompile(`(?is)<meta[^>]*charset\s*=["']?\s*([A-Za-z0-9\-]+)`)
		match := reg.FindStringSubmatch(body)
		if len(match) > 1 {
			contentType = strings.ToLower(match[1])
			log.Println(contentType)
			if strings.Contains(contentType, "gbk") || strings.Contains(contentType, "gb2312") || strings.Contains(contentType, "gb18030") || strings.Contains(contentType, "windows-1252") {
				htmlEncode = "gb18030"
			} else if strings.Contains(contentType, "big5") {
				htmlEncode = "big5"
			} else if strings.Contains(contentType, "utf-8") {
				htmlEncode = "utf-8"
			}
		}
		if htmlEncode == "" {
			reg = regexp.MustCompile(`(?is)<title[^>]*>(.*?)<\/title>`)
			match = reg.FindStringSubmatch(body)
			if len(match) > 1 {
				aa := match[1]
				_, contentType, _ = charset.DetermineEncoding([]byte(aa), "")
				log.Println(contentType)
				htmlEncode = strings.ToLower(htmlEncode)
				if strings.Contains(contentType, "gbk") || strings.Contains(contentType, "gb2312") || strings.Contains(contentType, "gb18030") || strings.Contains(contentType, "windows-1252") {
					htmlEncode = "gb18030"
				} else if strings.Contains(contentType, "big5") {
					htmlEncode = "big5"
				} else if strings.Contains(contentType, "utf-8") {
					htmlEncode = "utf-8"
				}
			}
		}
	}
	if htmlEncode != "" && htmlEncode != "utf-8" {
		body = ConvertToString(body, htmlEncode, "utf-8")
	}

	requestData := RequestData{
		Body:   body,
		Domain: resp.Request.Host,
		Scheme: resp.Request.URL.Scheme,
		Server: resp.Header.Get("Server"),
	}

	return &requestData, nil
}

func ConvertToString(src string, srcCode string, tagCode string) string {
	srcCoder := mahonia.NewDecoder(srcCode)
	srcResult := srcCoder.ConvertString(src)
	tagCoder := mahonia.NewDecoder(tagCode)
	_, cdata, _ := tagCoder.Translate([]byte(srcResult), true)
	result := string(cdata)
	return result
}
