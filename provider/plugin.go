package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/parnurzeal/gorequest"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/response"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// SitemapLimit 单个sitemap文件可包含的连接数
const SitemapLimit = 50000
const PushLogFile = "push.log"

type bingData struct {
	SiteUrl string   `json:"siteUrl"`
	UrlList []string `json:"urlList"`
}

type bingData2 struct {
	Host        string   `json:"host"`
	Key         string   `json:"key"`
	KeyLocation string   `json:"keyLocation"`
	UrlList     []string `json:"urlList"`
}

func (w *Website) PushArchive(link string) {
	if len(link) == 0 {
		return
	}
	_ = w.PushBaidu([]string{link})
	_ = w.PushBing([]string{link})
	if config.GoogleValid {
		_ = w.PushGoogle([]string{link})
	}
}

func (w *Website) PushBaidu(list []string) error {
	baiduApi := w.PluginPush.BaiduApi
	if baiduApi == "" {
		return errors.New(w.Tr("BaiduActivePushIsNotConfigured"))
	}
	urlString := strings.Replace(strings.Trim(fmt.Sprint(list), "[]"), " ", "\n", -1)

	resp, err := http.Post(baiduApi, "text/plain", strings.NewReader(urlString))
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	w.logPushResult("baidu", fmt.Sprintf("%v, %s", list, string(body)))
	return nil
}

func (w *Website) PushBing(list []string) error {
	bingApi := w.PluginPush.BingApi
	if bingApi == "" {
		return errors.New(w.Tr("BingActivePushIsNotConfigured"))
	}

	// bing 推送有2种方式，一种是传统的api，另一种是 IndexNow
	if strings.HasPrefix(bingApi, "https://www.bing.com/indexnow") {
		baseUrl, err := url.Parse(w.System.BaseUrl)
		if err != nil {
			return err
		}
		// IndexNow
		// 验证以下是否存在txt
		parsedUrl, err := url.Parse(bingApi)
		if err != nil {
			return err
		}
		apiKey := parsedUrl.Query().Get("key")

		txtFile := w.PublicPath + apiKey + ".txt"
		_, err = os.Stat(txtFile)
		if err != nil && os.IsNotExist(err) {
			// 生成一个
			_ = os.WriteFile(txtFile, []byte(apiKey), os.ModePerm)
		}
		// 开始推送
		postData := bingData2{
			Host:        baseUrl.Host,
			Key:         apiKey,
			KeyLocation: w.System.BaseUrl + "/" + apiKey + ".txt",
			UrlList:     list,
		}
		resp, body, errs := gorequest.New().Timeout(10*time.Second).Set("Content-Type", "application/json; charset=utf-8").Post(bingApi).Send(postData).End()
		if errs != nil {
			fmt.Println(errs)
			return errs[0]
		}
		if resp.StatusCode == 200 {
			body = "URL submitted successfully"
		}
		w.logPushResult("bing", fmt.Sprintf("%v, %s", list, body))
	} else {
		postData := bingData{
			SiteUrl: w.System.BaseUrl,
			UrlList: list,
		}

		_, body, errs := gorequest.New().Timeout(10*time.Second).Set("Content-Type", "application/json; charset=utf-8").Post(bingApi).Send(postData).End()
		if errs != nil {
			fmt.Println(errs)
			return errs[0]
		}
		w.logPushResult("bing", fmt.Sprintf("%v, %s", list, body))
	}

	return nil
}

func (w *Website) logPushResult(spider string, result string) {
	pushLog := response.PushLog{
		CreatedTime: time.Now().Unix(),
		Result:      result,
		Spider:      spider,
	}

	content, err := json.Marshal(pushLog)

	if err == nil {
		library.DebugLog(w.CachePath, PushLogFile, string(content))
	}
}

func (w *Website) GetLastPushList() ([]response.PushLog, error) {
	var pushLogs []response.PushLog
	//获取20条数据
	filePath := w.CachePath + PushLogFile
	logFile, err := os.Open(filePath)
	if nil != err {
		//打开失败
		return pushLogs, nil
	}
	defer logFile.Close()

	line := int64(1)
	cursor := int64(0)
	stat, err := logFile.Stat()
	fileSize := stat.Size()
	tmp := ""
	for {
		cursor -= 1
		logFile.Seek(cursor, io.SeekEnd)

		char := make([]byte, 1)
		logFile.Read(char)

		if cursor != -1 && (char[0] == 10 || char[0] == 13) {
			//跳到一个新行，清空
			line++
			//解析
			if tmp != "" {
				var pushLog response.PushLog
				err := json.Unmarshal([]byte(tmp), &pushLog)
				if err == nil {
					pushLogs = append(pushLogs, pushLog)
				}
			}
			tmp = ""
		}

		tmp = fmt.Sprintf("%s%s", string(char), tmp)

		if cursor == -fileSize {
			// stop if we are at the beginning
			break
		}
		if line == 100 {
			break
		}
	}
	//解析最后一条
	if tmp != "" {
		var pushLog response.PushLog
		err := json.Unmarshal([]byte(tmp), &pushLog)
		if err == nil {
			pushLogs = append(pushLogs, pushLog)
		}
	}

	return pushLogs, nil
}

func (w *Website) GetRobots() string {
	//robots 是一个文件，所以直接读取文件
	robotsPath := w.PublicPath + "robots.txt"
	robots, err := os.ReadFile(robotsPath)
	if err != nil {
		//文件不存在
		return ""
	}

	return string(robots)
}

func (w *Website) SaveRobots(robots string) error {
	robotsPath := w.PublicPath + "robots.txt"

	robotsFile, err := os.OpenFile(robotsPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}

	defer robotsFile.Close()

	_, err = robotsFile.WriteString(robots)
	if err != nil {
		return err
	}
	// 上传到静态服务器
	_ = w.SyncHtmlCacheToStorage(robotsPath, "robots.txt")

	return nil
}
