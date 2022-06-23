package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/parnurzeal/gorequest"
	"io"
	"io/ioutil"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// SitemapLimit 单个sitemap文件可包含的连接数
const SitemapLimit = 50000
const PushLogFile = "push"

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

func PushArchive(link string) {
	_ = PushBaidu([]string{link})
	_ = PushBing([]string{link})
}

func PushBaidu(list []string) error {
	baiduApi := config.JsonData.PluginPush.BaiduApi
	if baiduApi == "" {
		return errors.New("没有配置百度主动推送")
	}
	urlString := strings.Replace(strings.Trim(fmt.Sprint(list), "[]"), " ", "\n", -1)

	resp, err := http.Post(baiduApi, "text/plain", strings.NewReader(urlString))
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	logPushResult("baidu", fmt.Sprintf("%v, %s", list, string(body)))
	return nil
}

func PushBing(list []string) error {
	bingApi := config.JsonData.PluginPush.BingApi
	if bingApi == "" {
		return errors.New("没有配置必应主动推送")
	}

	// bing 推送有2种方式，一种是传统的api，另一种是 IndexNow
	if strings.HasPrefix(bingApi, "https://www.bing.com/indexnow") {
		baseUrl, err := url.Parse(config.JsonData.System.BaseUrl)
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

		txtFile := config.ExecPath + "public/" + apiKey + ".txt"
		_, err = os.Stat(txtFile)
		if err != nil && os.IsNotExist(err) {
			// 生成一个
			_ = ioutil.WriteFile(txtFile, []byte(apiKey), os.ModePerm)
		}
		// 开始推送
		postData := bingData2{
			Host: baseUrl.Host,
			Key: apiKey,
			KeyLocation: config.JsonData.System.BaseUrl + "/" + apiKey + ".txt",
			UrlList: list,
		}
		resp, body, errs := gorequest.New().Timeout(10*time.Second).Set("Content-Type", "application/json; charset=utf-8").Post(bingApi).Send(postData).End()
		if errs != nil {
			fmt.Println(errs)
			return errs[0]
		}
		if resp.StatusCode == 200 {
			body = "URL submitted successfully"
		}
		logPushResult("bing", fmt.Sprintf("%v, %s", list, body))
	} else {
		postData := bingData{
			SiteUrl: config.JsonData.System.BaseUrl,
			UrlList: list,
		}

		_, body, errs := gorequest.New().Timeout(10*time.Second).Set("Content-Type", "application/json; charset=utf-8").Post(bingApi).Send(postData).End()
		if errs != nil {
			fmt.Println(errs)
			return errs[0]
		}
		logPushResult("bing", fmt.Sprintf("%v, %s", list, body))
	}

	return nil
}

func logPushResult(spider string, result string) {
	pushLog := response.PushLog{
		CreatedTime: time.Now().Unix(),
		Result:      result,
		Spider:      spider,
	}

	content, err := json.Marshal(pushLog)

	if err == nil {
		library.DebugLog(PushLogFile, string(content))
	}
}

func GetLastPushList() ([]response.PushLog, error) {
	var pushLogs []response.PushLog
	//获取20条数据
	filePath := fmt.Sprintf("%scache/%s.log", config.ExecPath, PushLogFile)
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

func GetRobots() string {
	//robots 是一个文件，所以直接读取文件
	robotsPath := fmt.Sprintf("%spublic/robots.txt", config.ExecPath)
	robots, err := ioutil.ReadFile(robotsPath)
	if err != nil {
		//文件不存在
		return ""
	}

	return string(robots)
}

func SaveRobots(robots string) error {
	robotsPath := fmt.Sprintf("%spublic/robots.txt", config.ExecPath)

	robotsFile, err := os.OpenFile(robotsPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}

	defer robotsFile.Close()

	_, err = robotsFile.WriteString(robots)
	if err != nil {
		return err
	}

	return nil
}

func UpdateSitemapTime() error {
	path := fmt.Sprintf("%scache/sitemap-time.log", config.ExecPath)

	nowTime := fmt.Sprintf("%d", time.Now().Unix())
	err := ioutil.WriteFile(path, []byte(nowTime), 0666)

	if err != nil {
		return err
	}

	return nil
}

func GetSitemapTime() int64 {
	path := fmt.Sprintf("%scache/sitemap-time.log", config.ExecPath)
	timeBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return 0
	}

	timeInt, err := strconv.Atoi(string(timeBytes))
	if err != nil {
		return 0
	}

	return int64(timeInt)
}

