package provider

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm/clause"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
	"log"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type KeywordCollect struct {
	*Website
	RootWords   [][]string
	ExistsWords *sync.Map
	ErrorTimes  int
}

type BaiduSugJson struct {
	G []struct {
		Q string `json:"q"`
	} `json:"g"`
}

type SoSugJson struct {
	Result []struct {
		Word string `json:"word"`
	} `json:"result"`
}

type ToutiaoSugJson struct {
	Data []struct {
		Keyword string `json:"keyword"`
	} `json:"data"`
}

type ZhihuJson struct {
	Suggest []struct {
		Query string `json:"query"`
	} `json:"suggest"`
}

type SmJson struct {
	R []struct {
		W string `json:"w"`
	} `json:"r"`
}

type BingJson struct {
	AS struct {
		Query   string `json:"Query"`
		Results []struct {
			Suggests []struct {
				Txt string `json:"Txt"`
			} `json:"Suggests"`
		} `json:"Results"`
	} `json:"AS"`
}

type DuckDuckGoJson struct {
	Phrase string `json:"phrase"`
}

type YahooJson struct {
	Q string `json:"q"`
	R []struct {
		K string `json:"k"`
	} `json:"r"`
}

var cnLinkIndex = 0
var cnKeywordLinks = []string{
	"http://www.baidu.com/sugrec?prod=pc&wd=%s",
	"https://sug.so.360.cn/suggest?callback=suggest_so&word=%s",
	"https://sor.html5.qq.com/api/getsug?key=%s&type=pc&ori=yes&pr=web&abtestid=4",
	"https://www.toutiao.com/2/article/search_sug/?keyword=%s",
	"https://sugs.m.sm.cn/web?t=w&uc_param_str=dnnwnt&q=%s",
	"https://www.zhihu.com/api/v4/search/suggest?q=%s",
	"https://api.bing.com/qsonhs.aspx?type=cb&q=%s",
}

var enLinkIndex = 0
var enKeywordLinks = []string{
	"https://api.bing.com/qsonhs.aspx?type=cb&q=%s",
	//"https://yandex.com/suggest/suggest-ya.cgi?srv=morda_com_desktop&wiz=TrWth&uil=en&fact=1&v=4&icon=1&hl=1&bemjson=0&html=1&platform=desktop&rich_nav=1&verified_nav=1&rich_phone=1&use_favicon=1&nav_favicon=1&nav_text=1&a=0&mt_wizard=1&n=10&svg=1&part=%s",
	"http://clients1.google.com/complete/search?hl=en&output=toolbar&q=%s",
	"http://suggestqueries.google.com/complete/search?client=youtube&q=%s&hl=en&jsonp=window.google.ac.h",
	"https://duckduckgo.com/ac/?q=%s&kl=wt-wt",
	"https://sg.search.yahoo.com/sugg/gossip/gossip-sg-ura/?pq=&command=%s&callback=YAHOO.SA.apps%5B0%5D.cb.sacb12&l=1&bm=3&output=sd1&nresults=10&appid=yfp-t",
}

func NewKeywordCollect(w *Website) *KeywordCollect {
	collector := &KeywordCollect{
		Website:     w,
		ExistsWords: &sync.Map{},
	}

	return collector
}

func (k *KeywordCollect) InitRootWords() {
	var rootKeywords []string
	k.DB.Model(&model.Keyword{}).Where("`level` = 0").Limit(500).Pluck("title", &rootKeywords)

	var result = make([][]string, 0, len(rootKeywords))
	for i := range rootKeywords {
		result = append(result, WordSplit(strings.ToLower(rootKeywords[i]), false))
		k.ExistsWords.Store(rootKeywords[i], 0)
	}

	k.RootWords = result
}

func (k *KeywordCollect) Start() {
	//非严格的限制数量
	var maxNum int64
	k.DB.Model(&model.Keyword{}).Count(&maxNum)
	if maxNum >= k.KeywordConfig.MaxCount {
		return
	}

	k.InitRootWords()
	if len(k.RootWords) == 0 {
		// 没有核心词不需要执行
		return
	}

	var keywords []*model.Keyword
	k.DB.Model(&model.Keyword{}).Where("has_dig = 0").Order("id asc").Limit(100).Find(&keywords)
	if len(keywords) == 0 {
		// 从头开始
		log.Println("没有关键词了， 重新开始")
		k.DB.Model(&model.Keyword{}).Where("1=1").UpdateColumn("has_dig", 0)
		return
	}

	for _, keyword := range keywords {
		log.Println("采集", keyword.Title)
		//下一级的
		total, err := k.collectKeyword(keyword, false)
		if err != nil || total == 0 {
			// 从第一种方案中继续
			_, _ = k.collectKeyword(keyword, true)
		}
		keyword.HasDig = 1
		k.DB.Model(&model.Keyword{}).Where("`id` = ?", keyword.Id).UpdateColumn("has_dig", keyword.HasDig)
		//不能太快，每次休息随机1-10秒钟
		time.Sleep(time.Duration(1+rand.Intn(10)) * time.Second)
	}
}

