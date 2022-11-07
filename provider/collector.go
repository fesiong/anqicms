package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jinzhu/now"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/dao"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
	"log"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var emptyLinkPatternStr = `(^data:)|(^tel:)|(^mailto:)|(about:blank)|(javascript:)`
var emptyLinkPattern = regexp.MustCompile(emptyLinkPatternStr)

func GetUserCollectorSetting() config.CollectorJson {
	var collector config.CollectorJson
	buf, err := os.ReadFile(fmt.Sprintf("%scollector.json", config.ExecPath))
	configStr := ""
	if err != nil {
		//文件不存在
		return collector
	}
	configStr = string(buf[:])
	reg := regexp.MustCompile(`/\*.*\*/`)

	configStr = reg.ReplaceAllString(configStr, "")
	buf = []byte(configStr)

	if err = json.Unmarshal(buf, &collector); err != nil {
		return collector
	}

	return collector
}

func SaveUserCollectorSetting(req config.CollectorJson, focus bool) error {
	collector := GetUserCollectorSetting()
	if focus {
		collector = req
	} else {
		if req.ErrorTimes > 0 {
			collector.ErrorTimes = req.ErrorTimes
		}
		if req.Channels > 0 {
			collector.Channels = req.Channels
		}
		if req.TitleMinLength > 0 {
			collector.TitleMinLength = req.TitleMinLength
		}
		if req.ContentMinLength > 0 {
			collector.ContentMinLength = req.ContentMinLength
		}
		if req.TitleExclude != nil {
			collector.TitleExclude = req.TitleExclude
		}
		if req.TitleExcludePrefix != nil {
			collector.TitleExcludePrefix = req.TitleExcludePrefix
		}
		if req.TitleExcludeSuffix != nil {
			collector.TitleExcludeSuffix = req.TitleExcludeSuffix
		}
		if req.ContentExcludeLine != nil {
			collector.ContentExcludeLine = req.ContentExcludeLine
		}
		if req.ContentExclude != nil {
			collector.ContentExclude = req.ContentExclude
		}
		if req.ContentReplace != nil {
			collector.ContentReplace = req.ContentReplace
		}
		if req.AutoPseudo {
			collector.AutoPseudo = req.AutoPseudo
		}
		if req.CategoryId > 0 {
			collector.CategoryId = req.CategoryId
		}
		if req.StartHour > 0 {
			collector.StartHour = req.StartHour
		}
		if req.EndHour > 0 {
			collector.EndHour = req.EndHour
		}
		if req.DailyLimit > 0 {
			collector.DailyLimit = req.DailyLimit
		}
	}

	//将现有配置写回文件
	configFile, err := os.OpenFile(fmt.Sprintf("%scollector.json", config.ExecPath), os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}

	defer configFile.Close()

	buff := &bytes.Buffer{}

	buf, err := json.MarshalIndent(collector, "", "\t")
	if err != nil {
		return err
	}
	buff.Write(buf)

	_, err = io.Copy(configFile, buff)
	if err != nil {
		return err
	}

	//重新读取配置
	config.LoadCollectorConfig()

	return nil
}

// getBetweenSeconds 根据发布量获取每天发布间隔
// 根据设置的时间，随机休息秒数
func getBetweenSeconds() time.Duration {
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

	between := seconds/limit*2 - 1
	if config.CollectorConfig.AutoPseudo {
		between -= 1
	}

	//返回随机正负5秒的实际
	between += rand.Intn(11) - 5

	if between <= 0 {
		between = 1
	}

	return time.Duration(between) * time.Second
}

var runningCollectArticles = false

