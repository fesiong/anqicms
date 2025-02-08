package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/jinzhu/now"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"kandaoni.com/anqicms/response"
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

func (w *Website) GetUserCollectorSetting() config.CollectorJson {
	var collector config.CollectorJson
	value := w.GetSettingValue(CollectorSettingKey)
	if value == "" {
		return collector
	}

	if err := json.Unmarshal([]byte(value), &collector); err != nil {
		return collector
	}

	return collector
}

func (w *Website) SaveUserCollectorSetting(req config.CollectorJson, focus bool) error {
	collector := w.GetUserCollectorSetting()
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
		if req.AutoTranslate {
			collector.AutoTranslate = req.AutoTranslate
		}
		if len(req.ToLanguage) > 0 {
			collector.ToLanguage = req.ToLanguage
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

	_ = w.SaveSettingValue(CollectorSettingKey, collector)
	//重新读取配置
	w.LoadCollectorSetting(w.GetSettingValue(CollectorSettingKey))
	if collector.AutoCollect {
		go w.CollectArticles()
	}

	return nil
}

var runningCollectArticles = false

func (w *Website) CollectArticles() {
	if w.DB == nil {
		return
	}
	if !w.CollectorConfig.AutoCollect {
		return
	}
	if runningCollectArticles {
		return
	}
	runningCollectArticles = true
	defer func() {
		runningCollectArticles = false
	}()

	if w.CollectorConfig.StartHour > 0 && time.Now().Hour() < w.CollectorConfig.StartHour {
		return
	}

	if w.CollectorConfig.EndHour > 0 && time.Now().Hour() >= w.CollectorConfig.EndHour {
		return
	}

	// 如果采集的文章数量达到了设置的限制，则当天停止采集
	if w.GetTodayArticleCount(config.ArchiveFromCollect) > int64(w.CollectorConfig.DailyLimit) {
		return
	}

	lastId := uint(0)
	for {
		var keywords []*model.Keyword
		w.DB.Where("id > ? and last_time = 0", lastId).Order("id asc").Limit(10).Find(&keywords)
		if len(keywords) == 0 {
			break
		}
		lastId = keywords[len(keywords)-1].Id
		for i := 0; i < len(keywords); i++ {
			keyword := keywords[i]
			// 检查是否采集过
			if w.checkArticleExists(keyword.Title, "", "") {
				// 跳过这个关键词
				if keyword.ArticleCount == 0 {
					keyword.ArticleCount = 1
				}
				keyword.LastTime = time.Now().Unix()
				w.DB.Model(keyword).Select("article_count", "last_time").Updates(keyword)
				//log.Println("已存在于数据库", keyword.Title)
				continue
			}
			total, err := w.CollectArticlesByKeyword(*keyword, false)
			log.Printf("关键词：%s 采集了 %d 篇文章, %v", keyword.Title, total, err)
			// 达到数量了，退出
			if w.GetTodayArticleCount(config.ArchiveFromCollect) > int64(w.CollectorConfig.DailyLimit) {
				return
			}
			// 如果没有使用代理，则每个关键词都需要间隔30秒以上
			if w.Proxy == nil {
				time.Sleep(time.Duration(20+rand.Intn(20)) * time.Second)
			}
			if err != nil {
				// 采集出错了，多半是出验证码了，跳过该任务，等下次开始
				// 延时 10分钟以上
				// time.Sleep(time.Duration(10+rand.Intn(20)) * time.Minute)
				if w.Proxy == nil {
					break
				}
			}
		}
	}
}

func (w *Website) CollectArticlesByKeyword(keyword model.Keyword, focus bool) (total int, err error) {
	if w.CollectorConfig.CollectMode == config.CollectModeCombine {
		total, err = w.GenerateCombination(&keyword)
	} else {
		total, err = w.CollectArticleFromBaidu(&keyword, focus, 0)
	}

	if err != nil {
		return total, err
	}
	if total == 0 {
		return total, nil
	}

	keyword.ArticleCount = w.GetArticleTotalByKeywordId(keyword.Id)
	keyword.LastTime = time.Now().Unix()
	w.DB.Model(keyword).Select("article_count", "last_time").Updates(keyword)

	return total, nil
}

func (w *Website) SaveCollectArticle(archive *request.Archive, keyword *model.Keyword) error {
	//原始标题
	archive.OriginTitle = archive.Title

	if w.checkArticleExists(archive.OriginUrl, archive.OriginTitle, archive.Title) {
		//log.Println("已存在于数据库", archive.OriginTitle)
		return errors.New(w.Tr("AlreadyExistsInTheDatabase"))
	}

	archive.KeywordId = keyword.Id
	categoryId := keyword.CategoryId
	if categoryId == 0 {
		if len(w.CollectorConfig.CategoryIds) > 0 {
			categoryId = w.CollectorConfig.CategoryIds[rand.New(rand.NewSource(time.Now().UnixNano())).Intn(len(w.CollectorConfig.CategoryIds))]
		} else if w.CollectorConfig.CategoryId > 0 {
			categoryId = w.CollectorConfig.CategoryId
		}
		if categoryId == 0 {
			var category model.Category
			w.DB.Where("module_id = 1").Take(&category)
			w.CollectorConfig.CategoryIds = []uint{category.Id}
			categoryId = category.Id
		}
	}
	archive.CategoryId = categoryId
	//log.Println("draft:", w.CollectorConfig.SaveType)
	// 如果不是正常发布，则存到草稿
	isDraft := false
	if w.CollectorConfig.SaveType == 0 {
		isDraft = true
	}
	archive.Draft = isDraft
	res, err := w.SaveArchive(archive)
	if err != nil {
		log.Println("保存文章出错：", archive.Title, err.Error())
		return err
	}
	//文章计数
	w.UpdateTodayArticleCount(1)

	if w.CollectorConfig.AutoPseudo {
		// AI 改写
		_ = w.AnqiAiPseudoArticle(res, isDraft)
	}
	if w.CollectorConfig.AutoTranslate {
		// AI 翻译
		// 读取 data
		archiveData, err := w.GetArchiveDataById(res.Id)
		if err != nil {
			return nil
		}
		aiReq := &AnqiAiRequest{
			Title:      res.Title,
			Content:    archiveData.Content,
			ArticleId:  res.Id,
			Language:   w.CollectorConfig.Language,
			ToLanguage: w.CollectorConfig.ToLanguage,
			Async:      false, // 同步返回结果
		}
		result, err := w.AnqiTranslateString(aiReq)
		if err != nil {
			return nil
		}
		// 更新文档
		if result.Status == config.AiArticleStatusCompleted {
			res.Title = result.Title
			res.Description = library.ParseDescription(strings.ReplaceAll(library.StripTags(result.Content), "\n", " "))
			tx := w.DB
			if isDraft {
				tx = tx.Model(&model.ArchiveDraft{})
			} else {
				tx = tx.Model(&model.Archive{})
			}
			tx.Where("id = ?", res.Id).UpdateColumns(map[string]interface{}{
				"title":       res.Title,
				"description": res.Description,
			})
			// 再保存内容
			archiveData.Content = result.Content
			w.DB.Save(archiveData)
		}
		// 写入 plan
		_, _ = w.SaveAiArticlePlan(result, result.UseSelf)
	}

	return nil
}

func (w *Website) CollectArticleFromBaidu(keyword *model.Keyword, focus bool, retry int) (int, error) {
	collectUrl := fmt.Sprintf("https://www.baidu.com/s?wd=%s&tn=json&rn=50&pn=10", keyword.Title)
	if w.CollectorConfig.FromWebsite != "" {
		collectUrl = fmt.Sprintf("https://www.baidu.com/s?wd=inurl%%3A%s%%20%s&tn=json&rn=50&pn=10", keyword.Title, w.CollectorConfig.FromWebsite)
	}
	var proxyIp string
	if w.Proxy != nil {
		proxyIp = w.Proxy.GetIP()
	}
	resp, err := library.Request(collectUrl, &library.Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			"Referer":         "https://www.baidu.com",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		},
		Proxy: proxyIp,
	})
	if err != nil {
		if proxyIp != "" {
			w.Proxy.RemoveIP(proxyIp)
			// 重试2次
			if retry < 2 {
				return w.CollectArticleFromBaidu(keyword, focus, retry+1)
			}
		}
		return 0, err
	}

	var total int
	links := w.ParseBaiduJson(resp.Body)
	// 如果是使用了代理，则使用并发处理，最大10并发
	chNum := 1
	if w.Proxy != nil {
		chNum = w.Proxy.cfg.Concurrent * 2
		if chNum > 10 {
			chNum = 10
		}
	}
	wg := sync.WaitGroup{}
	ch := make(chan int, chNum)
	defer close(ch)
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

		ch <- 1
		wg.Add(1)
		go func(wl *response.WebLink) {
			defer func() {
				<-ch
				wg.Done()
			}()
			archive, err := w.CollectSingleArticle(wl, keyword, 0)
			if err == nil {
				err = w.SaveCollectArticle(archive, keyword)

				if err == nil {
					total++
					if !focus && proxyIp == "" {
						//如果没有使用代理，根据设置的时间，休息一定秒数
						time.Sleep(15 * time.Second)
					}
				}
			}
		}(link)
	}
	wg.Wait()

	return total, nil
}

