package provider

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/model"
)

func (w *Website) SelfAiTranslateResult(req *AnqiTranslateTextRequest) (*AnqiTranslateTextRequest, error) {
	// 内容逐个翻译
	// 对于纯txt的打包处理，对于html的单独处理
	var newText []string
	var newIndex []int
	for i := range req.Text {
		content := strings.TrimSpace(req.Text[i])
		if content == "" {
			// 空的跳过
			continue
		}
		if strings.HasPrefix(content, "<") {
			resp, err := w.TranslateHtml(content, req.Language, req.ToLanguage)
			if err != nil {
				return nil, err
			} else {
				// 需要只取 body部分
				htmlR := strings.NewReader(resp)
				doc, err2 := goquery.NewDocumentFromReader(htmlR)
				if err2 == nil {
					htmlCode, err2 := doc.Find("body").Html()
					if err2 == nil {
						resp = htmlCode
					}
				}
				req.Text[i] = resp
			}
		} else {
			newText = append(newText, req.Text[i])
			newIndex = append(newIndex, i)
		}
	}
	if len(newText) > 0 {
		result, err := w.TranslateTexts(newText, req.Language, req.ToLanguage)
		if err != nil {
			return nil, err
		} else {
			// 还原到text
			for i := range result {
				req.Text[newIndex[i]] = result[i]
			}
		}
	}

	return req, nil
}

type BaiduTranslate struct {
	AppId     string
	AppSecret string
}

type BaiduTransResult struct {
	ErrorCode   string `json:"error_code"`
	ErrorMsg    string `json:"error_msg"`
	From        string `json:"from"`
	To          string `json:"to"`
	TransResult []struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	} `json:"trans_result"`
}

func NewBaiduTranslate(appId, appSecret string) *BaiduTranslate {
	return &BaiduTranslate{
		AppId:     appId,
		AppSecret: appSecret,
	}
}

var baiduChan = make(chan struct{}, 1)

func (b *BaiduTranslate) Translate(content string, fromLanguage string, toLanguage string) (string, error) {
	// 百度翻译的QPS = 1
	baiduChan <- struct{}{}
	defer func() {
		<-baiduChan
	}()
	// 为了qps，所以这里暂停1秒
	time.Sleep(1 * time.Second)
	// 将请求参数中的 APPID(appid)， 翻译 query(q，注意为UTF-8编码)，随机数(salt)，以及平台分配的密钥(可在管理控制台查看) 按照 appid+q+salt+密钥的顺序拼接得到字符串 1。
	salt := library.GenerateRandString(5)
	query := url.Values{}
	query.Add("appid", b.AppId)
	query.Add("q", content)
	query.Add("salt", salt)
	query.Add("from", library.GetLanguageBaiduCode(fromLanguage))
	query.Add("to", library.GetLanguageBaiduCode(toLanguage))
	signStr := b.AppId + content + salt + b.AppSecret
	sign := library.Md5(signStr)
	query.Add("sign", sign)
	urlString := query.Encode()
	resp, err := http.Post("https://fanyi-api.baidu.com/api/trans/vip/translate", "application/x-www-form-urlencoded", strings.NewReader(urlString))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result BaiduTransResult
	_ = json.Unmarshal(body, &result)

	if result.ErrorCode != "" {
		return "", errors.New(result.ErrorMsg)
	}

	var translated string
	for _, item := range result.TransResult {
		translated += item.Dst
	}
	if len(translated) == 0 {
		return "", errors.New("empty result")
	}

	return translated, nil
}

type YoudaoTranslate struct {
	AppKey    string
	AppSecret string
}

type YoudaoTranslateResult struct {
	ErrorCode   string   `json:"errorCode"`
	Query       string   `json:"query"`
	Translation []string `json:"translation"`
	L           string   `json:"l"`
}

func NewYoudaoTranslate(appKey, appSecret string) *YoudaoTranslate {
	return &YoudaoTranslate{
		AppKey:    appKey,
		AppSecret: appSecret,
	}
}