func CollectArticles() {
	if dao.DB == nil {
		return
	}
	if !config.CollectorConfig.AutoCollect {
		return
	}
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

	// 如果采集的文章数量达到了设置的限制，则当天停止采集
	if GetTodayArticleCount() > int64(config.CollectorConfig.DailyLimit) {
		return
	}

	lastId := uint(0)
	for {
		var keywords []*model.Keyword
		dao.DB.Where("id > ? and last_time = 0", lastId).Order("id asc").Limit(100).Find(&keywords)
		if len(keywords) == 0 {
			break
		}
		lastId = keywords[len(keywords)-1].Id
		for i := 0; i < len(keywords); i++ {
			keyword := keywords[i]
			total, err := CollectArticlesByKeyword(*keyword, false)
			log.Printf("关键词：%s 采集了 %d 篇文章", keyword.Title, total)
			// 达到数量了，退出
			if GetTodayArticleCount() > int64(config.CollectorConfig.DailyLimit) {
				return
			}
			if err != nil {
				// 采集出错了，多半是出验证码了，跳过该任务，等下次开始
				break
			}
		}
	}
}

func CollectArticlesByKeyword(keyword model.Keyword, focus bool) (total int, err error) {
	if config.CollectorConfig.CollectMode == config.CollectModeCombine {
		total, err = GenerateCombination(&keyword)
	} else if config.CollectorConfig.CollectMode == config.CollectModeCollect {
		total, err = CollectArticleFromBaidu(&keyword, focus)
	}

	if err != nil {
		return total, err
	}

	keyword.ArticleCount = GetArticleTotalByKeywordId(keyword.Id)
	keyword.LastTime = time.Now().Unix()
	dao.DB.Model(keyword).Select("article_count", "last_time").Updates(keyword)

	return total, nil
}

func SaveCollectArticle(archive *request.Archive, keyword *model.Keyword) error {
	autoPseudo := false
	if config.CollectorConfig.AutoPseudo {
		autoPseudo = true
	}

	//原始标题
	archive.OriginTitle = archive.Title

	if checkArticleExists(archive.OriginUrl, archive.OriginTitle) {
		log.Println("已存在于数据库", archive.OriginTitle)
		return errors.New("已存在于数据库")
	}

	archive.KeywordId = keyword.Id
	archive.CategoryId = keyword.CategoryId
	if archive.CategoryId == 0 && config.CollectorConfig.CategoryId > 0 {
		archive.CategoryId = config.CollectorConfig.CategoryId
	}
	// 必须有一个分类，如果都没有，则获取第一个
	if archive.CategoryId == 0 {
		var category model.Category
		dao.DB.Where("module_id = 1").Take(&category)
		archive.CategoryId = category.Id
	}
	log.Println("draft:", config.CollectorConfig.SaveType)
	// 如果不是正常发布，则存到草稿
	if config.CollectorConfig.SaveType == 0 {
		archive.Draft = true
	} else {
		archive.Draft = false
	}
	modelArchive, err := SaveArchive(archive)
	if err != nil {
		log.Println("保存文章出错：", archive.Title, err.Error())
		return err
	}

	//如果自动伪原创
	if autoPseudo {
		archiveData, err := GetArchiveDataById(modelArchive.Id)
		if err == nil {
			go PseudoOriginalArticle(archiveData)
		}
	}
	//文章计数
	UpdateTodayArticleCount(1)

	return nil
}

func CollectArticleFromBaidu(keyword *model.Keyword, focus bool) (int, error) {
	collectUrl := fmt.Sprintf("https://www.baidu.com/s?wd=%s&tn=json&rn=50&pn=10", keyword.Title)
	if config.CollectorConfig.FromWebsite != "" {
		collectUrl = fmt.Sprintf("https://www.baidu.com/s?wd=inurl%%3A%s%%20%s&tn=json&rn=50&pn=10", keyword.Title, config.CollectorConfig.FromWebsite)
	}

	resp, err := library.Request(collectUrl, &library.Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			"Referer":         "https://www.baidu.com",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		},
	})
	if err != nil {
		return 0, err
	}

	var total int
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

		//预判标题是否相关
		if !ContainKeywords(link.Name, keyword.Title) {
			continue
		}

		archive, err := CollectSingleArticle(link, keyword)
		if err == nil {
			err = SaveCollectArticle(archive, keyword)

			if err == nil {
				total++
				if !focus {
					//根据设置的时间，随机休息秒数
					time.Sleep(getBetweenSeconds())
				}
			}
		}
	}

	return total, nil
}