func (w *Website) CollectSingleArticle(link *response.WebLink, keyword *model.Keyword, retry int) (*request.Archive, error) {
	//百度的不使用 chromedp
	var proxyIp string
	if w.Proxy != nil {
		proxyIp = w.Proxy.GetIP()
	}
	resp, err := library.Request(link.Url, &library.Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			"Referer": fmt.Sprintf("https://www.baidu.com/s?wd=%s", url.QueryEscape(keyword.Title)),
		},
		Proxy: proxyIp,
	})
	if err != nil {
		//log.Println("请求出错：", link.Url, err.Error())
		if proxyIp != "" {
			w.Proxy.RemoveIP(proxyIp)
			// 重试2次
			if retry < 2 {
				return w.CollectSingleArticle(link, keyword, retry+1)
			}
		}
		return nil, err
	}

	archive := &request.Archive{
		OriginUrl:   link.Url,
		ContentText: resp.Body,
	}
	_ = w.ParseArticleDetail(archive)
	if len(archive.Content) == 0 {
		//log.Println("链接无文章", archive.OriginUrl)
		return nil, errors.New(w.Tr("LinkHasNoArticle"))
	}
	if archive.Title == "" {
		//log.Println("链接无文章", archive.OriginUrl)
		return nil, errors.New(w.Tr("LinkHasNoArticle"))
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
		return nil, errors.New(w.Tr("GarbledCode"))
	}

	// 替换图片
	if w.CollectorConfig.InsertImage != config.CollectImageRetain {
		re, _ := regexp.Compile(`(?i)<img\s.*?>`)
		archive.Content = RemoveTags(re.ReplaceAllString(archive.Content, ""))
	}
	if w.CollectorConfig.InsertImage == config.CollectImageInsert && len(w.CollectorConfig.Images) > 0 {
		rd := rand.New(rand.NewSource(time.Now().UnixNano()))
		img := w.CollectorConfig.Images[rd.Intn(len(w.CollectorConfig.Images))]
		content := strings.SplitAfter(archive.Content, ">")

		index := len(content) / 3
		content = append(content, "")
		copy(content[index+1:], content[index:])
		content[index] = "<img src='" + img + "' alt='" + archive.Title + "'/>"
		archive.Content = strings.Join(content, "")
	}
	if w.CollectorConfig.InsertImage == config.CollectImageCategory {
		// 根据分类每次只取其中一张
		img := w.GetRandImageFromCategory(w.CollectorConfig.ImageCategoryId, keyword.Title)
		if len(img) > 0 {
			content := strings.SplitAfter(archive.Content, ">")
			index := len(content) / 3
			content = append(content, "")
			copy(content[index+1:], content[index:])
			content[index] = "<img src='" + img + "' alt='" + archive.Title + "'/>"
			archive.Content = strings.Join(content, "")
		}
	}
	//log.Println(archive.Title, len(archive.Content), archive.OriginUrl)

	return archive, nil
}

