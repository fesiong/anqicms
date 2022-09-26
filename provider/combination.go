package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"io/ioutil"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
)

func GetUserCombinationSetting() config.CombinationJson {
	var combination config.CombinationJson
	buf, err := ioutil.ReadFile(fmt.Sprintf("%scombination.json", config.ExecPath))
	configStr := ""
	if err != nil {
		//文件不存在
		return combination
	}
	configStr = string(buf[:])
	reg := regexp.MustCompile(`/\*.*\*/`)

	configStr = reg.ReplaceAllString(configStr, "")
	buf = []byte(configStr)

	if err = json.Unmarshal(buf, &combination); err != nil {
		return combination
	}

	return combination
}

func SaveUserCombinationSetting(req config.CombinationJson, focus bool) error {
	combination := GetUserCombinationSetting()
	if focus {
		combination = req
	} else {
		if req.ContentExclude != nil {
			combination.ContentExclude = req.ContentExclude
		}
		if req.ContentReplace != nil {
			combination.ContentReplace = req.ContentReplace
		}
		if req.AutoDigKeyword {
			combination.AutoDigKeyword = req.AutoDigKeyword
		}
		if req.CategoryId > 0 {
			combination.CategoryId = req.CategoryId
		}
		if req.StartHour > 0 {
			combination.StartHour = req.StartHour
		}
		if req.EndHour > 0 {
			combination.EndHour = req.EndHour
		}
		if req.DailyLimit > 0 {
			combination.DailyLimit = req.DailyLimit
		}
	}

	//将现有配置写回文件
	configFile, err := os.OpenFile(fmt.Sprintf("%scombination.json", config.ExecPath), os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}

	defer configFile.Close()

	buff := &bytes.Buffer{}

	buf, err := json.MarshalIndent(combination, "", "\t")
	if err != nil {
		return err
	}
	buff.Write(buf)

	_, err = io.Copy(configFile, buff)
	if err != nil {
		return err
	}

	//重新读取配置
	config.LoadCombinationConfig()

	return nil
}

func getCombinationEnginLink(keyword *model.Keyword) string {
	// default bing
	var link string
	switch config.KeywordConfig.FromEngine {
	case config.Engin360:
		link = fmt.Sprintf("https://www.so.com/s?ie=utf-8&q=%s", url.QueryEscape(keyword.Title))
		break
	case config.EnginSogou:
		link = fmt.Sprintf("http://sogou.com/web?query=%s", url.QueryEscape(keyword.Title))
		break
	case config.EnginGoogle:
		link = fmt.Sprintf("https://www.google.com/search?q=%s&sourceid=chrome&ie=UTF-8", url.QueryEscape(keyword.Title))
		break
	case config.EnginBaidu:
		link = fmt.Sprintf("https://www.baidu.com/s?wd=%s", url.QueryEscape(keyword.Title))
		break
	case config.EnginBing:
		link = fmt.Sprintf("https://cn.bing.com/search?q=%s&ensearch=1", url.QueryEscape(keyword.Title))
		break
	case config.EnginOther:
		if strings.Contains(config.KeywordConfig.FromWebsite, "%s") {
			link = fmt.Sprintf(config.KeywordConfig.FromWebsite, url.QueryEscape(keyword.Title))
			break
		}
	case config.EnginBingCn:
	default:
		link = fmt.Sprintf("https://cn.bing.com/search?q=%s", url.QueryEscape(keyword.Title))
		break
	}

	return link
}

func collectCombinationMaterials(keyword *model.Keyword) error {
	link := getCombinationEnginLink(keyword)
	resp, err := library.Request(link, &library.Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			"Referer":         link,
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		},
	})
	if err != nil {
		return err
	}

	result := parseSections(resp.Body, keyword.Title, link)

	log.Println(result)
	// 尝试读取前2页

	return nil
}

func parseSections(content, word, sourceLink string) error {
	htmlR := strings.NewReader(content)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err != nil {
		return err
	}

	doc.Find("script,style,head").Remove()
	doc.Find("span,em,p").Unwrap()

	parsedUrl, err := url.Parse(sourceLink)
	if err != nil {
		return err
	}
	parsedUrl.Path = "/"
	parsedUrl.RawQuery = ""
	parsedUrl.Fragment = ""
	baseUrl := parsedUrl.String()

	doc.Find("a").Each(func(i int, item *goquery.Selection) {
		title := strings.TrimSpace(item.Text())
		if ContainKeywords(title, word) {
			href, ok := item.Attr("href")
			if !ok {
				return
			}
			if !strings.HasPrefix(href, "http") {
				href = baseUrl + strings.TrimLeft(href, "/")
			}
			log.Println(title)
			log.Println(href)
		}
	})

	return nil
}