func CollectSingleArticle(link *response.WebLink, keyword *model.Keyword) (*request.Archive, error) {
	//百度的不使用 chromedp
	resp, err := library.Request(link.Url, &library.Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			"Referer": fmt.Sprintf("https://www.baidu.com/s?wd=%s", url.QueryEscape(keyword.Title)),
		},
	})
	if err != nil {
		return nil, err
	}

	archive := &request.Archive{
		OriginUrl:   link.Url,
		ContentText: resp.Body,
	}
	_ = ParseArticleDetail(archive)
	if len(archive.Content) == 0 {
		log.Println("链接无文章", archive.OriginUrl)
		return nil, errors.New("链接无文章")
	}
	if archive.Title == "" {
		log.Println("链接无文章", archive.OriginUrl)
		return nil, errors.New("链接无文章")
	}
	//对乱码的跳过
	runeTitle := []rune(archive.Title)
	isDeny := false
	for _, r := range runeTitle {
		if r == 65533 {
			isDeny = true
			break
		}
	}
	if isDeny {
		log.Println("乱码", archive.OriginUrl)
		return nil, errors.New("乱码")
	}

	log.Println(archive.Title, len(archive.Content), archive.OriginUrl)

	return archive, nil
}

func ParseBaiduJson(content string) []*response.WebLink {
	var baiduJson response.BaiduJson
	err := json.Unmarshal([]byte(content), &baiduJson)
	if err != nil {
		return nil
	}

	var links []*response.WebLink
	for _, v := range baiduJson.Feed.Entry {
		// 百度的链接，都加上https
		//if strings.HasPrefix(v.Url, "http://") && strings.Contains(v.Url, "baidu.com") {
		//	v.Url = "https://" + strings.TrimPrefix(v.Url, "http://")
		//}
		// 百度自家的不采集
		if strings.Contains(v.Url, "baidu.com") {
			continue
		}
		links = append(links, &response.WebLink{
			Name: v.Title,
			Url:  v.Url,
		})
	}

	return links
}

func ParseArticleDetail(archive *request.Archive) error {
	//先删除一些不必要的标签
	re, _ := regexp.Compile("\\<style[\\S\\s]+?\\</style\\>")
	contentText := re.ReplaceAllString(archive.ContentText, "")
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
	if strings.Contains(archive.OriginUrl, "baike.baidu.com") {
		ParseBaikeDetail(archive, doc)
	} else if strings.Contains(archive.OriginUrl, "mp.weixin.qq.com") {
		ParseWeixinDetail(archive, doc)
	} else if strings.Contains(archive.OriginUrl, "zhihu.com") {
		ParseZhihuDetail(archive, doc)
	} else if strings.Contains(archive.OriginUrl, "toutiao.com") {
		ParseToutiaoDetail(archive, doc)
	} else {
		ParseNormalDetail(archive, doc)
	}

	archive.Content = ReplaceContentFromConfig(archive.Content, config.CollectorConfig.ContentReplace)

	return nil
}

func ParseBaikeDetail(archive *request.Archive, doc *goquery.Document) {
	//获取标题
	archive.Title = strings.TrimSpace(doc.Find("h1").Text())

	doc.Find(".edit-icon").Remove()
	contentList := doc.Find(".para-title,.para")
	content := ""
	for i := range contentList.Nodes {
		content += "<p>" + contentList.Eq(i).Text() + "</p>"
	}

	archive.Content = content
	//如果获取不到内容，则 fallback 到normal
	if archive.Content == "" {
		ParseNormalDetail(archive, doc)
	}
}