func (w *Website) ParseBaiduJson(content string) []*response.WebLink {
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

func (w *Website) ParseArticleDetail(archive *request.Archive) error {
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
		w.ParseBaikeDetail(archive, doc)
	} else if strings.Contains(archive.OriginUrl, "mp.weixin.qq.com") {
		w.ParseWeixinDetail(archive, doc)
	} else if strings.Contains(archive.OriginUrl, "zhihu.com") {
		w.ParseZhihuDetail(archive, doc)
	} else if strings.Contains(archive.OriginUrl, "toutiao.com") {
		w.ParseToutiaoDetail(archive, doc)
	} else {
		w.ParseNormalDetail(archive, doc)
	}

	archive.Content = w.ReplaceContentFromConfig(archive.Content, w.CollectorConfig.ContentReplace)

	return nil
}

func (w *Website) ParseBaikeDetail(archive *request.Archive, doc *goquery.Document) {
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
		w.ParseNormalDetail(archive, doc)
	}
}

func (w *Website) ParseWeixinDetail(archive *request.Archive, doc *goquery.Document) {
	//获取标题
	archive.Title = strings.TrimSpace(doc.Find("h1").Eq(0).Text())

	contentList := doc.Find("#js_content")

	content, _ := w.CleanTags(contentList)

	archive.Content = content
	//如果获取不到内容，则 fallback 到normal
	if archive.Content == "" {
		w.ParseNormalDetail(archive, doc)
	}
}

