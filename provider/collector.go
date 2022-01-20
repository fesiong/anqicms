package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"irisweb/config"
	"irisweb/library"
	"irisweb/model"
	"irisweb/request"
	"irisweb/response"
	"log"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var emptyLinkPatternStr = `(^data:)|(^tel:)|(^mailto:)|(about:blank)|(javascript:)`
var emptyLinkPattern = regexp.MustCompile(emptyLinkPatternStr)

//始终保持只有一个keyword任务
var digKeywordRunning = false

var maxWordsNum = int64(100000)

// StartDigKeywords 开始挖掘关键词，通过核心词来拓展
// 最多只10万关键词，抓取前3级，如果超过3级，则每次只执行一级
func StartDigKeywords() {
	if config.CollectorConfig.AutoDigKeyword == false {
		return
	}
	if digKeywordRunning {
		return
	}
	digKeywordRunning = true
	defer func() {
		digKeywordRunning = false
	}()
	collectedWords := &sync.Map{}

	//非严格的限制数量
	var maxNum int64
	config.DB.Model(&model.Keyword{}).Count(&maxNum)
	if maxNum >= maxWordsNum {
		return
	}

	var keywords []*model.Keyword
	config.DB.Where("has_dig = 0").Order("id asc").Limit(100).Find(&keywords)
	if len(keywords) == 0 {
		return
	}

	for _, keyword := range keywords {
		//下一级的
		err := collectSuggestBaiduWord(collectedWords, keyword)
		if err != nil {
			break
		}
		keyword.HasDig = 1
		config.DB.Model(keyword).UpdateColumn("has_dig", keyword.HasDig)
		//不能太快，每次休息随机1-5秒钟
		time.Sleep(time.Duration(1 + rand.Intn(5)) * time.Second)
	}
	//重新计数
	config.DB.Model(&model.Keyword{}).Count(&maxNum)
}

type BaiduSuggest struct {
	G []BaiduSuggestItem `json:"g"`
}

type BaiduSuggestItem struct {
	Type string `json:"type"`
	Q string `json:"q"`
}

func collectSuggestBaiduWord(existsWords *sync.Map, keyword *model.Keyword) error {
	//执行一次，2层
	link := fmt.Sprintf("http://www.baidu.com/sugrec?prod=pc&wd=%s", url.QueryEscape(keyword.Title))
	resp, err := library.Request(link, &library.Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			"Host":            "www.baidu.com",
			"Referer":         "https://www.baidu.com",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		},
	})

	if err != nil {
		return err
	}

	var suggest BaiduSuggest
	err = json.Unmarshal([]byte(resp.Body), &suggest)
	if err != nil {
		return err
	}

	for _, sug := range suggest.G {
		if _, ok := existsWords.Load(sug.Q); ok {
			continue
		}
		existsWords.Store(sug.Q, keyword.Level + 1)
		word := &model.Keyword{
			Title:   sug.Q,
			CategoryId: keyword.CategoryId,
			Level:  keyword.Level + 1,
		}
		config.DB.Model(&model.Keyword{}).Where("title = ?", sug.Q).FirstOrCreate(&word)
	}

	return nil
}

// getBetweenSeconds 根据发布量获取每天发布间隔
// 根据设置的时间，随机休息秒数
func getBetweenSeconds() int {
	limit := config.CollectorConfig.DailyLimit
	if limit == 0 {
		limit = 1000
	}
	if config.CollectorConfig.EndHour > 24 {
		config.CollectorConfig.EndHour = 24
	}
	hour := config.CollectorConfig.EndHour - config.CollectorConfig.StartHour
	if hour > 24 || hour <= 0 {
		hour = 24
	}
	seconds := hour * 3600

	between := seconds / limit - 1
	if config.CollectorConfig.AutoPseudo {
		between -= 1
	}

	//返回随机正负5秒的实际
	between += rand.Intn(11) - 5

	if between <= 0 {
		between = 1
	}

	return between
}