func ParseWeixinDetail(archive *request.Archive, doc *goquery.Document) {
	//获取标题
	archive.Title = strings.TrimSpace(doc.Find("h1").Eq(0).Text())

	contentList := doc.Find("#js_content")

	content, _ := CleanTags(contentList)

	archive.Content = content
	//如果获取不到内容，则 fallback 到normal
	if archive.Content == "" {
		ParseNormalDetail(archive, doc)
	}
}

func ParseZhihuDetail(archive *request.Archive, doc *goquery.Document) {
	//获取标题
	archive.Title = strings.TrimSpace(doc.Find("h1").Eq(0).Text())

	var content, planText string
	doc.Find(".RichContent-inner .RichText,.Post-RichTextContainer .RichText").Each(func(i int, item *goquery.Selection) {
		tmpContent, tmpText := CleanTags(item)
		if len(tmpText) > len(planText) {
			content = tmpContent
			planText = tmpText
		}
	})

	archive.Content = content
	//如果获取不到内容，则 fallback 到normal
	if archive.Content == "" {
		ParseNormalDetail(archive, doc)
	}
}

func ParseToutiaoDetail(archive *request.Archive, doc *goquery.Document) {
	//获取标题
	archive.Title = strings.TrimSpace(doc.Find("h1").Eq(0).Text())

	contentList := doc.Find(".article-content article")

	content, _ := CleanTags(contentList)

	archive.Content = content
	//如果获取不到内容，则 fallback 到normal
	if archive.Content == "" {
		ParseNormalDetail(archive, doc)
	}
}

