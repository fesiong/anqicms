package provider

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"kandaoni.com/anqicms/request"
	"log"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type CombinationItem struct {
	w           *Website
	Title       string
	Description string
	Link        string
	Content     string
	Image       string
}

func (w *Website) getCombinationEnginLink(keyword *model.Keyword) string {
	// default bing
	var link string
	switch w.CollectorConfig.FromEngine {
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
		if strings.Contains(w.KeywordConfig.FromWebsite, "%s") {
			link = fmt.Sprintf(w.KeywordConfig.FromWebsite, url.QueryEscape(keyword.Title))
			break
		}
	//case config.EnginBingCn:
	default:
		link = fmt.Sprintf("https://cn.bing.com/search?q=%s", url.QueryEscape(keyword.Title))
		break
	}

	return link
}

func (w *Website) GenerateCombination(keyword *model.Keyword) (int, error) {
	// 检查是否采集过
	if w.checkArticleExists(keyword.Title, "") {
		//log.Println("已存在于数据库", keyword.Title)
		return 1, nil
	}
	result, err := w.collectCombinationMaterials(keyword)
	if err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, errors.New("错误，可能出现验证码了")
	}
	if len(result) < 5 {
		log.Println(fmt.Sprintf("有效内容不足: %d", len(result)))
		return 0, nil
	}
	var title = keyword.Title
	var content = make([]string, 0, len(result)*2+3)
	num := 0
	for i := range result {
		if utf8.RuneCountInString(title) < 10 {
			title = result[i].Title
		}
		content = append(content, "<h3>"+result[i].Title+"</h3>")
		text := result[i].Description
		if result[i].Content != "" {
			text = result[i].Content
		}
		if w.CollectorConfig.InsertImage && result[i].Image != "" && num < 3 && len(w.CollectorConfig.Images) == 0 {
			content = append(content, "<img src='"+result[i].Image+"'/>")
			num++
		}
		content = append(content, "<p>"+text+"</p>")
	}
	if w.CollectorConfig.InsertImage && num == 0 && len(w.CollectorConfig.Images) > 0 {
		img := w.CollectorConfig.Images[rand.Intn(len(w.CollectorConfig.Images))]
		index := 2 + rand.Intn(len(content)-3)
		content = append(content, "")
		copy(content[index+1:], content[index:])
		content[index] = "<img src='" + img + "'/>"
		num++
	}

	if w.CollectorConfig.CategoryId == 0 {
		var category model.Category
		w.DB.Where("module_id = 1").Take(&category)
		w.CollectorConfig.CategoryId = category.Id
	}

	archive := request.Archive{
		Title:      title,
		ModuleId:   0,
		CategoryId: w.CollectorConfig.CategoryId,
		Keywords:   keyword.Title,
		Content:    strings.Join(content, "\n"),
		KeywordId:  keyword.Id,
		OriginUrl:  keyword.Title,
	}
	if w.CollectorConfig.SaveType == 0 {
		archive.Draft = true
	} else {
		archive.Draft = false
	}
	res, err := w.SaveArchive(&archive)
	if err != nil {
		log.Println("保存组合文章出错：", archive.Title, err.Error())
		return 0, nil
	}
	log.Println(res.Id, res.Title)

	return 1, nil
}

func (w *Website) collectCombinationMaterials(keyword *model.Keyword) ([]CombinationItem, error) {
	link := w.getCombinationEnginLink(keyword)

	var ok bool
	var exists = map[string]struct{}{}
	var err error
	var result []CombinationItem
	page := 1
	for page <= 4 {
		var tmpData []CombinationItem
		tmpData, link, err = w.getEnginData(link, keyword.Title, page)
		if err != nil {
			break
		}
		for i := range tmpData {
			if _, ok = exists[tmpData[i].Description]; ok {
				continue
			}
			//if _, ok = exists[tmpData[i].Title]; ok {
			//	continue
			//}
			exists[tmpData[i].Description] = struct{}{}
			//exists[tmpData[i].Title] = struct{}{}
			result = append(result, tmpData[i])
		}
		if len(result) > 10 {
			break
		}
		if link == "" {
			break
		}
		page++
		// 每一页都需要等待10秒以上
		time.Sleep(time.Duration(10+rand.Intn(10)) * time.Second)
	}

	return result, nil
}