var runningCollectArticles = false

func CollectArticles() {
	if runningCollectArticles {
		return
	}
	runningCollectArticles = true
	defer func() {
		runningCollectArticles = false
	}()

	if config.CollectorConfig.StartHour > 0 && time.Now().Hour() < config.CollectorConfig.StartHour {
		return
	}

	if config.CollectorConfig.EndHour > 0 && time.Now().Hour() >= config.CollectorConfig.EndHour {
		return
	}

	lastId := uint(0)
	for {
		var keywords []*model.Keyword
		config.DB.Where("id > ? and last_time = 0", lastId).Order("id asc").Limit(100).Find(&keywords)
		if len(keywords) == 0 {
			break
		}
		lastId = keywords[len(keywords) - 1].Id
		for i := 0; i < len(keywords); i++ {
			keyword := keywords[i]
			err := CollectArticlesByKeyword(keyword)
			if err != nil {
				// 采集出错了，多半是出验证码了，跳过该任务，等下次开始
				break
			}
		}
	}
}

func CollectArticlesByKeyword(keyword *model.Keyword) error {
	var articles []*request.Article
	var err error
	articles, err = CollectArticleFromBaidu(keyword)

	if err != nil {
		return err
	}

	autoPseudo := false
	if config.CollectorConfig.AutoPseudo {
		autoPseudo = true
	}

	for _, article := range articles {
		//原始标题
		article.OriginTitle = article.Title
		if checkArticleExists(article.OriginUrl, article.OriginTitle) {
			continue
		}
		article.KeywordId = keyword.Id
		article.CategoryId = keyword.CategoryId
		if article.CategoryId == 0 && config.CollectorConfig.CategoryId > 0 {
			article.CategoryId = config.CollectorConfig.CategoryId
		}
		modelArticle, err := SaveArticle(article)
		if err != nil {
			log.Println("保存文章出错：", article.Title, err.Error())
			continue
		}
		//如果自动伪原创
		if autoPseudo {
			go PseudoOriginalArticle(modelArticle)
		}
	}

	keyword.ArticleCount = GetArticleTotalByKeywordId(keyword.Id)
	keyword.LastTime = time.Now().Unix()
	config.DB.Model(keyword).Select("article_count", "last_time").Updates(keyword)

	return nil
}

func CollectArticleFromBaidu(keyword *model.Keyword) ([]*request.Article, error) {
	resp, err := library.Request(fmt.Sprintf("https://www.baidu.com/s?wd=%s&tn=json&rn=50&pn=0",keyword.Title), &library.Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			"Referer":         "https://www.baidu.com",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		},
	})
	if err != nil {
		return nil, err
	}

	var articles []*request.Article
	links := ParseBaiduJson(resp.Body)
	for _, link := range links {
		//需要过滤可能不是内容的链接，/ 结尾的全部抛弃
		//if strings.HasSuffix(link.Url, "/") {
		//	continue
		//}
		//对首页排除
		parsedUrl, err := url.Parse(link.Url)
		if err == nil {
			if parsedUrl.Path == "/" {
				continue
			}
		}
		//抛弃通用的可能是聚合页面的链接
		if strings.Contains(link.Url, "/web?") ||
			strings.Contains(link.Url, "/ws?") ||
			strings.Contains(link.Url, "/search?") ||
			strings.Contains(link.Url, "/aclick?") ||
			strings.Contains(link.Url, "/aclk?") ||
			strings.Contains(link.Url, "/wd?") {
			continue
		}
		//百度的不使用 chromedp
		resp, err := library.Request(link.Url, &library.Options{
			Timeout:  5,
			IsMobile: false,
			Header: map[string]string{
				"Referer": fmt.Sprintf("https://www.baidu.com/s?wd=%s",keyword.Title),
			},
		})
		if err != nil {
			continue
		}

		//根据设置的时间，随机休息秒数
		time.Sleep(time.Duration(getBetweenSeconds()))

		article := &request.Article{
			OriginUrl:  link.Url,
			ContentText: resp.Body,
		}
		_ = ParseArticleDetail(article)
		if len(article.Content) == 0 {
			log.Println("链接无文章", article.OriginUrl)
			continue
		}
		if article.Title == "" {
			log.Println("链接无文章", article.OriginUrl)
			continue
		}
		log.Println(article.Title, len(article.Content), article.OriginUrl)

		articles = append(articles, article)
	}

	return articles, nil
}