func ParseNormalDetail(archive *request.Archive, doc *goquery.Document) {
	ParseLinking(doc, archive.OriginUrl)
	title := ParseArticleTitle(doc)

	otherItems := doc.Find("input,textarea,select,radio,form,button,header,aside,footer,.footer,noscript,meta,nav,hr")
	if otherItems.Length() > 0 {
		otherItems.Remove()
	}

	//根据标题判断是否是英文，如果是英文，则采用英文的计数
	isEnglish := CheckContentIsEnglish(title)
	if title != "" {
		archive.Title = title
	}

	if utf8.RuneCountInString(archive.Title) < config.CollectorConfig.TitleMinLength || HasContain(archive.Title, config.CollectorConfig.TitleExclude) || HasPrefix(archive.Title, config.CollectorConfig.TitleExcludePrefix) || HasSuffix(archive.Title, config.CollectorConfig.TitleExcludeSuffix) {
		archive.Title = ""
		//跳过这篇文章
		return
	}

	//尝试获取正文内容
	content, planText, _, _ := ParseArticleContent(doc.Find("body"), 0, isEnglish)
	if content != nil {
		archive.Content, planText = CleanTags(content)
		planLen := utf8.RuneCountInString(planText)
		if isEnglish {
			//英语使用空格来计算词数
			planLen = len(strings.Split(planText, " "))
		}
		if planLen < config.CollectorConfig.ContentMinLength {
			//小于指定次数的，直接抛弃了
			archive.Content = ""
		}
	}
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

func ParseArticleContent(nodeItem *goquery.Selection, deep int, isEnglish bool) (*goquery.Selection, string, int, int) {
	var content *goquery.Selection
	contentText := ""

	maxPCount := nodeItem.ChildrenFiltered("p").Length()
	maxDeep := deep
	planText := strings.TrimSpace(CleanTagsAndSpaces(nodeItem.Text()))
	planLen := utf8.RuneCountInString(planText)
	if isEnglish {
		//英语使用空格来计算词数
		planLen = len(strings.Split(planText, " "))
	}
	if planLen < config.CollectorConfig.ContentMinLength {
		//小于指定次数的，直接抛弃了
		return nil, contentText, maxDeep, maxPCount
	}
	// 如果有上一页、下一页明显的列表页特征，则跳过
	if strings.Contains(planText, "上一页") || strings.Contains(planText, "下一页") {
		return nil, "", maxDeep, maxPCount
	}

	children := nodeItem.Children()
	for i := range children.Nodes {
		item := children.Eq(i)
		tmpContent, tmpText, tmpDeep, tmpPCount := ParseArticleContent(item, deep+1, isEnglish)
		if maxPCount == -1 {
			maxPCount = tmpPCount
		}
		if tmpDeep > maxDeep {
			maxDeep = tmpDeep
		}
		if tmpText != "" && len(tmpText) > len(contentText) && tmpPCount >= maxPCount {
			//表示有内容
			content = tmpContent
			contentText = tmpText
			maxPCount = tmpPCount
		}
	}

	if content != nil {
		return content, contentText, maxDeep, maxPCount
		//return content, contentText, maxDeep, maxPCount
		//if nodeItem.ChildrenFiltered("p").Length() == 0 {
		//	return content, contentText, maxDeep, maxPCount
		//}
		////系数
		//if float64(len(contentText))*1.5 > float64(len(nodeItem.Text())) {
		//	return content, contentText, maxDeep, maxPCount
		//}
	}

	// 通过一级一级的往下查找
	liItems := nodeItem.Find("li")
	if liItems.Length() > 0 && liItems.Length() < liItems.Find("a").Length() {
		return content, contentText, maxDeep, maxPCount
	}
	aLinks := nodeItem.Find("a")
	aText := strings.TrimSpace(CleanTagsAndSpaces(aLinks.Text()))
	if len(aText)*5 > len(planText) {
		//a标签过多，这个不是文章内容
		return content, contentText, maxDeep, maxPCount
	}

	//如果内容包含指定关键词，则集体抛弃
	if HasContain(planText, config.CollectorConfig.ContentExclude) {
		return nil, contentText, maxDeep, maxPCount
	}

	if nodeItem.Is("ul") ||
		nodeItem.Is("ol") ||
		nodeItem.Is("dl") ||
		nodeItem.Is("aside") {
		return content, contentText, maxDeep, maxPCount
	}

	return nodeItem.Clone(), planText, maxDeep, maxPCount
}

func CleanTags(clonedItem *goquery.Selection) (string, string) {
	contentText := strings.TrimSpace(clonedItem.Text())
	//清理空格
	re, _ := regexp.Compile(`\s{2,}`)
	contentText = re.ReplaceAllString(contentText, " ")

	pLength := clonedItem.ChildrenFiltered("p").Length()
	divs := clonedItem.ChildrenFiltered("div")
	if pLength > divs.Length()*3 {
		divs.Remove()
	}
	clonedItem.Find("blockquote").Each(func(i int, item *goquery.Selection) {
		if item.Find("a").Length() > 1 {
			item.Remove()
			return
		}
	})
	clonedItem.Find("ul,ol,dl").Each(func(i int, item *goquery.Selection) {
		if item.Find("a").Length() >= item.Find("li,dd").Length() {
			item.Remove()
			return
		}
	})
	clonedItem.Find("a").Each(func(i int, item *goquery.Selection) {
		parent := item.Parent()
		if len(strings.TrimSpace(parent.Text())) < len(strings.TrimSpace(item.Text()))*2 {
			parent.Remove()
			return
		}
	})

	for {
		if clonedItem.Children().Length() == 1 && clonedItem.Children().Contents().Length() > 0 {
			clonedItem.Children().Contents().Unwrap()
		} else {
			break
		}
	}
	//降dom深度
	clonedItem.Children().Each(func(i int, item *goquery.Selection) {
		// 如果是隐藏的，则删除
		style, exists := item.Attr("style")
		if exists && strings.Contains(strings.ReplaceAll(strings.ToLower(style), " ", ""), "display:none") {
			item.Remove()
			return
		}
		if item.Is("pre") {
			return
		}
		if item.Is("code") {
			// 重新wrap
			item.WrapHtml("<pre></pre>")
			return
		}
		if item.Is("img") {
			src, _ := item.Attr("src")
			dataSrc, exists2 := item.Attr("data-src")
			if exists2 {
				src = dataSrc
			}
			dataSrc, exists2 = item.Attr("data-original")
			if exists2 {
				src = dataSrc
			}
			alt, _ := item.Attr("alt")
			if src == "" {
				item.Remove()
			} else {
				item.ReplaceWithHtml("<p><img src=\"" + src + "\" alt=\"" + alt + "\"/></p>")
			}
			return
		}
		if item.Find("pre").Length() > 0 {
			item.ReplaceWithSelection(item.Find("pre"))
			return
		}
		if item.Find("code").Length() > 0 {
			tmp := item.Find("code").Text()
			item.ReplaceWithHtml("<pre><code>" + tmp + "</code></pre>")
			return
		}
		if item.Find("img").Length() > 0 {
			item.Find("img").Each(func(i int, inner *goquery.Selection) {
				src, _ := inner.Attr("src")
				dataSrc, exists2 := inner.Attr("data-src")
				if exists2 {
					src = dataSrc
					inner.RemoveAttr("data-src")
				}
				dataSrc, exists2 = inner.Attr("data-original")
				if exists2 {
					src = dataSrc
					inner.RemoveAttr("data-original")
				}
				inner.SetAttr("src", src)
			})
			return
		}
		//其他情况
		// 如果一行内容，内部只有a，则移除
		if item.Find("a").Length() > 0 && strings.TrimSpace(item.Find("a").Text()) == strings.TrimSpace(item.Text()) {
			item.Remove()
			return
		}
	})
	clonedItem.Find("h1,a,span").Contents().Unwrap()
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
		if HasContain(innerItem.Text(), config.CollectorConfig.ContentExcludeLine) {
			if innerItem.Children().Length() == 0 || innerItem.Children().Text() == "" {
				innerItem.Remove()
				return
			}
		}
	})

	content, _ := clonedItem.Html()
	content = strings.TrimSpace(content)
	if clonedItem.Is("p") {
		content = "<p>" + content + "</p>"
	}
	contentText = strings.TrimSpace(clonedItem.Text())
	//清理空格
	re, _ = regexp.Compile(`\s{2,}`)
	contentText = re.ReplaceAllString(contentText, " ")
	//清理空标签
	content = RemoveTags(content)

	return content, contentText
}

