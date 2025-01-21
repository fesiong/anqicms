package provider

import (
	"encoding/json"
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
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type WebLink struct {
	Name      string `json:"name"`
	Url       string `json:"url"`
	OriginUrl string `json:"origin_url"`
	Content   string `json:"content"`
}

type ToutiaoJson struct {
	Dom string `json:"dom"`
}

var combineIndex = 0

// 如果出现验证码，则对应的休息15分钟
var baiduForbid = false
var soForbid = false
var sogouForbid = false

func (w *Website) GenerateCombination(keyword *model.Keyword) (int, error) {
	// 检查是否采集过
	if w.checkArticleExists(keyword.Title, "", "") {
		//log.Println("已存在于数据库", keyword.Title)
		return 1, nil
	}
	// 检查是否已经有素材
	materials := w.GetMaterialsByKeyword(keyword.Title)
	if len(materials) < 3 {
		result, err := w.collectCombinationMaterials(keyword)
		if err != nil {
			return 0, err
		}
		materials = append(result, materials...)
	}
	if len(materials) == 0 {
		return 0, errors.New(w.Tr("ErrorAVerificationCodeMayAppear"))
	}
	if len(materials) < 3 {
		log.Println(fmt.Sprintf("有效内容不足: %d", len(materials)))
		return 0, nil
	}
	var title = keyword.Title
	var content = make([]string, 0, len(materials)*2+3)
	for i := range materials {
		if utf8.RuneCountInString(title) < 10 {
			title = materials[i].Title
		}
		content = append(content, "<h3>"+materials[i].Title+"</h3>")
		text := materials[i].Content
		if w.CollectorConfig.InsertImage != config.CollectImageRetain {
			re, _ := regexp.Compile(`(?i)<img\s.*?>`)
			text = RemoveTags(re.ReplaceAllString(text, ""))
		}
		content = append(content, "<p>"+text+"</p>")

	}
	if w.CollectorConfig.InsertImage == config.CollectImageInsert && len(w.CollectorConfig.Images) > 0 {
		rd := rand.New(rand.NewSource(time.Now().UnixNano()))
		img := w.CollectorConfig.Images[rd.Intn(len(w.CollectorConfig.Images))]
		index := len(content) / 3
		content = append(content, "")
		copy(content[index+1:], content[index:])
		content[index] = "<img src='" + img + "' alt='" + title + "' />"
	}
	if w.CollectorConfig.InsertImage == config.CollectImageCategory {
		// 根据分类每次只取其中一张
		img := w.GetRandImageFromCategory(w.CollectorConfig.ImageCategoryId, keyword.Title)
		if len(img) > 0 {
			index := len(content) / 3
			content = append(content, "")
			copy(content[index+1:], content[index:])
			content[index] = "<img src='" + img + "' alt='" + title + "'/>"
		}
	}
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

	archive := request.Archive{
		Title:      title,
		ModuleId:   0,
		CategoryId: categoryId,
		Keywords:   keyword.Title,
		Content:    strings.Join(content, "\n"),
		KeywordId:  keyword.Id,
		OriginUrl:  keyword.Title,
	}
	isDraft := false
	if w.CollectorConfig.SaveType == 0 {
		isDraft = true
	}
	archive.Draft = isDraft
	// 保存前再检查一次
	if w.checkArticleExists(keyword.Title, "", archive.Title) {
		return 1, nil
	}
	res, err := w.SaveArchive(&archive)
	if err != nil {
		log.Println("保存组合文章出错：", archive.Title, err.Error())
		return 0, nil
	}
	log.Println(res.Id, res.Title)
	if w.CollectorConfig.AutoPseudo {
		// AI 改写
		_ = w.AnqiAiPseudoArticle(res, isDraft)
	}
	if w.CollectorConfig.AutoTranslate {
		// AI 改写
		// 读取 data
		archiveData, err := w.GetArchiveDataById(res.Id)
		if err != nil {
			return 1, nil
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
			return 1, nil
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

	return 1, nil
}

func (w *Website) GetCombinationArticle(keyword *model.Keyword) (*request.Archive, error) {
	// 检查是否已经有素材
	materials := w.GetMaterialsByKeyword(keyword.Title)
	if len(materials) < 3 {
		result, err := w.collectCombinationMaterials(keyword)
		if err != nil {
			return nil, err
		}
		materials = append(result, materials...)
	}
	if len(materials) == 0 {
		return nil, errors.New(w.Tr("ErrorAVerificationCodeMayAppear"))
	}
	if len(materials) < 3 {
		log.Println(fmt.Sprintf("有效内容不足: %d", len(materials)))
		return nil, errors.New(w.Tr("InsufficientValidContent"))
	}
	var title = keyword.Title
	var content = make([]string, 0, len(materials)*2+3)
	for i := range materials {
		if utf8.RuneCountInString(title) < 10 {
			title = materials[i].Title
		}
		content = append(content, "<h3>"+materials[i].Title+"</h3>")
		text := materials[i].Content
		if w.CollectorConfig.InsertImage != config.CollectImageRetain {
			re, _ := regexp.Compile(`(?i)<img\s.*?>`)
			text = RemoveTags(re.ReplaceAllString(text, ""))
		}
		content = append(content, "<p>"+text+"</p>")
	}
	if w.CollectorConfig.InsertImage == config.CollectImageInsert && len(w.CollectorConfig.Images) > 0 {
		randItem := rand.New(rand.NewSource(time.Now().UnixNano()))
		img := w.CollectorConfig.Images[randItem.Intn(len(w.CollectorConfig.Images))]
		index := len(content) / 3
		content = append(content, "")
		copy(content[index+1:], content[index:])
		content[index] = "<img src='" + img + "' alt='" + title + "'/>"
	}
	if w.CollectorConfig.InsertImage == config.CollectImageCategory {
		// 根据分类每次只取其中一张
		img := w.GetRandImageFromCategory(w.CollectorConfig.ImageCategoryId, keyword.Title)
		if len(img) > 0 {
			index := len(content) / 3
			content = append(content, "")
			copy(content[index+1:], content[index:])
			content[index] = "<img src='" + img + "' alt='" + title + "'/>"
		}
	}

	archive := request.Archive{
		Title:     title,
		ModuleId:  0,
		Keywords:  keyword.Title,
		Content:   strings.Join(content, "\n"),
		KeywordId: keyword.Id,
		OriginUrl: keyword.Title,
	}

	return &archive, nil
}

func (w *Website) collectCombinationMaterials(keyword *model.Keyword) ([]*model.Material, error) {
	combineIndex = (combineIndex + 1) % 4
	if combineIndex == 0 {
		if sogouForbid {
			combineIndex++
		} else {
			return w.getDataFromSogou(keyword)
		}
	}
	if combineIndex == 1 {
		if soForbid {
			combineIndex++
		} else {
			return w.getDataFrom360(keyword)
		}
	}
	if combineIndex == 2 {
		if baiduForbid {
			combineIndex++
		} else {
			return w.getDataFromBaidu(keyword)
		}
	}
	return w.getDataFromToutiao(keyword)
}

func (w *Website) getDataFrom360(keyword *model.Keyword) ([]*model.Material, error) {
	searchUrl := fmt.Sprintf("https://wenda.so.com/search/?q=%s", url.QueryEscape(keyword.Title))

	body, err := w.getEnginData(searchUrl, 0)
	if err != nil {
		return nil, err
	}
	// 分析360隐藏的标签
	re, _ := regexp.Compile(`(?s)<style.*?>(.*?)</style>`)
	styleMatches := re.FindAllString(body, -1)
	var hiddenClass = map[string]struct{}{}
	re1, _ := regexp.Compile(`\.[a-z0-9]+`)
	for _, match := range styleMatches {
		lines := strings.Split(match, "\n")
		for _, x := range lines {
			if strings.Contains(x, "visibility:hidden") || strings.Contains(x, "display:none") {
				x2 := strings.SplitN(x, "{", 2)
				m := re1.FindAllString(x2[0], -1)
				for _, v := range m {
					v = strings.TrimPrefix(v, ".")
					hiddenClass[v] = struct{}{}
				}
			}
		}
	}
	// end
	// 提取链接
	var links []*model.Material
	re, _ = regexp.Compile(`(?s)<a.*?href="(/q/.*?)".*?>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(body, -1)
	var existLinks = map[string]struct{}{}
	var count int
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
	for _, v := range matches {
		if !strings.Contains(v[0], "item__title") {
			continue
		}
		// 如果标题不合格，则要抛弃
		// 360 会添加随机字符
		re2, _ := regexp.Compile(`<[a-z0-9]+ class="([^"]+)"[^>]*>[^<]+?</[a-z0-9]+>`)
		title := re2.ReplaceAllStringFunc(v[2], func(s string) string {
			match := re2.FindStringSubmatch(s)
			if _, ok := hiddenClass[match[1]]; ok {
				return ""
			}
			return s
		})
		//end
		title = strings.ReplaceAll(library.StripTags(title), "\n", "")
		if _, ok := existLinks[v[1]]; ok {
			continue
		}
		existLinks[v[1]] = struct{}{}
		link := "https://wenda.so.com" + v[1]
		if count >= 5 {
			break
		}
		ch <- 1
		wg.Add(1)
		go func(link string, title string) {
			defer func() {
				<-ch
				wg.Done()
			}()
			// 逐个解析内容
			item, fetch, err2 := w.getAnswerSection(link, title, keyword)
			//log.Println(item, err2)
			if err2 == nil {
				count++
				links = append(links, item)
			}
			// 360 需要停顿
			if fetch && w.Proxy == nil {
				time.Sleep(5 * time.Second)
			}
		}(link, title)
	}
	wg.Wait()

	return links, nil
}

func (w *Website) getDataFromBaidu(keyword *model.Keyword) ([]*model.Material, error) {
	searchUrl := fmt.Sprintf("https://zhidao.baidu.com/search?pn=0&tn=ikaslist&rn=10&word=%s", keyword.Title)

	body, err := w.getEnginData(searchUrl, 0)
	if err != nil {
		return nil, err
	}
	// 提取链接
	var links []*model.Material
	re, _ := regexp.Compile(`(?s)<a.*?href="(http://zhidao.baidu.com/question/.*?)".*?>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(body, -1)
	var existLinks = map[string]struct{}{}
	var count int
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
	for _, v := range matches {
		// 如果标题不合格，则要抛弃
		title := strings.ReplaceAll(library.StripTags(v[2]), "\n", "")
		if _, ok := existLinks[v[1]]; ok {
			continue
		}
		existLinks[v[1]] = struct{}{}
		link := v[1]
		if count >= 5 {
			break
		}
		ch <- 1
		wg.Add(1)
		go func(link string, title string) {
			defer func() {
				<-ch
				wg.Done()
			}()
			// 逐个解析内容
			item, fetch, err2 := w.getAnswerSection(link, title, keyword)
			if err2 == nil {
				count++
				links = append(links, item)
			}
			if fetch && w.Proxy == nil {
				time.Sleep(2 * time.Second)
			}
		}(link, title)
	}
	wg.Wait()

	return links, nil
}

func (w *Website) getDataFromSogou(keyword *model.Keyword) ([]*model.Material, error) {
	searchUrl := fmt.Sprintf("https://www.sogou.com/sogou?query=%s&ie=utf8&insite=wenwen.sogou.com", keyword.Title)

	body, err := w.getEnginData(searchUrl, 0)
	if err != nil {
		return nil, err
	}
	// 提取链接
	var links []*model.Material
	re, _ := regexp.Compile(`(?s)<a.*?href="(/link\?url=.*?)".*?>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(body, -1)
	var existLinks = map[string]struct{}{}
	var count int
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
	for _, v := range matches {
		// 如果标题不合格，则要抛弃
		// nginx是什么意思 - 搜狗问问
		title := strings.Split(strings.ReplaceAll(library.StripTags(v[2]), "\n", ""), " - ")[0]
		if _, ok := existLinks[v[1]]; ok {
			continue
		}
		existLinks[v[1]] = struct{}{}
		link := "https://www.sogou.com" + v[1]
		if count >= 5 {
			break
		}
		ch <- 1
		wg.Add(1)
		go func(link string, title string) {
			defer func() {
				<-ch
				wg.Done()
			}()
			// 逐个解析内容
			item, fetch, err2 := w.getAnswerSection(link, title, keyword)
			//log.Println(item, err2)
			if err2 == nil {
				count++
				links = append(links, item)
			}
			if fetch && w.Proxy == nil {
				time.Sleep(5 * time.Second)
			}
		}(link, title)
	}
	wg.Wait()

	return links, nil
}

func (w *Website) getDataFromToutiao(keyword *model.Keyword) ([]*model.Material, error) {
	collectUrl := fmt.Sprintf("https://search5-search-lq.toutiaoapi.com/search?keyword=%s&pd=question&original_source=&format=json", keyword.Title)
	body, err := w.getEnginData(collectUrl, 0)
	if err != nil {
		return nil, err
	}

	var items []*model.Material
	links := w.ParseToutiaoJson(body)
	var count int
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
		if count >= 5 {
			break
		}
		ch <- 1
		wg.Add(1)
		go func(link *WebLink) {
			defer func() {
				<-ch
				wg.Done()
			}()
			item, fetch, err2 := w.getAnswerSection(link.Url, link.Name, keyword)
			if err2 == nil {
				count++
				items = append(items, item)
			}
			if fetch {
				//time.Sleep(5 * time.Second)
			}
		}(link)
	}
	wg.Wait()

	return items, nil
}
func (w *Website) ParseToutiaoJson(content string) []*WebLink {
	var toutiaoJson ToutiaoJson
	err := json.Unmarshal([]byte(content), &toutiaoJson)
	if err != nil {
		return nil
	}

	var links []*WebLink
	re, _ := regexp.Compile(`(?s)<a href="(.*?)".*?>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(toutiaoJson.Dom, -1)
	var existLinks = map[string]struct{}{}
	for _, v := range matches {
		if !strings.HasPrefix(v[1], "/search/jump") {
			continue
		}
		if _, ok := existLinks[v[1]]; ok {
			continue
		}
		existLinks[v[1]] = struct{}{}
		re, _ = regexp.Compile("url=(.+?)&amp;")
		match := re.FindStringSubmatch(v[1])
		if len(match) > 1 {
			link := match[1]
			link, _ = url.QueryUnescape(link)
			links = append(links, &WebLink{
				Name: v[2],
				Url:  link,
			})
		}
	}

	return links
}

func (w *Website) getAnswerSection(link, title string, keyword *model.Keyword) (material *model.Material, fetch bool, err error) {
	if !ContainKeywords(title, keyword.Title) {
		return nil, false, errors.New("bad title")
	}

	material, err = w.GetMaterialByOriginUrl(link)
	if err == nil {
		return material, false, nil
	}
	material, err = w.GetMaterialByTitle(title)
	if err == nil {
		return material, false, nil
	}
	body, err := w.getEnginData(link, 0)
	if err != nil {
		return nil, false, err
	}
	// 分析360隐藏的标签
	var hiddenClass = map[string]struct{}{}
	if strings.Contains(link, "wenda.so.com") {
		re, _ := regexp.Compile(`(?s)<style.*?>(.*?)</style>`)
		styleMatches := re.FindAllString(body, -1)
		re1, _ := regexp.Compile(`\.[a-z0-9]+`)
		for _, match := range styleMatches {
			lines := strings.Split(match, "\n")
			for _, x := range lines {
				if strings.Contains(x, "visibility:hidden") || strings.Contains(x, "display:none") {
					x2 := strings.SplitN(x, "{", 2)
					m := re1.FindAllString(x2[0], -1)
					for _, v := range m {
						v = strings.TrimPrefix(v, ".")
						hiddenClass[v] = struct{}{}
					}
				}
			}
		}
	}
	// end
	htmlR := strings.NewReader(body)
	doc, err := goquery.NewDocumentFromReader(htmlR)
	if err != nil {
		return nil, true, err
	}
	doc.Find("script,style").Remove()
	var item model.Material
	//log.Println(body)
	h1 := doc.Find("h1")
	h1.Find("*").Remove()
	item.Title = strings.TrimSpace(h1.Text())
	// wenda.so.com
	if strings.Contains(link, "wenda.so.com") {
		text1, _ := doc.Find(".question-content").Eq(0).Html()
		text2, _ := doc.Find(".answer-content").Eq(0).Html()

		item.Content = text1 + text2
		// 360 会添加随机字符
		re2, _ := regexp.Compile(`<[a-z0-9]+ class="([^"]+)"[^>]*>[^<]+?</[a-z0-9]+>`)
		item.Content = re2.ReplaceAllStringFunc(item.Content, func(s string) string {
			match := re2.FindStringSubmatch(s)
			if _, ok := hiddenClass[match[1]]; ok {
				return ""
			}
			return s
		})
		//end
	} else if strings.Contains(link, "zhidao.baidu.com") {
		item.Content = doc.Find(".rich-content-container").Eq(0).Text()
	} else if strings.Contains(link, "wenwen.sogou.com") || strings.Contains(link, "www.sogou.com") {
		item.Content = doc.Find(".replay-info-txt").Eq(0).Text()
	} else if strings.Contains(link, "wenda.tianya.cn") {
		item.Content = doc.Find(".arrowsCon,.comment_list .post-details").Eq(0).Text()
	} else if strings.Contains(link, "www.toutiao.com/question") || strings.Contains(link, "www.wukong.com") {
		item.Content = doc.Find("article").Eq(0).Text()
	} else if strings.Contains(link, "wenda.guidechem.com") {
		item.Title = doc.Find("h2").Eq(0).Text()
		item.Content = doc.Find(".ex_sho_main").Eq(0).Text()
	} else if strings.Contains(link, "www.qipeiren.com") {
		item.Content = doc.Find(".qcdlc-answ dt").Eq(0).Text()
	} else if strings.Contains(link, "wap.zol.com.cn") {
		item.Content = doc.Find(".autio-list__audio-detail").Eq(0).Text()
	} else if strings.Contains(link, "zixue.3d66.com") || strings.Contains(link, "www.yutu.cn") {
		item.Content = doc.Find(".ask-content-inner").Eq(0).Text()
	} else if strings.Contains(link, "so.toutiao.com/s/search_wenda_pc") || strings.Contains(link, "tsearch.toutiaoapi.com") {
		item.Title = doc.Find("h2").Eq(0).Text()
		re, _ := regexp.Compile(`(?s)<div class="answer_layout_.*?">(.*?)</div>`)
		match := re.FindStringSubmatch(body)
		if len(match) > 1 {
			item.Content = match[1]
		} else {
			// 尝试从json中解析
			re, _ = regexp.Compile(`(?s)"content":\s*"(.+?)"`)
			match = re.FindStringSubmatch(body)
			if len(match) > 1 {
				err = json.Unmarshal([]byte("\""+match[1]+"\""), &item.Content)
				if err != nil {
					item.Content = match[1]
				}
			}
			re, _ = regexp.Compile(`(?s)"title":\s*"(.+?)"`)
			match = re.FindStringSubmatch(body)
			if len(match) > 1 {
				err = json.Unmarshal([]byte("\""+match[1]+"\""), &item.Title)
				if err != nil {
					item.Title = match[1]
				}
			}
		}
	} else {
		content, _, _, _ := w.ParseArticleContent(doc.Find("body"), 0, false)
		if content != nil {
			item.Content = content.Text()
		}
	}

	if len(item.Title) == 0 {
		item.Title = w.ParseArticleTitle(doc)
	}
	if len(item.Content) == 0 {
		doc.Find("header,.header,footer,.footer,.footer-new,aside").Remove()
		var pContent string
		ps := doc.Find("p")
		for i := range ps.Nodes {
			text := ps.Eq(i).Text()
			if utf8.RuneCountInString(text) > utf8.RuneCountInString(pContent) {
				pContent = text
			}
		}
		item.Content = pContent
	}
	item.Content = strings.TrimSpace(item.Content)

	if utf8.RuneCountInString(item.Content) > 400 {
		// 裁剪只需要一部分
		runeText := []rune(item.Content)
		for i := 400; i < len(runeText); i++ {
			if runeText[i] == '。' ||
				runeText[i] == '.' ||
				runeText[i] == '；' ||
				runeText[i] == ';' ||
				runeText[i] == '！' ||
				runeText[i] == '!' ||
				runeText[i] == '　' ||
				runeText[i] == ' ' {
				item.Content = string(runeText[:i])
				break
			}
		}
	}

	if len(item.Title) == 0 || len(item.Content) < 40 {
		return nil, true, errors.New(w.Tr("NoContent"))
	}
	// 保存它
	if utf8.RuneCountInString(link) > 190 {
		link = string([]rune(link)[:190])
	}
	item.OriginUrl = link
	item.Keyword = keyword.Title
	w.DB.Save(&item)

	return &item, true, nil
}