func (w *Website) getEnginData(link, word string, page int) ([]CombinationItem, string, error) {
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
		return nil, "", err
	}

	htmlR := strings.NewReader(resp.Body)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err != nil {
		return nil, "", err
	}
	doc.Find("script,style,head").Remove()

	result, err := w.parseSections(doc.Find("body"), word, link)
	// 尝试获取下一页链接
	var nextLink string
	aLinks := doc.Find("a")
	for i2 := range aLinks.Nodes {
		child := aLinks.Eq(i2)
		aText := strings.TrimSpace(child.Text())
		if strings.Contains(aText, "下一页") || strings.Contains(aText, "Next") || strings.EqualFold(aText, strconv.Itoa(page+1)) {
			nextLink = child.AttrOr("href", "")
			if strings.HasPrefix(nextLink, "//") {
				nextLink = "http:" + nextLink
			} else if !strings.HasPrefix(nextLink, "http") {
				parsedUrl, _ := url.Parse(link)
				parsedUrl.Path = "/"
				parsedUrl.RawQuery = ""
				parsedUrl.Fragment = ""
				baseUrl := parsedUrl.String()
				nextLink = baseUrl + strings.TrimLeft(nextLink, "/")
			}
			break
		}
	}

	return result, nextLink, nil
}

func (w *Website) parseSections(sel *goquery.Selection, word, sourceLink string) ([]CombinationItem, error) {
	var list *goquery.Selection

	if strings.Contains(sourceLink, "www.so.com") {
		list = sel.Find("ul.result")
		list.Find("span,.gray-info,.g-linkinfo").Remove()
	} else if strings.Contains(sourceLink, "sogou.com") {
		list = sel.Find("div.results")
		list.Find(".str-blue,.citeurl").Remove()
	} else if strings.Contains(sourceLink, "baidu.com") {
		list = sel.Find("#content_left")
		list.Find(".c-color-gray,.c-color-gray2,.OP_LOG_LINK").Remove()
	} else if strings.Contains(sourceLink, "bing.com") {
		list = sel.Find("#b_results")
		list.Find(".b_attribution,.algoSlug_icon,.news_dt").Remove()
	} else if strings.Contains(sourceLink, "google.com") {
		list = sel.Find("#search")
	}
	if list.Length() == 0 {
		list = sel
	}
	list.Find("span,em,i,strong,b,br").Contents().Unwrap()
	list = filterList(list)
	if list == nil || list.Nodes == nil {
		return nil, errors.New(w.Lang("没有可用对象"))
	}
	list.AddClass("isTop")
	parsedUrl, err := url.Parse(sourceLink)
	if err != nil {
		return nil, err
	}
	parsedUrl.Path = "/"
	parsedUrl.RawQuery = ""
	parsedUrl.Fragment = ""
	baseUrl := parsedUrl.String()

	var exists = map[string]struct{}{}
	var links []CombinationItem
	list.Find("a").Each(func(i int, item *goquery.Selection) {
		title := strings.TrimSpace(item.Text())
		title = strings.Trim(title, "...…")
		index := strings.IndexAny(title, "|-_?？.")
		if index > 0 {
			title = title[:index]
			title = strings.TrimSpace(title)
		}
		if strings.Contains(title, "   ") {
			return
		}
		if !w.ContainKeywords(title, word) {
			return
		}
		//if _, ok := exists[title]; ok {
		//	return
		//}

		isEnglish := CheckContentIsEnglish(title)
		if w.KeywordConfig.Language == config.LanguageEn && !isEnglish {
			return
		}
		if w.KeywordConfig.Language == config.LanguageZh && isEnglish {
			return
		}

		href, ok := item.Attr("href")
		if !ok {
			return
		}
		if strings.HasPrefix(href, "//") {
			href = "http:" + href
		} else if !strings.HasPrefix(href, "http") {
			href = baseUrl + strings.TrimLeft(href, "/")
		}
		if _, ok = exists[href]; ok {
			return
		}

		// check if it contains ...
		var parentItem *goquery.Selection
		if item.ParentsFiltered("li").Length() > 0 {
			parentItem = item.ParentsFiltered("li")
		} else if item.ParentsFiltered("div").Length() > 0 {
			// 尝试
			parentItem = item.Parent()
			for {
				if parentItem.Parent().HasClass("isTop") {
					break
				}
				parentItem = parentItem.Parent()
			}
		}
		if parentItem == nil {
			return
		}
		parentItem.Find("a").Remove()
		itemText := parentItem.Text()
		var desc string
		if !strings.Contains(itemText, "...") && !strings.Contains(itemText, "…") {
			if strings.Contains(itemText, "广告") || strings.Contains(itemText, "Ad") {
				return
			}
			// invalid
			children := parentItem.Find("div,p")
			enough := false
			for i2 := range children.Nodes {
				child := children.Eq(i2)
				childrenText := strings.TrimSpace(child.Children().Text())
				childText := strings.TrimSpace(child.Text())
				if childrenText == childrenText && utf8.RuneCountInString(childText) > 60 {
					desc = child.Text()
					enough = true
					break
				}
			}
			if !enough {
				return
			}
		}
		if desc == "" {
			children := parentItem.Find("div,p")
			for i2 := range children.Nodes {
				child := children.Eq(i2)
				childText := strings.TrimSpace(child.Text())
				childrenText := strings.TrimSpace(child.Children().Text())
				if childrenText == childrenText && (strings.Contains(childText, "...") || strings.Contains(childText, "…")) {
					desc = childText
					break
				}
			}
		}
		if strings.Contains(desc, "·") {
			desc = desc[strings.LastIndex(desc, "·"):]
		}
		if strings.Contains(desc, "…") {
			desc = desc[:strings.LastIndex(desc, "…")]
		}
		if strings.Contains(desc, "...") {
			desc = desc[:strings.LastIndex(desc, "...")]
		}

		desc = strings.ReplaceAll(desc, "  ", "\n")
		var tmpDesc string
		descs := strings.Split(desc, "\n")
		for _, v := range descs {
			v = strings.TrimSpace(v)
			if len(v) > len(tmpDesc) {
				tmpDesc = v
			}
		}
		desc = tmpDesc
		if _, ok = exists[desc]; ok {
			return
		}
		if utf8.RuneCountInString(desc) < 50 || HasContain(desc, w.CollectorConfig.ContentExclude) {
			return
		}
		w.ReplaceContentFromConfig(desc, w.CollectorConfig.ContentReplace)
		//exists[title] = struct{}{}
		exists[href] = struct{}{}
		exists[desc] = struct{}{}
		links = append(links, CombinationItem{
			w:           w,
			Title:       title,
			Description: desc,
			Link:        href,
			Content:     "",
			Image:       "",
		})
	})

	//for i := range links {
	//	links[i].GetSingleLinkData(sourceLink)
	//	ReplaceContentFromConfig(links[i].Content, config.CombinationConfig.ContentReplace)
	//}

	return links, nil
}

