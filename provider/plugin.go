package provider

import (
	"errors"
	"fmt"
	"github.com/parnurzeal/gorequest"
	"io/ioutil"
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

//单个sitemap文件可包含的连接数
const SitemapLimit = 50000

type bingData struct {
	SiteUrl string `json:"siteUrl"`
	UrlList []string `json:"urlList"`
}

func PushArticle(link string) {
	_ = PushBaidu([]string{link})
	_ = PushBing([]string{link})
}

func PushProduct(link string) {
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

	library.DebugLog("push-baidu", string(body))
	return nil
}

func PushBing(list []string) error {
	bingApi := config.JsonData.PluginPush.BingApi
	if bingApi == "" {
		return errors.New("没有配置必应主动推送")
	}
	postData := bingData{
		SiteUrl:  config.JsonData.System.BaseUrl,
		UrlList: list,
	}

	_, body, errs := gorequest.New().Timeout(10 * time.Second).Set("Content-Type", "application/json; charset=utf-8​").Post(bingApi).Send(postData).End()
	if errs != nil {
		fmt.Println(errs)
		return errs[0]
	}

	library.DebugLog("push-bing", body)
	return nil
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

//手动生成sitemap
func BuildSitemap() error {
	//每一个sitemap包含50000条记录
	//当所有数量少于50000的时候，生成到sitemap.txt文件中
	//如果所有数量多于50000，则按种类生成。
	//sitemap将包含首页、分类首页、文章页、产品页
	basePath := fmt.Sprintf("%spublic/", config.ExecPath)
	baseUrl := config.JsonData.System.BaseUrl
	totalCount := int64(1)
	var categoryCount int64
	var articleCount int64
	var productCount int64
	categoryBuilder := config.DB.Model(&model.Category{}).Where("`status` = 1").Order("id asc").Count(&categoryCount)
	articleBuilder := config.DB.Model(&model.Article{}).Where("`status` = 1").Order("id asc").Count(&articleCount)
	productBuilder := config.DB.Model(&model.Product{}).Where("`status` = 1").Order("id asc").Count(&productCount)
	totalCount += categoryCount + articleCount + productCount
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
			categoryFile.WriteString(GetUrl("category", v, 0))
		}
		//写入文章
		pager := int(math.Ceil(float64(articleCount) / float64(SitemapLimit)))
		var articles []*model.Article
		for i := 1; i <= pager; i++ {
			//写入index
			indexFile.WriteString(fmt.Sprintf("%s/article-%d.txt\n", baseUrl, i))

			//写入article-sitemap
			articleFile, err := os.OpenFile(fmt.Sprintf("%sarticle-%d.txt", basePath, i), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				//无法创建
				return err
			}

			err = articleBuilder.Limit(SitemapLimit).Offset((i - 1) * SitemapLimit).Find(&articles).Error
			if err == nil {
				for _, v := range articles {
					articleFile.WriteString(GetUrl("article", v, 0))
				}
			}
			articleFile.Close()
		}
		//写入产品
		pager = int(math.Ceil(float64(productCount) / float64(SitemapLimit)))
		var products []*model.Product
		for i := 1; i <= pager; i++ {
			//写入index
			indexFile.WriteString(fmt.Sprintf("%s/product-%d.txt\n", baseUrl, i))

			//写入product-sitemap
			productFile, err := os.OpenFile(fmt.Sprintf("%sproduct-%d.txt", basePath, i), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				//无法创建
				return err
			}

			err = productBuilder.Limit(SitemapLimit).Offset((i - 1) * SitemapLimit).Find(&products).Error
			if err == nil {
				for _, v := range products {
					productFile.WriteString(GetUrl("product", v, 0))
				}
			}
			productFile.Close()
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
			sitemapFile.WriteString(GetUrl("category", v, 0))
		}
		//写入文章页
		var articles []*model.Article
		articleBuilder.Find(&articles)
		for _, v := range articles {
			sitemapFile.WriteString(GetUrl("article", v, 0))
		}
		//写入产品页
		var products []*model.Product
		productBuilder.Find(&products)
		for _, v := range products {
			sitemapFile.WriteString(GetUrl("product", v, 0))
		}
	}

	_ = UpdateSitemapTime()

	return nil
}

//追加sitemap
func AddonSitemap(itemType string, link string) error {
	basePath := fmt.Sprintf("%spublic/", config.ExecPath)
	totalCount := int64(1)
	var categoryCount int64
	var articleCount int64
	config.DB.Model(&model.Category{}).Where("`status` = 1").Count(&categoryCount)
	config.DB.Model(&model.Article{}).Where("`status` = 1").Count(&articleCount)
	totalCount += categoryCount + articleCount

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
			_, err = categoryFile.WriteString(link)

			if err == nil {
				_ = UpdateSitemapTime()
			}
		} else if itemType == "article" {
			//文章，由于本次统计的时候，这个文章已经存在，可以直接使用统计数量
			pager := int(math.Ceil(float64(articleCount) / float64(SitemapLimit)))
			articlePath := fmt.Sprintf("%sarticle-%d.txt", basePath, pager)
			_, err := os.Stat(articlePath)
			if err != nil {
				if os.IsNotExist(err) {
					return BuildSitemap()
				} else {
					return err
				}
			}
			articleFile, err := os.OpenFile(articlePath, os.O_RDWR|os.O_APPEND, 0666)
			if err != nil {
				return err
			}
			defer articleFile.Close()
			_, err = articleFile.WriteString(link)

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
			_, err = sitemapFile.WriteString(link)
		} else if itemType == "article" {
			_, err = sitemapFile.WriteString(link)
		}

		if err == nil {
			_ = UpdateSitemapTime()
		}
	}

	return nil
}