func (k *KeywordCollect) collectKeyword(keyword *model.Keyword, fix bool) (int, error) {
	link := k.getEngineLink(keyword.Title, k.KeywordConfig.Language, fix)
	ops := &library.Options{
		Timeout:  5,
		IsMobile: false,
		Header: map[string]string{
			"Referer":         link,
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
		},
	}
	resp, err := library.Request(link, ops)
	if err != nil {
		if strings.Contains(link, "google.com") {
			k.ErrorTimes++
			if k.ErrorTimes >= 3 {
				config.GoogleValid = false
			}
		}
		return 0, err
	}
	links := k.CollectKeywords(resp.Body, link)
	for _, l := range links {
		//log.Println(l.Title)
		// 判断是否包含核心词
		if !k.ContainRootWords(l.Title) {
			log.Println(l.Title, "不包含核心词")
			continue
		}
		// 移除排除词
		if len(k.KeywordConfig.TitleExclude) > 0 {
			exist := false
			for _, e := range k.KeywordConfig.TitleExclude {
				if strings.Contains(l.Title, e) {
					log.Println(l.Title, "包含排除词", e)
					exist = true
					break
				}
			}
			if exist {
				continue
			}
		}
		if _, ok := k.ExistsWords.Load(l.Title); ok {
			continue
		}
		k.ExistsWords.Store(l.Title, keyword.Level+1)
		word := &model.Keyword{
			Title:      l.Title,
			CategoryId: keyword.CategoryId,
			Level:      keyword.Level + 1,
		}
		k.DB.Clauses(clause.OnConflict{
			DoNothing: true,
		}).Model(&model.Keyword{}).Where("title = ?", word.Title).Create(&word)
	}

	return len(links), nil
}

func (k *KeywordCollect) getEngineLink(title, language string, fix bool) string {
	// default baidu
	var link string
	if language != config.LanguageEn {
		cnLinkIndex = (cnLinkIndex + 1) % len(cnKeywordLinks)
		if fix {
			link = cnKeywordLinks[0]
		} else {
			link = cnKeywordLinks[cnLinkIndex]
		}
	} else {
		if config.GoogleValid {
			enLinkIndex = (enLinkIndex + 1) % len(enKeywordLinks)
			link = enKeywordLinks[enLinkIndex]
		} else {
			link = enKeywordLinks[0]
		}
	}
	return fmt.Sprintf(link, url.QueryEscape(title))
}