func ParseBaiduJson(content string) []*response.WebLink {
	var baiduJson response.BaiduJson
	err := json.Unmarshal([]byte(content), &baiduJson)
	if err != nil {
		return nil
	}

	var links []*response.WebLink
	for _, v := range baiduJson.Feed.Entry {
		links = append(links, &response.WebLink{
			Name: v.Title,
			Url:  v.Url,
		})
	}

	return links
}

func ParseArticleDetail(article *request.Article) error {
	//先删除一些不必要的标签
	re, _ := regexp.Compile("\\<style[\\S\\s]+?\\</style\\>")
	contentText := re.ReplaceAllString(article.ContentText, "")
	re, _ = regexp.Compile("\\<script[\\S\\s]+?\\</script\\>")
	contentText = re.ReplaceAllString(contentText, "")
	re, _ = regexp.Compile("\\<!--[\\S\\s]*?--\\>")
	contentText = re.ReplaceAllString(contentText, "")
	contentText = strings.ReplaceAll(contentText, string(int32(8205)), "")

	htmlR := strings.NewReader(contentText)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err != nil {
		return err
	}

	//如果是百度百科地址，单独处理
	if strings.Contains(article.OriginUrl, "baike.baidu.com") {
		ParseBaikeDetail(article, doc)
	} else if strings.Contains(article.OriginUrl, "mp.weixin.qq.com") {
		ParseWeixinDetail(article, doc)
	} else if strings.Contains(article.OriginUrl, "zhihu.com") {
		ParseZhihuDetail(article, doc)
	} else if strings.Contains(article.OriginUrl, "toutiao.com") {
		ParseToutiaoDetail(article, doc)
	} else {
		ParseNormalDetail(article, doc)
	}

	article.Content = ReplaceContentFromConfig(article.Content)

	return nil
}


func ParseBaikeDetail(article *request.Article, doc *goquery.Document) {
	//获取标题
	article.Title = strings.TrimSpace(doc.Find("h1").Text())

	doc.Find(".edit-icon").Remove()
	contentList := doc.Find(".para-title,.para")
	content := ""
	for i := range contentList.Nodes {
		content += "<p>" + contentList.Eq(i).Text() + "</p>"
	}

	article.Content = content
	//如果获取不到内容，则 fallback 到normal
	if article.Content == "" {
		ParseNormalDetail(article, doc)
	}
}

func ParseWeixinDetail(article *request.Article, doc *goquery.Document) {
	//获取标题
	article.Title = strings.TrimSpace(doc.Find("h1").Eq(0).Text())

	contentList := doc.Find("#js_content")

	content, _ := CleanTags(contentList)

	article.Content = content
	//如果获取不到内容，则 fallback 到normal
	if article.Content == "" {
		ParseNormalDetail(article, doc)
	}
}

func ParseZhihuDetail(article *request.Article, doc *goquery.Document) {
	//获取标题
	article.Title = strings.TrimSpace(doc.Find("h1").Eq(0).Text())

	contentList := doc.Find(".RichContent-inner .RichText,.Post-RichTextContainer .RichText").Eq(0)

	content, _ := CleanTags(contentList)

	article.Content = content
	//如果获取不到内容，则 fallback 到normal
	if article.Content == "" {
		ParseNormalDetail(article, doc)
	}
}