func CleanTagsAndSpaces(content string) string {
	re, _ := regexp.Compile("\\<style[\\S\\s]+?\\</style\\>")
	content = re.ReplaceAllString(content, "")
	re, _ = regexp.Compile("\\<script[\\S\\s]+?\\</script\\>")
	content = re.ReplaceAllString(content, "")

	endReg, _ := regexp.Compile("\\</[\\S\\s]+?\\>")
	content = endReg.ReplaceAllString(content, "\n")
	re, _ = regexp.Compile("\\<[\\S\\s]+?\\>")
	content = re.ReplaceAllString(content, "")
	content = strings.ReplaceAll(content, string(rune(0xA0)), " ")
	content = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(content, "&nbsp;", " "), "&ensp;", " "), "&emsp;", " ")
	// 替换连续回车为1个回车
	re, _ = regexp.Compile("(\n *){2,}")
	content = re.ReplaceAllString(content, "\n")
	// 替换连续空格为1个空格
	re, _ = regexp.Compile("[ ]{2,}")
	content = re.ReplaceAllString(content, " ")
	content = strings.TrimSpace(content)

	return content
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
	if need == "" {
		return false
	}
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
func ReplaceContentFromConfig(content string, replacer []config.ReplaceKeyword) string {
	if content == "" || len(replacer) <= 0 {
		return content
	}

	var re *regexp.Regexp
	var err error

	// 替换功能，只替换内容，不替换标签， 因此需要将标签存起来，并在最后还原
	var replaced = map[string]string{}
	if strings.Contains(content, "<") {
		re, _ = regexp.Compile(`<[^>]+>`)
		results := re.FindAllString(content, -1)
		for i, v := range results {
			key := fmt.Sprintf("{$%d}", i)
			replaced[key] = v
			content = strings.ReplaceAll(content, v, key)
		}
	}

	for _, v := range replacer {
		// 增加支持正则表达式替换
		if strings.HasPrefix(v.From, "{") && strings.HasSuffix(v.From, "}") && len(v.From) > 2 {
			newWord := v.From[1 : len(v.From)-1]
			// 支持特定规则：邮箱地址，手机号，电话号码，网址、微信号，QQ号，
			if newWord == "邮箱地址" {
				re, err = regexp.Compile(`\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*`)
			} else if newWord == "日期" {
				re, err = regexp.Compile(`\d{2,4}[\-/年月日]\d{1,2}[\-/年月日]?(\d{1,2}[\-/年月日]?)?`)
			} else if newWord == "时间" {
				re, err = regexp.Compile(`\d{2}[:时分秒]\d{2}[:时分秒]?(\d{2}[:时分秒]?)?`)
			} else if newWord == "电话号码" {
				re, err = regexp.Compile(`[+\d]{2}[\d\-+\s]{5,16}`)
			} else if newWord == "QQ号" {
				re, err = regexp.Compile(`[1-9]\d{4,10}`)
			} else if newWord == "微信号" {
				re, err = regexp.Compile(`[a-zA-Z][a-zA-Z\d_-]{5,19}`)
			} else if newWord == "网址" {
				re, err = regexp.Compile(`(?i)((http|ftp|https)://)?[\w\-_]+(\.[\w\-_]+)+([\w\-.,@?^=%&:/~+#]*[\w\-@?^=%&/~+#])?`)
			} else {
				re, err = regexp.Compile(newWord)
			}

			if err == nil {
				content = re.ReplaceAllString(content, v.To)
			}
			continue
		}
		content = strings.ReplaceAll(content, v.From, v.To)
	}
	for key, val := range replaced {
		content = strings.ReplaceAll(content, key, val)
	}

	return content
}