func (k *KeywordCollect) ContainRootWords(word string) bool {
	word = strings.ToLower(word)
	for i := range k.RootWords {
		match := true
		for _, w := range k.RootWords[i] {
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

func (k *KeywordCollect) CollectKeywords(content string, link string) []*model.Keyword {
	var words []*model.Keyword
	existsWords := map[string]*model.Keyword{}

	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return words
	}
	if strings.Contains(link, "zhihu.com") {
		var result ZhihuJson
		err := json.Unmarshal([]byte(content), &result)
		if err != nil {
			log.Println("解析json失败1")
			return words
		}
		for _, v := range result.Suggest {
			existsWords[v.Query] = &model.Keyword{
				Title: v.Query,
			}
		}
	} else if strings.Contains(link, "baidu.com") {
		var result BaiduSugJson
		err := json.Unmarshal([]byte(content), &result)
		if err != nil {
			log.Println("解析json失败2")
			return words
		}
		for _, v := range result.G {
			existsWords[v.Q] = &model.Keyword{
				Title: v.Q,
			}
		}
	} else if strings.Contains(link, "baidu.com") {
		var result BaiduSugJson
		err := json.Unmarshal([]byte(content), &result)
		if err != nil {
			log.Println("解析json失败3")
			return words
		}
		for _, v := range result.G {
			existsWords[v.Q] = &model.Keyword{
				Title: v.Q,
			}
		}
	} else if strings.Contains(link, "so.360.cn") {
		var result SoSugJson
		idxs := strings.Index(content, "{")
		idxe := strings.LastIndex(content, "}")
		if idxs < 0 || idxe < 0 {
			log.Println("解析json失败4")
			return words
		}
		content = content[idxs : idxe+1]
		err := json.Unmarshal([]byte(content), &result)
		if err != nil {
			log.Println("解析json失败4")
			return words
		}
		for _, v := range result.Result {
			existsWords[v.Word] = &model.Keyword{
				Title: v.Word,
			}
		}
	} else if strings.Contains(link, "toutiao.com") {
		var result ToutiaoSugJson
		err := json.Unmarshal([]byte(content), &result)
		if err != nil {
			log.Println("解析json失败5")
			return words
		}
		for _, v := range result.Data {
			existsWords[v.Keyword] = &model.Keyword{
				Title: v.Keyword,
			}
		}
	} else if strings.Contains(link, "m.sm.cn") {
		var result SmJson
		err := json.Unmarshal([]byte(content), &result)
		if err != nil {
			log.Println("解析json失败6")
			return words
		}
		for _, v := range result.R {
			existsWords[v.W] = &model.Keyword{
				Title: v.W,
			}
		}
	} else if strings.Contains(link, "html5.qq.com") {
		reg := regexp.MustCompile(`(?i)"([^"]+)",`)
		matches := reg.FindAllStringSubmatch(content, -1)
		if len(matches) > 1 {
			for _, match := range matches {
				if strings.Contains(match[1], ";") || strings.Contains(match[1], "sug") || utf8.RuneCountInString(match[1]) < 4 {
					continue
				}
				existsWords[match[1]] = &model.Keyword{
					Title: match[1],
				}
			}
		}
	} else if strings.Contains(link, "api.bing.com") {
		var result BingJson
		idxs := strings.Index(content, "{")
		idxe := strings.LastIndex(content, "}")
		if idxs < 0 || idxe < 0 {
			log.Println("解析json失败7")
			return words
		}
		content = content[idxs : idxe+1]
		err := json.Unmarshal([]byte(content), &result)
		if err != nil {
			log.Println("解析json失败7")
			return words
		}
		for _, v := range result.AS.Results {
			for _, vv := range v.Suggests {
				existsWords[vv.Txt] = &model.Keyword{
					Title: vv.Txt,
				}
			}
		}
	} else if strings.Contains(link, "yandex.com") || strings.Contains(link, "suggestqueries.google.com") {
		reg := regexp.MustCompile(`(?i)"([^"]+)",`)
		matches := reg.FindAllStringSubmatch(content, -1)
		if len(matches) > 1 {
			for _, match := range matches {
				existsWords[match[1]] = &model.Keyword{
					Title: match[1],
				}
			}
		}
	} else if strings.Contains(link, "clients1.google.com") {
		reg := regexp.MustCompile(`(?i)data="([^"]+)"`)
		matches := reg.FindAllStringSubmatch(content, -1)
		if len(matches) > 1 {
			for _, match := range matches {
				existsWords[match[1]] = &model.Keyword{
					Title: match[1],
				}
			}
		}
	} else if strings.Contains(link, "duckduckgo.com") {
		var result []DuckDuckGoJson
		err := json.Unmarshal([]byte(content), &result)
		if err != nil {
			log.Println("解析json失败8")
			return words
		}
		for _, v := range result {
			existsWords[v.Phrase] = &model.Keyword{
				Title: v.Phrase,
			}
		}
	} else if strings.Contains(link, "sg.search.yahoo.com") {
		content = strings.TrimPrefix(content, "YAHOO.SA.apps[0].cb.sacb12(")
		content = strings.TrimRight(content, ")")
		var result YahooJson
		err := json.Unmarshal([]byte(content), &result)
		if err != nil {
			log.Println("解析json失败9")
			return words
		}
		for _, v := range result.R {
			existsWords[v.K] = &model.Keyword{
				Title: v.K,
			}
		}
	}
	if len(existsWords) == 0 {
		if content[:1] == "<" {
			//html
			//解析文档内容
			htmlR := strings.NewReader(content)
			doc, err := goquery.NewDocumentFromReader(htmlR)
			if err != nil {
				return nil
			}

			replacer := strings.NewReplacer("&nsbp;", ",", "\t", ",", "\n", ",", " ", ",")

			metas := doc.Find("h1,h2,h3,strong,a,suggestion")
			for i := range metas.Nodes {
				text := strings.TrimSpace(metas.Eq(i).Text())
				if text == "" {
					text, _ = metas.Eq(i).Attr("data")
				}
				if text != "" {
					//开始替换
					matchArr := strings.Split(replacer.Replace(text), ",")
					for _, v := range matchArr {
						v = strings.Trim(v, "“”·… ")
						v = strings.TrimSpace(v)
						var ok bool
						if v, ok = k.KeywordFilter(v); !ok {
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
					if v, ok = k.KeywordFilter(v); !ok {
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
	}
	for _, v := range existsWords {
		words = append(words, v)
	}

	return words
}

func (k *KeywordCollect) KeywordFilter(word string) (string, bool) {
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
	if k.KeywordConfig.Language == config.LanguageZh && isEnglish {
		return "", false
	}
	if k.KeywordConfig.Language == config.LanguageEn && !isEnglish {
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
	for _, v := range k.KeywordConfig.TitleExclude {
		if strings.Contains(word, v) {
			return "", false
		}
	}
	var err error
	for _, v := range k.KeywordConfig.TitleReplace {
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