func ParseToutiaoDetail(article *request.Article, doc *goquery.Document) {
	//获取标题
	article.Title = strings.TrimSpace(doc.Find("h1").Eq(0).Text())

	contentList := doc.Find(".article-content article")

	content, _ := CleanTags(contentList)

	article.Content = content
	//如果获取不到内容，则 fallback 到normal
	if article.Content == "" {
		ParseNormalDetail(article, doc)
	}
}

func ParseNormalDetail(article *request.Article, doc *goquery.Document) {
	ParseLinking(doc, article.OriginUrl)

	title := ParseArticleTitle(doc)

	//根据标题判断是否是英文，如果是英文，则采用英文的计数
	isEnglish := false
	if title != "" {
		re, err := regexp.Compile("^[\u0000-\u007E]+$")
		if err != nil {
			log.Println("reg err", err.Error())
		} else {
			if re.MatchString(title) {
				isEnglish = true
			}
		}
		article.Title = title
	}

	//尝试获取正文内容
	var planText string
	article.Content, planText, _ = ParseArticleContent(doc.Find("body"), 0, isEnglish)
	log.Println(len(article.Content), len(planText))
}

func ParseArticleTitle(doc *goquery.Document) string {
	//尝试获取标题
	//先尝试获取h1标签
	title := ""
	h1s := doc.Find("h1")
	if h1s.Length() > 0 {
		for i := range h1s.Nodes {
			item := h1s.Eq(i)
			item.Children().Remove()
			text := strings.TrimSpace(item.Text())
			textLen := utf8.RuneCountInString(text)
			if textLen >= config.CollectorConfig.TitleMinLength && textLen > utf8.RuneCountInString(title) && !HasContain(text, config.CollectorConfig.TitleExclude) && !HasPrefix(text, config.CollectorConfig.TitleExcludePrefix) && !HasSuffix(text, config.CollectorConfig.TitleExcludeSuffix) {
				title = text
			}
		}
	}
	if title == "" {
		//获取 政府网站的 <meta name='ArticleTitle' content='西城法院出台案件在线办理操作指南'>
		text, exist := doc.Find("meta[name=ArticleTitle]").Attr("content")
		if exist {
			text = strings.TrimSpace(text)
			if utf8.RuneCountInString(text) >= config.CollectorConfig.TitleMinLength && !HasContain(text, config.CollectorConfig.TitleExclude) && !HasPrefix(text, config.CollectorConfig.TitleExcludePrefix) && !HasSuffix(text, config.CollectorConfig.TitleExcludeSuffix) {
				title = text
			}
		}
	}
	if title == "" {
		//获取title标签
		text := doc.Find("title").Text()
		text = strings.ReplaceAll(text, "_", "-")
		sepIndex := strings.Index(text, "-")
		if sepIndex > 0 {
			text = text[:sepIndex]
		}
		text = strings.TrimSpace(text)
		if utf8.RuneCountInString(text) >= config.CollectorConfig.TitleMinLength && !HasContain(text, config.CollectorConfig.TitleExclude) && !HasPrefix(text, config.CollectorConfig.TitleExcludePrefix) && !HasSuffix(text, config.CollectorConfig.TitleExcludeSuffix) {
			title = text
		}
	}

	if title == "" {
		//获取title标签
		//title = doc.Find("#title,.title,.bt,.articleTit").First().Text()
		h2s := doc.Find("#title,.title,.bt,.articleTit,.right-xl>p,.biaoti")
		if h2s.Length() > 0 {
			for i := range h2s.Nodes {
				item := h2s.Eq(i)
				item.Children().Remove()
				text := strings.TrimSpace(item.Text())
				textLen := utf8.RuneCountInString(item.Text())
				if textLen >= config.CollectorConfig.TitleMinLength && textLen > utf8.RuneCountInString(title) && !HasContain(text, config.CollectorConfig.TitleExclude) && !HasPrefix(text, config.CollectorConfig.TitleExcludePrefix) && !HasSuffix(text, config.CollectorConfig.TitleExcludeSuffix) {
					title = text
				}
			}
		}
	}
	if title == "" {
		//如果标题为空，那么尝试h2
		h2s := doc.Find("h2,.name")
		if h2s.Length() > 0 {
			for i := range h2s.Nodes {
				item := h2s.Eq(i)
				item.Children().Remove()
				text := strings.TrimSpace(item.Text())
				textLen := utf8.RuneCountInString(text)
				if textLen >= config.CollectorConfig.TitleMinLength && textLen > utf8.RuneCountInString(title) && !HasContain(text, config.CollectorConfig.TitleExclude) && !HasPrefix(text, config.CollectorConfig.TitleExcludePrefix) && !HasSuffix(text, config.CollectorConfig.TitleExcludeSuffix) {
					title = text
				}
			}
		}
	}

	title = strings.Replace(strings.Replace(strings.TrimSpace(title), "\t", "", -1), "\n", " ", -1)
	title = strings.Replace(title, "<br>", "", -1)
	title = strings.Replace(title, "<br/>", "", -1)
	//只要第一个
	if utf8.RuneCountInString(title) > 50 {
		//减少误伤
		title = strings.ReplaceAll(title, "、", "-")
	}
	title = strings.ReplaceAll(title, "_", "-")
	sepIndex := strings.Index(title, "-")
	if sepIndex > 0 {
		title = title[:sepIndex]
	}

	return title
}