func filterList(list *goquery.Selection) *goquery.Selection {
	if list.ChildrenFiltered("div,li").Length() >= 10 {
		return list
	}
	children := list.Children()
	for i2 := range children.Nodes {
		child := children.Eq(i2)
		tmp := filterList(child)
		if tmp != nil {
			return tmp
		}
	}

	return nil
}

func (ci *CombinationItem) GetSingleLinkData(sourceLink string) error {
	// 取其中一段
	runeText := []rune(ci.Description)
	runeLen := len(runeText)
	if runeLen < 50 {
		return errors.New(ci.w.Lang("不符合条件"))
	}
	subText := string(runeText[runeLen/2-5 : runeLen/2+5])

	resp, err := library.Request(ci.Link, &library.Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			"Referer":         sourceLink,
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		},
	})
	if err != nil {
		return err
	}
	htmlR := strings.NewReader(resp.Body)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err != nil {
		return err
	}

	doc.Find("script,style,head").Remove()
	doc.Find("span,em,strong,b,br").Contents().Unwrap()
	//根据标题判断是否是英文，如果是英文，则采用英文的计数
	isEnglish := CheckContentIsEnglish(ci.Title)
	//尝试获取正文内容
	content, _, _, _ := ci.w.ParseArticleContent(doc.Find("body"), 0, isEnglish)
	if content != nil {
		text, _ := ci.w.CleanTags(content)
		htmlR = strings.NewReader(text)
		doc, err = goquery.NewDocumentFromReader(htmlR)
		if err != nil {
			return err
		}
		// 获取内容
		children := doc.Find("p,div")
		for i2 := range children.Nodes {
			child := children.Eq(i2)
			text2 := strings.ReplaceAll(strings.TrimSpace(child.Text()), "  ", "\n")
			var tmpText string
			descs := strings.Split(text2, "\n")
			for _, v := range descs {
				v = strings.TrimSpace(v)
				if len(v) > len(tmpText) {
					tmpText = v
				}
			}
			if utf8.RuneCountInString(tmpText) >= 30 && strings.Contains(text2, subText) {
				// 符合要求
				ci.Content = tmpText
				break
			}
		}
		// 尝试获取图片，第一张
		ci.Image = doc.Find("img").Eq(0).AttrOr("src", "")
		if strings.HasPrefix(ci.Image, "//") {
			ci.Image = "http:" + ci.Image
		} else if !strings.HasPrefix(ci.Image, "http") {
			parsedUrl, err := url.Parse(ci.Link)
			if err != nil {
				return err
			}
			parsedUrl.Path = "/"
			parsedUrl.RawQuery = ""
			parsedUrl.Fragment = ""
			baseUrl := parsedUrl.String()
			ci.Image = baseUrl + strings.TrimLeft(ci.Image, "/")
		}
	}

	return nil
}