func (yt *YoudaoTranslate) Translate(content string, fromLanguage string, toLanguage string) (string, error) {
	// 将请求参数中的 APPID(appid)， 翻译 query(q，注意为UTF-8编码)，随机数(salt)，以及平台分配的密钥(可在管理控制台查看) 按照 appid+q+salt+密钥的顺序拼接得到字符串 1。
	salt := library.GenerateRandString(5)
	if strings.ToLower(fromLanguage) == "zh-cn" {
		fromLanguage = "zh-CHS"
	} else if strings.ToLower(fromLanguage) == "zh-tw" {
		fromLanguage = "zh-CHT"
	}
	if strings.ToLower(toLanguage) == "zh-cn" {
		toLanguage = "zh-CHS"
	} else if strings.ToLower(toLanguage) == "zh-tw" {
		toLanguage = "zh-CHT"
	}
	curtime := strconv.FormatInt(time.Now().Unix(), 10)
	query := url.Values{}
	query.Add("appKey", yt.AppKey)
	query.Add("q", content)
	query.Add("salt", salt)
	query.Add("from", fromLanguage)
	query.Add("to", toLanguage)
	query.Add("curtime", curtime)
	query.Add("signType", "v3")
	// appKey + truncate(query) + salt + curtime + key;
	signStr := yt.AppKey + yt.truncate(content) + salt + curtime + yt.AppSecret
	sign := yt.encrypt(signStr)
	query.Add("sign", sign)
	urlString := query.Encode()
	resp, err := http.Post("https://openapi.youdao.com/api", "application/x-www-form-urlencoded", strings.NewReader(urlString))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result YoudaoTranslateResult
	_ = json.Unmarshal(body, &result)
	translated := strings.Join(result.Translation, "")
	if len(translated) == 0 {
		return "", errors.New("empty result")
	}

	return translated, nil
}

func (yt *YoudaoTranslate) truncate(q string) string {
	count := utf8.RuneCountInString(q)
	if count <= 20 {
		return q
	}
	runeText := []rune(q)
	return fmt.Sprintf("%s%d%s", string(runeText[0:10]), count, string(runeText[count-10:]))
}

func (yt *YoudaoTranslate) encrypt(strSrc string) string {
	bt := []byte(strSrc)
	bts := sha256.Sum256(bt)
	return hex.EncodeToString(bts[:])
}

func (yt *YoudaoTranslate) getInput(q string) string {
	str := []rune(q)
	strLen := len(str)
	if strLen <= 20 {
		return q
	} else {
		return string(str[:10]) + strconv.Itoa(strLen) + string(str[strLen-10:])
	}
}

func (yt *YoudaoTranslate) getUuid() string {
	b := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type Deepl struct {
	baseURL string
	apiKey  string
}

type TranslationPayload struct {
	Text       []string `json:"text"`
	TargetLang string   `json:"target_lang"`
	SourceLang string   `json:"source_lang"`
	GlossaryID string   `json:"glossary_id"`
}

type TranslationResponse struct {
	Translations []Translation `json:"translations"`
}

type Translation struct {
	DetectedSourceLanguage string `json:"detected_source_language"`
	Text                   string `json:"text"`
}

func NewDeepl(apiKey string) *Deepl {
	baseURL := "https://api.deepl.com/v2"
	if strings.HasSuffix(apiKey, ":fx") {
		baseURL = "https://api-free.deepl.com/v2"
	}

	return &Deepl{apiKey: apiKey, baseURL: baseURL}
}

func (d *Deepl) Translate(text string, sourceLang, targetLang string, glossaryID string) (string, string, error) {
	sourceLang = strings.ToUpper(sourceLang)
	targetLang = strings.ToUpper(targetLang)

	payload := TranslationPayload{
		Text:       []string{text},
		TargetLang: targetLang,
		SourceLang: sourceLang,
		GlossaryID: glossaryID,
	}

	j, err := json.Marshal(payload)

	if err != nil {
		return "", "", err
	}

	apiUrl := fmt.Sprintf("%s/translate", d.baseURL)

	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(j))

	if err != nil {
		return "", "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("DeepL-Auth-Key %s", d.apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	responseData := TranslationResponse{}

	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &responseData)

	if err != nil {
		return "", "", err
	}

	var result []string
	for _, item := range responseData.Translations {
		sourceLang = item.DetectedSourceLanguage
		result = append(result, item.Text)
	}

	return strings.Join(result, "\n"), sourceLang, nil
}