func ParseArticleContent(nodeItem *goquery.Selection, deep int, isEnglish bool) (string, string, int) {
	content := ""
	contentText := ""

	maxDeep := deep
	children := nodeItem.Children()
	for i := range children.Nodes {
		item := children.Eq(i)
		tmpContent, tmpText, tmpDeep := ParseArticleContent(item, deep+1, isEnglish)
		if tmpDeep > maxDeep {
			maxDeep = tmpDeep
		}
		if tmpText != "" && len(tmpText) > len(contentText) {
			//表示有内容
			content = tmpContent
			contentText = tmpText
		}
	}

	if content != "" {
		if nodeItem.ChildrenFiltered("p").Length() == 0 {
			return content, contentText, maxDeep
		}
		//系数
		if float64(len(contentText))*1.5 > float64(len(nodeItem.Text())) {
			return content, contentText, maxDeep
		}
	}

	//深度大于10的，抛弃
	//if maxDeep > 10 {
	//	return content, contentText, maxDeep
	//}

	// 通过一级一级的往下查找
	aLinks := nodeItem.Find("a")
	aText := strings.TrimSpace(RemoveTags(aLinks.Text()))
	planText := strings.TrimSpace(RemoveTags(nodeItem.Text()))
	planLen := utf8.RuneCountInString(planText)
	if isEnglish {
		//英语使用空格来计算词数
		planLen = len(strings.Split(planText, " "))
	}
	if planLen < config.CollectorConfig.ContentMinLength {
		//小于指定次数的，直接抛弃了
		return content, contentText, maxDeep
	}
	if len(aText)*5 > len(planText) {
		//a标签过多，这个不是文章内容
		return content, contentText, maxDeep
	}

	otherItems := nodeItem.Find("input,textarea,select,radio,form,button,footer,.footer,noscript,meta")
	if otherItems.Length() > 0 {
		otherItems.Remove()
	}

	//超过10个链接的抛弃
	//if aLinks.Length() > 10 {
	//	return content, contentText, maxDeep
	//}

	content, planText = CleanTags(nodeItem)
	planLen = utf8.RuneCountInString(planText)
	if isEnglish {
		//英语使用空格来计算词数
		planLen = len(strings.Split(planText, " "))
	}
	if planLen < config.CollectorConfig.ContentMinLength {
		//小于指定字数的，直接抛弃了
		return "", contentText, maxDeep
	}

	return content, planText, maxDeep
}