// BuildSitemap 手动生成sitemap
func BuildSitemap() error {
	//每一个sitemap包含50000条记录
	//当所有数量少于50000的时候，生成到sitemap.txt文件中
	//如果所有数量多于50000，则按种类生成。
	//sitemap将包含首页、分类首页、文章页、产品页
	basePath := fmt.Sprintf("%spublic/", config.ExecPath)
	baseUrl := config.JsonData.System.BaseUrl
	totalCount := int64(1)
	var categoryCount int64
	var archiveCount int64
	var tagCount int64
	categoryBuilder := dao.DB.Model(&model.Category{}).Where("`status` = 1").Order("id asc").Count(&categoryCount)
	archiveBuilder := dao.DB.Model(&model.Archive{}).Where("`status` = 1").Order("id asc").Count(&archiveCount)
	tagBuilder := dao.DB.Model(&model.Tag{}).Where("`status` = 1").Order("id asc").Count(&tagCount)
	totalCount += categoryCount + archiveCount + tagCount
	if totalCount > SitemapLimit {
		//开展分页模式
		//index 和 category 存放在同一个文件，文章单独一个文件
		indexFile, err := os.OpenFile(fmt.Sprintf("%ssitemap.txt", basePath), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			//无法创建
			return err
		}
		defer indexFile.Close()
		indexFile.WriteString(fmt.Sprintf("%s/category.txt\n", baseUrl))

		categoryFile, err := os.OpenFile(fmt.Sprintf("%scategory.txt", basePath), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			//无法创建
			return err
		}
		defer categoryFile.Close()
		//写入首页
		categoryFile.WriteString(baseUrl + "\n")
		//写入分类页
		var categories []*model.Category
		categoryBuilder.Find(&categories)
		for _, v := range categories {
			categoryFile.WriteString(GetUrl("category", v, 0) + "\n")
		}
		//写入文章
		pager := int(math.Ceil(float64(archiveCount) / float64(SitemapLimit)))
		var archives []*model.Archive
		for i := 1; i <= pager; i++ {
			//写入index
			indexFile.WriteString(fmt.Sprintf("%s/archive-%d.txt\n", baseUrl, i))

			//写入archive-sitemap
			archiveFile, err := os.OpenFile(fmt.Sprintf("%sarchive-%d.txt", basePath, i), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				//无法创建
				return err
			}

			err = archiveBuilder.Limit(SitemapLimit).Offset((i - 1) * SitemapLimit).Find(&archives).Error
			if err == nil {
				for _, v := range archives {
					archiveFile.WriteString(GetUrl("archive", v, 0) + "\n")
				}
			}
			archiveFile.Close()
		}
		//写入tag
		pager = int(math.Ceil(float64(tagCount) / float64(SitemapLimit)))
		var tags []*model.Tag
		for i := 1; i <= pager; i++ {
			//写入index
			indexFile.WriteString(fmt.Sprintf("%s/tag-%d.txt\n", baseUrl, i))

			//写入tag-sitemap
			tagFile, err := os.OpenFile(fmt.Sprintf("%stag-%d.txt", basePath, i), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				//无法创建
				return err
			}

			err = tagBuilder.Limit(SitemapLimit).Offset((i - 1) * SitemapLimit).Find(&tags).Error
			if err == nil {
				for _, v := range tags {
					tagFile.WriteString(GetUrl("tag", v, 0) + "\n")
				}
			}
			tagFile.Close()
		}
	} else {
		//单一文件模式
		sitemapFile, err := os.OpenFile(fmt.Sprintf("%ssitemap.txt", basePath), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			//无法创建
			return err
		}
		defer sitemapFile.Close()

		//写入首页
		sitemapFile.WriteString(baseUrl + "\n")

		//写入分类页
		var categories []*model.Category
		categoryBuilder.Find(&categories)
		for _, v := range categories {
			sitemapFile.WriteString(GetUrl("category", v, 0) + "\n")
		}
		//写入文章页
		var archives []*model.Archive
		archiveBuilder.Find(&archives)
		for _, v := range archives {
			sitemapFile.WriteString(GetUrl("archive", v, 0) + "\n")
		}
		//写入tag
		var tags []*model.Tag
		tagBuilder.Find(&tags)
		for _, v := range tags {
			sitemapFile.WriteString(GetUrl("tag", v, 0) + "\n")
		}
	}

	_ = UpdateSitemapTime()

	return nil
}