func ParseLinking(htmlDom *goquery.Document, baseUrl string) {
	aList := htmlDom.Find("[href]")
	aList.Each(func(i int, nodeItem *goquery.Selection) {
		subURL, exist := nodeItem.Attr("href")
		if !exist || emptyLinkPattern.MatchString(subURL) {
			return
		}
		fullURL, _ := joinURL(baseUrl, subURL)
		nodeItem.SetAttr("href", fullURL)
	})

	srcList := htmlDom.Find("[src]")
	srcList.Each(func(i int, nodeItem *goquery.Selection) {
		subURL, exist := nodeItem.Attr("src")
		// 尝试获取data-src
		dataSrc, exists := nodeItem.Attr("data-src")
		if exists {
			subURL = dataSrc
			nodeItem.RemoveAttr("data-src")
		}
		dataSrc, exists = nodeItem.Attr("data-original")
		if exists {
			subURL = dataSrc
			nodeItem.RemoveAttr("data-original")
		}
		if !exist || emptyLinkPattern.MatchString(subURL) {
			return
		}
		fullURL, _ := joinURL(baseUrl, subURL)
		nodeItem.SetAttr("src", fullURL)
	})
}

// 注意，第三方url的host不做覆盖
func joinURL(baseURL, subURL string) (fullURL, fullURLWithoutFrag string) {
	baseURL = strings.TrimSpace(baseURL)
	subURL = strings.TrimSpace(subURL)
	baseURLObj, err := url.Parse(baseURL)
	if err != nil {
		return
	}
	fullURLObj := baseURLObj
	subURLObj, err := url.Parse(subURL)
	if err == nil {
		fullURLObj = baseURLObj.ResolveReference(subURLObj)
	}
	fullURL = fullURLObj.String()
	fullURLObj.Fragment = ""
	fullURLWithoutFrag = fullURLObj.String()
	return
}