func CleanTags(nodeItem *goquery.Selection) (string, string) {
	clonedItem := nodeItem.Clone()
	for {
		if clonedItem.Children().Length() == 1 && clonedItem.Children().Contents().Length() > 0 {
			clonedItem.Children().Contents().Unwrap()
		} else {
			break
		}
	}

	//降dom深度
	clonedItem.Children().Each(func(i int, item *goquery.Selection) {
		//只保留 img,code,blockquote,pre
		if item.Is("img") || item.Is("code") || item.Is("blockquote") || item.Is("pre") {
			return
		}
		if item.Find("blockquote").Length() > 0 {
			item.ReplaceWithSelection(item.Find("blockquote"))
			return
		}
		if item.Find("code").Length() > 0 {
			item.ReplaceWithSelection(item.Find("code"))
			return
		}
		if item.Find("pre").Length() > 0 {
			item.ReplaceWithSelection(item.Find("pre"))
			return
		}
		if item.Find("img").Length() > 0 {
			item.ReplaceWithSelection(item.Find("img"))
			return
		}
		//其他情况
		item.SetText(strings.TrimSpace(strings.ReplaceAll(item.Text(), "\n", " ")))
	})

	inner := clonedItem.Find("*")
	inner.Each(func(i int, innerItem *goquery.Selection) {
		//移除 class,style
		innerItem.RemoveAttr("class")
		innerItem.RemoveAttr("style")
		innerItem.RemoveAttr("id")
		//Unwrap
		if innerItem.Children().Length() > 0 {

		}
		//移除不需要的词
		if innerItem.Children().Length() == 0 && HasContain(innerItem.Text(), config.CollectorConfig.ContentExcludeLine) {
			innerItem.Remove()
			return
		}
		//清理a
		if innerItem.Is("a") && HasContain(innerItem.Text(), config.CollectorConfig.LinkExclude) {
			innerItem.Remove()
			return
		}
	})

	clonedItem.Find("h1,a").Contents().Unwrap()

	content, _ := clonedItem.Html()
	content = strings.TrimSpace(content)
	contentText := strings.TrimSpace(clonedItem.Text())

	//清理空格
	re, _ := regexp.Compile(`\s+`)
	contentText = re.ReplaceAllString(contentText, " ")
	//清理空标签
	content = RemoveTags(content)

	return content, contentText
}

func RemoveTags(html string) string {
	re, _ := regexp.Compile("<[^>/]+></[^>]+>")
	rep := re.ReplaceAllString(html, "")
	if rep != html {
		return RemoveTags(rep)
	}

	//移除连续空行
	rep = strings.TrimSpace(rep)

	return rep
}

func HasPrefix(need string, needArray []string) bool {
	for _, v := range needArray {
		if v == "" {
			continue
		}
		if strings.HasPrefix(need, v) {
			return true
		}
	}

	return false
}

func HasSuffix(need string, needArray []string) bool {
	for _, v := range needArray {
		if v == "" {
			continue
		}
		if strings.HasSuffix(need, v) {
			return true
		}
	}

	return false
}

func HasContain(need string, needArray []string) bool {
	for _, v := range needArray {
		if v == "" {
			continue
		}
		if strings.Contains(need, v) {
			return true
		}
	}

	return false
}

// ReplaceContentFromConfig 替换文章内容
func ReplaceContentFromConfig(content string) string {
	if len(content) > 0 && len(config.CollectorConfig.ContentReplace) > 0 {
		for _, item := range config.CollectorConfig.ContentReplace {
			value := strings.Split(item, "||")
			if len(value) == 2 {
				//这是一对正确的值
				content = strings.ReplaceAll(content, value[0], value[1])
			}
		}
	}

	//remove telephone and wechat and qq

	return content
}