func (d *Deepl) GetGlossaries() (string, error) {

	url := fmt.Sprintf("%s/glossaries", d.baseURL)

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("DeepL-Auth-Key %s", d.apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	return buf.String(), nil
}

type CreateGlossaryPayload struct {
	Name          string `json:"name"`
	SourceLang    string `json:"source_lang"`
	TargetLang    string `json:"target_lang"`
	EntriesFormat string `json:"entries_format"`
	Entries       string `json:"entries"`
}

type Glossary struct {
	ID           string `json:"glossary_id"`
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	SourceLang   string `json:"source_lang"`
	TargetLang   string `json:"target_lang"`
	CreationTime string `json:"creation_time"`
	EntryCount   int    `json:"entry_count"`
}

func (d *Deepl) CreateGlossary(name, sourceLang, targetLang string, entriesTSV io.Reader) (*Glossary, error) {

	buf := new(strings.Builder)
	_, err := io.Copy(buf, entriesTSV)

	if err != nil {
		return nil, err
	}

	entries := strings.ReplaceAll(buf.String(), "\r\n", "\n")

	payload := CreateGlossaryPayload{
		Name:          name,
		SourceLang:    sourceLang,
		TargetLang:    targetLang,
		EntriesFormat: "tsv",
		Entries:       entries,
	}

	j, err := json.Marshal(payload)

	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/glossaries", d.baseURL)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("DeepL-Auth-Key %s", d.apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	createdGlossary := Glossary{}

	err = json.NewDecoder(resp.Body).Decode(&createdGlossary)

	if err != nil {
		return nil, err
	}

	return &createdGlossary, nil
}

// TranslateHtml 将传入的html中的文字翻译成对应的语言
func (w *Website) TranslateHtml(content string, from, to string) (string, error) {
	// 解析HTML
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return "", err
	}

	// 提取需要翻译的文本
	textNodes := extractTextNodes(doc)
	texts := make([]string, len(textNodes))
	for i, info := range textNodes {
		texts[i] = info.text
	}

	// 翻译文本
	translatedTexts, err := w.TranslateTexts(texts, from, to)
	if err != nil {
		return "", err
	}

	// 将翻译后的文本替换回HTML
	for i, info := range textNodes {
		if i < len(translatedTexts) {
			if info.node.Type == html.ElementNode {
				// 处理属性翻译
				for j, attr := range info.node.Attr {
					if (attr.Key == "title" || attr.Key == "placeholder" || attr.Key == "alt" || attr.Key == "value") ||
						(info.node.Data == "meta" && attr.Key == "content" &&
							(containsAttr(info.node, "name", "description") ||
								containsAttr(info.node, "name", "keywords") ||
								containsAttr(info.node, "name", "title") ||
								containsAttr(info.node, "property", "description") ||
								containsAttr(info.node, "property", "keywords") ||
								containsAttr(info.node, "property", "title") ||
								containsAttr(info.node, "property", "image:alt") ||
								containsAttr(info.node, "property", "site_name"))) {
						info.node.Attr[j].Val = translatedTexts[i]
						break
					}
				}
			} else if info.node.Type == html.TextNode {
				info.node.Data = translatedTexts[i]
			}
		}
	}

	var output strings.Builder
	if err = html.Render(&output, doc); err != nil {
		return "", err
	}

	return output.String(), nil
}

// TranslateTexts 翻译文本数组
func (w *Website) TranslateTexts(texts []string, from, to string) ([]string, error) {
	// 创建去重映射表
	textMap := make(map[string]string)
	textIndices := make(map[string][]int)
	var uniqueTexts []string

	// 构建去重映射
	for i, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		// 有一些字符，是不需要走接口翻译的
		text2, isNeed := localReplace(text)
		if !isNeed {
			textMap[text] = text2
		} else {
			if _, exists := textMap[text]; !exists {
				textMap[text] = "" // 添加到映射表，先存空值
				uniqueTexts = append(uniqueTexts, text)
			}
		}
		textIndices[text] = append(textIndices[text], i)
	}

	// 处理未缓存的文本
	if len(uniqueTexts) > 0 {
		translatedBatch, err := w.translateBatch(uniqueTexts, from, to)
		if err != nil {
			return nil, err
		}
		for text, translated := range translatedBatch {
			textMap[text] = translated
		}
	}

	// 还原翻译结果到原始顺序
	translatedTexts := make([]string, len(texts))
	for text, indices := range textIndices {
		translated := textMap[text]
		for _, idx := range indices {
			translatedTexts[idx] = translated
		}
	}

	// 处理空文本
	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			translatedTexts[i] = text
		}
	}

	return translatedTexts, nil
}