// PseudoOriginalArticle 伪原创一篇文章
func PseudoOriginalArticle(archiveData *model.ArchiveData) error {
	isEnglish := CheckContentIsEnglish(archiveData.Content)

	content := ParsePlanText(archiveData.Content, "")

	content = PseudoArticle(content, isEnglish)
	if content == "" {
		return errors.New(fmt.Sprintf("伪原创失败：%d", archiveData.Id))
	}

	//替换回html
	content = ParsePlanText(archiveData.Content, content)

	dao.DB.Model(&model.Archive{}).Where("id = ?", archiveData.Id).UpdateColumn("has_pseudo", 1)
	dao.DB.Model(&model.ArchiveData{}).Where("id = ?", archiveData.Id).UpdateColumn("pseudo_content", content)

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
	if float64(enCount) > float64(len(content))*0.95 {
		return true
	}

	return false
}

func ReplaceArticles() {
	startId := uint(0)
	var archives []*model.Archive
	for {
		tx := dao.DB.Model(&model.Archive{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&archives)
		if len(archives) == 0 {
			break
		}
		startId = archives[len(archives)-1].Id
		for _, archive := range archives {
			var archiveData model.ArchiveData
			title := ReplaceContentFromConfig(archive.Title, config.CollectorConfig.ContentReplace)
			dao.DB.Where("id = ?", archive.Id).Take(&archiveData)
			content := ReplaceContentFromConfig(archiveData.Content, config.CollectorConfig.ContentReplace)

			//替换完了
			hasReplace := false
			if title != archive.Title {
				hasReplace = true
				dao.DB.Model(archive).UpdateColumn("title", title)
			}
			if content != archiveData.Content {
				hasReplace = true
				archiveData.Content = content
				dao.DB.Model(&archiveData).UpdateColumns(archiveData)
			}
			if hasReplace {
				log.Println("替换文章：" + archive.Title)
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
	to := archiveLang[langIndex]
	content = translateFunc(content, to)
	if content == "" {
		//进行一次尝试
		translateFunc := TranslateSources.getSource()
		to := archiveLang[langIndex]
		content = translateFunc(content, to)
	}
	//第二次转换
	translateFunc = TranslateSources.getSource()
	to = archiveLang[(langIndex+1)%2]
	content = translateFunc(content, to)
	if content == "" {
		//进行一次尝试
		translateFunc = TranslateSources.getSource()
		to = archiveLang[(langIndex+1)%2]
		content = translateFunc(content, to)
	}
	return content
}

var cachedTodayArticleCount response.CacheArticleCount

func GetTodayArticleCount() int64 {
	today := now.BeginningOfDay()
	if cachedTodayArticleCount.Day == today.Day() {
		return cachedTodayArticleCount.Count
	}

	cachedTodayArticleCount.Day = today.Day()
	cachedTodayArticleCount.Count = 0

	todayUnix := today.Unix()
	dao.DB.Model(&model.Archive{}).Where("created_time >= ? and created_time < ?", todayUnix, todayUnix+86400).Count(&cachedTodayArticleCount.Count)

	return cachedTodayArticleCount.Count
}

func UpdateTodayArticleCount(addNum int) {
	cachedTodayArticleCount.Count += int64(addNum)
}

func GetArticleTotalByKeywordId(id uint) int64 {
	var total int64
	dao.DB.Model(&model.Archive{}).Where("keyword_id = ?", id).Count(&total)

	return total
}

func checkArticleExists(originUrl, originTitle string) bool {
	var total int64
	dao.DB.Model(&model.Archive{}).Where("origin_url = ?", originUrl).Count(&total)
	if total > 0 {
		return true
	}
	dao.DB.Model(&model.Archive{}).Where("origin_title = ?", originTitle).Count(&total)
	if total > 0 {
		return true
	}

	return false
}