func (w *Website) ParseZhihuDetail(archive *request.Archive, doc *goquery.Document) {
	//获取标题
	archive.Title = strings.TrimSpace(doc.Find("h1").Eq(0).Text())

	var content, planText string
	doc.Find(".RichContent-inner .RichText,.Post-RichTextContainer .RichText").Each(func(i int, item *goquery.Selection) {
		tmpContent, tmpText := w.CleanTags(item)
		if len(tmpText) > len(planText) {
			content = tmpContent
			planText = tmpText
		}
	})

	archive.Content = content
	//如果获取不到内容，则 fallback 到normal
	if archive.Content == "" {
		w.ParseNormalDetail(archive, doc)
	}
}

func (w *Website) ParseToutiaoDetail(archive *request.Archive, doc *goquery.Document) {
	//获取标题
	archive.Title = strings.TrimSpace(doc.Find("h1").Eq(0).Text())

	contentList := doc.Find(".article-content article")

	content, _ := w.CleanTags(contentList)

	archive.Content = content
	//如果获取不到内容，则 fallback 到normal
	if archive.Content == "" {
		w.ParseNormalDetail(archive, doc)
	}
}

func (w *Website) ParseNormalDetail(archive *request.Archive, doc *goquery.Document) {
	w.ParseLinking(doc, archive.OriginUrl)
	title := w.ParseArticleTitle(doc)

	otherItems := doc.Find("input,textarea,select,radio,form,button,header,aside,footer,.footer,noscript,meta,nav,hr.modal")
	if otherItems.Length() > 0 {
		otherItems.Remove()
	}

	//根据标题判断是否是英文，如果是英文，则采用英文的计数
	isEnglish := CheckContentIsEnglish(title)
	if title != "" {
		archive.Title = title
	}

	if utf8.RuneCountInString(archive.Title) < w.CollectorConfig.TitleMinLength || HasContain(archive.Title, w.CollectorConfig.TitleExclude) || HasPrefix(archive.Title, w.CollectorConfig.TitleExcludePrefix) || HasSuffix(archive.Title, w.CollectorConfig.TitleExcludeSuffix) {
		archive.Title = ""
		//跳过这篇文章
		return
	}

	//尝试获取正文内容
	content, planText, _, _ := w.ParseArticleContent(doc.Find("body"), 0, isEnglish)
	if content != nil {
		archive.Content, planText = w.CleanTags(content)
		planLen := utf8.RuneCountInString(planText)
		if isEnglish {
			//英语使用空格来计算词数
			planLen = len(strings.Split(planText, " "))
		}
		if planLen < w.CollectorConfig.ContentMinLength {
			//小于指定次数的，直接抛弃了
			archive.Content = ""
		}
	}
}

