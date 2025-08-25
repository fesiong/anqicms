package provider

import (
	"github.com/PuerkitoBio/goquery"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/response"
	"log"
	"strings"
	"testing"
)

func (w *Website) TestCollectSingleArticle(t *testing.T) {
	link := &response.WebLink{Url: "http://blog.niunan.net/blog/show/1295"}
	keyword := &model.Keyword{Title: "PHP 报错"}
	result, err := w.CollectSingleArticle(link, keyword, 0)

	if err != nil {
		t.Fatal(err)
	}

	log.Printf("%#v", result.Content)
}

func TestCollectArticlesByKeyword(t *testing.T) {
	keyword := model.Keyword{
		Title:  "golang面试题",
		Status: 1,
	}
	GetDefaultDB()
	dbSite, _ := GetDBWebsiteInfo(1)
	InitWebsite(dbSite)
	w := CurrentSite(nil)
	num, err := w.CollectArticlesByKeyword(keyword, true)
	if err != nil {
		t.Fatal(err)
	}

	log.Println(num)
}

func (w *Website) TestGoquery(t *testing.T) {
	str := "<p>来张图中吧：</p>\n<p><img data-original=\"https://cdn.jiler.cn/techug/uploads/2017/03/420532-20170305205228282-609193437-1000x519.png\" title=\"图0：2017年的golang、python、php、c++、c、java、Nodejs性能对比\" alt=\"图0：2017年的golang、python、php、c++、c、java、Nodejs性能对比\"/></p>\n<p>总结：</p>"

	htmlR := strings.NewReader(str)
	doc, err := goquery.NewDocumentFromReader(htmlR)

	if err != nil {
		t.Fatal(err)
	}

	doc.Find("img").Each(func(i int, item *goquery.Selection) {
		src, _ := item.Attr("src")
		dataSrc, exists2 := item.Attr("data-src")
		if exists2 {
			src = dataSrc
		}
		dataSrc, exists2 = item.Attr("data-original")
		if exists2 {
			src = dataSrc
		}
		log.Println(src, dataSrc)
		log.Println(item.Parent().Html())
	})
}
