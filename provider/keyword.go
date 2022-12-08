package provider

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"math/rand"
	"mime/multipart"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type KeywordCSV struct {
	Title      string `csv:"title"`
	CategoryId uint   `csv:"category_id"`
}

func (w *Website) GetUserKeywordSetting() config.KeywordJson {
	var keywordJson config.KeywordJson
	value := w.GetSettingValue(KeywordSettingKey)
	if value == "" {
		return keywordJson
	}

	if err := json.Unmarshal([]byte(value), &keywordJson); err != nil {
		return keywordJson
	}

	return keywordJson
}

func (w *Website) SaveUserKeywordSetting(req config.KeywordJson, focus bool) error {
	keywordJson := w.GetUserKeywordSetting()
	if focus {
		keywordJson = req
	} else {
		if req.TitleExclude != nil {
			keywordJson.TitleExclude = req.TitleExclude
		}
		if req.TitleReplace != nil {
			keywordJson.TitleReplace = req.TitleReplace
		}
		if req.Language != "" {
			keywordJson.Language = req.Language
		}
		if req.FromEngine != "" {
			keywordJson.FromEngine = req.FromEngine
		}
		if req.FromWebsite != "" {
			keywordJson.FromWebsite = req.FromWebsite
		}
	}

	_ = w.SaveSettingValue(KeywordSettingKey, keywordJson)
	//重新读取配置
	w.LoadKeywordSetting()

	return nil
}

// 始终保持只有一个keyword任务
var digKeywordRunning = false

var maxWordsNum = int64(100000)

func (w *Website) GetKeywordList(keyword string, currentPage, pageSize int) ([]*model.Keyword, int64, error) {
	var keywords []*model.Keyword
	offset := (currentPage - 1) * pageSize
	var total int64

	builder := w.DB.Model(&model.Keyword{}).Order("id desc")
	if keyword != "" {
		//模糊搜索
		builder = builder.Where("(`title` like ?)", "%"+keyword+"%")
	}

	err := builder.Count(&total).Limit(pageSize).Offset(offset).Find(&keywords).Error
	if err != nil {
		return nil, 0, err
	}

	return keywords, total, nil
}

func (w *Website) GetAllKeywords() ([]*model.Keyword, error) {
	var keywords []*model.Keyword
	err := w.DB.Model(&model.Keyword{}).Order("id desc").Find(&keywords).Error
	if err != nil {
		return nil, err
	}

	return keywords, nil
}

func (w *Website) GetKeywordById(id uint) (*model.Keyword, error) {
	var keyword model.Keyword

	err := w.DB.Where("`id` = ?", id).First(&keyword).Error
	if err != nil {
		return nil, err
	}

	return &keyword, nil
}

func (w *Website) GetKeywordByTitle(title string) (*model.Keyword, error) {
	var keyword model.Keyword

	err := w.DB.Where("`title` = ?", title).First(&keyword).Error
	if err != nil {
		return nil, err
	}

	return &keyword, nil
}

func (w *Website) ImportKeywords(file multipart.File, info *multipart.FileHeader) (string, error) {
	buff, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(buff), "\n")
	var total int
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// 格式：title, category_id
		if i == 0 {
			continue
		}
		values := strings.Split(line, ",")
		if len(values) < 2 {
			continue
		}
		title := strings.TrimSpace(values[0])
		if title == "" {
			continue
		}
		keyword, err := w.GetKeywordByTitle(title)
		if err != nil {
			//表示不存在
			keyword = &model.Keyword{
				Title:  title,
				Status: 1,
			}
			total++
		}
		categoryId, _ := strconv.Atoi(values[1])
		keyword.CategoryId = uint(categoryId)

		keyword.Save(w.DB)
	}

	return fmt.Sprintf(w.Lang("成功导入了%d个关键词"), total), nil
}

func (w *Website) DeleteKeyword(keyword *model.Keyword) error {
	err := w.DB.Delete(keyword).Error
	if err != nil {
		return err
	}

	return nil
}