func ParseLinking(htmlDom *goquery.Document, baseUrl string) {
	aList := htmlDom.Find("a")
	aList.Each(func(i int, nodeItem *goquery.Selection) {
		subURL, exist := nodeItem.Attr("href")
		if !exist || emptyLinkPattern.MatchString(subURL) {
			return
		}
		fullURL, _ := joinURL(baseUrl, subURL)
		nodeItem.SetAttr("href", fullURL)
	})

	linkList := htmlDom.Find("link")
	linkList.Each(func(i int, nodeItem *goquery.Selection) {
		subURL, exist := nodeItem.Attr("href")
		if !exist || emptyLinkPattern.MatchString(subURL) {
			return
		}
		fullURL, _ := joinURL(baseUrl, subURL)
		nodeItem.SetAttr("href", fullURL)
	})

	scriptList := htmlDom.Find("script")
	scriptList.Each(func(i int, nodeItem *goquery.Selection) {
		subURL, exist := nodeItem.Attr("src")
		if !exist || emptyLinkPattern.MatchString(subURL) {
			return
		}
		fullURL, _ := joinURL(baseUrl, subURL)
		nodeItem.SetAttr("src", fullURL)
	})

	imgList := htmlDom.Find("img")
	imgList.Each(func(i int, nodeItem *goquery.Selection) {
		subURL, exist := nodeItem.Attr("src")
		if !exist || emptyLinkPattern.MatchString(subURL) {
			return
		}
		fullURL, _ := joinURL(baseUrl, subURL)
		nodeItem.SetAttr("src", fullURL)
	})

	videoList := htmlDom.Find("video")
	videoList.Each(func(i int, nodeItem *goquery.Selection) {
		subURL, exist := nodeItem.Attr("src")
		if !exist || emptyLinkPattern.MatchString(subURL) {
			return
		}
		fullURL, _ := joinURL(baseUrl, subURL)
		nodeItem.SetAttr("src", fullURL)
	})

	audioList := htmlDom.Find("audio")
	audioList.Each(func(i int, nodeItem *goquery.Selection) {
		subURL, exist := nodeItem.Attr("src")
		if !exist || emptyLinkPattern.MatchString(subURL) {
			return
		}
		fullURL, _ := joinURL(baseUrl, subURL)
		nodeItem.SetAttr("src", fullURL)
	})
}

//注意，第三方url的host不做覆盖
func joinURL(baseURL, subURL string) (fullURL, fullURLWithoutFrag string) {
	baseURL = strings.TrimSpace(baseURL)
	subURL = strings.TrimSpace(subURL)
	baseURLObj, err := url.Parse(baseURL)
	if err != nil {
		return
	}
	subURLObj, err := url.Parse(subURL)
	if err != nil {
		return
	}
	fullURLObj := baseURLObj.ResolveReference(subURLObj)
	fullURL = fullURLObj.String()
	fullURLObj.Fragment = ""
	fullURLWithoutFrag = fullURLObj.String()
	return
}

// PseudoOriginalArticle 伪原创一篇文章
func PseudoOriginalArticle(article *model.Article) error {
	isEnglish := CheckContentIsEnglish(article.ArticleData.Content)

	content := ParsePlanText(article.ArticleData.Content, "")
	content = article.Title + "\n" + content

	content = PseudoArticle(content, isEnglish)
	if content == "" {
		return errors.New(fmt.Sprintf("伪原创失败：%s", article.Title))
	}

	contents := strings.SplitN(content, "\n", 2)
	if len(contents) < 2 {
		return errors.New(fmt.Sprintf("伪原创失败：%s", article.Title))
	}

	title := strings.TrimSpace(contents[0])
	if title == "" {
		title = article.Title
	}
	content = contents[1]
	//替换回html
	content = ParsePlanText(article.ArticleData.Content, content)

	article.Title = title
	article.HasPseudo = 1
	config.DB.Model(&model.Article{}).Where("id = ?", article.Id).Select("pseudo_title", "has_pseudo").Updates(article)
	article.ArticleData.Content = content
	config.DB.Model(&model.ArticleData{}).Where("id = ?", article.Id).UpdateColumn("pseudo_content", content)

	return nil
}

func TrimContents(content string) string {
	//移除连续空行
	re, _ := regexp.Compile(`\n+`)
	content = re.ReplaceAllString(content, "\n")

	return strings.TrimSpace(content)
}