// translateBatch 批量翻译文本
func (w *Website) translateBatch(texts []string, from, to string) (map[string]string, error) {
	if from == "" {
		from = "auto"
	}
	result := make(map[string]string)
	processBatch := func(text string) error {
		if len(text) == 0 {
			return nil
		}
		// 添加重试机制
		var translated string
		var err error
		// 最多重试10次
		for retries := 0; retries < 10; retries++ {
			translated, err = w.TranslateText(text, from, to)
			if err == nil {
				break
			}
			log.Printf("翻译重试 %d/10: %v", retries+1, err)
			time.Sleep(time.Second * time.Duration(retries+1))
		}
		if err != nil {
			return fmt.Errorf("翻译失败: %w", err)
		}

		mu.Lock()
		defer mu.Unlock()
		result[text] = translated

		return nil
	}

	wg := sync.WaitGroup{}
	var err error
	for _, text := range texts {
		wg.Add(1)
		go func(text string) {
			defer func() {
				wg.Done()
			}()
			err1 := processBatch(text)
			if err1 != nil {
				err = err1
			}
		}(text)
	}

	wg.Wait()

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (w *Website) TranslateText(text, from, to string) (string, error) {
	// 检查数据库存储内容
	textMd5 := library.Md5(from + "-" + to + "-" + text)
	var textLog model.TranslateTextLog
	if err := w.DB.Where("`md5` = ?", textMd5).First(&textLog).Error; err == nil {
		return textLog.Translated, nil
	}

	var content string
	var err error
	// 使用AI翻译
	if w.PluginTranslate.Engine == config.TranslateEngineAi {
		content, err = w.SelfAiTranslate(text, to)
		if err != nil {
			return "", err
		}
	} else if w.PluginTranslate.Engine == config.TranslateEngineBaidu {
		// 使用百度翻译
		baiduTranslate := NewBaiduTranslate(w.PluginTranslate.BaiduAppId, w.PluginTranslate.BaiduAppSecret)
		content, err = baiduTranslate.Translate(text, from, to)
		if err != nil {
			return "", err
		}
	} else if w.PluginTranslate.Engine == config.TranslateEngineYoudao {
		// 有道翻译
		youdaoTranslate := NewYoudaoTranslate(w.PluginTranslate.YoudaoAppKey, w.PluginTranslate.YoudaoAppSecret)
		content, err = youdaoTranslate.Translate(text, from, to)
		if err != nil {
			return "", err
		}
	} else if w.PluginTranslate.Engine == config.TranslateEngineDeepl {
		// Deepl翻译
		client := NewDeepl(w.PluginTranslate.DeeplAuthKey)
		glossariesId, err2 := client.GetGlossaries()
		if err2 != nil {
			return "", err2
		}
		content, _, err = client.Translate(
			text,
			from,
			to,
			glossariesId,
		)
		if err != nil {
			return "", err
		}
	} else {
		err = errors.New(w.Tr("NoAiGenerationSourceSelected"))
	}
	if content != "" {
		textLog = model.TranslateTextLog{
			Md5:        textMd5,
			Language:   from,
			ToLanguage: to,
			Text:       text,
			Translated: content,
		}
		_ = w.DB.Where("`md5` = ?", textMd5).FirstOrCreate(&textLog).Error
	}

	return content, err
}

// contains 检查字符串是否在切片中
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func containsAttr(node *html.Node, key, val string) bool {
	for _, attr := range node.Attr {
		if attr.Key == key && strings.Contains(attr.Val, val) {
			return true
		}
	}
	return false
}

type textNodeInfo struct {
	node  *html.Node
	text  string
	index int
}

// extractTextNodes 从HTML节点中提取需要翻译的文本节点
func extractTextNodes(n *html.Node) []textNodeInfo {
	var textNodes []textNodeInfo
	var traverse func(*html.Node, bool)
	var ignoreClass = []string{
		"languages",
	}
	var ignoreId = []string{
		"languages",
	}
	traverse = func(node *html.Node, parentSkip bool) {
		// 如果父节点被跳过，则当前节点及其所有子节点都跳过
		if parentSkip {
			return
		}

		// 跳过script和style标签，code标签
		if node.Type == html.ElementNode && (node.Data == "script" || node.Data == "style" || node.Data == "code") {
			return
		}

		// 检查是否需要跳过该节点
		shouldSkip := false
		for _, attr := range node.Attr {
			// 跳过 ignore-translate 类的节点
			if attr.Key == "ignore-translate" && attr.Val != "false" {
				shouldSkip = true
				break
			}
			if attr.Key == "class" && contains(ignoreClass, attr.Val) {
				shouldSkip = true
				break
			}
			if attr.Key == "id" && contains(ignoreId, attr.Val) {
				shouldSkip = true
				break
			}
			// 提取需要翻译的属性文本
			if !shouldSkip && node.Type == html.ElementNode {
				switch {
				case (attr.Key == "title" || attr.Key == "placeholder" || attr.Key == "alt" || attr.Key == "value") && strings.TrimSpace(attr.Val) != "":
					textNodes = append(textNodes, textNodeInfo{
						node:  node,
						text:  attr.Val,
						index: len(textNodes),
					})
				case node.Data == "meta" && attr.Key == "content" &&
					(containsAttr(node, "name", "description") ||
						containsAttr(node, "name", "keywords") ||
						containsAttr(node, "name", "title") ||
						containsAttr(node, "property", "description") ||
						containsAttr(node, "property", "keywords") ||
						containsAttr(node, "property", "title") ||
						containsAttr(node, "property", "image:alt") ||
						containsAttr(node, "property", "site_name")) && strings.TrimSpace(attr.Val) != "":
					textNodes = append(textNodes, textNodeInfo{
						node:  node,
						text:  attr.Val,
						index: len(textNodes),
					})
				}
			}
		}

		if !shouldSkip && node.Type == html.TextNode && strings.TrimSpace(node.Data) != "" {
			textNodes = append(textNodes, textNodeInfo{
				node:  node,
				text:  strings.TrimSpace(node.Data),
				index: len(textNodes),
			})
		}

		// 递归遍历子节点，传递当前节点是否应该被跳过的状态
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			traverse(child, shouldSkip)
		}
	}

	traverse(n, false)
	return textNodes
}