// StartDigKeywords 开始挖掘关键词，通过核心词来拓展
// 最多只10万关键词，抓取前3级，如果超过3级，则每次只执行一级
func (w *Website) StartDigKeywords(focus bool) {
	if w.DB == nil {
		return
	}
	if w.KeywordConfig.AutoDig == false && !focus {
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
	w.DB.Model(&model.Keyword{}).Count(&maxNum)
	if maxNum >= maxWordsNum {
		return
	}

	var keywords []*model.Keyword
	w.DB.Where("has_dig = 0").Order("id asc").Limit(10).Find(&keywords)
	if len(keywords) == 0 {
		return
	}

	for _, keyword := range keywords {
		//下一级的
		err := w.collectKeyword(collectedWords, keyword)
		if err != nil {
			break
		}
		keyword.HasDig = 1
		w.DB.Model(keyword).UpdateColumn("has_dig", keyword.HasDig)
		//不能太快，每次休息随机1-30秒钟
		time.Sleep(time.Duration(1+rand.Intn(30)) * time.Second)
	}
	//重新计数
	w.DB.Model(&model.Keyword{}).Count(&maxNum)
}

func (w *Website) collectKeyword(existsWords *sync.Map, keyword *model.Keyword) error {
	link := w.getKeywordEnginLink(keyword)
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
	links := w.CollectKeywords(resp.Body)
	rootWords := w.GetRootWords()
	for _, k := range links {
		// 判断是否包含核心词
		if !w.ContainRootWords(rootWords, k.Title) {
			continue
		}
		if _, ok := existsWords.Load(k.Title); ok {
			continue
		}
		existsWords.Store(k.Title, keyword.Level+1)
		word := &model.Keyword{
			Title:      k.Title,
			CategoryId: keyword.CategoryId,
			Level:      keyword.Level + 1,
		}
		w.DB.Model(&model.Keyword{}).Where("title = ?", k.Title).FirstOrCreate(&word)
	}

	return nil
}

func (w *Website) getKeywordEnginLink(keyword *model.Keyword) string {
	// default baidu
	var link string
	switch w.KeywordConfig.FromEngine {
	case config.Engin360:
		link = fmt.Sprintf("https://sug.so.360.cn/suggest?src=so_home&word=%s", url.QueryEscape(keyword.Title))
		break
	case config.EnginSogou:
		link = fmt.Sprintf("https://www.sogou.com/suggnew/ajajjson?key=%s&type=web", url.QueryEscape(keyword.Title))
		break
	case config.EnginGoogle:
		link = fmt.Sprintf("https://www.google.com/complete/search?q=%s&cp=5&client=gws-wiz", url.QueryEscape(keyword.Title))
		break
	case config.EnginBingCn:
		link = fmt.Sprintf("https://cn.bing.com/search?q=%s", url.QueryEscape(keyword.Title))
		break
	case config.EnginBing:
		link = fmt.Sprintf("https://cn.bing.com/search?q=%s&ensearch=1", url.QueryEscape(keyword.Title))
		break
	case config.EnginOther:
		if strings.Contains(w.KeywordConfig.FromWebsite, "%s") {
			link = fmt.Sprintf(w.KeywordConfig.FromWebsite, url.QueryEscape(keyword.Title))
			break
		}
	//case config.EnginBaidu:
	default:
		link = fmt.Sprintf("http://www.baidu.com/sugrec?prod=pc&wd=%s", url.QueryEscape(keyword.Title))
		break
	}

	return link
}

func (w *Website) CollectKeywords(content string) []*model.Keyword {
	var words []*model.Keyword
	existsWords := map[string]*model.Keyword{}

	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return words
	}

	if content[:1] == "<" {
		//html
		//解析文档内容
		htmlR := strings.NewReader(content)
		doc, err := goquery.NewDocumentFromReader(htmlR)
		if err != nil {
			return nil
		}

		replacer := strings.NewReplacer("&nsbp;", ",", "\t", ",", "\n", ",", " ", ",")

		metas := doc.Find("h1,h2,h3,strong,a")

		for i := range metas.Nodes {
			text := strings.TrimSpace(metas.Eq(i).Text())
			if text != "" {
				//开始替换
				matchArr := strings.Split(replacer.Replace(text), ",")
				for _, v := range matchArr {
					v = strings.Trim(v, "“”·… ")
					v = strings.TrimSpace(v)
					var ok bool
					if v, ok = w.KeywordFilter(v); !ok {
						continue
					}

					runeLength := utf8.RuneCountInString(v)
					if CheckContentIsEnglish(v) && strings.Count(v, " ") > 1 {
						runeLength = len(strings.Split(v, " "))
					}
					//超过30长度的丢弃
					if runeLength >= 3 && runeLength < 30 {
						if existsWords[v] == nil {
							existsWords[v] = &model.Keyword{
								Title: v,
							}
						}
					}
				}
			}
		}
	} else {
		//可能是json:
		//采用正则表达式来匹配
		tagRe, _ := regexp.Compile("\\<[\\S\\s]+?\\>")
		reg := regexp.MustCompile(`(?i)"([^"]+)"`)
		matches := reg.FindAllStringSubmatch(content, -1)
		if len(matches) > 1 {
			//读取所有可能的关键词
			for _, match := range matches {
				v := strings.Trim(match[1], "“”·…")
				v = strings.ReplaceAll(v, "\\u003c", "<")
				v = strings.ReplaceAll(v, "\\u003e", ">")
				v = strings.ReplaceAll(v, "\\/", "/")
				v = strings.TrimSpace(v)
				v = tagRe.ReplaceAllString(v, "")
				var ok bool
				if v, ok = w.KeywordFilter(v); !ok {
					continue
				}

				runeLength := utf8.RuneCountInString(v)
				if CheckContentIsEnglish(v) && strings.Count(v, " ") > 1 {
					runeLength = len(strings.Split(v, " "))
				}
				if runeLength >= 3 && runeLength < 30 {
					if existsWords[v] == nil {
						existsWords[v] = &model.Keyword{
							Title: v,
						}
					}
				}
			}
		}
	}

	for _, v := range existsWords {
		words = append(words, v)
	}

	return words
}