// CheckContentIsEnglish 统计落在 0-127之间的数量，如果达到95%，则认为是英文, 简单的方式处理
func CheckContentIsEnglish(content string) bool {
	if len(content) > 128 {
		content = content[:128]
	}

	enCount := 0
	for i := 0; i < len(content); i++ {
		if content[i] < 128 {
			enCount++
		}
	}

	if float64(enCount) > float64(len(content)) * 0.95 {
		return true
	}

	return false
}

func ReplaceArticles(replaceItems []string) {
	startId := uint(0)

	var articles []*model.Article
	for {
		tx := config.DB.Model(&model.Article{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&articles)
		if len(articles) == 0 {
			break
		}
		startId = articles[len(articles)-1].Id
		for _, article := range articles {
			var articleData model.ArticleData
			title := article.Title
			config.DB.Where("id = ?", article.Id).Take(&articleData)
			content := articleData.Content

			for _, v := range replaceItems {
				replaceItem := strings.Split(v, "->")
				if len(replaceItem) != 2 {
					continue
				}
				title = strings.ReplaceAll(title, replaceItem[0], replaceItem[1])
				if content != "" {
					content = strings.ReplaceAll(content, replaceItem[0], replaceItem[1])
				}
			}
			//替换完了
			hasReplace := false
			if title != article.Title {
				hasReplace = true
				config.DB.Model(article).UpdateColumn("title", title)
			}
			if content != articleData.Content {
				hasReplace = true
				articleData.Content = content
				config.DB.Model(&articleData).UpdateColumns(articleData)
			}
			if hasReplace {
				log.Println("替换文章：" + article.Title)
			}
		}
	}
}

func ParsePlanText(content string, planText string) string {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("捕获异常:", err)
		}
	}()

	htmlR := strings.NewReader(content)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err != nil {
		return content
	}

	if planText == "" {
		var contents []string
		doc.Find("body").Contents().Each(func(i int, item *goquery.Selection) {
			text := item.Text()
			if item.Children().Length() == 0 && text != "" {
				contents = append(contents, item.Text())
			}
		})

		return strings.Join(contents, "\n")
	} else {
		contents := strings.Split(planText, "\n")
		index := 0
		doc.Find("body").Contents().Each(func(i int, item *goquery.Selection) {
			text := item.Text()

			if item.Children().Length() == 0 && text != "" {
				if index < len(contents) {
					item.SetText(contents[index])
					index++
				}
			}
		})

		content, _ = doc.Find("body").Html()

		return content
	}
}

// PseudoArticle 伪原创一篇文章，先转成英语，再转成中文
func PseudoArticle(content string, isEnglish bool) string {
	langIndex := 1
	if isEnglish {
		langIndex = 0
	}
	//第一次转换
	translateFunc := TranslateSources.getSource()
	to := articleLang[langIndex]
	content = translateFunc(content, to)
	if content == "" {
		//进行一次尝试
		translateFunc := TranslateSources.getSource()
		to := articleLang[langIndex]
		content = translateFunc(content, to)
	}
	//第二次转换
	translateFunc = TranslateSources.getSource()
	to = articleLang[(langIndex + 1)%2]
	content = translateFunc(content, to)
	if content == "" {
		//进行一次尝试
		translateFunc = TranslateSources.getSource()
		to = articleLang[(langIndex + 1)%2]
		content = translateFunc(content, to)
	}
	return content
}

func GetArticleTotalByKeywordId(id uint) int64 {
	var total int64
	config.DB.Model(&model.Article{}).Where("keyword_id = ?", id).Count(&total)

	return total
}

func checkArticleExists(originUrl, originTitle string) bool {
	var total int64
	config.DB.Model(&model.Article{}).Where("origin_url = ?", originUrl).Count(&total)
	if total > 0 {
		return true
	}
	config.DB.Model(&model.Article{}).Where("origin_title = ?", originTitle).Count(&total)
	if total > 0 {
		return true
	}

	return false
}