// AddonSitemap 追加sitemap
func AddonSitemap(itemType string, link string) error {
	basePath := fmt.Sprintf("%spublic/", config.ExecPath)
	totalCount := int64(1)
	var categoryCount int64
	var archiveCount int64
	var tagCount int64
	dao.DB.Model(&model.Category{}).Where("`status` = 1").Count(&categoryCount)
	dao.DB.Model(&model.Archive{}).Where("`status` = 1").Count(&archiveCount)
	dao.DB.Model(&model.Tag{}).Where("`status` = 1").Count(&tagCount)
	totalCount += categoryCount + archiveCount

	if totalCount > SitemapLimit {
		//开展分页模式
		//index 和 category 存放在同一个文件，文章单独一个文件
		if itemType == "category" {
			categoryPath := fmt.Sprintf("%scategory.txt", basePath)
			_, err := os.Stat(categoryPath)
			if err != nil {
				if os.IsNotExist(err) {
					return BuildSitemap()
				} else {
					return err
				}
			}

			categoryFile, err := os.OpenFile(categoryPath, os.O_RDWR|os.O_APPEND, 0666)
			if err != nil {
				return err
			}
			defer categoryFile.Close()
			//写入分类页
			_, err = categoryFile.WriteString(link + "\n")

			if err == nil {
				_ = UpdateSitemapTime()
			}
		} else if itemType == "archive" {
			//文章，由于本次统计的时候，这个文章已经存在，可以直接使用统计数量
			pager := int(math.Ceil(float64(archiveCount) / float64(SitemapLimit)))
			archivePath := fmt.Sprintf("%sarchive-%d.txt", basePath, pager)
			_, err := os.Stat(archivePath)
			if err != nil {
				if os.IsNotExist(err) {
					return BuildSitemap()
				} else {
					return err
				}
			}
			archiveFile, err := os.OpenFile(archivePath, os.O_RDWR|os.O_APPEND, 0666)
			if err != nil {
				return err
			}
			defer archiveFile.Close()
			_, err = archiveFile.WriteString(link + "\n")

			if err == nil {
				_ = UpdateSitemapTime()
			}
		} else if itemType == "tag" {
			//tag
			pager := int(math.Ceil(float64(tagCount) / float64(SitemapLimit)))
			tagPath := fmt.Sprintf("%stag-%d.txt", basePath, pager)
			_, err := os.Stat(tagPath)
			if err != nil {
				if os.IsNotExist(err) {
					return BuildSitemap()
				} else {
					return err
				}
			}
			tagFile, err := os.OpenFile(tagPath, os.O_RDWR|os.O_APPEND, 0666)
			if err != nil {
				return err
			}
			defer tagFile.Close()
			_, err = tagFile.WriteString(link + "\n")

			if err == nil {
				_ = UpdateSitemapTime()
			}
		}
	} else {
		sitemapPath := fmt.Sprintf("%ssitemap.txt", basePath)
		//单一文件模式
		//文件不存在，则全量生成
		//否则直接追加
		_, err := os.Stat(sitemapPath)
		if err != nil {
			if os.IsNotExist(err) {
				return BuildSitemap()
			} else {
				return err
			}
		}

		sitemapFile, err := os.OpenFile(sitemapPath, os.O_RDWR|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		defer sitemapFile.Close()
		//开始追加写入
		if itemType == "category" {
			_, err = sitemapFile.WriteString(link + "\n")
		} else if itemType == "archive" {
			_, err = sitemapFile.WriteString(link + "\n")
		} else if itemType == "tag" {
			_, err = sitemapFile.WriteString(link + "\n")
		}

		if err == nil {
			_ = UpdateSitemapTime()
		}
	}

	return nil
}