func (w *Website) KeywordFilter(word string) (string, bool) {
	if strings.Contains(word, "/") {
		// maybe it's a link
		return "", false
	}
	if strings.Contains(word, "\\u") || strings.Contains(word, "_") || strings.Contains(word, "=") || strings.Contains(word, " - ") || strings.Contains(word, " | ") || strings.Contains(word, "｜") || strings.Contains(word, "…") || strings.Contains(word, "...") {
		return "", false
	}
	re, _ := regexp.Compile(`^[0-9.,\s]+$`)
	if re.MatchString(word) {
		return "", false
	}
	isEnglish := CheckContentIsEnglish(word)
	if strings.Contains(word, "'") && (!isEnglish || (isEnglish && strings.Count(word, " ") == 0)) {
		return "", false
	}
	if strings.Contains(word, "-") && !isEnglish {
		return "", false
	}
	if w.KeywordConfig.Language == config.LanguageZh && isEnglish {
		return "", false
	}
	if w.KeywordConfig.Language == config.LanguageEn && !isEnglish {
		return "", false
	}

	runeLength := utf8.RuneCountInString(word)
	if isEnglish && strings.Count(word, " ") > 1 {
		runeLength = len(strings.Split(word, " "))
	}
	//超过30长度的丢弃
	if runeLength < 3 || runeLength > 30 {
		return "", false
	}
	for _, v := range w.KeywordConfig.TitleExclude {
		if strings.Contains(word, v) {
			return "", false
		}
	}
	var err error
	for _, v := range w.KeywordConfig.TitleReplace {
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
				word = re.ReplaceAllString(word, v.To)
			}
			continue
		}
		word = strings.ReplaceAll(word, v.From, v.To)
	}

	return word, true
}

func (w *Website) GetRootWords() [][]string {
	var rootKeywords []string
	w.DB.Model(&model.Keyword{}).Where("`level` = 0").Limit(1000).Pluck("title", &rootKeywords)

	var result = make([][]string, 0, len(rootKeywords))
	for i := range rootKeywords {
		result = append(result, library.WordSplit(strings.ToLower(rootKeywords[i]), false))
	}

	return result
}

func (w *Website) ContainRootWords(rootWords [][]string, word string) bool {
	word = strings.ToLower(word)
	for i := range rootWords {
		match := true
		for _, w := range rootWords[i] {
			if !strings.Contains(word, w) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}

	return false
}

func (w *Website) ContainKeywords(title, keyword string) bool {
	if len(title) <= 2 {
		return false
	}
	title = strings.ToLower(title)
	words := library.WordSplit(strings.ToLower(keyword), false)
	maxLen := 0
	matchLen := 0
	for _, w := range words {
		maxLen += len(w)
		if strings.Contains(title, w) {
			matchLen += len(w)
		}
	}
	if float64(matchLen)/float64(maxLen) >= 0.60 {
		return true
	}

	return false
}