func (w *Website) ParseArticleTitle(doc *goquery.Document) string {
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
			if textLen >= w.CollectorConfig.TitleMinLength && textLen > utf8.RuneCountInString(title) && !HasContain(text, w.CollectorConfig.TitleExclude) && !HasPrefix(text, w.CollectorConfig.TitleExcludePrefix) && !HasSuffix(text, w.CollectorConfig.TitleExcludeSuffix) {
				title = text
			}
		}
	}
	if title == "" {
		//获取 政府网站的 <meta name='ArticleTitle' content='西城法院出台案件在线办理操作指南'>
		text, exist := doc.Find("meta[name=ArticleTitle]").Attr("content")
		if exist {
			text = strings.TrimSpace(text)
			if utf8.RuneCountInString(text) >= w.CollectorConfig.TitleMinLength && !HasContain(text, w.CollectorConfig.TitleExclude) && !HasPrefix(text, w.CollectorConfig.TitleExcludePrefix) && !HasSuffix(text, w.CollectorConfig.TitleExcludeSuffix) {
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
		if utf8.RuneCountInString(text) >= w.CollectorConfig.TitleMinLength && !HasContain(text, w.CollectorConfig.TitleExclude) && !HasPrefix(text, w.CollectorConfig.TitleExcludePrefix) && !HasSuffix(text, w.CollectorConfig.TitleExcludeSuffix) {
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
				if textLen >= w.CollectorConfig.TitleMinLength && textLen > utf8.RuneCountInString(title) && !HasContain(text, w.CollectorConfig.TitleExclude) && !HasPrefix(text, w.CollectorConfig.TitleExcludePrefix) && !HasSuffix(text, w.CollectorConfig.TitleExcludeSuffix) {
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
				if textLen >= w.CollectorConfig.TitleMinLength && textLen > utf8.RuneCountInString(title) && !HasContain(text, w.CollectorConfig.TitleExclude) && !HasPrefix(text, w.CollectorConfig.TitleExcludePrefix) && !HasSuffix(text, w.CollectorConfig.TitleExcludeSuffix) {
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

func (w *Website) ParseArticleContent(nodeItem *goquery.Selection, deep int, isEnglish bool) (*goquery.Selection, string, int, int) {
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
	if planLen < w.CollectorConfig.ContentMinLength {
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
		tmpContent, tmpText, tmpDeep, tmpPCount := w.ParseArticleContent(item, deep+1, isEnglish)
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
	if HasContain(planText, w.CollectorConfig.ContentExclude) {
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

func (w *Website) CleanTags(clonedItem *goquery.Selection) (string, string) {
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
		if HasContain(innerItem.Text(), w.CollectorConfig.ContentExcludeLine) {
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

func (w *Website) ParseLinking(htmlDom *goquery.Document, baseUrl string) {
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
	contents := []rune(content)
	for i := 0; i < len(contents); i++ {
		if contents[i] < 2000 {
			enCount++
		}
	}
	if float64(enCount) > float64(len(contents))*0.9 {
		return true
	}

	return false
}

func (w *Website) ReplaceArticles() {
	startId := int64(0)
	var archives []*model.Archive
	for {
		tx := w.DB.Model(&model.Archive{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&archives)
		if len(archives) == 0 {
			break
		}
		startId = archives[len(archives)-1].Id
		for _, archive := range archives {
			var archiveData model.ArchiveData
			title := w.ReplaceContentFromConfig(archive.Title, w.CollectorConfig.ContentReplace)
			w.DB.Where("id = ?", archive.Id).Take(&archiveData)
			content := w.ReplaceContentFromConfig(archiveData.Content, w.CollectorConfig.ContentReplace)

			//替换完了
			hasReplace := false
			if title != archive.Title {
				hasReplace = true
				w.DB.Model(archive).UpdateColumn("title", title)
			}
			if content != archiveData.Content {
				hasReplace = true
				archiveData.Content = content
				w.DB.Model(&archiveData).UpdateColumns(archiveData)
			}
			if hasReplace {
				log.Println("替换文章：" + archive.Title)
			}
		}
	}
	// 草稿
	startId = 0
	var archiveDrafts []*model.ArchiveDraft
	for {
		tx := w.DB.Model(&model.ArchiveDraft{})
		tx.Where("id > ?", startId).Order("id asc").Limit(1000).Find(&archiveDrafts)
		if len(archiveDrafts) == 0 {
			break
		}
		startId = archiveDrafts[len(archiveDrafts)-1].Id
		for _, archive := range archiveDrafts {
			var archiveData model.ArchiveData
			title := w.ReplaceContentFromConfig(archive.Title, w.CollectorConfig.ContentReplace)
			w.DB.Where("id = ?", archive.Id).Take(&archiveData)
			content := w.ReplaceContentFromConfig(archiveData.Content, w.CollectorConfig.ContentReplace)

			//替换完了
			hasReplace := false
			if title != archive.Title {
				hasReplace = true
				w.DB.Model(archive).UpdateColumn("title", title)
			}
			if content != archiveData.Content {
				hasReplace = true
				archiveData.Content = content
				w.DB.Model(&archiveData).UpdateColumns(archiveData)
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

	if !strings.Contains(content, "</") {
		// 如果不是html，则直接返回
		if planText == "" {
			return content
		}
		return planText
	}
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "<") {
		content = "<>" + content + "</>"
	}

	if planText == "" {
		var contents []string
		reg, _ := regexp.Compile("(?s)>(.*?)<")
		matches := reg.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			text := strings.TrimSpace(strings.ReplaceAll(match[1], "\n", " "))
			if strings.HasPrefix(text, "<") {
				continue
			}
			if len(text) > 0 {
				contents = append(contents, text)
			}
		}

		return strings.Join(contents, "\n")
	} else {
		contents := strings.Split(planText, "\n")
		index := 0
		reg, _ := regexp.Compile("(?s)>(.*?)<")
		content = reg.ReplaceAllStringFunc(content, func(s string) string {
			match := reg.FindStringSubmatch(s)
			if len(match) < 2 {
				return s
			}
			text := strings.TrimSpace(match[1])
			if strings.HasPrefix(text, "<") {
				return s
			}
			if len(text) > 0 {
				if index < len(contents) {
					s = strings.Replace(s, text, contents[index], 1)
					index++
				}
			}
			return s
		})
		// trim content = "<>" + content + "</>"
		content = strings.TrimPrefix(strings.TrimSuffix(content, "</>"), "<>")

		return content
	}
}

func (w *Website) GetTodayArticleCount(from int) int64 {
	today := now.BeginningOfDay()
	if w.cachedTodayArticleCount.Day == today.Day() {
		if from == config.ArchiveFromAi {
			return w.cachedTodayArticleCount.AiGenerateCount
		}
		return w.cachedTodayArticleCount.CollectCount
	}

	w.cachedTodayArticleCount.Day = today.Day()
	w.cachedTodayArticleCount.CollectCount = 0
	w.cachedTodayArticleCount.AiGenerateCount = 0

	todayUnix := today.Unix()
	var collectCount int64
	var aiCount int64
	w.DB.Model(&model.Archive{}).Where("`origin_id` = ? and created_time >= ? and created_time < ?", config.ArchiveFromCollect, todayUnix, todayUnix+86400).Count(&w.cachedTodayArticleCount.CollectCount)
	w.DB.Model(&model.ArchiveDraft{}).Where("`origin_id` = ? and created_time >= ? and created_time < ?", config.ArchiveFromCollect, todayUnix, todayUnix+86400).Count(&collectCount)
	w.cachedTodayArticleCount.CollectCount += collectCount
	w.DB.Model(&model.Archive{}).Where("`origin_id` = ? and created_time >= ? and created_time < ?", config.ArchiveFromAi, todayUnix, todayUnix+86400).Count(&w.cachedTodayArticleCount.AiGenerateCount)
	w.DB.Model(&model.ArchiveDraft{}).Where("`origin_id` = ? and created_time >= ? and created_time < ?", config.ArchiveFromAi, todayUnix, todayUnix+86400).Count(&aiCount)
	w.cachedTodayArticleCount.AiGenerateCount += aiCount

	if from == config.ArchiveFromAi {
		return w.cachedTodayArticleCount.AiGenerateCount
	}
	return w.cachedTodayArticleCount.CollectCount
}

func (w *Website) UpdateTodayArticleCount(addNum int) {
	w.cachedTodayArticleCount.CollectCount += int64(addNum)
}

func (w *Website) GetArticleTotalByKeywordId(id uint) int64 {
	var total int64
	w.DB.Model(&model.Archive{}).Where("keyword_id = ?", id).Count(&total)
	var total2 int64
	w.DB.Model(&model.ArchiveDraft{}).Where("keyword_id = ?", id).Count(&total2)
	return total + total2
}

func (w *Website) checkArticleExists(originUrl, originTitle, title string) bool {
	var total int64
	if len(originUrl) > 0 {
		if utf8.RuneCountInString(originUrl) > 190 {
			originUrl = string([]rune(originUrl)[:190])
		}
		w.DB.Model(&model.Archive{}).Where("origin_url = ?", originUrl).Count(&total)
		if total > 0 {
			return true
		}
		w.DB.Model(&model.ArchiveDraft{}).Where("origin_url = ?", originUrl).Count(&total)
		if total > 0 {
			return true
		}
	}
	if len(originTitle) > 0 {
		if utf8.RuneCountInString(originTitle) > 190 {
			originTitle = string([]rune(originTitle)[:190])
		}
		w.DB.Model(&model.Archive{}).Where("origin_title = ?", originTitle).Count(&total)
		if total > 0 {
			return true
		}
		w.DB.Model(&model.ArchiveDraft{}).Where("origin_title = ?", originTitle).Count(&total)
		if total > 0 {
			return true
		}
	}
	if len(title) > 0 {
		w.DB.Model(&model.Archive{}).Where("`title` = ?", title).Count(&total)
		if total > 0 {
			return true
		}
		w.DB.Model(&model.ArchiveDraft{}).Where("`title` = ?", title).Count(&total)
		if total > 0 {
			return true
		}
	}

	return false
}