// localReplace 本地直接替换的部分，不需要翻译。
// return 字符串，字符串是否还需要翻译
func localReplace(s string) (string, bool) {
	// 本地直接替换的部分，不需要翻译。
	// 从中文的标点符号直接替换为英文的标点符号
	if len(s) < 2 {
		return s, false
	}

	if utf8.RuneCountInString(s) == 1 {
		// 从中文的标点符号直接替换为英文的标点符号
		switch s {
		case "，":
			return ",", false
		case "。":
			return ".", false
		case "；":
			return ";", false
		case "：":
			return ":", false
		case "？":
			return "?", false
		case "！":
			return "!", false
		case "（":
			return "(", false
		case "）":
			return ")", false
		case "【":
			return "[", false
		case "】":
			return "]", false
		case "《":
			return "<", false
		case "》":
			return ">", false
		case "“":
			return "\"", false
		case "”":
			return "\"", false
		case "‘":
			return "'", false
		case "’":
			return "'", false
		case "、":
			return "/", false
		case "～":
			return "~", false
		default:
			return s, true
		}
	} else {
		// s 是纯数字和点
		isNumberAndDot := true
		for _, r := range s {
			if !((r >= '0' && r <= '9') || r == '.' || r == ' ' || r == ':' || r == '-' || r == '(' || r == ')') {
				isNumberAndDot = false
				break
			}
		}
		if isNumberAndDot {
			return s, false
		}

		// s 是备案号
		// 备案号格式：京ICP备12345678号-1 或 粤ICP备12345678号
		beianRegex := regexp.MustCompile(`^[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼使领][A-Z]{2,3}备\d{6,10}号(-\d+)?$`)
		if beianRegex.MatchString(s) {
			return s, false
		}
	}

	return s, true
}

func (w *Website) ReplaceTranslateText(buf []byte, oldText, newText string) ([]byte, bool) {
	if len(oldText) == 0 {
		return buf, false
	}
	if bytes.Contains(buf, []byte(oldText)) {
		// 只替换 >.*< 以及 alt=".*" 之间的内容
		buf = regexp.MustCompile(`(>[^<]*?)`+regexp.QuoteMeta(oldText)+`([^<]*<)`).ReplaceAllFunc(buf, func(match []byte) []byte {
			return bytes.Replace(match, []byte(oldText), []byte(newText), 1)
		})
		buf = regexp.MustCompile(`(\balt\s*=\s*["\'][^"\']*?)`+regexp.QuoteMeta(oldText)+`([^"\']*?["\'])`).ReplaceAllFunc(buf, func(match []byte) []byte {
			return bytes.Replace(match, []byte(oldText), []byte(newText), 1)
		})
		buf = regexp.MustCompile(`(\btitle\s*=\s*["\'][^"\']*?)`+regexp.QuoteMeta(oldText)+`([^"\']*?["\'])`).ReplaceAllFunc(buf, func(match []byte) []byte {
			return bytes.Replace(match, []byte(oldText), []byte(newText), 1)
		})
		buf = regexp.MustCompile(`(\bcontent\s*=\s*["\'][^"\']*?)`+regexp.QuoteMeta(oldText)+`([^"\']*?["\'])`).ReplaceAllFunc(buf, func(match []byte) []byte {
			return bytes.Replace(match, []byte(oldText), []byte(newText), 1)
		})
		return buf, true
	}
	return buf, false
}
