package library

import (
	"fmt"
	"log"
	"testing"
)

func TestRequest(t *testing.T) {
	link := "https://baijiahao.baidu.com/s?id=1672606603627218710&wfr=spider&for=pc"
	resp, err := Request(link, &Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			//"Referer": fmt.Sprintf("https://www.baidu.com/s?wd=%s", "SEO学习教程"),
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"Accept-Encoding": "gzip, deflate, br",
			"Accept-Language": "zh-CN,zh;q=0.9",
			"Cache-Control":   "no-cache",
			"Pragma":          "no-cache",
			"User-Agent":      "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		},
	})

	log.Println(err)

	log.Println(resp)
}

func TestGetURLData(t *testing.T) {
	resp, err := GetURLData("https://baijiahao.baidu.com/s?id=1672606603627218710&wfr=spider&for=pc", fmt.Sprintf("https://www.baidu.com/s?wd=%s", "SEO学习教程"), 10)
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("%#v", resp.Request)
	log.Printf("%#v", resp.Body)
}