func (w *Website) getEnginData(link string, retry int) (string, error) {
	var proxyIp string
	if w.Proxy != nil {
		proxyIp = w.Proxy.GetIP()
	}
	ops := &library.Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			"Referer":         link,
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		},
		Proxy: proxyIp,
	}

	resp, err := library.Request(link, ops)
	if err != nil {
		if proxyIp != "" {
			w.Proxy.RemoveIP(proxyIp)
			// 重试2次
			if retry < 2 {
				return w.getEnginData(link, retry+1)
			}
		}
		return "", err
	}
	// sogou.com 可能需要中转
	re, _ := regexp.Compile(`(?i)<META http-equiv="refresh"[^>]+?URL='(.+?)'[^>]*>`)
	match := re.FindStringSubmatch(resp.Body)
	if len(match) > 1 {
		return w.getEnginData(match[1], 0)
	}
	// 如果出现验证码，并没有使用代理，则暂停一段时间
	if proxyIp == "" {
		if strings.Contains(resp.Body, "百度安全验证") ||
			strings.Contains(resp.Body, "系统检测到您网络中存在异常访问请求") ||
			strings.Contains(resp.Body, "通过验证才能继续操作哦") ||
			strings.Contains(resp.Body, "请输入验证码以便正常访问") {
			// 出现验证码
			if strings.Contains(link, "baidu.com") {
				baiduForbid = true
				go func() {
					select {
					case <-time.After(15 * time.Minute):
						baiduForbid = false
					}
				}()
			} else if strings.Contains(link, "sogou.com") {
				sogouForbid = true
				go func() {
					select {
					case <-time.After(15 * time.Minute):
						sogouForbid = false
					}
				}()
			} else if strings.Contains(link, "so.com") || strings.Contains(link, "360.cn") {
				soForbid = true
				go func() {
					select {
					case <-time.After(15 * time.Minute):
						soForbid = false
					}
				}()
			}
		}
	}

	return resp.Body, nil
}
